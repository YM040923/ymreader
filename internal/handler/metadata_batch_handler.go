package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
)

func (h *MetadataHandler) Batch(c *gin.Context) {
	var body struct {
		Mode        string `json:"mode"`
		Lang        string `json:"lang"`
		UpdateTitle bool   `json:"updateTitle"`
		SkipCover   bool   `json:"skipCover"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.Mode = "all"
		body.Lang = "en"
	}
	if body.Lang == "" {
		body.Lang = "en"
	}
	targets, err := discoverMetadataTargets(c, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get metadata targets"})
		return
	}
	targets = filterMissingMetadataTargets(targets, body.Mode)
	total := len(targets)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
	sendSSE := func(data interface{}) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
		c.Writer.Flush()
	}
	sendSSE(gin.H{"type": "start", "total": total})

	success, failed := 0, 0
	for index, target := range targets {
		progress := metadataProgressEvent(target, index+1, total)
		source, matchTitle, scrapeErr := executeStandardMetadataTarget(
			c.Request.Context(), target, body.Lang, body.UpdateTitle, body.SkipCover,
		)
		if scrapeErr == nil {
			progress["status"] = "success"
			progress["source"] = source
			if matchTitle != "" {
				progress["matchTitle"] = matchTitle
			}
			success++
		} else {
			progress["status"] = "failed"
			progress["message"] = scrapeErr.Error()
			failed++
		}
		sendSSE(progress)
	}
	sendSSE(gin.H{"type": "complete", "total": total, "success": success, "failed": failed})
}

// POST /api/metadata/translate-batch — SSE stream（多引擎支持）
func (h *MetadataHandler) TranslateBatch(c *gin.Context) {
	var body struct {
		TargetLang string                  `json:"targetLang"`
		Engine     service.TranslateEngine `json:"engine"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetLang == "" {
		c.JSON(400, gin.H{"error": "targetLang is required"})
		return
	}

	targets, err := discoverMetadataTargets(c, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get metadata targets"})
		return
	}
	total := len(targets)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	sendSSE := func(data interface{}) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
		c.Writer.Flush()
	}
	sendSSE(gin.H{"type": "start", "total": total})

	translated, skipped, failed := 0, 0, 0
	for index, target := range targets {
		progress := metadataProgressEvent(target, index+1, total)
		result, translateErr := translateMetadataTarget(target, body.TargetLang, body.Engine)
		if translateErr != nil {
			progress["status"] = "failed"
			progress["error"] = translateErr.Error()
			sendSSE(progress)
			failed++
			continue
		}
		if result == nil || len(result.Fields) == 0 {
			progress["status"] = "skipped"
			sendSSE(progress)
			skipped++
			continue
		}
		if applyErr := applyMetadataTranslation(target, result.Fields); applyErr != nil {
			progress["status"] = "failed"
			progress["error"] = applyErr.Error()
			sendSSE(progress)
			failed++
			continue
		}
		progress["status"] = "translated"
		progress["engine"] = string(result.Engine)
		progress["cached"] = result.Cached
		sendSSE(progress)
		translated++
	}

	sendSSE(gin.H{
		"type":       "done",
		"total":      total,
		"translated": translated,
		"skipped":    skipped,
		"failed":     failed,
	})
}

// GET /api/metadata/stats — 元数据统计概览
func (h *MetadataHandler) Stats(c *gin.Context) {
	targets, err := discoverMetadataTargets(c, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get metadata targets"})
		return
	}
	withMeta, missing := 0, 0
	for _, target := range targets {
		if metadataTargetMissing(target) {
			missing++
		} else {
			withMeta++
		}
	}
	c.JSON(200, gin.H{"total": len(targets), "withMetadata": withMeta, "missing": missing})
}

func executeStandardMetadataTarget(
	ctx context.Context,
	target metadataTarget,
	lang string,
	updateTitle bool,
	skipCover bool,
) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if target.EntityType == "work" {
		work, err := requireMetadataTargetWork(target)
		if err != nil {
			return "", "", err
		}
		query := metadataTargetSearchQuery(target)
		if query == "" {
			return "", "", fmt.Errorf("no Work search query")
		}
		if err := waitMetadataRateLimit(ctx); err != nil {
			return "", "", err
		}
		if err := service.ScrapeWorkMetadataContext(ctx, work, query, skipCover || work.CoverLocked); err != nil {
			return "", "", err
		}
		return "work", "", nil
	}
	if target.Comic == nil {
		return "", "", fmt.Errorf("Comic metadata target is unavailable")
	}
	comic := *target.Comic
	resolved, resolveErr := service.GlobalFileResolver.ResolveContentPath(comic.ID)
	filePath := ""
	if resolveErr == nil {
		filePath = resolved.AbsolutePath
	}
	if filePath != "" && strings.EqualFold(filepath.Ext(comic.Filename), ".epub") {
		if epubMeta, err := service.ExtractEpubMetadata(filePath); err == nil && epubMeta != nil && epubMeta.Title != "" {
			if _, err := service.ApplyMetadata(comic.ID, *epubMeta, lang, updateTitle, service.ApplyOption{SkipCover: skipCover}); err == nil {
				return "epub_opf", epubMeta.Title, nil
			}
		}
	}
	query := metadataTargetSearchQuery(target)
	if query == "" {
		return "", "", fmt.Errorf("no Comic search query")
	}
	if err := waitMetadataRateLimit(ctx); err != nil {
		return "", "", err
	}
	results := service.SearchMetadata(query, nil, lang, "novel")
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if len(results) == 0 {
		return "", "", fmt.Errorf("no metadata found")
	}
	if _, err := service.ApplyMetadata(comic.ID, results[0], lang, updateTitle, service.ApplyOption{SkipCover: skipCover}); err != nil {
		return "", "", err
	}
	return results[0].Source, results[0].Title, nil
}

func waitMetadataRateLimit(ctx context.Context) error {
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// POST /api/metadata/ai-batch — AI 智能批量刮削 (SSE)
// 流水线：AI 解析文件名 → 在线搜索 → AI 补全 → 应用元数据
func (h *MetadataHandler) AIBatch(c *gin.Context) {
	var body struct {
		Mode        string `json:"mode"`
		Lang        string `json:"lang"`
		UpdateTitle bool   `json:"updateTitle"`
		SkipCover   bool   `json:"skipCover"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.Mode = "missing"
		body.Lang = "zh"
	}
	if body.Lang == "" {
		body.Lang = "zh"
	}
	aiCfg := service.LoadAIConfig()
	if !aiCfg.EnableCloudAI || aiCfg.CloudAPIKey == "" {
		c.JSON(400, gin.H{"error": "AI not configured"})
		return
	}
	targets, err := discoverMetadataTargets(c, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get metadata targets"})
		return
	}
	targets = filterMissingMetadataTargets(targets, body.Mode)
	total := len(targets)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
	sendSSE := func(data interface{}) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
		c.Writer.Flush()
	}
	sendSSE(gin.H{"type": "start", "total": total})
	success, failed := 0, 0
	for index, target := range targets {
		progress := metadataProgressEvent(target, index+1, total)
		progress["step"] = "recognize"
		sendSSE(progress)
		source, matchTitle, scrapeErr := executeAIMetadataTarget(
			c.Request.Context(), target, aiCfg, body.Lang, body.UpdateTitle, body.SkipCover,
		)
		progress["step"] = "done"
		if scrapeErr == nil {
			progress["status"] = "success"
			progress["source"] = source
			if matchTitle != "" {
				progress["matchTitle"] = matchTitle
			}
			success++
		} else {
			progress["status"] = "failed"
			progress["message"] = scrapeErr.Error()
			failed++
		}
		sendSSE(progress)
	}
	sendSSE(gin.H{"type": "complete", "total": total, "success": success, "failed": failed})
}

func executeAIMetadataTarget(
	ctx context.Context,
	target metadataTarget,
	cfg service.AIConfig,
	lang string,
	updateTitle bool,
	skipCover bool,
) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	comicID := target.EntityID
	filename := target.Filename
	if target.EntityType == "work" {
		work, err := requireMetadataTargetWork(target)
		if err != nil {
			return "", "", err
		}
		comicID = work.RepresentativeComicID
		if comicID == "" {
			ids := service.PhysicalComicIDs(work)
			if len(ids) > 0 {
				comicID = ids[0]
			}
		}
	}
	var coverData []byte
	if comicID != "" {
		if cover, _, _, err := service.GetComicThumbnail(comicID); err == nil {
			coverData = cover
		}
	}
	pageImages := make([][]byte, 0, 2)
	for page := 0; comicID != "" && page < 2; page++ {
		if image, err := service.GetPageImage(comicID, page); err == nil && image != nil && len(image.Data) > 0 {
			pageImages = append(pageImages, image.Data)
		}
	}
	query := ""
	if recognized, err := service.AIRecognizeComicContent(cfg, coverData, pageImages, lang); err == nil && recognized != nil && recognized.Title != "" {
		query = strings.TrimSpace(strings.Join([]string{recognized.Title, recognized.Author}, " "))
	}
	if query == "" && target.EntityType == "comic" {
		if parsed, err := service.AIParseFilename(cfg, filename); err == nil && parsed != nil && parsed.Title != "" {
			query = strings.TrimSpace(strings.Join([]string{parsed.Title, parsed.Author}, " "))
		}
	}
	if query == "" {
		query = metadataTargetSearchQuery(target)
	}
	if query == "" {
		return "", "", fmt.Errorf("no metadata search query")
	}
	if err := waitMetadataRateLimit(ctx); err != nil {
		return "", "", err
	}
	results := service.SearchMetadata(query, nil, lang, target.ContentType)
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if len(results) > 0 {
		if target.EntityType == "work" {
			work, _ := requireMetadataTargetWork(target)
			if err := service.ApplyWorkMetadataContext(ctx, work, skipCover || work.CoverLocked, results[0]); err != nil {
				return "", "", err
			}
		} else if _, err := service.ApplyMetadata(target.EntityID, results[0], lang, updateTitle, service.ApplyOption{SkipCover: skipCover}); err != nil {
			return "", "", err
		}
		return results[0].Source, results[0].Title, nil
	}
	completed, err := service.AICompleteMetadata(cfg, filename, target.Title, coverData, lang)
	if err != nil || completed == nil {
		if err == nil {
			err = fmt.Errorf("no metadata found")
		}
		return "", "", err
	}
	meta := service.ComicMetadata{
		Title: completed.Title, Author: completed.Author, Genre: completed.Genre,
		Description: completed.Description, Language: completed.Language, Year: completed.Year,
		Source: "ai_complete",
	}
	if target.EntityType == "work" {
		work, _ := requireMetadataTargetWork(target)
		if err := service.ApplyWorkMetadataContext(ctx, work, skipCover || work.CoverLocked, meta); err != nil {
			return "", "", err
		}
	} else if _, err := service.ApplyMetadata(target.EntityID, meta, lang, updateTitle, service.ApplyOption{SkipCover: skipCover}); err != nil {
		return "", "", err
	}
	return "ai_complete", completed.Title, nil
}

// GET /api/metadata/library — 书库管理列表（带元数据状态过滤）
