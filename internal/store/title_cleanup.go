package store

import (
	"regexp"
	"strings"
)

// Filename and title cleanup helpers shared by scanning, search, and metadata inference.

// extractSeriesName 从标题中提取系列名（去掉卷号等）。
// 支持的格式：
//   - [Comic][FL063][佛陀01][手塚治虫][時報][HMM] → "佛陀"
//   - [三眼神童典藏版][手塚治虫][東販]Vol.01 → "三眼神童典藏版"
//   - 佛陀01 → "佛陀"
//   - NARUTO vol.23 → "NARUTO"
//
// trimTrailingVolumeNumber 去掉末尾的卷号标记。
// isLikelyCode 判断是否像编号（如 FL063, HMM, DL版 等）。
// 注意：含有多个CJK字符的字符串不应被判定为编号。
// 【P2修复】放宽对2字符CJK名称的限制，避免 "火影"、"死神"、"棋魂" 被误判。
func isLikelyCode(s string) bool {
	if len(s) <= 2 {
		// 如果是2字符且都是CJK，不算编号（如 "火影"、"死神"、"棋魂"）
		runes := []rune(s)
		if len(runes) == 2 && isCJKRune(runes[0]) && isCJKRune(runes[1]) {
			return false
		}
		return true
	}
	// 如果包含CJK字符且CJK字符数>=2，不太可能是编号
	if cjkRuneCount(s) >= 2 {
		return false
	}
	// 全大写字母+数字且较短 → 编号
	if len(s) <= 8 {
		allUpper := true
		hasLetter := false
		for _, r := range s {
			if r >= 'a' && r <= 'z' {
				allUpper = false
				break
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				hasLetter = true
			}
		}
		if allUpper && hasLetter && isAlphaNumeric(s) {
			return true
		}
	}
	// 以常见出版标签结尾，但仅对纯标签短字符串（如"DL版"）生效
	// 不对长CJK字符串（如"三眼神童典藏版"）误判
	for _, tag := range []string{"版", "出版", "文庫", "文库"} {
		if strings.HasSuffix(s, tag) {
			// 去掉标签后缀，如果剩余部分很短或不含CJK，才是编号
			base := strings.TrimSuffix(s, tag)
			if cjkRuneCount(base) < 2 {
				return true
			}
			return false
		}
	}
	return false
}

// cjkRuneCount 统计字符串中CJK字符的数量。
func cjkRuneCount(s string) int {
	count := 0
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
			(r >= 0x3040 && r <= 0x30FF) || // Hiragana + Katakana
			(r >= 0xAC00 && r <= 0xD7A3) { // Korean
			count++
		}
	}
	return count
}

func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// cleanDirName 清理文件夹名称，提取出可读的组名。
// 处理策略：
//  1. 去除方括号标签（如 [汉化组]、[作者名]），保留核心名称
//  2. 如果文件夹名本身就是简洁的系列名（如"海贼王"），直接返回
//  3. 处理嵌套路径时只取最近一级（path.Base 已在调用处完成）
//
// CleanDirNameForGrouping 清理目录名称，供 store 外部的扫描规则目录整理复用。
func CleanDirNameForGrouping(name string) string {
	return cleanDirName(name)
}

func cleanDirName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// 如果文件夹名不含方括号，直接作为组名（去掉首尾空白即可）
	if !strings.ContainsAny(name, "[]【】「」『』") {
		// 去掉常见的无意义前缀/后缀
		name = strings.TrimSpace(name)
		if name == "" {
			return ""
		}
		return name
	}

	// 包含方括号时，尝试提取核心名称
	// 收集方括号外的部分和方括号内含CJK的部分
	var outsideParts []string
	var bracketParts []string
	inBracket := false
	var current strings.Builder
	for _, r := range name {
		switch r {
		case '[', '【', '「', '『':
			if current.Len() > 0 && !inBracket {
				outsideParts = append(outsideParts, strings.TrimSpace(current.String()))
			}
			current.Reset()
			inBracket = true
		case ']', '】', '」', '』':
			if inBracket && current.Len() > 0 {
				part := strings.TrimSpace(current.String())
				if part != "" && !isLikelyCode(part) {
					bracketParts = append(bracketParts, part)
				}
			}
			current.Reset()
			inBracket = false
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 && !inBracket {
		outsideParts = append(outsideParts, strings.TrimSpace(current.String()))
	}

	// 优先使用方括号外的非空内容（更可能是文件夹的主名称），
	// 但如果方括号外只是"格式标签"（如 PDF、ZIP、漫画 等），则降级使用方括号内的内容。
	for _, p := range outsideParts {
		p = strings.TrimSpace(p)
		if p == "" || len([]rune(p)) < 2 {
			continue
		}
		if isFormatTag(p) || isStatusTag(p) || isScanGroupTag(p) {
			continue
		}
		return p
	}

	// 其次使用方括号内CJK字符最多的部分（排除扫图组、状态标签、格式标签）
	bestPart := ""
	bestCJK := 0
	for _, p := range bracketParts {
		if isFormatTag(p) || isStatusTag(p) || isScanGroupTag(p) {
			continue
		}
		c := cjkRuneCount(p)
		if c > bestCJK {
			bestPart = p
			bestCJK = c
		}
	}
	if bestPart != "" {
		return bestPart
	}

	// 兜底：使用方括号内最长的非噪声部分
	for _, p := range bracketParts {
		if isFormatTag(p) || isStatusTag(p) || isScanGroupTag(p) {
			continue
		}
		if len([]rune(p)) > len([]rune(bestPart)) {
			bestPart = p
		}
	}
	if bestPart != "" {
		return bestPart
	}

	// 最终兜底：返回原始名称
	return name
}

// isFormatTag 判断字符串是否只是"格式/类型"标签（如 PDF、ZIP、漫画、电子书），
// 这类字符串本身没有作品名信息，不适合作为组名主体。
func isFormatTag(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "pdf", "epub", "mobi", "azw3", "azw", "cbz", "cbr", "zip", "rar", "7z", "tar", "txt", "html", "htm",
		"漫画", "漫畫", "小说", "小說", "电子书", "電子書", "书籍", "書籍",
		"comic", "manga", "novel", "book", "books", "ebook", "ebooks":
		return true
	}
	return false
}

// isStatusTag 判断字符串是否是"状态/进度"标签（如 已完结、连载中、完结）。
// 这类字符串本身不含作品名信息，不适合作为组名主体。
func isStatusTag(s string) bool {
	s = strings.TrimSpace(s)
	switch s {
	case "已完结", "已完結", "完结", "完結", "完", "完本", "完整版",
		"连载", "連載", "连载中", "連載中", "未完", "未完结", "未完結",
		"百度", "百度网盘", "百度雲", "度盘", "度盤":
		return true
	}
	lower := strings.ToLower(s)
	switch lower {
	case "complete", "completed", "finished", "end", "ended",
		"ongoing", "serializing", "serialized":
		return true
	}
	return false
}

// scanGroupSuffixes 是常见"扫图组/汉化组"标记后缀的小写形式，
// 出现这些后缀的方括号内容通常是制作组名而非作品名。
var scanGroupSuffixes = []string{
	"汉化组", "漢化組", "汉化版", "漢化版", "汉化", "漢化",
	"扫图组", "掃圖組", "扫图", "掃圖", "扫图版", "掃圖版",
	"嵌字组", "嵌字組", "嵌字",
	"翻译组", "翻譯組", "翻译社", "翻譯社", "翻译", "翻譯",
	"汉译组", "漢譯組",
	"在乎版", // "誰在乎版" 这类自造扫图版署名
	"个人汉化", "個人漢化", "個人翻譯", "个人翻译",
	"重嵌", "重嵌版", "修正版", "重制版", "重製版",
}

// scanGroupKeywords 是包含即视为扫图组的关键词（小写）。
var scanGroupKeywords = []string{
	"scanlation", "scanlations", "scanlator", "scan-team", "scanteam", "scan group",
	"fansub", "fansubs", "sub group", "subteam",
}

// isScanGroupTag 判断方括号内的字符串是否是"扫图组/汉化组/翻译组"标签。
// 例如：[誰在乎版]、[XX汉化组]、[XX scan team]、[百度] 等。
// 注意：作者名、出版社名（如 [黃玉郎]、[東販]）不应被误判，因此只匹配明确的组别后缀/关键词。
func isScanGroupTag(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	for _, kw := range scanGroupKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	for _, suf := range scanGroupSuffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	// "XX组" / "XX社" 但排除 "出版社" 这类常规词、且要求长度合理
	if strings.HasSuffix(s, "组") || strings.HasSuffix(s, "組") {
		runes := []rune(s)
		if len(runes) >= 2 && len(runes) <= 8 {
			return true
		}
	}
	return false
}

// stripScanGroupPrefix 去掉文件名/标题前缀里的扫图组署名。
// 例如："誰在乎版 YongBing-000" → "YongBing-000"
//
//	"[誰在乎版] 海贼王 第1卷" → "海贼王 第1卷"（方括号包裹的扫图组）
//
// 仅去掉"开头"的扫图组前缀，避免误删作品名内的合法字符。
func stripScanGroupPrefix(s string) string {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return orig
	}

	// 形式 1：方括号包裹的前缀，如 "[誰在乎版] xxx" / "【誰在乎版】xxx"
	for _, pair := range [][2]rune{{'[', ']'}, {'【', '】'}, {'「', '」'}, {'『', '』'}} {
		if r := []rune(s); len(r) > 0 && r[0] == pair[0] {
			runes := r
			closeIdx := -1
			for i := 1; i < len(runes); i++ {
				if runes[i] == pair[1] {
					closeIdx = i
					break
				}
			}
			if closeIdx > 1 {
				inside := strings.TrimSpace(string(runes[1:closeIdx]))
				if isScanGroupTag(inside) {
					rest := strings.TrimSpace(string(runes[closeIdx+1:]))
					if rest != "" {
						return rest
					}
				}
			}
		}
	}

	// 形式 2：以扫图组词开头 + 空白 + 其余内容，如 "誰在乎版 YongBing-000"
	// 找首个空白分隔符，前缀如果命中扫图组规则，则剥掉。
	runes := []rune(s)
	for i, r := range runes {
		if r == ' ' || r == '\t' || r == '_' || r == '-' {
			prefix := strings.TrimSpace(string(runes[:i]))
			if prefix != "" && isScanGroupTag(prefix) {
				rest := strings.TrimSpace(string(runes[i+1:]))
				if rest != "" {
					return rest
				}
			}
			// 只检查首个分隔符前缀，再往后无意义
			break
		}
	}
	return orig
}

// StripScanGroupPrefix 是 stripScanGroupPrefix 的导出版本，供其他包复用。
func StripScanGroupPrefix(s string) string { return stripScanGroupPrefix(s) }

// IsScanGroupTag 是 isScanGroupTag 的导出版本，供其他包复用。
func IsScanGroupTag(s string) bool { return isScanGroupTag(s) }

// isNumericTitle 判断标题是否为纯数字命名（如 "1"、"02"、"123"）。
// volumePartPatterns 用于识别"分卷词"——即仅表示分卷/分册位置、本身不含作品名信息的目录或文件名。
// 这类名字单独存在时无法表达系列含义，需要借助上级目录或同级目录补全上下文。
var volumePartPatterns = []*regexp.Regexp{
	// 中文分卷：第一部、第二卷、第3集、第十二话、第二十回、上篇、中篇、下篇、上、中、下、前篇、后篇、外传、番外
	regexp.MustCompile(`^第\s*[一二三四五六七八九十百零〇两\d]+\s*[部卷集册篇話话章回本辑輯季]$`),
	regexp.MustCompile(`^[上中下前后]篇$`),
	regexp.MustCompile(`^[上中下]卷?$`),
	regexp.MustCompile(`^(外传|番外|特别篇|特典|附录|附錄|完结篇|完結篇)$`),
	// 英文分卷：Vol.1、Volume 2、Part 3、Book 4、Chapter 5、Season 6
	regexp.MustCompile(`(?i)^(vol|volume|part|book|chapter|ch|season|s|ep|episode)\s*\.?\s*\d+$`),
	// 纯数字（如目录直接叫 "1"、"02"）
	regexp.MustCompile(`^\d{1,4}$`),
}

// isVolumePartName 判断给定名称是否是单纯的"分卷词"。
// 用于：
//  1. 路径分组时识别最后一级是分卷词，需要与上级拼接组名。
//  2. 智能标题构建时识别文件名只是分卷序号，需要从父目录补全标题。
//  3. 搜索查询构建时识别父目录是分卷词，需要继续向上查找真正的作品名。
func isVolumePartName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, re := range volumePartPatterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// IsVolumePartNameForQuery 是 isVolumePartName 的导出版本，
// 供其他包（如 service 层构建搜索查询时）共享同一份分卷词识别规则。
func IsVolumePartNameForQuery(name string) bool {
	return isVolumePartName(name)
}
