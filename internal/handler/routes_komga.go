package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
)

// registerKomgaRoutes exposes a Komga-compatible REST surface so Mihon /
// Tachiyomi (Komga extension), Paperback, Panels and Chunky can browse and read
// the library without speaking our native API.
//
// Everything is read-only apart from the read-progress endpoints, and all of it
// sits behind KomgaAuthRequired (session / Bearer key / X-API-Key / Basic).
func registerKomgaRoutes(api *gin.RouterGroup) {
	komga := NewKomgaHandler()
	group := api.Group("/v1")
	group.Use(middleware.KomgaAuthRequired())
	{
		group.GET("/libraries", komga.Libraries)

		group.GET("/series", komga.Series)
		group.GET("/series/:id", komga.SeriesDetail)
		group.GET("/series/:id/books", komga.SeriesBooks)
		group.GET("/series/:id/thumbnail", komga.SeriesThumbnail)
		group.GET("/series/:id/read-progress", komga.SeriesReadProgress)
		group.POST("/series/:id/read-progress", komga.SeriesReadProgressUpdate)

		group.GET("/books", komga.Books)
		group.GET("/books/:id", komga.BookDetail)
		group.GET("/books/:id/thumbnail", komga.BookThumbnail)
		group.GET("/books/:id/pages", komga.BookPages)
		group.GET("/books/:id/pages/:pageNumber", komga.BookPageImage)
		group.GET("/books/:id/pages/:pageNumber/thumbnail", komga.BookPageThumbnail)
		group.GET("/books/:id/file", komga.BookFile)
		group.GET("/books/:id/read-progress", komga.BookReadProgress)
		group.PATCH("/books/:id/read-progress", komga.BookReadProgress)
		group.DELETE("/books/:id/read-progress", komga.BookReadProgress)

		// Collections are not modelled; returning an empty page keeps the
		// extension's filter screen working.
		group.GET("/collections", komga.Collections)

		group.GET("/genres", komga.Genres)
		group.GET("/tags", komga.Tags)
		group.GET("/publishers", komga.Publishers)
		group.GET("/authors", komga.Authors)
	}
}
