package workmodel

import "testing"

func TestDetectWorksSingleArchiveAsFullWork(t *testing.T) {
	works := DetectWorks([]string{"恶魔X天使 不能友好相处.zip"})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	if works[0].Title != "恶魔X天使 不能友好相处" {
		t.Fatalf("Title = %q", works[0].Title)
	}
	if len(works[0].Units) != 1 {
		t.Fatalf("len(Units) = %d", len(works[0].Units))
	}
	if works[0].Units[0].Unit.Kind != UnitKindFull {
		t.Fatalf("Unit.Kind = %q", works[0].Units[0].Unit.Kind)
	}
	if works[0].Units[0].RelativePath != "恶魔X天使 不能友好相处.zip" {
		t.Fatalf("RelativePath = %q", works[0].Units[0].RelativePath)
	}
}

func TestDetectWorksFolderChaptersAsOneSortedWork(t *testing.T) {
	works := DetectWorks([]string{
		"大王饶命/第002话.cbz",
		"大王饶命/第001话.cbz",
	})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	if works[0].Title != "大王饶命" {
		t.Fatalf("Title = %q", works[0].Title)
	}
	wantPaths := []string{"大王饶命/第001话.cbz", "大王饶命/第002话.cbz"}
	for i, want := range wantPaths {
		if works[0].Units[i].RelativePath != want {
			t.Fatalf("Units[%d].RelativePath = %q, want %q", i, works[0].Units[i].RelativePath, want)
		}
		if works[0].Units[i].Unit.Kind != UnitKindChapter {
			t.Fatalf("Units[%d].Kind = %q", i, works[0].Units[i].Unit.Kind)
		}
	}
}

func TestDetectWorksSiblingChaptersAsOneWork(t *testing.T) {
	works := DetectWorks([]string{
		"大王饶命 第002话.cbz",
		"大王饶命 第001话.cbz",
	})

	if len(works) != 1 {
		t.Fatalf("len(works) = %d", len(works))
	}
	if works[0].Title != "大王饶命" {
		t.Fatalf("Title = %q", works[0].Title)
	}
	wantPaths := []string{"大王饶命 第001话.cbz", "大王饶命 第002话.cbz"}
	for i, want := range wantPaths {
		if works[0].Units[i].RelativePath != want {
			t.Fatalf("Units[%d].RelativePath = %q, want %q", i, works[0].Units[i].RelativePath, want)
		}
		if works[0].Units[i].Unit.Kind != UnitKindChapter {
			t.Fatalf("Units[%d].Kind = %q", i, works[0].Units[i].Unit.Kind)
		}
	}
}
