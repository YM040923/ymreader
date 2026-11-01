package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestWorkMergeAndSplitAPI(t *testing.T) {
	router := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	cookie := registerAndLogin(t, router)
	createOPDSHandlerLibrary(t, "merge-lib", "comic", true)
	createOPDSHandlerComic(t, "merge-c1", "Series A/01.cbz", "comic", "merge-lib")
	createOPDSHandlerComic(t, "merge-c2", "Series B/01.cbz", "comic", "merge-lib")

	type workList struct {
		Works []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			RootPath string `json:"rootPath"`
			Units    []struct {
				ID string `json:"id"`
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
	unitID := afterMerge.Works[0].Units[0].ID
	mergedTitle := afterMerge.Works[0].Title
	mergedRootPath := afterMerge.Works[0].RootPath

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
		}
	}
	if !found {
		t.Fatalf("split work %s not visible", splitResponse.WorkID)
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
