package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// recordOnlySingleDeleteGuard records the source identity before a single
// record-only deletion. For physical deletion it closes pooled archive readers
// before the existing handler removes the database row.
func recordOnlySingleDeleteGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query("deleteFiles") == "true" {
			comicID := c.Param("id")
			resolved, err := service.GlobalFileResolver.ResolveContentPath(comicID)
			if err != nil {
				// A missing physical file is already in the desired state. Allow the
				// existing handler to remove the stale database row.
				if strings.Contains(strings.ToLower(err.Error()), "file not found") {
					c.Next()
					return
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to resolve physical file before deletion: " + err.Error(),
				})
				return
			}
			if resolved.AbsolutePath != "" {
				if err := service.RemoveContentFile(comicID, resolved.AbsolutePath); err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"error": "Failed to delete physical file: " + err.Error(),
					})
					return
				}
			}

			// The physical file is gone now. The existing handler will resolve an
			// empty path, delete the database record, and return the normal response.
			c.Next()
			return
		}

		identities, err := store.GetComicSourceIdentities([]string{c.Param("id")})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to preserve record-only deletion"})
			return
		}
		if err := store.AddIgnoredLibraryContents(identities); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to preserve record-only deletion"})
			return
		}

		c.Next()
		if c.Writer.Status() >= http.StatusBadRequest {
			if err := store.RemoveIgnoredLibraryContents(identities); err != nil {
				log.Printf("[delete-guard] failed to roll back single delete tombstone: %v", err)
			}
		}
	}
}

// recordOnlyBatchDeleteGuard applies record-only tombstones to batch deletes.
// When physical files are requested it validates permissions, releases cached
// readers, removes the files first, then rewrites the request so the existing
// handler performs only the database deletion.
func recordOnlyBatchDeleteGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var payload struct {
			Action      string   `json:"action"`
			ComicIDs    []string `json:"comicIds"`
			DeleteFiles bool     `json:"deleteFiles"`
		}
		if json.Unmarshal(body, &payload) != nil || payload.Action != "delete" || len(payload.ComicIDs) == 0 {
			c.Next()
			return
		}

		if payload.DeleteFiles {
			user := middleware.GetCurrentUser(c)
			if user == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}
			if err := checkBatchManagePermission(user, payload.ComicIDs); err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			deletedIDs := make([]string, 0, len(payload.ComicIDs))
			var failures []string
			for _, comicID := range payload.ComicIDs {
				resolved, resolveErr := service.GlobalFileResolver.ResolveContentPath(comicID)
				if resolveErr != nil || resolved.AbsolutePath == "" {
					// The file is already absent or cannot be resolved. Removing the
					// database record remains the correct cleanup action.
					deletedIDs = append(deletedIDs, comicID)
					continue
				}
				if err := service.RemoveContentFile(comicID, resolved.AbsolutePath); err != nil {
					failures = append(failures, fmt.Sprintf("%s: %v", resolved.AbsolutePath, err))
					continue
				}
				deletedIDs = append(deletedIDs, comicID)
			}

			if len(failures) > 0 {
				// Keep database state consistent for files that were successfully
				// removed, while preserving failed rows so the user can retry them.
				if len(deletedIDs) > 0 {
					if _, err := store.BatchDeleteComics(deletedIDs); err != nil {
						c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
							"error": "Some files were removed, but database cleanup failed: " + err.Error(),
						})
						return
					}
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":        fmt.Sprintf("Deleted %d item(s), but %d physical file(s) are still locked", len(deletedIDs), len(failures)),
					"failedFiles":  failures,
					"deletedCount": len(deletedIDs),
					"failedCount":  len(failures),
				})
				return
			}

			// Files are already removed. Preserve the original request fields and
			// let the existing handler run permission checks and DB cleanup only.
			var rawPayload map[string]interface{}
			if err := json.Unmarshal(body, &rawPayload); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
				return
			}
			rawPayload["deleteFiles"] = false
			rewritten, err := json.Marshal(rawPayload)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare database deletion"})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(rewritten))
			c.Request.ContentLength = int64(len(rewritten))
			c.Next()
			return
		}

		identities, err := store.GetComicSourceIdentities(payload.ComicIDs)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to preserve record-only deletion"})
			return
		}
		if err := store.AddIgnoredLibraryContents(identities); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to preserve record-only deletion"})
			return
		}

		c.Next()
		if c.Writer.Status() >= http.StatusBadRequest {
			if err := store.RemoveIgnoredLibraryContents(identities); err != nil {
				log.Printf("[delete-guard] failed to roll back batch delete tombstones: %v", err)
			}
		}
	}
}
