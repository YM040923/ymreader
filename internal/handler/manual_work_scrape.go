package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func (h *MetadataHandler) StartLibraryWorkScrapeTask(c *gin.Context) {
	libraryID := strings.TrimSpace(c.Param("id"))
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	library, err := store.GetLibraryByID(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library"})
		return
	}
	if library == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
		return
	}
	if library.Type == "novel" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Work scrape tasks are only available for comic libraries"})
		return
	}
	canManage, err := store.UserCanManageLibrary(user.ID, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check library permission"})
		return
	}
	if !canManage {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: no manage permission for this library"})
		return
	}
	var body struct {
		Force bool `json:"force"`
	}
	_ = c.ShouldBindJSON(&body)
	query := c.Request.URL.Query()
	query.Set("libraryId", libraryID)
	c.Request.URL.RawQuery = query.Encode()
	targets, err := discoverMetadataTargets(c, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to discover library Works"})
		return
	}
	works := make([]service.Work, 0, len(targets))
	for _, target := range targets {
		if target.EntityType == "work" && target.Work != nil && target.Work.LibraryID == libraryID {
			works = append(works, *target.Work)
		}
	}
	task, err := service.StartManualWorkScrapeTask(libraryID, works, body.Force)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already running") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, task)
}

func (h *MetadataHandler) GetWorkScrapeTask(c *gin.Context) {
	task, ok := service.GetManualWorkScrapeTask(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work scrape task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *MetadataHandler) CancelWorkScrapeTask(c *gin.Context) {
	if !service.CancelManualWorkScrapeTask(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work scrape task not found"})
		return
	}
	task, _ := service.GetManualWorkScrapeTask(c.Param("id"))
	c.JSON(http.StatusAccepted, task)
}
