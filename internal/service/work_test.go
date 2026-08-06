package service

import (
	"archive/zip"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestBuildWorksFromComicListGroupsFolderChaptersAndUsesFirstUnitCover(t *testing.T) {
	items := []store.ComicListItem{
		{ID: "c10", Title: "第010话", Filename: "第010话/", LibraryID: "lib", RelativePath: "恶魔X天使 不能友好相处/第010话/", PageCount: 10, CoverURL: "/api/comics/c10/thumbnail"},
		{ID: "c2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "恶魔X天使 不能友好相处/第002话/", PageCount: 12, CoverURL: "/api/comics/c2/thumbnail"},
		{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "恶魔X天使 不能友好相处/第001话/", PageCount: 8, CoverURL: "/api/comics/c1/thumbnail"},
	}

	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("len(works)=%d, want 1", len(works))
	}
	work := works[0]
	if work.Title != "恶魔X天使 不能友好相处" {
		t.Fatalf("title=%q", work.Title)
	}
	if got := unitLabels(work.Units); !reflect.DeepEqual(got, []string{"第001话", "第002话", "第010话"}) {
		t.Fatalf("labels=%#v", got)
	}
	if work.CoverComicID != "c1" || work.CoverURL != "/api/comics/c1/thumbnail" {
		t.Fatalf("cover comic/url=%q/%q, want c1 cover", work.CoverComicID, work.CoverURL)
	}
}

func TestBuildWorksFromComicListGroupsMixedFolderAndArchiveUnits(t *testing.T) {
	items := []store.ComicListItem{
		{ID: "c10", Title: "第010话", Filename: "第010话.cbz", LibraryID: "lib", RelativePath: "我为邪帝/第010话.cbz", PageCount: 10},
		{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "我为邪帝/第001话/", PageCount: 8},
		{ID: "c2", Title: "第002话", Filename: "第002话.zip", LibraryID: "lib", RelativePath: "我为邪帝/第002话.zip", PageCount: 9},
	}

	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("len(works)=%d, want 1: %#v", len(works), works)
	}
	if got := unitLabels(works[0].Units); !reflect.DeepEqual(got, []string{"第001话", "第002话", "第010话"}) {
		t.Fatalf("labels=%#v", got)
	}
}

func TestBuildWorksFromComicListGroupsDecimalAndSuffixlessChineseChapterArchives(t *testing.T) {
	items := []store.ComicListItem{
		{
			ID:           "decimal",
			Title:        "女子学院的男生 第118.1话_你对秦枫是怎么想的？",
			Filename:     "第118.1话_你对秦枫是怎么想的？.cbz",
			LibraryID:    "lib",
			RelativePath: "女子学院的男生/第118.1话_你对秦枫是怎么想的？.cbz",
		},
		{
			ID:           "suffixless",
			Title:        "女子学院的男生 第119_新的开始",
			Filename:     "第119_新的开始.cbz",
			LibraryID:    "lib",
			RelativePath: "女子学院的男生/第119_新的开始.cbz",
		},
	}

	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("len(works)=%d, want one logical work: %#v", len(works), works)
	}
	if works[0].RootPath != "女子学院的男生" || works[0].ItemCount != 2 {
		t.Fatalf("work=%#v", works[0])
	}
	if got := unitLabels(works[0].Units); !reflect.DeepEqual(got, []string{
		"第118.1话_你对秦枫是怎么想的？",
		"第119_新的开始",
	}) {
		t.Fatalf("labels=%#v", got)
	}
}

func TestBuildWorksFromComicListGroupsRootLevelNamedArchives(t *testing.T) {
	items := []store.ComicListItem{
		{ID: "v10", Title: "作品 Vol.10", Filename: "作品 Vol.10.cbz", LibraryID: "lib", RelativePath: "作品 Vol.10.cbz"},
		{ID: "v2", Title: "作品 Vol.02", Filename: "作品 Vol.02.zip", LibraryID: "lib", RelativePath: "作品 Vol.02.zip"},
		{ID: "v1", Title: "作品 Vol.01", Filename: "作品 Vol.01.cbz", LibraryID: "lib", RelativePath: "作品 Vol.01.cbz"},
	}
	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 || works[0].Title != "作品" {
		t.Fatalf("works=%#v", works)
	}
	if got := unitLabels(works[0].Units); !reflect.DeepEqual(got, []string{"Vol.01", "Vol.02", "Vol.10"}) {
		t.Fatalf("labels=%#v", got)
	}
}

func TestBuildWorksFromComicListCollapsesSectionDirectories(t *testing.T) {
	items := []store.ComicListItem{
		{ID: "s2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "作品/第二季/第002话/"},
		{ID: "s1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第一季/第001话/"},
	}
	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 || works[0].RootPath != "作品" {
		t.Fatalf("works=%#v", works)
	}
	if got := unitLabels(works[0].Units); !reflect.DeepEqual(got, []string{"第一季 / 第001话", "第二季 / 第002话"}) {
		t.Fatalf("labels=%#v", got)
	}
}

func TestBuildWorksFromComicListRootPDFGetsFullTextUnit(t *testing.T) {
	items := []store.ComicListItem{{
		ID: "pdf1", Title: "大医凌然", Filename: "大医凌然.pdf",
		LibraryID: "lib", RelativePath: "大医凌然.pdf", PageCount: 88,
	}}

	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("len(works)=%d", len(works))
	}
	if works[0].Title != "大医凌然" {
		t.Fatalf("title=%q", works[0].Title)
	}
	if len(works[0].Units) != 1 || works[0].Units[0].DisplayLabel != "全文" {
		t.Fatalf("units=%#v", works[0].Units)
	}
}

func TestBuildWorksFromComicListIDsAreStableAcrossInputOrder(t *testing.T) {
	a := store.ComicListItem{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第001话/"}
	b := store.ComicListItem{ID: "c2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "作品/第002话/"}

	forward := BuildWorksFromComicList([]store.ComicListItem{a, b}, WorkBuildOptions{})
	reverse := BuildWorksFromComicList([]store.ComicListItem{b, a}, WorkBuildOptions{})
	if len(forward) != 1 || len(reverse) != 1 {
		t.Fatalf("unexpected work counts: %d/%d", len(forward), len(reverse))
	}
	if forward[0].ID != reverse[0].ID {
		t.Fatalf("work IDs differ: %q/%q", forward[0].ID, reverse[0].ID)
	}
	forwardIDs := map[string]string{}
	for _, unit := range forward[0].Units {
		forwardIDs[unit.ComicID] = unit.ID
	}
	for _, unit := range reverse[0].Units {
		if forwardIDs[unit.ComicID] != unit.ID {
			t.Fatalf("unit ID for %s differs: %q/%q", unit.ComicID, forwardIDs[unit.ComicID], unit.ID)
		}
	}
}

func TestBuildWorksFromComicListAssignsDeterministicSeriesHostBeforeMetadataOverlay(t *testing.T) {
	works := BuildWorksFromComicList([]store.ComicListItem{
		{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第001话/"},
		{ID: "c2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "作品/第002话/"},
	}, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("works=%#v", works)
	}
	wantSeriesID := stableSeriesID("series", "lib", "作品")
	if works[0].SeriesID != wantSeriesID || works[0].MetadataHostType != "series" || works[0].MetadataHostID != wantSeriesID {
		t.Fatalf("unstable initial metadata host: %#v", works[0])
	}
}

func TestBuildWorksFromComicListMergesSameNamedArchiveWithSiblingChapter(t *testing.T) {
	works := BuildWorksFromComicList([]store.ComicListItem{
		{ID: "archive", Title: "作品", Filename: "作品.zip", LibraryID: "lib", RelativePath: "作品/作品.zip", PageCount: 20},
		{ID: "chapter", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第001话/", PageCount: 10},
	}, WorkBuildOptions{})
	if len(works) != 1 || works[0].RootPath != "作品" || len(works[0].Units) != 2 {
		t.Fatalf("same-name archive was not merged with sibling chapter: %#v", works)
	}
}

func TestBuildWorksFromComicListUsesStandaloneScrapedTitleRatingsAndTimestamps(t *testing.T) {
	addedAt := "2025-01-02T03:04:05Z"
	updatedAt := "2026-08-05T06:07:08Z"
	externalRating := 9.2
	works := BuildWorksFromComicList([]store.ComicListItem{{
		ID: "pdf", Filename: "raw-name.pdf", RelativePath: "raw-name.pdf", LibraryID: "lib",
		Title: "刮削后的作品名", ExternalRating: &externalRating, ExternalRatingMax: 10,
		ExternalRatingSource: "bangumi", AddedAt: addedAt, UpdatedAt: updatedAt, SortOrder: 37,
	}}, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("works=%#v", works)
	}
	work := works[0]
	if work.Title != "刮削后的作品名" || work.ExternalRating == nil || *work.ExternalRating != 9.2 ||
		work.ExternalRatingMax == nil || *work.ExternalRatingMax != 10 || work.ExternalRatingSource != "bangumi" {
		t.Fatalf("standalone scraped metadata missing: %#v", work)
	}
	if work.AddedAt != addedAt || work.UpdatedAt != updatedAt || work.SortOrder != 37 {
		t.Fatalf("work sort metadata missing: %#v", work)
	}
}

func TestBuildWorksFromComicListAggregatesRepresentativeMetadataAndUserState(t *testing.T) {
	year := 2024
	rating := 5
	lastReadAt := "2026-08-05T13:00:00Z"
	items := []store.ComicListItem{
		{
			ID: "c2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "作品/第002话/",
			Author: "作者", Publisher: "出版社", Year: &year, Description: "简介", Language: "zh",
			Genre: "奇幻,冒险", MetadataSource: "bangumi", CoverAspectRatio: 0.72,
			IsFavorite: true, Rating: &rating, ReadingStatus: "reading",
			LastReadPage: 7, LastReadAt: &lastReadAt,
			Tags:       []store.ComicTagInfo{{Name: "热血", Color: "#f00"}},
			Categories: []store.ComicCategoryInfo{{ID: 1, Name: "国漫", Slug: "cn", Icon: "book"}},
		},
		{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第001话/"},
	}

	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("len(works)=%d", len(works))
	}
	work := works[0]
	if work.Author != "作者" || work.Publisher != "出版社" || work.Year == nil || *work.Year != 2024 ||
		work.Description != "简介" || work.Language != "zh" || work.Genre != "奇幻,冒险" ||
		work.MetadataSource != "bangumi" || work.CoverAspectRatio != 0.72 {
		t.Fatalf("representative metadata not aggregated: %#v", work)
	}
	if !work.IsFavorite || work.Rating == nil || *work.Rating != 5 || work.ReadingStatus != "reading" {
		t.Fatalf("user state not aggregated: %#v", work)
	}
	if !reflect.DeepEqual(work.Tags, []store.ComicTagInfo{{Name: "热血", Color: "#f00"}}) ||
		!reflect.DeepEqual(work.Categories, []store.ComicCategoryInfo{{ID: 1, Name: "国漫", Slug: "cn", Icon: "book"}}) {
		t.Fatalf("tags/categories not aggregated: %#v / %#v", work.Tags, work.Categories)
	}
	if work.ContinueComicID != "c2" || work.ContinueUnitID == "" || work.ContinuePage != 7 {
		t.Fatalf("continue target=%q/%q/%d", work.ContinueComicID, work.ContinueUnitID, work.ContinuePage)
	}
}

func TestBuildWorksFromComicListMapsArchiveContinuePageToInternalUnit(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "作品.zip")
	createTestZip(t, fp, []string{
		"cover.jpg",
		"第001话/001.jpg",
		"第001话/002.jpg",
		"第002话/001.jpg",
		"第002话/002.jpg",
		"第002话/003.jpg",
	})
	lastReadAt := "2026-08-05T14:00:00Z"
	items := []store.ComicListItem{{
		ID: "archive", Title: "作品", Filename: "作品.zip", RelativePath: "作品.zip",
		PageCount: 6, LastReadPage: 4, LastReadAt: &lastReadAt,
	}}
	t.Setenv("COMICS_DIR", filepath.Dir(fp))

	works := BuildWorksFromComicList(items, WorkBuildOptions{ProbeInternalArchive: true})
	if len(works) != 1 || len(works[0].Units) != 2 {
		t.Fatalf("works=%#v", works)
	}
	work := works[0]
	if work.ContinueComicID != "archive" || work.ContinueUnitID != work.Units[1].ID || work.ContinuePage != 4 {
		t.Fatalf("continue target=%q/%q/%d, units=%#v", work.ContinueComicID, work.ContinueUnitID, work.ContinuePage, work.Units)
	}
	if work.Units[1].LastReadPage != 1 || work.Units[1].LastReadAt == nil {
		t.Fatalf("unit progress not projected: %#v", work.Units[1])
	}
}

func TestApplySeriesMetadataUsesPersistentSeriesAsWorkHost(t *testing.T) {
	works := BuildWorksFromComicList([]store.ComicListItem{
		{ID: "c1", Title: "第001话", Filename: "第001话/", LibraryID: "lib", RelativePath: "作品/第001话/"},
		{ID: "c2", Title: "第002话", Filename: "第002话/", LibraryID: "lib", RelativePath: "作品/第002话/"},
	}, WorkBuildOptions{})
	year := 2025
	externalRating := 8.8
	externalRatingMax := 10.0
	ApplySeriesMetadata(works, []store.SeriesSummary{{
		ID: "ser-1", LibraryID: "lib", RootRelativePath: "作品", Title: "作品（官方标题）",
		CoverComicID: "c2", CoverURL: "/api/comics/series_ser-1/thumbnail",
		Author: "系列作者", Publisher: "系列出版社", Year: &year, Description: "系列简介",
		Language: "zh", Genre: "奇幻", Status: "ongoing",
		ExternalRating: &externalRating, ExternalRatingMax: &externalRatingMax, ExternalRatingSource: "bangumi",
		Tags: []store.Tag{{Name: "系列标签", Color: "#abc"}},
	}})

	work := works[0]
	if work.SeriesID != "ser-1" || work.MetadataHostType != "series" || work.MetadataHostID != "ser-1" {
		t.Fatalf("series host not linked: %#v", work)
	}
	if work.Title != "作品（官方标题）" || work.CoverComicID != "c2" ||
		work.CoverURL != "/api/comics/series_ser-1/thumbnail" || work.Author != "系列作者" ||
		work.Publisher != "系列出版社" || work.Year == nil || *work.Year != 2025 ||
		work.Description != "系列简介" || work.Status != "ongoing" {
		t.Fatalf("series metadata not applied: %#v", work)
	}
	if len(work.Tags) != 1 || work.Tags[0].Name != "系列标签" {
		t.Fatalf("series tags not applied: %#v", work.Tags)
	}
}

func TestApplySeriesMetadataPreservesComicRatingAndMergesSeriesCoverAndUpdatedAt(t *testing.T) {
	rating := 8.6
	ratingMax := 10.0
	works := []Work{{
		ID: "work", LibraryID: "lib", RootPath: "作品", Title: "作品",
		UpdatedAt:      "2026-08-05T01:00:00Z",
		ExternalRating: &rating, ExternalRatingMax: &ratingMax, ExternalRatingSource: "comic",
		CoverURL: "/api/comics/c1/thumbnail", CoverAspectRatio: 0.7,
	}}
	ApplySeriesMetadata(works, []store.SeriesSummary{{
		ID: "series", LibraryID: "lib", RootRelativePath: "作品",
		UpdatedAt:    "2026-08-05T02:00:00Z",
		CoverComicID: "c2", CoverURL: "/api/comics/series_series/thumbnail", CoverAspectRatio: 0.66,
	}})
	got := works[0]
	if got.ExternalRating == nil || *got.ExternalRating != rating ||
		got.ExternalRatingMax == nil || *got.ExternalRatingMax != ratingMax ||
		got.ExternalRatingSource != "comic" {
		t.Fatalf("empty Series rating erased Comic rating: %#v", got)
	}
	if got.UpdatedAt != "2026-08-05T02:00:00Z" ||
		got.CoverURL != "/api/comics/series_series/thumbnail" ||
		got.CoverComicID != "c2" || got.CoverAspectRatio != 0.66 {
		t.Fatalf("Series cover/timestamp priority not applied: %#v", got)
	}
}

func TestNaturalLessSortsChineseNumberedWorkTitles(t *testing.T) {
	titles := []string{"作品十", "作品二", "作品十一", "作品一"}
	sort.Slice(titles, func(i, j int) bool { return NaturalLess(titles[i], titles[j]) })
	want := []string{"作品一", "作品二", "作品十", "作品十一"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("titles=%#v want=%#v", titles, want)
	}
}

func TestStandaloneWorkUsesPhysicalComicAsMetadataHost(t *testing.T) {
	works := BuildWorksFromComicList([]store.ComicListItem{{
		ID: "zip-1", Title: "作品", Filename: "作品.zip", LibraryID: "lib", RelativePath: "作品.zip",
	}}, WorkBuildOptions{})
	if len(works) != 1 || works[0].MetadataHostType != "comic" || works[0].MetadataHostID != "zip-1" {
		t.Fatalf("standalone host=%#v", works)
	}
}

func TestWorkReadingStatusRequiresEveryPhysicalUnitToFinish(t *testing.T) {
	finishedAt := "2026-08-06T01:00:00Z"
	tests := []struct {
		name   string
		items  []store.ComicListItem
		status string
	}{
		{
			name: "all finished",
			items: []store.ComicListItem{
				{ID: "a1", LibraryID: "lib", RelativePath: "作品/第一话.cbz", PageCount: 10, LastReadPage: 9, LastReadAt: &finishedAt, ReadingStatus: "finished"},
				{ID: "a2", LibraryID: "lib", RelativePath: "作品/第二话.cbz", PageCount: 10, LastReadPage: 9, LastReadAt: &finishedAt, ReadingStatus: "finished"},
			},
			status: "finished",
		},
		{
			name: "partially started",
			items: []store.ComicListItem{
				{ID: "b1", LibraryID: "lib", RelativePath: "作品/第一话.cbz", PageCount: 10, LastReadPage: 9, LastReadAt: &finishedAt, ReadingStatus: "finished"},
				{ID: "b2", LibraryID: "lib", RelativePath: "作品/第二话.cbz", PageCount: 10},
			},
			status: "reading",
		},
		{
			name: "never started",
			items: []store.ComicListItem{
				{ID: "c1", LibraryID: "lib", RelativePath: "作品/第一话.cbz", PageCount: 10},
				{ID: "c2", LibraryID: "lib", RelativePath: "作品/第二话.cbz", PageCount: 10},
			},
			status: "unread",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			works := BuildWorksFromComicList(test.items, WorkBuildOptions{})
			if len(works) != 1 || works[0].ReadingStatus != test.status {
				t.Fatalf("works=%#v want status=%q", works, test.status)
			}
		})
	}
}

func TestPhysicalComicIDsDeduplicatesInternalUnits(t *testing.T) {
	work := Work{Units: []WorkUnit{
		{ComicID: "archive"},
		{ComicID: "archive"},
		{ComicID: "chapter"},
	}}
	got := PhysicalComicIDs(work)
	want := []string{"archive", "chapter"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestProbeArchiveChapterUnitsAtArchiveRoot(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "败犬女主太多了.zip")
	createTestZip(t, fp, []string{
		"cover.jpg",
		"第001卷 序章/001.jpg",
		"第001卷 序章/002.jpg",
		"第002卷 日常/001.jpg",
	})

	units := ProbeArchiveChapterUnits(fp)
	assertArchiveUnits(t, units, []archiveUnit{
		{Title: "第001卷 序章", DisplayLabel: "第001卷 序章", Path: "第001卷 序章", StartPage: 1, PageCount: 2, CoverPage: 1},
		{Title: "第002卷 日常", DisplayLabel: "第002卷 日常", Path: "第002卷 日常", StartPage: 3, PageCount: 1, CoverPage: 3},
	})
}

func TestProbeArchiveChapterUnitsBelowSingleWorkRoot(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "恶魔X天使 不能友好相处.zip")
	createTestZip(t, fp, []string{
		"恶魔X天使 不能友好相处/cover.jpg",
		"恶魔X天使 不能友好相处/第000话 预告/001.jpg",
		"恶魔X天使 不能友好相处/第001话 其实是恶魔/001.jpg",
		"恶魔X天使 不能友好相处/第001话 其实是恶魔/002.jpg",
	})

	units := ProbeArchiveChapterUnits(fp)
	assertArchiveUnits(t, units, []archiveUnit{
		{Title: "第000话 预告", DisplayLabel: "第000话 预告", Path: "恶魔X天使 不能友好相处/第000话 预告", StartPage: 1, PageCount: 1, CoverPage: 1},
		{Title: "第001话 其实是恶魔", DisplayLabel: "第001话 其实是恶魔", Path: "恶魔X天使 不能友好相处/第001话 其实是恶魔", StartPage: 2, PageCount: 2, CoverPage: 2},
	})
}

func TestProbeArchiveChapterUnitsIgnoresNestedImageDirectoriesWithinChapter(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "nested.cbz")
	createTestZip(t, fp, []string{
		"作品/第001话/images/001.jpg",
		"作品/第001话/images/002.jpg",
		"作品/第002话/images/001.jpg",
	})

	units := ProbeArchiveChapterUnits(fp)
	assertArchiveUnits(t, units, []archiveUnit{
		{Title: "第001话", DisplayLabel: "第001话", Path: "作品/第001话", StartPage: 0, PageCount: 2, CoverPage: 0},
		{Title: "第002话", DisplayLabel: "第002话", Path: "作品/第002话", StartPage: 2, PageCount: 1, CoverPage: 2},
	})
}

func TestProbeArchiveChapterUnitsSupportsWorkSectionChapterHierarchyAndChapterCovers(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "hierarchy.cbz")
	createTestZip(t, fp, []string{
		"作品/cover.jpg",
		"作品/第一季/第十话/001.jpg",
		"作品/第一季/第十话/002.jpg",
		"作品/第一季/第十话/cover.jpg",
		"作品/第二季/第二话/001.jpg",
		"作品/第二季/第二话/cover.jpg",
	})

	units := ProbeArchiveChapterUnits(fp)
	if len(units) != 2 {
		t.Fatalf("units=%#v", units)
	}
	if units[0].Title != "第十话" || units[0].DisplayLabel != "第一季 / 第十话" ||
		units[0].Path != "作品/第一季/第十话" || units[0].PageCount != 3 || units[0].CoverPage < units[0].StartPage {
		t.Fatalf("first hierarchical unit=%#v", units[0])
	}
	if units[1].Title != "第二话" || units[1].DisplayLabel != "第二季 / 第二话" ||
		units[1].Path != "作品/第二季/第二话" || units[1].PageCount != 2 || units[1].CoverPage < units[1].StartPage {
		t.Fatalf("second hierarchical unit=%#v", units[1])
	}
}

func TestBuildWorksArchiveChapterCoverCountsAsReadablePageAndRootCoverOnlyCountsAtWorkLevel(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "cover-pages.zip")
	createTestZip(t, fp, []string{
		"作品/cover.jpg",
		"作品/第001话/001.jpg",
		"作品/第001话/002.jpg",
		"作品/第001话/cover.jpg",
	})
	works := BuildWorksFromComicList([]store.ComicListItem{{
		ID: "archive", Title: "作品", Filename: "cover-pages.zip", RelativePath: "cover-pages.zip",
	}}, WorkBuildOptions{
		ProbeInternalArchive: true,
		ResolveComicPath: func(store.ComicListItem) (string, bool) {
			return fp, true
		},
	})
	if len(works) != 1 || len(works[0].Units) != 1 {
		t.Fatalf("works=%#v", works)
	}
	unit := works[0].Units[0]
	if unit.PageCount != 3 {
		t.Fatalf("chapter cover must count as a readable page, unit=%#v", unit)
	}
	if works[0].PageCount != 4 {
		t.Fatalf("work page count must include root cover without subtracting chapter cover: %#v", works[0])
	}
	if !strings.Contains(unit.CoverURL, "/page/") {
		t.Fatalf("chapter cover did not become unit cover: %q", unit.CoverURL)
	}
}

func TestNaturalLessSortsChineseNumeralsNumerically(t *testing.T) {
	labels := []string{"第十话", "第二话", "第十一话", "第一话"}
	sort.Slice(labels, func(i, j int) bool { return naturalLess(labels[i], labels[j]) })
	want := []string{"第一话", "第二话", "第十话", "第十一话"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels=%#v, want %#v", labels, want)
	}
}

func TestResolveWorkComicPathPrefersComicLibraryOverLegacyComicsDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "work-path.db")
	if err := store.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.CloseDB)

	libraryRoot := t.TempDir()
	fallbackRoot := t.TempDir()
	library := &model.Library{
		ID: "lib-path", Name: "Path", Type: "comic", RootPath: libraryRoot,
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	createTestZip(t, filepath.Join(libraryRoot, "作品.zip"), []string{
		"第001话/001.jpg", "第002话/001.jpg",
	})
	createTestZip(t, filepath.Join(fallbackRoot, "作品.zip"), []string{
		"普通目录/001.jpg", "普通目录/002.jpg",
	})
	if err := store.BulkCreateComicsWithSource([]struct {
		ID       string
		Filename string
		Title    string
		FileSize int64
	}{{ID: "path-comic", Filename: "作品.zip", Title: "作品"}}, map[string]string{"path-comic": "comics"}, map[string]string{"path-comic": library.ID}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMICS_DIR", fallbackRoot)

	works := BuildWorksFromComicList([]store.ComicListItem{{
		ID: "path-comic", Title: "作品", Filename: "作品.zip", RelativePath: "作品.zip", LibraryID: library.ID,
	}}, WorkBuildOptions{ProbeInternalArchive: true})
	if len(works) != 1 || len(works[0].Units) != 2 {
		t.Fatalf("resolver used legacy COMICS_DIR instead of comic/library identity: %#v", works)
	}
}

func TestProbeArchiveChapterUnitsDoesNotTreatSingleWorkFolderAsChapter(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "ordinary.cbz")
	createTestZip(t, fp, []string{
		"作品/001.jpg",
		"作品/002.jpg",
	})
	if units := ProbeArchiveChapterUnits(fp); len(units) != 0 {
		t.Fatalf("ordinary single-folder CBZ was treated as chapters: %#v", units)
	}
}

func unitLabels(units []WorkUnit) []string {
	result := make([]string, len(units))
	for i, unit := range units {
		result[i] = unit.DisplayLabel
	}
	return result
}

func assertArchiveUnits(t *testing.T, got, want []archiveUnit) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("archive units=%#v, want %#v", got, want)
	}
}

func TestWorkTimestampAggregationUsesEarliestAddedLatestUpdatedAndFirstUnitCustomOrder(t *testing.T) {
	works := BuildWorksFromComicList([]store.ComicListItem{
		{ID: "c2", Title: "第二话", LibraryID: "lib", RelativePath: "作品/第二话/", AddedAt: "2025-02-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", SortOrder: 20},
		{ID: "c1", Title: "第一话", LibraryID: "lib", RelativePath: "作品/第一话/", AddedAt: "2025-01-01T00:00:00Z", UpdatedAt: "2026-02-01T00:00:00Z", SortOrder: 10},
	}, WorkBuildOptions{})
	work := works[0]
	if work.AddedAt != "2025-01-01T00:00:00Z" || work.UpdatedAt != "2026-02-01T00:00:00Z" || work.SortOrder != 10 {
		t.Fatalf("timestamp/custom aggregation=%#v", work)
	}
}

var _ = time.RFC3339

func createTestZip(t *testing.T, fp string, names []string) {
	t.Helper()
	f, err := os.Create(fp)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.RGBA{255, 0, 0, 255})
		if err := jpeg.Encode(w, img, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
