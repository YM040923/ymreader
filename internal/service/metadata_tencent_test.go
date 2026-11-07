package service

import (
	"os"
	"strings"
	"testing"
)

// 下面的 fixture 取自 ac.qq.com 的真实响应结构（2026-11 实测），
// 站点改版时这些用例会先红，比线上刮到空结果更容易发现。

const tencentSearchFixture = `<ul class="mod_book_list mod_all_works_list mod_of">
  <li>
    <a href="/Comic/comicInfo/id/652762" title="超神宠兽店" class="mod_book_cover db" target="_blank">
      <img src='//gtimgcdn.ac.qq.com/images/blank.gif' data-original='https://manhua.acimg.cn/vertical/0/02_18_06_d7a4.jpg/420' class='img100x133 lazy' />
      <div class="mod_book_detail">
        <div class="mod_book_detail_mask"></div>
        <div class="mod_book_detail_info"><h3 class="mod_book_update fw">更新至230话</h3></div>
      </div>
    </a>
    <h4 class="mod_book_name"><a href="/Comic/comicInfo/id/652762" title="超神宠兽店" class="f14" target="_blank">超神宠兽店</a></h4>
  </li>
  <li>
    <a href="/Comic/comicInfo/id/511915" title="中国惊奇先生（神鬼七杀令）" class="mod_book_cover db" target="_blank">
      <img src='//gtimgcdn.ac.qq.com/images/blank.gif' data-original='//manhua.acimg.cn/vertical/0/17_16_53.jpg/420' class='img100x133 lazy' />
      <div class="mod_book_detail">
        <div class="mod_book_detail_info"><h3 class="mod_book_update fw">更新至1841话</h3></div>
      </div>
    </a>
  </li>
  <li>
    <a href="/Comic/comicInfo/id/700001" title="带 &amp; 实体的标题" class="mod_book_cover db" target="_blank">
      <div class="mod_book_detail"><div class="mod_book_detail_info"><h3 class="mod_book_update fw">全43话</h3></div></div>
    </a>
  </li>
</ul>`

const tencentDetailFixture = `<html><head><title>《超神宠兽店》漫画_周不困 x 诶嘿工作室_全集免费在线阅读_腾讯动漫</title></head><body>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[]}</script>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"ComicSeries","name":"超神宠兽店","url":"https://ac.qq.com/Comic/comicInfo/id/652762","image":"https://manhua.acimg.cn/vertical/0/02_18_06_d7a4.jpg/420","author":{"@type":"Person","name":"周不困 x 诶嘿工作室"},"description":"宠兽时代，宠兽实力决定一切！","genre":["战斗","怪物"],"publisher":{"@type":"Organization","name":"腾讯动漫"},"inLanguage":"zh-CN","creativeWorkStatus":"Ongoing","aggregateRating":{"@type":"AggregateRating","ratingValue":9,"ratingCount":4001,"bestRating":10,"worstRating":0}}</script>
</body></html>`

func TestParseTencentSearchItems(t *testing.T) {
	items := parseTencentSearchItems(tencentSearchFixture)
	if len(items) != 3 {
		t.Fatalf("解析条目数 = %d, want 3", len(items))
	}
	if items[0].id != "652762" || items[0].title != "超神宠兽店" {
		t.Fatalf("首条 = %+v, want 652762/超神宠兽店", items[0])
	}
	if !strings.HasPrefix(items[0].coverURL, "https://manhua.acimg.cn/") {
		t.Fatalf("封面地址 = %q, 应为 https 绝对地址", items[0].coverURL)
	}
	// 协议相对的懒加载地址要补上 https
	if items[1].coverURL != "https://manhua.acimg.cn/vertical/0/17_16_53.jpg/420" {
		t.Fatalf("协议相对封面 = %q", items[1].coverURL)
	}
	// 括号别名属于标题的一部分，保持原样交给 relevance 排序处理
	if items[1].title != "中国惊奇先生（神鬼七杀令）" {
		t.Fatalf("别名标题 = %q", items[1].title)
	}
	if items[2].coverURL != "" {
		t.Fatalf("无封面条目不应有封面地址: %q", items[2].coverURL)
	}
	if !strings.Contains(items[2].title, "&") {
		t.Fatalf("HTML 实体未还原: %q", items[2].title)
	}
}

func TestParseTencentComicSeries(t *testing.T) {
	series := parseTencentComicSeries(tencentDetailFixture)
	if series == nil {
		t.Fatal("未解析出 ComicSeries")
	}
	if series.Name != "超神宠兽店" || series.Author.Name != "周不困 x 诶嘿工作室" {
		t.Fatalf("标题/作者 = %q / %q", series.Name, series.Author.Name)
	}
	if series.Publisher.Name != "腾讯动漫" || series.InLanguage != "zh-CN" {
		t.Fatalf("出版方/语言 = %q / %q", series.Publisher.Name, series.InLanguage)
	}
	if got := parseTencentGenre(series.Genre); got != "战斗, 怪物" {
		t.Fatalf("genre = %q, want '战斗, 怪物'", got)
	}
	if series.Rating == nil {
		t.Fatal("缺少 aggregateRating")
	}
	if score, ok := parseTencentFloat(series.Rating.RatingValue); !ok || score != 9 {
		t.Fatalf("评分 = %v (%v), want 9", score, ok)
	}
}

func TestBuildTencentMetadataFromFixture(t *testing.T) {
	items := parseTencentSearchItems(tencentSearchFixture)
	series := parseTencentComicSeries(tencentDetailFixture)
	meta := buildTencentMetadata(items[0], series)

	if meta.Source != tencentSourceName {
		t.Fatalf("Source = %q", meta.Source)
	}
	if meta.Title != "超神宠兽店" || meta.Author == "" || meta.Description == "" {
		t.Fatalf("字段不完整: %+v", meta)
	}
	if meta.Language != "zh-CN" {
		t.Fatalf("Language = %q", meta.Language)
	}
	if meta.ExternalRating == nil || meta.ExternalRatingMax == nil || *meta.ExternalRatingMax != 10 {
		t.Fatalf("评分字段 = %+v / %+v", meta.ExternalRating, meta.ExternalRatingMax)
	}
	if meta.ExternalRatingSource != tencentSourceName {
		t.Fatalf("评分来源 = %q", meta.ExternalRatingSource)
	}

	// 详情拉取失败时必须还能给出标题与封面，不能整条丢掉
	fallback := buildTencentMetadata(items[1], nil)
	if fallback.Title != "中国惊奇先生（神鬼七杀令）" || fallback.CoverURL == "" {
		t.Fatalf("详情缺失时的兜底结果不完整: %+v", fallback)
	}
}

func TestParseTencentGenreAndRatingVariants(t *testing.T) {
	variant := `<script type="application/ld+json">{"@type":"ComicSeries","name":"某漫画","genre":"热血","aggregateRating":{"ratingValue":"9.3","bestRating":"10"}}</script>`
	series := parseTencentComicSeries(variant)
	if series == nil {
		t.Fatal("字符串形态的 genre/评分未解析")
	}
	if got := parseTencentGenre(series.Genre); got != "热血" {
		t.Fatalf("字符串 genre = %q", got)
	}
	if series.Rating == nil {
		t.Fatal("字符串形态评分丢失")
	}
	if score, ok := parseTencentFloat(series.Rating.RatingValue); !ok || score != 9.3 {
		t.Fatalf("字符串评分 = %v (%v), want 9.3", score, ok)
	}
	if maxScore, ok := parseTencentFloat(series.Rating.BestRating); !ok || maxScore != 10 {
		t.Fatalf("字符串满分 = %v (%v), want 10", maxScore, ok)
	}
}

func TestNormalizeTencentCoverURLKeepsSizeSuffix(t *testing.T) {
	// ac.qq.com 的封面必须带 /420 缩放段，去掉会 400，所以不能被"优化"掉
	got := normalizeTencentCoverURL("https://manhua.acimg.cn/vertical/0/x.jpg/420")
	if got != "https://manhua.acimg.cn/vertical/0/x.jpg/420" {
		t.Fatalf("封面地址被改写: %q", got)
	}
}

// TestSearchTencentComicLive 打线上接口，默认跳过；
// 需要看真实命中率时：TENCENT_LIVE=1 go test ./internal/service -run TencentLive -v
func TestSearchTencentComicLive(t *testing.T) {
	if os.Getenv("TENCENT_LIVE") == "" {
		t.Skip("set TENCENT_LIVE=1 to hit ac.qq.com")
	}
	// 前四个是站内热门，后三个取自真实本地库目录名，用来观察命中率
	for _, q := range []string{"超神宠兽店", "斗破苍穹", "一人之下", "狐妖小红娘", "绝对剑感", "某天成为魔神", "魔道转生记"} {
		// 走 SearchMetadata 而不是直接调 SearchTencentComic：
		// 顺带验证 "tencent" 在 doSearch 的 switch 里注册成功，并经过 relevance 排序
		results := SearchMetadata(q, []string{"tencent"}, "zh-CN")
		if len(results) == 0 {
			t.Errorf("[%s] 零结果", q)
			continue
		}
		t.Logf("[%s] 共 %d 条", q, len(results))
		limit := 3
		if len(results) < limit {
			limit = len(results)
		}
		for _, top := range results[:limit] {
			t.Logf("    %q 作者=%q 标签=%q 评分=%v 简介=%q 封面=%q",
				top.Title, top.Author, top.Genre, derefRating(top.ExternalRating),
				truncate(top.Description, 24), top.CoverURL)
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func derefRating(v *float64) any {
	if v == nil {
		return "无"
	}
	return *v
}

func TestTencentComicIDFromQuery(t *testing.T) {
	cases := map[string]string{
		"649765":     "649765",
		"  649765  ": "649765",
		"https://ac.qq.com/Comic/comicInfo/id/649765": "649765",
		"http://ac.qq.com/Comic/comicInfo/id/649765":  "649765",
		"ac.qq.com/Comic/comicInfo/id/649765":         "649765",
		"/Comic/comicInfo/id/649765":                  "649765",
		"大龟甲师":                                        "",
		"神印王座外传 大龟甲师":                                 "",
		"神印王座":                                        "",
		"one piece":                                   "",
	}
	for input, want := range cases {
		if got := tencentComicIDFromQuery(input); got != want {
			t.Errorf("tencentComicIDFromQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestSearchTencentComicByIDLive 验证按 ID 直取绕过了搜索索引。
// 649765（神印王座外传 大龟甲师）站内搜索查不到，只能按 ID 访问。
func TestSearchTencentComicByIDLive(t *testing.T) {
	if os.Getenv("TENCENT_LIVE") == "" {
		t.Skip("set TENCENT_LIVE=1 to hit ac.qq.com")
	}
	for _, query := range []string{
		"649765",
		"https://ac.qq.com/Comic/comicInfo/id/649765",
	} {
		results := SearchMetadata(query, []string{"tencent"}, "zh-CN")
		if len(results) != 1 {
			t.Fatalf("[%s] 期望 1 条，实际 %d 条", query, len(results))
		}
		r := results[0]
		if r.Title != "神印王座外传 大龟甲师" {
			t.Errorf("[%s] 标题 = %q", query, r.Title)
		}
		if r.Author == "" || r.CoverURL == "" {
			t.Errorf("[%s] 作者/封面缺失: %+v", query, r)
		}
		if r.ExternalRating == nil || *r.ExternalRating <= 0 {
			t.Errorf("[%s] 评分缺失", query)
		}
		t.Logf("[%s] %s | 作者=%s | 分类=%s | 评分=%v", query, r.Title, r.Author, r.Genre, *r.ExternalRating)
	}
}
