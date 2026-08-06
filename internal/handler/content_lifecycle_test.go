package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestRecordOnlyDeleteDoesNotReturnAfterRescanInsert(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)

	root := t.TempDir()
	lib := &model.Library{
		ID:            "delete-guard-library",
		Name:          "Delete Guard Library",
		Type:          "novel",
		RootPath:      root,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(lib); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	const relativePath = "Robinson Crusoe.epub"
	if _, err := store.DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES ('delete-guard-book', ?, 'Robinson Crusoe', 'novel', ?, ?)
	`, relativePath, lib.ID, relativePath); err != nil {
		t.Fatalf("insert comic: %v", err)
	}

	w := performAuthedRequest(r, http.MethodDelete, "/api/comics/delete-guard-book", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("record-only delete: %d %s", w.Code, w.Body.String())
	}

	// Simulate the automatic scanner trying to restore the still-existing file.
	// The delete guard's tombstone trigger must silently ignore the insert.
	if _, err := store.DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES ('delete-guard-rescan', ?, 'Robinson Crusoe', 'novel', ?, ?)
	`, relativePath, lib.ID, relativePath); err != nil {
		t.Fatalf("scanner-style reinsert returned error: %v", err)
	}

	var count int
	if err := store.DB().QueryRow(`
		SELECT COUNT(*) FROM "Comic" WHERE "libraryId" = ? AND "relativePath" = ?
	`, lib.ID, relativePath).Scan(&count); err != nil {
		t.Fatalf("count comics: %v", err)
	}
	if count != 0 {
		t.Fatalf("record-only deleted book returned after rescan: count=%d", count)
	}
}

func TestShelfListAutomaticallyMergesParentChildDuplicate(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)

	root := t.TempDir()
	novelRoot := filepath.Join(root, "novels")
	if err := os.MkdirAll(novelRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(novelRoot, "Robinson Crusoe.epub"), []byte("epub"), 0644); err != nil {
		t.Fatal(err)
	}

	parent := &model.Library{
		ID:            "default-parent",
		Name:          "Default Library",
		Type:          "comic",
		RootPath:      root,
		Enabled:       true,
		DefaultAccess: "public",
		ScanEnabled:   true,
	}
	child := &model.Library{
		ID:            "novel-child",
		Name:          "小说",
		Type:          "novel",
		RootPath:      novelRoot,
		Enabled:       true,
		DefaultAccess: "public",
		ScanEnabled:   true,
	}
	for _, lib := range []*model.Library{parent, child} {
		if err := store.CreateLibrary(lib); err != nil {
			t.Fatalf("CreateLibrary(%s): %v", lib.ID, err)
		}
	}

	now := time.Now().UTC()
	rows := []struct {
		id           string
		filename     string
		comicType    string
		libraryID    string
		relativePath string
	}{
		{store.PathToID(parent.ID, "novels/Robinson Crusoe.epub"), "novels/Robinson Crusoe.epub", "comic", parent.ID, "novels/Robinson Crusoe.epub"},
		{store.PathToID(child.ID, "Robinson Crusoe.epub"), "Robinson Crusoe.epub", "novel", child.ID, "Robinson Crusoe.epub"},
	}
	for _, row := range rows {
		if _, err := store.DB().Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "pageCount", "fileSize", "type", "libraryId", "relativePath", "addedAt", "updatedAt")
			VALUES (?, ?, '鲁滨逊漂流记', 1, 4, ?, ?, ?, ?, ?)
		`, row.id, row.filename, row.comicType, row.libraryID, row.relativePath, now, now); err != nil {
			t.Fatalf("insert duplicate row: %v", err)
		}
	}

	ownershipReconcileMu.Lock()
	ownershipReconcileLastAttempt = time.Time{}
	ownershipInitialCheckDone = false
	ownershipReconcileMu.Unlock()

	w := performAuthedRequest(r, http.MethodGet, "/api/libraries/accessible", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list accessible libraries: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Libraries []struct {
			ID         string `json:"id"`
			ComicCount int    `json:"comicCount"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode libraries: %v", err)
	}

	var count int
	var gotID, gotLibrary, gotPath, gotType string
	if err := store.DB().QueryRow(`
		SELECT COUNT(*), "id", "libraryId", "relativePath", "type" FROM "Comic"
	`).Scan(&count, &gotID, &gotLibrary, &gotPath, &gotType); err != nil {
		t.Fatalf("query canonical comic: %v", err)
	}
	wantID := store.PathToID(child.ID, "Robinson Crusoe.epub")
	if count != 1 || gotID != wantID || gotLibrary != child.ID || gotPath != "Robinson Crusoe.epub" || gotType != "novel" {
		t.Fatalf("unexpected canonical row: count=%d id=%s library=%s path=%s type=%s", count, gotID, gotLibrary, gotPath, gotType)
	}

	counts := map[string]int{}
	for _, library := range response.Libraries {
		counts[library.ID] = library.ComicCount
	}
	if counts[parent.ID] != 0 || counts[child.ID] != 1 {
		t.Fatalf("library counts not reconciled: parent=%d child=%d", counts[parent.ID], counts[child.ID])
	}
}
