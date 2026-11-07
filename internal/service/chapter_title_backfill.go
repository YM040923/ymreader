package service

import (
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ============================================================
// 章节标题回填
//
// 本地库里有不少单元只有「第12话」这种编号（源文件夹就没有标题）。
// 这里按作品名匹配腾讯动漫，抓它的章节列表，再按话号把标题补上：
//
//	第12话        +  永夜降临   →  第12话 永夜降临
//	番外3         +  转世的御扶摇 →  番外3 转世的御扶摇
//
// 只存「标题」本身（不存「第12话 永夜降临」），这样以后调整编号格式
// 不会把旧数据格式带回来。匹配不上的作品保持原样，不做猜测。
// ============================================================

const (
	chapterTitleMaxBodyBytes = 8 << 20
	chapterTitleMaxRunes     = 100
)

var (
	tencentChapterAnchorRe = regexp.MustCompile(`(?s)<a[^>]*href="/ComicView/index/id/(\d+)/cid/(\d+)"[^>]*>(.*?)</a>`)

	// 章节名有两种常见写法：带编号（「01 我死了？」「第12话 标题」）
	// 和不带编号。只有前者能可靠地对到本地话号。
	tencentChapterTitlePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^第\s*(\d+(?:\.\d+)?)\s*[话話]\s*[·:：\-_ ]?\s*(.+)$`),
		regexp.MustCompile(`^总\s*(\d+(?:\.\d+)?)\s*[·:：\-_ ]\s*(.+)$`),
		// 编号后是空格：01 我死了？ / 1.5 番外小剧场
		regexp.MustCompile(`^(\d+(?:\.\d+)?)\s+(.+)$`),
		// 编号后用中日文分隔符
		regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*[、·:：\-_]\s*(.+)$`),
		// 1.学生怎么能撩妹(I)；点号后面必须不是数字，避免把 1.5 拆成 1 + 5
		regexp.MustCompile(`^(\d+)\.(\D.*)$`),
	}
	tencentBonusTitleRe = regexp.MustCompile(`^番外\s*(\d+)?\s*[-_·:： ]?\s*(.+)$`)

	unitHeadingOnlyRe      = regexp.MustCompile(`^第\s*(\d+(?:\.\d+)?)\s*[话話]$`)
	unitBonusHeadingOnlyRe = regexp.MustCompile(`^番外\s*(\d*)$`)

	chapterTitlePunctuation = strings.NewReplacer(
		"。", "", "！", "", "？", "", "!", "", "?", "", "：", "", ":", "",
		"·", "", "、", "", "，", "", ",", "", ".", "", "-", "", "—", "",
		"–", "", "_", "", "（", "", "）", "", "(", "", ")", "",
		"《", "", "》", "", "「", "", "」", "", "【", "", "】", "",
		"[", "", "]", "", " ", "", "\u3000", "",
	)
)

// ChapterTitleSample is a small preview item returned by the API.
type ChapterTitleSample struct {
	UnitLabel string `json:"unitLabel"`
	Title     string `json:"title"`
}

// ChapterTitleOverride is one unitId -> title mapping ready to persist.
type ChapterTitleOverride struct {
	UnitID string
	WorkID string
	Title  string
	Source string
}

// ChapterTitleBackfillResult describes what a backfill run found for one Work.
type ChapterTitleBackfillResult struct {
	WorkID      string                 `json:"workId"`
	WorkTitle   string                 `json:"workTitle"`
	SourceID    string                 `json:"sourceId,omitempty"`
	SourceTitle string                 `json:"sourceTitle,omitempty"`
	Needed      int                    `json:"needed"`
	Matched     int                    `json:"matched"`
	DryRun      bool                   `json:"dryRun,omitempty"`
	Samples     []ChapterTitleSample   `json:"samples,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Overrides   []ChapterTitleOverride `json:"-"`
}

// WorkHasTitlelessChapters reports whether any unit still only has a chapter
// number. Used to skip works that need no network work at all.
func WorkHasTitlelessChapters(work Work) bool {
	for _, unit := range work.Units {
		if _, ok := unitTitleKey(unit.DisplayLabel); ok {
			return true
		}
	}
	return false
}

// CollectChapterTitleBackfill matches one Work against Tencent Comic and
// returns the titles that can be filled. It performs network requests.
func CollectChapterTitleBackfill(work Work, dryRun bool) ChapterTitleBackfillResult {
	result := ChapterTitleBackfillResult{
		WorkID:    work.ID,
		WorkTitle: work.Title,
		DryRun:    dryRun,
	}

	needed := make(map[string]string) // 章节键 -> 原始展示名
	for _, unit := range work.Units {
		if key, ok := unitTitleKey(unit.DisplayLabel); ok {
			needed[key] = unit.DisplayLabel
		}
	}
	result.Needed = len(needed)
	if len(needed) == 0 {
		return result
	}

	comicID, sourceTitle, matchedBy := findTencentComicForTitle(work.Title)
	if comicID == "" {
		result.Error = "腾讯动漫没有找到同名作品"
		return result
	}
	result.SourceID = comicID
	result.SourceTitle = sourceTitle

	titles, err := fetchTencentChapterTitles(comicID)
	if err != nil {
		result.Error = "腾讯动漫章节列表读取失败：" + err.Error()
		return result
	}
	if len(titles) == 0 {
		result.Error = "腾讯动漫章节列表没有可用的话号"
		return result
	}

	unitByKey := make(map[string]string, len(work.Units))
	for _, unit := range work.Units {
		if key, ok := unitTitleKey(unit.DisplayLabel); ok {
			unitByKey[key] = unit.ID
		}
	}
	for key, unitID := range unitByKey {
		title, ok := titles[key]
		if !ok || strings.TrimSpace(title) == "" {
			continue
		}
		result.Overrides = append(result.Overrides, ChapterTitleOverride{
			UnitID: unitID, WorkID: work.ID, Title: title, Source: "tencent",
		})
		if len(result.Samples) < 5 {
			result.Samples = append(result.Samples, ChapterTitleSample{
				UnitLabel: needed[key], Title: title,
			})
		}
	}
	result.Matched = len(result.Overrides)

	// 名称只是包含关系时（例如「大龟甲师」vs「神印王座外传 大龟甲师」），
	// 要求话号至少有六成对得上，避免把同名前传/外传的标题套进来。
	if matchedBy == "contains" && result.Needed >= 5 &&
		float64(result.Matched)/float64(result.Needed) < 0.6 {
		result.Overrides = nil
		result.Matched = 0
		result.Error = "腾讯动漫匹配到相似作品，但话号对不上，已跳过"
	}
	return result
}

// ApplyChapterTitleOverrides appends a backfilled title to heading-only
// labels such as 「第12话」. Labels that already carry a title are left alone,
// so re-running the backfill can never double-append.
func ApplyChapterTitleOverrides(works []Work, titles map[string]string) {
	if len(titles) == 0 {
		return
	}
	for workIndex := range works {
		for unitIndex := range works[workIndex].Units {
			unit := &works[workIndex].Units[unitIndex]
			title, ok := titles[unit.ID]
			title = strings.TrimSpace(title)
			if !ok || title == "" {
				continue
			}
			if _, headingOnly := unitTitleKey(unit.DisplayLabel); !headingOnly {
				continue
			}
			unit.DisplayLabel = strings.TrimSpace(unit.DisplayLabel + " " + title)
			if _, headingOnly := unitTitleKey(unit.Title); headingOnly {
				unit.Title = strings.TrimSpace(unit.Title + " " + title)
			}
		}
	}
}

// unitTitleKey returns a stable chapter key for labels that only carry a
// number (「第12话」「番外3」). Labels with a title or volumes return false.
func unitTitleKey(label string) (string, bool) {
	label = strings.TrimSpace(label)
	if m := unitHeadingOnlyRe.FindStringSubmatch(label); m != nil {
		return "第" + normalizeUnitNumber(m[1]), true
	}
	if m := unitBonusHeadingOnlyRe.FindStringSubmatch(label); m != nil {
		return "番外" + normalizeUnitNumber(m[1]), true
	}
	return "", false
}

// findTencentComicForTitle searches Tencent Comic by Work title and returns
// the first exact (normalized) match, then a containment match.
func findTencentComicForTitle(workTitle string) (comicID, matchedTitle, how string) {
	query := strings.TrimSpace(workTitle)
	if query == "" {
		return "", "", ""
	}
	items := fetchTencentSearchItems(fmt.Sprintf(
		"%s/Comic/searchList?search=%s", tencentBaseURL, url.QueryEscape(query),
	))
	if len(items) == 0 {
		return "", "", ""
	}
	target := normalizeChapterMatchTitle(query)
	if target == "" {
		return "", "", ""
	}
	for _, item := range items {
		if normalizeChapterMatchTitle(item.title) == target {
			return item.id, item.title, "exact"
		}
	}
	for _, item := range items {
		candidate := normalizeChapterMatchTitle(item.title)
		if len([]rune(candidate)) < 4 || len([]rune(target)) < 4 {
			continue
		}
		if strings.Contains(candidate, target) || strings.Contains(target, candidate) {
			return item.id, item.title, "contains"
		}
	}
	return "", "", ""
}

// fetchTencentChapterTitles downloads one comic detail page and parses its
// chapter list. It only needs the HTML header/anchors, but the page carries
// the whole list inline, so the body is capped defensively.
func fetchTencentChapterTitles(comicID string) (map[string]string, error) {
	endpoint := fmt.Sprintf("%s/Comic/comicInfo/id/%s?tab=1", tencentBaseURL, url.PathEscape(comicID))
	resp, err := httpGet(endpoint, map[string]string{
		"User-Agent": "NowenReader/1.0",
		"Accept":     "text/html,application/xhtml+xml",
	}, 25*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, chapterTitleMaxBodyBytes))
	if err != nil {
		return nil, err
	}
	return ParseTencentChapterTitles(string(body)), nil
}

// ParseTencentChapterTitles extracts chapter-number -> title mappings from a
// Tencent Comic detail page. Exported for tests.
func ParseTencentChapterTitles(page string) map[string]string {
	texts := make(map[string]string)
	for _, match := range tencentChapterAnchorRe.FindAllStringSubmatch(page, -1) {
		cid := match[2]
		text := cleanTencentChapterText(match[3])
		if text == "" {
			continue
		}
		if existing := texts[cid]; existing != "" {
			continue
		}
		texts[cid] = text
	}

	result := make(map[string]string)
	for _, text := range texts {
		key, title, ok := parseTencentChapterText(text)
		if !ok || title == "" {
			continue
		}
		if _, exists := result[key]; !exists {
			result[key] = title
		}
	}
	return result
}

func cleanTencentChapterText(raw string) string {
	text := stripHTML(raw)
	text = html.UnescapeString(text)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "[]【】")
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func parseTencentChapterText(text string) (key, title string, ok bool) {
	if m := tencentBonusTitleRe.FindStringSubmatch(text); m != nil {
		title = sanitizeChapterTitle(m[2])
		if title == "" {
			return "", "", false
		}
		return "番外" + normalizeUnitNumber(m[1]), title, true
	}
	for _, pattern := range tencentChapterTitlePatterns {
		m := pattern.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		title = sanitizeChapterTitle(m[2])
		if title == "" {
			return "", "", false
		}
		return "第" + normalizeUnitNumber(m[1]), title, true
	}
	// 只有编号没有标题（如「001」），没有可回填的内容
	return "", "", false
}

func sanitizeChapterTitle(raw string) string {
	title := strings.TrimSpace(html.UnescapeString(raw))
	title = strings.Trim(title, "[]【】-–—_·:： ")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return ""
	}
	// 纯数字是编号，不是标题
	digitsOnly := true
	for _, r := range title {
		if r < '0' || r > '9' {
			digitsOnly = false
			break
		}
	}
	if digitsOnly {
		return ""
	}
	if runes := []rune(title); len(runes) > chapterTitleMaxRunes {
		title = string(runes[:chapterTitleMaxRunes])
	}
	return title
}

func normalizeChapterMatchTitle(title string) string {
	return chapterTitlePunctuation.Replace(strings.ToLower(strings.TrimSpace(title)))
}
