package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestWorkWriteAPIAppliesSeriesHostAndEveryPhysicalComic(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "write-series-lib", "Write Series", "private")
	createWorkTestComics(t, "write-series-lib", []workTestComic{
		{ID: "write-series-1", Path: "作品/第一话.cbz", Title: "第一话"},
		{ID: "write-series-2", Path: "作品/第二话.cbz", Title: "第二话"},
	})
	if err := service.RebuildComicSeriesForLibrary("write-series-lib"); err != nil {
		t.Fatal(err)
	}
	work := fetchSingleWorkForTest(t, r, cookie, "")

	requests := []struct {
		method string
		path   string
		body   interface{}
	}{
		{http.MethodPut, "/api/works/" + work.ID + "/favorite", map[string]interface{}{"isFavorite": true}},
		{http.MethodPut, "/api/works/" + work.ID + "/reading-status", map[string]interface{}{"status": "finished"}},
		{http.MethodPut, "/api/works/" + work.ID + "/tags", map[string]interface{}{"tags": []string{"热血"}}},
		{http.MethodPut, "/api/works/" + work.ID + "/categories", map[string]interface{}{"categorySlugs": []string{"action"}}},
		{http.MethodPut, "/api/works/" + work.ID + "/metadata", map[string]interface{}{"title": "手动作品名", "author": "作者", "description": "简介"}},
		{http.MethodPut, "/api/works/" + work.ID + "/cover", map[string]interface{}{"url": "https://example.test/cover.jpg", "coverAspectRatio": 0.68}},
	}
	for _, request := range requests {
		response := performAuthedRequest(r, request.method, request.path, request.body, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, response.Code, response.Body.String())
		}
	}

	updated := fetchSingleWorkForTest(t, r, cookie, "")
	if !updated.IsFavorite || updated.ReadingStatus != "finished" || updated.Title != "手动作品名" ||
		updated.Author != "作者" || updated.CoverAspectRatio != 0.68 ||
		len(updated.Tags) != 1 || updated.Tags[0].Name != "热血" ||
		len(updated.Categories) != 1 || updated.Categories[0].Slug != "action" {
		t.Fatalf("updated work=%#v", updated)
	}
	for _, comicID := range []string{"write-series-1", "write-series-2"} {
		comic, err := store.GetComicByIDForUser(comicID, storeTestAdminUserID(t))
		if err != nil || comic == nil || !comic.IsFavorite || comic.ReadingStatus != "finished" ||
			len(comic.Tags) != 1 || len(comic.Categories) != 1 {
			t.Fatalf("comic %s=%#v err=%v", comicID, comic, err)
		}
	}
}

func TestWorkCoverSwitchFromRemoteURLToComicClearsRemoteURL(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "cover-switch-lib", "Cover Switch", "private")
	createWorkTestComics(t, "cover-switch-lib", []workTestComic{
		{ID: "cover-switch-1", Path: "作品/第一话.cbz", Title: "第一话"},
		{ID: "cover-switch-2", Path: "作品/第二话.cbz", Title: "第二话"},
	})
	work := fetchSingleWorkForTest(t, r, cookie, "")

	response := performAuthedRequest(r, http.MethodPut, "/api/works/"+work.ID+"/cover",
		map[string]interface{}{"url": "https://example.test/remote.jpg"}, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("set remote cover status=%d body=%s", response.Code, response.Body.String())
	}
	response = performAuthedRequest(r, http.MethodPut, "/api/works/"+work.ID+"/cover",
		map[string]interface{}{"coverComicId": "cover-switch-2"}, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("switch cover status=%d body=%s", response.Code, response.Body.String())
	}

	logical, err := store.GetLogicalWork(work.ID)
	if err != nil {
		t.Fatal(err)
	}
	if logical.CoverComicID != "cover-switch-2" || logical.CoverURL != "" || logical.CoverSource != "comic" {
		t.Fatalf("cover source was not switched cleanly: %#v", logical)
	}
}

func TestWorkWriteAPISupportsComicMetadataHostAndImmediateCustomOrder(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "write-single-lib", "Singles", "private")
	createWorkTestComics(t, "write-single-lib", []workTestComic{
		{ID: "single-a", Path: "甲.zip", Title: "甲"},
		{ID: "single-b", Path: "乙.zip", Title: "乙"},
	})
	works := fetchWorksForTest(t, r, cookie, "")
	if len(works) != 2 {
		t.Fatalf("works=%#v", works)
	}
	response := performAuthedRequest(r, http.MethodPut, "/api/works/"+works[0].ID+"/metadata",
		map[string]interface{}{"title": "单本新标题", "publisher": "出版社", "status": "ongoing"}, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("metadata status=%d body=%s", response.Code, response.Body.String())
	}
	reorderBody := map[string]interface{}{"orders": []map[string]interface{}{
		{"id": works[0].ID, "sortOrder": 20},
		{"id": works[1].ID, "sortOrder": 10},
	}}
	response = performAuthedRequest(r, http.MethodPut, "/api/works/reorder", reorderBody, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("reorder status=%d body=%s", response.Code, response.Body.String())
	}
	sorted := fetchWorksForTest(t, r, cookie, "?sortBy=custom&sortOrder=asc")
	if len(sorted) != 2 || sorted[0].ID != works[1].ID || sorted[1].Title != "单本新标题" || sorted[1].Status != "ongoing" {
		t.Fatalf("sorted=%#v", sorted)
	}
}

func TestWorkBatchPreflightsManagePermissionAcrossLibraries(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	_ = registerAndLogin(t, r)
	createWorkTestLibrary(t, "manage-allowed", "Allowed", "private")
	createWorkTestLibrary(t, "manage-denied", "Denied", "private")
	createWorkTestComics(t, "manage-allowed", []workTestComic{{ID: "manage-a", Path: "甲.zip", Title: "甲"}})
	createWorkTestComics(t, "manage-denied", []workTestComic{{ID: "manage-b", Path: "乙.zip", Title: "乙"}})
	cookie := createWorkTestUserSession(t, "manager", "manager-session")
	if err := store.SetUserLibraryAccess("manager", []store.LibraryAccessReq{
		{LibraryID: "manage-allowed", CanView: true, CanManage: true},
		{LibraryID: "manage-denied", CanView: true, CanManage: false},
	}); err != nil {
		t.Fatal(err)
	}
	works := fetchWorksForTest(t, r, cookie, "")
	body := map[string]interface{}{"workIds": []string{works[0].ID, works[1].ID}, "action": "favorite", "isFavorite": true}
	response := performAuthedRequest(r, http.MethodPost, "/api/works/batch", body, cookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, id := range []string{"manage-a", "manage-b"} {
		comic, _ := store.GetComicByIDForUser(id, "manager")
		if comic != nil && comic.IsFavorite {
			t.Fatalf("partial write occurred for %s", id)
		}
	}
}

func TestWorkDeleteDeduplicatesUnitsAndDeletesWholeWork(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "delete-work-lib", "Delete", "private")
	createWorkTestComics(t, "delete-work-lib", []workTestComic{
		{ID: "delete-work-1", Path: "作品/第一话.cbz", Title: "第一话"},
		{ID: "delete-work-2", Path: "作品/第二话.cbz", Title: "第二话"},
	})
	work := fetchSingleWorkForTest(t, r, cookie, "")
	response := performAuthedRequest(r, http.MethodDelete, "/api/works/"+work.ID, nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	for _, id := range []string{"delete-work-1", "delete-work-2"} {
		comic, err := store.GetComicByID(id)
		if err != nil || comic != nil {
			t.Fatalf("comic %s still exists: %#v err=%v", id, comic, err)
		}
	}
}

func TestWorkTagAndCategoryStatsCountWorksNotUnits(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "work-stat-lib", "Stats", "private")
	createWorkTestComics(t, "work-stat-lib", []workTestComic{
		{ID: "stat-1", Path: "作品/第一话.cbz", Title: "第一话"},
		{ID: "stat-2", Path: "作品/第二话.cbz", Title: "第二话"},
	})
	if err := service.RebuildComicSeriesForLibrary("work-stat-lib"); err != nil {
		t.Fatal(err)
	}
	work := fetchSingleWorkForTest(t, r, cookie, "")
	for path, body := range map[string]interface{}{
		"/api/works/" + work.ID + "/tags":       map[string]interface{}{"tags": []string{"热血"}},
		"/api/works/" + work.ID + "/categories": map[string]interface{}{"categorySlugs": []string{"action"}},
	} {
		response := performAuthedRequest(r, http.MethodPut, path, body, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/works/tags", "/api/works/categories"} {
		response := performAuthedRequest(r, http.MethodGet, path, nil, cookie)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"count":1`) {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func fetchSingleWorkForTest(t *testing.T, r *gin.Engine, cookie, query string) service.Work {
	t.Helper()
	works := fetchWorksForTest(t, r, cookie, query)
	if len(works) != 1 {
		t.Fatalf("works=%#v", works)
	}
	return works[0]
}

func fetchWorksForTest(t *testing.T, r *gin.Engine, cookie, query string) []service.Work {
	t.Helper()
	response := performAuthedRequest(r, http.MethodGet, "/api/works"+query, nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Works []service.Work `json:"works"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Works
}

func storeTestAdminUserID(t *testing.T) string {
	t.Helper()
	users, err := store.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.Role == "admin" {
			return user.ID
		}
	}
	t.Fatal("admin user not found")
	return ""
}
