package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
)

// rebuildWorksAfterScan keeps the logical Work/WorkUnit projection in sync
// after successful scans. Novel libraries are skipped by the service layer.
func rebuildWorksAfterScan() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		status := c.Writer.Status()
		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			return
		}

		libraryID := c.Param("id")
		var err error
		if libraryID != "" {
			err = service.RebuildWorksForLibrary(libraryID)
		} else {
			err = service.RebuildAllWorks()
		}
		if err != nil {
			log.Printf("[works] rebuild after scan failed: %v", err)
		}
	}
}
