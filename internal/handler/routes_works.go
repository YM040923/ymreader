package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
)

func registerWorkRoutes(api *gin.RouterGroup) {
	handler := NewWorkHandler()
	works := api.Group("/works")
	works.Use(middleware.AuthRequired())
	{
		works.GET("", handler.List)
		works.GET("/tags", handler.TagStats)
		works.GET("/categories", handler.CategoryStats)
		works.POST("/batch", handler.Batch)
		works.PUT("/reorder", handler.Reorder)
		works.GET("/:id/units", handler.Units)
		works.PUT("/:id/favorite", handler.SetFavorite)
		works.PUT("/:id/reading-status", handler.SetReadingStatus)
		works.PUT("/:id/tags", handler.SetTags)
		works.PUT("/:id/categories", handler.SetCategories)
		works.PUT("/:id/cover", handler.UpdateCover)
		works.PUT("/:id/metadata", handler.UpdateMetadata)
		works.POST("/:id/scrape-metadata", middleware.ScraperRequired(), handler.ScrapeMetadata)
		works.POST("/:id/apply-metadata", middleware.ScraperRequired(), handler.ApplyScrapedMetadata)
		works.POST("/:id/ai-recognize", middleware.ScraperRequired(), handler.AIRecognize)
		works.DELETE("/:id", handler.Delete)
		works.GET("/:id", handler.Get)
	}
}
