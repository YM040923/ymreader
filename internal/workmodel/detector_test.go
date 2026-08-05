package workmodel

import "testing"

const (
	singleArchivePath  = "\u6076\u9b54X\u5929\u4f7f \u4e0d\u80fd\u53cb\u597d\u76f8\u5904.zip"
	singleArchiveTitle = "\u6076\u9b54X\u5929\u4f7f \u4e0d\u80fd\u53cb\u597d\u76f8\u5904"
	singleArchiveSort  = "\u6076\u9b54x\u5929\u4f7f \u4e0d\u80fd\u53cb\u597d\u76f8\u5904"
	chapterWorkTitle   = "\u5927\u738b\u9976\u547d"
	folderChapter001   = "\u5927\u738b\u9976\u547d/\u7b2c001\u8bdd.cbz"
	folderChapter002   = "\u5927\u738b\u9976\u547d/\u7b2c002\u8bdd.cbz"
	siblingChapter001  = "\u5927\u738b\u9976\u547d \u7b2c001\u8bdd.cbz"
	siblingChapter002  = "\u5927\u738b\u9976\u547d \u7b2c002\u8bdd.cbz"
)

func TestDetectWorksSingleArchiveAsFullWork(t *testing.T) {
	works := DetectWorks([]SourceItem{{
		ID:           "comic-1",
		LibraryID:    "library-1",
		RelativePath: singleArchivePath,
		Title:        singleArchiveTitle,
		PageCount:    123,
		FileSize:     456789,
	}})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	work := works[0]
	if work.ID == "" {
		t.Fatal("work.ID is empty")
	}
	if work.LibraryID != "library-1" {
		t.Fatalf("LibraryID = %q", work.LibraryID)
	}
	if work.RootRelativePath != singleArchiveTitle {
		t.Fatalf("RootRelativePath = %q", work.RootRelativePath)
	}
	if work.Title != singleArchiveTitle {
		t.Fatalf("Title = %q", work.Title)
	}
	if work.SortTitle != singleArchiveSort {
		t.Fatalf("SortTitle = %q", work.SortTitle)
	}
	if len(work.Units) != 1 {
		t.Fatalf("len(Units) = %d", len(work.Units))
	}
	unit := work.Units[0]
	if unit.ID == "" {
		t.Fatal("unit.ID is empty")
	}
	if unit.ComicID != "comic-1" {
		t.Fatalf("ComicID = %q", unit.ComicID)
	}
	if unit.RelativePath != singleArchivePath {
		t.Fatalf("RelativePath = %q", unit.RelativePath)
	}
	if unit.Kind != UnitKindFull {
		t.Fatalf("Kind = %q", unit.Kind)
	}
	if unit.Title != singleArchiveTitle || unit.DisplayLabel != "\u5168\u672c" {
		t.Fatalf("Title/DisplayLabel = %q/%q", unit.Title, unit.DisplayLabel)
	}
	if unit.SortIndex != 0 || unit.PageCount != 123 || unit.FileSize != 456789 {
		t.Fatalf("unit counters = %#v", unit)
	}
}

func TestDetectWorksFolderChaptersAsOneSortedWork(t *testing.T) {
	works := DetectWorks([]SourceItem{
		{ID: "comic-2", LibraryID: "library-1", RelativePath: folderChapter002, PageCount: 20, FileSize: 2000},
		{ID: "comic-1", LibraryID: "library-1", RelativePath: folderChapter001, PageCount: 10, FileSize: 1000},
	})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	if works[0].Title != chapterWorkTitle || works[0].RootRelativePath != chapterWorkTitle {
		t.Fatalf("work = %#v", works[0])
	}
	want := []struct {
		comicID       string
		relativePath  string
		chapterNumber float64
		sortIndex     int
		pageCount     int
		fileSize      int64
	}{
		{"comic-1", folderChapter001, 1, 0, 10, 1000},
		{"comic-2", folderChapter002, 2, 1, 20, 2000},
	}
	for i, w := range want {
		unit := works[0].Units[i]
		if unit.ComicID != w.comicID || unit.RelativePath != w.relativePath {
			t.Fatalf("Units[%d] identity = %#v", i, unit)
		}
		if unit.Kind != UnitKindChapter || unit.ChapterNumber == nil || *unit.ChapterNumber != w.chapterNumber {
			t.Fatalf("Units[%d] chapter = %#v", i, unit)
		}
		if unit.SortIndex != w.sortIndex || unit.PageCount != w.pageCount || unit.FileSize != w.fileSize {
			t.Fatalf("Units[%d] counters = %#v", i, unit)
		}
	}
}

func TestDetectWorksSiblingChaptersAsOneWork(t *testing.T) {
	works := DetectWorks([]SourceItem{
		{ID: "comic-2", LibraryID: "library-1", RelativePath: siblingChapter002},
		{ID: "comic-1", LibraryID: "library-1", RelativePath: siblingChapter001},
	})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	if works[0].Title != chapterWorkTitle || works[0].RootRelativePath != chapterWorkTitle {
		t.Fatalf("work = %#v", works[0])
	}
	wantPaths := []string{siblingChapter001, siblingChapter002}
	for i, want := range wantPaths {
		unit := works[0].Units[i]
		if unit.RelativePath != want {
			t.Fatalf("Units[%d].RelativePath = %q, want %q", i, unit.RelativePath, want)
		}
		if unit.Kind != UnitKindChapter || unit.SortIndex != i {
			t.Fatalf("Units[%d] = %#v", i, unit)
		}
	}
}

func TestDetectWorksSortsSameTitleAndRootAcrossLibrariesDeterministically(t *testing.T) {
	items := []SourceItem{
		{ID: "comic-b", LibraryID: "library-b", RelativePath: folderChapter001},
		{ID: "comic-a", LibraryID: "library-a", RelativePath: folderChapter001},
	}

	for n := 0; n < 25; n++ {
		works := DetectWorks(items)
		if len(works) != 2 {
			t.Fatalf("len(works) = %d", len(works))
		}
		if works[0].LibraryID != "library-a" || works[1].LibraryID != "library-b" {
			t.Fatalf("iteration %d libraries = %q, %q", n, works[0].LibraryID, works[1].LibraryID)
		}
	}
}

func TestDetectWorksNestedSingleArchiveUsesDirectParentAsFullWorkTitle(t *testing.T) {
	works := DetectWorks([]SourceItem{{
		ID:           "comic-1",
		LibraryID:    "library-1",
		RelativePath: "??/??A/??A.cbz",
	}})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	work := works[0]
	if work.Title != "??A" || work.RootRelativePath != "??/??A" {
		t.Fatalf("work = %#v", work)
	}
	if len(work.Units) != 1 || work.Units[0].Kind != UnitKindFull {
		t.Fatalf("Units = %#v", work.Units)
	}
}

func TestDetectWorksNestedVolumeChaptersAsOneSortedWork(t *testing.T) {
	vol01Chapter := "\u5927\u738b\u9976\u547d/Vol.01/\u7b2c001\u8bdd.cbz"
	vol02Chapter := "\u5927\u738b\u9976\u547d/Vol.02/\u7b2c002\u8bdd.cbz"
	works := DetectWorks([]SourceItem{
		{ID: "comic-2", LibraryID: "library-1", RelativePath: vol02Chapter},
		{ID: "comic-1", LibraryID: "library-1", RelativePath: vol01Chapter},
	})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	work := works[0]
	if work.Title != chapterWorkTitle || work.RootRelativePath != chapterWorkTitle {
		t.Fatalf("work = %#v", work)
	}
	want := []struct {
		comicID       string
		relativePath  string
		displayLabel  string
		chapterNumber float64
	}{
		{"comic-1", vol01Chapter, "Vol.01/\u7b2c001\u8bdd", 1},
		{"comic-2", vol02Chapter, "Vol.02/\u7b2c002\u8bdd", 2},
	}
	for i, w := range want {
		unit := work.Units[i]
		if unit.ComicID != w.comicID || unit.RelativePath != w.relativePath {
			t.Fatalf("Units[%d] identity = %#v", i, unit)
		}
		if unit.Kind != UnitKindChapter || unit.ChapterNumber == nil || *unit.ChapterNumber != w.chapterNumber {
			t.Fatalf("Units[%d] chapter = %#v", i, unit)
		}
		if unit.DisplayLabel != w.displayLabel || unit.SortIndex != i {
			t.Fatalf("Units[%d] label/sort = %#v", i, unit)
		}
	}
}
