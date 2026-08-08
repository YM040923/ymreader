package service

import "testing"

func TestBuildWorkFileStatsCountsWorksInsteadOfPhysicalUnits(t *testing.T) {
	works := []Work{
		{
			ID:        "work-a",
			Title:     "A",
			RootPath:  "日漫/A",
			FileSize:  300,
			PageCount: 30,
			Units: []WorkUnit{
				{ComicID: "a-1", RelativePath: "日漫/A/01.cbz"},
				{ComicID: "a-2", RelativePath: "日漫/A/02.cbz"},
			},
		},
		{
			ID:        "work-b",
			Title:     "B",
			RootPath:  "国漫/B",
			FileSize:  100,
			PageCount: 10,
			Units: []WorkUnit{
				{ComicID: "b", RelativePath: "国漫/B.zip"},
			},
		},
	}

	stats := BuildWorkFileStats(works)
	if stats.TotalFiles != 2 || stats.TotalPages != 40 ||
		stats.AvgFileSize != 200 || stats.AvgPageCount != 20 {
		t.Fatalf("work stats = %#v", stats)
	}
	if len(stats.FormatStats) != 2 ||
		stats.FormatStats[0].Count != 1 ||
		stats.FormatStats[1].Count != 1 {
		t.Fatalf("format stats must count works, not physical units: %#v", stats.FormatStats)
	}
}

func TestBuildWorkFolderTreeListsEachWorkOnce(t *testing.T) {
	tree := BuildWorkFolderTree([]Work{
		{
			ID:        "work-a",
			Title:     "A",
			RootPath:  "日漫/A",
			FileSize:  300,
			PageCount: 30,
			ItemCount: 2,
		},
	})
	if len(tree) != 1 || tree[0].Name != "日漫" ||
		tree[0].FileCount != 1 || len(tree[0].Files) != 1 {
		t.Fatalf("tree = %#v", tree)
	}
	if tree[0].Files[0].ID != "work-a" {
		t.Fatalf("work file = %#v", tree[0].Files[0])
	}
}
