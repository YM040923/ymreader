package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestWorkStatsAndHistoryAggregateUnitsButKeepNovelIdentity(t *testing.T) {
	router := setupTestRouter(t)
	cookie := registerAndLogin(t, router)
	user, err := store.GetUserByUsername("admin")
	if err != nil || user == nil {
		t.Fatalf("load admin: %v", err)
	}
	createWorkTestLibrary(t, "comic-lib", "Comics", "read")
	if err := store.CreateLibrary(&model.Library{
		ID: "novel-lib", Name: "Novels", Type: "novel", RootPath: "/test/novels",
		Enabled: true, ScanEnabled: true, DefaultAccess: "read",
	}); err != nil {
		t.Fatal(err)
	}
	createWorkTestComics(t, "comic-lib", []workTestComic{
		{ID: "chapter-1", Path: "同一部漫画/第001话", Title: "第一话"},
		{ID: "chapter-2", Path: "同一部漫画/第002话", Title: "第二话"},
	})
	createWorkTestComics(t, "novel-lib", []workTestComic{
		{ID: "novel-1", Path: "一本小说.epub", Title: "一本小说"},
	})
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "type" = 'novel' WHERE "id" = 'novel-1'`); err != nil {
		t.Fatal(err)
	}
	for index, session := range []struct {
		comicID  string
		duration int
	}{
		{"chapter-1", 60},
		{"chapter-2", 90},
		{"novel-1", 120},
	} {
		if _, err := store.DB().Exec(`
			INSERT INTO "ReadingSession"
				("comicId", "userId", "startedAt", "duration", "startPage", "endPage")
			VALUES (?, ?, ?, ?, 0, 5)
		`, session.comicID, user.ID, time.Date(2026, time.March, index+1, 8, 0, 0, 0, time.Local), session.duration); err != nil {
			t.Fatal(err)
		}
	}

	statsResponse := performAuthedRequest(router, http.MethodGet, "/api/stats/enhanced?view=work&includeNovels=true", nil, cookie)
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("stats status = %d: %s", statsResponse.Code, statsResponse.Body.String())
	}
	var stats service.WorkReadingAnalytics
	if err := json.Unmarshal(statsResponse.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalComicsRead != 2 || stats.TotalSessions != 3 || stats.TotalReadTime != 270 {
		t.Fatalf("stats totals = %#v", stats)
	}
	if len(stats.RecentSessions) != 2 {
		t.Fatalf("recent sessions = %#v", stats.RecentSessions)
	}

	yearlyResponse := performAuthedRequest(router, http.MethodGet, "/api/stats/yearly?year=2026&view=work&includeNovels=true", nil, cookie)
	if yearlyResponse.Code != http.StatusOK {
		t.Fatalf("yearly status = %d: %s", yearlyResponse.Code, yearlyResponse.Body.String())
	}
	var yearly store.YearlyReadingReport
	if err := json.Unmarshal(yearlyResponse.Body.Bytes(), &yearly); err != nil {
		t.Fatal(err)
	}
	if yearly.TotalComicsRead != 2 || len(yearly.TopComics) != 2 {
		t.Fatalf("yearly report = %#v", yearly)
	}
	works := service.BuildWorksFromComicList([]store.ComicListItem{
		{ID: "chapter-1", LibraryID: "comic-lib", RelativePath: "同一部漫画/第001话", Title: "第一话"},
		{ID: "chapter-2", LibraryID: "comic-lib", RelativePath: "同一部漫画/第002话", Title: "第二话"},
	}, service.WorkBuildOptions{})
	if len(works) != 1 || yearly.TopComics[0].ID != works[0].ID {
		t.Fatalf("top item id = %q, want Work %q", yearly.TopComics[0].ID, works[0].ID)
	}

	historyResponse := performAuthedRequest(router, http.MethodGet, "/api/history?view=work&includeNovels=true", nil, cookie)
	if historyResponse.Code != http.StatusOK {
		t.Fatalf("history status = %d: %s", historyResponse.Code, historyResponse.Body.String())
	}
	var history struct {
		Items []service.WorkReadingHistoryItem `json:"items"`
		Total int                              `json:"total"`
	}
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if history.Total != 2 || len(history.Items) != 2 {
		t.Fatalf("history = %#v", history)
	}
	if history.Items[1].ContentType != "comic" || history.Items[1].SessionCount != 2 || history.Items[1].TotalReadTime != 150 {
		t.Fatalf("comic history = %#v", history.Items[1])
	}
}
