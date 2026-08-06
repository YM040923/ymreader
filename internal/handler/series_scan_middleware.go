package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
)

// rebuildSeriesAfterScan keeps the logical Series/Section projection in sync
// with successful manual, upload-triggered and per-library scans. Background
// scans are additionally covered by the lazy shelf refresh.
func rebuildSeriesAfterScan() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		status := c.Writer.Status()
		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			return
		}

		libraryID := c.Param("id")
		var err error
		if libraryID != "" {
			err = service.RebuildComicSeriesForLibrary(libraryID)
		} else {
			err = service.RebuildAllComicSeries()
		}
		if err != nil {
			log.Printf("[series] rebuild after scan failed: %v", err)
		}
	}
}
