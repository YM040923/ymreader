package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// ============================================================
// Phase 5-2: POST /api/ai/batch-suggest-tags — AI 批量标签标注 (SSE)
// ============================================================

func (h *AIHandler) BatchSuggestTags(c *gin.Context) {
	var body aiBatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}
	if body.TargetLang == "" {
		body.TargetLang = "zh"
	}
	selection, err := resolveAIBatchSelection(c, &body, "tags")
	if err != nil {
		writeAIBatchSelectionError(c, err)
		return
	}
	body.ComicIDs = selection.ComicIDs

	cfg := service.LoadAIConfig()
	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		c.JSON(400, gin.H{"error": "AI not configured"})
		return
	}

	// SSE 流式返回，逐条推送
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	if body.Selector != nil {
		selectionData, _ := json.Marshal(gin.H{
			"selection": gin.H{
				"eligible": selection.Eligible,
				"selected": len(body.ComicIDs),
			},
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", selectionData)
		c.Writer.Flush()
	}

	successCount := 0
	failCount := 0

	for i, comicID := range body.ComicIDs {
		// 权限校验：检查用户是否有权访问该漫画
		if err := checkComicAccess(c, comicID); err != nil {
			failCount++
			data, _ := json.Marshal(gin.H{
				"comicId": comicID,
				"index":   i,
				"total":   len(body.ComicIDs),
				"error":   "access denied",
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			continue
		}

		comic, err := store.GetComicByID(comicID)
		if err != nil || comic == nil {
			failCount++
			data, _ := json.Marshal(gin.H{
				"comicId": comicID,
				"index":   i,
				"total":   len(body.ComicIDs),
				"error":   "Comic not found",
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			continue
		}

		// 收集已有标签
		var existingTags []string
		for _, t := range comic.Tags {
			existingTags = append(existingTags, t.Name)
		}

		// 判断内容类型
		contentType := "comic/manga"
		if service.IsNovelFilename(comic.Filename) {
			contentType = "novel/light novel"
		}

		suggestedTags, err := service.SuggestTags(
			cfg,
			comic.Title,
			comic.Author,
			comic.Genre,
			comic.Description,
			contentType,
			body.TargetLang,
			existingTags,
		)

		if err != nil {
			failCount++
			data, _ := json.Marshal(gin.H{
				"comicId": comicID,
				"title":   comic.Title,
				"index":   i,
				"total":   len(body.ComicIDs),
				"error":   err.Error(),
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			continue
		}

		// 如果 apply=true，自动添加标签
		if body.Apply && len(suggestedTags) > 0 {
			_ = store.AddTagsToComic(comicID, suggestedTags)
		}

		successCount++
		data, _ := json.Marshal(gin.H{
			"comicId":       comicID,
			"title":         comic.Title,
			"index":         i,
			"total":         len(body.ComicIDs),
			"suggestedTags": suggestedTags,
			"applied":       body.Apply,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	// 完成标记
	doneData, _ := json.Marshal(gin.H{
		"done":    true,
		"success": successCount,
		"failed":  failCount,
		"total":   len(body.ComicIDs),
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", doneData)
	c.Writer.Flush()
}

// ============================================================
