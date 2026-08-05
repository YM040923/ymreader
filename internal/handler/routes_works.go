package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
)

func registerWorkRoutes(api *gin.RouterGroup) {
	work := NewWorkHandler()

	group := api.Group("/works")
	group.Use(middleware.AuthRequired())
	{
		group.GET("", work.List)
		group.POST("/rebuild", work.Rebuild)
		group.GET("/:id", work.Get)
		group.GET("/:id/continue", work.Continue)
		group.PUT("/:id/progress", work.UpdateProgress)
		group.GET("/:id/units/:unitId/adjacent", work.Adjacent)
	}

	admin := api.Group("/admin/works")
	admin.Use(middleware.AdminRequired())
	{
		admin.POST("/rebuild", work.Rebuild)
	}
}
