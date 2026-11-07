package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// ============================================================
// Komga REST compatibility layer
//
// Mihon / Tachiyomi's Komga extension (and other Komga clients) talk to the
// REST API documented at https://komga.org, not to OPDS. Mapping:
//   Komga Library -> Nowen Library
//   Komga Series  -> Nowen Work     (id: work_xxx)
//   Komga Book    -> Nowen WorkUnit (id: comic id, one physical volume)
// Page images reuse the same extraction + cache path as the web reader.
// ============================================================

const komgaDefaultPageSize = 20

type KomgaHandler struct {
	works *WorkHandler
}

func NewKomgaHandler() *KomgaHandler {
	return &KomgaHandler{works: NewWorkHandler()}
}

// ------------------------------------------------------------
// DTOs — field names/optionality match what the Mihon extension decodes.
// ------------------------------------------------------------

type komgaPageWrapper[T any] struct {
	Content          []T  `json:"content"`
	Empty            bool `json:"empty"`
	First            bool `json:"first"`
	Last             bool `json:"last"`
	Number           int  `json:"number"`
	NumberOfElements int  `json:"numberOfElements"`
	Size             int  `json:"size"`
	TotalElements    int  `json:"totalElements"`
	TotalPages       int  `json:"totalPages"`
}

type komgaLibrary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type komgaAuthor struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type komgaSeriesMetadata struct {
	Status             string        `json:"status"`
	Created            *string       `json:"created"`
	LastModified       *string       `json:"lastModified"`
	Title              string        `json:"title"`
	TitleSort          string        `json:"titleSort"`
	Summary            string        `json:"summary"`
	SummaryLock        bool          `json:"summaryLock"`
	ReadingDirection   string        `json:"readingDirection"`
	ReadingDirectionLock bool        `json:"readingDirectionLock"`
	Publisher          string        `json:"publisher"`
	PublisherLock      bool          `json:"publisherLock"`
	AgeRating          *int          `json:"ageRating"`
	AgeRatingLock      bool          `json:"ageRatingLock"`
	Language           string        `json:"language"`
	LanguageLock       bool          `json:"languageLock"`
	Genres             []string      `json:"genres"`
	GenresLock         bool          `json:"genresLock"`
	Tags               []string      `json:"tags"`
	TagsLock           bool          `json:"tagsLock"`
	TotalBookCount     *int          `json:"totalBookCount"`
}

type komgaBooksMetadata struct {
	Authors       []komgaAuthor `json:"authors"`
	Tags          []string      `json:"tags"`
	ReleaseDate   *string       `json:"releaseDate"`
	Summary       string        `json:"summary"`
	SummaryNumber string        `json:"summaryNumber"`
	Created       string        `json:"created"`
	LastModified  string        `json:"lastModified"`
}

type komgaSeries struct {
	ID             string             `json:"id"`
	LibraryID      string             `json:"libraryId"`
	Name           string             `json:"name"`
	URL            string             `json:"url"`
	Created        *string            `json:"created"`
	LastModified   *string            `json:"lastModified"`
	FileLastModified string           `json:"fileLastModified"`
	BooksCount     int                `json:"booksCount"`
	BooksReadCount int                `json:"booksReadCount"`
	BooksUnreadCount int              `json:"booksUnreadCount"`
	BooksInProgressCount int          `json:"booksInProgressCount"`
	Metadata       komgaSeriesMetadata `json:"metadata"`
	BooksMetadata  komgaBooksMetadata  `json:"booksMetadata"`
	Deleted        bool               `json:"deleted"`
}

type komgaMedia struct {
	Status               string `json:"status"`
	MediaType            string `json:"mediaType"`
	PagesCount           int    `json:"pagesCount"`
	MediaProfile         string `json:"mediaProfile"`
	EpubDivinaCompatible bool   `json:"epubDivinaCompatible"`
}

type komgaBookMetadata struct {
	Title         string        `json:"title"`
	TitleLock     bool          `json:"titleLock"`
	Summary       string        `json:"summary"`
	SummaryLock   bool          `json:"summaryLock"`
	Number        string        `json:"number"`
	NumberLock    bool          `json:"numberLock"`
	NumberSort    float64       `json:"numberSort"`
	NumberSortLock bool         `json:"numberSortLock"`
	ReleaseDate   *string       `json:"releaseDate"`
	ReleaseDateLock bool        `json:"releaseDateLock"`
	Authors       []komgaAuthor `json:"authors"`
	AuthorsLock   bool          `json:"authorsLock"`
	Tags          []string      `json:"tags"`
	TagsLock      bool          `json:"tagsLock"`
}

type komgaBook struct {
	ID               string            `json:"id"`
	SeriesID         string            `json:"seriesId"`
	SeriesTitle      string            `json:"seriesTitle"`
	Name             string            `json:"name"`
	URL              string            `json:"url"`
	Number           int               `json:"number"`
	Created          *string           `json:"created"`
	LastModified     *string           `json:"lastModified"`
	FileLastModified string            `json:"fileLastModified"`
	SizeBytes        int64             `json:"sizeBytes"`
	Size             string            `json:"size"`
	Media            komgaMedia        `json:"media"`
	Metadata         komgaBookMetadata `json:"metadata"`
	Deleted          bool              `json:"deleted"`
}

type komgaPage struct {
	Number    int    `json:"number"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
}

// ------------------------------------------------------------
// Entry points
// ------------------------------------------------------------

// GET /api/v1/libraries
func (h *KomgaHandler) Libraries(c *gin.Context) {
	libraries, err := store.GetAccessibleLibrariesWithCount(getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch libraries"})
		return
	}
	result := make([]komgaLibrary, 0, len(libraries))
	for _, library := range libraries {
		if library.Type != "" && library.Type != "comic" && library.Type != "mixed" {
			continue
		}
		result = append(result, komgaLibrary{ID: library.ID, Name: library.Name})
	}
	c.JSON(http.StatusOK, result)
}

// GET /api/v1/series?search=&page=&size=&library_id=&sort=&read_status=
func (h *KomgaHandler) Series(c *gin.Context) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return
	}
	filtered := filterKomgaSeries(works, c)
	userID := getUserID(c)
	series := make([]komgaSeries, 0, len(filtered))
	for _, work := range filtered {
		series = append(series, komgaSeriesFromWork(work, userID))
	}
	page, pageSize := parseKomgaPagination(c)
	c.JSON(http.StatusOK, paginateKomga(series, page, pageSize))
}

// GET /api/v1/series/:id
func (h *KomgaHandler) SeriesDetail(c *gin.Context) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	for index := range works {
		if works[index].ID == id {
			c.JSON(http.StatusOK, komgaSeriesFromWork(works[index], getUserID(c)))
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Series not found"})
}

// GET /api/v1/series/:id/books
func (h *KomgaHandler) SeriesBooks(c *gin.Context) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	for index := range works {
		if works[index].ID != id {
			continue
		}
		books := komgaBooksFromWork(works[index], getUserID(c))
		if c.Query("unpaged") == "true" {
			c.JSON(http.StatusOK, komgaPageWrapperAll(books))
			return
		}
		page, pageSize := parseKomgaPagination(c)
		c.JSON(http.StatusOK, paginateKomga(books, page, pageSize))
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Series not found"})
}

// GET /api/v1/series/:id/thumbnail
func (h *KomgaHandler) SeriesThumbnail(c *gin.Context) {
	work, ok := h.findWork(c, c.Param("id"))
	if !ok {
		return
	}
	h.serveWorkCover(c, work)
}

// GET /api/v1/series/:id/read-progress
func (h *KomgaHandler) SeriesReadProgress(c *gin.Context) {
	work, ok := h.findWork(c, c.Param("id"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, h.seriesReadProgressMap(work, getUserID(c)))
}

// POST /api/v1/series/:id/read-progress
func (h *KomgaHandler) SeriesReadProgressUpdate(c *gin.Context) {
	work, ok := h.findWork(c, c.Param("id"))
	if !ok {
		return
	}
	var body struct {
		Books []struct {
			BookID string `json:"bookId"`
		} `json:"books"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Books) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "books is required"})
		return
	}
	unitByComic := make(map[string]service.WorkUnit, len(work.Units))
	for _, unit := range work.Units {
		unitByComic[komgaBookID(work, unit)] = unit
	}
	for _, entry := range body.Books {
		unit, exists := unitByComic[entry.BookID]
		if !exists {
			continue
		}
		if err := h.markUnitRead(c, work, unit); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// GET /api/v1/books/:id
func (h *KomgaHandler) BookDetail(c *gin.Context) {
	work, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return // findWorkAndUnit already wrote the 404
	}
	c.JSON(http.StatusOK, komgaBookFromUnit(work, unit, getUserID(c)))
}

// GET /api/v1/books?unpaged=true (used when the extension searches "Books")
func (h *KomgaHandler) Books(c *gin.Context) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return
	}
	query := strings.ToLower(strings.TrimSpace(c.Query("search")))
	books := make([]komgaBook, 0, len(works))
	for _, work := range works {
		for _, unit := range work.Units {
			if query != "" && !strings.Contains(strings.ToLower(work.Title+" "+unit.Title), query) {
				continue
			}
			books = append(books, komgaBookFromUnit(work, unit, getUserID(c)))
		}
	}
	if c.Query("unpaged") == "true" {
		c.JSON(http.StatusOK, komgaPageWrapperAll(books))
		return
	}
	page, pageSize := parseKomgaPagination(c)
	c.JSON(http.StatusOK, paginateKomga(books, page, pageSize))
}

// GET /api/v1/books/:id/thumbnail
func (h *KomgaHandler) BookThumbnail(c *gin.Context) {
	_, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	h.serveUnitCover(c, unit)
}

// GET /api/v1/books/:id/pages
func (h *KomgaHandler) BookPages(c *gin.Context) {
	_, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	pages, err := service.GetComicPagesEx(unit.ComicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read book pages"})
		return
	}
	// A Book is one Unit, so the page list is the Unit's slice of the archive.
	start, count := komgaUnitPageWindow(unit, len(pages.Entries))
	result := make([]komgaPage, 0, count)
	for offset := 0; offset < count; offset++ {
		result = append(result, komgaPage{
			Number:    offset + 1,
			FileName:  filepath.Base(pages.Entries[start+offset]),
			MediaType: komgaMediaTypeForName(pages.Entries[start+offset]),
		})
	}
	c.JSON(http.StatusOK, result)
}

// GET /api/v1/books/:id/pages/:pageNumber  (1-based like Komga)
func (h *KomgaHandler) BookPageImage(c *gin.Context) {
	_, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	number, err := strconv.Atoi(c.Param("pageNumber"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	// Komga pages are 1-based; `zero_based=true` shifts the index down by one.
	index := number - 1
	if c.Query("zero_based") == "true" {
		index = number
	}
	if index < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	start, count := komgaUnitPageWindow(unit, 0)
	if count > 0 && index >= count {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	index += start
	// The Mihon extension appends `?convert=png` when a page's declared format is
	// outside its decoder whitelist. We answer with the normalized JPEG rendition
	// (always decodable) instead of re-encoding to PNG, which buys nothing here.
	if convert := strings.ToLower(c.Query("convert")); convert == "png" {
		image, err := service.GetOPDSPSEPageImage(unit.ComicID, index, 0)
		if err != nil || image == nil || len(image.Data) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
			return
		}
		h.renderImage(c, image.Data, "image/jpeg", 86400)
		return
	}
	result, err := service.GetPageFile(unit.ComicID, index)
	if err != nil || result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	file, err := os.Open(result.Path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	defer file.Close()
	c.Header("Content-Type", result.MimeType)
	c.Header("Cache-Control", "private, max-age=86400, must-revalidate")
	c.Header("ETag", result.ETag)
	http.ServeContent(c.Writer, c.Request, filepath.Base(result.Path), result.ModTime, file)
}

// GET /api/v1/books/:id/pages/:pageNumber/thumbnail — Komga resizes to 300px.
func (h *KomgaHandler) BookPageThumbnail(c *gin.Context) {
	_, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	number, err := strconv.Atoi(c.Param("pageNumber"))
	if err != nil || number < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	start, _ := komgaUnitPageWindow(unit, 0)
	image, pageErr := service.GetOPDSPSEPageImage(unit.ComicID, start+number-1, 300)
	if pageErr != nil || image == nil || len(image.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page thumbnail not found"})
		return
	}
	h.renderImage(c, image.Data, "image/jpeg", 300)
}

// GET /api/v1/books/:id/file — original archive download.
func (h *KomgaHandler) BookFile(c *gin.Context) {
	_, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	h.serveUnitFile(c, unit)
}

// GET|PATCH|DELETE /api/v1/books/:id/read-progress
func (h *KomgaHandler) BookReadProgress(c *gin.Context) {
	work, unit, ok := h.findWorkAndUnit(c, c.Param("id"))
	if !ok {
		return
	}
	userID := getUserID(c)
	switch c.Request.Method {
	case http.MethodGet:
		c.JSON(http.StatusOK, h.unitReadProgress(work, unit, userID))
	case http.MethodPatch:
		var body struct {
			Page      *int  `json:"page"`
			Completed *bool `json:"completed"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if body.Completed != nil && *body.Completed {
			if err := h.markUnitRead(c, work, unit); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
				return
			}
			c.Status(http.StatusNoContent)
			return
		}
		if body.Page == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page is required"})
			return
		}
		page := *body.Page
		if page < 1 || (unit.PageCount > 0 && page > unit.PageCount) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page is outside the book"})
			return
		}
		if err := store.UpdateReadingProgress(unit.ComicID, unit.StartPage+page-1, 0, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
			return
		}
		if err := h.persistWorkCursor(work, unit, page-1, userID); err != nil {
			// Comic progress already saved; the Work cursor is best-effort.
			log.Printf("[komga] work cursor update failed for %s: %v", work.ID, err)
		}
		c.Status(http.StatusNoContent)
	case http.MethodDelete:
		if err := h.clearUnitProgress(work, unit, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear progress"})
			return
		}
		c.Status(http.StatusNoContent)
	default:
		c.Status(http.StatusMethodNotAllowed)
	}
}

// GET /api/v1/collections — no collections; the extension shows an empty page.
func (h *KomgaHandler) Collections(c *gin.Context) {
	c.JSON(http.StatusOK, komgaPageWrapperAll([]komgaLibrary{}))
}

// GET /api/v1/genres|tags|publishers|authors
func (h *KomgaHandler) Genres(c *gin.Context)  { h.taxonomy(c, "genre") }
func (h *KomgaHandler) Tags(c *gin.Context)    { h.taxonomy(c, "tag") }
func (h *KomgaHandler) Publishers(c *gin.Context) { h.taxonomy(c, "publisher") }
func (h *KomgaHandler) Authors(c *gin.Context) { h.taxonomy(c, "author") }

func (h *KomgaHandler) taxonomy(c *gin.Context, kind string) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	authors := make([]komgaAuthor, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, work := range works {
		switch kind {
		case "genre":
			for _, part := range strings.Split(work.Genre, ",") {
				add(part)
			}
		case "tag":
			for _, tag := range work.Tags {
				add(tag.Name)
			}
		case "publisher":
			add(work.Publisher)
		case "author":
			for _, name := range splitKomgaAuthorNames(work.Author) {
				key := name + "\x00writer"
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				authors = append(authors, komgaAuthor{Name: name, Role: "writer"})
			}
		}
	}
	sort.Strings(result)
	if kind == "author" {
		c.JSON(http.StatusOK, authors)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ------------------------------------------------------------
// Catalog / mapping helpers
// ------------------------------------------------------------

func (h *KomgaHandler) loadCatalog(c *gin.Context) ([]service.Work, bool) {
	works, err := h.works.loadWorks(c, false)
	if err != nil {
		log.Printf("[komga] catalog load failed: %v", err)
		if store.IsSQLiteBusyError(err) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is busy"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch series"})
		}
		return nil, false
	}
	// Komga clients only understand comics.
	filtered := make([]service.Work, 0, len(works))
	for _, work := range works {
		if len(work.Units) == 0 {
			continue
		}
		filtered = append(filtered, work)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left := strings.ToLower(filtered[i].Title)
		right := strings.ToLower(filtered[j].Title)
		if left == right {
			return filtered[i].ID < filtered[j].ID
		}
		return left < right
	})
	return filtered, true
}

func (h *KomgaHandler) findWork(c *gin.Context, workID string) (service.Work, bool) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return service.Work{}, false
	}
	for _, work := range works {
		if work.ID == workID {
			return work, true
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Series not found"})
	return service.Work{}, false
}

// findWorkAndUnit resolves a Book id. A Book is one Unit (a volume for Japanese
// manga, a single chapter for webtoon-style archives packed into one file), so
// the unit id is the canonical Book id. A bare comic id still resolves when the
// Work has exactly one unit, which keeps older clients working.
func (h *KomgaHandler) findWorkAndUnit(c *gin.Context, bookID string) (service.Work, service.WorkUnit, bool) {
	works, ok := h.loadCatalog(c)
	if !ok {
		return service.Work{}, service.WorkUnit{}, false
	}
	for _, work := range works {
		for _, unit := range work.Units {
			if unit.ID == bookID {
				return work, unit, true
			}
		}
		for _, unit := range work.Units {
			if unit.ComicID == bookID && len(work.Units) == 1 {
				return work, unit, true
			}
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
	return service.Work{}, service.WorkUnit{}, false
}

// komgaBookID is the id a Komga client uses for one readable Unit. Single-unit
// works keep the physical comic id (stable for existing clients); multi-unit
// works use the unit id so every chapter is addressable.
func komgaBookID(work service.Work, unit service.WorkUnit) string {
	if len(work.Units) == 1 {
		return unit.ComicID
	}
	return unit.ID
}

// komgaUnitPageWindow returns the absolute start offset and page count of a Unit
// inside its physical archive. total is the archive page count when known; 0
// means "trust the unit metadata".
func komgaUnitPageWindow(unit service.WorkUnit, total int) (start int, count int) {
	start = unit.StartPage
	if start < 0 {
		start = 0
	}
	count = unit.PageCount
	if count <= 0 {
		if total > start {
			count = total - start
		} else {
			count = maxInt(0, total)
		}
	}
	if total > 0 {
		if start >= total {
			start = 0
		}
		if start+count > total {
			count = total - start
		}
	}
	if count < 0 {
		count = 0
	}
	return start, count
}

func komgaSeriesFromWork(work service.Work, userID string) komgaSeries {
	created := normalizeKomgaTime(work.AddedAt)
	modified := normalizeKomgaTime(work.UpdatedAt)
	fileModified := modified
	if fileModified == "" {
		fileModified = created
	}
	if fileModified == "" {
		fileModified = komgaNow()
	}
	tags := make([]string, 0, len(work.Tags))
	for _, tag := range work.Tags {
		if strings.TrimSpace(tag.Name) != "" {
			tags = append(tags, tag.Name)
		}
	}
	genres := splitKomgaList(work.Genre)
	authors := make([]komgaAuthor, 0, 2)
	for _, name := range splitKomgaAuthorNames(work.Author) {
		authors = append(authors, komgaAuthor{Name: name, Role: "writer"})
	}
	read, inProgress := komgaReadCounts(work)
	totalBookCount := work.ItemCount
	return komgaSeries{
		ID:               work.ID,
		LibraryID:        work.LibraryID,
		Name:             work.Title,
		URL:              "/api/v1/series/" + work.ID,
		Created:          optionalKomgaTime(created),
		LastModified:     optionalKomgaTime(modified),
		FileLastModified: fileModified,
		BooksCount:       work.ItemCount,
		BooksReadCount:   read,
		BooksUnreadCount: maxInt(0, work.ItemCount-read-inProgress),
		BooksInProgressCount: inProgress,
		Metadata: komgaSeriesMetadata{
			Status:             komgaSeriesStatus(work.Status, work.ReadingStatus),
			Created:            optionalKomgaTime(created),
			LastModified:       optionalKomgaTime(modified),
			Title:              work.Title,
			TitleSort:          work.Title,
			Summary:            work.Description,
			ReadingDirection:   "LEFT_TO_RIGHT",
			Publisher:          work.Publisher,
			Language:           komgaLanguage(work.Language),
			Genres:             genres,
			Tags:               tags,
			TotalBookCount:     &totalBookCount,
		},
		BooksMetadata: komgaBooksMetadata{
			Authors:       authors,
			Tags:          tags,
			Summary:       work.Description,
			SummaryNumber: "",
			Created:       created,
			LastModified:  modified,
		},
	}
}

func komgaReadCounts(work service.Work) (read int, inProgress int) {
	for _, unit := range work.Units {
		switch unit.ReadingStatus {
		case "finished":
			read++
		case "reading":
			inProgress++
		}
	}
	return read, inProgress
}

func komgaBooksFromWork(work service.Work, userID string) []komgaBook {
	books := make([]komgaBook, 0, len(work.Units))
	for _, unit := range work.Units {
		books = append(books, komgaBookFromUnit(work, unit, userID))
	}
	return books
}

func komgaBookFromUnit(work service.Work, unit service.WorkUnit, userID string) komgaBook {
	number := unit.SortIndex + 1
	created := normalizeKomgaTime(work.AddedAt)
	modified := normalizeKomgaTime(work.UpdatedAt)
	fileModified := modified
	if fileModified == "" {
		fileModified = komgaNow()
	}
	tags := make([]string, 0, len(work.Tags))
	for _, tag := range work.Tags {
		if strings.TrimSpace(tag.Name) != "" {
			tags = append(tags, tag.Name)
		}
	}
	authors := make([]komgaAuthor, 0, 2)
	for _, name := range splitKomgaAuthorNames(work.Author) {
		authors = append(authors, komgaAuthor{Name: name, Role: "writer"})
	}
	pageCount := unit.PageCount
	if pageCount <= 0 {
		pageCount = work.PageCount
	}
	name := unit.DisplayLabel
	if strings.TrimSpace(name) == "" {
		name = unit.Title
	}
	bookID := komgaBookID(work, unit)
	return komgaBook{
		ID:               bookID,
		SeriesID:         work.ID,
		SeriesTitle:      work.Title,
		Name:             name,
		URL:              "/api/v1/books/" + bookID,
		Number:           number,
		Created:          optionalKomgaTime(created),
		LastModified:     optionalKomgaTime(modified),
		FileLastModified: fileModified,
		SizeBytes:        work.FileSize,
		Size:             komgaHumanSize(work.FileSize),
		Media: komgaMedia{
			Status:               "READY",
			MediaType:            "application/zip",
			PagesCount:           pageCount,
			MediaProfile:         "DIVINA",
			EpubDivinaCompatible: false,
		},
		Metadata: komgaBookMetadata{
			Title:        name,
			Summary:      work.Description,
			Number:       strconv.Itoa(number),
			NumberSort:   float64(number),
			Authors:      authors,
			Tags:         tags,
		},
	}
}

func (h *KomgaHandler) seriesReadProgressMap(work service.Work, userID string) map[string]any {
	progress := make(map[string]any, len(work.Units))
	for _, unit := range work.Units {
		progress[komgaBookID(work, unit)] = h.unitReadProgress(work, unit, userID)
	}
	return progress
}

func (h *KomgaHandler) unitReadProgress(work service.Work, unit service.WorkUnit, userID string) gin.H {
	page := unit.LastReadPage
	completed := unit.ReadingStatus == "finished"
	if cursor, err := store.GetUserWorkProgress(userID, work.ID); err == nil && cursor != nil && cursor.ComicID == unit.ComicID {
		page = cursor.RelativePage
	}
	if unit.PageCount > 0 && page >= unit.PageCount {
		completed = true
	}
	updated := komgaNow()
	if unit.LastReadAt != nil {
		if normalized := normalizeKomgaTime(*unit.LastReadAt); normalized != "" {
			updated = normalized
		}
	}
	return gin.H{
		"page":      page + 1, // Komga pages are 1-based
		"completed": completed,
		"readDate":  updated,
	}
}

func (h *KomgaHandler) markUnitRead(c *gin.Context, work service.Work, unit service.WorkUnit) error {
	userID := getUserID(c)
	lastPage := unit.PageCount
	if lastPage <= 0 {
		lastPage = 1
	}
	if err := store.UpdateReadingProgress(unit.ComicID, lastPage-1, 0, userID); err != nil {
		return err
	}
	if err := store.BatchSetReadingStatus(userID, []string{unit.ComicID}, "finished"); err != nil {
		return err
	}
	if err := h.persistWorkCursor(work, unit, lastPage-1, userID); err != nil {
		log.Printf("[komga] work cursor update failed for %s: %v", work.ID, err)
	}
	return nil
}

// persistWorkCursor keeps the Work-level "continue reading" cursor in sync so
// the web/Android clients resume where the Komga client stopped.
func (h *KomgaHandler) persistWorkCursor(work service.Work, unit service.WorkUnit, relativePage int, userID string) error {
	if userID == "" || work.ID == "" || unit.ComicID == "" {
		return nil
	}
	sessionID := fmt.Sprintf("komga-%s", userID)
	_, err := store.RecordReadingActivityWithWork(store.ReadingActivityInput{
		ComicID:         unit.ComicID,
		UserID:          userID,
		ClientSessionID: sessionID,
		Page:            unit.StartPage + relativePage,
		ActiveSeconds:   0,
		Sequence:        komgaCursorSequence(),
		TrackProgress:   true,
		WorkID:          work.ID,
		UnitID:          unit.ID,
		RelativePage:    relativePage,
		UnitStartPage:   unit.StartPage,
		UnitPageCount:   unit.PageCount,
	})
	if err != nil {
		return err
	}
	return nil
}

func (h *KomgaHandler) clearUnitProgress(work service.Work, unit service.WorkUnit, userID string) error {
	if err := store.BatchSetReadingStatus(userID, []string{unit.ComicID}, ""); err != nil {
		return err
	}
	cursor, err := store.GetUserWorkProgress(userID, work.ID)
	if err != nil || cursor == nil || cursor.ComicID != unit.ComicID {
		// Missing Work-cursor tables must not fail a read-progress reset.
		return nil
	}
	if err := store.ClearUserWorkProgress(userID, work.ID); err != nil {
		log.Printf("[komga] clear work cursor failed for %s: %v", work.ID, err)
	}
	return nil
}

// ------------------------------------------------------------
// Images / files
// ------------------------------------------------------------

func (h *KomgaHandler) serveWorkCover(c *gin.Context, work service.Work) {
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(work.ID))
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		mimeType := archive.ThumbnailMimeType(data)
		h.renderImage(c, data, mimeType, 300)
		return
	}
	if work.CoverComicID != "" {
		if thumbnail, mimeType, _, err := service.GetComicThumbnail(work.CoverComicID); err == nil && len(thumbnail) > 0 {
			h.renderImage(c, thumbnail, mimeType, 300)
			return
		}
	}
	if strings.HasPrefix(work.CoverURL, "http://") || strings.HasPrefix(work.CoverURL, "https://") {
		go service.DownloadWorkCover(work.ID, work.CoverURL)
		c.Redirect(http.StatusTemporaryRedirect, work.CoverURL)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Cover not found"})
}

func (h *KomgaHandler) serveUnitCover(c *gin.Context, unit service.WorkUnit) {
	if unit.CoverURL != "" && strings.HasPrefix(unit.CoverURL, "http") {
		c.Redirect(http.StatusTemporaryRedirect, unit.CoverURL)
		return
	}
	start, _ := komgaUnitPageWindow(unit, 0)
	image, err := service.GetOPDSPSEPageImage(unit.ComicID, start+unit.CoverPage, 400)
	if err == nil && image != nil && len(image.Data) > 0 {
		h.renderImage(c, image.Data, "image/jpeg", 300)
		return
	}
	if thumbnail, mimeType, _, thumbErr := service.GetComicThumbnail(unit.ComicID); thumbErr == nil && len(thumbnail) > 0 {
		h.renderImage(c, thumbnail, mimeType, 300)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Cover not found"})
}

func (h *KomgaHandler) serveUnitFile(c *gin.Context, unit service.WorkUnit) {
	resolved, err := service.GlobalFileResolver.ResolveContentPath(unit.ComicID)
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
	contentType, _ := service.OPDSAcquisitionMIMEForFilename(filepath.Base(resolved.AbsolutePath))
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	http.ServeContent(c.Writer, c.Request, filepath.Base(resolved.AbsolutePath), info.ModTime(), file)
}

func (h *KomgaHandler) renderImage(c *gin.Context, data []byte, mimeType string, maxAge int) {
	etag := fmt.Sprintf(`"%x"`, komgaHash(data))
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d, must-revalidate", maxAge))
	c.Header("Vary", "Authorization, Cookie, X-API-Key")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, mimeType, data)
}

// ------------------------------------------------------------
// Query / formatting helpers
// ------------------------------------------------------------

func filterKomgaSeries(works []service.Work, c *gin.Context) []service.Work {
	query := strings.ToLower(strings.TrimSpace(c.Query("search")))
	libraryIDs := splitKomgaList(c.Query("library_id"))
	librarySet := make(map[string]struct{}, len(libraryIDs))
	for _, id := range libraryIDs {
		librarySet[id] = struct{}{}
	}
	readStatuses := map[string]struct{}{}
	for _, value := range c.QueryArray("read_status") {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.ToUpper(strings.TrimSpace(part))
			if trimmed != "" {
				readStatuses[trimmed] = struct{}{}
			}
		}
	}

	result := make([]service.Work, 0, len(works))
	for _, work := range works {
		if len(librarySet) > 0 {
			if _, ok := librarySet[work.LibraryID]; !ok {
				continue
			}
		}
		if query != "" {
			haystack := strings.ToLower(work.Title + " " + work.Author + " " + work.Description)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		if len(readStatuses) > 0 {
			read, inProgress := komgaReadCounts(work)
			unread := work.ItemCount - read - inProgress
			matched := false
			for status := range readStatuses {
				switch status {
				case "READ":
					matched = matched || read > 0
				case "IN_PROGRESS":
					matched = matched || inProgress > 0
				case "UNREAD":
					matched = matched || unread > 0
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, work)
	}

	applyKomgaSort(result, c.Query("sort"))
	return result
}

func applyKomgaSort(works []service.Work, sortParam string) {
	if sortParam == "" {
		return
	}
	field := sortParam
	direction := "asc"
	if index := strings.LastIndex(sortParam, ","); index >= 0 {
		field = sortParam[:index]
		direction = strings.ToLower(strings.TrimSpace(sortParam[index+1:]))
	}
	descending := direction == "desc"
	switch field {
	case "createdDate":
		sort.SliceStable(works, func(i, j int) bool {
			if descending {
				return works[i].AddedAt > works[j].AddedAt
			}
			return works[i].AddedAt < works[j].AddedAt
		})
	case "lastModifiedDate":
		sort.SliceStable(works, func(i, j int) bool {
			if descending {
				return works[i].UpdatedAt > works[j].UpdatedAt
			}
			return works[i].UpdatedAt < works[j].UpdatedAt
		})
	case "metadata.titleSort", "title":
		sort.SliceStable(works, func(i, j int) bool {
			left := strings.ToLower(works[i].Title)
			right := strings.ToLower(works[j].Title)
			if descending {
				return left > right
			}
			return left < right
		})
	case "random":
		// Stable pseudo-random order; good enough for a browse feed.
		sort.SliceStable(works, func(i, j int) bool {
			return works[i].ID < works[j].ID
		})
	}
}

func parseKomgaPagination(c *gin.Context) (page int, pageSize int) {
	page = 0
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			page = parsed
		}
	}
	pageSize = komgaDefaultPageSize
	if raw := c.Query("size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			pageSize = parsed
		}
	}
	return page, pageSize
}

func paginateKomga[T any](items []T, page int, pageSize int) komgaPageWrapper[T] {
	total := len(items)
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := page * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	content := make([]T, 0, end-start)
	content = append(content, items[start:end]...)
	return komgaPageWrapper[T]{
		Content:          content,
		Empty:            len(content) == 0,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Number:           page,
		NumberOfElements: len(content),
		Size:             pageSize,
		TotalElements:    total,
		TotalPages:       totalPages,
	}
}

func komgaPageWrapperAll[T any](items []T) komgaPageWrapper[T] {
	return komgaPageWrapper[T]{
		Content:          items,
		Empty:            len(items) == 0,
		First:            true,
		Last:             true,
		Number:           0,
		NumberOfElements: len(items),
		Size:             len(items),
		TotalElements:    len(items),
		TotalPages:       1,
	}
}

func splitKomgaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitKomgaAuthorNames(value string) []string {
	replaced := strings.NewReplacer("、", ",", "，", ",", "/", ",", "&", ",", "・", ",")
	parts := strings.Split(replaced.Replace(value), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func komgaSeriesStatus(status, readingStatus string) string {
	value := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(value, "完结") || strings.Contains(value, "完結") || value == "completed" || value == "complete" || value == "ended":
		return "ENDED"
	case strings.Contains(value, "连载") || strings.Contains(value, "連載") || value == "ongoing" || value == "publishing":
		return "ONGOING"
	case strings.Contains(value, "休载") || value == "hiatus":
		return "HIATUS"
	case strings.Contains(value, "弃") || value == "abandoned" || value == "cancelled" || value == "canceled":
		return "ABANDONED"
	}
	if readingStatus == "finished" {
		return "ENDED"
	}
	return "ONGOING"
}

func komgaLanguage(language string) string {
	value := strings.ToLower(strings.TrimSpace(language))
	switch value {
	case "", "unknown":
		return "zh"
	case "zh", "cn", "zh-cn", "chs", "chinese", "中文", "简体中文", "日文", "日本語", "ja", "en", "english":
		switch value {
		case "ja", "日文", "日本語":
			return "ja"
		case "en", "english":
			return "en"
		}
		return "zh"
	}
	return value
}

func komgaMediaTypeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".jxl":
		return "image/jxl"
	case ".heif", ".heic":
		return "image/heif"
	case ".bmp":
		return "image/bmp"
	default:
		// Not in the Mihon extension's decoder whitelist, so it asks for a
		// converted rendition and we answer with a normalized JPEG.
		return "application/octet-stream"
	}
}

func komgaHumanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for value := size / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func normalizeKomgaTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return value
}

func optionalKomgaTime(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func komgaNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func komgaHash(data []byte) []byte {
	// Cheap non-cryptographic fingerprint is enough for an ETag.
	var hash uint64 = 1469598103934665603
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	return []byte(strconv.FormatUint(hash, 36))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var komgaCursorSeq int64

func komgaCursorSequence() int {
	komgaCursorSeq++
	return int(komgaCursorSeq % 1_000_000_000)
}
