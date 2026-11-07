package service

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 腾讯动漫 ac.qq.com（中文正版源）
//
// 现有五个源（AniList / Bangumi / MangaDex / MangaUpdates / Kitsu）都是国际向
// 数据库，国漫与不少韩漫查不到，书架那一半没有封面和简介。腾讯动漫是中文侧
// 覆盖最全的一家，作为补充源接进来。
//
// 站点没有公开的 JSON 搜索接口，搜索页与详情页都是服务端渲染的 HTML；
// 实测（2026-11）免登录、无签名、不校验 User-Agent：
//
//	搜索  GET https://ac.qq.com/Comic/searchList?search=<关键词>
//	      结果条目：<a href="/Comic/comicInfo/id/{id}" title="{标题}" class="mod_book_cover db">
//	      封面：<img data-original='https://manhua.acimg.cn/vertical/....jpg/420'>
//	      （末尾的 /420 是必需的缩放段，去掉会 400，所以原样保留）
//	详情  GET https://ac.qq.com/Comic/comicInfo/id/{id}
//	      文档末尾有 schema.org 的 ComicSeries JSON-LD，含 name / author /
//	      publisher / genre / description / inLanguage / aggregateRating，
//	      比解析正文 DOM 稳得多。
//
// 详情页数百 KB 且 JSON-LD 在文档末尾，没法只读头部，所以只对搜索前排候选拉
// 详情；并发上限与 Bangumi 一致（5）。
// ============================================================

const (
	tencentBaseURL    = "https://ac.qq.com"
	tencentSourceName = "tencent"
	// 详情页很大，只对搜索结果的头部若干条补全详情
	tencentDetailLimit = 6
	// 搜索结果最多取这么多条
	tencentSearchLimit = 12
	// 防御性上限：单页最多读 4MB，避免站点改版返回超大文档把内存吃光
	tencentMaxBodyBytes = 4 << 20
)

var (
	// 搜索结果条目：href + title + class="mod_book_cover ..."，到 </a> 之间含封面
	tencentSearchItemRe = regexp.MustCompile(`(?s)href="/Comic/comicInfo/id/(\d+)"\s+title="([^"]*)"[^>]*class="mod_book_cover[^"]*"[^>]*>(.*?)</a>`)
	tencentCoverURLRe   = regexp.MustCompile(`data-original='([^']+)'`)
	tencentJSONLDRe     = regexp.MustCompile(`(?s)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)
	// 用户直接粘贴作品链接或纯数字 ID 时，不再走搜索（有些作品站内搜不到）
	tencentComicIDRe = regexp.MustCompile(`(?i)(?:ac\.qq\.com)?/?(?:comic/)?comicinfo/id/(\d{2,})|^\s*(\d{2,})\s*$`)
)

// tencentComicIDFromQuery 从用户输入里识别作品 ID。
// 支持三种写法：纯数字、完整详情页链接、省略协议的主机路径。
func tencentComicIDFromQuery(query string) string {
	if m := tencentComicIDRe.FindStringSubmatch(strings.TrimSpace(query)); m != nil {
		if m[1] != "" {
			return m[1]
		}
		if m[2] != "" {
			return m[2]
		}
	}
	return ""
}

// tencentSearchItem 是搜索结果页里的一条条目（只有 ID、标题与封面）。
type tencentSearchItem struct {
	id       string
	title    string
	coverURL string
}

// tencentComicSeries 对应详情页 JSON-LD 里的 schema.org ComicSeries。
type tencentComicSeries struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	Image       string `json:"image"`
	Description string `json:"description"`
	InLanguage  string `json:"inLanguage"`
	Author      struct {
		Name string `json:"name"`
	} `json:"author"`
	Publisher struct {
		Name string `json:"name"`
	} `json:"publisher"`
	// genre 实测是数组，但站点改版可能退化成字符串，两种都接住
	Genre json.RawMessage `json:"genre"`
	// 评分字段同理：数字或字符串都接住，解析失败只丢评分不丢整条元数据
	Rating *struct {
		RatingValue json.RawMessage `json:"ratingValue"`
		BestRating  json.RawMessage `json:"bestRating"`
	} `json:"aggregateRating"`
}

// SearchTencentComic 在腾讯动漫搜索漫画，并给前排候选补全详情。
//
// 如果 query 是一个作品 ID 或详情页链接，直接按 ID 取详情，跳过搜索——
// 站内有一部分作品（如改版遗留条目）不出现在搜索索引里，只能按 ID 访问。
func SearchTencentComic(query, lang string) []ComicMetadata {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	if id := tencentComicIDFromQuery(q); id != "" {
		return searchTencentByID(id)
	}

	items := fetchTencentSearchItems(fmt.Sprintf("%s/Comic/searchList?search=%s", tencentBaseURL, url.QueryEscape(q)))
	if len(items) == 0 {
		return nil
	}

	// 详情拉不到的条目仍保留标题与封面，不至于整源为空
	detailCount := len(items)
	if detailCount > tencentDetailLimit {
		detailCount = tencentDetailLimit
	}

	details := make([]*tencentComicSeries, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i := 0; i < detailCount; i++ {
		wg.Add(1)
		go func(idx int, comicID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			details[idx] = fetchTencentComicSeries(comicID)
		}(i, items[i].id)
	}
	wg.Wait()

	results := make([]ComicMetadata, 0, len(items))
	for i, item := range items {
		results = append(results, buildTencentMetadata(item, details[i]))
	}
	return results
}

// searchTencentByID 直接按作品 ID 取详情，用于搜索索引里取不到的作品。
func searchTencentByID(comicID string) []ComicMetadata {
	series := fetchTencentComicSeries(comicID)
	if series == nil {
		log.Printf("[metadata] 腾讯动漫按 ID 取详情失败: id=%s", comicID)
		return nil
	}
	// 详情页已有正式标题与封面，搜索条目只是兜底，构造一个空壳交给同一套合并逻辑
	item := tencentSearchItem{id: comicID, title: strings.TrimSpace(series.Name)}
	if cover := normalizeTencentCoverURL(series.Image); cover != "" {
		item.coverURL = cover
	}
	return []ComicMetadata{buildTencentMetadata(item, series)}
}

// fetchTencentSearchItems 取回并解析搜索结果页。
func fetchTencentSearchItems(u string) []tencentSearchItem {
	return parseTencentSearchItems(tencentGet(u, 15*time.Second))
}

// parseTencentSearchItems 按站内顺序解析结果条目并去重。
func parseTencentSearchItems(body string) []tencentSearchItem {
	if body == "" {
		return nil
	}

	seen := make(map[string]bool)
	var items []tencentSearchItem
	for _, m := range tencentSearchItemRe.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		item := tencentSearchItem{
			id:    id,
			title: html.UnescapeString(strings.TrimSpace(m[2])),
		}
		if cover := tencentCoverURLRe.FindStringSubmatch(m[3]); len(cover) > 1 {
			item.coverURL = normalizeTencentCoverURL(cover[1])
		}
		if item.title == "" {
			continue
		}
		items = append(items, item)
		if len(items) >= tencentSearchLimit {
			break
		}
	}
	return items
}

// fetchTencentComicSeries 拉详情页并取出 ComicSeries JSON-LD。
func fetchTencentComicSeries(comicID string) *tencentComicSeries {
	body := tencentGet(fmt.Sprintf("%s/Comic/comicInfo/id/%s", tencentBaseURL, comicID), 20*time.Second)
	if body == "" {
		log.Printf("[metadata] 腾讯动漫详情获取失败: id=%s", comicID)
		return nil
	}
	return parseTencentComicSeries(body)
}

// parseTencentComicSeries 从详情页的 JSON-LD 块里挑出 ComicSeries。
func parseTencentComicSeries(body string) *tencentComicSeries {
	for _, raw := range tencentJSONLDRe.FindAllStringSubmatch(body, -1) {
		var series tencentComicSeries
		if err := json.Unmarshal([]byte(strings.TrimSpace(raw[1])), &series); err != nil {
			continue
		}
		if series.Type == "ComicSeries" && series.Name != "" {
			return &series
		}
	}
	return nil
}

// buildTencentMetadata 合并搜索条目与详情 JSON-LD，详情缺失时退回搜索条目。
func buildTencentMetadata(item tencentSearchItem, series *tencentComicSeries) ComicMetadata {
	meta := ComicMetadata{
		Title:    item.title,
		CoverURL: item.coverURL,
		Source:   tencentSourceName,
	}
	if series == nil {
		return meta
	}

	// 搜索标题常带括号别名（"中国惊奇先生（神鬼七杀令）"），
	// JSON-LD 的 name 是站方正式标题，优先用它。
	if name := strings.TrimSpace(series.Name); name != "" {
		meta.Title = name
	}
	if author := strings.TrimSpace(series.Author.Name); author != "" {
		meta.Author = author
	}
	if publisher := strings.TrimSpace(series.Publisher.Name); publisher != "" {
		meta.Publisher = publisher
	}
	if desc := strings.TrimSpace(series.Description); desc != "" {
		meta.Description = desc
	}
	if genres := parseTencentGenre(series.Genre); genres != "" {
		meta.Genre = genres
	}
	if lang := strings.TrimSpace(series.InLanguage); lang != "" {
		meta.Language = lang
	} else {
		meta.Language = "zh-CN"
	}
	if cover := normalizeTencentCoverURL(series.Image); cover != "" {
		meta.CoverURL = cover
	}

	// 站点给的是 0-10 真实打分（不是人气值），可直接对接 ExternalRating
	if series.Rating != nil {
		if score, ok := parseTencentFloat(series.Rating.RatingValue); ok && score > 0 {
			maxScore, okMax := parseTencentFloat(series.Rating.BestRating)
			if !okMax || maxScore <= 0 {
				maxScore = 10
			}
			meta.ExternalRating = &score
			meta.ExternalRatingMax = &maxScore
			meta.ExternalRatingSource = tencentSourceName
		}
	}
	return meta
}

// parseTencentGenre 兼容 genre 为数组或字符串两种形态。
func parseTencentGenre(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		var names []string
		for _, g := range list {
			if g = strings.TrimSpace(g); g != "" {
				names = append(names, g)
			}
		}
		return strings.Join(names, ", ")
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return strings.TrimSpace(single)
	}
	return ""
}

// parseTencentFloat 接住 JSON 里写成数字或字符串的浮点值。
func parseTencentFloat(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if f, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// normalizeTencentCoverURL 补协议并保留缩放段：manhua.acimg.cn 的
// "...jpg/420" 去掉 /420 会 400，所以这里只处理协议相对与 http 前缀。
func normalizeTencentCoverURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return raw
}

// tencentGet 取回文本响应体，失败返回空串。
func tencentGet(u string, timeout time.Duration) string {
	resp, err := httpGet(u, map[string]string{
		"User-Agent": "NowenReader/1.0",
		"Accept":     "text/html,application/xhtml+xml",
	}, timeout)
	if err != nil {
		log.Printf("[metadata] 腾讯动漫请求失败 %s: %v", u, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, tencentMaxBodyBytes))
	if err != nil {
		log.Printf("[metadata] 腾讯动漫读取响应失败 %s: %v", u, err)
		return ""
	}
	return string(body)
}
