package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestOPDSAllSeriesAndWorksExposeSameDownloadScopedWorkCatalog(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-work-user", "opds-work-user")
	noDownloadUser, noDownloadToken := createOPDSTestUserAndKey(t, "opds-no-download-user", "opds-no-download-user")
	createOPDSHandlerLibrary(t, "opds-work-download", "comic", true)
	createOPDSHandlerLibrary(t, "opds-work-view", "comic", true)
	createOPDSHandlerComic(t, "opds-work-ch1", "Unified Work/Chapter 001.cbz", "comic", "opds-work-download")
	createOPDSHandlerComic(t, "opds-work-ch2", "Unified Work/Chapter 002.cbz", "comic", "opds-work-download")
	createOPDSHandlerComic(t, "opds-private-ch1", "Private Work/Chapter 001.cbz", "comic", "opds-work-view")

	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{
		{LibraryID: "opds-work-download", CanDownload: true},
		{LibraryID: "opds-work-view", CanView: true},
	}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}
	if err := store.SetUserLibraryAccess(noDownloadUser.ID, []store.LibraryAccessReq{
		{LibraryID: "opds-work-download", CanView: true},
	}); err != nil {
		t.Fatalf("SetUserLibraryAccess(no-download) failed: %v", err)
	}

	for _, endpoint := range []string{"/api/opds/all", "/api/opds/series", "/api/opds/works"} {
		response := performOPDSBasicRequest(router, endpoint, user.Username, token)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", endpoint, response.Code, response.Body.String())
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, service.OPDSNavigationMIME) {
			t.Fatalf("%s Content-Type = %q, want Work navigation feed", endpoint, contentType)
		}
		body := response.Body.String()
		if strings.Count(body, "<entry>") != 1 {
			t.Fatalf("%s should contain one Work entry, got: %s", endpoint, body)
		}
		if !strings.Contains(body, "<title>Unified Work</title>") {
			t.Fatalf("%s missing aggregated Work: %s", endpoint, body)
		}
		if strings.Contains(body, "Private Work") || strings.Contains(body, "opds-private-ch1") {
			t.Fatalf("%s leaked view-only library content: %s", endpoint, body)
		}
		if !strings.Contains(body, "/api/opds/works/work_") {
			t.Fatalf("%s does not link to canonical Work unit feed: %s", endpoint, body)
		}

		denied := performOPDSBasicRequest(router, endpoint, noDownloadUser.Username, noDownloadToken)
		if denied.Code != http.StatusOK {
			t.Fatalf("%s for no-download user returned %d: %s", endpoint, denied.Code, denied.Body.String())
		}
		if strings.Contains(denied.Body.String(), "<entry>") {
			t.Fatalf("%s fell back to full catalog for no-download user: %s", endpoint, denied.Body.String())
		}
	}
}

func TestOPDSWorkUnitFeedUsesPhysicalAcquisitionAndInternalPageOffsets(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-archive-work-user", "opds-archive-work-user")
	libraryDir := t.TempDir()
	library := &model.Library{
		ID:            "opds-archive-work-lib",
		Name:          "opds-archive-work-lib",
		Type:          "comic",
		RootPath:      libraryDir,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	const filename = "Archive Work.cbz"
	archivePath := filepath.Join(libraryDir, filename)
	if err := os.WriteFile(archivePath, createChapterImageCBZ(t), 0o600); err != nil {
		t.Fatalf("write chapter CBZ failed: %v", err)
	}
	t.Cleanup(func() {
		service.InvalidateContentFileCaches("opds-archive-work-comic", archivePath)
	})
	createOPDSHandlerComic(t, "opds-archive-work-comic", filename, "comic", library.ID)
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "title" = 'Archive Work', "pageCount" = 4, "fileSize" = 1234
		WHERE "id" = 'opds-archive-work-comic'
	`); err != nil {
		t.Fatalf("update archive comic failed: %v", err)
	}
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: library.ID, CanDownload: true,
	}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}
	readAt := time.Date(2026, time.August, 5, 12, 34, 56, 0, time.UTC)
	if _, err := store.DB().Exec(`
		INSERT INTO "UserComicState" ("userId", "comicId", "lastReadPage", "lastReadAt")
		VALUES (?, 'opds-archive-work-comic', 3, ?)
	`, user.ID, readAt); err != nil {
		t.Fatalf("insert archive Work progress failed: %v", err)
	}

	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType:      "comic",
		UserID:           user.ID,
		LibraryIDs:       []string{library.ID},
		FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatalf("GetAllComics failed: %v", err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{ProbeInternalArchive: true})
	if len(works) != 1 || len(works[0].Units) != 2 {
		t.Fatalf("fixture built %d works and %d units, want 1 work and 2 units", len(works), unitCount(works))
	}

	response := performOPDSBasicRequest(router, "/api/opds/works/"+works[0].ID, user.Username, token)
	if response.Code != http.StatusOK {
		t.Fatalf("Work unit feed returned %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Count(body, "<entry>") != 2 {
		t.Fatalf("Work unit feed should expose two Unit entries: %s", body)
	}
	for _, unit := range works[0].Units {
		if !strings.Contains(body, "<id>urn:nowen:unit:"+unit.ID+"</id>") {
			t.Fatalf("Work unit feed missing stable Unit identity %q: %s", unit.ID, body)
		}
		expectedCover := fmt.Sprintf("/api/opds/unit-cover/opds-archive-work-comic?page=%d", unit.StartPage)
		if !strings.Contains(body, expectedCover) {
			t.Fatalf("Unit %q missing independent cover %q: %s", unit.DisplayLabel, expectedCover, body)
		}
		expectedStream := fmt.Sprintf(
			"/api/opds/stream/opds-archive-work-comic?page={pageNumber}&amp;width={maxWidth}&amp;startPage=%d&amp;pageCount=%d",
			unit.StartPage,
			unit.PageCount,
		)
		if !strings.Contains(body, expectedStream) {
			t.Fatalf("Unit %q missing page offset stream %q: %s", unit.DisplayLabel, expectedStream, body)
		}
	}
	if strings.Contains(body, "/api/opds/download/opds-archive-work-comic/Archive%20Work.cbz") {
		t.Fatalf("internal virtual Units must be PSE-only instead of duplicating the whole archive acquisition: %s", body)
	}
	firstCover := performOPDSBasicRequest(
		router,
		fmt.Sprintf("/api/opds/unit-cover/opds-archive-work-comic?page=%d", works[0].Units[0].StartPage),
		user.Username,
		token,
	)
	secondCover := performOPDSBasicRequest(
		router,
		fmt.Sprintf("/api/opds/unit-cover/opds-archive-work-comic?page=%d", works[0].Units[1].StartPage),
		user.Username,
		token,
	)
	if firstCover.Code != http.StatusOK || secondCover.Code != http.StatusOK {
		t.Fatalf("Unit covers returned %d and %d", firstCover.Code, secondCover.Code)
	}
	if bytes.Equal(firstCover.Body.Bytes(), secondCover.Body.Bytes()) {
		t.Fatal("different internal Units returned the same cover image")
	}
	if strings.Count(body, `pse:lastRead="2"`) != 1 ||
		!strings.Contains(body, `pse:lastReadDate="`+readAt.Format(time.RFC3339)+`"`) {
		t.Fatalf("internal Unit progress was not projected to the relative page: %s", body)
	}

	outOfUnit := performOPDSBasicRequest(
		router,
		"/api/opds/stream/opds-archive-work-comic?page=2&startPage=0&pageCount=2",
		user.Username,
		token,
	)
	if outOfUnit.Code != http.StatusNotFound {
		t.Fatalf("page beyond Unit boundary returned %d, want 404: %s", outOfUnit.Code, outOfUnit.Body.String())
	}
}

func TestOPDSInternalUnitCoverUsesDetectedCoverPage(t *testing.T) {
	unit := service.WorkUnit{
		ComicID:      "archive-comic",
		InternalPath: "Chapter 001",
		StartPage:    4,
		CoverPage:    7,
	}
	got := opdsUnitCoverHref(unit)
	want := "/api/opds/unit-cover/archive-comic?page=7"
	if got != want {
		t.Fatalf("opdsUnitCoverHref() = %q, want detected non-leading cover %q", got, want)
	}
}

func TestOPDSWorkCoverBuildsDownloadScopedWorkIndexOnceForMultipleCovers(t *testing.T) {
	setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, _ := createOPDSTestUserAndKey(t, "opds-cover-index-user", "opds-cover-index-user")
	libraryDir := t.TempDir()
	library := &model.Library{
		ID: "opds-cover-index-lib", Name: "opds-cover-index-lib", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}
	const comicID = "opds-cover-index-comic"
	const filename = "Cover Index.cbz"
	archivePath := filepath.Join(libraryDir, filename)
	if err := os.WriteFile(archivePath, createChapterImageCBZ(t), 0o600); err != nil {
		t.Fatalf("write cover index CBZ: %v", err)
	}
	t.Cleanup(func() { service.InvalidateContentFileCaches(comicID, archivePath) })
	createOPDSHandlerComic(t, comicID, filename, "comic", library.ID)
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "pageCount" = 4 WHERE "id" = ?`, comicID); err != nil {
		t.Fatalf("update cover index comic: %v", err)
	}
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: library.ID, CanDownload: true,
	}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}

	handler := NewOPDSHandler()
	loadCalls := 0
	handler.workLoader = func(c *gin.Context) ([]opdsWorkCatalogItem, error) {
		loadCalls++
		return []opdsWorkCatalogItem{
			{Work: service.Work{ID: "work-a", LibraryID: library.ID, CoverComicID: comicID}},
			{Work: service.Work{ID: "work-b", LibraryID: library.ID, CoverComicID: comicID}},
		}, nil
	}
	for _, workID := range []string{"work-a", "work-b"} {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/opds/work-cover/"+workID, nil)
		c.Params = gin.Params{{Key: "id", Value: workID}}
		c.Set("auth_user", &model.AuthUser{ID: user.ID, Username: user.Username, Role: user.Role})
		handler.WorkCover(c)
		if response.Code != http.StatusOK {
			t.Fatalf("WorkCover(%s) returned %d: %s", workID, response.Code, response.Body.String())
		}
	}
	if loadCalls != 1 {
		t.Fatalf("two Work covers rebuilt the full catalog %d times, want one indexed load", loadCalls)
	}
	if err := store.SetUserLibraryAccess(user.ID, nil); err != nil {
		t.Fatalf("revoke download access: %v", err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/opds/work-cover/work-a", nil)
	c.Params = gin.Params{{Key: "id", Value: "work-a"}}
	c.Set("auth_user", &model.AuthUser{ID: user.ID, Username: user.Username, Role: user.Role})
	handler.WorkCover(c)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cached Work cover survived download permission revocation: status=%d body=%s", response.Code, response.Body.String())
	}
	if loadCalls != 1 {
		t.Fatalf("permission revocation triggered an unnecessary full catalog rebuild: calls=%d", loadCalls)
	}
}

func TestOPDSRecentFavoritesSearchAggregateWorksAndExcludeWrongLibraryTypes(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-filtered-work-user", "opds-filtered-work-user")
	createOPDSHandlerLibrary(t, "opds-filter-comic", "comic", true)
	createOPDSHandlerLibrary(t, "opds-filter-novel", "novel", true)
	createOPDSHandlerLibrary(t, "opds-filter-disabled", "comic", false)
	createOPDSHandlerComic(t, "filtered-ch1", "Filtered Work/Chapter Alpha.cbz", "comic", "opds-filter-comic")
	createOPDSHandlerComic(t, "novel-fake-comic", "Novel Shelf/Fake.cbz", "comic", "opds-filter-novel")
	createOPDSHandlerComic(t, "disabled-comic", "Disabled Work/Chapter.cbz", "comic", "opds-filter-disabled")
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{
		{LibraryID: "opds-filter-comic", CanDownload: true},
		{LibraryID: "opds-filter-novel", CanDownload: true},
		{LibraryID: "opds-filter-disabled", CanDownload: true},
	}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "addedAt" = '2026-08-05T01:00:00Z', "updatedAt" = '2026-08-05T02:00:00Z'
		WHERE "id" = 'filtered-ch1';
		INSERT INTO "ComicSeries" ("id", "libraryId", "rootRelativePath", "title", "sortTitle", "coverComicId", "manualLocked")
		VALUES ('filtered-series', 'opds-filter-comic', 'Filtered Work', 'Filtered Work', 'filtered work', 'filtered-ch1', 1);
		INSERT INTO "ComicSeriesItem" ("seriesId", "comicId", "sortIndex", "displayLabel")
		VALUES ('filtered-series', 'filtered-ch1', 0, 'Alpha');
		INSERT INTO "UserComicState" ("userId", "comicId", "isFavorite")
		VALUES (?, 'filtered-ch1', 1)
	`, user.ID); err != nil {
		t.Fatalf("prepare Work filters failed: %v", err)
	}

	for _, endpoint := range []string{
		"/api/opds/recent",
		"/api/opds/favorites",
		"/api/opds/search?q=Alpha",
	} {
		response := performOPDSBasicRequest(router, endpoint, user.Username, token)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", endpoint, response.Code, response.Body.String())
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, service.OPDSNavigationMIME) {
			t.Fatalf("%s Content-Type = %q, want navigation", endpoint, contentType)
		}
		body := response.Body.String()
		if strings.Count(body, "<entry>") != 1 {
			t.Fatalf("%s did not aggregate matching comics into one Work: %s", endpoint, body)
		}
		if strings.Contains(body, "Novel Shelf") || strings.Contains(body, "Disabled Work") {
			t.Fatalf("%s exposed a non-comic or disabled library: %s", endpoint, body)
		}
	}
}

func TestOPDSPDFWorkKeepsPhysicalAcquisition(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-pdf-work-user", "opds-pdf-work-user")
	createOPDSHandlerLibrary(t, "opds-pdf-work-lib", "comic", true)
	createOPDSHandlerComic(t, "opds-pdf-work", "PDF Work.pdf", "comic", "opds-pdf-work-lib")
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{LibraryID: "opds-pdf-work-lib", CanDownload: true}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}
	list := performOPDSBasicRequest(router, "/api/opds/works", user.Username, token)
	if list.Code != http.StatusOK {
		t.Fatalf("PDF Work list returned %d: %s", list.Code, list.Body.String())
	}
	works, err := loadOPDSWorksFromResponseFixture(user.ID, "opds-pdf-work-lib")
	if err != nil || len(works) != 1 {
		t.Fatalf("build PDF Work fixture: %v, count=%d", err, len(works))
	}
	detail := performOPDSBasicRequest(router, "/api/opds/works/"+works[0].ID, user.Username, token)
	if detail.Code != http.StatusOK {
		t.Fatalf("PDF Work detail returned %d: %s", detail.Code, detail.Body.String())
	}
	body := detail.Body.String()
	if strings.Count(body, "<entry>") != 1 ||
		!strings.Contains(body, `type="application/pdf"`) ||
		!strings.Contains(body, "/api/opds/download/opds-pdf-work/PDF%20Work.pdf") {
		t.Fatalf("PDF Work did not retain one physical acquisition: %s", body)
	}
}

func TestOPDSWorkCatalogAppliesSeriesMetadataAndActualUpdatedAt(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-metadata-user", "opds-metadata-user")
	createOPDSHandlerLibrary(t, "opds-metadata-lib", "comic", true)
	createOPDSHandlerComic(t, "opds-meta-ch1", "Metadata Work/Chapter 001.cbz", "comic", "opds-metadata-lib")
	createOPDSHandlerComic(t, "opds-meta-ch2", "Metadata Work/Chapter 002.cbz", "comic", "opds-metadata-lib")
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{LibraryID: "opds-metadata-lib", CanDownload: true}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "addedAt" = '2026-08-01T00:00:00Z', "updatedAt" = '2026-08-02T00:00:00Z'
		WHERE "id" IN ('opds-meta-ch1', 'opds-meta-ch2');
		INSERT INTO "ComicSeries" (
			"id", "libraryId", "rootRelativePath", "title", "sortTitle", "coverComicId",
			"coverUrl", "author", "description", "publisher", "language", "genre", "updatedAt", "manualLocked"
		) VALUES (
			'opds-metadata-series', 'opds-metadata-lib', 'Metadata Work', 'Scraped Work', 'scraped work', 'opds-meta-ch2',
			'https://metadata.example/cover.jpg', 'Scraped Author', 'Scraped Description',
			'Scraped Publisher', 'zh-CN', 'Action', '2026-08-05T04:05:06Z', 1
		);
		INSERT INTO "ComicSeriesItem" ("seriesId", "comicId", "sortIndex", "displayLabel") VALUES
			('opds-metadata-series', 'opds-meta-ch1', 0, '001'),
			('opds-metadata-series', 'opds-meta-ch2', 1, '002')
	`); err != nil {
		t.Fatalf("insert series metadata failed: %v", err)
	}
	if err := store.SetSeriesTags("opds-metadata-series", []string{"Completed"}); err != nil {
		t.Fatalf("SetSeriesTags failed: %v", err)
	}
	response := performOPDSBasicRequest(router, "/api/opds/works", user.Username, token)
	if response.Code != http.StatusOK {
		t.Fatalf("metadata Work feed returned %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		"<title>Scraped Work</title>",
		"<name>Scraped Author</name>",
		"<summary type=\"text\">Scraped Description</summary>",
		"<dcterms:language>zh-CN</dcterms:language>",
		`term="Completed"`,
		"<updated>2026-08-05T04:05:06Z</updated>",
		"/api/opds/work-cover/",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metadata Work feed missing %q: %s", expected, body)
		}
	}
}

func loadOPDSWorksFromResponseFixture(userID, libraryID string) ([]service.Work, error) {
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", UserID: userID, LibraryIDs: []string{libraryID}, FilterLibraryIDs: true,
	})
	if err != nil {
		return nil, err
	}
	return service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{ProbeInternalArchive: true}), nil
}

func TestOPDSWorkUnitFeedKeepsFolderUnitsAsPhysicalPageStreams(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-folder-work-user", "opds-folder-work-user")
	libraryDir := t.TempDir()
	library := &model.Library{
		ID:            "opds-folder-work-lib",
		Name:          "opds-folder-work-lib",
		Type:          "comic",
		RootPath:      libraryDir,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}
	for _, chapter := range []string{"Chapter 001", "Chapter 002"} {
		if err := os.MkdirAll(filepath.Join(libraryDir, "Folder Work", chapter), 0o755); err != nil {
			t.Fatalf("create folder chapter %s: %v", chapter, err)
		}
	}
	createOPDSHandlerComic(t, "opds-folder-ch1", "Folder Work/Chapter 001/", "comic", library.ID)
	createOPDSHandlerComic(t, "opds-folder-ch2", "Folder Work/Chapter 002/", "comic", library.ID)
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "pageCount" = 2
		WHERE "id" IN ('opds-folder-ch1', 'opds-folder-ch2')
	`); err != nil {
		t.Fatalf("update folder page counts failed: %v", err)
	}
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: library.ID, CanDownload: true,
	}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}

	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType:      "comic",
		UserID:           user.ID,
		LibraryIDs:       []string{library.ID},
		FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatalf("GetAllComics failed: %v", err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{ProbeInternalArchive: true})
	if len(works) != 1 || len(works[0].Units) != 2 {
		t.Fatalf("fixture built %d works and %d units, want 1 work and 2 units", len(works), unitCount(works))
	}

	response := performOPDSBasicRequest(router, "/api/opds/works/"+works[0].ID, user.Username, token)
	if response.Code != http.StatusOK {
		t.Fatalf("folder Work unit feed returned %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Count(body, "<entry>") != 2 {
		t.Fatalf("folder Work should expose two Unit entries: %s", body)
	}
	for _, comicID := range []string{"opds-folder-ch1", "opds-folder-ch2"} {
		if !strings.Contains(body, "/api/opds/stream/"+comicID+"?") {
			t.Fatalf("folder Unit %s is missing physical comic page stream: %s", comicID, body)
		}
		if strings.Contains(body, "/api/opds/download/"+comicID) {
			t.Fatalf("folder Unit %s must not advertise an unavailable raw-file download: %s", comicID, body)
		}
	}
}

func unitCount(works []service.Work) int {
	if len(works) == 0 {
		return 0
	}
	return len(works[0].Units)
}

func createChapterImageCBZ(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := []struct {
		name  string
		color color.NRGBA
	}{
		{name: "Ch001/001.png", color: color.NRGBA{R: 255, A: 255}},
		{name: "Ch001/002.png", color: color.NRGBA{G: 255, A: 255}},
		{name: "Ch002/001.png", color: color.NRGBA{B: 255, A: 255}},
		{name: "Ch002/002.png", color: color.NRGBA{R: 255, G: 255, A: 255}},
	}
	for _, spec := range entries {
		entry, err := writer.Create(spec.name)
		if err != nil {
			t.Fatalf("create %s: %v", spec.name, err)
		}
		page := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				page.Set(x, y, spec.color)
			}
		}
		if err := png.Encode(entry, page); err != nil {
			t.Fatalf("encode %s: %v", spec.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close chapter CBZ: %v", err)
	}
	return buffer.Bytes()
}
