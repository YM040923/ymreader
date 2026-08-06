package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func (h *MetadataHandler) Library(c *gin.Context) {
	search := c.Query("search")
	metaFilter := c.DefaultQuery("metaFilter", "all") // "all" | "with" | "missing"
	contentType := c.Query("contentType")             // "comic" | "novel" | ""
	sortBy := c.DefaultQuery("sortBy", "title")       // "title" | "fileSize" | "updatedAt" | "metaStatus"
	sortOrder := c.DefaultQuery("sortOrder", "asc")   // "asc" | "desc"
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if n, err := fmt.Sscanf(p, "%d", &page); n == 0 || err != nil {
			page = 1
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if n, err := fmt.Sscanf(ps, "%d", &pageSize); n == 0 || err != nil {
			pageSize = 20
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 验证排序字段白名单
	allowedSortBy := map[string]bool{"title": true, "fileSize": true, "updatedAt": true, "metaStatus": true, "addedAt": true}
	if !allowedSortBy[sortBy] {
		sortBy = "title"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	// metaStatus 排序需要特殊处理：映射到数据库字段
	dbSortBy := sortBy
	if sortBy == "metaStatus" {
		dbSortBy = "metadataSource"
	}

	result, err := store.GetAllComics(store.ComicListOptions{
		Search:      search,
		SortBy:      dbSortBy,
		SortOrder:   sortOrder,
		Page:        page,
		PageSize:    pageSize,
		ContentType: contentType,
		MetaFilter:  metaFilter, // SQL 层面过滤，分页准确
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get comics"})
		return
	}

	type LibraryItemTag struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	type LibraryItemCategory struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Icon string `json:"icon"`
	}
	type LibraryItem struct {
		ID             string                `json:"id"`
		Title          string                `json:"title"`
		Filename       string                `json:"filename"`
		Author         string                `json:"author"`
		Genre          string                `json:"genre"`
		Description    string                `json:"description"`
		Year           *int                  `json:"year"`
		Publisher      string                `json:"publisher"`
		Language       string                `json:"language"`
		FileSize       int64                 `json:"fileSize"`
		UpdatedAt      string                `json:"updatedAt"`
		MetadataSource string                `json:"metadataSource"`
		HasMetadata    bool                  `json:"hasMetadata"`
		ContentType    string                `json:"contentType"`
		Tags           []LibraryItemTag      `json:"tags"`
		Rating         *int                  `json:"rating"`
		IsFavorite     bool                  `json:"isFavorite"`
		Categories     []LibraryItemCategory `json:"categories"`
	}

	var items []LibraryItem
	for _, comic := range result.Comics {
		hasMeta := comic.MetadataSource != ""

		ct := comic.ComicType
		if ct == "" {
			if service.IsNovelFilename(comic.Filename) {
				ct = "novel"
			} else {
				ct = "comic"
			}
		}

		var tags []LibraryItemTag
		for _, t := range comic.Tags {
			tags = append(tags, LibraryItemTag{Name: t.Name, Color: t.Color})
		}
		if tags == nil {
			tags = []LibraryItemTag{}
		}

		var cats []LibraryItemCategory
		for _, cat := range comic.Categories {
			cats = append(cats, LibraryItemCategory{Slug: cat.Slug, Name: cat.Name, Icon: cat.Icon})
		}
		if cats == nil {
			cats = []LibraryItemCategory{}
		}

		items = append(items, LibraryItem{
			ID:             comic.ID,
			Title:          comic.Title,
			Filename:       comic.Filename,
			Author:         comic.Author,
			Genre:          comic.Genre,
			Description:    comic.Description,
			Year:           comic.Year,
			Publisher:      comic.Publisher,
			Language:       comic.Language,
			FileSize:       comic.FileSize,
			UpdatedAt:      comic.UpdatedAt,
			MetadataSource: comic.MetadataSource,
			HasMetadata:    hasMeta,
			ContentType:    ct,
			Tags:           tags,
			Rating:         comic.Rating,
			IsFavorite:     comic.IsFavorite,
			Categories:     cats,
		})
	}

	if items == nil {
		items = []LibraryItem{}
	}

	c.JSON(200, gin.H{
		"items":      items,
		"total":      result.Total,
		"page":       result.Page,
		"pageSize":   result.PageSize,
		"totalPages": result.TotalPages,
	})
}

// POST /api/metadata/batch-selected — 对选中项执行批量刮削 (SSE)
