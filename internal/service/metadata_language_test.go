package service

import (
	"strings"
	"testing"
)

func TestPickLangValuePrefersSimplifiedAndTraditionalChineseAliases(t *testing.T) {
	values := map[string]string{
		"en":      "English",
		"zh-hant": "繁體中文",
		"zh-hans": "简体中文",
	}
	if got := pickLangValue(values, "zh-CN"); got != "简体中文" {
		t.Fatalf("pickLangValue(zh-CN) = %q, want simplified Chinese", got)
	}
	if got := pickLangValue(values, "zh-TW"); got != "繁體中文" {
		t.Fatalf("pickLangValue(zh-TW) = %q, want traditional Chinese", got)
	}
}

func TestPickMangaDexLocalizedValueUsesChineseAltTitle(t *testing.T) {
	primary := map[string]string{"en": "Spare Me, Great Lord!"}
	alternatives := []map[string]string{
		{"ja": "大王饒命"},
		{"zh-hans": "大王饶命"},
	}
	if got := pickMangaDexLocalizedValue(primary, alternatives, "zh-CN"); got != "大王饶命" {
		t.Fatalf("localized MangaDex title = %q, want Chinese alt title", got)
	}
}

func TestSortByRelevancePrefersCompleteChineseMetadataForChineseQuery(t *testing.T) {
	results := []ComicMetadata{
		{
			Title:       "Spare Me, Great Lord!",
			Description: "English description",
			Source:      "mangaupdates",
		},
		{
			Title:       "大王饶命",
			Description: "灵气复苏时代的故事。",
			Genre:       "动作, 奇幻",
			Source:      "bangumi",
		},
	}
	sortByRelevance(results, "大王饶命", "zh-CN")
	if results[0].Source != "bangumi" {
		t.Fatalf("top source = %q, want bangumi Chinese result", results[0].Source)
	}
}

func TestAniListQueryHasBalancedBraces(t *testing.T) {
	query := buildAniListQuery("MANGA")
	depth := 0
	for _, char := range query {
		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				t.Fatalf("AniList query closes more braces than it opens: %s", query)
			}
		}
	}
	if depth != 0 {
		t.Fatalf("AniList query has brace depth %d: %s", depth, query)
	}
	if strings.Contains(query, "}}}}") {
		t.Fatalf("AniList query still contains the previous extra closing brace: %s", query)
	}
}
