package handler

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type WorkHandler struct{}

func NewWorkHandler() *WorkHandler { return &WorkHandler{} }

type workCatalog struct {
	Works []service.Work
	ByID  map[string]*service.Work
}

type workCatalogCacheEntry struct {
	fingerprint string
	catalog     *workCatalog
}

type workCatalogCache struct {
	mu      sync.RWMutex
	entries map[string]workCatalogCacheEntry
}

func newWorkCatalogCache() *workCatalogCache {
	return &workCatalogCache{entries: make(map[string]workCatalogCacheEntry)}
}

var globalWorkCatalogCache = newWorkCatalogCache()

func resetWorkCatalogCache() {
	globalWorkCatalogCache = newWorkCatalogCache()
}

func (cache *workCatalogCache) get(key, fingerprint string, loader func() ([]service.Work, error)) (*workCatalog, error) {
	cache.mu.RLock()
	entry, ok := cache.entries[key]
	cache.mu.RUnlock()
	if ok && entry.fingerprint == fingerprint {
		return cloneWorkCatalog(entry.catalog), nil
	}
	works, err := loader()
	if err != nil {
		return nil, err
	}
	loaded := indexWorkCatalog(works)
	cache.mu.Lock()
	if current, exists := cache.entries[key]; exists && current.fingerprint == fingerprint {
		loaded = current.catalog
	} else {
		cache.entries[key] = workCatalogCacheEntry{fingerprint: fingerprint, catalog: loaded}
	}
	cache.mu.Unlock()
	return cloneWorkCatalog(loaded), nil
}

func indexWorkCatalog(works []service.Work) *workCatalog {
	catalog := &workCatalog{Works: cloneWorks(works), ByID: make(map[string]*service.Work, len(works))}
	for index := range catalog.Works {
		catalog.ByID[catalog.Works[index].ID] = &catalog.Works[index]
	}
	return catalog
}

func cloneWorkCatalog(source *workCatalog) *workCatalog {
	if source == nil {
		return &workCatalog{Works: []service.Work{}, ByID: map[string]*service.Work{}}
	}
	return indexWorkCatalog(source.Works)
}

func cloneWorks(source []service.Work) []service.Work {
	result := append([]service.Work(nil), source...)
	for index := range result {
		result[index].Units = append([]service.WorkUnit(nil), source[index].Units...)
		result[index].Tags = append([]store.ComicTagInfo(nil), source[index].Tags...)
		result[index].Categories = append([]store.ComicCategoryInfo(nil), source[index].Categories...)
	}
	return result
}

func (h *WorkHandler) List(c *gin.Context) {
	h.listWithResponseKey(c, "works")
}

func (h *WorkHandler) ListAsComics(c *gin.Context) {
	h.listWithResponseKey(c, "comics")
}

func (h *WorkHandler) listWithResponseKey(c *gin.Context, responseKey string) {
	works, err := h.loadWorks(c, true)
	if err != nil {
		log.Printf("[works] list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch works"})
		return
	}
	page, pageSize := parseWorkPagination(c)
	total := len(works)
	totalPages := 1
	if pageSize > 0 && total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if pageSize > 0 {
		start := (page - 1) * pageSize
		if start >= total {
			works = []service.Work{}
		} else {
			end := start + pageSize
			if end > total {
				end = total
			}
			works = works[start:end]
		}
	}
	c.Header("Cache-Control", "private, max-age=15, stale-while-revalidate=60")
	c.JSON(http.StatusOK, gin.H{
		responseKey: works, "total": total, "page": page,
		"pageSize": pageSize, "totalPages": totalPages,
	})
}

func (h *WorkHandler) Get(c *gin.Context) {
	work := h.findAccessibleWork(c)
	if work == nil {
		return
	}
	c.JSON(http.StatusOK, work)
}

func (h *WorkHandler) Units(c *gin.Context) {
	work := h.findAccessibleWork(c)
	if work == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"workId": work.ID,
		"title":  work.Title,
		"units":  work.Units,
		"total":  len(work.Units),
	})
}

func (h *WorkHandler) findAccessibleWork(c *gin.Context) *service.Work {
	catalog, err := h.loadWorkCatalog(c)
	if err != nil {
		log.Printf("[works] detail failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch works"})
		return nil
	}
	id := strings.TrimSpace(c.Param("id"))
	if work := catalog.ByID[id]; work != nil {
		return work
	}
	// Do not reveal whether a Work exists in an inaccessible library.
	c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
	return nil
}

func (h *WorkHandler) loadWorks(c *gin.Context, applyFilters bool) ([]service.Work, error) {
	catalog, err := h.loadWorkCatalog(c)
	if err != nil {
		return nil, err
	}
	works := cloneWorks(catalog.Works)
	if applyFilters {
		works = filterWorks(works, parseWorkFilters(c))
		sortWorks(works, c.DefaultQuery("sortBy", "title"), c.DefaultQuery("sortOrder", "asc"))
	}
	return works, nil
}

func (h *WorkHandler) loadWorkCatalog(c *gin.Context) (*workCatalog, error) {
	userID := getUserID(c)
	libraryIDs, filterLibraries, err := resolveWorkLibraryScope(c)
	if err != nil {
		return nil, err
	}
	if filterLibraries && len(libraryIDs) == 0 {
		return indexWorkCatalog(nil), nil
	}
	if err := service.EnsureComicSeriesFresh(); err != nil {
		log.Printf("[works] series metadata unavailable: %v", err)
	}
	if err := store.MigrateComicSeriesToLogicalWorks(); err != nil {
		log.Printf("[works] series-to-work metadata migration unavailable: %v", err)
	}
	fingerprint, err := store.GetWorkSourceFingerprint(userID, libraryIDs, filterLibraries)
	if err != nil {
		return nil, err
	}
	cacheKey := workCatalogCacheKey(userID, libraryIDs, filterLibraries)
	return globalWorkCatalogCache.get(cacheKey, fingerprint, func() ([]service.Work, error) {
		result, queryErr := store.GetAllComics(store.ComicListOptions{
			ContentType:      "comic",
			SortBy:           "title",
			SortOrder:        "asc",
			Page:             0,
			PageSize:         0,
			UserID:           userID,
			LibraryIDs:       libraryIDs,
			FilterLibraryIDs: filterLibraries,
		})
		if queryErr != nil {
			return nil, queryErr
		}
		if hydrateErr := hydrateWorkLibraryIDs(result.Comics); hydrateErr != nil {
			return nil, hydrateErr
		}
		works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{ProbeInternalArchive: true})
		comicIDs := make([]string, 0, len(result.Comics))
		for _, comic := range result.Comics {
			comicIDs = append(comicIDs, comic.ID)
		}
		statuses, statusErr := store.GetComicWorkStatuses(comicIDs)
		if statusErr != nil {
			return nil, statusErr
		}
		service.ApplyComicWorkStatuses(works, statuses)
		workIDs := make([]string, 0, len(works))
		for _, work := range works {
			workIDs = append(workIDs, work.ID)
		}
		orders, orderErr := store.GetWorkSortOrders(workIDs)
		if orderErr != nil {
			return nil, orderErr
		}
		service.ApplyWorkSortOrders(works, orders)
		summaries, seriesErr := store.ListSeriesSummaries(libraryIDs, userID, "")
		if seriesErr != nil {
			log.Printf("[works] list series metadata failed: %v", seriesErr)
		} else {
			service.ApplySeriesMetadata(works, summaries)
		}
		if persistErr := persistAndApplyLogicalWorks(works); persistErr != nil {
			return nil, persistErr
		}
		return works, nil
	})
}

func persistAndApplyLogicalWorks(works []service.Work) error {
	if !store.LogicalWorkPersistenceAvailable() {
		return nil
	}
	seeds := make([]store.LogicalWorkSeed, 0, len(works))
	workIDs := make([]string, 0, len(works))
	for _, work := range works {
		seeds = append(seeds, store.LogicalWorkSeed{
			ID:               work.ID,
			LibraryID:        work.LibraryID,
			RootPath:         work.RootPath,
			ContentType:      "comic",
			Title:            work.Title,
			CoverComicID:     work.CoverComicID,
			CoverAspectRatio: work.CoverAspectRatio,
		})
		workIDs = append(workIDs, work.ID)
	}
	if err := store.UpsertDetectedLogicalWorks(seeds); err != nil {
		return err
	}
	for _, work := range works {
		if err := store.SeedLogicalWorkRelations(work.ID, work.Tags, work.Categories); err != nil {
			return err
		}
	}
	persisted, err := store.GetLogicalWorksByIDs(workIDs)
	if err != nil {
		return err
	}
	tags, err := store.GetLogicalWorkTagsByWorkIDs(workIDs)
	if err != nil {
		return err
	}
	categories, err := store.GetLogicalWorkCategoriesByWorkIDs(workIDs)
	if err != nil {
		return err
	}
	service.ApplyLogicalWorkMetadata(works, persisted, tags, categories)
	return nil
}

func workCatalogCacheKey(userID string, libraryIDs []string, filterLibraries bool) string {
	ids := append([]string(nil), libraryIDs...)
	sort.Strings(ids)
	return fmt.Sprintf("%s|%t|%s", userID, filterLibraries, strings.Join(ids, ","))
}

func hydrateWorkLibraryIDs(comics []store.ComicListItem) error {
	ids := make([]string, 0, len(comics))
	for _, comic := range comics {
		if comic.LibraryID == "" {
			ids = append(ids, comic.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	libraryByComic, err := store.GetComicsLibraryIDsByIDs(ids)
	if err != nil {
		return err
	}
	for index := range comics {
		if comics[index].LibraryID == "" {
			comics[index].LibraryID = libraryByComic[comics[index].ID]
		}
	}
	return nil
}

func resolveWorkLibraryScope(c *gin.Context) ([]string, bool, error) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return nil, true, nil
	}
	requested := append(splitQueryIDs(c.Query("libraryIds")), splitQueryIDs(c.Query("libraryId"))...)
	requested = uniqueStrings(requested)
	if user.Role == "admin" {
		return requested, len(requested) > 0, nil
	}
	accessible, err := store.GetUserAccessibleLibraryIDs(user.ID)
	if err != nil {
		return nil, true, err
	}
	if len(requested) == 0 {
		return accessible, true, nil
	}
	allowed := make(map[string]struct{}, len(accessible))
	for _, id := range accessible {
		allowed[id] = struct{}{}
	}
	intersection := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, ok := allowed[id]; ok {
			intersection = append(intersection, id)
		}
	}
	return intersection, true, nil
}

type workFilters struct {
	search        string
	tags          []string
	favoritesOnly bool
	category      string
	readingStatus string
	metaFilter    string
	uncategorized bool
	untagged      bool
}

func parseWorkFilters(c *gin.Context) workFilters {
	return workFilters{
		search:        strings.TrimSpace(c.Query("search")),
		tags:          splitQueryIDs(c.Query("tags")),
		favoritesOnly: c.Query("favorites") == "true",
		category:      strings.TrimSpace(c.Query("category")),
		readingStatus: strings.TrimSpace(c.Query("readingStatus")),
		metaFilter:    strings.TrimSpace(c.Query("metaFilter")),
		uncategorized: c.Query("uncategorized") == "true",
		untagged:      c.Query("untagged") == "true",
	}
}

func filterWorks(works []service.Work, filters workFilters) []service.Work {
	if filters.search == "" && len(filters.tags) == 0 && !filters.favoritesOnly &&
		filters.category == "" && filters.readingStatus == "" && filters.metaFilter == "" &&
		!filters.uncategorized && !filters.untagged {
		return works
	}
	result := make([]service.Work, 0, len(works))
	for _, work := range works {
		if !workMatchesFilters(work, filters) {
			continue
		}
		result = append(result, work)
	}
	return result
}

func workMatchesFilters(work service.Work, filters workFilters) bool {
	if filters.search != "" && !workContainsSearch(work, filters.search) {
		return false
	}
	if filters.favoritesOnly && !work.IsFavorite {
		return false
	}
	if filters.readingStatus != "" && !strings.EqualFold(work.ReadingStatus, filters.readingStatus) {
		return false
	}
	hasMetadata := workHasMetadata(work)
	if filters.metaFilter == "with" && !hasMetadata {
		return false
	}
	if filters.metaFilter == "missing" && hasMetadata {
		return false
	}
	if filters.untagged && len(work.Tags) > 0 {
		return false
	}
	if len(filters.tags) > 0 && !workHasAnyTag(work, filters.tags) {
		return false
	}
	if filters.uncategorized || filters.category == "uncategorized" {
		if len(work.Categories) > 0 {
			return false
		}
	} else if filters.category != "" && !workHasCategory(work, filters.category) {
		return false
	}
	return true
}

func workHasMetadata(work service.Work) bool {
	return strings.TrimSpace(work.MetadataSource) != "" ||
		strings.TrimSpace(work.Author) != "" ||
		strings.TrimSpace(work.Publisher) != "" ||
		strings.TrimSpace(work.Description) != "" ||
		strings.TrimSpace(work.Genre) != "" ||
		work.Year != nil ||
		work.ExternalRating != nil ||
		(work.MetadataHostType == "series" && strings.TrimSpace(work.CoverURL) != "")
}

func workContainsSearch(work service.Work, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	for _, value := range []string{
		work.Title, work.RootPath, work.Author, work.Publisher,
		work.Description, work.Genre,
	} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	for _, unit := range work.Units {
		if strings.Contains(strings.ToLower(unit.DisplayLabel), query) ||
			strings.Contains(strings.ToLower(unit.Title), query) {
			return true
		}
	}
	return false
}

func workHasAnyTag(work service.Work, requested []string) bool {
	tags := make(map[string]struct{}, len(work.Tags))
	for _, tag := range work.Tags {
		tags[strings.ToLower(strings.TrimSpace(tag.Name))] = struct{}{}
	}
	for _, tag := range requested {
		if _, ok := tags[strings.ToLower(strings.TrimSpace(tag))]; ok {
			return true
		}
	}
	return false
}

func workHasCategory(work service.Work, requested string) bool {
	requested = strings.ToLower(strings.TrimSpace(requested))
	for _, category := range work.Categories {
		if strings.EqualFold(category.Slug, requested) || strings.EqualFold(category.Name, requested) {
			return true
		}
	}
	return false
}

func sortWorks(works []service.Work, sortBy, sortOrder string) {
	descending := strings.EqualFold(sortOrder, "desc")
	sort.SliceStable(works, func(i, j int) bool {
		left, right := works[i], works[j]
		less := false
		switch sortBy {
		case "addedAt":
			less = left.AddedAt < right.AddedAt
		case "updatedAt":
			less = left.UpdatedAt < right.UpdatedAt
		case "custom":
			less = left.SortOrder < right.SortOrder
		case "lastReadAt":
			less = compareOptionalStrings(left.LastReadAt, right.LastReadAt) < 0
		case "rating":
			less = compareOptionalInts(left.Rating, right.Rating) < 0
		case "fileSize":
			less = left.FileSize < right.FileSize
		case "pageCount":
			less = left.PageCount < right.PageCount
		default:
			less = naturalWorkLess(left.Title, right.Title)
		}
		if descending {
			return !less && !workSortEqual(left, right, sortBy)
		}
		return less
	})
}

func naturalWorkLess(left, right string) bool {
	return service.NaturalLess(left, right)
}

func workSortEqual(left, right service.Work, sortBy string) bool {
	switch sortBy {
	case "addedAt":
		return left.AddedAt == right.AddedAt
	case "updatedAt":
		return left.UpdatedAt == right.UpdatedAt
	case "custom":
		return left.SortOrder == right.SortOrder
	case "lastReadAt":
		return compareOptionalStrings(left.LastReadAt, right.LastReadAt) == 0
	case "rating":
		return compareOptionalInts(left.Rating, right.Rating) == 0
	case "fileSize":
		return left.FileSize == right.FileSize
	case "pageCount":
		return left.PageCount == right.PageCount
	default:
		return left.Title == right.Title
	}
}

func compareOptionalStrings(left, right *string) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	return strings.Compare(*left, *right)
}

func compareOptionalInts(left, right *int) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	if *left < *right {
		return -1
	}
	if *left > *right {
		return 1
	}
	return 0
}

func parseWorkPagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 0
	if value, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(c.Query("pageSize")); err == nil && value > 0 {
		pageSize = value
	}
	return page, pageSize
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
