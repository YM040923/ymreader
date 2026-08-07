package handler

import (
	"fmt"
	"net/http"
	"strings"

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
	if contentType != "novel" {
		works, err := NewWorkHandler().loadWorks(c, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comic Works"})
			return
		}
		filters := parseWorkFilters(c)
		filters.metaFilter = ""
		filters.search = ""
		works = filterWorks(works, filters)
		if search != "" {
			filtered := works[:0]
			query := strings.ToLower(strings.TrimSpace(search))
			for _, work := range works {
				if strings.Contains(strings.ToLower(work.Title), query) ||
					strings.Contains(strings.ToLower(work.Author), query) {
					filtered = append(filtered, work)
				}
			}
			works = filtered
		}
		if metaFilter == "with" || metaFilter == "missing" {
			filtered := works[:0]
			for _, work := range works {
				hasMeta := metadataWorkHasMetadata(work)
				if (metaFilter == "with" && hasMeta) || (metaFilter == "missing" && !hasMeta) {
					filtered = append(filtered, work)
				}
			}
			works = filtered
		}
		sortWorks(works, sortBy, sortOrder)
		total := len(works)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		items := make([]gin.H, 0, end-start)
		for _, work := range works[start:end] {
			tags := make([]gin.H, 0, len(work.Tags))
			for _, tag := range work.Tags {
				tags = append(tags, gin.H{"name": tag.Name, "color": tag.Color})
			}
			categories := make([]gin.H, 0, len(work.Categories))
			for _, category := range work.Categories {
				categories = append(categories, gin.H{"slug": category.Slug, "name": category.Name, "icon": category.Icon})
			}
			items = append(items, gin.H{
				"id": work.ID, "title": work.Title, "filename": work.RootPath,
				"author": work.Author, "genre": work.Genre, "description": work.Description,
				"year": work.Year, "publisher": work.Publisher, "language": work.Language,
				"fileSize": work.FileSize, "updatedAt": work.UpdatedAt,
				"metadataSource": work.MetadataSource, "hasMetadata": metadataWorkHasMetadata(work),
				"contentType": "comic", "entityType": "work", "itemCount": work.ItemCount,
				"representativeComicId": work.RepresentativeComicID,
				"coverUrl":              work.CoverURL, "tags": tags, "rating": work.Rating,
				"isFavorite": work.IsFavorite, "categories": categories,
			})
		}
		totalPages := 1
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages})
		return
	}

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

func metadataWorkHasMetadata(work service.Work) bool {
	logical, err := store.GetLogicalWork(work.ID)
	if err != nil {
		return false
	}
	return !metadataTargetMissing(metadataTarget{
		EntityType: "work", EntityID: work.ID, Title: work.Title, Author: work.Author,
		Work: &work, LogicalWork: logical,
	})
}

// POST /api/metadata/batch-selected — 对选中项执行批量刮削 (SSE)
