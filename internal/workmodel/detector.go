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
		return works[i].RootRelativePath < works[j].RootRelativePath
	})
	return works
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
