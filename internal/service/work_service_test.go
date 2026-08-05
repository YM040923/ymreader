package service

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestRebuildWorksForLibraryCreatesWorkAndSortedUnits(t *testing.T) {
	setupTestDB(t)
	db := store.DB()
	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('work-svc-lib', 'Work Service Lib', 'comic', '/tmp/work-svc')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('work-svc-2', '大王饶命/第002话.cbz', '第002话', 'comic', 'work-svc-lib', '大王饶命/第002话.cbz', 22, 2200),
			('work-svc-1', '大王饶命/第001话.cbz', '第001话', 'comic', 'work-svc-lib', '大王饶命/第001话.cbz', 11, 1100)
	`); err != nil {
		t.Fatal(err)
	}

	if err := RebuildWorksForLibrary("work-svc-lib"); err != nil {
		t.Fatalf("RebuildWorksForLibrary failed: %v", err)
	}

	works, err := store.ListWorks([]string{"work-svc-lib"}, "user-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 || works[0].Title != "大王饶命" || works[0].ItemCount != 2 {
		t.Fatalf("works = %#v", works)
	}
	_, units, err := store.GetWorkDetail(works[0].ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("units = %#v", units)
	}
	want := []struct {
		comicID      string
		relativePath string
		sortIndex    int
	}{
		{"work-svc-1", "大王饶命/第001话.cbz", 0},
		{"work-svc-2", "大王饶命/第002话.cbz", 1},
	}
	for i, w := range want {
		if units[i].ComicID != w.comicID || units[i].RelativePath != w.relativePath || units[i].SortIndex != w.sortIndex {
			t.Fatalf("units[%d] = %#v, want %#v", i, units[i], w)
		}
	}
}

func TestRebuildWorksForLibrarySkipsNovelLibrary(t *testing.T) {
	setupTestDB(t)
	db := store.DB()
	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('work-svc-novel-lib', 'Novel Lib', 'novel', '/tmp/novels')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('work-svc-book', 'Book.epub', 'Book', 'novel', 'work-svc-novel-lib', 'Book.epub', 1, 100)
	`); err != nil {
		t.Fatal(err)
	}

	if err := RebuildWorksForLibrary("work-svc-novel-lib"); err != nil {
		t.Fatalf("RebuildWorksForLibrary failed: %v", err)
	}

	works, err := store.ListWorks([]string{"work-svc-novel-lib"}, "user-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 0 {
		t.Fatalf("novel library should be skipped, got works %#v", works)
	}
}
