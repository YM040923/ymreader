package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/config"
)

// ScraperRequired is a middleware that checks if the scraper feature is enabled.
// Returns 403 if scraper is disabled in site config.
func ScraperRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.IsScraperEnabled() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Scraper feature is disabled",
				"code":  "SCRAPER_DISABLED",
			})
			return
		}
		c.Next()
	}
}
