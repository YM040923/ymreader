package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestMetadataLibraryListsOneItemPerComicWork(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "metadata-work-library", "Metadata Works", "private")
	createWorkTestComics(t, "metadata-work-library", []workTestComic{
		{ID: "metadata-work-1", Path: "整部作品/第001话.cbz", Title: "第001话"},
		{ID: "metadata-work-2", Path: "整部作品/第002话.cbz", Title: "第002话"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"metadata-work-library"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(r, http.MethodGet, "/api/metadata/library?contentType=comic&libraryId=metadata-work-library", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []struct {
			ID         string `json:"id"`
			EntityType string `json:"entityType"`
			Title      string `json:"title"`
			ItemCount  int    `json:"itemCount"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("metadata library expanded Work units: %s", response.Body.String())
	}
	if payload.Items[0].EntityType != "work" || payload.Items[0].ItemCount != 2 || payload.Items[0].ID != works[0].ID {
		t.Fatalf("unexpected Work metadata item: %#v", payload.Items[0])
	}
}

func TestLibraryManualScrapeCountsOneEligibleTargetPerWork(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "manual-scrape-library", "Manual scrape", "private")
	createWorkTestComics(t, "manual-scrape-library", []workTestComic{
		{ID: "manual-scrape-1", Path: "已完成作品/第001话.cbz", Title: "第001话"},
		{ID: "manual-scrape-2", Path: "已完成作品/第002话.cbz", Title: "第002话"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"manual-scrape-library"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	source, author := "manual", "作者"
	if err := store.UpdateLogicalWorkMetadata(works[0].ID, store.LogicalWorkMetadataUpdate{
		MetadataSource: &source, Author: &author,
	}); err != nil {
		t.Fatal(err)
	}
	coverURL := "https://example.test/cover.jpg"
	if err := store.UpdateLogicalWorkCover(works[0].ID, store.LogicalWorkCoverUpdate{CoverURL: &coverURL}); err != nil {
		t.Fatal(err)
	}

	response := performAuthedRequest(r, http.MethodPost, "/api/admin/libraries/manual-scrape-library/scrape", bytes.NewBufferString(`{}`), cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload service.ManualWorkScrapeResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 1 || payload.Skipped != 1 || payload.Success != 0 || payload.Failed != 0 {
		t.Fatalf("manual scrape counted Units instead of Work: %#v", payload)
	}
}
