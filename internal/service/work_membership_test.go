package service

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func manualMembershipFixture() []Work {
	return []Work{
		{
			ID: "work_a", LibraryID: "lib", Title: "A", RootPath: "A",
			Units: []WorkUnit{
				{ID: "u1", ComicID: "c1", DisplayLabel: "01", PageCount: 10},
			},
		},
		{
			ID: "work_b", LibraryID: "lib", Title: "B", RootPath: "B",
			Units: []WorkUnit{
				{ID: "u2", ComicID: "c2", DisplayLabel: "02", PageCount: 20},
			},
		},
	}
}

func manualComicSources() map[string]store.ComicListItem {
	readAt := "2026-11-01T10:00:00Z"
	return map[string]store.ComicListItem{
		"c1": {
			ID: "c1", Filename: "A/01.cbz", RelativePath: "A/01.cbz",
			PageCount: 10, FileSize: 100, IsFavorite: true, ReadingStatus: "finished",
			LastReadAt: &readAt, LastReadPage: 9, TotalReadTime: 30,
			AddedAt: "2026-10-01T00:00:00Z",
		},
		"c2": {
			ID: "c2", Filename: "B/01.cbz", RelativePath: "B/01.cbz",
			PageCount: 20, FileSize: 200, ReadingStatus: "unread",
			AddedAt: "2026-10-02T00:00:00Z",
		},
	}
}

func TestApplyManualWorkMembershipsMergesAndSplits(t *testing.T) {
	works := manualMembershipFixture()

	merged := ApplyManualWorkMemberships(works, map[string]string{
		"c1": "work_a",
		"c2": "work_a",
	}, nil, manualComicSources())
	if len(merged) != 1 || merged[0].ID != "work_a" {
		t.Fatalf("merge result = %#v, want one Work a", merged)
	}
	if merged[0].ItemCount != 2 || merged[0].PageCount != 30 || merged[0].FileSize != 300 {
		t.Fatalf("merged work totals = items:%d pages:%d size:%d", merged[0].ItemCount, merged[0].PageCount, merged[0].FileSize)
	}
	if merged[0].CompletedItemCount != 1 || !merged[0].IsFavorite || merged[0].ReadingStatus != "reading" ||
		merged[0].TotalReadTime != 30 || merged[0].ContinueUnitID != "u1" {
		t.Fatalf("merged work user state = %#v", merged[0])
	}

	split := ApplyManualWorkMemberships(manualMembershipFixture(), map[string]string{
		"c1": "work_manual_x",
	}, map[string]store.LogicalWork{
		"work_manual_x": {ID: "work_manual_x", LibraryID: "lib", RootPath: "A\x1fwork_manual_x", Title: "A 拆分"},
	}, manualComicSources())
	if len(split) != 2 {
		for _, derived := range split {
			t.Logf("derived work %s title=%s units=%d", derived.ID, derived.Title, len(derived.Units))
		}
		t.Fatalf("split result count = %d, want 2", len(split))
	}
	var manual, original *Work
	for index := range split {
		switch split[index].ID {
		case "work_manual_x":
			manual = &split[index]
		case "work_b":
			original = &split[index]
		}
	}
	if manual == nil || manual.Title != "A 拆分" || manual.PageCount != 10 || manual.RootPath != "A" {
		t.Fatalf("manual work = %#v", manual)
	}
	if manual.CompletedItemCount != 1 || !manual.IsFavorite || manual.ReadingStatus != "finished" ||
		manual.TotalReadTime != 30 || manual.ContinueUnitID != "u1" {
		t.Fatalf("manual work user state = %#v", manual)
	}
	if original == nil || original.PageCount != 20 || original.CompletedItemCount != 0 || original.IsFavorite {
		t.Fatalf("original work was not preserved: %#v", original)
	}
}
