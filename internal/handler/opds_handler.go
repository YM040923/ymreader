package handler

import (
	stdzip "archive/zip"
	"crypto/sha256"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

const (
	opdsDefaultPageSize = 100
	opdsMaxPageSize     = 500
	opdsPSEMaxWidth     = 4096
	opdsWorkIndexTTL    = 30 * time.Second
)

var (
	opdsVirtualCBZSlots       = make(chan struct{}, 2)
	opdsVirtualCBZCleanupOnce sync.Once
)

type opdsWorkIndexCacheEntry struct {
	permissionScope string
	expiresAt       time.Time
	byID            map[string]opdsWorkCatalogItem
}

type OPDSHandler struct {
	workIndexMu sync.Mutex
	workIndexes map[string]opdsWorkIndexCacheEntry
	workLoader  func(*gin.Context) ([]opdsWorkCatalogItem, error)
}

func NewOPDSHandler() *OPDSHandler {
	return &OPDSHandler{
		workIndexes: make(map[string]opdsWorkIndexCacheEntry),
		workLoader:  loadOPDSWorks,
	}
}

func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	prefix := config.BasePath()

	if config.TrustProxyHeaders() {
		if forwardedProto := strings.ToLower(firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))); forwardedProto == "http" || forwardedProto == "https" {
			scheme = forwardedProto
		}
		if forwardedHost := firstForwardedValue(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
			host = forwardedHost
		}
		if prefix == "" || prefix == "/" {
			if fwdPrefix := c.GetHeader("X-Forwarded-Prefix"); fwdPrefix != "" {
				if norm, err := config.NormalizeBasePath(fwdPrefix); err == nil {
					prefix = norm
				} else {
					prefix = ""
				}
			} else {
				prefix = ""
			}
		}
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, prefix)
}

func firstForwardedValue(value string) string {
	if index := strings.Index(value, ","); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

// GET /api/opds
func (h *OPDSHandler) Root(c *gin.Context) {
	setOPDSPrivateResponseHeaders(c)
	xml := service.GenerateRootCatalog(getBaseURL(c))
	c.Data(http.StatusOK, service.OPDSNavigationMIME, []byte(xml))
}

// GET /api/opds/search.xml
func (h *OPDSHandler) SearchDescription(c *gin.Context) {
	setOPDSPrivateResponseHeaders(c)
	xml := service.GenerateOpenSearchDescription(getBaseURL(c))
	c.Data(http.StatusOK, service.OpenSearchMIME, []byte(xml))
}

// GET /api/opds/all
func (h *OPDSHandler) All(c *gin.Context) {
	h.renderWorkNavigationFeed(c, "All Comics")
}

// GET /api/opds/recent
func (h *OPDSHandler) Recent(c *gin.Context) {
	h.renderFilteredWorkNavigationFeed(c, "Recently Added", "recent")
}

// GET /api/opds/favorites
func (h *OPDSHandler) Favorites(c *gin.Context) {
	h.renderFilteredWorkNavigationFeed(c, "Favorites", "favorites")
}

// GET /api/opds/works
func (h *OPDSHandler) Works(c *gin.Context) {
	h.renderWorkNavigationFeed(c, "Works")
}

func (h *OPDSHandler) renderWorkNavigationFeed(c *gin.Context, title string) {
	h.renderFilteredWorkNavigationFeed(c, title, "")
}

func (h *OPDSHandler) renderFilteredWorkNavigationFeed(c *gin.Context, title, mode string) {
	items, err := loadOPDSWorks(c)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to get works"))
		return
	}
	if mode == "favorites" {
		items = filterOPDSWorkItems(items, func(item opdsWorkCatalogItem) bool { return item.Work.IsFavorite })
	}
	if mode == "search" {
		query := strings.TrimSpace(c.Query("q"))
		items = filterOPDSWorkItems(items, func(item opdsWorkCatalogItem) bool {
			return workContainsSearch(item.Work, query)
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if mode == "recent" && items[i].AddedAt != items[j].AddedAt {
			return items[i].AddedAt > items[j].AddedAt
		}
		return naturalWorkLess(items[i].Work.Title, items[j].Work.Title)
	})
	page, pageSize := parseOPDSPagination(c)
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = []opdsWorkCatalogItem{}
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}
	works := make([]service.OPDSWork, 0, len(items))
	for _, item := range items {
		works = append(works, item.OPDSWork())
	}
	baseURL := getBaseURL(c)
	xml := service.GenerateWorkNavigationFeed(baseURL, title, opdsFeedID(baseURL, c), works, buildOPDSPagination(c, page, pageSize, total))
	setOPDSPrivateResponseHeaders(c)
	c.Data(http.StatusOK, service.OPDSNavigationMIME, []byte(xml))
}

// GET /api/opds/works/:id
func (h *OPDSHandler) WorkDetail(c *gin.Context) {
	if h.renderWorkDetail(c, c.Param("id")) {
		return
	}
	c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("Work not found"))
}

func (h *OPDSHandler) renderWorkDetail(c *gin.Context, workID string) bool {
	items, err := loadOPDSWorks(c)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to get works"))
		return true
	}
	for _, item := range items {
		work := item.Work
		if work.ID != workID {
			continue
		}
		rows := make([]service.OPDSComic, 0, len(work.Units)+1)
		rows = append(rows, service.OPDSComic{
			ID:              work.ID,
			EntryID:         "continuous_" + work.ID,
			Title:           "连续阅读（整部）",
			Author:          work.Author,
			Description:     work.Description,
			Language:        work.Language,
			Genre:           work.Genre,
			Publisher:       work.Publisher,
			PageCount:       work.PageCount,
			FileSize:        0,
			AddedAt:         work.AddedAt,
			UpdatedAt:       work.UpdatedAt,
			Tags:            comicTagNames(work.Tags),
			Filename:        work.Title + ".cbz",
			ComicType:       "comic",
			CoverHref:       "/api/opds/work-cover/" + url.PathEscape(work.ID),
			AcquisitionHref: "/api/opds/works/" + url.PathEscape(work.ID) + "/continuous/download",
			AcquisitionType: "application/vnd.comicbook+zip",
			StreamHref:      "/api/opds/works/" + url.PathEscape(work.ID) + "/continuous/stream?page={pageNumber}&width={maxWidth}",
		})
		for _, unit := range work.Units {
			comic, ok := getOPDSPublication(unit.ComicID)
			if !ok {
				continue
			}
			year := 0
			if comic.Year != nil {
				year = *comic.Year
			}
			streamStartPage := 0
			streamPageCount := 0
			if unit.InternalPath != "" {
				streamStartPage = unit.StartPage
				streamPageCount = unit.PageCount
			}
			lastReadAt := ""
			if unit.LastReadAt != nil {
				lastReadAt = *unit.LastReadAt
			}
			row := service.OPDSComic{
				ID:                  comic.ID,
				EntryID:             unit.ID,
				Title:               unit.DisplayLabel,
				Author:              comic.Author,
				Description:         comic.Description,
				Language:            comic.Language,
				Genre:               comic.Genre,
				Publisher:           comic.Publisher,
				Year:                year,
				PageCount:           unit.PageCount,
				FileSize:            comic.FileSize,
				AddedAt:             comic.AddedAt,
				UpdatedAt:           comic.UpdatedAt,
				Tags:                comicTagNames(comic.Tags),
				Filename:            comic.Filename,
				ComicType:           comic.ComicType,
				LastReadPage:        unit.LastReadPage,
				LastReadAt:          lastReadAt,
				StreamStartPage:     streamStartPage,
				StreamPageCount:     streamPageCount,
				CoverHref:           opdsUnitCoverHref(unit),
				SuppressAcquisition: unit.InternalPath != "",
			}
			_, hasPhysicalAcquisition := service.OPDSAcquisitionMIMEForFilename(comic.Filename)
			if unit.InternalPath != "" || !hasPhysicalAcquisition {
				row.SuppressAcquisition = false
				row.AcquisitionHref = "/api/opds/units/" + url.PathEscape(unit.ID) + "/download"
				row.AcquisitionType = "application/vnd.comicbook+zip"
				row.FileSize = 0
			}
			rows = append(rows, row)
		}
		page, pageSize := parseOPDSPagination(c)
		total := len(rows)
		start := (page - 1) * pageSize
		if start >= total {
			rows = []service.OPDSComic{}
		} else {
			end := start + pageSize
			if end > total {
				end = total
			}
			rows = rows[start:end]
		}
		baseURL := getBaseURL(c)
		xml := service.GenerateAcquisitionFeed(service.OPDSAcquisitionFeedOptions{
			BaseURL:    baseURL,
			Title:      work.Title,
			FeedID:     opdsFeedID(baseURL, c),
			Comics:     rows,
			Pagination: buildOPDSPagination(c, page, pageSize, total),
		})
		setOPDSPrivateResponseHeaders(c)
		c.Data(http.StatusOK, service.OPDSAcquisitionMIME, []byte(xml))
		return true
	}
	return false
}

func (h *OPDSHandler) findDownloadableWork(c *gin.Context, workID string) (*service.Work, bool) {
	items, err := loadOPDSWorks(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get works"})
		return nil, false
	}
	for index := range items {
		if items[index].Work.ID == workID {
			return &items[index].Work, true
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
	return nil, false
}

func (h *OPDSHandler) findDownloadableUnit(c *gin.Context, unitID string) (*service.Work, *service.WorkUnit, bool) {
	items, err := loadOPDSWorks(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get works"})
		return nil, nil, false
	}
	for workIndex := range items {
		work := &items[workIndex].Work
		for unitIndex := range work.Units {
			if work.Units[unitIndex].ID == unitID {
				return work, &work.Units[unitIndex], true
			}
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Unit not found"})
	return nil, nil, false
}

// GET/HEAD /api/opds/units/:id/download
func (h *OPDSHandler) UnitDownload(c *gin.Context) {
	work, unit, ok := h.findDownloadableUnit(c, c.Param("id"))
	if !ok {
		return
	}
	h.renderVirtualCBZ(c, work.Title+" - "+unit.DisplayLabel, []service.WorkUnit{*unit})
}

// GET/HEAD /api/opds/works/:id/continuous/download
func (h *OPDSHandler) WorkContinuousDownload(c *gin.Context) {
	work, ok := h.findDownloadableWork(c, c.Param("id"))
	if !ok {
		return
	}
	h.renderVirtualCBZ(c, work.Title, work.Units)
}

func (h *OPDSHandler) renderVirtualCBZ(c *gin.Context, title string, units []service.WorkUnit) {
	filename := sanitizeOPDSFilename(title) + ".cbz"
	select {
	case opdsVirtualCBZSlots <- struct{}{}:
		defer func() { <-opdsVirtualCBZSlots }()
	case <-c.Request.Context().Done():
		return
	}

	tempDir := filepath.Join(config.DataDir(), "opds-virtual")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare virtual publication"})
		return
	}
	opdsVirtualCBZCleanupOnce.Do(func() { cleanupStaleVirtualCBZs(tempDir, 24*time.Hour) })
	tempFile, err := os.CreateTemp(tempDir, "publication-*.cbz")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare virtual publication"})
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if err := writeVirtualCBZ(c, tempFile, units); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate virtual publication"})
		return
	}
	info, err := tempFile.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to finalize virtual publication"})
		return
	}
	if _, err := tempFile.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to finalize virtual publication"})
		return
	}
	setVirtualCBZResponseHeaders(c, filename, info.Size())
	http.ServeContent(c.Writer, c.Request, filename, time.Time{}, tempFile)
}

func cleanupStaleVirtualCBZs(directory string, maxAge time.Duration) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "publication-") || !strings.HasSuffix(entry.Name(), ".cbz") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(directory, entry.Name()))
		}
	}
}

func writeVirtualCBZ(c *gin.Context, output *os.File, units []service.WorkUnit) error {
	writer := stdzip.NewWriter(output)
	pageNumber := 1
	for _, unit := range units {
		for offset := 0; offset < unit.PageCount; offset++ {
			select {
			case <-c.Request.Context().Done():
				_ = writer.Close()
				return c.Request.Context().Err()
			default:
			}
			result, err := service.GetOPDSPSEPageImage(unit.ComicID, unit.StartPage+offset, 0)
			if err != nil || result == nil || len(result.Data) == 0 {
				_ = writer.Close()
				if err != nil {
					return err
				}
				return fmt.Errorf("page %d of comic %s is unavailable", unit.StartPage+offset, unit.ComicID)
			}
			header := &stdzip.FileHeader{
				Name:   fmt.Sprintf("%06d.jpg", pageNumber),
				Method: stdzip.Store,
			}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				_ = writer.Close()
				return err
			}
			if _, err := entry.Write(result.Data); err != nil {
				_ = writer.Close()
				return err
			}
			pageNumber++
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return output.Sync()
}

func setVirtualCBZResponseHeaders(c *gin.Context, filename string, fileSize int64) {
	c.Header("Content-Type", "application/vnd.comicbook+zip")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Accept-Ranges", "bytes")
	if fileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
	}
	setOPDSPrivateResponseHeaders(c)
}

func sanitizeOPDSFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "comic"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")
	value = strings.Trim(replacer.Replace(value), ". ")
	if value == "" {
		return "comic"
	}
	return value
}

// GET/HEAD /api/opds/works/:id/continuous/stream?page={pageNumber}&width={maxWidth}
func (h *OPDSHandler) WorkContinuousStream(c *gin.Context) {
	work, ok := h.findDownloadableWork(c, c.Param("id"))
	if !ok {
		return
	}
	pageIndex, err := strconv.Atoi(c.Query("page"))
	if err != nil || pageIndex < 0 || pageIndex >= work.PageCount {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	maxWidth := 0
	if rawWidth := strings.TrimSpace(c.Query("width")); rawWidth != "" {
		maxWidth, err = strconv.Atoi(rawWidth)
		if err != nil || maxWidth < 0 || maxWidth > opdsPSEMaxWidth {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("width must be between 0 and %d", opdsPSEMaxWidth)})
			return
		}
	}
	unit, pageInUnit := continuousWorkPage(work.Units, pageIndex)
	if unit == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	result, err := service.GetOPDSPSEPageImage(unit.ComicID, unit.StartPage+pageInUnit, maxWidth)
	if err != nil || result == nil || len(result.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	sum := sha256.Sum256(result.Data)
	etag := fmt.Sprintf(`"%x"`, sum[:12])
	c.Header("Content-Type", "image/jpeg")
	c.Header("Content-Length", strconv.Itoa(len(result.Data)))
	c.Header("Cache-Control", "private, max-age=86400, must-revalidate")
	c.Header("Vary", "Authorization, Cookie")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, "image/jpeg", result.Data)
}

func continuousWorkPage(units []service.WorkUnit, pageIndex int) (*service.WorkUnit, int) {
	for index := range units {
		if pageIndex < units[index].PageCount {
			return &units[index], pageIndex
		}
		pageIndex -= units[index].PageCount
	}
	return nil, 0
}

type opdsWorkCatalogItem struct {
	Work      service.Work
	AddedAt   string
	UpdatedAt string
}

func (item opdsWorkCatalogItem) OPDSWork() service.OPDSWork {
	return service.OPDSWork{
		Work:      item.Work,
		CoverHref: "/api/opds/work-cover/" + url.PathEscape(item.Work.ID),
		AddedAt:   item.AddedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func filterOPDSWorkItems(items []opdsWorkCatalogItem, keep func(opdsWorkCatalogItem) bool) []opdsWorkCatalogItem {
	result := make([]opdsWorkCatalogItem, 0, len(items))
	for _, item := range items {
		if keep(item) {
			result = append(result, item)
		}
	}
	return result
}

func loadOPDSWorks(c *gin.Context) ([]opdsWorkCatalogItem, error) {
	comicLibraryIDs, err := getOPDSDownloadableComicLibraryIDs(c)
	if err != nil {
		return nil, err
	}
	if len(comicLibraryIDs) == 0 {
		return []opdsWorkCatalogItem{}, nil
	}
	allowed := make(map[string]struct{}, len(comicLibraryIDs))
	for _, id := range comicLibraryIDs {
		allowed[id] = struct{}{}
	}
	works, err := NewWorkHandler().loadWorks(c, false)
	if err != nil {
		return nil, err
	}
	filtered := make([]service.Work, 0, len(works))
	for _, work := range works {
		if _, ok := allowed[work.LibraryID]; ok {
			filtered = append(filtered, work)
		}
	}
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType:      "comic",
		Page:             0,
		PageSize:         0,
		UserID:           getOPDSUserID(c),
		LibraryIDs:       comicLibraryIDs,
		FilterLibraryIDs: true,
	})
	if err != nil {
		return nil, err
	}
	timesByComic := make(map[string][2]string, len(result.Comics))
	for _, comic := range result.Comics {
		timesByComic[comic.ID] = [2]string{comic.AddedAt, comic.UpdatedAt}
	}
	items := make([]opdsWorkCatalogItem, 0, len(filtered))
	for _, work := range filtered {
		item := opdsWorkCatalogItem{Work: work}
		for _, unit := range work.Units {
			values := timesByComic[unit.ComicID]
			item.AddedAt = maxAtomTime(item.AddedAt, values[0])
			item.UpdatedAt = maxAtomTime(item.UpdatedAt, values[1])
		}
		item.UpdatedAt = maxAtomTime(item.UpdatedAt, work.UpdatedAt)
		items = append(items, item)
	}
	return items, nil
}

func getOPDSDownloadableComicLibraryIDs(c *gin.Context) ([]string, error) {
	libraryIDs, err := store.GetUserDownloadableLibraryIDs(getOPDSUserID(c))
	if err != nil {
		return nil, err
	}
	comicLibraryIDs := make([]string, 0, len(libraryIDs))
	for _, id := range libraryIDs {
		library, libraryErr := store.GetLibraryByID(id)
		if libraryErr != nil {
			return nil, libraryErr
		}
		if library != nil && library.Enabled && library.Type == "comic" {
			comicLibraryIDs = append(comicLibraryIDs, id)
		}
	}
	sort.Strings(comicLibraryIDs)
	return comicLibraryIDs, nil
}

func maxAtomTime(left, right string) string {
	leftTime, leftErr := time.Parse(time.RFC3339, left)
	rightTime, rightErr := time.Parse(time.RFC3339, right)
	if rightErr == nil && (leftErr != nil || rightTime.After(leftTime)) {
		return rightTime.UTC().Format(time.RFC3339)
	}
	return left
}

func opdsUnitCoverHref(unit service.WorkUnit) string {
	if unit.InternalPath != "" {
		return fmt.Sprintf("/api/opds/unit-cover/%s?page=%d", url.PathEscape(unit.ComicID), unit.CoverPage)
	}
	return "/api/opds/cover/" + url.PathEscape(unit.ComicID)
}

// GET /api/opds/series
func (h *OPDSHandler) Series(c *gin.Context) {
	h.renderWorkNavigationFeed(c, "Series")
}

// GET /api/opds/series/:id
func (h *OPDSHandler) SeriesDetail(c *gin.Context) {
	if h.renderWorkDetail(c, c.Param("id")) {
		return
	}
	series, libraryIDs, ok := getAccessibleOPDSSeries(c, c.Param("id"))
	if !ok {
		return
	}
	page, pageSize := parseOPDSPagination(c)
	rows, total, err := store.GetOPDSComics(store.OPDSQueryOptions{
		LibraryIDs: libraryIDs,
		UserID:     getOPDSUserID(c),
		SeriesID:   series.ID,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	})
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to get series comics"))
		return
	}

	baseURL := getBaseURL(c)
	xml := service.GenerateAcquisitionFeed(service.OPDSAcquisitionFeedOptions{
		BaseURL:    baseURL,
		Title:      series.Title,
		FeedID:     opdsFeedID(baseURL, c),
		Comics:     toOPDSSeriesComics(rows),
		Pagination: buildOPDSPagination(c, page, pageSize, total),
	})
	setOPDSPrivateResponseHeaders(c)
	c.Data(http.StatusOK, service.OPDSAcquisitionMIME, []byte(xml))
}

// GET /api/opds/series/:id/cover
func (h *OPDSHandler) SeriesCover(c *gin.Context) {
	series, _, ok := getAccessibleOPDSSeries(c, c.Param("id"))
	if !ok {
		return
	}
	h.renderCover(c, series.CoverComicID)
}

// GET /api/opds/search?q=...
func (h *OPDSHandler) Search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.Data(http.StatusBadRequest, "text/plain; charset=utf-8", []byte("q parameter required"))
		return
	}
	h.renderFilteredWorkNavigationFeed(c, "Search: "+query, "search")
}

func (h *OPDSHandler) renderAcquisitionFeed(c *gin.Context, title string, opts store.OPDSQueryOptions) {
	userID := getOPDSUserID(c)
	libraryIDs, err := store.GetUserDownloadableLibraryIDs(userID)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to resolve library access"))
		return
	}

	page, pageSize := parseOPDSPagination(c)
	opts.LibraryIDs = libraryIDs
	opts.UserID = userID
	opts.Limit = pageSize
	opts.Offset = (page - 1) * pageSize
	rows, total, err := store.GetOPDSComics(opts)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to get comics"))
		return
	}

	baseURL := getBaseURL(c)
	pagination := buildOPDSPagination(c, page, pageSize, total)
	xml := service.GenerateAcquisitionFeed(service.OPDSAcquisitionFeedOptions{
		BaseURL:    baseURL,
		Title:      title,
		FeedID:     opdsFeedID(baseURL, c),
		Comics:     toOPDSComics(rows),
		Pagination: pagination,
	})
	setOPDSPrivateResponseHeaders(c)
	c.Data(http.StatusOK, service.OPDSAcquisitionMIME, []byte(xml))
}

func parseOPDSPagination(c *gin.Context) (page, pageSize int) {
	page = 1
	pageSize = opdsDefaultPageSize
	if value, err := strconv.Atoi(c.Query("page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(c.Query("pageSize")); err == nil && value > 0 {
		pageSize = value
	}
	if pageSize > opdsMaxPageSize {
		pageSize = opdsMaxPageSize
	}
	return page, pageSize
}

func buildOPDSPagination(c *gin.Context, page, pageSize, total int) service.OPDSPagination {
	lastPage := 1
	startIndex := 0
	if total > 0 {
		lastPage = (total + pageSize - 1) / pageSize
		startIndex = (page-1)*pageSize + 1
	}
	result := service.OPDSPagination{
		SelfHref:     opdsPageHref(c, page, pageSize),
		FirstHref:    opdsPageHref(c, 1, pageSize),
		LastHref:     opdsPageHref(c, lastPage, pageSize),
		TotalResults: total,
		ItemsPerPage: pageSize,
		StartIndex:   startIndex,
	}
	if page > 1 {
		result.PreviousHref = opdsPageHref(c, page-1, pageSize)
	}
	if page < lastPage {
		result.NextHref = opdsPageHref(c, page+1, pageSize)
	}
	return result
}

func opdsPageHref(c *gin.Context, page, pageSize int) string {
	query := cloneURLValues(c.Request.URL.Query())
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))
	return (&url.URL{Path: c.Request.URL.Path, RawQuery: query.Encode()}).String()
}

func opdsFeedID(baseURL string, c *gin.Context) string {
	query := make(url.Values)
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		query.Set("q", search)
	}
	feedURL := &url.URL{Path: c.Request.URL.Path, RawQuery: query.Encode()}
	return strings.TrimRight(baseURL, "/") + feedURL.String()
}

func cloneURLValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, entries := range values {
		clone[key] = append([]string(nil), entries...)
	}
	return clone
}

// GET /api/opds/cover/:id
func (h *OPDSHandler) Cover(c *gin.Context) {
	h.renderCover(c, c.Param("id"))
}

// GET /api/opds/work-cover/:id
func (h *OPDSHandler) WorkCover(c *gin.Context) {
	item, found, err := h.findIndexedDownloadableWork(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get works"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	if logicalWork, logicalErr := store.GetLogicalWork(item.Work.ID); logicalErr == nil && logicalWork != nil {
		h.renderLogicalWorkCover(c, logicalWork, item.Work.CoverComicID)
		return
	}
	if item.Work.SeriesID != "" {
		h.renderSeriesWorkCover(c, item.Work.SeriesID, item.Work.CoverComicID)
		return
	}
	h.renderCover(c, item.Work.CoverComicID)
}

func (h *OPDSHandler) renderLogicalWorkCover(c *gin.Context, work *store.LogicalWork, fallbackComicID string) {
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(work.ID))
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		h.renderOPDSImage(c, data, http.DetectContentType(data), 300)
		return
	}
	if work.CoverURL != "" && (strings.HasPrefix(work.CoverURL, "http://") || strings.HasPrefix(work.CoverURL, "https://")) {
		go service.DownloadWorkCover(work.ID, work.CoverURL)
		c.Redirect(http.StatusTemporaryRedirect, work.CoverURL)
		return
	}
	if work.CoverComicID != "" {
		fallbackComicID = work.CoverComicID
	}
	if fallbackComicID != "" {
		h.renderCover(c, fallbackComicID)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Work cover unavailable"})
}

func (h *OPDSHandler) findIndexedDownloadableWork(c *gin.Context, workID string) (opdsWorkCatalogItem, bool, error) {
	libraryIDs, err := getOPDSDownloadableComicLibraryIDs(c)
	if err != nil {
		return opdsWorkCatalogItem{}, false, err
	}
	if len(libraryIDs) == 0 {
		return opdsWorkCatalogItem{}, false, nil
	}
	userID := getOPDSUserID(c)
	permissionScope := strings.Join(libraryIDs, "\x00")

	h.workIndexMu.Lock()
	defer h.workIndexMu.Unlock()
	if h.workIndexes == nil {
		h.workIndexes = make(map[string]opdsWorkIndexCacheEntry)
	}
	if cached, ok := h.workIndexes[userID]; ok &&
		cached.permissionScope == permissionScope &&
		time.Now().Before(cached.expiresAt) {
		item, found := cached.byID[workID]
		return item, found, nil
	}
	loader := h.workLoader
	if loader == nil {
		loader = loadOPDSWorks
	}
	items, err := loader(c)
	if err != nil {
		return opdsWorkCatalogItem{}, false, err
	}
	byID := make(map[string]opdsWorkCatalogItem, len(items))
	allowed := make(map[string]struct{}, len(libraryIDs))
	for _, libraryID := range libraryIDs {
		allowed[libraryID] = struct{}{}
	}
	for _, item := range items {
		if _, ok := allowed[item.Work.LibraryID]; ok {
			byID[item.Work.ID] = item
		}
	}
	h.workIndexes[userID] = opdsWorkIndexCacheEntry{
		permissionScope: permissionScope,
		expiresAt:       time.Now().Add(opdsWorkIndexTTL),
		byID:            byID,
	}
	item, found := byID[workID]
	return item, found, nil
}

func (h *OPDSHandler) renderSeriesWorkCover(c *gin.Context, seriesID, fallbackComicID string) {
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.SeriesCoverCacheName(seriesID))
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		h.renderOPDSImage(c, data, http.DetectContentType(data), 300)
		return
	}
	if fallbackComicID != "" {
		h.renderCover(c, fallbackComicID)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Work cover unavailable"})
}

func (h *OPDSHandler) renderOPDSImage(c *gin.Context, data []byte, mimeType string, maxAge int) {
	sum := sha256.Sum256(data)
	etag := fmt.Sprintf(`"%x"`, sum[:12])
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d, must-revalidate", maxAge))
	c.Header("Vary", "Authorization, Cookie")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, mimeType, data)
}

// GET /api/opds/unit-cover/:id?page={absolutePage}
func (h *OPDSHandler) UnitCover(c *gin.Context) {
	comic, ok := getOPDSPublication(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comic not found"})
		return
	}
	if err := checkComicDownloadAccess(c, comic.ID); err != nil {
		return
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 0 || page >= comic.PageCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page"})
		return
	}
	result, err := service.GetOPDSPSEPageImage(comic.ID, page, 0)
	if err != nil || result == nil || len(result.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Unit cover unavailable"})
		return
	}
	sum := sha256.Sum256(result.Data)
	etag := fmt.Sprintf(`"%x"`, sum[:12])
	c.Header("Cache-Control", "private, max-age=86400, must-revalidate")
	c.Header("Vary", "Authorization, Cookie")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "image/jpeg", result.Data)
}

func (h *OPDSHandler) renderCover(c *gin.Context, comicID string) {
	comic, ok := getOPDSPublication(comicID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comic not found"})
		return
	}
	if err := checkComicDownloadAccess(c, comic.ID); err != nil {
		return
	}
	thumbnail, mimeType, _, err := service.GetComicThumbnail(comic.ID)
	if err != nil || len(thumbnail) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail unavailable"})
		return
	}
	h.renderOPDSImage(c, thumbnail, mimeType, 300)
}

// GET/HEAD /api/opds/download/:id
func (h *OPDSHandler) Download(c *gin.Context) {
	comic, ok := getOPDSPublication(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comic not found"})
		return
	}
	if err := checkComicDownloadAccess(c, comic.ID); err != nil {
		return
	}

	resolved, err := service.GlobalFileResolver.ResolveContentPath(comic.ID)
	if err != nil || resolved.AbsolutePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	file, err := os.Open(resolved.AbsolutePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	contentType, _ := service.OPDSAcquisitionMIMEForFilename(comic.Filename)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(comic.Filename)})
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Content-Type-Options", "nosniff")
	setOPDSPrivateResponseHeaders(c)
	http.ServeContent(c.Writer, c.Request, filepath.Base(comic.Filename), info.ModTime(), file)
}

// GET /api/opds/stream/:id?page={pageNumber}&width={maxWidth}
func (h *OPDSHandler) StreamPage(c *gin.Context) {
	comic, ok := getOPDSPublication(c.Param("id"))
	if !ok || !service.OPDSPSESupported(comic.Filename, comic.ComicType, comic.PageCount) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comic page stream not found"})
		return
	}
	if err := checkComicDownloadAccess(c, comic.ID); err != nil {
		return
	}

	pageIndex, err := strconv.Atoi(c.Query("page"))
	if err != nil || pageIndex < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page"})
		return
	}
	startPage, pageCount, err := parseOPDSUnitPageWindow(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if pageCount > 0 && pageIndex >= pageCount {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	pageIndex += startPage
	if pageIndex >= comic.PageCount {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}

	maxWidth := 0
	if rawWidth := strings.TrimSpace(c.Query("width")); rawWidth != "" {
		maxWidth, err = strconv.Atoi(rawWidth)
		if err != nil || maxWidth < 0 || maxWidth > opdsPSEMaxWidth {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("width must be between 0 and %d", opdsPSEMaxWidth)})
			return
		}
	}

	result, err := service.GetOPDSPSEPageImage(comic.ID, pageIndex, maxWidth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render page"})
		return
	}
	if result == nil || len(result.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}

	sum := sha256.Sum256(result.Data)
	etag := fmt.Sprintf(`"%x"`, sum[:12])
	c.Header("Content-Type", "image/jpeg")
	c.Header("Content-Length", strconv.Itoa(len(result.Data)))
	c.Header("Cache-Control", "private, max-age=86400, must-revalidate")
	c.Header("Vary", "Authorization, Cookie")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "image/jpeg", result.Data)
}

func parseOPDSUnitPageWindow(c *gin.Context) (startPage, pageCount int, err error) {
	if raw := strings.TrimSpace(c.Query("startPage")); raw != "" {
		startPage, err = strconv.Atoi(raw)
		if err != nil || startPage < 0 {
			return 0, 0, fmt.Errorf("Invalid startPage")
		}
	}
	if raw := strings.TrimSpace(c.Query("pageCount")); raw != "" {
		pageCount, err = strconv.Atoi(raw)
		if err != nil || pageCount <= 0 {
			return 0, 0, fmt.Errorf("Invalid pageCount")
		}
	}
	return startPage, pageCount, nil
}

func setOPDSPrivateResponseHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Vary", "Authorization, Cookie")
}

func getOPDSPublication(comicID string) (*store.ComicListItem, bool) {
	comic, err := store.GetComicByID(comicID)
	if err != nil || comic == nil || comic.LibraryID == "" {
		return nil, false
	}
	_, canAcquire := service.OPDSAcquisitionMIMEForFilename(comic.Filename)
	canStream := service.OPDSPSESupported(comic.Filename, comic.ComicType, comic.PageCount)
	if !canAcquire && !canStream {
		return nil, false
	}
	library, err := store.GetLibraryByID(comic.LibraryID)
	if err != nil || library == nil || !library.Enabled || library.Type != "comic" {
		return nil, false
	}
	return comic, true
}

func getOPDSUserID(c *gin.Context) string {
	if value, exists := c.Get("auth_user"); exists {
		if user, ok := value.(*model.AuthUser); ok {
			return user.ID
		}
	}
	return ""
}

func getAccessibleOPDSSeries(c *gin.Context, seriesID string) (*store.OPDSSeriesRow, []string, bool) {
	if err := service.EnsureComicSeriesFresh(); err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to refresh comic series"))
		return nil, nil, false
	}
	libraryIDs, err := store.GetUserDownloadableLibraryIDs(getOPDSUserID(c))
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to resolve library access"))
		return nil, nil, false
	}
	rows, _, err := store.GetOPDSSeries(store.OPDSSeriesQueryOptions{
		LibraryIDs: libraryIDs,
		SeriesID:   seriesID,
		Limit:      1,
	})
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Failed to get comic series"))
		return nil, nil, false
	}
	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comic series not found"})
		return nil, nil, false
	}
	return &rows[0], libraryIDs, true
}

func toOPDSComics(rows []store.OPDSComicRow) []service.OPDSComic {
	comics := make([]service.OPDSComic, 0, len(rows))
	for _, row := range rows {
		comics = append(comics, service.OPDSComic{
			ID:           row.ID,
			Title:        row.Title,
			Author:       row.Author,
			Description:  row.Description,
			Language:     row.Language,
			Genre:        row.Genre,
			Publisher:    row.Publisher,
			Year:         row.Year,
			PageCount:    row.PageCount,
			FileSize:     row.FileSize,
			AddedAt:      row.AddedAt,
			UpdatedAt:    row.UpdatedAt,
			Tags:         row.Tags,
			Filename:     row.Filename,
			ComicType:    row.ComicType,
			SeriesID:     row.SeriesID,
			SeriesTitle:  row.SeriesTitle,
			LastReadPage: row.LastReadPage,
			LastReadAt:   row.LastReadAt,
		})
	}
	return comics
}

func comicTagNames(tags []store.ComicTagInfo) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func toOPDSSeriesComics(rows []store.OPDSComicRow) []service.OPDSComic {
	comics := toOPDSComics(rows)
	for i, row := range rows {
		title := strings.TrimSpace(row.DisplayLabel)
		if title == "" {
			title = strings.TrimSpace(row.Title)
		}
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(row.Filename), filepath.Ext(row.Filename))
		}
		if section := strings.TrimSpace(row.SectionTitle); section != "" {
			title = section + " - " + title
		}
		comics[i].Title = title
	}
	return comics
}

func toOPDSSeries(rows []store.OPDSSeriesRow) []service.OPDSSeries {
	series := make([]service.OPDSSeries, 0, len(rows))
	for _, row := range rows {
		series = append(series, service.OPDSSeries{
			ID:        row.ID,
			Title:     row.Title,
			ItemCount: row.ItemCount,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return series
}
