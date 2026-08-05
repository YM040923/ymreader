package workmodel

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Work is a logical comic work detected from one or more physical inventory paths.
type Work struct {
	Title string
	Units []WorkUnit
}

// WorkUnit links a resolved unit to its physical inventory path.
type WorkUnit struct {
	RelativePath string
	Unit         ResolvedUnit
}

// DetectWorks groups existing comic inventory paths into logical works.
func DetectWorks(relativePaths []string) []Work {
	byTitle := make(map[string]*Work)
	order := make([]string, 0)

	for _, relativePath := range relativePaths {
		clean := cleanInventoryPath(relativePath)
		if clean == "" {
			continue
		}

		resolved := ResolvePath(clean, parentWorkTitle(clean))
		title := strings.TrimSpace(resolved.WorkTitle)
		if title == "" {
			continue
		}

		work := byTitle[title]
		if work == nil {
			work = &Work{Title: title}
			byTitle[title] = work
			order = append(order, title)
		}
		work.Units = append(work.Units, WorkUnit{RelativePath: resolved.RelativePath, Unit: resolved.Unit})
	}

	works := make([]Work, 0, len(order))
	for _, title := range order {
		work := *byTitle[title]
		sortWorkUnits(work.Units)
		works = append(works, work)
	}
	sort.SliceStable(works, func(i, j int) bool {
		return strings.Compare(works[i].Title, works[j].Title) < 0
	})
	return works
}

func parentWorkTitle(cleanRelativePath string) string {
	dir := path.Dir(cleanRelativePath)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	return path.Base(dir)
}

func cleanInventoryPath(relativePath string) string {
	clean := path.Clean(filepath.ToSlash(strings.TrimSpace(relativePath)))
	if clean == "." {
		return ""
	}
	return clean
}

func sortWorkUnits(units []WorkUnit) {
	sort.SliceStable(units, func(i, j int) bool {
		a, b := units[i].Unit.SortKey, units[j].Unit.SortKey
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
