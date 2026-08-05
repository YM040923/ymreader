package store

import (
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
)

func seedWorkLibraryAndComics(t *testing.T) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('work-lib', 'Work Lib', 'comic', '/tmp/work')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('comic-1', 'work/01.cbz', 'Chapter 1', 'comic', 'work-lib', 'work/01.cbz', 10, 100),
			('comic-2', 'work/02.cbz', 'Chapter 2', 'comic', 'work-lib', 'work/02.cbz', 20, 200),
			('comic-3', 'work/03.cbz', 'Chapter 3', 'comic', 'work-lib', 'work/03.cbz', 30, 300)
	`); err != nil {
		t.Fatal(err)
	}
}

func TestWorkReplaceCreatesWorkAndUnit(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)

	works := []model.Work{{
		ID:           "work-1",
		LibraryID:    "work-lib",
		Title:        "Test Work",
		SortTitle:    "test work",
		CoverComicID: "comic-1",
		Units: []model.WorkUnit{{
			ID:           "unit-1",
			WorkID:       "work-1",
			ComicID:      "comic-1",
			Title:        "Chapter 1",
			SortIndex:    0,
			RelativePath: "work/01.cbz",
		}},
	}}
	if err := ReplaceWorksForLibrary("work-lib", works); err != nil {
		t.Fatalf("ReplaceWorksForLibrary failed: %v", err)
	}

	work, err := GetWorkDetail("work-1")
	if err != nil {
		t.Fatalf("GetWorkDetail failed: %v", err)
	}
	if work == nil || work.ID != "work-1" || len(work.Units) != 1 || work.Units[0].ComicID != "comic-1" {
		t.Fatalf("persisted work detail = %#v", work)
	}
}

func TestWorkListItemCount(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)

	if err := ReplaceWorksForLibrary("work-lib", []model.Work{{
		ID: "work-1", LibraryID: "work-lib", Title: "Test Work", SortTitle: "test work",
		Units: []model.WorkUnit{{ID: "unit-1", WorkID: "work-1", ComicID: "comic-1", Title: "Chapter 1", SortIndex: 0}, {ID: "unit-2", WorkID: "work-1", ComicID: "comic-2", Title: "Chapter 2", SortIndex: 1}},
	}}); err != nil {
		t.Fatal(err)
	}
	works, err := ListWorks("work-lib")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 || works[0].ItemCount != 2 {
		t.Fatalf("ListWorks = %#v, want one item with itemCount=2", works)
	}
}

func TestWorkDetailUnitsSorted(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)

	if err := ReplaceWorksForLibrary("work-lib", []model.Work{{
		ID: "work-1", LibraryID: "work-lib", Title: "Test Work", SortTitle: "test work",
		Units: []model.WorkUnit{{ID: "unit-2", WorkID: "work-1", ComicID: "comic-2", Title: "Chapter 2", SortIndex: 2}, {ID: "unit-1", WorkID: "work-1", ComicID: "comic-1", Title: "Chapter 1", SortIndex: 1}},
	}}); err != nil {
		t.Fatal(err)
	}
	detail, err := GetWorkDetail("work-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Units) != 2 || detail.Units[0].ID != "unit-1" || detail.Units[1].ID != "unit-2" {
		t.Fatalf("units not sorted by sortIndex: %#v", detail.Units)
	}
}

func TestWorkProgressSaveRead(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if _, err := db.Exec(`INSERT INTO "User" ("id", "username", "password", "role") VALUES ('user-1', 'user1', 'hash', 'user')`); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceWorksForLibrary("work-lib", []model.Work{{ID: "work-1", LibraryID: "work-lib", Title: "Test Work", Units: []model.WorkUnit{{ID: "unit-1", WorkID: "work-1", ComicID: "comic-1", Title: "Chapter 1"}}}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	progress := &model.UserWorkProgress{UserID: "user-1", WorkID: "work-1", UnitID: "unit-1", LastPage: 7, Progress: 0.7, UpdatedAt: now}
	if err := SetUserWorkProgress(progress); err != nil {
		t.Fatal(err)
	}
	got, err := GetUserWorkProgress("user-1", "work-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.UnitID != "unit-1" || got.LastPage != 7 || got.Progress != 0.7 {
		t.Fatalf("progress = %#v", got)
	}
}

func TestWorkAdjacentUnit(t *testing.T) {
	setupTestDB(t)
	seedWorkLibraryAndComics(t)
	if err := ReplaceWorksForLibrary("work-lib", []model.Work{{
		ID: "work-1", LibraryID: "work-lib", Title: "Test Work",
		Units: []model.WorkUnit{{ID: "unit-1", WorkID: "work-1", ComicID: "comic-1", Title: "Chapter 1", SortIndex: 0}, {ID: "unit-2", WorkID: "work-1", ComicID: "comic-2", Title: "Chapter 2", SortIndex: 1}, {ID: "unit-3", WorkID: "work-1", ComicID: "comic-3", Title: "Chapter 3", SortIndex: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	next, err := GetAdjacentWorkUnit("work-1", "unit-2", 1)
	if err != nil {
		t.Fatal(err)
	}
	prev, err := GetAdjacentWorkUnit("work-1", "unit-2", -1)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "unit-3" || prev == nil || prev.ID != "unit-1" {
		t.Fatalf("prev=%#v next=%#v", prev, next)
	}
}
