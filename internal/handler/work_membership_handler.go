package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// Merge combines two or more Works into the first one by persisting manual
// physical-unit membership.
func (h *WorkHandler) Merge(c *gin.Context) {
	var req struct {
		WorkIDs []string `json:"workIds"`
		Title   string   `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	workIDs := uniqueStrings(req.WorkIDs)
	if len(workIDs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least two works are required"})
		return
	}
	targets, ok := h.requireManageTargets(c, workIDs)
	if !ok {
		return
	}
	libraryID := targets[0].Work.LibraryID
	for _, target := range targets[1:] {
		if target.Work.LibraryID != libraryID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only works in the same library can be merged"})
			return
		}
	}
	target := targets[0].Work
	if err := store.EnsureLogicalWorkRecord(store.LogicalWork{
		ID:               target.ID,
		LibraryID:        target.LibraryID,
		RootPath:         target.RootPath,
		Title:            target.Title,
		CoverComicID:     target.CoverComicID,
		CoverAspectRatio: target.CoverAspectRatio,
	}); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	entries := make([]store.WorkMembershipEntry, 0)
	for _, work := range targets {
		for _, comicID := range work.ComicIDs {
			entries = append(entries, store.WorkMembershipEntry{
				ComicID: comicID, WorkID: target.ID, LibraryID: libraryID,
			})
		}
	}
	if err := store.SetWorkMemberships(entries); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		if err := store.UpdateLogicalWorkMetadata(target.ID, store.LogicalWorkMetadataUpdate{Title: &title}); err != nil {
			writeWorkMutationError(c, err)
			return
		}
	}
	completeWorkMutation(c, gin.H{"workId": target.ID, "units": len(entries)})
}

// Split moves selected whole units out of a Work into a new manual Work.
func (h *WorkHandler) Split(c *gin.Context) {
	workID := strings.TrimSpace(c.Param("id"))
	if workID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Work ID is required"})
		return
	}
	var req struct {
		UnitIDs []string `json:"unitIds"`
		Title   string   `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	unitIDs := uniqueStrings(req.UnitIDs)
	if len(unitIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one unit is required"})
		return
	}
	targets, ok := h.requireManageTargets(c, []string{workID})
	if !ok {
		return
	}
	work := targets[0].Work
	unitsByID := make(map[string]int, len(work.Units))
	for index, unit := range work.Units {
		unitsByID[unit.ID] = index
	}
	selected := make(map[string]struct{}, len(unitIDs))
	comicIDs := make([]string, 0, len(unitIDs))
	comicSet := make(map[string]struct{}, len(unitIDs))
	for _, unitID := range unitIDs {
		index, exists := unitsByID[unitID]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unit does not belong to this work"})
			return
		}
		selected[unitID] = struct{}{}
		comicID := work.Units[index].ComicID
		if _, seen := comicSet[comicID]; !seen {
			comicSet[comicID] = struct{}{}
			comicIDs = append(comicIDs, comicID)
		}
	}
	// Units backed by the same physical archive cannot be separated safely.
	for comicID := range comicSet {
		total, moved := 0, 0
		for _, unit := range work.Units {
			if unit.ComicID != comicID {
				continue
			}
			total++
			if _, ok := selected[unit.ID]; ok {
				moved++
			}
		}
		if moved != total {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Units inside the same physical archive cannot be split apart",
			})
			return
		}
	}
	manualID := store.ManualWorkIDPrefix + strings.ReplaceAll(uuid.New().String(), "-", "")
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = work.Title + " 拆分"
	}
	first := work.Units[unitsByID[unitIDs[0]]]
	if err := store.EnsureLogicalWorkRecord(store.LogicalWork{
		ID:               manualID,
		LibraryID:        work.LibraryID,
		RootPath:         work.RootPath,
		Title:            title,
		CoverComicID:     first.ComicID,
		CoverAspectRatio: first.CoverAspectRatio,
	}); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	entries := make([]store.WorkMembershipEntry, 0, len(comicIDs))
	for _, comicID := range comicIDs {
		entries = append(entries, store.WorkMembershipEntry{
			ComicID: comicID, WorkID: manualID, LibraryID: work.LibraryID,
		})
	}
	if err := store.SetWorkMemberships(entries); err != nil {
		writeWorkMutationError(c, err)
		return
	}
	completeWorkMutation(c, gin.H{"workId": manualID, "units": len(unitIDs)})
}
