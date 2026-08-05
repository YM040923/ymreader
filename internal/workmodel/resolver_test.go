package workmodel

import "testing"

func TestResolveSingleArchiveAsFullWork(t *testing.T) {
	got := ResolvePath("恶魔X天使 不能友好相处.zip", "")
	if got.WorkTitle != "恶魔X天使 不能友好相处" {
		t.Fatalf("WorkTitle = %q", got.WorkTitle)
	}
	if got.Unit.Kind != UnitKindFull || got.Unit.DisplayLabel != "全本" {
		t.Fatalf("Unit = %#v", got.Unit)
	}
}

func TestResolveChineseChapter(t *testing.T) {
	got := ResolvePath("大王饶命/大王饶命 第01话.cbz", "大王饶命")
	if got.WorkTitle != "大王饶命" {
		t.Fatalf("WorkTitle = %q", got.WorkTitle)
	}
	if got.Unit.Kind != UnitKindChapter || got.Unit.ChapterNumber == nil || *got.Unit.ChapterNumber != 1 {
		t.Fatalf("Unit = %#v", got.Unit)
	}
	if got.Unit.DisplayLabel != "第01话" {
		t.Fatalf("DisplayLabel = %q", got.Unit.DisplayLabel)
	}
}

func TestResolveVolume(t *testing.T) {
	got := ResolvePath("败犬女主太多了 Vol.02.cbz", "败犬女主太多了")
	if got.Unit.Kind != UnitKindVolume || got.Unit.VolumeNumber == nil || *got.Unit.VolumeNumber != 2 {
		t.Fatalf("Unit = %#v", got.Unit)
	}
	if got.Unit.DisplayLabel != "Vol.02" {
		t.Fatalf("DisplayLabel = %q", got.Unit.DisplayLabel)
	}
}

func TestResolveChineseVolume(t *testing.T) {
	got := ResolvePath("葬送的芙莉莲 第03卷.cbz", "葬送的芙莉莲")
	if got.Unit.Kind != UnitKindVolume || got.Unit.VolumeNumber == nil || *got.Unit.VolumeNumber != 3 {
		t.Fatalf("Unit = %#v", got.Unit)
	}
	if got.Unit.DisplayLabel != "第03卷" {
		t.Fatalf("DisplayLabel = %q", got.Unit.DisplayLabel)
	}
}

func TestResolveSpecialAfterMain(t *testing.T) {
	items := []ResolvedUnit{
		{Kind: UnitKindSpecial, DisplayLabel: "特典", SortKey: SortKey{KindRank: 3, Number: 0, Raw: "特典"}},
		{Kind: UnitKindExtra, DisplayLabel: "番外", SortKey: SortKey{KindRank: 2, Number: 0, Raw: "番外"}},
		{Kind: UnitKindChapter, DisplayLabel: "第02话", ChapterNumber: floatPtr(2), SortKey: SortKey{KindRank: 1, Number: 2, Raw: "第02话"}},
		{Kind: UnitKindChapter, DisplayLabel: "第01话", ChapterNumber: floatPtr(1), SortKey: SortKey{KindRank: 1, Number: 1, Raw: "第01话"}},
	}
	SortUnits(items)
	want := []string{"第01话", "第02话", "番外", "特典"}
	for i, label := range want {
		if items[i].DisplayLabel != label {
			t.Fatalf("order = %#v", items)
		}
	}
}
