package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type WorkHandler struct{}

func NewWorkHandler() *WorkHandler { return &WorkHandler{} }

func (h *WorkHandler) List(c *gin.Context) {
	libraryIDs, err := requestedAccessibleLibraries(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve library access"})
		return
	}
	user := middleware.GetCurrentUser(c)
	if user == nil || (user.Role != "admin" && len(libraryIDs) == 0) {
		c.JSON(http.StatusOK, gin.H{"works": []model.Work{}, "total": 0})
		return
	}
	works, err := store.ListWorks(libraryIDs, getUserID(c), c.Query("search"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list works"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"works": works, "total": len(works)})
}

func (h *WorkHandler) Get(c *gin.Context) {
	workID := c.Param("id")
	work, units, err := store.GetWorkDetail(workID, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load work"})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	allowed, err := store.UserCanViewLibrary(getUserID(c), work.LibraryID)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to this library"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"work": work, "units": units})
}

func (h *WorkHandler) Continue(c *gin.Context) {
	work, unit, pageIndex, err := h.resolveContinueTarget(c.Param("id"), getUserID(c))
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "No access") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"workId":    work.ID,
		"unitId":    unit.ID,
		"comicId":   unit.ComicID,
		"pageIndex": pageIndex,
	})
}

func (h *WorkHandler) UpdateProgress(c *gin.Context) {
	workID := c.Param("id")
	work, _, err := store.GetWorkDetail(workID, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load work"})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	allowed, err := store.UserCanViewLibrary(getUserID(c), work.LibraryID)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to this library"})
		return
	}
	var body struct {
		UnitID    string `json:"unitId"`
		PageIndex int    `json:"pageIndex"`
		ComicID   string `json:"comicId"`
		Finalize  bool   `json:"finalize"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if strings.TrimSpace(body.UnitID) == "" && strings.TrimSpace(body.ComicID) != "" {
		if unit, err := store.GetWorkUnitByComicID(body.ComicID); err == nil && unit != nil {
			body.UnitID = unit.ID
		}
	}
	if strings.TrimSpace(body.UnitID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unitId is required"})
		return
	}
	unit, err := store.GetWorkUnit(body.UnitID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load work unit"})
		return
	}
	if unit == nil || unit.WorkID != workID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unitId does not belong to this work"})
		return
	}
	if body.PageIndex < 0 {
		body.PageIndex = 0
	}
	if err := store.SetUserWorkProgress(getUserID(c), workID, body.UnitID, body.PageIndex); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update work progress"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkHandler) Adjacent(c *gin.Context) {
	workID := c.Param("id")
	unitID := c.Param("unitId")
	work, units, err := store.GetWorkDetail(workID, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load work"})
		return
	}
	if work == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	allowed, err := store.UserCanViewLibrary(getUserID(c), work.LibraryID)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to this library"})
		return
	}
	currentIdx := -1
	for i := range units {
		if units[i].ID == unitID {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work unit not found"})
		return
	}
	var prev, next *model.WorkUnit
	if currentIdx > 0 {
		prev = &units[currentIdx-1]
	}
	if currentIdx+1 < len(units) {
		next = &units[currentIdx+1]
	}
	c.JSON(http.StatusOK, gin.H{"previous": prev, "next": next})
}

func (h *WorkHandler) Rebuild(c *gin.Context) {
	libraryID := strings.TrimSpace(c.Query("libraryId"))
	if libraryID != "" {
		canManage, err := store.UserCanManageLibrary(getUserID(c), libraryID)
		if err != nil || !canManage {
			c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage this library"})
			return
		}
		if err := service.RebuildWorksForLibrary(libraryID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "libraryId": libraryID})
		return
	}
	if user := middleware.GetCurrentUser(c); user == nil || user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin required"})
		return
	}
	if err := service.RebuildAllWorks(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WorkHandler) resolveContinueTarget(workID, userID string) (*model.Work, *model.WorkUnit, int, error) {
	work, _, err := store.GetWorkDetail(workID, userID)
	if err != nil {
		return nil, nil, 0, err
	}
	if work == nil {
		return nil, nil, 0, &handlerError{"work not found"}
	}
	allowed, err := store.UserCanViewLibrary(userID, work.LibraryID)
	if err != nil {
		return nil, nil, 0, err
	}
	if !allowed {
		return nil, nil, 0, &handlerError{"No access to this library"}
	}
	progress, err := store.GetUserWorkProgress(userID, workID)
	if err != nil {
		return nil, nil, 0, err
	}
	if progress != nil && strings.TrimSpace(progress.UnitID) != "" {
		if unit, err := store.GetWorkUnit(progress.UnitID); err == nil && unit != nil && unit.WorkID == workID {
			return work, unit, progress.PageIndex, nil
		}
	}
	unit, err := store.GetFirstWorkUnit(workID)
	if err != nil {
		return nil, nil, 0, err
	}
	if unit == nil {
		return nil, nil, 0, &handlerError{"work unit not found"}
	}
	return work, unit, 0, nil
}

type handlerError struct{ msg string }

func (e *handlerError) Error() string { return e.msg }

func parseIntDefault(value string, def int) int {
	if strings.TrimSpace(value) == "" {
		return def
	}
	if v, err := strconv.Atoi(value); err == nil {
		return v
	}
	return def
}
