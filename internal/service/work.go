package service

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// Work is the logical manga/book shown to clients. Physical Comic rows are
// implementation details and become one or more Units under a Work.
type Work struct {
	ID                    string                    `json:"id"`
	LibraryID             string                    `json:"libraryId"`
	Title                 string                    `json:"title"`
	RootPath              string                    `json:"rootPath"`
	SeriesID              string                    `json:"seriesId,omitempty"`
	MetadataHostType      string                    `json:"metadataHostType"`
	MetadataHostID        string                    `json:"metadataHostId"`
	RepresentativeComicID string                    `json:"representativeComicId"`
	CoverComicID          string                    `json:"coverComicId"`
	CoverURL              string                    `json:"coverUrl,omitempty"`
	CoverAspectRatio      float64                   `json:"coverAspectRatio"`
	ItemCount             int                       `json:"itemCount"`
	CompletedItemCount    int                       `json:"completedItemCount"`
	PageCount             int                       `json:"pageCount"`
	FileSize              int64                     `json:"fileSize"`
	TotalReadTime         int                       `json:"totalReadTime"`
	AddedAt               string                    `json:"addedAt"`
	UpdatedAt             string                    `json:"updatedAt"`
	SortOrder             int                       `json:"sortOrder"`
	Author                string                    `json:"author"`
	Publisher             string                    `json:"publisher"`
	Year                  *int                      `json:"year"`
	Description           string                    `json:"description"`
	Language              string                    `json:"language"`
	Genre                 string                    `json:"genre"`
	Status                string                    `json:"status"`
	MetadataSource        string                    `json:"metadataSource"`
	ExternalRating        *float64                  `json:"externalRating,omitempty"`
	ExternalRatingMax     *float64                  `json:"externalRatingMax,omitempty"`
	ExternalRatingSource  string                    `json:"externalRatingSource,omitempty"`
	Tags                  []store.ComicTagInfo      `json:"tags"`
	Categories            []store.ComicCategoryInfo `json:"categories"`
	IsFavorite            bool                      `json:"isFavorite"`
	Rating                *int                      `json:"rating"`
	ReadingStatus         string                    `json:"readingStatus"`
	LastReadAt            *string                   `json:"lastReadAt,omitempty"`
	ContinueComicID       string                    `json:"continueComicId,omitempty"`
	ContinuePage          int                       `json:"continuePage"`
	ContinueUnitID        string                    `json:"continueUnitId,omitempty"`
	Units                 []WorkUnit                `json:"units,omitempty"`
}

// WorkUnit is a readable volume/chapter/section. StartPage is the absolute
// page offset in the physical Comic. LastReadPage is relative to this Unit.
type WorkUnit struct {
	ID               string  `json:"id"`
	WorkID           string  `json:"workId"`
	ComicID          string  `json:"comicId"`
	Title            string  `json:"title"`
	DisplayLabel     string  `json:"displayLabel"`
	RelativePath     string  `json:"relativePath,omitempty"`
	InternalPath     string  `json:"internalPath,omitempty"`
	StartPage        int     `json:"startPage"`
	PageCount        int     `json:"pageCount"`
	CoverPage        int     `json:"coverPage"`
	SortIndex        int     `json:"sortIndex"`
	CoverURL         string  `json:"coverUrl,omitempty"`
	CoverAspectRatio float64 `json:"coverAspectRatio"`
	LastReadPage     int     `json:"lastReadPage"`
	LastReadAt       *string `json:"lastReadAt,omitempty"`
	ReadingStatus    string  `json:"readingStatus,omitempty"`
}

type WorkBuildOptions struct {
	ProbeInternalArchive bool
	ResolveComicPath     func(store.ComicListItem) (string, bool)
}

type workAccumulator struct {
	work       *Work
	sources    map[string]store.ComicListItem
	standalone map[string]bool
}

// BuildWorksFromComicList converts every supported physical layout into the
// same Work -> Unit -> Page model.
func BuildWorksFromComicList(items []store.ComicListItem, opts WorkBuildOptions) []Work {
	sortedItems := append([]store.ComicListItem(nil), items...)
	sort.SliceStable(sortedItems, func(i, j int) bool {
		if sortedItems[i].LibraryID != sortedItems[j].LibraryID {
			return sortedItems[i].LibraryID < sortedItems[j].LibraryID
		}
		left := normalizeWorkPath(firstNonEmpty(sortedItems[i].RelativePath, sortedItems[i].Filename))
		right := normalizeWorkPath(firstNonEmpty(sortedItems[j].RelativePath, sortedItems[j].Filename))
		if left != right {
			return naturalLess(left, right)
		}
		return sortedItems[i].ID < sortedItems[j].ID
	})

	byKey := make(map[string]*workAccumulator)
	keys := make([]string, 0)
	for _, item := range sortedItems {
		rel := normalizeWorkPath(firstNonEmpty(item.RelativePath, item.Filename))
		if rel == "" {
			continue
		}
		root, itemLabel, standaloneFile := splitWorkRoot(rel, item.Title)
		if root == "" {
			continue
		}
		key := item.LibraryID + "\x00" + root
		acc := byKey[key]
		if acc == nil {
			workID := stableWorkID(item.LibraryID, root)
			acc = &workAccumulator{
				work: &Work{
					ID:         workID,
					LibraryID:  item.LibraryID,
					Title:      workTitle(root, item.Title),
					RootPath:   root,
					Tags:       []store.ComicTagInfo{},
					Categories: []store.ComicCategoryInfo{},
					Units:      []WorkUnit{},
				},
				sources:    make(map[string]store.ComicListItem),
				standalone: make(map[string]bool),
			}
			byKey[key] = acc
			keys = append(keys, key)
		}
		acc.sources[item.ID] = item
		acc.standalone[item.ID] = standaloneFile
		acc.work.FileSize += item.FileSize
		acc.work.PageCount += item.PageCount

		internalLayout := archiveLayout{}
		if opts.ProbeInternalArchive && standaloneFile && isZipLike(rel) {
			resolver := opts.ResolveComicPath
			if resolver == nil {
				resolver = func(candidate store.ComicListItem) (string, bool) {
					return resolveWorkComicPath(candidate, rel)
				}
			}
			if filePath, ok := resolver(item); ok {
				internalLayout = probeArchiveLayout(filePath)
			}
		}
		if len(internalLayout.Units) > 0 {
			for _, chapter := range internalLayout.Units {
				appendWorkUnit(acc.work, item, chapter.Title, chapter.DisplayLabel, chapter.Path, chapter.StartPage, chapter.PageCount, chapter.CoverPage)
			}
			if item.PageCount <= 0 {
				acc.work.PageCount += internalLayout.TotalPages
			}
			continue
		}

		label := strings.TrimSpace(itemLabel)
		if label == "" || label == "." || standaloneFile {
			label = "全文"
		}
		appendWorkUnit(acc.work, item, label, label, "", 0, item.PageCount, 0)
	}

	result := make([]Work, 0, len(keys))
	for _, key := range keys {
		acc := byKey[key]
		finalizeWork(acc)
		result = append(result, *acc.work)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Title != result[j].Title {
			return naturalLess(result[i].Title, result[j].Title)
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func finalizeWork(acc *workAccumulator) {
	work := acc.work
	sort.SliceStable(work.Units, func(i, j int) bool {
		if work.Units[i].DisplayLabel != work.Units[j].DisplayLabel {
			return naturalLess(work.Units[i].DisplayLabel, work.Units[j].DisplayLabel)
		}
		if work.Units[i].StartPage != work.Units[j].StartPage {
			return work.Units[i].StartPage < work.Units[j].StartPage
		}
		return work.Units[i].ID < work.Units[j].ID
	})
	for i := range work.Units {
		work.Units[i].SortIndex = i
	}
	work.ItemCount = len(work.Units)

	sources := make([]store.ComicListItem, 0, len(acc.sources))
	for _, item := range acc.sources {
		sources = append(sources, item)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		left := normalizeWorkPath(firstNonEmpty(sources[i].RelativePath, sources[i].Filename))
		right := normalizeWorkPath(firstNonEmpty(sources[j].RelativePath, sources[j].Filename))
		if left != right {
			return naturalLess(left, right)
		}
		return sources[i].ID < sources[j].ID
	})

	representative := chooseRepresentativeSource(sources)
	if representative != nil {
		work.RepresentativeComicID = representative.ID
		applyRepresentativeMetadata(work, *representative)
	}
	if len(sources) > 1 {
		work.SeriesID = stableSeriesID("series", work.LibraryID, work.RootPath)
		work.MetadataHostType = "series"
		work.MetadataHostID = work.SeriesID
	} else if representative != nil {
		work.MetadataHostType = "comic"
		work.MetadataHostID = representative.ID
		if acc.standalone[representative.ID] && strings.TrimSpace(representative.Title) != "" {
			work.Title = strings.TrimSpace(representative.Title)
		}
	}
	aggregateWorkOrdering(work, sources)
	for _, item := range sources {
		mergeMissingMetadata(work, item)
		mergeWorkUserState(work, item)
		work.TotalReadTime += item.TotalReadTime
		work.Tags = mergeTags(work.Tags, item.Tags)
		work.Categories = mergeCategories(work.Categories, item.Categories)
	}
	sort.SliceStable(work.Tags, func(i, j int) bool { return naturalLess(work.Tags[i].Name, work.Tags[j].Name) })
	sort.SliceStable(work.Categories, func(i, j int) bool {
		return naturalLess(work.Categories[i].Name, work.Categories[j].Name)
	})

	coverSource := chooseCoverSource(work.Units, acc.sources, sources)
	if coverSource != nil {
		work.CoverComicID = coverSource.ID
		work.CoverURL = firstNonEmpty(coverSource.CoverURL, store.BuildComicCoverURL(coverSource.ID))
		work.CoverAspectRatio = coverSource.CoverAspectRatio
	}
	if work.CoverAspectRatio <= 0 && representative != nil {
		work.CoverAspectRatio = representative.CoverAspectRatio
	}
	work.ReadingStatus = aggregateWorkReadingStatus(sources)
	projectReadingProgress(work, acc.sources)
	projectCompletedUnits(work, acc.sources)
}

// ApplySeriesMetadata overlays the persistent ComicSeries metadata for
// directory/multi-file Works. Standalone ZIP/CBZ/PDF Works keep their physical
// Comic as the metadata host.
func ApplySeriesMetadata(works []Work, summaries []store.SeriesSummary) {
	seriesByKey := make(map[string]store.SeriesSummary, len(summaries))
	for _, summary := range summaries {
		key := summary.LibraryID + "\x00" + normalizeWorkPath(summary.RootRelativePath)
		seriesByKey[key] = summary
	}
	for index := range works {
		work := &works[index]
		summary, ok := seriesByKey[work.LibraryID+"\x00"+normalizeWorkPath(work.RootPath)]
		if !ok {
			continue
		}
		work.SeriesID = summary.ID
		work.MetadataHostType = "series"
		work.MetadataHostID = summary.ID
		if strings.TrimSpace(summary.Title) != "" {
			work.Title = summary.Title
		}
		if summary.CoverComicID != "" {
			work.CoverComicID = summary.CoverComicID
		}
		if summary.CoverURL != "" {
			work.CoverURL = summary.CoverURL
			if summary.CoverAspectRatio > 0 {
				work.CoverAspectRatio = summary.CoverAspectRatio
			}
		} else if summary.CoverComicID != "" {
			work.CoverURL = store.BuildComicCoverURL(summary.CoverComicID)
			if summary.CoverAspectRatio > 0 {
				work.CoverAspectRatio = summary.CoverAspectRatio
			}
		}
		if summary.Author != "" {
			work.Author = summary.Author
		}
		if summary.Publisher != "" {
			work.Publisher = summary.Publisher
		}
		if summary.Year != nil {
			work.Year = cloneInt(summary.Year)
		}
		if summary.Description != "" {
			work.Description = summary.Description
		}
		if summary.Language != "" {
			work.Language = summary.Language
		}
		if summary.Genre != "" {
			work.Genre = summary.Genre
		}
		if summary.Status != "" {
			work.Status = summary.Status
		}
		if summary.MetadataSource != "" {
			work.MetadataSource = summary.MetadataSource
		}
		if summary.ExternalRating != nil {
			work.ExternalRating = cloneFloat64(summary.ExternalRating)
			work.ExternalRatingMax = cloneFloat64(summary.ExternalRatingMax)
			work.ExternalRatingSource = summary.ExternalRatingSource
		}
		if summary.UpdatedAt > work.UpdatedAt {
			work.UpdatedAt = summary.UpdatedAt
		}
		work.IsFavorite = work.IsFavorite || summary.IsFavorite
		if readAtAfter(summary.LastReadAt, work.LastReadAt) {
			work.LastReadAt = cloneString(summary.LastReadAt)
		}
		seriesTags := make([]store.ComicTagInfo, 0, len(summary.Tags))
		for _, tag := range summary.Tags {
			seriesTags = append(seriesTags, store.ComicTagInfo{Name: tag.Name, Color: tag.Color})
		}
		work.Tags = mergeTags(work.Tags, seriesTags)
		sort.SliceStable(work.Tags, func(i, j int) bool { return naturalLess(work.Tags[i].Name, work.Tags[j].Name) })
	}
}

// NaturalLess exposes the Work/Unit natural ordering to API adapters so every
// representation sorts Arabic and Chinese numerals identically.
func NaturalLess(left, right string) bool {
	return naturalLess(left, right)
}

func PhysicalComicIDs(work Work) []string {
	seen := make(map[string]struct{}, len(work.Units))
	result := make([]string, 0, len(work.Units))
	for _, unit := range work.Units {
		if unit.ComicID == "" {
			continue
		}
		if _, ok := seen[unit.ComicID]; ok {
			continue
		}
		seen[unit.ComicID] = struct{}{}
		result = append(result, unit.ComicID)
	}
	return result
}

func ApplyComicWorkStatuses(works []Work, statuses map[string]string) {
	for index := range works {
		if works[index].MetadataHostType != "comic" {
			continue
		}
		if status := strings.TrimSpace(statuses[works[index].MetadataHostID]); status != "" {
			works[index].Status = status
		}
	}
}

func ApplyWorkSortOrders(works []Work, orders map[string]int) {
	for index := range works {
		if order, ok := orders[works[index].ID]; ok {
			works[index].SortOrder = order
		}
	}
}

func aggregateWorkReadingStatus(sources []store.ComicListItem) string {
	if len(sources) == 0 {
		return "unread"
	}
	allFinished := true
	anyStarted := false
	for _, source := range sources {
		finished := strings.EqualFold(source.ReadingStatus, "finished") ||
			(source.PageCount > 0 && source.LastReadAt != nil && source.LastReadPage >= source.PageCount-1)
		started := finished || strings.EqualFold(source.ReadingStatus, "reading") ||
			source.LastReadAt != nil || source.LastReadPage > 0
		allFinished = allFinished && finished
		anyStarted = anyStarted || started
	}
	if allFinished {
		return "finished"
	}
	if anyStarted {
		return "reading"
	}
	return "unread"
}

func appendWorkUnit(work *Work, item store.ComicListItem, title, displayLabel, internalPath string, startPage, pageCount, coverPage int) {
	title = strings.TrimSpace(title)
	displayLabel = strings.TrimSpace(displayLabel)
	if displayLabel == "" {
		displayLabel = title
	}
	label := displayLabel
	label = strings.TrimSpace(label)
	if label == "" {
		label = "全文"
	}
	unitTitle := strings.TrimSpace(item.Title)
	if internalPath != "" || unitTitle == "" || strings.EqualFold(unitTitle, work.Title) {
		unitTitle = firstNonEmpty(title, label)
	}
	coverURL := firstNonEmpty(item.CoverURL, store.BuildComicCoverURL(item.ID))
	if internalPath != "" {
		if coverPage < startPage {
			coverPage = startPage
		}
		coverURL = config.JoinBasePath(fmt.Sprintf("/api/comics/%s/page/%d", item.ID, coverPage))
	}
	work.Units = append(work.Units, WorkUnit{
		ID:               stableUnitID(work.ID, item.ID, internalPath, label),
		WorkID:           work.ID,
		ComicID:          item.ID,
		Title:            unitTitle,
		DisplayLabel:     label,
		RelativePath:     item.RelativePath,
		InternalPath:     internalPath,
		StartPage:        startPage,
		PageCount:        pageCount,
		CoverPage:        coverPage,
		CoverURL:         coverURL,
		CoverAspectRatio: item.CoverAspectRatio,
	})
}

func projectCompletedUnits(work *Work, sourceByID map[string]store.ComicListItem) {
	unitCountByComic := make(map[string]int, len(sourceByID))
	for _, unit := range work.Units {
		unitCountByComic[unit.ComicID]++
	}
	completed := 0
	for index := range work.Units {
		unit := &work.Units[index]
		source, ok := sourceByID[unit.ComicID]
		if !ok {
			continue
		}
		finished := strings.EqualFold(source.ReadingStatus, "finished")
		if !finished && source.LastReadAt != nil && unit.PageCount > 0 {
			finished = source.LastReadPage >= unit.StartPage+unit.PageCount-1
		}
		if finished {
			unit.ReadingStatus = "finished"
			completed++
		} else if unit.LastReadAt != nil ||
			(unitCountByComic[unit.ComicID] == 1 && strings.EqualFold(source.ReadingStatus, "reading")) {
			unit.ReadingStatus = "reading"
		} else if unitCountByComic[unit.ComicID] == 1 {
			unit.ReadingStatus = source.ReadingStatus
		}
	}
	work.CompletedItemCount = completed
}

func chooseRepresentativeSource(items []store.ComicListItem) *store.ComicListItem {
	if len(items) == 0 {
		return nil
	}
	bestIndex, bestScore := 0, metadataScore(items[0])
	for i := 1; i < len(items); i++ {
		if score := metadataScore(items[i]); score > bestScore {
			bestIndex, bestScore = i, score
		}
	}
	return &items[bestIndex]
}

func metadataScore(item store.ComicListItem) int {
	score := 0
	for _, value := range []string{
		item.Author, item.Publisher, item.Description, item.Language,
		item.Genre, item.MetadataSource, item.CoverImageURL,
	} {
		if strings.TrimSpace(value) != "" {
			score++
		}
	}
	if item.Year != nil {
		score++
	}
	if len(item.Tags) > 0 {
		score++
	}
	if len(item.Categories) > 0 {
		score++
	}
	return score
}

func applyRepresentativeMetadata(work *Work, item store.ComicListItem) {
	work.Author = item.Author
	work.Publisher = item.Publisher
	work.Year = cloneInt(item.Year)
	work.Description = item.Description
	work.Language = item.Language
	work.Genre = item.Genre
	work.MetadataSource = item.MetadataSource
	work.ExternalRating = cloneFloat64(item.ExternalRating)
	if item.ExternalRatingMax > 0 {
		maximum := item.ExternalRatingMax
		work.ExternalRatingMax = &maximum
	}
	work.ExternalRatingSource = item.ExternalRatingSource
}

func aggregateWorkOrdering(work *Work, sources []store.ComicListItem) {
	if len(sources) == 0 {
		return
	}
	work.SortOrder = sources[0].SortOrder
	for _, source := range sources {
		if source.AddedAt != "" && (work.AddedAt == "" || source.AddedAt < work.AddedAt) {
			work.AddedAt = source.AddedAt
		}
		if source.UpdatedAt != "" && source.UpdatedAt > work.UpdatedAt {
			work.UpdatedAt = source.UpdatedAt
		}
	}
}

func mergeMissingMetadata(work *Work, item store.ComicListItem) {
	if work.Author == "" {
		work.Author = item.Author
	}
	if work.Publisher == "" {
		work.Publisher = item.Publisher
	}
	if work.Year == nil {
		work.Year = cloneInt(item.Year)
	}
	if work.Description == "" {
		work.Description = item.Description
	}
	if work.Language == "" {
		work.Language = item.Language
	}
	if work.Genre == "" {
		work.Genre = item.Genre
	}
	if work.MetadataSource == "" {
		work.MetadataSource = item.MetadataSource
	}
}

func mergeWorkUserState(work *Work, item store.ComicListItem) {
	work.IsFavorite = work.IsFavorite || item.IsFavorite
	if work.Rating == nil && item.Rating != nil {
		work.Rating = cloneInt(item.Rating)
	}
	if readingStatusRank(item.ReadingStatus) > readingStatusRank(work.ReadingStatus) {
		work.ReadingStatus = item.ReadingStatus
	}
}

func readingStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "reading":
		return 4
	case "want":
		return 3
	case "finished":
		return 2
	case "shelved":
		return 1
	default:
		return 0
	}
}

func chooseCoverSource(units []WorkUnit, sourceByID map[string]store.ComicListItem, sources []store.ComicListItem) *store.ComicListItem {
	for _, source := range sources {
		if strings.TrimSpace(source.CoverImageURL) != "" {
			value := source
			return &value
		}
	}
	for _, unit := range units {
		if source, ok := sourceByID[unit.ComicID]; ok {
			value := source
			return &value
		}
	}
	if len(sources) > 0 {
		return &sources[0]
	}
	return nil
}

func projectReadingProgress(work *Work, sourceByID map[string]store.ComicListItem) {
	unitsByComic := make(map[string][]int)
	for index := range work.Units {
		unitsByComic[work.Units[index].ComicID] = append(unitsByComic[work.Units[index].ComicID], index)
	}

	var latest *store.ComicListItem
	comicIDs := make([]string, 0, len(unitsByComic))
	for comicID := range unitsByComic {
		comicIDs = append(comicIDs, comicID)
	}
	sort.Strings(comicIDs)
	for _, comicID := range comicIDs {
		indexes := unitsByComic[comicID]
		item, ok := sourceByID[comicID]
		if !ok || item.LastReadAt == nil {
			continue
		}
		target := locateProgressUnit(work.Units, indexes, item.LastReadPage)
		unit := &work.Units[target]
		unit.LastReadAt = cloneString(item.LastReadAt)
		unit.LastReadPage = item.LastReadPage
		if unit.InternalPath != "" {
			unit.LastReadPage = item.LastReadPage - unit.StartPage
			if unit.LastReadPage < 0 {
				unit.LastReadPage = 0
			}
			if unit.PageCount > 0 && unit.LastReadPage >= unit.PageCount {
				unit.LastReadPage = unit.PageCount - 1
			}
		}
		if latest == nil || readAtAfter(item.LastReadAt, latest.LastReadAt) {
			value := item
			latest = &value
			work.ContinueUnitID = unit.ID
		}
	}
	if latest == nil {
		return
	}
	work.LastReadAt = cloneString(latest.LastReadAt)
	work.ContinueComicID = latest.ID
	// ContinuePage remains absolute because the reader opens the physical Comic.
	work.ContinuePage = latest.LastReadPage
}

func locateProgressUnit(units []WorkUnit, indexes []int, absolutePage int) int {
	if len(indexes) == 0 {
		return 0
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		return units[indexes[i]].StartPage < units[indexes[j]].StartPage
	})
	target := indexes[0]
	for _, index := range indexes {
		unit := units[index]
		if absolutePage < unit.StartPage {
			break
		}
		target = index
		if unit.PageCount <= 0 || absolutePage < unit.StartPage+unit.PageCount {
			break
		}
	}
	return target
}

func readAtAfter(left, right *string) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	leftTime, leftErr := time.Parse(time.RFC3339Nano, *left)
	rightTime, rightErr := time.Parse(time.RFC3339Nano, *right)
	if leftErr == nil && rightErr == nil {
		return leftTime.After(rightTime)
	}
	return *left > *right
}

func mergeTags(existing, incoming []store.ComicTagInfo) []store.ComicTagInfo {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	result := make([]store.ComicTagInfo, 0, len(existing)+len(incoming))
	for _, tag := range append(append([]store.ComicTagInfo(nil), existing...), incoming...) {
		key := strings.ToLower(strings.TrimSpace(tag.Name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func mergeCategories(existing, incoming []store.ComicCategoryInfo) []store.ComicCategoryInfo {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	result := make([]store.ComicCategoryInfo, 0, len(existing)+len(incoming))
	for _, category := range append(append([]store.ComicCategoryInfo(nil), existing...), incoming...) {
		key := category.Slug
		if key == "" {
			key = fmt.Sprintf("id:%d", category.ID)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, category)
	}
	return result
}

type archiveUnit struct {
	Title        string
	DisplayLabel string
	Path         string
	StartPage    int
	PageCount    int
	CoverPage    int
}

type archiveProbeCacheEntry struct {
	size    int64
	modTime int64
	layout  archiveLayout
}

type archiveLayout struct {
	Units      []archiveUnit
	TotalPages int
}

type archiveImage struct {
	pageIndex int
	dirs      []string
	base      string
	isCover   bool
}

var archiveProbeCache = struct {
	sync.RWMutex
	items map[string]archiveProbeCacheEntry
}{items: make(map[string]archiveProbeCacheEntry)}

// ProbeArchiveChapterUnits recognizes both:
//   - 第001话/001.jpg
//   - 作品名/第001话/001.jpg
//
// It only reads archive headers and never extracts page contents.
func ProbeArchiveChapterUnits(filePath string) []archiveUnit {
	return cloneArchiveUnits(probeArchiveLayout(filePath).Units)
}

func probeArchiveLayout(filePath string) archiveLayout {
	if info, err := os.Stat(filePath); err == nil {
		modTime := info.ModTime().UnixNano()
		archiveProbeCache.RLock()
		cached, ok := archiveProbeCache.items[filePath]
		archiveProbeCache.RUnlock()
		if ok && cached.size == info.Size() && cached.modTime == modTime {
			return cloneArchiveLayout(cached.layout)
		}
		layout := probeArchiveLayoutUncached(filePath)
		archiveProbeCache.Lock()
		archiveProbeCache.items[filePath] = archiveProbeCacheEntry{
			size: info.Size(), modTime: modTime, layout: cloneArchiveLayout(layout),
		}
		archiveProbeCache.Unlock()
		return layout
	}
	return probeArchiveLayoutUncached(filePath)
}

func probeArchiveLayoutUncached(filePath string) archiveLayout {
	reader, err := archive.NewReader(filePath)
	if err != nil {
		return archiveLayout{}
	}
	defer reader.Close()
	images := archive.GetImageEntries(reader)
	if len(images) == 0 {
		return archiveLayout{}
	}

	parsed := make([]archiveImage, 0, len(images))
	for pageIndex, imagePath := range images {
		clean := strings.Trim(strings.ReplaceAll(imagePath, "\\", "/"), "/")
		if clean == "" {
			continue
		}
		dir := path.Dir(clean)
		dirs := []string{}
		if dir != "." && dir != "" {
			dirs = strings.Split(dir, "/")
		}
		base := path.Base(clean)
		parsed = append(parsed, archiveImage{
			pageIndex: pageIndex, dirs: dirs, base: base, isCover: isArchiveCoverName(base),
		})
	}
	if len(parsed) == 0 {
		return archiveLayout{}
	}

	wrapper := detectArchiveWorkWrapper(parsed)

	type bucket struct {
		title        string
		displayLabel string
		path         string
		start        int
		count        int
		coverPage    int
	}
	buckets := make(map[string]*bucket)
	order := make([]string, 0)
	for _, image := range parsed {
		dirs := image.dirs
		if wrapper != "" && len(dirs) > 0 && dirs[0] == wrapper {
			dirs = dirs[1:]
		}
		chapterIndex := deepestArchiveChapterIndex(dirs)
		if chapterIndex < 0 {
			continue
		}
		chapterTitle := strings.TrimSpace(dirs[chapterIndex])
		fullParts := image.dirs[:]
		chapterPathLength := chapterIndex + 1
		if wrapper != "" {
			chapterPathLength++
		}
		if chapterPathLength > len(fullParts) {
			continue
		}
		chapterPath := strings.Join(fullParts[:chapterPathLength], "/")
		sectionParts := dirs[:chapterIndex]
		displayParts := append(append([]string(nil), sectionParts...), chapterTitle)
		displayLabel := strings.Join(displayParts, " / ")
		chapterBucket := buckets[chapterPath]
		if chapterBucket == nil {
			chapterBucket = &bucket{
				title: chapterTitle, displayLabel: displayLabel, path: chapterPath,
				start: image.pageIndex, coverPage: -1,
			}
			buckets[chapterPath] = chapterBucket
			order = append(order, chapterPath)
		}
		chapterBucket.count++
		if image.pageIndex < chapterBucket.start {
			chapterBucket.start = image.pageIndex
		}
		if image.isCover {
			chapterBucket.coverPage = image.pageIndex
		}
	}
	if len(order) == 0 {
		return archiveLayout{TotalPages: len(images)}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return naturalLess(buckets[order[i]].displayLabel, buckets[order[j]].displayLabel)
	})
	units := make([]archiveUnit, 0, len(order))
	for _, key := range order {
		bucket := buckets[key]
		coverPage := bucket.coverPage
		if coverPage < 0 {
			coverPage = bucket.start
		}
		units = append(units, archiveUnit{
			Title: bucket.title, DisplayLabel: bucket.displayLabel, Path: bucket.path,
			StartPage: bucket.start, PageCount: bucket.count, CoverPage: coverPage,
		})
	}
	return archiveLayout{Units: units, TotalPages: len(images)}
}

func detectArchiveWorkWrapper(images []archiveImage) string {
	common := ""
	found := false
	for _, image := range images {
		if image.isCover || len(image.dirs) < 2 {
			continue
		}
		if !found {
			common = image.dirs[0]
			found = true
			continue
		}
		if image.dirs[0] != common {
			return ""
		}
	}
	if !found || looksLikeUnitLabel(common) || isSectionLabel(common) {
		return ""
	}
	return common
}

func deepestArchiveChapterIndex(dirs []string) int {
	for index := len(dirs) - 1; index >= 0; index-- {
		if isChapterUnitLabel(dirs[index]) {
			return index
		}
	}
	return -1
}

func cloneArchiveUnits(units []archiveUnit) []archiveUnit {
	return append([]archiveUnit(nil), units...)
}

func cloneArchiveLayout(layout archiveLayout) archiveLayout {
	layout.Units = cloneArchiveUnits(layout.Units)
	return layout
}

var (
	chineseChapterPattern        = regexp.MustCompile(`(?i)第\s*[零〇一二三四五六七八九十百千万两\d]+(?:\.\d+)?\s*(?:话|話|卷|回|章|册|冊)`)
	chineseChapterReversePattern = regexp.MustCompile(`(?i)(?:话|話|卷|回|章|册|冊)\s*[零〇一二三四五六七八九十百千万两\d]+`)
	chineseNumberedUnitPattern   = regexp.MustCompile(`(?i)^第\s*[零〇一二三四五六七八九十百千万两\d]+(?:\.\d+)?(?:\s*(?:话|話|卷|回|章|册|冊).+|[\s._-]+.+)$`)
	englishUnitPattern           = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:ch(?:apter)?|vol(?:ume)?|ep(?:isode)?)\.?\s*\d+`)
	rootLevelUnitPattern         = regexp.MustCompile(`(?i)^(.+?)[\s._-]+((?:第\s*[零〇一二三四五六七八九十百千万两\d]+\s*(?:话|話|卷|回|章|册|冊).*)|(?:(?:ch(?:apter)?|vol(?:ume)?|ep(?:isode)?)\.?\s*\d+.*))$`)
	sectionPattern               = regexp.MustCompile(`(?i)^(?:第\s*[零〇一二三四五六七八九十百千万两\d]+\s*(?:季|部|篇)|season\s*\d+|part\s*\d+|正传|前传|后传|特别篇|special|extras?)$`)
)

func splitWorkRoot(rel, title string) (root, itemLabel string, standaloneFile bool) {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return "", strings.TrimSpace(title), false
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	isFile := config.IsSupportedArchive(last) || isPDF(last)
	if isFile {
		stem := strings.TrimSuffix(last, filepath.Ext(last))
		if len(parts) == 1 {
			if matches := rootLevelUnitPattern.FindStringSubmatch(stem); len(matches) == 3 {
				return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2]), false
			}
			return stem, "全文", true
		}
		parentParts := parts[:len(parts)-1]
		if strings.EqualFold(strings.TrimSpace(stem), strings.TrimSpace(parentParts[len(parentParts)-1])) {
			return strings.Join(parentParts, "/"), "全文", true
		}
		if archiveNameLooksLikeUnit(stem, parentParts[len(parentParts)-1]) {
			rootParts := trimSectionParents(parentParts)
			labelParts := append(append([]string(nil), parentParts[len(rootParts):]...), stem)
			return strings.Join(rootParts, "/"), strings.Join(labelParts, " / "), false
		}
		standaloneParts := append(append([]string(nil), parentParts...), stem)
		return strings.Join(standaloneParts, "/"), "全文", true
	}
	if len(parts) >= 2 {
		parentParts := parts[:len(parts)-1]
		rootParts := trimSectionParents(parentParts)
		labelParts := parts[len(rootParts):]
		return strings.Join(rootParts, "/"), strings.Join(labelParts, " / "), false
	}
	return parts[0], strings.TrimSpace(title), false
}

func trimSectionParents(parts []string) []string {
	end := len(parts)
	for end > 1 && sectionPattern.MatchString(strings.TrimSpace(parts[end-1])) {
		end--
	}
	return parts[:end]
}

func archiveNameLooksLikeUnit(stem, parentBase string) bool {
	if isCategoryLikeWorkWrapper(parentBase) {
		return false
	}
	return looksLikeUnitLabel(stem)
}

func looksLikeUnitLabel(value string) bool {
	return isChapterUnitLabel(value) || isSectionLabel(value)
}

func isChapterUnitLabel(value string) bool {
	name := strings.TrimSpace(value)
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	if chineseChapterPattern.MatchString(name) || chineseChapterReversePattern.MatchString(name) || chineseNumberedUnitPattern.MatchString(name) || englishUnitPattern.MatchString(name) {
		return true
	}
	for _, marker := range []string{"序章", "终章", "終章", "番外", "特典", "后记", "後記", "extra", "prologue", "epilogue"} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func isSectionLabel(value string) bool {
	return sectionPattern.MatchString(strings.TrimSpace(value))
}

func isCategoryLikeWorkWrapper(name string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "format-") {
		return true
	}
	switch name {
	case "国漫", "大陆漫画", "日漫", "韩漫", "漫画", "manga", "comic", "comics", "pdf", "nowen":
		return true
	default:
		return false
	}
}

func workTitle(root, fallback string) string {
	if root = strings.TrimSpace(root); root != "" {
		return path.Base(root)
	}
	return strings.TrimSpace(fallback)
}

func stableWorkID(libraryID, root string) string {
	sum := sha1.Sum([]byte("work\x00" + libraryID + "\x00" + normalizeWorkPath(root)))
	return fmt.Sprintf("work_%x", sum[:10])
}

func stableUnitID(workID, comicID, internalPath, label string) string {
	sum := sha1.Sum([]byte("unit\x00" + workID + "\x00" + comicID + "\x00" + normalizeWorkPath(internalPath) + "\x00" + strings.TrimSpace(label)))
	return fmt.Sprintf("unit_%x", sum[:10])
}

func normalizeWorkPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Trim(value, "/")
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isZipLike(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".zip" || extension == ".cbz"
}

func isPDF(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pdf")
}

func isArchiveCoverName(base string) bool {
	name := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base))))
	switch name {
	case "cover", "folder", "poster", "front", "封面":
		return true
	default:
		return false
	}
}

func resolveWorkComicPath(item store.ComicListItem, rel string) (string, bool) {
	if strings.TrimSpace(item.LibraryID) != "" {
		if filePath, _, err := FindComicFilePath(item.ID); err == nil && filePath != "" {
			return filePath, true
		}
		if library, err := store.GetLibraryByID(item.LibraryID); err == nil && library != nil {
			roots := library.RootPaths
			if len(roots) == 0 {
				roots = []string{library.RootPath}
			}
			for _, root := range roots {
				filePath := filepath.Join(root, filepath.FromSlash(rel))
				if _, err := os.Stat(filePath); err == nil {
					return filePath, true
				}
			}
		}
	}
	for _, root := range []string{os.Getenv("COMICS_DIR"), config.GetComicsDir()} {
		if strings.TrimSpace(root) != "" {
			filePath := filepath.Join(root, filepath.FromSlash(rel))
			if _, err := os.Stat(filePath); err == nil {
				return filePath, true
			}
		}
	}
	return "", false
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
