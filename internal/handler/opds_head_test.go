package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestOPDSHeadMatchesGetWithoutResponseBody(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	user, token := createOPDSTestUserAndKey(t, "opds-head-user", "opds-head-user")

	libraryDir := t.TempDir()
	library := &model.Library{
		ID:            "opds-head-lib",
		Name:          "opds-head-lib",
		Type:          "comic",
		RootPath:      libraryDir,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	const comicID = "opds-head-comic"
	const filename = "OPDS Head Comic.cbz"
	archivePath := filepath.Join(libraryDir, filename)
	if err := os.WriteFile(archivePath, createImageCBZ(t), 0o600); err != nil {
		t.Fatalf("write OPDS HEAD CBZ failed: %v", err)
	}
	t.Cleanup(func() {
		service.InvalidateContentFileCaches(comicID, archivePath)
	})
	createOPDSHandlerComic(t, comicID, filename, "comic", library.ID)
	if _, err := store.DB().Exec(
		`UPDATE "Comic" SET "title" = 'OPDS Head Comic', "pageCount" = 1 WHERE "id" = ?`,
		comicID,
	); err != nil {
		t.Fatalf("update OPDS HEAD comic failed: %v", err)
	}
	if err := store.SetUserLibraryAccess(user.ID, []store.LibraryAccessReq{{
		LibraryID: library.ID, CanDownload: true,
	}}); err != nil {
		t.Fatalf("SetUserLibraryAccess failed: %v", err)
	}

	works, err := loadOPDSWorksFromResponseFixture(user.ID, library.ID)
	if err != nil || len(works) != 1 {
		t.Fatalf("build OPDS HEAD Work fixture: %v, count=%d", err, len(works))
	}

	paths := []string{
		"/api/opds",
		"/api/opds/all",
		"/api/opds/recent",
		"/api/opds/favorites",
		"/api/opds/works",
		"/api/opds/works/" + works[0].ID,
		"/api/opds/search.xml",
		"/api/opds/search?q=OPDS",
		"/api/opds/cover/" + comicID,
		"/api/opds/work-cover/" + works[0].ID,
		"/api/opds/unit-cover/" + comicID + "?page=0",
		"/api/opds/stream/" + comicID + "?page=0",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			getResponse := performOPDSRequest(router, http.MethodGet, path, user.Username, token, nil)
			headResponse := performOPDSRequest(router, http.MethodHead, path, user.Username, token, nil)

			if headResponse.Code != getResponse.Code {
				t.Fatalf("HEAD status = %d, GET status = %d; HEAD body=%q", headResponse.Code, getResponse.Code, headResponse.Body.String())
			}
			if got, want := headResponse.Header().Get("Content-Type"), getResponse.Header().Get("Content-Type"); got != want {
				t.Fatalf("HEAD Content-Type = %q, GET Content-Type = %q", got, want)
			}
			if headResponse.Body.Len() != 0 {
				t.Fatalf("HEAD returned %d body bytes: %q", headResponse.Body.Len(), headResponse.Body.String())
			}
		})
	}
}
