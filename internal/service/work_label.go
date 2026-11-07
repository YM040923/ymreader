package service

import (
	"regexp"
	"strings"
)

// ============================================================
// 章节名格式化
//
// 本地文件/压缩包里的章节名来自各种抓取源，常见形态有
//
//	第0001话 我死了？        ← 四位补零，读取时像机器编号
//	第0916话 青空之死        ← 同上
//	久保同学不放过我 - Vol. 001 ← 作品名重复 12 遍
//	<作品名> - Ch. 0001       ← 同上，且英文缩写
//	第0001-0020话             ← 合订区间
//	番外1_凶许主任            ← 下划线当分隔符
//
// 展示层统一整理成「第1话 我死了？」「第1卷」「番外1 凶许主任」这类
// 腾讯动漫/哔哩哔哩漫画的写法。只改展示文本，不参与 Unit ID 计算：
// Unit ID 在 appendWorkUnit 里用原始 label 生成，阅读进度不会因为改名丢失。
// ============================================================

var (
	unitVolumeRe         = regexp.MustCompile(`(?i)^vol(?:ume)?\s*\.?\s*0*(\d+(?:\.\d+)?)\s*(.*)$`)
	unitChapterRe        = regexp.MustCompile(`(?i)^ch(?:apter)?\s*\.?\s*0*(\d+(?:\.\d+)?(?:\s*-\s*0*\d+)?)\s*(.*)$`)
	unitEpisodeRe        = regexp.MustCompile(`(?i)^ep(?:isode)?\s*\.?\s*0*(\d+(?:\.\d+)?)\s*(.*)$`)
	unitChineseRe        = regexp.MustCompile(`^第\s*(0*\d+(?:\.\d+)?(?:\s*-\s*0*\d+)?)\s*(话|話|卷|回|章|册|冊)\s*(.*)$`)
	unitChineseNumeralRe = regexp.MustCompile(`^第\s*([零〇一二三四五六七八九十百千万两]+)\s*(话|話|卷|回|章|册|冊)\s*(.*)$`)
	unitChineseBareRe    = regexp.MustCompile(`^第\s*(0*\d+)\s*[_\s]\s*(\S.*)$`)
	unitBareNumberRe     = regexp.MustCompile(`^0*(\d+)\s+(\S.*)$`)
	unitBonusRe          = regexp.MustCompile(`^番外\s*0*(\d+)?\s*[-_－—:：·]?\s*(.*)$`)
	unitExtraRe          = regexp.MustCompile(`(?i)^extra\s*0*(\d+)?\s*[-_－—:：·]?\s*(.*)$`)
)

// formatUnitLabel 把一条原始章节名整理成展示用文本。
// workTitle 用来剥掉「作品名 - Vol. 001」里重复的作品名。
func formatUnitLabel(label, workTitle string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	cleaned := stripUnitWorkPrefix(label, workTitle)
	// 带分区前缀的标签保留「第一季 / 第001话」这类层级，只整理最后一段
	if parts := strings.Split(cleaned, " / "); len(parts) > 1 {
		parts[len(parts)-1] = formatUnitSegment(parts[len(parts)-1])
		return strings.Join(parts, " / ")
	}
	return formatUnitSegment(cleaned)
}

// stripUnitWorkPrefix 去掉章节名开头重复的作品名。
// 只在作品名后面紧跟分隔符或章节标记时才剥离，避免误伤
// 「大王饶命之新亭是好刀」这种以作品名开头但本身是完整标题的情况。
func stripUnitWorkPrefix(label, workTitle string) string {
	workTitle = strings.TrimSpace(workTitle)
	if workTitle == "" || len(label) <= len(workTitle) {
		return label
	}
	if !strings.EqualFold(label[:len(workTitle)], workTitle) {
		return label
	}
	rest := strings.TrimSpace(label[len(workTitle):])
	if rest == "" {
		return label
	}
	if !startsWithUnitSeparator(rest) && !startsWithUnitMarker(rest) {
		return label
	}
	rest = strings.TrimLeft(rest, " \t\r\n-_－—–/·:：")
	if rest == "" {
		return label
	}
	return rest
}

func startsWithUnitSeparator(value string) bool {
	return strings.ContainsRune("-_－—–/·:：", []rune(value)[0])
}

func startsWithUnitMarker(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, marker := range []string{"第", "番外", "特别篇", "特典", "序章", "终章", "終章", "后记", "後記", "vol", "ch", "ep", "extra", "prologue", "epilogue", "season", "part"} {
		if strings.HasPrefix(lower, marker) {
			return true
		}
	}
	return false
}

// formatUnitSegment 整理单个章节/卷名。
func formatUnitSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" || segment == "全文" {
		return segment
	}
	if m := unitVolumeRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading("第"+normalizeUnitNumber(m[1])+"卷", m[2])
	}
	if m := unitChapterRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading("第"+normalizeUnitNumber(m[1])+"话", m[2])
	}
	if m := unitEpisodeRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading("第"+normalizeUnitNumber(m[1])+"话", m[2])
	}
	if m := unitChineseRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading("第"+normalizeUnitNumber(m[1])+m[2], m[3])
	}
	if m := unitChineseNumeralRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading("第"+m[1]+m[2], m[3])
	}
	if m := unitChineseBareRe.FindStringSubmatch(segment); m != nil && !startsWithSectionWord(m[2]) {
		return "第" + normalizeUnitNumber(m[1]) + "话 " + cleanUnitTitle(m[2])
	}
	if m := unitBonusRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading(withUnitNumber("番外", m[1]), m[2])
	}
	if m := unitExtraRe.FindStringSubmatch(segment); m != nil {
		return joinUnitHeading(withUnitNumber("番外", m[1]), m[2])
	}
	if m := unitBareNumberRe.FindStringSubmatch(segment); m != nil && !startsWithSectionWord(m[2]) {
		return "第" + normalizeUnitNumber(m[1]) + "话 " + cleanUnitTitle(m[2])
	}
	return segment
}

// joinUnitHeading 拼接「第1话」和标题；标题为空时只留编号。
func joinUnitHeading(heading, title string) string {
	title = cleanUnitTitle(title)
	if title == "" {
		return heading
	}
	return heading + " " + title
}

func withUnitNumber(prefix, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return prefix
	}
	return prefix + normalizeUnitNumber(raw)
}

// cleanUnitTitle 清掉标题开头残留的分隔符，并把下划线换成空格。
func cleanUnitTitle(raw string) string {
	title := strings.ReplaceAll(strings.TrimSpace(raw), "_", " ")
	title = strings.TrimLeft(title, " \t\r\n-_－—–·:：")
	return strings.Join(strings.Fields(title), " ")
}

// normalizeUnitNumber 去掉阿拉伯数字编号的前导零，保留小数与区间。
func normalizeUnitNumber(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if index := strings.Index(raw, "-"); index >= 0 {
		return normalizeUnitNumber(raw[:index]) + "-" + normalizeUnitNumber(raw[index+1:])
	}
	if index := strings.Index(raw, "."); index >= 0 {
		integer := strings.TrimLeft(raw[:index], "0")
		if integer == "" {
			integer = "0"
		}
		return integer + "." + raw[index+1:]
	}
	trimmed := strings.TrimLeft(raw, "0")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

// unitSortRank 给阅读单元分组：正篇=0，番外/特别篇=1。
// 番外按字典序总是排在「第…话」前面，排序时需要显式压到最后。
func unitSortRank(label string) int {
	for _, segment := range strings.Split(label, " / ") {
		segment = strings.ToLower(strings.TrimSpace(segment))
		for _, marker := range []string{"番外", "特别篇", "特別篇", "特典", "后记", "後記", "extra", "special", "omake", "bonus"} {
			if strings.HasPrefix(segment, marker) {
				return 1
			}
		}
	}
	return 0
}

// startsWithSectionWord 防止把「2 季」「3 部」这类分区名当成章节编号。
func startsWithSectionWord(value string) bool {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"季", "部", "篇", "卷", "话", "話", "集", "章", "回", "册", "冊"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
