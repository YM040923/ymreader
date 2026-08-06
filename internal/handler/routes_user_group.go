package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
)

func registerUserGroupRoutes(api *gin.RouterGroup) {
	group := NewUserGroupHandler()

	groupAdmin := api.Group("/admin/user-groups")
	groupAdmin.Use(middleware.AdminRequired())
	{
		groupAdmin.GET("", group.ListGroups)
		groupAdmin.POST("", group.CreateGroup)
		groupAdmin.PUT("/:id", group.UpdateGroup)
		groupAdmin.DELETE("/:id", group.DeleteGroup)

		// Members management
		groupAdmin.GET("/:id/members", group.GetMembers)
		groupAdmin.PUT("/:id/members", group.SetMembers)

		// Library access management
		groupAdmin.GET("/:id/library-access", group.GetLibraryAccess)
		groupAdmin.PUT("/:id/library-access", group.SetLibraryAccess)
	}
}
