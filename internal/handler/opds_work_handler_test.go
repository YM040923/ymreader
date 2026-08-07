package handler

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
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

func TestOPDSSinglePhysicalArchivePublishesOnlyOriginalAcquisition(t *testing.T) {
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
	if strings.Count(body, "<entry>") != 1 {
		t.Fatalf("single physical archive should expose exactly one acquisition: %s", body)
	}
	if !strings.Contains(body, "<id>urn:nowen:comic:opds-archive-work-comic</id>") {
		t.Fatalf("single physical archive is missing stable comic identity: %s", body)
	}
	if !strings.Contains(body, "/api/opds/download/opds-archive-work-comic/Archive%20Work.cbz") {
		t.Fatalf("single physical archive did not publish its original file: %s", body)
	}
	for _, forbidden := range []string{"/api/opds/units/", "/continuous/", "startPage=", "pageCount="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("single physical archive leaked %q instead of one original acquisition: %s", forbidden, body)
		}
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		virtualUnit := performOPDSRequest(
			router,
			method,
			"/api/opds/units/"+works[0].Units[0].ID+"/download",
			user.Username,
			token,
			nil,
		)
		if virtualUnit.Code != http.StatusNotFound {
			t.Fatalf("%s physical internal Unit virtual download status=%d, want 404", method, virtualUnit.Code)
		}
		if method == http.MethodHead && virtualUnit.Body.Len() != 0 {
			t.Fatalf("physical internal Unit HEAD returned body %q", virtualUnit.Body.String())
		}
	}

	if strings.Count(body, `pse:lastRead="4"`) != 1 ||
		!strings.Contains(body, `pse:lastReadDate="`+readAt.Format(time.RFC3339)+`"`) {
		t.Fatalf("physical archive progress was not preserved on the whole-file stream: %s", body)
	}
}

func TestOPDSMultiPhysicalWorkPublishesAllFilesInNaturalOrderWithStablePagination(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-multi-physical-user", "opds-multi-physical-user")
	libraryDir := t.TempDir()
	library := &model.Library{
		ID: "opds-multi-physical-lib", Name: "opds-multi-physical-lib", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	files := []struct {
		id       string
		relative string
		data     []byte
	}{
		{id: "opds-physical-ch1", relative: "Adaptive Work/Ch.0001.cbz", data: createImageCBZ(t)},
		{id: "opds-physical-v10", relative: "Adaptive Work/Volume 10.cbz", data: createImageCBZ(t)},
		{id: "opds-physical-v2", relative: "Adaptive Work/Volume 2.zip", data: createImageCBZ(t)},
		{id: "opds-physical-v1", relative: "Adaptive Work/Volume 01.pdf", data: []byte("%PDF-1.4\n%%EOF\n")},
	}
	for _, file := range files {
		absolute := filepath.Join(libraryDir, filepath.FromSlash(file.relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, file.data, 0o600); err != nil {
			t.Fatal(err)
		}
		createOPDSHandlerComic(t, file.id, file.relative, "comic", library.ID)
		if _, err := store.DB().Exec(
			`UPDATE "Comic" SET "title" = ?, "pageCount" = 1, "fileSize" = ? WHERE "id" = ?`,
			strings.TrimSuffix(filepath.Base(file.relative), filepath.Ext(file.relative)),
			len(file.data),
			file.id,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: library.ID, CanDownload: true,
	}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}

	works, err := loadOPDSWorksFromResponseFixture(user.ID, library.ID)
	if err != nil || len(works) != 1 || len(works[0].Units) != 4 {
		t.Fatalf("multi-physical fixture works=%d units=%d err=%v", len(works), unitCount(works), err)
	}
	catalog := performOPDSBasicRequest(router, "/api/opds/works", user.Username, token)
	if catalog.Code != http.StatusOK {
		t.Fatalf("multi-physical Work catalog returned %d: %s", catalog.Code, catalog.Body.String())
	}
	catalogBody := catalog.Body.String()
	workHref := "/api/opds/works/" + works[0].ID
	if !strings.Contains(catalogBody, `rel="subsection" href="http://example.com`+workHref+`" type="`+service.OPDSAcquisitionMIME+`"`) {
		t.Fatalf("multi-physical Work must link directly to its acquisition feed: %s", catalogBody)
	}
	path := "/api/opds/works/" + works[0].ID
	response := performOPDSBasicRequest(router, path, user.Username, token)
	if response.Code != http.StatusOK {
		t.Fatalf("multi-physical Work feed returned %d: %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, service.OPDSAcquisitionMIME) {
		t.Fatalf("multi-physical Work feed Content-Type = %q, want acquisition", contentType)
	}
	body := response.Body.String()
	if strings.Count(body, "<entry>") != 4 {
		t.Fatalf("multi-physical Work should expose four physical acquisitions: %s", body)
	}
	if strings.Contains(body, "/continuous/") || !strings.Contains(body, `rel="http://opds-spec.org/acquisition"`) {
		t.Fatalf("multi-physical Work did not expose direct acquisitions: %s", body)
	}
	if strings.Contains(body, `rel="http://vaemendis.net/opds-pse/stream"`) {
		t.Fatalf("physical CBZ entries must not advertise a competing PSE stream: %s", body)
	}
	for _, title := range []string{
		"<title>第1话</title>",
		"<title>Volume 01</title>",
		"<title>Volume 2</title>",
		"<title>Volume 10</title>",
	} {
		if !strings.Contains(body, title) {
			t.Fatalf("multi-physical acquisition display title must hide its file extension; missing %s: %s", title, body)
		}
	}
	expectedUnits := []string{
		"/api/opds/download/opds-physical-ch1/%E7%AC%AC1%E8%AF%9D.cbz",
		"/api/opds/download/opds-physical-v1/Volume%2001.pdf",
		"/api/opds/download/opds-physical-v2/Volume%202.zip",
		"/api/opds/download/opds-physical-v10/Volume%2010.cbz",
	}
	lastPosition := -1
	for _, link := range expectedUnits {
		position := strings.Index(body, link)
		if position < 0 {
			t.Fatalf("multi-physical Work missing %s: %s", link, body)
		}
		if position <= lastPosition {
			t.Fatalf("physical acquisitions are not naturally sorted: %s", body)
		}
		lastPosition = position
	}
	fullIDs := opdsEntryIDs(t, body)
	if len(fullIDs) != 4 {
		t.Fatalf("physical acquisition IDs are not unique: %#v", fullIDs)
	}
	seenIDs := make(map[string]struct{}, len(fullIDs))
	for _, id := range fullIDs {
		if _, exists := seenIDs[id]; exists {
			t.Fatalf("physical acquisition IDs are not unique: %#v", fullIDs)
		}
		seenIDs[id] = struct{}{}
	}
	repeated := performOPDSBasicRequest(router, path, user.Username, token)
	if got := opdsEntryIDs(t, repeated.Body.String()); !equalStrings(got, fullIDs) {
		t.Fatalf("physical acquisition IDs changed between requests: first=%#v second=%#v", fullIDs, got)
	}

	firstPage := performOPDSBasicRequest(router, path+"?page=1&pageSize=2", user.Username, token)
	secondPage := performOPDSBasicRequest(router, path+"?page=2&pageSize=2", user.Username, token)
	pagedIDs := append(opdsEntryIDs(t, firstPage.Body.String()), opdsEntryIDs(t, secondPage.Body.String())...)
	if !equalStrings(pagedIDs, fullIDs) {
		t.Fatalf("pagination changed or duplicated physical Units: full=%#v paged=%#v", fullIDs, pagedIDs)
	}

	unitDetail := performOPDSBasicRequest(
		router,
		"/api/opds/works/"+works[0].ID+"/units/opds-physical-v2",
		user.Username,
		token,
	)
	if unitDetail.Code != http.StatusOK ||
		!strings.Contains(unitDetail.Body.String(), `/api/opds/download/opds-physical-v2/Volume%202.zip`) {
		t.Fatalf("physical Unit detail did not expose the real acquisition: %d %s", unitDetail.Code, unitDetail.Body.String())
	}

	rangeResponse := performOPDSRequest(
		router,
		http.MethodGet,
		"/api/opds/download/opds-physical-v2/Volume%202.zip",
		user.Username,
		token,
		map[string]string{"Range": "bytes=0-9"},
	)
	if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.Len() != 10 {
		t.Fatalf("physical Unit range status=%d length=%d", rangeResponse.Code, rangeResponse.Body.Len())
	}

	localizedDownload := performOPDSBasicRequest(
		router,
		"/api/opds/download/opds-physical-ch1/%E7%AC%AC1%E8%AF%9D.cbz",
		user.Username,
		token,
	)
	if localizedDownload.Code != http.StatusOK {
		t.Fatalf("localized acquisition download returned %d: %s", localizedDownload.Code, localizedDownload.Body.String())
	}
	_, dispositionParams, err := mime.ParseMediaType(localizedDownload.Header().Get("Content-Disposition"))
	if err != nil || dispositionParams["filename"] != "第1话.cbz" {
		t.Fatalf("localized acquisition Content-Disposition = %q params=%#v err=%v", localizedDownload.Header().Get("Content-Disposition"), dispositionParams, err)
	}
}

func TestStripOPDSDisplayExtensionPreservesChapterNumberDots(t *testing.T) {
	tests := map[string]string{
		"Ch.0001 - 第一话.cbz": "Ch.0001 - 第一话",
		"Ch.0002 - 第二话":     "Ch.0002 - 第二话",
		"C.003":             "C.003",
		"第001话.zip":         "第001话",
		"Volume 01.PDF":     "Volume 01",
	}
	for input, expected := range tests {
		if actual := stripOPDSDisplayExtension(input); actual != expected {
			t.Fatalf("stripOPDSDisplayExtension(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestOPDSDisplayTitleLocalizesChapterWithoutShowingExtension(t *testing.T) {
	tests := map[string]string{
		"Ch.0001.cbz":         "第1话",
		"Ch.0002 - 其实是恶魔.cbz": "第2话 其实是恶魔",
		"Chapter 0012.zip":    "第12话",
		"第003话 预告.cbz":        "第3话 预告",
	}
	for filename, expected := range tests {
		title := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		if actual := opdsDisplayTitle(title); actual != expected {
			t.Fatalf("opdsDisplayTitle(%q) = %q, want %q", title, actual, expected)
		}
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

func TestOPDSWorkCoverUsesPersistentLogicalWorkCache(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-logical-cover-user", "opds-logical-cover-user")
	createOPDSHandlerLibrary(t, "opds-logical-cover-lib", "comic", true)
	createOPDSHandlerComic(t, "opds-logical-cover-comic", "作品.cbz", "comic", "opds-logical-cover-lib")
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: "opds-logical-cover-lib", CanDownload: true,
	}}); err != nil {
		t.Fatal(err)
	}
	items, err := loadOPDSWorksFromResponseFixture(user.ID, "opds-logical-cover-lib")
	if err != nil || len(items) != 1 {
		t.Fatalf("load Work: count=%d err=%v", len(items), err)
	}
	work := items[0]
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		ID: work.ID, LibraryID: work.LibraryID, RootPath: work.RootPath,
		Title: work.Title, CoverComicID: work.CoverComicID,
	}}); err != nil {
		t.Fatal(err)
	}
	coverURL := "https://example.test/persistent-cover.jpg"
	locked := true
	if err := store.UpdateLogicalWorkCover(work.ID, store.LogicalWorkCoverUpdate{
		CoverURL: &coverURL, CoverLocked: &locked,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(config.GetThumbnailsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(work.ID))
	coverData := []byte("persistent-work-cover")
	if err := os.WriteFile(cachePath, coverData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cachePath) })

	response := performOPDSBasicRequest(router, "/api/opds/work-cover/"+work.ID, user.Username, token)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), coverData) {
		t.Fatalf("logical Work cover status=%d body=%q", response.Code, response.Body.Bytes())
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
	if err := store.MigrateComicSeriesToLogicalWorks(); err != nil {
		t.Fatalf("MigrateComicSeriesToLogicalWorks failed: %v", err)
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
		chapterDir := filepath.Join(libraryDir, "Folder Work", chapter)
		if err := os.MkdirAll(chapterDir, 0o755); err != nil {
			t.Fatalf("create folder chapter %s: %v", chapter, err)
		}
		if err := os.WriteFile(filepath.Join(chapterDir, "001.png"), createTestPNG(t), 0o600); err != nil {
			t.Fatalf("write folder chapter page %s: %v", chapter, err)
		}
	}
	createOPDSHandlerComic(t, "opds-folder-ch1", "Folder Work/Chapter 001/", "comic", library.ID)
	createOPDSHandlerComic(t, "opds-folder-ch2", "Folder Work/Chapter 002/", "comic", library.ID)
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "pageCount" = 1
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
		t.Fatalf("folder Work should expose only its two Unit entries: %s", body)
	}
	if strings.Contains(body, "/continuous/") {
		t.Fatalf("folder Work exposed removed continuous publication: %s", body)
	}
	for _, comicID := range []string{"opds-folder-ch1", "opds-folder-ch2"} {
		if !strings.Contains(body, "/api/opds/stream/"+comicID+"?") {
			t.Fatalf("folder Unit %s is missing physical comic page stream: %s", comicID, body)
		}
		if strings.Contains(body, "/api/opds/download/"+comicID) {
			t.Fatalf("folder Unit %s must not advertise an unavailable raw-file download: %s", comicID, body)
		}
	}
	for _, unit := range works[0].Units {
		virtualDownload := "/api/opds/units/" + unit.ID + "/download"
		if !strings.Contains(body, virtualDownload) {
			t.Fatalf("folder Unit %s is missing standard virtual CBZ acquisition %s: %s", unit.ID, virtualDownload, body)
		}
	}

	virtualPath := "/api/opds/units/" + works[0].Units[0].ID + "/download"
	virtual := performOPDSBasicRequest(router, virtualPath, user.Username, token)
	if virtual.Code != http.StatusOK {
		t.Fatalf("folder virtual CBZ returned %d: %s", virtual.Code, virtual.Body.String())
	}
	virtualZip, err := zip.NewReader(bytes.NewReader(virtual.Body.Bytes()), int64(virtual.Body.Len()))
	if err != nil || len(virtualZip.File) != 1 {
		t.Fatalf("folder virtual acquisition is not a one-page CBZ: entries=%d err=%v", len(virtualZip.File), err)
	}
	head := performOPDSRequest(router, http.MethodHead, virtualPath, user.Username, token, nil)
	if head.Code != http.StatusOK || head.Header().Get("Content-Type") != virtual.Header().Get("Content-Type") || head.Body.Len() != 0 {
		t.Fatalf("folder virtual HEAD status=%d type=%q body=%q", head.Code, head.Header().Get("Content-Type"), head.Body.String())
	}
	rangeResponse := performOPDSRequest(
		router,
		http.MethodGet,
		virtualPath,
		user.Username,
		token,
		map[string]string{"Range": "bytes=0-9"},
	)
	if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.Len() != 10 {
		t.Fatalf("folder virtual range status=%d length=%d", rangeResponse.Code, rangeResponse.Body.Len())
	}
}

func TestOPDSContinuousRoutesAreRemoved(t *testing.T) {
	router := setupTestRouter(t)
	for _, route := range router.Routes() {
		if strings.Contains(route.Path, "/continuous/") {
			t.Fatalf("removed continuous OPDS route remains registered: %s %s", route.Method, route.Path)
		}
	}
}

func TestVirtualCBZGenerationFailureReturnsServerErrorInsteadOfBrokenSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	router.Use(suppressOPDSHeadResponseBody())
	handler := NewOPDSHandler()
	brokenHandler := func(c *gin.Context) {
		handler.renderVirtualCBZ(c, "Broken", []service.WorkUnit{{
			ID: "broken-unit", ComicID: "missing-comic", PageCount: 1,
		}})
	}
	router.GET("/broken.cbz", brokenHandler)
	router.HEAD("/broken.cbz", brokenHandler)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/broken.cbz", nil)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s broken virtual CBZ status = %d, want 500; body=%q", method, response.Code, response.Body.String())
		}
		if strings.Contains(response.Header().Get("Content-Type"), "comicbook") {
			t.Fatalf("%s broken virtual CBZ was advertised as a valid archive: headers=%v", method, response.Header())
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatalf("broken virtual CBZ HEAD returned body %q", response.Body.String())
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

func createTestPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	page := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			page.Set(x, y, color.NRGBA{R: 40, G: 120, B: 220, A: 255})
		}
	}
	if err := png.Encode(&buffer, page); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func opdsEntryIDs(t *testing.T, feed string) []string {
	t.Helper()
	matches := regexp.MustCompile(`(?s)<entry>.*?<id>([^<]+)</id>`).FindAllStringSubmatch(feed, -1)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		result = append(result, match[1])
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
