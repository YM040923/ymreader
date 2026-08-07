package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type workWriteTarget struct {
	Work      service.Work
	ComicIDs  []string
	SeriesIDs []string
}

func (h *WorkHandler) SetFavorite(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		IsFavorite bool `json:"isFavorite"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := store.SetWorkFavorite(getUserID(c), targets[0].ComicIDs, body.IsFavorite); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "isFavorite": body.IsFavorite})
}

func (h *WorkHandler) SetRating(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		Rating *int `json:"rating"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if body.Rating != nil && (*body.Rating < 1 || *body.Rating > 5) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be between 1 and 5"})
		return
	}
	for _, comicID := range targets[0].ComicIDs {
		if err := store.UpdateRating(comicID, body.Rating, getUserID(c)); err != nil {
			writeWorkMutationError(c, err)
			return
		}
	}
	completeWorkMutation(c, gin.H{"success": true, "rating": body.Rating})
}

func (h *WorkHandler) SetReadingStatus(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := store.SetWorkReadingStatus(getUserID(c), targets[0].ComicIDs, body.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "readingStatus": normalizedWorkReadingStatus(body.Status)})
}

func (h *WorkHandler) SetTags(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		Tags []string `json:"tags"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	target := targets[0]
	if err := store.ReplaceLogicalWorkAndComicTags(target.Work.ID, target.ComicIDs, body.Tags); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "tags": body.Tags})
}

func (h *WorkHandler) SetCategories(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		CategorySlugs []string `json:"categorySlugs"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := store.ReplaceLogicalWorkAndComicCategories(targets[0].Work.ID, targets[0].ComicIDs, body.CategorySlugs); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "categorySlugs": body.CategorySlugs})
}

func (h *WorkHandler) UpdateMetadata(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		Title       *string `json:"title"`
		Author      *string `json:"author"`
		Publisher   *string `json:"publisher"`
		Year        *int    `json:"year"`
		Description *string `json:"description"`
		Language    *string `json:"language"`
		Genre       *string `json:"genre"`
		Status      *string `json:"status"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	target := targets[0].Work
	if err := store.UpdateWorkMetadata(target.MetadataHostType, target.MetadataHostID, store.WorkMetadataUpdate{
		Title: body.Title, Author: body.Author, Publisher: body.Publisher, Year: body.Year,
		Description: body.Description, Language: body.Language, Genre: body.Genre, Status: body.Status,
	}); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true})
}

func (h *WorkHandler) UpdateCover(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		URL              string  `json:"url"`
		CoverComicID     string  `json:"coverComicId"`
		CoverAspectRatio float64 `json:"coverAspectRatio"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	target := targets[0].Work
	if body.CoverComicID == "" && strings.TrimSpace(body.URL) == "" {
		body.CoverComicID = target.CoverComicID
	}
	if err := store.UpdateWorkCover(target.MetadataHostType, target.MetadataHostID, body.CoverComicID, strings.TrimSpace(body.URL), body.CoverAspectRatio); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true})
}

func (h *WorkHandler) Delete(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	deleted, err := store.DeleteWorkComics(targets[0].ComicIDs)
	if err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "deleted": deleted})
}

func (h *WorkHandler) Batch(c *gin.Context) {
	var body struct {
		WorkIDs       []string `json:"workIds"`
		ComicIDs      []string `json:"comicIds"`
		Action        string   `json:"action"`
		IsFavorite    bool     `json:"isFavorite"`
		ReadingStatus string   `json:"readingStatus"`
		Tags          []string `json:"tags"`
		CategorySlugs []string `json:"categorySlugs"`
	}
	if c.ShouldBindJSON(&body) != nil || (len(body.WorkIDs) == 0 && len(body.ComicIDs) == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workIds and action are required"})
		return
	}
	if len(body.WorkIDs) == 0 {
		body.WorkIDs = h.workIDsForComicIDs(c, body.ComicIDs)
	}
	if len(body.WorkIDs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No accessible works found"})
		return
	}
	targets, ok := h.requireManageTargets(c, body.WorkIDs)
	if !ok {
		return
	}
	comicIDs, _ := flattenWorkWriteTargets(targets)
	targetWorkIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		targetWorkIDs = append(targetWorkIDs, target.Work.ID)
	}
	var err error
	switch body.Action {
	case "favorite":
		err = store.SetWorkFavorite(getUserID(c), comicIDs, true)
	case "unfavorite":
		err = store.SetWorkFavorite(getUserID(c), comicIDs, false)
	case "setReadingStatus":
		err = store.SetWorkReadingStatus(getUserID(c), comicIDs, body.ReadingStatus)
	case "addTags":
		err = store.AddLogicalWorkAndComicTags(targetWorkIDs, comicIDs, body.Tags)
	case "removeTags":
		err = store.RemoveLogicalWorkAndComicTags(targetWorkIDs, comicIDs, body.Tags)
	case "setCategory":
		for _, target := range targets {
			if err = store.ReplaceLogicalWorkAndComicCategories(target.Work.ID, target.ComicIDs, body.CategorySlugs); err != nil {
				break
			}
		}
	case "delete":
		_, err = store.DeleteWorkComics(comicIDs)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported work batch action"})
		return
	}
	if err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "affectedWorks": len(targets)})
}

func (h *WorkHandler) Reorder(c *gin.Context) {
	var body struct {
		Orders []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sortOrder"`
		} `json:"orders"`
	}
	if c.ShouldBindJSON(&body) != nil || len(body.Orders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orders are required"})
		return
	}
	workIDs := make([]string, 0, len(body.Orders))
	orderByID := make(map[string]int, len(body.Orders))
	for _, order := range body.Orders {
		workIDs = append(workIDs, order.ID)
		orderByID[order.ID] = order.SortOrder
	}
	catalog, err := h.loadWorkCatalog(c)
	if err != nil {
		writeWorkMutationError(c, err)
		return
	}
	for index, inputID := range workIDs {
		if catalog.ByID[inputID] != nil {
			continue
		}
		for _, work := range catalog.Works {
			if containsWorkComicID(work, inputID) {
				workIDs[index] = work.ID
				orderByID[work.ID] = orderByID[inputID]
				break
			}
		}
	}
	targets, ok := h.requireManageTargets(c, workIDs)
	if !ok {
		return
	}
	updates := make([]store.WorkOrderUpdate, 0, len(targets))
	for _, target := range targets {
		updates = append(updates, store.WorkOrderUpdate{
			WorkID: target.Work.ID, SortOrder: orderByID[target.Work.ID],
		})
	}
	if err := store.ReorderWorkHosts(updates); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"success": true, "updated": len(updates)})
}

func (h *WorkHandler) workIDsForComicIDs(c *gin.Context, comicIDs []string) []string {
	catalog, err := h.loadWorkCatalog(c)
	if err != nil {
		return nil
	}
	ids := []string{}
	for _, comicID := range comicIDs {
		for _, work := range catalog.Works {
			if containsWorkComicID(work, comicID) {
				ids = append(ids, work.ID)
				break
			}
		}
	}
	return uniqueStrings(ids)
}

func containsWorkComicID(work service.Work, comicID string) bool {
	for _, id := range service.PhysicalComicIDs(work) {
		if id == comicID {
			return true
		}
	}
	return false
}

func (h *WorkHandler) TagStats(c *gin.Context) {
	works, err := h.loadWorks(c, false)
	if err != nil {
		writeWorkMutationError(c, err)
		return
	}
	type item struct {
		Name  string `json:"name"`
		Color string `json:"color"`
		Count int    `json:"count"`
	}
	byName := map[string]*item{}
	for _, work := range works {
		for _, tag := range work.Tags {
			if byName[tag.Name] == nil {
				byName[tag.Name] = &item{Name: tag.Name, Color: tag.Color}
			}
			byName[tag.Name].Count++
		}
	}
	items := make([]item, 0, len(byName))
	for _, value := range byName {
		items = append(items, *value)
	}
	sort.Slice(items, func(i, j int) bool { return service.NaturalLess(items[i].Name, items[j].Name) })
	c.JSON(http.StatusOK, gin.H{"tags": items})
}

func (h *WorkHandler) CategoryStats(c *gin.Context) {
	works, err := h.loadWorks(c, false)
	if err != nil {
		writeWorkMutationError(c, err)
		return
	}
	type item struct {
		Name  string `json:"name"`
		Slug  string `json:"slug"`
		Icon  string `json:"icon"`
		Count int    `json:"count"`
	}
	bySlug := map[string]*item{}
	for _, work := range works {
		for _, category := range work.Categories {
			if bySlug[category.Slug] == nil {
				bySlug[category.Slug] = &item{Name: category.Name, Slug: category.Slug, Icon: category.Icon}
			}
			bySlug[category.Slug].Count++
		}
	}
	items := make([]item, 0, len(bySlug))
	for _, value := range bySlug {
		items = append(items, *value)
	}
	sort.Slice(items, func(i, j int) bool { return service.NaturalLess(items[i].Name, items[j].Name) })
	c.JSON(http.StatusOK, gin.H{"categories": items})
}

func (h *WorkHandler) requireManageTargets(c *gin.Context, workIDs []string) ([]workWriteTarget, bool) {
	catalog, err := h.loadWorkCatalog(c)
	if err != nil {
		writeWorkMutationError(c, err)
		return nil, false
	}
	// Write handlers may be the first caller after a fresh scan. Materialize
	// the detected Work rows here, while keeping all GET handlers read-only.
	if err := service.PersistAndApplyLogicalWorks(catalog.Works); err != nil {
		writeWorkMutationError(c, err)
		return nil, false
	}
	targets := make([]workWriteTarget, 0, len(workIDs))
	libraryIDs := map[string]struct{}{}
	seenWorks := map[string]struct{}{}
	for _, workID := range workIDs {
		if _, exists := seenWorks[workID]; exists {
			continue
		}
		seenWorks[workID] = struct{}{}
		work := catalog.ByID[workID]
		if work == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
			return nil, false
		}
		comicIDs := service.PhysicalComicIDs(*work)
		libraries, queryErr := store.GetComicsLibraryIDsByIDs(comicIDs)
		if queryErr != nil {
			writeWorkMutationError(c, queryErr)
			return nil, false
		}
		for _, libraryID := range libraries {
			libraryIDs[libraryID] = struct{}{}
		}
		if work.LibraryID != "" {
			libraryIDs[work.LibraryID] = struct{}{}
		}
		target := workWriteTarget{Work: *work, ComicIDs: comicIDs}
		if work.MetadataHostType == "series" && work.MetadataHostID != "" {
			target.SeriesIDs = []string{work.MetadataHostID}
		}
		targets = append(targets, target)
	}
	for libraryID := range libraryIDs {
		allowed, permissionErr := store.UserCanManageLibrary(getUserID(c), libraryID)
		if permissionErr != nil {
			writeWorkMutationError(c, permissionErr)
			return nil, false
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage one or more work libraries"})
			return nil, false
		}
	}
	return targets, true
}

func flattenWorkWriteTargets(targets []workWriteTarget) ([]string, []string) {
	comicSet, seriesSet := map[string]struct{}{}, map[string]struct{}{}
	for _, target := range targets {
		for _, id := range target.ComicIDs {
			comicSet[id] = struct{}{}
		}
		for _, id := range target.SeriesIDs {
			seriesSet[id] = struct{}{}
		}
	}
	comicIDs, seriesIDs := make([]string, 0, len(comicSet)), make([]string, 0, len(seriesSet))
	for id := range comicSet {
		comicIDs = append(comicIDs, id)
	}
	for id := range seriesSet {
		seriesIDs = append(seriesIDs, id)
	}
	sort.Strings(comicIDs)
	sort.Strings(seriesIDs)
	return comicIDs, seriesIDs
}

func completeWorkMutation(c *gin.Context, response gin.H) {
	resetWorkCatalogCache()
	c.JSON(http.StatusOK, response)
}

func writeWorkMutationError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func normalizedWorkReadingStatus(status string) string {
	if status == "" || status == "unread" {
		return "unread"
	}
	return status
}
