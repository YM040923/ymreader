package workmodel

import (
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	chapterPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)第\s*(\d+(?:\.\d+)?)\s*(?:话|話|回|章)`),
		regexp.MustCompile(`(?i)\b(?:ch|chapter)[\s._-]*(\d+(?:\.\d+)?)\b`),
	}
	volumePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)第\s*(\d+(?:\.\d+)?)\s*(?:卷|冊|册)`),
		regexp.MustCompile(`(?i)\bvol(?:ume)?[\s._-]*(\d+(?:\.\d+)?)\b`),
	}
	extraPattern   = regexp.MustCompile(`(?i)(番外|外传|外傳|extra|omake|bonus)`)
	specialPattern = regexp.MustCompile(`(?i)(特典|公告|请假条|請假條|庆典|慶典|设定集|設定集|special)`)
)

// ResolvePath parses a physical file or folder path into a pure work/unit model.
func ResolvePath(relativePath string, parentTitle string) ResolvedPath {
	clean := path.Clean(filepath.ToSlash(strings.TrimSpace(relativePath)))
	if clean == "." {
		clean = ""
	}
	base := path.Base(clean)
	stem := trimKnownExt(base)

	workTitle := strings.TrimSpace(parentTitle)
	if workTitle == "" {
		workTitle = cleanWorkTitle(stem)
	}

	unit := resolveUnit(stem, workTitle)
	return ResolvedPath{RelativePath: clean, WorkTitle: workTitle, Unit: unit}
}

func resolveUnit(stem, workTitle string) ResolvedUnit {
	labelSource := unitLabelSource(stem, workTitle)

	if specialPattern.MatchString(stem) {
		label := firstNonEmpty(labelSource, specialPattern.FindString(stem), stem)
		return ResolvedUnit{Kind: UnitKindSpecial, Title: stem, DisplayLabel: label, SortKey: SortKey{KindRank: 3, Raw: stem}}
	}
	if extraPattern.MatchString(stem) {
		label := firstNonEmpty(labelSource, extraPattern.FindString(stem), stem)
		return ResolvedUnit{Kind: UnitKindExtra, Title: stem, DisplayLabel: label, SortKey: SortKey{KindRank: 2, Raw: stem}}
	}
	for _, re := range volumePatterns {
		if m := re.FindStringSubmatch(stem); len(m) == 2 {
			n := parseFloat(m[1])
			label := firstNonEmpty(labelSource, strings.TrimSpace(re.FindString(stem)))
			return ResolvedUnit{Kind: UnitKindVolume, Title: stem, DisplayLabel: label, VolumeNumber: &n, SortKey: SortKey{KindRank: 1, Number: n, Raw: stem}}
		}
	}
	for _, re := range chapterPatterns {
		if m := re.FindStringSubmatch(stem); len(m) == 2 {
			n := parseFloat(m[1])
			label := firstNonEmpty(labelSource, strings.TrimSpace(re.FindString(stem)))
			return ResolvedUnit{Kind: UnitKindChapter, Title: stem, DisplayLabel: label, ChapterNumber: &n, SortKey: SortKey{KindRank: 1, Number: n, Raw: stem}}
		}
	}
	return ResolvedUnit{Kind: UnitKindFull, Title: stem, DisplayLabel: "全本", SortKey: SortKey{KindRank: 0, Raw: stem}}
}

func cleanWorkTitle(stem string) string {
	out := stem
	for _, re := range append(volumePatterns, chapterPatterns...) {
		out = re.ReplaceAllString(out, "")
	}
	out = extraPattern.ReplaceAllString(out, "")
	out = specialPattern.ReplaceAllString(out, "")
	out = strings.TrimSpace(trimSeparators(out))
	if out == "" {
		return stem
	}
	return out
}

func unitLabelSource(stem, workTitle string) string {
	label := strings.TrimSpace(stem)
	if workTitle != "" {
		label = strings.TrimSpace(strings.TrimPrefix(label, workTitle))
	}
	return strings.TrimSpace(trimSeparators(label))
}

func trimKnownExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".zip", ".cbz", ".rar", ".cbr", ".7z", ".cb7", ".pdf":
		return strings.TrimSuffix(name, filepath.Ext(name))
	default:
		return name
	}
}

func trimSeparators(s string) string {
	return strings.Trim(s, " \t\r\n-_—–~·.[]【】()（）")
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "全本"
}
