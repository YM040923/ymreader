package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestLibraryImmediateScrapeCreatesObservableTask(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "task-library", "Task Library", "private")
	createWorkTestComics(t, "task-library", []workTestComic{
		{ID: "task-unit-1", Path: "Task Work/Chapter 001.cbz", Title: "Chapter 001"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"task-library"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	source, author := "manual", "Task Author"
	if err := store.UpdateLogicalWorkMetadata(works[0].ID, store.LogicalWorkMetadataUpdate{
		MetadataSource: &source, Author: &author,
	}); err != nil {
		t.Fatal(err)
	}
	cover := "https://example.test/task-cover.jpg"
	if err := store.UpdateLogicalWorkCover(works[0].ID, store.LogicalWorkCoverUpdate{CoverURL: &cover}); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(
		r, http.MethodPost, "/api/metadata/tasks/libraries/task-library",
		map[string]bool{"force": false}, cookie,
	)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start task status=%d body=%s", response.Code, response.Body.String())
	}
	var started service.ManualWorkScrapeTask
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.ID == "" || started.LibraryID != "task-library" {
		t.Fatalf("invalid task response: %#v", started)
	}
	response = performAuthedRequest(r, http.MethodGet, "/api/metadata/tasks/"+started.ID, nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("observe task status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMetadataStatsCountsComicWorksAndNovelComics(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)

	createWorkTestLibrary(t, "stats-comics", "Comic stats", "private")
	createWorkTestComics(t, "stats-comics", []workTestComic{
		{ID: "stats-chapter-1", Path: "Unified Work/Chapter 001.cbz", Title: "Chapter 001"},
		{ID: "stats-chapter-2", Path: "Unified Work/Chapter 002.cbz", Title: "Chapter 002"},
	})
	if err := store.CreateLibrary(&model.Library{
		ID: "stats-novels", Name: "Novel stats", Type: "novel", RootPath: "/test/stats-novels",
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}); err != nil {
		t.Fatal(err)
	}
	createWorkTestComics(t, "stats-novels", []workTestComic{
		{ID: "stats-novel-1", Path: "Novel One.epub", Title: "Novel One"},
		{ID: "stats-novel-2", Path: "Novel Two.epub", Title: "Novel Two"},
	})
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "type" = 'novel' WHERE "libraryId" = 'stats-novels'`); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(r, http.MethodGet, "/api/metadata/stats", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 3 {
		t.Fatalf("metadata stats total=%d, want one comic Work plus two novel Comics; body=%s", payload.Total, response.Body.String())
	}
}

func TestMetadataTargetMissingUsesLogicalWorkState(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "missing-work", "Missing Work", "private")
	createWorkTestComics(t, "missing-work", []workTestComic{
		{ID: "missing-unit-1", Path: "Logical Metadata/Chapter 001.cbz", Title: "Chapter metadata"},
		{ID: "missing-unit-2", Path: "Logical Metadata/Chapter 002.cbz", Title: "Chapter metadata"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"missing-work"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "metadataSource" = 'chapter-only', "coverImageUrl" = 'chapter.jpg'
		WHERE "libraryId" = 'missing-work'
	`); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(r, http.MethodGet, "/api/metadata/library?contentType=comic&libraryId=missing-work&metaFilter=missing", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var missingPayload struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &missingPayload); err != nil {
		t.Fatal(err)
	}
	if missingPayload.Total != 1 {
		logical, _ := store.GetLogicalWork(works[0].ID)
		t.Fatalf("chapter Comic metadata incorrectly satisfied LogicalWork missing state: logical=%#v response=%s", logical, response.Body.String())
	}

	source := "manual"
	author := "Work Author"
	if err := store.UpdateLogicalWorkMetadata(works[0].ID, store.LogicalWorkMetadataUpdate{
		MetadataSource: &source, Author: &author,
	}); err != nil {
		t.Fatal(err)
	}
	cover := "https://example.test/work-cover.jpg"
	if err := store.UpdateLogicalWorkCover(works[0].ID, store.LogicalWorkCoverUpdate{CoverURL: &cover}); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()
	response = performAuthedRequest(r, http.MethodGet, "/api/metadata/library?contentType=comic&libraryId=missing-work&metaFilter=missing", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &missingPayload); err != nil {
		t.Fatal(err)
	}
	if missingPayload.Total != 0 {
		t.Fatalf("complete LogicalWork metadata and cover were still classified as missing: %s", response.Body.String())
	}
}

func TestClearMetadataClearsLogicalWorkTarget(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "clear-work", "Clear Work", "private")
	createWorkTestComics(t, "clear-work", []workTestComic{
		{ID: "clear-unit-1", Path: "Clear Me/Chapter 001.cbz", Title: "Chapter 001"},
		{ID: "clear-unit-2", Path: "Clear Me/Chapter 002.cbz", Title: "Chapter 002"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"clear-work"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	workID := works[0].ID
	source, author, publisher, description, language, genre, status := "bangumi", "Author", "Publisher", "Description", "en", "Action", "ongoing"
	year := 2026
	locked := true
	if err := store.UpdateLogicalWorkMetadata(workID, store.LogicalWorkMetadataUpdate{
		Author: &author, Publisher: &publisher, Year: &year, Description: &description,
		Language: &language, Genre: &genre, Status: &status,
		MetadataSource: &source, MetadataLocked: &locked,
	}); err != nil {
		t.Fatal(err)
	}
	coverURL := "https://example.test/cover.jpg"
	if err := store.UpdateLogicalWorkCover(workID, store.LogicalWorkCoverUpdate{
		CoverURL: &coverURL, CoverSource: &source, CoverLocked: &locked,
	}); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(r, http.MethodPost, "/api/metadata/clear", map[string]interface{}{
		"targets": []map[string]string{{"id": workID, "entityType": "work"}},
	}, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("clear status=%d body=%s", response.Code, response.Body.String())
	}
	work, err := store.GetLogicalWork(workID)
	if err != nil {
		t.Fatal(err)
	}
	if work.Author != "" || work.Publisher != "" || work.Year != nil || work.Description != "" ||
		work.Language != "" || work.Genre != "" || work.Status != "" || work.MetadataSource != "" ||
		work.MetadataLocked || work.CoverURL != "" || work.CoverSource != "" || work.CoverLocked {
		t.Fatalf("logical Work metadata was not cleared: %#v", work)
	}
}

func TestComicMetadataSearchQueryUsesOnlyWorkTitleAndAuthor(t *testing.T) {
	target := metadataTarget{
		EntityType: "work",
		EntityID:   "work-1",
		Title:      "Work Title",
		Author:     "Work Author",
		Filename:   "Work Title/Chapter 099 - noisy filename.cbz",
	}
	query := metadataTargetSearchQuery(target)
	if query != "Work Title Work Author" {
		t.Fatalf("query=%q, want Work title and author only", query)
	}
	if strings.Contains(query, "Chapter") || strings.Contains(query, "filename") {
		t.Fatalf("Work search query leaked Unit filename: %q", query)
	}
}

func TestMetadataProgressEventIncludesStableEntityFields(t *testing.T) {
	event := metadataProgressEvent(metadataTarget{
		EntityType: "work",
		EntityID:   "work-1",
		Title:      "Work Title",
	}, 1, 2)
	if event["entityType"] != "work" || event["entityId"] != "work-1" || event["title"] != "Work Title" {
		t.Fatalf("incomplete entity identity in SSE event: %#v", event)
	}
}

func TestMetadataTargetTreatsLogicalWorkRepresentativeCoverAsPresent(t *testing.T) {
	target := metadataTarget{
		EntityType: "work",
		EntityID:   "work-local-cover",
		LogicalWork: &store.LogicalWork{
			ID: "work-local-cover", MetadataSource: "comicinfo", CoverComicID: "unit-cover",
		},
	}
	if metadataTargetMissing(target) {
		t.Fatal("LogicalWork representative Comic cover was treated as a missing Work cover")
	}
}
