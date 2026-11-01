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

func TestApplyManualWorkMembershipsMergesAndSplits(t *testing.T) {
	works := manualMembershipFixture()

	merged := ApplyManualWorkMemberships(works, map[string]string{
		"c1": "work_a",
		"c2": "work_a",
	}, nil, map[string]int64{"c1": 100, "c2": 200})
	if len(merged) != 1 || merged[0].ID != "work_a" {
		t.Fatalf("merge result = %#v, want one Work a", merged)
	}
	if merged[0].ItemCount != 2 || merged[0].PageCount != 30 || merged[0].FileSize != 300 {
		t.Fatalf("merged work totals = items:%d pages:%d size:%d", merged[0].ItemCount, merged[0].PageCount, merged[0].FileSize)
	}

	split := ApplyManualWorkMemberships(manualMembershipFixture(), map[string]string{
		"c1": "work_manual_x",
	}, map[string]store.LogicalWork{
		"work_manual_x": {ID: "work_manual_x", LibraryID: "lib", RootPath: "A\x1fwork_manual_x", Title: "A 拆分"},
	}, map[string]int64{"c1": 100, "c2": 200})
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
	if original == nil || original.PageCount != 20 {
		t.Fatalf("original work was not preserved: %#v", original)
	}
}
