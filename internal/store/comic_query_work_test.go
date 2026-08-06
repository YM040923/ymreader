package store

import (
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
)

func TestGetAllComicsReturnsWorkRequiredIdentityMetadata(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "work-query-library", Name: "Work Query", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	addedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 5, 6, 7, 8, 0, time.UTC)
	if _, err := DB().Exec(`
		INSERT INTO "Comic" (
			"id", "filename", "title", "type", "libraryId", "relativePath",
			"coverImageUrl", "addedAt", "updatedAt"
		) VALUES (?, ?, ?, 'comic', ?, ?, ?, ?, ?)
	`, "work-query-comic", "chapter.cbz", "Chapter", library.ID, "作品/第一话.cbz",
		"https://example.test/cover.jpg", addedAt, updatedAt); err != nil {
		t.Fatal(err)
	}

	result, err := GetAllComics(ComicListOptions{ContentType: "comic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Comics) != 1 {
		t.Fatalf("comics=%#v", result.Comics)
	}
	comic := result.Comics[0]
	if comic.LibraryID != library.ID || comic.RelativePath != "作品/第一话.cbz" ||
		comic.CoverImageURL != "https://example.test/cover.jpg" ||
		comic.AddedAt != addedAt.Format(time.RFC3339Nano) ||
		comic.UpdatedAt != updatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("work-required query fields missing: %#v", comic)
	}
}

func TestGetWorkSourceFingerprintChangesWithComicMetadataAndRelations(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "work-fingerprint-library", Name: "Fingerprint", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "updatedAt")
		VALUES ('fingerprint-comic', '作品.pdf', '作品', 'comic', ?, '作品.pdf', ?)
	`, library.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	first, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		UPDATE "Comic" SET "author" = '新作者', "updatedAt" = ? WHERE "id" = 'fingerprint-comic'
	`, time.Now().UTC().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	second, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("comic metadata update did not change fingerprint: %q", first)
	}
	if err := AddTagsToComic("fingerprint-comic", []string{"新标签"}); err != nil {
		t.Fatal(err)
	}
	third, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatalf("tag relation update did not change fingerprint: %q", second)
	}
}

func TestGetWorkSourceFingerprintChangesWithSeriesTags(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatal(err)
	}
	library := &model.Library{
		ID: "work-series-tag-library", Name: "Series Tags", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	labels := []string{"第一话", "第二话"}
	for index, id := range []string{"series-tag-1", "series-tag-2"} {
		if _, err := DB().Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
			VALUES (?, ?, ?, 'comic', ?, ?)
		`, id, id+".cbz", id, library.ID, "作品/"+labels[index]+".cbz"); err != nil {
			t.Fatal(err)
		}
	}
	if err := ReplaceDetectedSeries(library.ID, []DetectedSeries{{
		ID:               "series-tag-work",
		LibraryID:        library.ID,
		RootRelativePath: "作品",
		Title:            "作品",
		SortTitle:        "作品",
		CoverComicID:     "series-tag-1",
		Items: []DetectedSeriesItem{
			{ComicID: "series-tag-1", SortIndex: 0, DisplayLabel: "第一话"},
			{ComicID: "series-tag-2", SortIndex: 1, DisplayLabel: "第二话"},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	before, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetSeriesTags("series-tag-work", []string{"连载中"}); err != nil {
		t.Fatal(err)
	}
	after, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatalf("series tag update did not change Work fingerprint: %q", before)
	}
}

func TestGetWorkSourceFingerprintHashesOrderedTagAndCategoryContent(t *testing.T) {
	setupTestDB(t)
	library := &model.Library{
		ID: "work-relation-hash-library", Name: "Relation Hash", Type: "comic", RootPath: t.TempDir(),
		Enabled: true, ScanEnabled: true, DefaultAccess: "private",
	}
	if err := CreateLibrary(library); err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES ('relation-hash-comic', '作品.cbz', '作品', 'comic', ?, '作品.cbz');
		INSERT INTO "Tag" ("id", "name", "color") VALUES (101, '甲乙', '#111');
		INSERT INTO "ComicTag" ("comicId", "tagId") VALUES ('relation-hash-comic', 101);
		INSERT INTO "Category" ("id", "name", "slug", "icon") VALUES (102, '分类甲', 'kind-a', 'book');
		INSERT INTO "ComicCategory" ("comicId", "categoryId") VALUES ('relation-hash-comic', 102);
	`, library.ID); err != nil {
		t.Fatal(err)
	}
	first, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DB().Exec(`UPDATE "Tag" SET "name" = '丙丁' WHERE "id" = 101`); err != nil {
		t.Fatal(err)
	}
	second, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("same-length tag replacement collided: %q", first)
	}
	if _, err := DB().Exec(`UPDATE "Category" SET "name" = '分类乙' WHERE "id" = 102`); err != nil {
		t.Fatal(err)
	}
	third, err := GetWorkSourceFingerprint("", []string{library.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatalf("same-length category replacement collided: %q", second)
	}
}
