package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestBatchDeleteExpandedWorkRequiresConfirmation(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	cookie := registerAndLogin(t, router)
	libraryDir := t.TempDir()
	if err := store.CreateLibrary(&model.Library{
		ID: "delete-safety-lib", Name: "delete-safety-lib", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}); err != nil {
		t.Fatalf("create library: %v", err)
	}
	files := map[string][]byte{
		"Series A/01.cbz": []byte("a"),
		"Series B/01.cbz": []byte("b"),
	}
	for relative, content := range files {
		absolute := filepath.Join(libraryDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatalf("create directory: %v", err)
		}
		if err := os.WriteFile(absolute, content, 0o600); err != nil {
			t.Fatalf("create file: %v", err)
		}
	}
	createOPDSHandlerComic(t, "delete-safety-c1", "Series A/01.cbz", "comic", "delete-safety-lib")
	createOPDSHandlerComic(t, "delete-safety-c2", "Series B/01.cbz", "comic", "delete-safety-lib")

	var works struct {
		Works []struct {
			ID string `json:"id"`
		} `json:"works"`
	}
	response := performAuthedRequest(router, http.MethodGet, "/api/works?pageSize=100", nil, cookie)
	if err := json.Unmarshal(response.Body.Bytes(), &works); err != nil || len(works.Works) != 2 {
		t.Fatalf("load works status=%d body=%s err=%v", response.Code, response.Body.String(), err)
	}
	merge := performAuthedRequest(router, http.MethodPost, "/api/works/merge", map[string]interface{}{
		"workIds": []string{works.Works[0].ID, works.Works[1].ID},
	}, cookie)
	if merge.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", merge.Code, merge.Body.String())
	}

	deleteBody := map[string]interface{}{
		"action":      "delete",
		"comicIds":    []string{"delete-safety-c1", "delete-safety-c2"},
		"deleteFiles": true,
	}
	blocked := performAuthedRequest(router, http.MethodPost, "/api/comics/batch", deleteBody, cookie)
	if blocked.Code != http.StatusConflict {
		t.Fatalf("expanded delete status=%d body=%s, want 409", blocked.Code, blocked.Body.String())
	}
	for relative := range files {
		if _, err := os.Stat(filepath.Join(libraryDir, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("file was removed before confirmation: %s: %v", relative, err)
		}
	}

	deleteBody["confirmExpandedDelete"] = true
	confirmed := performAuthedRequest(router, http.MethodPost, "/api/comics/batch", deleteBody, cookie)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirmed delete status=%d body=%s", confirmed.Code, confirmed.Body.String())
	}
	for relative := range files {
		if _, err := os.Stat(filepath.Join(libraryDir, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Fatalf("confirmed delete left file %s: %v", relative, err)
		}
	}
	if comic, err := store.GetComicByID("delete-safety-c1"); err != nil || comic != nil {
		t.Fatalf("comic record was not deleted: %#v err=%v", comic, err)
	}
}

func TestWorkMergeAndSplitAPI(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	cookie := registerAndLogin(t, router)
	createOPDSHandlerLibrary(t, "merge-lib", "comic", true)
	createOPDSHandlerComic(t, "merge-c1", "Series A/01.cbz", "comic", "merge-lib")
	createOPDSHandlerComic(t, "merge-c2", "Series B/01.cbz", "comic", "merge-lib")
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "pageCount" = 3 WHERE "id" IN ('merge-c1', 'merge-c2')`); err != nil {
		t.Fatalf("set page counts: %v", err)
	}
	var userID string
	if err := store.DB().QueryRow(`SELECT "id" FROM "User" WHERE "username" = 'admin'`).Scan(&userID); err != nil {
		t.Fatalf("resolve admin user: %v", err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "UserComicState" ("userId", "comicId", "lastReadPage", "lastReadAt", "isFavorite", "totalReadTime", "readingStatus")
		VALUES (?, 'merge-c1', 2, '2026-11-01T10:00:00Z', 1, 120, 'finished')
	`, userID); err != nil {
		t.Fatalf("seed reading state: %v", err)
	}

	type workList struct {
		Works []struct {
			ID                 string `json:"id"`
			Title              string `json:"title"`
			RootPath           string `json:"rootPath"`
			IsFavorite         bool   `json:"isFavorite"`
			CompletedItemCount int    `json:"completedItemCount"`
			ReadingStatus      string `json:"readingStatus"`
			TotalReadTime      int    `json:"totalReadTime"`
			Units              []struct {
				ID        string `json:"id"`
				ComicID   string `json:"comicId"`
				StartPage int    `json:"startPage"`
				PageCount int    `json:"pageCount"`
			} `json:"units"`
		} `json:"works"`
	}
	loadWorks := func() workList {
		response := performAuthedRequest(router, http.MethodGet, "/api/works?pageSize=100", nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("list works status=%d body=%s", response.Code, response.Body.String())
		}
		var result workList
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode works: %v", err)
		}
		return result
	}
	initial := loadWorks()
	if len(initial.Works) != 2 {
		t.Fatalf("initial works = %d, want 2", len(initial.Works))
	}
	seriesA := initial.Works[0].ID
	seriesB := initial.Works[1].ID

	merge := performAuthedRequest(router, http.MethodPost, "/api/works/merge", map[string]interface{}{
		"workIds": []string{seriesA, seriesB},
	}, cookie)
	if merge.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", merge.Code, merge.Body.String())
	}
	var merged struct {
		WorkID string `json:"workId"`
	}
	if err := json.Unmarshal(merge.Body.Bytes(), &merged); err != nil || merged.WorkID == "" {
		t.Fatalf("merge response=%s err=%v", merge.Body.String(), err)
	}
	afterMerge := loadWorks()
	if len(afterMerge.Works) != 1 {
		t.Fatalf("works after merge = %d, want 1: %#v", len(afterMerge.Works), afterMerge.Works)
	}
	if len(afterMerge.Works[0].Units) != 2 {
		t.Fatalf("merged units = %d, want 2", len(afterMerge.Works[0].Units))
	}
	if !afterMerge.Works[0].IsFavorite || afterMerge.Works[0].CompletedItemCount != 1 ||
		afterMerge.Works[0].TotalReadTime != 120 || afterMerge.Works[0].ReadingStatus != "reading" {
		t.Fatalf("merged work lost source user state: %#v", afterMerge.Works[0])
	}
	unitID := ""
	allUnitIDs := make([]string, 0, len(afterMerge.Works[0].Units))
	for _, unit := range afterMerge.Works[0].Units {
		allUnitIDs = append(allUnitIDs, unit.ID)
		if unit.ComicID == "merge-c1" {
			unitID = unit.ID
		}
	}
	if unitID == "" {
		t.Fatal("merged Work is missing the read comic")
	}
	blockedSplit := performAuthedRequest(router, http.MethodPost, "/api/works/"+merged.WorkID+"/split", map[string]interface{}{
		"unitIds": allUnitIDs,
	}, cookie)
	if blockedSplit.Code != http.StatusBadRequest {
		t.Fatalf("all-unit split status=%d body=%s, want 400", blockedSplit.Code, blockedSplit.Body.String())
	}
	mergedTitle := afterMerge.Works[0].Title
	mergedRootPath := afterMerge.Works[0].RootPath
	progressUnit := afterMerge.Works[0].Units[1]
	activity := performAuthedRequest(router, http.MethodPost, "/api/reading/"+progressUnit.ComicID+"/activity", map[string]interface{}{
		"clientSessionId": "merged-work-session",
		"page":            1,
		"totalPages":      progressUnit.PageCount,
		"activeSeconds":   2,
		"sequence":        1,
		"workId":          merged.WorkID,
		"unitId":          progressUnit.ID,
		"relativePage":    1,
	}, cookie)
	if activity.Code != http.StatusOK {
		t.Fatalf("merged Work progress status=%d body=%s", activity.Code, activity.Body.String())
	}

	split := performAuthedRequest(router, http.MethodPost, "/api/works/"+merged.WorkID+"/split", map[string]interface{}{
		"unitIds": []string{unitID},
		"title":   "Series A 拆分",
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
	afterSplit := loadWorks()
	if len(afterSplit.Works) != 2 {
		t.Fatalf("works after split = %d, want 2: %#v", len(afterSplit.Works), afterSplit.Works)
	}
	found := false
	for _, work := range afterSplit.Works {
		if work.ID == splitResponse.WorkID {
			found = true
			if work.Title != "Series A 拆分" {
				t.Fatalf("split work title = %q", work.Title)
			}
			if work.RootPath != mergedRootPath || strings.Contains(work.RootPath, "\x1f") {
				t.Fatalf("split work rootPath = %q, want %q", work.RootPath, mergedRootPath)
			}
			if !work.IsFavorite || work.CompletedItemCount != 1 || work.TotalReadTime != 120 ||
				work.ReadingStatus != "finished" {
				t.Fatalf("split work lost source user state: %#v", work)
			}
		}
	}
	if !found {
		t.Fatalf("split work %s not visible", splitResponse.WorkID)
	}

	var splitUnit struct {
		ID        string `json:"id"`
		ComicID   string `json:"comicId"`
		StartPage int    `json:"startPage"`
		PageCount int    `json:"pageCount"`
	}
	for _, work := range afterSplit.Works {
		if work.ID == splitResponse.WorkID && len(work.Units) > 0 {
			splitUnit = work.Units[0]
		}
	}
	splitActivity := performAuthedRequest(router, http.MethodPost, "/api/reading/"+splitUnit.ComicID+"/activity", map[string]interface{}{
		"clientSessionId": "split-work-session",
		"page":            2,
		"totalPages":      splitUnit.PageCount,
		"activeSeconds":   2,
		"sequence":        1,
		"workId":          splitResponse.WorkID,
		"unitId":          splitUnit.ID,
		"relativePage":    2,
	}, cookie)
	if splitActivity.Code != http.StatusOK {
		t.Fatalf("split Work progress status=%d body=%s", splitActivity.Code, splitActivity.Body.String())
	}

	// A later write must persist only the manual Work and leave the automatic
	// Work that shares its directory untouched.
	update := performAuthedRequest(router, http.MethodPut, "/api/works/"+splitResponse.WorkID+"/metadata", map[string]interface{}{
		"title": "Series A 拆分改名",
	}, cookie)
	if update.Code != http.StatusOK {
		t.Fatalf("split metadata update status=%d body=%s", update.Code, update.Body.String())
	}
	afterUpdate := loadWorks()
	if len(afterUpdate.Works) != 2 {
		t.Fatalf("works after metadata update = %d, want 2", len(afterUpdate.Works))
	}
	for _, work := range afterUpdate.Works {
		switch work.ID {
		case splitResponse.WorkID:
			if work.Title != "Series A 拆分改名" {
				t.Fatalf("split work title after update = %q", work.Title)
			}
		case merged.WorkID:
			if work.Title != mergedTitle {
				t.Fatalf("automatic work title was overwritten: %q, want %q", work.Title, mergedTitle)
			}
		}
	}
}
