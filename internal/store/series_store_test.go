package store

import (
	"os"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
)

func TestReplaceDetectedSeriesAndCollapseShelf(t *testing.T) {
	originalBasePath := os.Getenv("BASE_PATH")
	t.Cleanup(func() { _ = os.Setenv("BASE_PATH", originalBasePath) })
	if err := os.Setenv("BASE_PATH", "/reader"); err != nil {
		t.Fatal(err)
	}

	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	library := &model.Library{
		ID:            "series-library",
		Name:          "Comic Library",
		Type:          "comic",
		RootPath:      t.TempDir(),
		Enabled:       true,
		DefaultAccess: "public",
		ScanEnabled:   true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	insert := func(id, relativePath, title string) {
		t.Helper()
		if _, err := DB().Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "pageCount", "fileSize", "type", "libraryId", "relativePath")
			VALUES (?, ?, ?, 20, 1024, 'comic', ?, ?)
		`, id, relativePath, title, library.ID, relativePath); err != nil {
			t.Fatalf("insert comic %s: %v", id, err)
		}
	}
	insert("volume-1", "作品/作品 01.pdf", "作品 01")
	insert("volume-2", "作品/作品 02.pdf", "作品 02")
	insert("standalone", "单本.pdf", "单本")

	detected := []DetectedSeries{{
		ID:               "ser-test",
		LibraryID:        library.ID,
		RootRelativePath: "作品",
		Title:            "作品",
		SortTitle:        "作品",
		CoverComicID:     "volume-1",
		Items: []DetectedSeriesItem{
			{ComicID: "volume-1", SortIndex: 0, DisplayLabel: "01"},
			{ComicID: "volume-2", SortIndex: 1, DisplayLabel: "02"},
		},
	}}
	if err := ReplaceDetectedSeries(library.ID, detected); err != nil {
		t.Fatalf("ReplaceDetectedSeries failed: %v", err)
	}

	flat, err := GetAllComics(ComicListOptions{LibraryIDs: []string{library.ID}})
	if err != nil {
		t.Fatalf("GetAllComics failed: %v", err)
	}
	collapsed, err := CollapseComicListIntoSeries(flat.Comics, "")
	if err != nil {
		t.Fatalf("CollapseComicListIntoSeries failed: %v", err)
	}
	if len(collapsed) != 2 {
		t.Fatalf("collapsed count = %d, want series + standalone", len(collapsed))
	}
	foundSeries, foundStandalone := false, false
	for _, item := range collapsed {
		switch item.ID {
		case SeriesShelfIDPrefix + "ser-test":
			foundSeries = item.PageCount == 2 && item.Title == "作品"
		case "standalone":
			foundStandalone = true
		}
	}
	if !foundSeries || !foundStandalone {
		t.Fatalf("unexpected collapsed shelf: %#v", collapsed)
	}

	seen := make(map[string]bool)
	for page := 1; page <= 2; page++ {
		mixed, err := GetAllComics(ComicListOptions{
			LibraryIDs: []string{library.ID},
			SeriesView: true,
			SortBy:     "title",
			SortOrder:  "asc",
			Page:       page,
			PageSize:   1,
		})
		if err != nil {
			t.Fatalf("GetAllComics series view page %d failed: %v", page, err)
		}
		if mixed.Total != 2 || mixed.TotalPages != 2 || len(mixed.Comics) != 1 {
			t.Fatalf("series view page %d = %#v", page, mixed)
		}
		seen[mixed.Comics[0].ID] = true
	}
	if !seen[SeriesShelfIDPrefix+"ser-test"] || !seen["standalone"] || seen["volume-1"] || seen["volume-2"] {
		t.Fatalf("series view ids = %#v", seen)
	}

	detail, err := GetSeriesDetail("ser-test", "")
	if err != nil {
		t.Fatalf("GetSeriesDetail failed: %v", err)
	}
	if detail == nil || len(detail.Unsectioned) != 2 || detail.Series.ItemCount != 2 {
		t.Fatalf("unexpected series detail: %#v", detail)
	}
	if detail.Series.CoverURL != "/reader/api/comics/volume-1/thumbnail" {
		t.Fatalf("series cover URL = %q, want Base Path prefixed URL", detail.Series.CoverURL)
	}
}

func TestSetSeriesItemStructureRejectsForeignSection(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	library := &model.Library{ID: "structure-library", Name: "Structure", Type: "comic", RootPath: t.TempDir(), Enabled: true, DefaultAccess: "public", ScanEnabled: true}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath") VALUES ('item', 'work/01.pdf', '01', 'comic', ?, 'work/01.pdf')`, library.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`INSERT INTO "ComicSeries" ("id", "libraryId", "rootRelativePath", "title", "sortTitle") VALUES ('series-a', ?, 'work', 'work', 'work'), ('series-b', ?, 'other', 'other', 'other')`, library.ID, library.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`INSERT INTO "ComicSeriesSection" ("id", "seriesId", "title", "relativePath") VALUES ('foreign-section', 'series-b', '第一季', 'other/第一季')`); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`INSERT INTO "ComicSeriesItem" ("seriesId", "comicId") VALUES ('series-a', 'item')`); err != nil {
		t.Fatal(err)
	}

	if err := SetSeriesItemStructure("series-a", "item", "foreign-section", 0); err == nil {
		t.Fatal("foreign section assignment succeeded, want rejection")
	}
}

func TestSeriesMetadataAndTagsSurviveDirectoryRefresh(t *testing.T) {
	setupTestDB(t)

	library := &model.Library{
		ID:            "metadata-series-library",
		Name:          "Metadata Series",
		Type:          "comic",
		RootPath:      t.TempDir(),
		Enabled:       true,
		DefaultAccess: "public",
		ScanEnabled:   true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	for index, id := range []string{"metadata-volume-1", "metadata-volume-2"} {
		if _, err := DB().Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
			VALUES (?, ?, ?, 'comic', ?, ?)
		`, id, "作品/卷.pdf", id, library.ID, "作品/卷"+string(rune('1'+index))+".pdf"); err != nil {
			t.Fatal(err)
		}
	}
	detected := []DetectedSeries{{
		ID:               "ser-metadata",
		LibraryID:        library.ID,
		RootRelativePath: "作品",
		Title:            "作品",
		SortTitle:        "作品",
		CoverComicID:     "metadata-volume-1",
		Items: []DetectedSeriesItem{
			{ComicID: "metadata-volume-1", SortIndex: 0},
			{ComicID: "metadata-volume-2", SortIndex: 1},
		},
	}}
	if err := ReplaceDetectedSeries(library.ID, detected); err != nil {
		t.Fatal(err)
	}

	title := "刮削后的作品名"
	author := "作者"
	description := "简介"
	publisher := "出版社"
	language := "zh"
	genre := "冒险,奇幻"
	coverURL := "https://example.com/cover.jpg"
	year := 2026
	rating := 8.5
	ratingMax := 10.0
	ratingSource := "test"
	ratingAt := time.Now().UTC().Truncate(time.Second)
	metadataLocked := true
	if err := UpdateSeriesMetadata("ser-metadata", SeriesMetadataUpdate{
		Title:                   &title,
		CoverURL:                &coverURL,
		Author:                  &author,
		Description:             &description,
		Year:                    &year,
		Publisher:               &publisher,
		Language:                &language,
		Genre:                   &genre,
		ExternalRating:          &rating,
		ExternalRatingMax:       &ratingMax,
		ExternalRatingSource:    &ratingSource,
		ExternalRatingUpdatedAt: &ratingAt,
		MetadataLocked:          &metadataLocked,
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetSeriesTags("ser-metadata", []string{"冒险", "奇幻"}); err != nil {
		t.Fatal(err)
	}
	total, synced, tagsCount, err := SyncSeriesTagsToItems("ser-metadata")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || synced != 2 || tagsCount != 2 {
		t.Fatalf("tag sync = total:%d synced:%d tags:%d", total, synced, tagsCount)
	}

	detected[0].Title = "扫描识别标题"
	detected[0].SortTitle = "扫描识别标题"
	detected[0].Items[0].DisplayLabel = "已刷新结构"
	if err := ReplaceDetectedSeries(library.ID, detected); err != nil {
		t.Fatal(err)
	}
	detail, err := GetSeriesDetail("ser-metadata", "")
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("series metadata disappeared after directory refresh")
	}
	got := detail.Series
	if got.Title != title || got.Author != author || got.Description != description || got.Publisher != publisher ||
		got.Language != language || got.Genre != genre || got.Year == nil || *got.Year != year {
		t.Fatalf("unexpected series metadata after refresh: %#v", got)
	}
	if got.ExternalRating == nil || *got.ExternalRating != rating || got.ExternalRatingMax == nil || *got.ExternalRatingMax != ratingMax {
		t.Fatalf("unexpected external rating: %#v", got)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("series tags = %#v, want 2", got.Tags)
	}
	if !got.MetadataLocked || got.ManualLocked {
		t.Fatalf("metadata and structure locks = metadata:%v manual:%v", got.MetadataLocked, got.ManualLocked)
	}
	if len(detail.Unsectioned) != 2 || detail.Unsectioned[0].DisplayLabel != "已刷新结构" {
		t.Fatalf("directory structure was not refreshed: %#v", detail.Unsectioned)
	}
	if got.CoverURL != "/api/comics/series_ser-metadata/thumbnail" {
		t.Fatalf("series cover URL = %q", got.CoverURL)
	}
	for _, comicID := range []string{"metadata-volume-1", "metadata-volume-2"} {
		var count int
		err := DB().QueryRow(`SELECT COUNT(*) FROM "ComicTag" WHERE "comicId" = ?`, comicID).Scan(&count)
		if err != nil || count != 2 {
			t.Fatalf("comic %s tag count = %d, err=%v", comicID, count, err)
		}
	}

	if err := ReplaceDetectedSeries(library.ID, nil); err != nil {
		t.Fatalf("remove missing detected series: %v", err)
	}
	removed, err := GetSeriesDetail("ser-metadata", "")
	if err != nil {
		t.Fatalf("load removed series: %v", err)
	}
	if removed != nil {
		t.Fatalf("metadata lock kept a stale directory series: %#v", removed.Series)
	}
}

func TestReplaceDetectedSeriesSeedsNewSeriesMetadataFromRepresentativeComic(t *testing.T) {
	setupTestDB(t)

	library := &model.Library{
		ID: "series-seed-library", Name: "Series Seed", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, DefaultAccess: "public", ScanEnabled: true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	ratingAt := time.Date(2026, 8, 5, 1, 2, 3, 0, time.UTC)
	if _, err := DB().Exec(`
		INSERT INTO "Comic" (
			"id", "filename", "title", "type", "libraryId", "relativePath",
			"author", "description", "year", "publisher", "language", "genre",
			"externalRating", "externalRatingMax", "externalRatingSource", "externalRatingUpdatedAt"
		) VALUES (?, ?, ?, 'comic', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "series-seed-1", "第一话.cbz", "第一话", library.ID, "作品/第一话.cbz",
		"作品作者", "作品简介", 2026, "作品出版社", "zh-CN", "奇幻",
		9.2, 10.0, "bangumi", ratingAt); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES ('series-seed-2', '第二话.cbz', '第二话', 'comic', ?, '作品/第二话.cbz')
	`, library.ID); err != nil {
		t.Fatal(err)
	}
	if err := AddTagsToComic("series-seed-1", []string{"冒险", "连载中"}); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceDetectedSeries(library.ID, []DetectedSeries{{
		ID:               "series-seed-work",
		LibraryID:        library.ID,
		RootRelativePath: "作品",
		Title:            "作品",
		SortTitle:        "作品",
		CoverComicID:     "series-seed-1",
		Items: []DetectedSeriesItem{
			{ComicID: "series-seed-1", SortIndex: 0, DisplayLabel: "第一话"},
			{ComicID: "series-seed-2", SortIndex: 1, DisplayLabel: "第二话"},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	detail, err := GetSeriesDetail("series-seed-work", "")
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("newly detected series was not persisted")
	}
	got := detail.Series
	if got.Title != "作品" || got.Author != "作品作者" || got.Description != "作品简介" ||
		got.Publisher != "作品出版社" || got.Language != "zh-CN" || got.Genre != "奇幻" ||
		got.Year == nil || *got.Year != 2026 {
		t.Fatalf("representative metadata was not seeded without replacing Work title: %#v", got)
	}
	if got.ExternalRating == nil || *got.ExternalRating != 9.2 ||
		got.ExternalRatingMax == nil || *got.ExternalRatingMax != 10 ||
		got.ExternalRatingSource != "bangumi" || got.ExternalRatingUpdatedAt == nil ||
		!got.ExternalRatingUpdatedAt.Equal(ratingAt) {
		t.Fatalf("representative external rating was not seeded: %#v", got)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("representative tags were not seeded: %#v", got.Tags)
	}
}

func TestUpdateSeriesMetadataUsesSubsecondUpdatedAtForWorkCacheInvalidation(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "series-updated-at-library", Name: "Series UpdatedAt", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, DefaultAccess: "public", ScanEnabled: true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"series-updated-at-1", "series-updated-at-2"} {
		if _, err := DB().Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
			VALUES (?, ?, ?, 'comic', ?, ?)
		`, id, id+".cbz", id, library.ID, "作品/"+id+".cbz"); err != nil {
			t.Fatal(err)
		}
	}
	if err := ReplaceDetectedSeries(library.ID, []DetectedSeries{{
		ID: "series-updated-at-work", LibraryID: library.ID, RootRelativePath: "作品",
		Title: "作品", SortTitle: "作品", CoverComicID: "series-updated-at-1",
		Items: []DetectedSeriesItem{
			{ComicID: "series-updated-at-1", SortIndex: 0},
			{ComicID: "series-updated-at-2", SortIndex: 1},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	firstAuthor := "甲乙"
	if err := UpdateSeriesMetadata("series-updated-at-work", SeriesMetadataUpdate{Author: &firstAuthor}); err != nil {
		t.Fatal(err)
	}
	var firstUpdatedAt string
	if err := DB().QueryRow(`SELECT CAST("updatedAt" AS TEXT) FROM "ComicSeries" WHERE "id" = 'series-updated-at-work'`).Scan(&firstUpdatedAt); err != nil {
		t.Fatal(err)
	}
	secondAuthor := "丙丁"
	if err := UpdateSeriesMetadata("series-updated-at-work", SeriesMetadataUpdate{Author: &secondAuthor}); err != nil {
		t.Fatal(err)
	}
	var secondUpdatedAt string
	if err := DB().QueryRow(`SELECT CAST("updatedAt" AS TEXT) FROM "ComicSeries" WHERE "id" = 'series-updated-at-work'`).Scan(&secondUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if firstUpdatedAt == secondUpdatedAt {
		t.Fatalf("same-second metadata updates reused updatedAt %q", firstUpdatedAt)
	}
}

func TestUpdateSeriesUsesSubsecondUpdatedAtForWorkCacheInvalidation(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "series-structure-updated-at-library", Name: "Series Structure UpdatedAt",
		Type: "comic", RootPath: t.TempDir(), Enabled: true, DefaultAccess: "public", ScanEnabled: true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "ComicSeries" ("id", "libraryId", "rootRelativePath", "title", "sortTitle")
		VALUES ('series-structure-updated-at-work', ?, '作品', '作品', '作品')
	`, library.ID); err != nil {
		t.Fatal(err)
	}

	if err := UpdateSeries("series-structure-updated-at-work", "作品甲", "", nil); err != nil {
		t.Fatal(err)
	}
	var firstUpdatedAt string
	if err := DB().QueryRow(`SELECT CAST("updatedAt" AS TEXT) FROM "ComicSeries" WHERE "id" = 'series-structure-updated-at-work'`).Scan(&firstUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSeries("series-structure-updated-at-work", "作品乙", "", nil); err != nil {
		t.Fatal(err)
	}
	var secondUpdatedAt string
	if err := DB().QueryRow(`SELECT CAST("updatedAt" AS TEXT) FROM "ComicSeries" WHERE "id" = 'series-structure-updated-at-work'`).Scan(&secondUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if firstUpdatedAt == secondUpdatedAt {
		t.Fatalf("same-second series updates reused updatedAt %q", firstUpdatedAt)
	}
}

func TestScrapedSingleChapterMetadataMigratesWhenSecondChapterCreatesSeries(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "series-host-transition-library", Name: "Host Transition", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, DefaultAccess: "public", ScanEnabled: true,
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "Comic" (
			"id", "filename", "title", "type", "libraryId", "relativePath",
			"author", "description", "publisher", "language", "genre", "metadataSource",
			"coverImageUrl", "coverAspectRatio", "externalRating", "externalRatingMax", "externalRatingSource"
		) VALUES (
			'host-transition-1', '第一话.cbz', '刮削后的作品标题', 'comic', ?, '作品/第一话.cbz',
			'作品作者', '作品简介', '出版社', 'zh-CN', '奇幻', 'bangumi',
			'https://example.test/work-cover.jpg', 0.68, 9.1, 10, 'bangumi'
		)
	`, library.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES ('host-transition-2', '第二话.cbz', '第二话', 'comic', ?, '作品/第二话.cbz')
	`, library.ID); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceDetectedSeries(library.ID, []DetectedSeries{{
		ID: "host-transition-series", LibraryID: library.ID, RootRelativePath: "作品",
		Title: "作品", SortTitle: "作品", CoverComicID: "host-transition-1",
		Items: []DetectedSeriesItem{
			{ComicID: "host-transition-1", SortIndex: 0, DisplayLabel: "第一话"},
			{ComicID: "host-transition-2", SortIndex: 1, DisplayLabel: "第二话"},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	var title, coverURL, metadataSource string
	var coverAspectRatio float64
	var metadataLocked bool
	if err := DB().QueryRow(`
		SELECT "title", "coverUrl", "metadataSource", "coverAspectRatio", "metadataLocked"
		FROM "ComicSeries" WHERE "id" = 'host-transition-series'
	`).Scan(&title, &coverURL, &metadataSource, &coverAspectRatio, &metadataLocked); err != nil {
		t.Fatal(err)
	}
	if title != "刮削后的作品标题" || coverURL != "https://example.test/work-cover.jpg" ||
		metadataSource != "bangumi" || coverAspectRatio != 0.68 || !metadataLocked {
		t.Fatalf("Comic -> Series metadata migration incomplete: title=%q cover=%q source=%q ratio=%v locked=%v",
			title, coverURL, metadataSource, coverAspectRatio, metadataLocked)
	}
}
