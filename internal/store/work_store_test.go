package store

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func seedWorkLibraryAndComics(t *testing.T) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('work-lib', 'Work Lib', 'comic', '/tmp/work')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('comic-1', 'Series/01.cbz', 'Chapter 1', 'comic', 'work-lib', 'Series/01.cbz', 10, 100),
			('comic-2', 'Series/02.cbz', 'Chapter 2', 'comic', 'work-lib', 'Series/02.cbz', 20, 200),
			('comic-3', 'Series/03.cbz', 'Chapter 3', 'comic', 'work-lib', 'Series/03.cbz', 30, 300),
			('comic-other', 'Other/01.cbz', 'Other 1', 'comic', 'work-lib', 'Other/01.cbz', 5, 50)
	`); err != nil {
		t.Fatal(err)
	}
}

func detectedWorkFixture() []workmodel.DetectedWork {
	return []workmodel.DetectedWork{{
		ID: "work-1", LibraryID: "work-lib", RootRelativePath: "Series", Title: "Series", SortTitle: "series",
		Units: []workmodel.DetectedUnit{
			{ID: "unit-1", ComicID: "comic-1", RelativePath: "Series/01.cbz", Kind: workmodel.UnitKindChapter, Title: "Chapter 1", DisplayLabel: "第 1 话", ChapterNumber: floatPtr(1), SortIndex: 0, PageCount: 10, FileSize: 100},
			{ID: "unit-2", ComicID: "comic-2", RelativePath: "Series/02.cbz", Kind: workmodel.UnitKindChapter, Title: "Chapter 2", DisplayLabel: "第 2 话", ChapterNumber: floatPtr(2), SortIndex: 1, PageCount: 20, FileSize: 200},
			{ID: "unit-3", ComicID: "comic-3", RelativePath: "Series/03.cbz", Kind: workmodel.UnitKindChapter, Title: "Chapter 3", DisplayLabel: "第 3 话", ChapterNumber: floatPtr(3), SortIndex: 2, PageCount: 30, FileSize: 300},
		},
	}}
}

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

func TestWorkReplaceCreatesDetectedWorkAndUnits(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)

	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatalf("ReplaceWorksForLibrary failed: %v", err)
	}

	work, units, err := GetWorkDetail("work-1", "user-1")
	if err != nil {
		t.Fatalf("GetWorkDetail failed: %v", err)
	}
	if work == nil || work.ID != "work-1" || work.LibraryID != "work-lib" || work.RootRelativePath != "Series" || work.ItemCount != 3 {
		t.Fatalf("persisted work = %#v", work)
	}
	if len(units) != 3 || units[0].ID != "unit-1" || units[0].ComicID != "comic-1" || units[0].UnitKind != string(workmodel.UnitKindChapter) || units[0].CoverURL == "" {
		t.Fatalf("persisted units = %#v", units)
	}
}

func TestWorkListFiltersLibrariesAndSearches(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatal(err)
	}

	works, err := ListWorks([]string{"work-lib"}, "user-1", "series")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 || works[0].ID != "work-1" || works[0].ItemCount != 3 {
		t.Fatalf("ListWorks = %#v, want one item with itemCount=3", works)
	}

	missing, err := ListWorks([]string{"work-lib"}, "user-1", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("ListWorks search missing = %#v", missing)
	}
}

func TestWorkDetailUnitsSortedAndAdjacentBySortIndex(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	works := detectedWorkFixture()
	works[0].Units[0].SortIndex = 2
	works[0].Units[1].SortIndex = 0
	works[0].Units[2].SortIndex = 1
	if err := ReplaceWorksForLibrary("work-lib", works); err != nil {
		t.Fatal(err)
	}

	_, units, err := GetWorkDetail("work-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 3 || units[0].ID != "unit-2" || units[1].ID != "unit-3" || units[2].ID != "unit-1" {
		t.Fatalf("units not sorted by sortIndex: %#v", units)
	}
	next, err := GetAdjacentWorkUnit("work-1", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	prev, err := GetAdjacentWorkUnit("work-1", 1, -1)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "unit-1" || prev == nil || prev.ID != "unit-2" {
		t.Fatalf("prev=%#v next=%#v", prev, next)
	}
}

func TestWorkProgressSaveReadUsesPageIndex(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if _, err := db.Exec(`INSERT INTO "User" ("id", "username", "password", "role") VALUES ('user-1', 'user1', 'hash', 'user')`); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatal(err)
	}
	if err := SetUserWorkProgress("user-1", "work-1", "unit-2", 7); err != nil {
		t.Fatal(err)
	}
	got, err := GetUserWorkProgress("user-1", "work-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.UnitID != "unit-2" || got.PageIndex != 7 || got.UpdatedAt == nil {
		t.Fatalf("progress = %#v", got)
	}
}

func TestWorkReplacePreservesManualAndLockedMetadata(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatal(err)
	}
	year := 2024
	if _, err := db.Exec(`UPDATE "Work" SET "title" = 'Manual Title', "sortTitle" = 'manual title', "author" = 'Author A', "publisher" = 'Pub A', "year" = ?, "description" = 'Desc A', "language" = 'ja', "genre" = 'manga', "metadataSource" = 'manual', "coverUrl" = 'https://example.test/cover.jpg', "coverUnitId" = 'unit-2', "metadataLocked" = 1, "manualLocked" = 1 WHERE "id" = 'work-1'`, year); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceWorksForLibrary("work-lib", []workmodel.DetectedWork{{ID: "work-new", LibraryID: "work-lib", RootRelativePath: "Other", Title: "Other", SortTitle: "other", Units: []workmodel.DetectedUnit{{ID: "unit-other", ComicID: "comic-other", RelativePath: "Other/01.cbz", Kind: workmodel.UnitKindChapter, Title: "Other 1", SortIndex: 0, PageCount: 5, FileSize: 50}}}}); err != nil {
		t.Fatal(err)
	}

	manual, units, err := GetWorkDetail("work-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if manual == nil || len(units) != 3 || manual.Title != "Manual Title" || manual.Author != "Author A" || manual.Year == nil || *manual.Year != 2024 || !manual.MetadataLocked || !manual.ManualLocked || manual.CoverUnitID != "unit-2" {
		t.Fatalf("manual locked work not preserved with metadata: work=%#v units=%#v", manual, units)
	}
}

func TestWorkUnitByComicAndSourceItems(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatal(err)
	}
	unit, err := GetWorkUnitByComicID("comic-2")
	if err != nil {
		t.Fatal(err)
	}
	if unit == nil || unit.ID != "unit-2" {
		t.Fatalf("GetWorkUnitByComicID = %#v", unit)
	}
	items, err := ListWorkSourceItems("work-lib")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 || items[0].ID == "" || items[0].LibraryID != "work-lib" || items[0].RelativePath == "" {
		t.Fatalf("ListWorkSourceItems = %#v", items)
	}
}

func TestWorkSchemaIndexesAndUniqueRelativePath(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if err := ReplaceWorksForLibrary("work-lib", detectedWorkFixture()); err != nil {
		t.Fatal(err)
	}
	var count int
	for _, name := range []string{"Work_library_sort_idx", "WorkUnit_work_sort_idx", "WorkUnit_comic_idx"} {
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing index %s", name)
		}
	}
	_, err := db.Exec(`INSERT INTO "WorkUnit" ("id", "workId", "comicId", "relativePath", "title") VALUES ('dupe-unit', 'work-1', 'comic-1', 'Series/01.cbz', 'dupe')`)
	if err == nil {
		t.Fatalf("expected unique(workId, relativePath) violation")
	}
}
