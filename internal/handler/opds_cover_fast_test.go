package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func TestWorkCoverFastUsesPersistedWorkWithoutLoadingFullCatalog(t *testing.T) {
	setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	user, _ := createOPDSTestUserAndKey(t, "opds-fast-cover-user", "opds-fast-cover-user")
	createOPDSHandlerLibrary(t, "opds-fast-cover-library", "comic", true)
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: "opds-fast-cover-library", CanDownload: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		LibraryID: "opds-fast-cover-library",
		RootPath:  "Fast Work",
		Title:     "Fast Work",
	}}); err != nil {
		t.Fatal(err)
	}
	workID := workmodel.StableWorkID("opds-fast-cover-library", "Fast Work")
	work, err := store.GetLogicalWork(workID)
	if err != nil || work == nil {
		t.Fatalf("persisted Work %q unavailable: work=%#v err=%v", workID, work, err)
	}

	if err := os.MkdirAll(config.GetThumbnailsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(workID))
	coverData := []byte("\x89PNG\r\n\x1a\nfast-cover")
	if err := os.WriteFile(cachePath, coverData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cachePath) })

	handler := NewOPDSHandler()
	loadCalls := 0
	handler.workLoader = func(*gin.Context) ([]opdsWorkCatalogItem, error) {
		loadCalls++
		return nil, nil
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/opds/work-cover/"+workID, nil)
	c.Params = gin.Params{{Key: "id", Value: workID}}
	c.Set("auth_user", &model.AuthUser{ID: user.ID, Username: user.Username, Role: user.Role})

	handler.WorkCoverFast(c)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if loadCalls != 0 {
		t.Fatalf("persisted Work cover loaded full catalog %d times, want 0", loadCalls)
	}
	if response.Body.String() != string(coverData) {
		t.Fatalf("body=%q want cached cover", response.Body.Bytes())
	}
}

func TestWorkCoverFastReturnsNotModifiedBeforeReadingCacheFile(t *testing.T) {
	handler, user, workID, cachePath, _ := setupPersistedFastWorkCover(t)
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	etag := fmt.Sprintf(`"%s-%s"`,
		strconv.FormatInt(info.ModTime().UnixNano(), 36),
		strconv.FormatInt(info.Size(), 36),
	)

	originalReadFile := opdsCoverReadFile
	readCalls := 0
	opdsCoverReadFile = func(string) ([]byte, error) {
		readCalls++
		return nil, fmt.Errorf("cache file must not be read for a matching ETag")
	}
	t.Cleanup(func() { opdsCoverReadFile = originalReadFile })

	response := performFastWorkCoverRequest(handler, user, http.MethodGet, workID, map[string]string{
		"If-None-Match": etag,
	})
	if response.Code != http.StatusNotModified {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if readCalls != 0 {
		t.Fatalf("matching ETag read cache file %d times, want 0", readCalls)
	}
	if response.Header().Get("ETag") != etag {
		t.Fatalf("ETag=%q want %q", response.Header().Get("ETag"), etag)
	}
}

func TestWorkCoverFastHeadUsesStatWithoutReadingCacheFile(t *testing.T) {
	handler, user, workID, _, coverData := setupPersistedFastWorkCover(t)
	originalReadFile := opdsCoverReadFile
	readCalls := 0
	opdsCoverReadFile = func(string) ([]byte, error) {
		readCalls++
		return nil, fmt.Errorf("HEAD must not read cache file")
	}
	t.Cleanup(func() { opdsCoverReadFile = originalReadFile })

	response := performFastWorkCoverRequest(handler, user, http.MethodHead, workID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if readCalls != 0 {
		t.Fatalf("HEAD read cache file %d times, want 0", readCalls)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("HEAD body length=%d want 0", response.Body.Len())
	}
	if response.Header().Get("Content-Length") != strconv.Itoa(len(coverData)) {
		t.Fatalf("Content-Length=%q want %d", response.Header().Get("Content-Length"), len(coverData))
	}
	if response.Header().Get("ETag") == "" {
		t.Fatal("HEAD response missing ETag")
	}
}

func TestWorkCoverFastRedirectsRemoteCoverPrivatelyAndStartsDownload(t *testing.T) {
	handler, user, workID, cachePath, _ := setupPersistedFastWorkCover(t)
	if err := os.Remove(cachePath); err != nil {
		t.Fatal(err)
	}
	remoteURL := "https://covers.example.test/work.jpg"
	if err := store.UpdateLogicalWorkCover(workID, store.LogicalWorkCoverUpdate{
		CoverURL: &remoteURL,
	}); err != nil {
		t.Fatal(err)
	}

	originalDownload := opdsWorkCoverDownload
	downloaded := make(chan [2]string, 1)
	opdsWorkCoverDownload = func(id, rawURL string) {
		downloaded <- [2]string{id, rawURL}
	}
	t.Cleanup(func() { opdsWorkCoverDownload = originalDownload })

	response := performFastWorkCoverRequest(handler, user, http.MethodGet, workID, nil)
	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != remoteURL {
		t.Fatalf("Location=%q want %q", response.Header().Get("Location"), remoteURL)
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control=%q", response.Header().Get("Cache-Control"))
	}
	vary := response.Header().Get("Vary")
	if !strings.Contains(vary, "Authorization") || !strings.Contains(vary, "Cookie") {
		t.Fatalf("Vary=%q want Authorization and Cookie", vary)
	}
	select {
	case got := <-downloaded:
		if got != [2]string{workID, remoteURL} {
			t.Fatalf("download=%#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("remote Work cover download was not started")
	}
}

func TestWorkCoverFastFallsBackToRepresentativeComicCover(t *testing.T) {
	handler, user, workID, workCachePath, _ := setupPersistedFastWorkCover(t)
	if err := os.Remove(workCachePath); err != nil {
		t.Fatal(err)
	}
	const comicID = "opds-fast-fallback-comic"
	createOPDSHandlerComic(t, comicID, "Fallback.cbz", "comic", "opds-fast-fixture-library")
	if _, err := store.DB().Exec(
		`UPDATE "Comic" SET "coverImageUrl" = 'https://covers.example.test/fallback.jpg' WHERE "id" = ?`,
		comicID,
	); err != nil {
		t.Fatal(err)
	}
	coverComicID := comicID
	if err := store.UpdateLogicalWorkCover(workID, store.LogicalWorkCoverUpdate{
		CoverComicID: &coverComicID,
	}); err != nil {
		t.Fatal(err)
	}
	comicCoverData := []byte("\x89PNG\r\n\x1a\nfallback-comic-cover")
	comicCachePath := filepath.Join(config.GetThumbnailsDir(), archive.ThumbnailCacheName(comicID))
	if err := os.WriteFile(comicCachePath, comicCoverData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(comicCachePath) })

	response := performFastWorkCoverRequest(handler, user, http.MethodGet, workID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != string(comicCoverData) {
		t.Fatalf("body=%q want representative Comic cover", response.Body.Bytes())
	}
}

func TestWorkCoverFastFallsBackToLegacyWorkCatalogWhenLogicalWorkIsMissing(t *testing.T) {
	setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	user, _ := createOPDSTestUserAndKey(t, "opds-fast-legacy-user", "opds-fast-legacy-user")
	const libraryID = "opds-fast-legacy-library"
	const comicID = "opds-fast-legacy-comic"
	const workID = "legacy-unpersisted-work"
	createOPDSHandlerLibrary(t, libraryID, "comic", true)
	createOPDSHandlerComic(t, comicID, "Legacy.cbz", "comic", libraryID)
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: libraryID, CanDownload: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(
		`UPDATE "Comic" SET "coverImageUrl" = 'https://covers.example.test/legacy.jpg' WHERE "id" = ?`,
		comicID,
	); err != nil {
		t.Fatal(err)
	}
	coverData := []byte("\x89PNG\r\n\x1a\nlegacy-cover")
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.ThumbnailCacheName(comicID))
	if err := os.MkdirAll(config.GetThumbnailsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, coverData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cachePath) })

	handler := NewOPDSHandler()
	loadCalls := 0
	handler.workLoader = func(*gin.Context) ([]opdsWorkCatalogItem, error) {
		loadCalls++
		return []opdsWorkCatalogItem{{
			Work: service.Work{
				ID: workID, LibraryID: libraryID, CoverComicID: comicID,
			},
		}}, nil
	}
	response := performFastWorkCoverRequest(handler, user, http.MethodGet, workID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if loadCalls != 1 {
		t.Fatalf("legacy fallback loader calls=%d want 1", loadCalls)
	}
	if response.Body.String() != string(coverData) {
		t.Fatalf("body=%q want legacy Comic cover", response.Body.Bytes())
	}
}

func TestWorkCoverFastRequiresDownloadPermissionForPersistedWork(t *testing.T) {
	handler, user, workID, _, _ := setupPersistedFastWorkCover(t)
	if err := store.SetUserLibraryAccess(user.ID, nil); err != nil {
		t.Fatal(err)
	}
	loadCalls := 0
	handler.workLoader = func(*gin.Context) ([]opdsWorkCatalogItem, error) {
		loadCalls++
		return nil, nil
	}

	response := performFastWorkCoverRequest(handler, user, http.MethodGet, workID, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if loadCalls != 0 {
		t.Fatalf("permission denial loaded full catalog %d times, want 0", loadCalls)
	}
}

func setupPersistedFastWorkCover(t *testing.T) (*OPDSHandler, *model.User, string, string, []byte) {
	t.Helper()
	setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	user, _ := createOPDSTestUserAndKey(t, "opds-fast-fixture-user", "opds-fast-fixture-user")
	const libraryID = "opds-fast-fixture-library"
	const rootPath = "Fast Fixture Work"
	createOPDSHandlerLibrary(t, libraryID, "comic", true)
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: libraryID, CanDownload: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		LibraryID: libraryID,
		RootPath:  rootPath,
		Title:     rootPath,
	}}); err != nil {
		t.Fatal(err)
	}
	workID := workmodel.StableWorkID(libraryID, rootPath)
	if err := os.MkdirAll(config.GetThumbnailsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(workID))
	coverData := []byte("\x89PNG\r\n\x1a\nfast-fixture-cover")
	if err := os.WriteFile(cachePath, coverData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cachePath) })
	return NewOPDSHandler(), user, workID, cachePath, coverData
}

func performFastWorkCoverRequest(
	handler *OPDSHandler,
	user *model.User,
	method string,
	workID string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(method, "/api/opds/work-cover/"+workID, nil)
	for name, value := range headers {
		c.Request.Header.Set(name, value)
	}
	c.Params = gin.Params{{Key: "id", Value: workID}}
	c.Set("auth_user", &model.AuthUser{ID: user.ID, Username: user.Username, Role: user.Role})
	handler.WorkCoverFast(c)
	c.Writer.WriteHeaderNow()
	return response
}
