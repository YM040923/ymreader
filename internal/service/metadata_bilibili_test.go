package service

import (
	"encoding/json"
	"os"
	"testing"
)

// biliSearchFixture 取自 2026-11 真实响应：标题带 <em class="keyword"> 高亮，
// 封面是 http:// 与协议相对地址两种形态。
const biliSearchFixture = `{
  "code": 0,
  "msg": "",
  "data": {
    "list": [
      {
        "id": 25332,
        "title": "<em class=\"keyword\">大理寺日志</em>",
        "org_title": "",
        "alia_title": [],
        "horizontal_cover": "http://i0.hdslb.com/bfs/manga-static/f7787893cdbadbe9b380a389ad4656ccbe054f36.jpg",
        "square_cover": "http://i0.hdslb.com/bfs/manga-static/eb8e2d2c68f316f98efc139b3615970a13ed80c3.jpg",
        "vertical_cover": "http://i0.hdslb.com/bfs/manga-static/784520fb63d127779e2d508e963988188efb1439.jpg",
        "author_name": ["RC", "好传文化"],
        "styles": ["古风"],
        "recommendation": "武周年间，一介贫民陈拾只身来到洛阳寻找哥哥。",
        "is_finish": 0,
        "total": 340,
        "jump_value": "https://manga.bilibili.com/detail/mc25332",
        "attribution": 7
      },
      {
        "id": 26505,
        "title": "<em class=\"keyword\">咒术回战</em>",
        "vertical_cover": "//i0.hdslb.com/bfs/manga-static/aaa.jpg",
        "author_name": ["芥见下下"],
        "styles": ["热血", "战斗"],
        "recommendation": "诅咒之王的手指散落人间。",
        "wiki": {"publish_time": "2018-03-05"},
        "total": 271,
        "attribution": 1
      },
      {
        "id": 0,
        "title": "无效条目"
      }
    ]
  }
}`

// biliDetailFixture 是详情接口的真实字段形状（ep_list 等无关字段已省略）。
const biliDetailFixture = `{
  "id": 25332,
  "title": "大理寺日志",
  "comic_type": 0,
  "horizontal_cover": "http://i0.hdslb.com/bfs/manga-static/f7787893cdbadbe9b380a389ad4656ccbe054f36.jpg",
  "square_cover": "http://i0.hdslb.com/bfs/manga-static/eb8e2d2c68f316f98efc139b3615970a13ed80c3.jpg",
  "vertical_cover": "http://i0.hdslb.com/bfs/manga-static/784520fb63d127779e2d508e963988188efb1439.jpg",
  "author_name": ["RC", "好传文化"],
  "styles": ["古风"],
  "tags": [{"id": 659, "name": "搞笑"}, {"id": 718, "name": "古风"}],
  "evaluate": "武周年间，一介贫民陈拾只身来到洛阳寻找哥哥，阴差阳错下成为大理寺少卿的随身书吏。",
  "introduction": "一只大白猫和他的书吏。",
  "release_time": "",
  "renewal_time": "2020-11-21 10:00:00",
  "is_finish": 0,
  "total": 340,
  "japan_comic": false
}`

func parseBiliSearchFixture(t *testing.T) []biliSearchItem {
	t.Helper()
	var env biliEnvelope
	if err := json.Unmarshal([]byte(biliSearchFixture), &env); err != nil {
		t.Fatalf("search fixture 解析失败: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("search fixture code = %d", env.Code)
	}
	var data struct {
		List []biliSearchItem `json:"list"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("search data 解析失败: %v", err)
	}

	// 与 fetchBiliSearchItems 相同的清洗步骤
	out := make([]biliSearchItem, 0, len(data.List))
	for _, item := range data.List {
		if item.ID <= 0 {
			continue
		}
		item.Title = biliStripTags(item.Title)
		out = append(out, item)
	}
	return out
}

func TestBuildBiliMetadataFromFixtures(t *testing.T) {
	items := parseBiliSearchFixture(t)
	if len(items) != 2 {
		t.Fatalf("期望 2 条有效搜索结果（无效 id 被丢弃），实际 %d", len(items))
	}
	if items[0].Title != "大理寺日志" {
		t.Fatalf("搜索标题未剥离 <em> 高亮: %q", items[0].Title)
	}

	var detail biliComicDetail
	if err := json.Unmarshal([]byte(biliDetailFixture), &detail); err != nil {
		t.Fatalf("detail fixture 解析失败: %v", err)
	}

	meta := buildBiliMetadata(items[0], &detail)
	if meta.Source != "bilibili" {
		t.Fatalf("Source = %q", meta.Source)
	}
	if meta.Title != "大理寺日志" {
		t.Fatalf("Title = %q", meta.Title)
	}
	if meta.Author != "RC, 好传文化" {
		t.Fatalf("Author = %q", meta.Author)
	}
	// styles(古风) 与 tags(搞笑/古风) 合并去重
	if meta.Genre != "古风, 搞笑" {
		t.Fatalf("Genre = %q", meta.Genre)
	}
	// evaluate 优先于 introduction
	if meta.Description == "" || meta.Description == detail.Introduction {
		t.Fatalf("Description 应取 evaluate，实际 %q", meta.Description)
	}
	if meta.CoverURL != "https://i0.hdslb.com/bfs/manga-static/784520fb63d127779e2d508e963988188efb1439.jpg" {
		t.Fatalf("CoverURL 未转 https 或未取竖版: %q", meta.CoverURL)
	}
	// release_time 为空时回落到 renewal_time
	if meta.Year == nil || *meta.Year != 2020 {
		t.Fatalf("Year = %v，期望 2020", meta.Year)
	}
	if meta.Language != "zh-CN" {
		t.Fatalf("Language = %q", meta.Language)
	}

	jp := buildBiliMetadata(items[1], nil)
	if jp.Title != "咒术回战" {
		t.Fatalf("无详情时标题应保留: %q", jp.Title)
	}
	if jp.CoverURL != "https://i0.hdslb.com/bfs/manga-static/aaa.jpg" {
		t.Fatalf("协议相对封面未补全: %q", jp.CoverURL)
	}
	if jp.Year == nil || *jp.Year != 2018 {
		t.Fatalf("无详情时应从 wiki.publish_time 取年份，实际 %v", jp.Year)
	}
	if jp.Language != "" {
		t.Fatalf("无详情时不应猜语言，实际 %q", jp.Language)
	}
	if jp.Description == "" {
		t.Fatalf("description 应回落到 search recommendation")
	}
}

func TestBiliJapaneseFlagSetsLanguage(t *testing.T) {
	var detail biliComicDetail
	if err := json.Unmarshal([]byte(biliDetailFixture), &detail); err != nil {
		t.Fatalf("detail fixture 解析失败: %v", err)
	}
	detail.JapanComic = true
	meta := buildBiliMetadata(parseBiliSearchFixture(t)[0], &detail)
	if meta.Language != "ja" {
		t.Fatalf("japan_comic=true 时 Language = %q，期望 ja", meta.Language)
	}
}

func TestBiliYearExtraction(t *testing.T) {
	cases := map[string]int{
		"2020-11-21 10:00:00": 2020,
		"1999-01-01":          1999,
		"":                    0,
		"未知":                  0,
		// 0001-01-01 是 Go 的零值时间，站点用它表示"没有日期"，不能当成公元 1 年
		"0001-01-01 00:00:00": 0,
	}
	for in, want := range cases {
		if got := biliYear(in); got != want {
			t.Fatalf("biliYear(%q) = %d，期望 %d", in, got, want)
		}
	}
}

func TestBiliStripTagsAndCoverNormalize(t *testing.T) {
	if got := biliStripTags(`<em class="keyword">大理寺</em>日志`); got != "大理寺日志" {
		t.Fatalf("biliStripTags = %q", got)
	}
	if got := normalizeBiliCoverURL("//i0.hdslb.com/a.jpg"); got != "https://i0.hdslb.com/a.jpg" {
		t.Fatalf("协议相对地址处理错误: %q", got)
	}
	if got := normalizeBiliCoverURL("http://i0.hdslb.com/a.jpg"); got != "https://i0.hdslb.com/a.jpg" {
		t.Fatalf("http 升级错误: %q", got)
	}
}

// derefInt 便于日志里打印 *int 年份。
func derefInt(v *int) any {
	if v == nil {
		return "无"
	}
	return *v
}

// TestSearchBilibiliComicLive 需要外网，默认跳过：BILI_LIVE=1 go test -run BilibiliLive
func TestSearchBilibiliComicLive(t *testing.T) {
	if os.Getenv("BILI_LIVE") == "" {
		t.Skip("设置 BILI_LIVE=1 才跑线上验证")
	}
	queries := []string{"大理寺日志", "天官赐福", "咒术回战", "航海王", "火影忍者", "东京复仇者", "我独自升级"}
	for _, q := range queries {
		results := SearchMetadata(q, []string{"bilibili"}, "zh-CN")
		t.Logf("[%s] 共 %d 条", q, len(results))
		for i, r := range results {
			if i >= 3 {
				break
			}
			rating := "无"
			if r.ExternalRating != nil {
				rating = "有"
			}
			t.Logf("    %q 作者=%q 标签=%q 语言=%q 年份=%v 评分=%s 简介=%q 封面=%q",
				r.Title, r.Author, r.Genre, r.Language, derefInt(r.Year), rating, truncate(r.Description, 24), r.CoverURL)
		}
	}
}
