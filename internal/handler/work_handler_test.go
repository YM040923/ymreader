package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func TestWorkAPIDetailProgressContinueAndAdjacent(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)
	createWorkAPIFixture(t)

	list := performAuthedRequest(r, http.MethodGet, "/api/works", nil, cookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list works: %d %s", list.Code, list.Body.String())
	}
	var listResp struct {
		Works []model.Work `json:"works"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if listResp.Total != 1 || len(listResp.Works) != 1 || listResp.Works[0].Title != "Da Wang Rao Ming" {
		t.Fatalf("list response = %#v", listResp)
	}

	detail := performAuthedRequest(r, http.MethodGet, "/api/works/work-api-1", nil, cookie)
	if detail.Code != http.StatusOK {
		t.Fatalf("work detail: %d %s", detail.Code, detail.Body.String())
	}
	var detailResp struct {
		Work  model.Work       `json:"work"`
		Units []model.WorkUnit `json:"units"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &detailResp); err != nil {
		t.Fatal(err)
	}
	if detailResp.Work.ID != "work-api-1" || len(detailResp.Units) != 2 {
		t.Fatalf("detail response = %#v", detailResp)
	}

	initial := performAuthedRequest(r, http.MethodGet, "/api/works/work-api-1/continue", nil, cookie)
	if initial.Code != http.StatusOK {
		t.Fatalf("initial continue: %d %s", initial.Code, initial.Body.String())
	}
	assertContinueTarget(t, initial, "unit-api-1", 0)

	progress := performAuthedRequest(r, http.MethodPut, "/api/works/work-api-1/progress", map[string]interface{}{
		"unitId":    "unit-api-2",
		"pageIndex": 7,
	}, cookie)
	if progress.Code != http.StatusOK {
		t.Fatalf("update progress: %d %s", progress.Code, progress.Body.String())
	}

	resumed := performAuthedRequest(r, http.MethodGet, "/api/works/work-api-1/continue", nil, cookie)
	if resumed.Code != http.StatusOK {
		t.Fatalf("resumed continue: %d %s", resumed.Code, resumed.Body.String())
	}
	assertContinueTarget(t, resumed, "unit-api-2", 7)

	adjacent := performAuthedRequest(r, http.MethodGet, "/api/works/work-api-1/units/unit-api-1/adjacent", nil, cookie)
	if adjacent.Code != http.StatusOK {
		t.Fatalf("adjacent: %d %s", adjacent.Code, adjacent.Body.String())
	}
	var adjacentResp struct {
		Previous *model.WorkUnit `json:"previous"`
		Next     *model.WorkUnit `json:"next"`
	}
	if err := json.Unmarshal(adjacent.Body.Bytes(), &adjacentResp); err != nil {
		t.Fatal(err)
	}
	if adjacentResp.Previous != nil || adjacentResp.Next == nil || adjacentResp.Next.ID != "unit-api-2" {
		t.Fatalf("adjacent response = %#v", adjacentResp)
	}
}

func TestWorkAPIRejectsProgressForForeignUnit(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)
	createWorkAPIFixture(t)
	if err := store.ReplaceWorksForLibrary("work-api-lib", append(workAPIDetectedWork(), workmodel.DetectedWork{
		ID:               "work-api-other",
		LibraryID:        "work-api-lib",
		RootRelativePath: "Other",
		Title:            "Other",
		SortTitle:        "other",
		Units: []workmodel.DetectedUnit{{
			ID:           "unit-api-other",
			ComicID:      "comic-api-other",
			RelativePath: "Other/Ch.001.cbz",
			Kind:         workmodel.UnitKindChapter,
			Title:        "Ch.001",
			DisplayLabel: "Ch.001",
			SortIndex:    0,
			PageCount:    10,
			FileSize:     1000,
		}},
	})); err != nil {
		t.Fatal(err)
	}

	w := performAuthedRequest(r, http.MethodPut, "/api/works/work-api-1/progress", map[string]interface{}{
		"unitId":    "unit-api-other",
		"pageIndex": 3,
	}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("foreign unit progress status = %d, body = %s", w.Code, w.Body.String())
	}
}

func createWorkAPIFixture(t *testing.T) {
	t.Helper()
	lib := &model.Library{
		ID:            "work-api-lib",
		Name:          "Work API Lib",
		Type:          "comic",
		RootPath:      t.TempDir(),
		Enabled:       true,
		DefaultAccess: "public",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('comic-api-1', 'Da Wang Rao Ming/Ch.001.cbz', 'Ch.001', 'comic', 'work-api-lib', 'Da Wang Rao Ming/Ch.001.cbz', 10, 1000),
			('comic-api-2', 'Da Wang Rao Ming/Ch.002.cbz', 'Ch.002', 'comic', 'work-api-lib', 'Da Wang Rao Ming/Ch.002.cbz', 12, 1200),
			('comic-api-other', 'Other/Ch.001.cbz', 'Ch.001', 'comic', 'work-api-lib', 'Other/Ch.001.cbz', 10, 1000)
	`); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceWorksForLibrary(lib.ID, workAPIDetectedWork()); err != nil {
		t.Fatal(err)
	}
}

func workAPIDetectedWork() []workmodel.DetectedWork {
	return []workmodel.DetectedWork{{
		ID:               "work-api-1",
		LibraryID:        "work-api-lib",
		RootRelativePath: "Da Wang Rao Ming",
		Title:            "Da Wang Rao Ming",
		SortTitle:        "da wang rao ming",
		Units: []workmodel.DetectedUnit{
			{
				ID:           "unit-api-1",
				ComicID:      "comic-api-1",
				RelativePath: "Da Wang Rao Ming/Ch.001.cbz",
				Kind:         workmodel.UnitKindChapter,
				Title:        "Ch.001",
				DisplayLabel: "Ch.001",
				SortIndex:    0,
				PageCount:    10,
				FileSize:     1000,
			},
			{
				ID:           "unit-api-2",
				ComicID:      "comic-api-2",
				RelativePath: "Da Wang Rao Ming/Ch.002.cbz",
				Kind:         workmodel.UnitKindChapter,
				Title:        "Ch.002",
				DisplayLabel: "Ch.002",
				SortIndex:    1,
				PageCount:    12,
				FileSize:     1200,
			},
		},
	}}
}

func assertContinueTarget(t *testing.T, w *httptest.ResponseRecorder, wantUnit string, wantPage int) {
	t.Helper()
	var got struct {
		WorkID    string `json:"workId"`
		UnitID    string `json:"unitId"`
		ComicID   string `json:"comicId"`
		PageIndex int    `json:"pageIndex"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.WorkID != "work-api-1" || got.UnitID != wantUnit || got.PageIndex != wantPage {
		t.Fatalf("continue target = %#v, want unit=%s page=%d", got, wantUnit, wantPage)
	}
}
