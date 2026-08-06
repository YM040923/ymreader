package service

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

type WorkReadingHistoryItem struct {
	ID            string `json:"id"`
	ContentType   string `json:"contentType"`
	LibraryID     string `json:"libraryId,omitempty"`
	Title         string `json:"title"`
	CoverURL      string `json:"coverUrl,omitempty"`
	Genre         string `json:"genre,omitempty"`
	TotalReadTime int    `json:"totalReadTime"`
	SessionCount  int    `json:"sessionCount"`
	LastReadAt    string `json:"lastReadAt"`
	LastComicID   string `json:"lastComicId"`
	LastUnitID    string `json:"lastUnitId,omitempty"`
	StartPage     int    `json:"startPage"`
	EndPage       int    `json:"endPage"`
	DetailHref    string `json:"detailHref"`
	ReadHref      string `json:"readHref"`
}

type WorkRecentSessionItem struct {
	ID           int     `json:"id"`
	ComicID      string  `json:"comicId"`
	WorkID       string  `json:"workId,omitempty"`
	UnitID       string  `json:"unitId,omitempty"`
	ContentType  string  `json:"contentType"`
	ComicTitle   string  `json:"comicTitle"`
	CoverURL     string  `json:"coverUrl,omitempty"`
	StartedAt    string  `json:"startedAt"`
	EndedAt      *string `json:"endedAt"`
	Duration     int     `json:"duration"`
	StartPage    int     `json:"startPage"`
	EndPage      int     `json:"endPage"`
	SessionCount int     `json:"sessionCount"`
}

type WorkDailyStatItem struct {
	Date     string `json:"date"`
	Duration int    `json:"duration"`
	Sessions int    `json:"sessions"`
	Works    int    `json:"works"`
}

type WorkMonthlyStatItem struct {
	Month    string `json:"month"`
	Duration int    `json:"duration"`
	Sessions int    `json:"sessions"`
	Comics   int    `json:"comics"`
}

type WorkGenreStatItem struct {
	Genre      string `json:"genre"`
	TotalTime  int    `json:"totalTime"`
	ComicCount int    `json:"comicCount"`
}

type WorkReadingAnalytics struct {
	TotalReadTime   int                      `json:"totalReadTime"`
	TotalSessions   int                      `json:"totalSessions"`
	TotalComicsRead int                      `json:"totalComicsRead"`
	TodayReadTime   int                      `json:"todayReadTime"`
	WeekReadTime    int                      `json:"weekReadTime"`
	CurrentStreak   int                      `json:"currentStreak"`
	LongestStreak   int                      `json:"longestStreak"`
	AvgPagesPerHour float64                  `json:"avgPagesPerHour"`
	RecentSessions  []WorkRecentSessionItem  `json:"recentSessions"`
	DailyStats      []WorkDailyStatItem      `json:"dailyStats"`
	MonthlyStats    []WorkMonthlyStatItem    `json:"monthlyStats"`
	GenreStats      []WorkGenreStatItem      `json:"genreStats"`
	History         []WorkReadingHistoryItem `json:"history"`
}

type readingCatalogEntry struct {
	ID          string
	ContentType string
	LibraryID   string
	Title       string
	CoverURL    string
	Genre       string
	Units       []WorkUnit
}

type readingAggregate struct {
	entry     readingCatalogEntry
	duration  int
	sessions  int
	latest    store.ReadingSessionRecord
	hasLatest bool
}

func AggregateWorkReadingAnalytics(sessions []store.ReadingSessionRecord, works []Work, novels []store.ComicListItem, now time.Time) *WorkReadingAnalytics {
	result := &WorkReadingAnalytics{
		RecentSessions: []WorkRecentSessionItem{},
		DailyStats:     []WorkDailyStatItem{},
		MonthlyStats:   []WorkMonthlyStatItem{},
		GenreStats:     []WorkGenreStatItem{},
		History:        []WorkReadingHistoryItem{},
	}
	catalog := readingCatalog(works, novels)
	aggregates := map[string]*readingAggregate{}
	daily := map[string]*periodAggregate{}
	monthly := map[string]*periodAggregate{}
	genre := map[string]*periodAggregate{}
	activeDates := map[string]struct{}{}
	totalPages := 0
	now = now.In(time.Local)
	today := localDate(now)
	weekStart := startOfLocalWeek(now)

	for _, session := range sessions {
		entry, ok := catalog[session.ComicID]
		if !ok || session.Duration <= 0 {
			continue
		}
		result.TotalReadTime += session.Duration
		result.TotalSessions++
		if session.EndPage > session.StartPage {
			totalPages += session.EndPage - session.StartPage
		}
		localStarted := session.StartedAt.In(time.Local)
		dateKey := localStarted.Format("2006-01-02")
		monthKey := localStarted.Format("2006-01")
		activeDates[dateKey] = struct{}{}
		if dateKey == today {
			result.TodayReadTime += session.Duration
		}
		if !localStarted.Before(weekStart) && localStarted.Before(weekStart.AddDate(0, 0, 7)) {
			result.WeekReadTime += session.Duration
		}
		addPeriod(daily, dateKey, entry.ID, session.Duration)
		addPeriod(monthly, monthKey, entry.ID, session.Duration)
		if entry.Genre != "" {
			addPeriod(genre, entry.Genre, entry.ID, session.Duration)
		}
		aggregate := aggregates[entry.ID]
		if aggregate == nil {
			aggregate = &readingAggregate{entry: entry}
			aggregates[entry.ID] = aggregate
		}
		aggregate.duration += session.Duration
		aggregate.sessions++
		if !aggregate.hasLatest || session.StartedAt.After(aggregate.latest.StartedAt) ||
			(session.StartedAt.Equal(aggregate.latest.StartedAt) && session.ID > aggregate.latest.ID) {
			aggregate.latest = session
			aggregate.hasLatest = true
		}
	}
	result.TotalComicsRead = len(aggregates)
	if result.TotalReadTime > 0 {
		result.AvgPagesPerHour = float64(totalPages) / (float64(result.TotalReadTime) / 3600)
	}
	result.CurrentStreak, result.LongestStreak = readingStreaks(activeDates, now)
	result.DailyStats = buildDailyStats(daily, now.AddDate(0, 0, -90))
	result.MonthlyStats = buildMonthlyStats(monthly, now.AddDate(-1, 0, 0))
	result.GenreStats = buildGenreStats(genre)
	result.History, result.RecentSessions = buildReadingHistory(aggregates)
	return result
}

type periodAggregate struct {
	duration int
	sessions int
	works    map[string]struct{}
}

func addPeriod(periods map[string]*periodAggregate, key, workID string, duration int) {
	item := periods[key]
	if item == nil {
		item = &periodAggregate{works: map[string]struct{}{}}
		periods[key] = item
	}
	item.duration += duration
	item.sessions++
	item.works[workID] = struct{}{}
}

func readingCatalog(works []Work, novels []store.ComicListItem) map[string]readingCatalogEntry {
	result := make(map[string]readingCatalogEntry)
	for _, work := range works {
		entry := readingCatalogEntry{
			ID: work.ID, ContentType: "comic", LibraryID: work.LibraryID,
			Title: work.Title, CoverURL: work.CoverURL, Genre: work.Genre,
			Units: append([]WorkUnit(nil), work.Units...),
		}
		for _, unit := range work.Units {
			result[unit.ComicID] = entry
		}
	}
	for _, novel := range novels {
		result[novel.ID] = readingCatalogEntry{
			ID: novel.ID, ContentType: "novel", LibraryID: novel.LibraryID,
			Title: novel.Title, CoverURL: novel.CoverURL, Genre: novel.Genre,
		}
	}
	return result
}

func buildReadingHistory(aggregates map[string]*readingAggregate) ([]WorkReadingHistoryItem, []WorkRecentSessionItem) {
	items := make([]*readingAggregate, 0, len(aggregates))
	for _, item := range aggregates {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].latest.StartedAt.After(items[j].latest.StartedAt)
	})
	history := make([]WorkReadingHistoryItem, 0, len(items))
	recent := make([]WorkRecentSessionItem, 0, minInt(len(items), 50))
	for _, item := range items {
		latest := item.latest
		startedAt := latest.StartedAt.UTC().Format(time.RFC3339Nano)
		detailHref := readingDetailHref(item.entry)
		lastUnitID := ""
		readHref := "/novel/" + url.PathEscape(item.entry.ID)
		if item.entry.ContentType == "comic" {
			lastUnitID = readingUnitForPage(item.entry.Units, latest.ComicID, latest.EndPage)
			readHref = fmt.Sprintf(
				"/reader/%s?page=%d&workId=%s",
				url.PathEscape(latest.ComicID), latest.EndPage, url.QueryEscape(item.entry.ID),
			)
			if lastUnitID != "" {
				readHref += "&unitId=" + url.QueryEscape(lastUnitID)
			}
		}
		history = append(history, WorkReadingHistoryItem{
			ID: item.entry.ID, ContentType: item.entry.ContentType, LibraryID: item.entry.LibraryID,
			Title: item.entry.Title, CoverURL: item.entry.CoverURL, Genre: item.entry.Genre,
			TotalReadTime: item.duration, SessionCount: item.sessions, LastReadAt: startedAt,
			LastComicID: latest.ComicID, LastUnitID: lastUnitID,
			StartPage: latest.StartPage, EndPage: latest.EndPage,
			DetailHref: detailHref, ReadHref: readHref,
		})
		if len(recent) < 50 {
			var endedAt *string
			if latest.EndedAt != nil {
				value := latest.EndedAt.UTC().Format(time.RFC3339Nano)
				endedAt = &value
			}
			workID := ""
			if item.entry.ContentType == "comic" {
				workID = item.entry.ID
			}
			recent = append(recent, WorkRecentSessionItem{
				ID: latest.ID, ComicID: item.entry.ID, WorkID: workID,
				UnitID: lastUnitID, ContentType: item.entry.ContentType, ComicTitle: item.entry.Title,
				CoverURL: item.entry.CoverURL, StartedAt: startedAt, EndedAt: endedAt,
				Duration: item.duration, StartPage: latest.StartPage, EndPage: latest.EndPage,
				SessionCount: item.sessions,
			})
		}
	}
	return history, recent
}

func buildDailyStats(periods map[string]*periodAggregate, earliest time.Time) []WorkDailyStatItem {
	keys := make([]string, 0, len(periods))
	earliestKey := earliest.In(time.Local).Format("2006-01-02")
	for key := range periods {
		if key >= earliestKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]WorkDailyStatItem, 0, len(keys))
	for _, key := range keys {
		item := periods[key]
		result = append(result, WorkDailyStatItem{
			Date: key, Duration: item.duration, Sessions: item.sessions, Works: len(item.works),
		})
	}
	return result
}

func buildMonthlyStats(periods map[string]*periodAggregate, earliest time.Time) []WorkMonthlyStatItem {
	keys := make([]string, 0, len(periods))
	earliestKey := earliest.In(time.Local).Format("2006-01")
	for key := range periods {
		if key >= earliestKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]WorkMonthlyStatItem, 0, len(keys))
	for _, key := range keys {
		item := periods[key]
		result = append(result, WorkMonthlyStatItem{
			Month: key, Duration: item.duration, Sessions: item.sessions, Comics: len(item.works),
		})
	}
	return result
}

func buildGenreStats(periods map[string]*periodAggregate) []WorkGenreStatItem {
	result := make([]WorkGenreStatItem, 0, len(periods))
	for key, item := range periods {
		result = append(result, WorkGenreStatItem{
			Genre: key, TotalTime: item.duration, ComicCount: len(item.works),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalTime == result[j].TotalTime {
			return result[i].Genre < result[j].Genre
		}
		return result[i].TotalTime > result[j].TotalTime
	})
	return result
}

func readingStreaks(activeDates map[string]struct{}, now time.Time) (int, int) {
	if len(activeDates) == 0 {
		return 0, 0
	}
	dates := make([]time.Time, 0, len(activeDates))
	for value := range activeDates {
		if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
			dates = append(dates, parsed)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) })
	longest, run := 0, 0
	for index, date := range dates {
		if index == 0 || dates[index-1].Sub(date) <= 24*time.Hour {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	current := 0
	today := startOfLocalDay(now)
	if len(dates) > 0 && (dates[0].Equal(today) || dates[0].Equal(today.AddDate(0, 0, -1))) {
		current = 1
		for index := 1; index < len(dates); index++ {
			if dates[index-1].Sub(dates[index]) > 24*time.Hour {
				break
			}
			current++
		}
	}
	return current, longest
}

func AggregateWorkYearlyReport(year int, sessions []store.ReadingSessionRecord, works []Work, novels []store.ComicListItem) *store.YearlyReadingReport {
	report := &store.YearlyReadingReport{
		Year: year, MonthlyStats: make([]store.MonthlyReadingStat, 12),
		TopComics: []store.TopReadComic{}, GenreDistribution: []store.GenreDistributionItem{},
	}
	for month := 1; month <= 12; month++ {
		report.MonthlyStats[month-1].Month = month
	}
	catalog := readingCatalog(works, novels)
	byWork := map[string]*periodAggregate{}
	byGenre := map[string]*periodAggregate{}
	monthWorks := make([]map[string]struct{}, 12)
	for index := range monthWorks {
		monthWorks[index] = map[string]struct{}{}
	}
	allWorks := map[string]struct{}{}
	for _, session := range sessions {
		entry, ok := catalog[session.ComicID]
		localStarted := session.StartedAt.In(time.Local)
		if !ok || session.Duration <= 0 || localStarted.Year() != year {
			continue
		}
		report.TotalReadTime += session.Duration
		report.TotalSessions++
		if session.EndPage > session.StartPage {
			report.TotalPagesRead += session.EndPage - session.StartPage
		}
		allWorks[entry.ID] = struct{}{}
		month := localStarted.Month()
		monthly := &report.MonthlyStats[int(month)-1]
		monthly.Duration += session.Duration
		monthly.Sessions++
		monthWorks[int(month)-1][entry.ID] = struct{}{}
		addPeriod(byWork, entry.ID, entry.ID, session.Duration)
		if entry.Genre != "" {
			addPeriod(byGenre, entry.Genre, entry.ID, session.Duration)
		}
	}
	report.TotalComicsRead = len(allWorks)
	for index := range report.MonthlyStats {
		report.MonthlyStats[index].Comics = len(monthWorks[index])
	}
	for id, item := range byWork {
		entry := readingCatalogEntry{}
		for _, candidate := range catalog {
			if candidate.ID == id {
				entry = candidate
				break
			}
		}
		report.TopComics = append(report.TopComics, store.TopReadComic{
			ID: id, Title: entry.Title, CoverURL: entry.CoverURL, ContentType: entry.ContentType,
			DetailHref: readingDetailHref(entry), ReadTime: item.duration, Sessions: item.sessions,
		})
	}
	sort.Slice(report.TopComics, func(i, j int) bool {
		if report.TopComics[i].ReadTime == report.TopComics[j].ReadTime {
			return report.TopComics[i].Title < report.TopComics[j].Title
		}
		return report.TopComics[i].ReadTime > report.TopComics[j].ReadTime
	})
	if len(report.TopComics) > 10 {
		report.TopComics = report.TopComics[:10]
	}
	for genre, item := range byGenre {
		report.GenreDistribution = append(report.GenreDistribution, store.GenreDistributionItem{
			Genre: genre, Count: len(item.works), ReadTime: item.duration,
		})
	}
	sort.Slice(report.GenreDistribution, func(i, j int) bool {
		return report.GenreDistribution[i].ReadTime > report.GenreDistribution[j].ReadTime
	})
	return report
}

func localDate(value time.Time) string {
	return value.In(time.Local).Format("2006-01-02")
}

func readingDetailHref(entry readingCatalogEntry) string {
	if entry.ContentType == "novel" {
		return "/comic/" + url.PathEscape(entry.ID)
	}
	return "/work/" + url.PathEscape(entry.ID)
}

func readingUnitForPage(units []WorkUnit, comicID string, page int) string {
	candidates := make([]WorkUnit, 0, len(units))
	for _, unit := range units {
		if unit.ComicID == comicID {
			candidates = append(candidates, unit)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].StartPage < candidates[j].StartPage
	})
	target := candidates[0]
	for _, unit := range candidates {
		if page < unit.StartPage {
			break
		}
		target = unit
		if unit.PageCount <= 0 || page < unit.StartPage+unit.PageCount {
			break
		}
	}
	return target.ID
}

func startOfLocalDay(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func startOfLocalWeek(value time.Time) time.Time {
	start := startOfLocalDay(value)
	offset := (int(start.Weekday()) + 6) % 7
	return start.AddDate(0, 0, -offset)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
