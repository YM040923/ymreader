package service

import (
	"reflect"
	"testing"
)

func TestParseTencentChapterTitles(t *testing.T) {
	page := `
<html><body>
<a href="/ComicView/index/id/645654/cid/1">开始阅读</a>
<a href="/ComicView/index/id/645654/cid/1">预告</a>
<a href="/ComicView/index/id/645654/cid/2"><span class="x">01 我死了？</span></a>
<a href="/ComicView/index/id/645654/cid/3">第2话 神奇的系统</a>
<a href="/ComicView/index/id/645654/cid/4">番外1-一切的开始（上）</a>
<a href="/ComicView/index/id/645654/cid/5">003</a>
<a href="/ComicView/index/id/645654/cid/6">1.5 番外小剧场</a>
<a href="/ComicView/index/id/645654/cid/7">总8·雪人（完结）</a>
</body></html>`
	got := ParseTencentChapterTitles(page)
	want := map[string]string{
		"第1":   "我死了？",
		"第2":   "神奇的系统",
		"番外1":  "一切的开始（上）",
		"第1.5": "番外小剧场",
		"第8":   "雪人（完结）",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("titles=%#v, want %#v", got, want)
	}
	if _, ok := got["第3"]; ok {
		t.Fatalf("pure numeric title must not be treated as a chapter title: %#v", got)
	}
}

func TestUnitTitleKeyOnlyMatchesHeadingOnlyLabels(t *testing.T) {
	cases := []struct {
		label string
		key   string
		ok    bool
	}{
		{"第12话", "第12", true},
		{"第0012话", "第12", true},
		{"第12.5话", "第12.5", true},
		{"番外3", "番外3", true},
		{"番外", "番外", true},
		{"第12话 永夜降临", "", false},
		{"第12卷", "", false},
		{"全文", "", false},
		{"番外1 一切的开始", "", false},
	}
	for _, tc := range cases {
		key, ok := unitTitleKey(tc.label)
		if key != tc.key || ok != tc.ok {
			t.Errorf("unitTitleKey(%q) = %q,%v want %q,%v", tc.label, key, ok, tc.key, tc.ok)
		}
	}
}

func TestApplyChapterTitleOverrides(t *testing.T) {
	works := []Work{{Units: []WorkUnit{
		{ID: "u1", DisplayLabel: "第12话", Title: "第12话"},
		{ID: "u2", DisplayLabel: "第13话 已有标题", Title: "第13话 已有标题"},
		{ID: "u3", DisplayLabel: "番外3", Title: "番外3"},
		{ID: "u4", DisplayLabel: "第12卷", Title: "第12卷"},
	}}}
	ApplyChapterTitleOverrides(works, map[string]string{
		"u1": "永夜降临",
		"u2": "不该覆盖",
		"u3": "转世的御扶摇",
		"u4": "卷标题",
	})
	if works[0].Units[0].DisplayLabel != "第12话 永夜降临" || works[0].Units[0].Title != "第12话 永夜降临" {
		t.Fatalf("unit1=%#v", works[0].Units[0])
	}
	if works[0].Units[1].DisplayLabel != "第13话 已有标题" {
		t.Fatalf("existing title was overwritten: %#v", works[0].Units[1])
	}
	if works[0].Units[2].DisplayLabel != "番外3 转世的御扶摇" {
		t.Fatalf("unit3=%#v", works[0].Units[2])
	}
	if works[0].Units[3].DisplayLabel != "第12卷" {
		t.Fatalf("volume label should stay untouched: %#v", works[0].Units[3])
	}
	// 再跑一次不允许叠加
	ApplyChapterTitleOverrides(works, map[string]string{"u1": "永夜降临"})
	if works[0].Units[0].DisplayLabel != "第12话 永夜降临" {
		t.Fatalf("double append: %q", works[0].Units[0].DisplayLabel)
	}
}
