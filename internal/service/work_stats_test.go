package service

import (
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestAggregateWorkReadingAnalyticsMergesComicUnitsAndKeepsNovels(t *testing.T) {
	startedFirst := time.Date(2026, time.July, 1, 10, 0, 0, 0, time.UTC)
	startedSecond := time.Date(2026, time.July, 2, 11, 0, 0, 0, time.UTC)
	startedNovel := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	works := []Work{{
		ID:        "work_demo",
		LibraryID: "comic-lib",
		Title:     "同一部漫画",
		CoverURL:  "/api/works/work_demo/cover",
		Genre:     "热血",
		Units: []WorkUnit{
			{ID: "unit_1", ComicID: "comic-1"},
			{ID: "unit_2", ComicID: "comic-2"},
		},
	}}
	novels := []store.ComicListItem{{
		ID: "novel-1", LibraryID: "novel-lib", Title: "一本小说",
		CoverURL: "/api/comics/novel-1/thumbnail", Genre: "奇幻", ComicType: "novel",
	}}
	sessions := []store.ReadingSessionRecord{
		{ID: 1, ComicID: "comic-1", StartedAt: startedFirst, Duration: 120, StartPage: 0, EndPage: 10},
		{ID: 2, ComicID: "comic-2", StartedAt: startedSecond, Duration: 180, StartPage: 1, EndPage: 20},
		{ID: 3, ComicID: "novel-1", StartedAt: startedNovel, Duration: 240, StartPage: 2, EndPage: 30},
	}

	result := AggregateWorkReadingAnalytics(sessions, works, novels, time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC))

	if result.TotalReadTime != 540 {
		t.Fatalf("totalReadTime = %d, want 540", result.TotalReadTime)
	}
	if result.TotalSessions != 3 {
		t.Fatalf("totalSessions = %d, want 3", result.TotalSessions)
	}
	if result.TotalComicsRead != 2 {
		t.Fatalf("totalComicsRead = %d, want one Work plus one novel", result.TotalComicsRead)
	}
	if len(result.History) != 2 {
		t.Fatalf("history length = %d, want 2 logical works", len(result.History))
	}
	if got := result.History[1]; got.ID != "work_demo" || got.TotalReadTime != 300 || got.SessionCount != 2 {
		t.Fatalf("comic history = %#v, want merged Work totals", got)
	}
	if got := result.History[1]; got.LastUnitID != "unit_2" ||
		got.ReadHref != "/reader/comic-2?page=20&workId=work_demo&unitId=unit_2" {
		t.Fatalf("comic history navigation = %#v", got)
	}
	if got := result.History[0]; got.ID != "novel-1" || got.ContentType != "novel" || got.TotalReadTime != 240 {
		t.Fatalf("novel history = %#v, want unchanged novel identity", got)
	}
	if got := result.History[0]; got.DetailHref != "/comic/novel-1" || got.ReadHref != "/novel/novel-1" {
		t.Fatalf("novel navigation = %#v", got)
	}
	if len(result.RecentSessions) != 2 {
		t.Fatalf("recent sessions length = %d, want one row per logical work", len(result.RecentSessions))
	}
	if got := result.RecentSessions[1]; got.ComicID != "work_demo" || got.ComicTitle != "同一部漫画" || got.Duration != 300 || got.SessionCount != 2 {
		t.Fatalf("merged recent work = %#v", got)
	}
}

func TestAggregateWorkReadingAnalyticsCountsDistinctWorksPerMonthAndGenre(t *testing.T) {
	works := []Work{{
		ID: "work_demo", Title: "同一部漫画", Genre: "热血", CoverURL: "/api/works/work_demo/cover",
		Units: []WorkUnit{{ComicID: "comic-1"}, {ComicID: "comic-2"}},
	}}
	sessions := []store.ReadingSessionRecord{
		{ID: 1, ComicID: "comic-1", StartedAt: time.Date(2026, time.June, 1, 8, 0, 0, 0, time.Local), Duration: 60},
		{ID: 2, ComicID: "comic-2", StartedAt: time.Date(2026, time.June, 2, 8, 0, 0, 0, time.Local), Duration: 90},
	}

	result := AggregateWorkReadingAnalytics(sessions, works, nil, time.Date(2026, time.June, 3, 0, 0, 0, 0, time.Local))

	if len(result.MonthlyStats) != 1 || result.MonthlyStats[0].Comics != 1 || result.MonthlyStats[0].Sessions != 2 {
		t.Fatalf("monthly stats = %#v, want one Work and two sessions", result.MonthlyStats)
	}
	if len(result.GenreStats) != 1 || result.GenreStats[0].ComicCount != 1 || result.GenreStats[0].TotalTime != 150 {
		t.Fatalf("genre stats = %#v, want one Work with accumulated duration", result.GenreStats)
	}
}

func TestAggregateWorkYearlyReportUsesWorkIDsForTopItems(t *testing.T) {
	works := []Work{{
		ID: "work_demo", Title: "同一部漫画", Genre: "热血", CoverURL: "/api/works/work_demo/cover",
		Units: []WorkUnit{{ComicID: "comic-1"}, {ComicID: "comic-2"}},
	}}
	sessions := []store.ReadingSessionRecord{
		{ID: 1, ComicID: "comic-1", StartedAt: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.Local), Duration: 60, StartPage: 0, EndPage: 5},
		{ID: 2, ComicID: "comic-2", StartedAt: time.Date(2026, time.March, 2, 8, 0, 0, 0, time.Local), Duration: 90, StartPage: 1, EndPage: 8},
	}

	report := AggregateWorkYearlyReport(2026, sessions, works, nil)

	if report.TotalComicsRead != 1 || report.TotalSessions != 2 || report.TotalReadTime != 150 {
		t.Fatalf("yearly totals = %#v", report)
	}
	if got := report.MonthlyStats[2]; got.Comics != 1 || got.Sessions != 2 || got.Duration != 150 {
		t.Fatalf("March stats = %#v", got)
	}
	if len(report.TopComics) != 1 || report.TopComics[0].ID != "work_demo" || report.TopComics[0].ReadTime != 150 || report.TopComics[0].Sessions != 2 {
		t.Fatalf("top works = %#v", report.TopComics)
	}
	if report.TopComics[0].CoverURL != "/api/works/work_demo/cover" ||
		report.TopComics[0].ContentType != "comic" ||
		report.TopComics[0].DetailHref != "/work/work_demo" {
		t.Fatalf("top Work navigation metadata = %#v", report.TopComics[0])
	}
}
