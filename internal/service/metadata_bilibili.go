package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 哔哩哔哩漫画 manga.bilibili.com
//
// 站点是 Nuxt 客户端渲染，详情页 HTML 里没有任何元数据，只能走它自己的 twirp
// 接口。实测（2026-11）契约：
//
//	搜索  POST https://manga.bilibili.com/twirp/comic.v1.Comic/Search
//	      ?mobi_app=android_comic&device=android&platform=android
//	      body {"keyWord":"关键词","pageSize":10,"pageNum":1}
//	      → data.list[]；title 带 <em class="keyword"> 高亮，需要剥掉
//	      ⚠ 查询串带上 version=5.21.0 时 list 是空的，只回 recommends，
//	        所以 version 这个参数不能带。
//	详情  POST https://manga.bilibili.com/twirp/comic.v1.Comic/ComicDetail
//	      同一组参数，body {"comic_id":25332}
//	      ⚠ 只有 App 协议给明文，device=pc&platform=web 返回 code=99。
//
// 两个接口都要求带上访客指纹 cookie。实测匿名 GET https://www.bilibili.com/
// 时主站会 Set-Cookie: buvid3，只要回传 buvid3 接口就给 code=0——不需要登录、
// 不需要 SESSDATA、不需要任何签名。
//
// 详情页响应里 ep_list 可能有几百条（境·界 708 话，约 1MB），所以只解需要的字段
// 并给响应体设上限。
// ============================================================

const (
	biliBaseURL    = "https://manga.bilibili.com"
	biliSourceName = "bilibili"
	// App 协议三件套：只有这套参数返回明文数据
	biliAppQuery = "mobi_app=android_comic&device=android&platform=android"
	// 站点校验 Referer，值本身不敏感
	biliUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	// 详情响应很大，只给前排若干条补详情
	biliDetailLimit = 5
	// 搜索返回条数
	biliSearchPageSize = 10
	// 详情页最多读 8MB，防御站点改版
	biliMaxBodyBytes = 8 << 20
	// buvid3 指纹的本地缓存时长，过期后重新引导
	biliCookieTTL = 6 * time.Hour
)

var (
	biliEmTagRe = regexp.MustCompile(`</?em[^>]*>`)
	biliYearRe  = regexp.MustCompile(`(?:19|20)\d{2}`)
)

// ============================================================
// 接口响应结构
// ============================================================

// biliEnvelope 是 twirp 接口统一的响应外壳。
type biliEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// biliSearchItem 是搜索接口返回的一条漫画。
type biliSearchItem struct {
	ID         int      `json:"id"`
	Title      string   `json:"title"`
	OrgTitle   string   `json:"org_title"`
	AliaTitle  []string `json:"alia_title"`
	AuthorName []string `json:"author_name"`
	Styles     []string `json:"styles"`
	Recommend  string   `json:"recommendation"`
	Total      int      `json:"total"`
	Vertical   string   `json:"vertical_cover"`
	Horizontal string   `json:"horizontal_cover"`
	Square     string   `json:"square_cover"`
	Wiki       struct {
		PublishTime string `json:"publish_time"`
	} `json:"wiki"`
}

// biliComicDetail 是详情接口里真正用到的字段，其余（ep_list 等）直接忽略。
type biliComicDetail struct {
	ID         int      `json:"id"`
	Title      string   `json:"title"`
	ComicAlias []string `json:"comic_alias"`
	AuthorName []string `json:"author_name"`
	Styles     []string `json:"styles"`
	Tags       []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Evaluate     string `json:"evaluate"`
	Introduction string `json:"introduction"`
	ReleaseTime  string `json:"release_time"`
	RenewalTime  string `json:"renewal_time"`
	Total        int    `json:"total"`
	Vertical     string `json:"vertical_cover"`
	Horizontal   string `json:"horizontal_cover"`
	Square       string `json:"square_cover"`
	JapanComic   bool   `json:"japan_comic"`
}

// ============================================================
// 匿名 buvid3 cookie 引导
// ============================================================

var (
	biliCookieMu    sync.Mutex
	biliCookieValue string
	biliCookieAt    time.Time
)

// biliCookie 返回可用的 buvid3；force 为真时忽略缓存重新向主站引导。
// 站点不校验 UA 与签名，只校验有没有这个访客指纹。
func biliCookie(force bool) string {
	biliCookieMu.Lock()
	defer biliCookieMu.Unlock()

	if !force && biliCookieValue != "" && time.Since(biliCookieAt) < biliCookieTTL {
		return biliCookieValue
	}

	resp, err := httpGet("https://www.bilibili.com/", map[string]string{
		"User-Agent": biliUserAgent,
		"Accept":     "text/html,application/xhtml+xml",
	}, 15*time.Second)
	if err != nil {
		log.Printf("[metadata] 哔哩哔哩访客 cookie 引导失败: %v", err)
		return biliCookieValue
	}
	defer resp.Body.Close()

	for _, c := range resp.Cookies() {
		if c.Name == "buvid3" && c.Value != "" {
			biliCookieValue = "buvid3=" + c.Value
			biliCookieAt = time.Now()
			return biliCookieValue
		}
	}
	// 主站没下发 buvid3（改版或被风控），留着旧值继续试
	log.Printf("[metadata] 哔哩哔哩主站未下发 buvid3，接口可能返回 code=99")
	return biliCookieValue
}

// ============================================================
// 入口
// ============================================================

// SearchBilibiliComic 在哔哩哔哩漫画搜索漫画，并为前排结果补全详情。
func SearchBilibiliComic(query, lang string) []ComicMetadata {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	items := fetchBiliSearchItems(q)
	if len(items) == 0 {
		return nil
	}

	detailCount := len(items)
	if detailCount > biliDetailLimit {
		detailCount = biliDetailLimit
	}

	// 并发补详情，与 Bangumi 一致（最多 5 个在飞）
	details := make([]*biliComicDetail, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i := 0; i < detailCount; i++ {
		wg.Add(1)
		go func(idx, comicID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			details[idx] = fetchBiliComicDetail(comicID)
		}(i, items[i].ID)
	}
	wg.Wait()

	results := make([]ComicMetadata, 0, len(items))
	for i, item := range items {
		results = append(results, buildBiliMetadata(item, details[i]))
	}
	return results
}

// ============================================================
// 搜索与详情
// ============================================================

func fetchBiliSearchItems(query string) []biliSearchItem {
	var data struct {
		List []biliSearchItem `json:"list"`
	}
	if _, ok := biliPost("/twirp/comic.v1.Comic/Search", map[string]any{
		"keyWord":  query,
		"pageSize": biliSearchPageSize,
		"pageNum":  1,
	}, &data); !ok {
		return nil
	}
	if len(data.List) == 0 {
		return nil
	}

	out := make([]biliSearchItem, 0, len(data.List))
	for _, item := range data.List {
		if item.ID <= 0 {
			continue
		}
		item.Title = strings.TrimSpace(biliStripTags(item.Title))
		if item.Title == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func fetchBiliComicDetail(comicID int) *biliComicDetail {
	var detail biliComicDetail
	if _, ok := biliPost("/twirp/comic.v1.Comic/ComicDetail", map[string]any{
		"comic_id": comicID,
	}, &detail); !ok {
		return nil
	}
	if detail.ID == 0 || strings.TrimSpace(detail.Title) == "" {
		return nil
	}
	return &detail
}

// biliPost 调用 twirp 接口并把 data 解进 out。
// buvid3 会过期，code=99 时强制重新引导一次再重试；其余错误码直接放弃。
func biliPost(path string, payload map[string]any, out any) (json.RawMessage, bool) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}

	var last json.RawMessage
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := httpPostJSON(biliBaseURL+path+"?"+biliAppQuery, body, map[string]string{
			"User-Agent": biliUserAgent,
			"Accept":     "application/json",
			"Referer":    biliBaseURL + "/",
			"Cookie":     biliCookie(attempt > 0),
		}, 20*time.Second)
		if err != nil {
			log.Printf("[metadata] 哔哩哔哩接口 %s 请求失败: %v", path, err)
			return nil, false
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, biliMaxBodyBytes))
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[metadata] 哔哩哔哩接口 %s 读取失败: %v", path, readErr)
			return nil, false
		}

		var env biliEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			log.Printf("[metadata] 哔哩哔哩接口 %s 非 JSON 响应: %v", path, err)
			return nil, false
		}
		last = env.Data

		if env.Code == 0 && len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, out); err != nil {
				log.Printf("[metadata] 哔哩哔哩接口 %s 解析失败: %v", path, err)
				return env.Data, false
			}
			return env.Data, true
		}

		if env.Code != 99 {
			log.Printf("[metadata] 哔哩哔哩接口 %s code=%d msg=%s", path, env.Code, env.Msg)
			return env.Data, false
		}
		log.Printf("[metadata] 哔哩哔哩接口 %s 返回 code=99，刷新访客 cookie 后重试", path)
	}
	return last, false
}

// ============================================================
// 组装 ComicMetadata
// ============================================================

func buildBiliMetadata(item biliSearchItem, detail *biliComicDetail) ComicMetadata {
	meta := ComicMetadata{
		Title:    item.Title,
		Author:   joinBiliNames(item.AuthorName),
		Genre:    joinBiliNames(item.Styles),
		CoverURL: biliFirstCover(item.Vertical, item.Horizontal, item.Square),
		Source:   biliSourceName,
	}
	if desc := strings.TrimSpace(biliStripTags(item.Recommend)); desc != "" {
		meta.Description = desc
	}
	if year := biliYear(item.Wiki.PublishTime); year > 0 {
		y := year
		meta.Year = &y
	}
	// 详情缺失时不下猜语言：站点没有语言字段，只有日漫标记可用。
	if detail == nil {
		return meta
	}

	if title := strings.TrimSpace(detail.Title); title != "" {
		meta.Title = title
	}
	if author := joinBiliNames(detail.AuthorName); author != "" {
		meta.Author = author
	}
	if genre := joinBiliNames(biliGenres(detail)); genre != "" {
		meta.Genre = genre
	}
	// evaluate 是长简介，introduction 是一句话推荐语，刮削要的是前者
	if desc := strings.TrimSpace(detail.Evaluate); desc != "" {
		meta.Description = desc
	} else if desc := strings.TrimSpace(detail.Introduction); desc != "" {
		meta.Description = desc
	}
	if cover := biliFirstCover(detail.Vertical, detail.Horizontal, detail.Square); cover != "" {
		meta.CoverURL = cover
	}
	if year := biliYear(detail.ReleaseTime); year > 0 {
		y := year
		meta.Year = &y
	} else if year := biliYear(detail.RenewalTime); year > 0 {
		y := year
		meta.Year = &y
	}
	if detail.JapanComic {
		meta.Language = "ja"
	} else {
		meta.Language = "zh-CN"
	}
	return meta
}

// biliGenres 合并 styles 与 tags；tags 通常是更细的情感标签。
func biliGenres(detail *biliComicDetail) []string {
	var out []string
	out = append(out, detail.Styles...)
	for _, t := range detail.Tags {
		out = append(out, t.Name)
	}
	return out
}

// biliFirstCover 按优先级取第一个可用封面，并统一成 https。
func biliFirstCover(urls ...string) string {
	for _, u := range urls {
		if normalized := normalizeBiliCoverURL(u); normalized != "" {
			return normalized
		}
	}
	return ""
}

// biliYear 从 "2020-01-01 00:00:00" 这类字符串里取年份，取不到返回 0。
// 站点用 Go 零值时间 0001-01-01 表示"没有日期"，这种情况也当取不到。
func biliYear(value string) int {
	m := biliYearRe.FindString(value)
	if m == "" {
		return 0
	}
	var y int
	if _, err := fmt.Sscanf(m, "%d", &y); err != nil || y <= 1 {
		return 0
	}
	return y
}

// normalizeBiliCoverURL 把站点里残留的 http:// 与协议相对地址统一成 https，
// 避免 HTTPS 环境下混合内容被浏览器拦掉。
func normalizeBiliCoverURL(raw string) string {
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

// joinBiliNames 去空、去重后用逗号连接。
func joinBiliNames(names []string) string {
	var kept []string
	seen := make(map[string]bool)
	for _, n := range names {
		n = strings.TrimSpace(biliStripTags(n))
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		kept = append(kept, n)
	}
	return strings.Join(kept, ", ")
}

// biliStripTags 先剥掉搜索接口的高亮 <em>，再走公共的剥 HTML 逻辑。
func biliStripTags(s string) string {
	return stripHTML(biliEmTagRe.ReplaceAllString(s, ""))
}
