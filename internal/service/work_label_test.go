package service

import (
	"reflect"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestFormatUnitLabelMatchesStreamingSiteStyle(t *testing.T) {
	cases := []struct {
		label string
		work  string
		want  string
	}{
		// 腾讯动漫/哔哩哔哩漫画都这样排：不补零，编号与标题之间一个空格
		{"第0001话 我死了？", "大王饶命", "第1话 我死了？"},
		{"第0916话 青空之死", "大王饶命", "第916话 青空之死"},
		{"第001话", "从姑获鸟开始", "第1话"},
		{"第0000话 序章_同时被几个女生追求是什么体验？", "我的假女友正全力防御她们的进攻", "第0话 序章 同时被几个女生追求是什么体验？"},
		{"第116 妖皇秘辛", "炼气练了三千年", "第116话 妖皇秘辛"},
		{"第118.1话_你对秦枫是怎么想的？", "女子学院的男生", "第118.1话 你对秦枫是怎么想的？"},
		{"第0001-0020话", "妹子与科学", "第1-20话"},
		// 卷/英文章节缩写统一成中文
		{"久保同学不放过我 - Vol. 001", "久保同学不放过我", "第1卷"},
		{"作品 Vol.02", "作品", "第2卷"},
		{"Ch. 0001", "作品", "第1话"},
		{"Ch.0000.5", "作品", "第0.5话"},
		{"Ch. 33-37", "作品", "第33-37话"},
		{"Ep. 12 归途", "作品", "第12话 归途"},
		// 番外
		{"番外1-一切的开始（上）", "大王饶命", "番外1 一切的开始（上）"},
		{"番外1_凶许主任", "大王饶命", "番外1 凶许主任"},
		{"番外", "某作品", "番外"},
		{"相反的你我 - Extra - 动画化", "相反的你我", "番外 动画化"},
		{"相反的你我 - Extra 01", "相反的你我", "番外1"},
		// 分区前缀保留
		{"第一季 / 第001话", "作品", "第一季 / 第1话"},
		{"Extras / Archive.601", "败犬女主太多了", "Extras / Archive.601"},
		// 不该动的
		{"全文", "作品", "全文"},
		{"第十话", "作品", "第十话"},
		{"0 序章 楔子", "斗罗大陆 第三部 龙王传说", "第0话 序章 楔子"},
		{"448 番外", "斗罗大陆 第三部 龙王传说", "第448话 番外"},
		{"2 季", "作品", "2 季"},
	}
	for _, tc := range cases {
		if got := formatUnitLabel(tc.label, tc.work); got != tc.want {
			t.Errorf("formatUnitLabel(%q, %q) = %q, want %q", tc.label, tc.work, got, tc.want)
		}
	}
}

func TestBuildWorksSortsExtrasAfterMainChapters(t *testing.T) {
	items := []store.ComicListItem{
		{ID: "e1", Title: "番外1 凶许主任", Filename: "番外1 凶许主任/", LibraryID: "lib", RelativePath: "大王饶命/番外1 凶许主任/"},
		{ID: "c2", Title: "第0002话", Filename: "第0002话/", LibraryID: "lib", RelativePath: "大王饶命/第0002话/"},
		{ID: "c1", Title: "第0001话", Filename: "第0001话/", LibraryID: "lib", RelativePath: "大王饶命/第0001话/"},
		{ID: "e2", Title: "Extra 01", Filename: "Archive.601.cbz", LibraryID: "lib", RelativePath: "大王饶命/Extras/Archive.601.cbz"},
	}
	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 {
		t.Fatalf("works=%#v", works)
	}
	got := unitLabels(works[0].Units)
	want := []string{"第1话", "第2话", "Extras / Archive.601", "番外1 凶许主任"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("labels=%#v, want %#v", got, want)
	}
}

func TestFormatUnitLabelKeepsPrefixWhenWorkTitleIsRealTitle(t *testing.T) {
	// 以作品名开头但本身是完整作品名，不能把前缀当重复内容剥掉
	if got := formatUnitLabel("大王饶命之新亭是好刀", "大王饶命"); got != "大王饶命之新亭是好刀" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatUnitLabelDoesNotChangeUnitIdentity(t *testing.T) {
	// DisplayLabel 会被格式化，但 Unit ID 必须继续由原始标签生成，
	// 否则老用户的阅读进度会全部对不上。
	items := []store.ComicListItem{
		{ID: "c1", Title: "第0916话 青空之死", Filename: "第0916话 青空之死/", LibraryID: "lib", RelativePath: "大王饶命/第0916话 青空之死/"},
	}
	works := BuildWorksFromComicList(items, WorkBuildOptions{})
	if len(works) != 1 || len(works[0].Units) != 1 {
		t.Fatalf("works=%#v", works)
	}
	unit := works[0].Units[0]
	if unit.DisplayLabel != "第916话 青空之死" {
		t.Fatalf("display label=%q", unit.DisplayLabel)
	}
	wantID := stableUnitID(works[0].ID, "c1", "", "第0916话 青空之死")
	if unit.ID != wantID {
		t.Fatalf("unit id=%q, want raw-label id %q", unit.ID, wantID)
	}
}
