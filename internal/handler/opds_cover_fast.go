package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

var opdsCoverReadFile = os.ReadFile
var opdsWorkCoverDownload = service.DownloadWorkCover

// WorkCoverFast serves persisted LogicalWork covers without rebuilding the
// complete downloadable Work catalog. Unpersisted legacy Works retain the old
// indexed compatibility path.
func (h *OPDSHandler) WorkCoverFast(c *gin.Context) {
	workID := c.Param("id")
	work, err := store.GetLogicalWork(workID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Work"})
		return
	}
	if work == nil {
		h.WorkCover(c)
		return
	}
	if work.ContentType != "comic" || work.MissingSince != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	allowed, err := store.UserCanDownloadLibrary(getOPDSUserID(c), work.LibraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check download permission"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Download permission required"})
		return
	}

	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(work.ID))
	if info, statErr := os.Stat(cachePath); statErr == nil && !info.IsDir() && info.Size() > 0 {
		etag := fmt.Sprintf(`"%s-%s"`,
			strconv.FormatInt(info.ModTime().UnixNano(), 36),
			strconv.FormatInt(info.Size(), 36),
		)
		c.Header("Cache-Control", "private, max-age=300, must-revalidate")
		c.Header("Vary", "Authorization, Cookie")
		c.Header("ETag", etag)
		c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
		if c.GetHeader("If-None-Match") == etag {
			c.Status(http.StatusNotModified)
			return
		}
		if c.Request.Method == http.MethodHead {
			c.Status(http.StatusOK)
			return
		}
		data, readErr := opdsCoverReadFile(cachePath)
		if readErr == nil && len(data) > 0 {
			c.Data(http.StatusOK, archive.ThumbnailMimeType(data), data)
			return
		}
	}
	if strings.HasPrefix(work.CoverURL, "http://") || strings.HasPrefix(work.CoverURL, "https://") {
		c.Header("Cache-Control", "private, no-store")
		c.Header("Vary", "Authorization, Cookie")
		go opdsWorkCoverDownload(work.ID, work.CoverURL)
		c.Redirect(http.StatusTemporaryRedirect, work.CoverURL)
		return
	}
	if work.CoverComicID != "" {
		h.renderCover(c, work.CoverComicID)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Work cover unavailable"})
}
