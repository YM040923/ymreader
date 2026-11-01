package service

import (
	"sort"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

const manualWorkIDPrefix = store.ManualWorkIDPrefix

func isManualWorkID(workID string) bool {
	return strings.HasPrefix(workID, manualWorkIDPrefix)
}

// ApplyManualWorkMemberships moves physical units between Works according to
// user-defined membership. It runs after automatic directory grouping and
// before metadata overlays, so merged/split Works keep stable IDs and metadata.
func ApplyManualWorkMemberships(
	works []Work,
	memberships map[string]string,
	manuals map[string]store.LogicalWork,
	comics map[string]store.ComicListItem,
) []Work {
	if len(memberships) == 0 {
		return works
	}

	byID := make(map[string]*Work, len(works)+len(manuals))
	order := make([]string, 0, len(works)+len(manuals))
	for index := range works {
		work := &works[index]
		byID[work.ID] = work
		order = append(order, work.ID)
	}
	for workID, row := range manuals {
		if _, exists := byID[workID]; exists {
			continue
		}
		byID[workID] = &Work{
			ID:               workID,
			LibraryID:        row.LibraryID,
			Title:            row.Title,
			RootPath:         store.DisplayWorkRootPath(workID, row.RootPath),
			CoverComicID:     row.CoverComicID,
			CoverAspectRatio: row.CoverAspectRatio,
			Tags:             []store.ComicTagInfo{},
			Categories:       []store.ComicCategoryInfo{},
			Units:            []WorkUnit{},
		}
		order = append(order, workID)
	}

	groupedUnits := make(map[string][]WorkUnit, len(byID))
	for index := range works {
		work := &works[index]
		for _, unit := range work.Units {
			targetID := memberships[unit.ComicID]
			if targetID == "" {
				targetID = work.ID
			}
			if _, exists := byID[targetID]; !exists {
				targetID = work.ID
			}
			groupedUnits[targetID] = append(groupedUnits[targetID], unit)
		}
	}

	result := make([]Work, 0, len(order))
	for _, workID := range order {
		work := byID[workID]
		units := groupedUnits[workID]
		if len(units) == 0 {
			work.Units = []WorkUnit{}
			work.ItemCount = 0
			continue
		}
		sort.SliceStable(units, func(i, j int) bool {
			if units[i].DisplayLabel != units[j].DisplayLabel {
				return NaturalLess(units[i].DisplayLabel, units[j].DisplayLabel)
			}
			if units[i].StartPage != units[j].StartPage {
				return units[i].StartPage < units[j].StartPage
			}
			return units[i].ID < units[j].ID
		})

		comicIDs := make(map[string]struct{}, len(units))
		totalPages := 0
		for index := range units {
			units[index].WorkID = workID
			units[index].SortIndex = index
			comicIDs[units[index].ComicID] = struct{}{}
			totalPages += units[index].PageCount
		}
		work.Units = units
		work.ItemCount = len(units)
		work.PageCount = totalPages

		sources := make([]store.ComicListItem, 0, len(comicIDs))
		for comicID := range comicIDs {
			if comic, ok := comics[comicID]; ok {
				sources = append(sources, comic)
			}
		}
		sort.SliceStable(sources, func(i, j int) bool {
			left := normalizeWorkPath(firstNonEmpty(sources[i].RelativePath, sources[i].Filename))
			right := normalizeWorkPath(firstNonEmpty(sources[j].RelativePath, sources[j].Filename))
			if left != right {
				return naturalLess(left, right)
			}
			return sources[i].ID < sources[j].ID
		})
		if len(sources) == len(comicIDs) {
			var totalSize int64
			var earliestAdded string
			work.TotalReadTime = 0
			work.IsFavorite = false
			work.Rating = nil
			work.Tags = []store.ComicTagInfo{}
			work.Categories = []store.ComicCategoryInfo{}
			for _, source := range sources {
				totalSize += source.FileSize
				work.TotalReadTime += source.TotalReadTime
				mergeWorkUserState(work, source)
				work.Tags = mergeTags(work.Tags, source.Tags)
				work.Categories = mergeCategories(work.Categories, source.Categories)
				if earliestAdded == "" || source.AddedAt < earliestAdded {
					earliestAdded = source.AddedAt
				}
			}
			work.FileSize = totalSize
			work.ReadingStatus = aggregateWorkReadingStatus(sources)
			if work.AddedAt == "" {
				work.AddedAt = earliestAdded
			}
			sort.SliceStable(work.Tags, func(i, j int) bool { return naturalLess(work.Tags[i].Name, work.Tags[j].Name) })
			sort.SliceStable(work.Categories, func(i, j int) bool {
				return naturalLess(work.Categories[i].Name, work.Categories[j].Name)
			})
			work.LastReadAt = nil
			work.ContinueComicID = ""
			work.ContinueUnitID = ""
			work.ContinuePage = 0
			projectReadingProgress(work, comics)
			projectCompletedUnits(work, comics)
		}
		if work.RepresentativeComicID == "" || !containsComicID(comicIDs, work.RepresentativeComicID) {
			work.RepresentativeComicID = units[0].ComicID
		}
		if work.CoverComicID == "" || !containsComicID(comicIDs, work.CoverComicID) {
			if work.StoredCoverURL == "" {
				work.CoverComicID = units[0].ComicID
			}
		}
		result = append(result, *work)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return NaturalLess(result[i].Title, result[j].Title)
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func containsComicID(set map[string]struct{}, comicID string) bool {
	_, ok := set[comicID]
	return ok
}
