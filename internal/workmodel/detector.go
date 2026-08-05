package workmodel

import (
	"crypto/md5"
	"encoding/hex"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// SourceItem is the physical comic inventory item used as work detection input.
type SourceItem struct {
	ID           string
	LibraryID    string
	RelativePath string
	Title        string
	FileSize     int64
	PageCount    int
}

// DetectedUnit is one readable item inside a detected logical work.
type DetectedUnit struct {
	ID            string
	ComicID       string
	RelativePath  string
	Kind          UnitKind
	Title         string
	DisplayLabel  string
	VolumeNumber  *float64
	ChapterNumber *float64
	SortIndex     int
	PageCount     int
	FileSize      int64
	SortKey       SortKey
}

// DetectedWork is a logical comic work detected from one or more source items.
type DetectedWork struct {
	ID               string
	LibraryID        string
	RootRelativePath string
	Title            string
	SortTitle        string
	Units            []DetectedUnit
}

// DetectWorks groups existing comic inventory items into logical works.
func DetectWorks(items []SourceItem) []DetectedWork {
	groups := make(map[string]*DetectedWork)

	for _, item := range items {
		rel := cleanInventoryPath(item.RelativePath)
		if rel == "" {
			continue
		}

		parent := path.Dir(rel)
		if parent == "." || parent == "/" {
			parent = ""
		}
		parentTitle := ""
		if parent != "" {
			parentTitle = path.Base(parent)
		}

		resolved := ResolvePath(rel, parentTitle)
		root := parent
		if categoryRoot, categoryTitle, ok := categoryWorkRoot(parent, rel, resolved.Unit.Kind); ok {
			root = categoryRoot
			resolved.WorkTitle = categoryTitle
			if resolved.Unit.Kind != UnitKindFull {
				applyNestedUnitContext(&resolved.Unit, rel, root)
			}
		} else if top, ok := topLevelParent(parent); ok && resolved.Unit.Kind != UnitKindFull {
			root = top
			resolved.WorkTitle = top
			applyNestedUnitContext(&resolved.Unit, rel, root)
		}
		if root == "" && resolved.Unit.Kind != UnitKindFull {
			root = resolved.WorkTitle
		}
		if root == "" {
			root = trimKnownExt(path.Base(rel))
		}

		key := item.LibraryID + "\x00" + root
		work := groups[key]
		if work == nil {
			work = &DetectedWork{
				ID:               stableID("work", item.LibraryID, root),
				LibraryID:        item.LibraryID,
				RootRelativePath: root,
				Title:            resolved.WorkTitle,
				SortTitle:        strings.ToLower(resolved.WorkTitle),
			}
			groups[key] = work
		}

		work.Units = append(work.Units, DetectedUnit{
			ID:            stableID("unit", item.LibraryID, rel),
			ComicID:       item.ID,
			RelativePath:  rel,
			Kind:          resolved.Unit.Kind,
			Title:         resolved.Unit.Title,
			DisplayLabel:  resolved.Unit.DisplayLabel,
			VolumeNumber:  resolved.Unit.VolumeNumber,
			ChapterNumber: resolved.Unit.ChapterNumber,
			SortIndex:     0,
			PageCount:     item.PageCount,
			FileSize:      item.FileSize,
			SortKey:       resolved.Unit.SortKey,
		})
	}

	works := make([]DetectedWork, 0, len(groups))
	for _, work := range groups {
		sortDetectedUnits(work.Units)
		for i := range work.Units {
			work.Units[i].SortIndex = i
		}
		works = append(works, *work)
	}
	sort.SliceStable(works, func(i, j int) bool {
		if works[i].SortTitle != works[j].SortTitle {
			return works[i].SortTitle < works[j].SortTitle
		}
		if works[i].RootRelativePath != works[j].RootRelativePath {
			return works[i].RootRelativePath < works[j].RootRelativePath
		}
		if works[i].LibraryID != works[j].LibraryID {
			return works[i].LibraryID < works[j].LibraryID
		}
		return works[i].ID < works[j].ID
	})
	return works
}

func categoryWorkRoot(parent, rel string, kind UnitKind) (root, title string, ok bool) {
	segments := splitPathSegments(parent)
	if len(segments) == 0 || !isCategorySegment(segments[0]) {
		return "", "", false
	}
	if len(segments) >= 2 {
		return path.Join(segments[0], segments[1]), segments[1], true
	}
	if kind == UnitKindFull {
		title := trimKnownExt(path.Base(rel))
		if title != "" {
			return path.Join(segments[0], title), title, true
		}
	}
	return "", "", false
}

func splitPathSegments(value string) []string {
	if value == "" {
		return nil
	}
	raw := strings.Split(value, "/")
	segments := make([]string, 0, len(raw))
	for _, segment := range raw {
		segment = strings.TrimSpace(segment)
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	return segments
}

func isCategorySegment(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	switch normalized {
	case "国漫", "國漫", "大陆漫画", "大陸漫畫", "国产漫画", "國產漫畫", "国产", "國產",
		"日漫", "日本漫画", "日本漫畫",
		"韩漫", "韓漫", "韩国漫画", "韓國漫畫",
		"美漫", "欧美漫画", "歐美漫畫",
		"guoman", "cncomic", "cncomics", "manhua",
		"riman", "jpcomic", "jpcomics", "manga",
		"hanman", "krcomic", "krcomics", "manhwa",
		"uscomic", "uscomics", "comic", "comics":
		return true
	default:
		return false
	}
}

func topLevelParent(parent string) (string, bool) {
	if parent == "" || !strings.Contains(parent, "/") {
		return "", false
	}
	return strings.Split(parent, "/")[0], true
}

func applyNestedUnitContext(unit *ResolvedUnit, rel, root string) {
	nested := strings.TrimPrefix(rel, root+"/")
	nestedTitle := trimKnownExt(nested)
	if nestedTitle == "" {
		return
	}
	unit.Title = nestedTitle
	unit.DisplayLabel = nestedTitle
	unit.SortKey.Raw = nestedTitle

	volumeFolder := strings.Split(nested, "/")[0]
	volumeUnit := resolveUnit(trimKnownExt(volumeFolder), "")
	if volumeUnit.Kind != UnitKindVolume || volumeUnit.VolumeNumber == nil {
		return
	}
	unit.VolumeNumber = volumeUnit.VolumeNumber
	if unit.ChapterNumber != nil {
		unit.SortKey.Number = (*volumeUnit.VolumeNumber * 1000000) + *unit.ChapterNumber
	}
}

func cleanInventoryPath(relativePath string) string {
	clean := path.Clean(filepath.ToSlash(strings.TrimSpace(relativePath)))
	if clean == "." {
		return ""
	}
	return clean
}

func sortDetectedUnits(units []DetectedUnit) {
	sort.SliceStable(units, func(i, j int) bool {
		a, b := units[i].SortKey, units[j].SortKey
		if a.KindRank != b.KindRank {
			return a.KindRank < b.KindRank
		}
		if a.Number != b.Number {
			return a.Number < b.Number
		}
		if a.Raw != b.Raw {
			return strings.Compare(a.Raw, b.Raw) < 0
		}
		return strings.Compare(units[i].RelativePath, units[j].RelativePath) < 0
	})
}

func stableID(prefix, libraryID, value string) string {
	sum := md5.Sum([]byte(libraryID + "\x00" + strings.ReplaceAll(value, "\\", "/")))
	return prefix + "_" + hex.EncodeToString(sum[:])
}
