package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type progressRow struct {
	WorkID       string
	ComicID      string
	RelativePage int
}

func readProgressRows(t *testing.T) []progressRow {
	t.Helper()
	rows, err := store.DB().Query(`
		SELECT "workId", "comicId", "relativePage"
		FROM "UserWorkProgress"
		ORDER BY "workId"
	`)
	if err != nil {
		t.Fatalf("read cursors: %v", err)
	}
	defer rows.Close()
	result := make([]progressRow, 0, 4)
	for rows.Next() {
		var entry progressRow
		if err := rows.Scan(&entry.WorkID, &entry.ComicID, &entry.RelativePage); err != nil {
			t.Fatalf("scan cursor: %v", err)
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate cursors: %v", err)
	}
	return result
}

func mustSingleRow(t *testing.T, rows []progressRow) progressRow {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("cursors = %#v, want exactly one", rows)
	}
	return rows[0]
}

// A merge must not strand the reading position on the Work that left the
// catalogue, and undo must hand it back.
func TestWorkMergeSplitRemapsReadingCursors(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	cookie := registerAndLogin(t, router)
	libraryDir := t.TempDir()
	if err := store.CreateLibrary(&model.Library{
		ID: "cursor-lib", Name: "cursor-lib", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}); err != nil {
		t.Fatalf("create library: %v", err)
	}
	createOPDSHandlerComic(t, "cursor-c1", "Series A/01.cbz", "comic", "cursor-lib")
	createOPDSHandlerComic(t, "cursor-c2", "Series B/01.cbz", "comic", "cursor-lib")
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "pageCount" = 3 WHERE "id" IN ('cursor-c1', 'cursor-c2')`); err != nil {
		t.Fatalf("set page counts: %v", err)
	}
	var userID string
	if err := store.DB().QueryRow(`SELECT "id" FROM "User" WHERE "username" = 'admin'`).Scan(&userID); err != nil {
		t.Fatalf("resolve admin user: %v", err)
	}

	type workEntry struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		RootPath string `json:"rootPath"`
		Units    []struct {
			ID      string `json:"id"`
			ComicID string `json:"comicId"`
		} `json:"units"`
	}
	var initial struct {
		Works []workEntry `json:"works"`
	}
	response := performAuthedRequest(router, http.MethodGet, "/api/works?pageSize=100", nil, cookie)
	if err := json.Unmarshal(response.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode works: %v body=%s", err, response.Body.String())
	}
	if len(initial.Works) != 2 {
		t.Fatalf("initial works = %d, want 2", len(initial.Works))
	}
	// Directory Works only receive a LogicalWork row from the scanner; the
	// reader cursor has a foreign key on it.
	for _, work := range initial.Works {
		if err := store.EnsureLogicalWorkRecord(store.LogicalWork{
			ID: work.ID, LibraryID: "cursor-lib", RootPath: work.RootPath, Title: work.Title,
		}); err != nil {
			t.Fatalf("ensure LogicalWork for %s: %v", work.ID, err)
		}
	}
	survivorID, absorbedID, absorbedUnit := "", "", ""
	for _, work := range initial.Works {
		switch work.Title {
		case "cursor-c1":
			survivorID = work.ID
		case "cursor-c2":
			absorbedID = work.ID
			absorbedUnit = work.Units[0].ID
		}
	}
	if survivorID == "" || absorbedID == "" || absorbedUnit == "" {
		t.Fatalf("unresolved works: survivor=%q absorbed=%q unit=%q", survivorID, absorbedID, absorbedUnit)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "UserWorkProgress" (
			"userId", "workId", "unitId", "comicId", "relativePage",
			"absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		) VALUES (?, ?, ?, 'cursor-c2', 2, 2, '2026-11-01T10:00:00Z', 'cursor-session', 1)
	`, userID, absorbedID, absorbedUnit); err != nil {
		t.Fatalf("seed Work cursor: %v", err)
	}

	merge := performAuthedRequest(router, http.MethodPost, "/api/works/merge", map[string]interface{}{
		"workIds": []string{survivorID, absorbedID},
	}, cookie)
	if merge.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", merge.Code, merge.Body.String())
	}
	moved := mustSingleRow(t, readProgressRows(t))
	if moved.WorkID != survivorID || moved.ComicID != "cursor-c2" || moved.RelativePage != 2 {
		t.Fatalf("cursor after merge = %#v, want it on the surviving Work", moved)
	}

	// 继续阅读 sorts by lastReadAt, so the surviving Work has to carry it.
	var rail struct {
		Works []struct {
			ID       string `json:"id"`
			LastRead string `json:"lastReadAt"`
			Units    []struct {
				ID       string `json:"id"`
				LastRead string `json:"lastReadAt"`
			} `json:"units"`
		} `json:"works"`
	}
	railResponse := performAuthedRequest(router, http.MethodGet,
		"/api/works?sortBy=lastReadAt&sortOrder=desc&pageSize=12", nil, cookie)
	if err := json.Unmarshal(railResponse.Body.Bytes(), &rail); err != nil {
		t.Fatalf("decode rail: %v body=%s", err, railResponse.Body.String())
	}
	if len(rail.Works) != 1 || rail.Works[0].ID != survivorID || rail.Works[0].LastRead == "" {
		t.Fatalf("rail after merge = %#v, want the surviving Work with a read time", rail.Works)
	}
	unitCarriesCursor := false
	for _, unit := range rail.Works[0].Units {
		if unit.ID == absorbedUnit && unit.LastRead != "" {
			unitCarriesCursor = true
		}
	}
	if !unitCarriesCursor {
		t.Fatalf("rail units lost the reading position: %#v", rail.Works[0].Units)
	}

	undo := performAuthedRequest(router, http.MethodPost, "/api/works/"+survivorID+"/undo-last", nil, cookie)
	if undo.Code != http.StatusOK {
		t.Fatalf("undo status=%d body=%s", undo.Code, undo.Body.String())
	}
	back := mustSingleRow(t, readProgressRows(t))
	if back.WorkID != absorbedID || back.ComicID != "cursor-c2" || back.RelativePage != 2 {
		t.Fatalf("cursor after undo = %#v, want it back on the absorbed Work", back)
	}

	// Splitting an archive out must take its cursor with it, and undo must
	// return the cursor to the Work it was split from.
	if _, err := store.DB().Exec(`DELETE FROM "UserWorkProgress"`); err != nil {
		t.Fatalf("reset cursors: %v", err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "UserWorkProgress" (
			"userId", "workId", "unitId", "comicId", "relativePage",
			"absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		) VALUES (?, ?, ?, 'cursor-c2', 2, 2, '2026-11-01T10:00:00Z', 'cursor-session', 1)
	`, userID, survivorID, absorbedUnit); err != nil {
		t.Fatalf("seed cursor on survivor: %v", err)
	}
	reMerge := performAuthedRequest(router, http.MethodPost, "/api/works/merge", map[string]interface{}{
		"workIds": []string{survivorID, absorbedID},
	}, cookie)
	if reMerge.Code != http.StatusOK {
		t.Fatalf("re-merge status=%d body=%s", reMerge.Code, reMerge.Body.String())
	}
	split := performAuthedRequest(router, http.MethodPost, "/api/works/"+survivorID+"/split", map[string]interface{}{
		"unitIds": []string{absorbedUnit},
		"title":   "拆出去",
	}, cookie)
	if split.Code != http.StatusOK {
		t.Fatalf("split status=%d body=%s", split.Code, split.Body.String())
	}
	var splitResponse struct {
		WorkID string `json:"workId"`
	}
	if err := json.Unmarshal(split.Body.Bytes(), &splitResponse); err != nil || splitResponse.WorkID == "" {
		t.Fatalf("split response=%s err=%v", split.Body.String(), err)
	}
	splitRow := mustSingleRow(t, readProgressRows(t))
	if splitRow.WorkID != splitResponse.WorkID || splitRow.RelativePage != 2 {
		t.Fatalf("cursor after split = %#v, want it on %s", splitRow, splitResponse.WorkID)
	}

	undoSplit := performAuthedRequest(router, http.MethodPost, "/api/works/"+survivorID+"/undo-last", nil, cookie)
	if undoSplit.Code != http.StatusOK {
		t.Fatalf("undo split status=%d body=%s", undoSplit.Code, undoSplit.Body.String())
	}
	restored := mustSingleRow(t, readProgressRows(t))
	if restored.WorkID != survivorID || restored.RelativePage != 2 {
		t.Fatalf("cursor after undo split = %#v, want it back on %s", restored, survivorID)
	}
	var remaining int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM "UserWorkProgress" WHERE "workId" = ?`, splitResponse.WorkID).
		Scan(&remaining); err != nil && err != sql.ErrNoRows {
		t.Fatalf("count split cursors: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("split Work kept %d cursor(s) after undo", remaining)
	}
}
