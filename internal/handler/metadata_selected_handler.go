package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func (h *MetadataHandler) BatchSelected(c *gin.Context) {
	var body struct {
		ComicIDs []string `json:"comicIds"`
		Targets  []struct {
			ID         string `json:"id"`
			EntityType string `json:"entityType"`
		} `json:"targets"`
		Lang        string `json:"lang"`
		UpdateTitle bool   `json:"updateTitle"`
		Mode        string `json:"mode"`
		SkipCover   bool   `json:"skipCover"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || (len(body.ComicIDs) == 0 && len(body.Targets) == 0) {
		c.JSON(400, gin.H{"error": "comicIds or targets required"})
		return
	}
	if body.Lang == "" {
		body.Lang = "en"
	}
	selections := make([]metadataTargetSelection, 0, len(body.ComicIDs)+len(body.Targets))
	for _, id := range body.ComicIDs {
		selections = append(selections, metadataTargetSelection{ID: id, EntityType: "comic"})
	}
	for _, target := range body.Targets {
		selections = append(selections, metadataTargetSelection{ID: target.ID, EntityType: target.EntityType})
	}
	targets, err := discoverMetadataTargets(c, selections)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to resolve metadata targets"})
		return
	}
	if len(targets) == 0 {
		c.JSON(400, gin.H{"error": "No valid metadata targets found"})
		return
	}
	var aiCfg *service.AIConfig
	if body.Mode == "ai" {
		cfg := service.LoadAIConfig()
		if cfg.EnableCloudAI && cfg.CloudAPIKey != "" {
			aiCfg = &cfg
		}
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
	send := func(data interface{}) {
		encoded, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: %s\n\n", encoded)
		c.Writer.Flush()
	}
	send(gin.H{"type": "start", "total": len(targets)})
	success, failed := 0, 0
	for index, target := range targets {
		progress := metadataProgressEvent(target, index+1, len(targets))
		var source, matchTitle string
		var scrapeErr error
		if body.Mode == "ai" && aiCfg != nil {
			progress["step"] = "recognize"
			send(progress)
			source, matchTitle, scrapeErr = executeAIMetadataTarget(
				c.Request.Context(), target, *aiCfg, body.Lang, body.UpdateTitle, body.SkipCover,
			)
		} else {
			source, matchTitle, scrapeErr = executeStandardMetadataTarget(
				c.Request.Context(), target, body.Lang, body.UpdateTitle, body.SkipCover,
			)
		}
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
		send(progress)
	}
	send(gin.H{"type": "complete", "total": len(targets), "success": success, "failed": failed})
}

// POST /api/metadata/clear — 清除选中项的元数据
func (h *MetadataHandler) ClearMetadata(c *gin.Context) {
	var body struct {
		ComicIDs []string `json:"comicIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.ComicIDs) == 0 {
		c.JSON(400, gin.H{"error": "comicIds array required"})
		return
	}

	cleared := 0
	for _, id := range body.ComicIDs {
		err := store.UpdateComicFields(id, map[string]interface{}{
			"author":         "",
			"publisher":      "",
			"description":    "",
			"genre":          "",
			"language":       "",
			"metadataSource": "",
			"coverImageUrl":  "",
		})
		if err == nil {
			cleared++
		}
	}

	c.JSON(200, gin.H{"success": true, "cleared": cleared})
}

// POST /api/metadata/batch-rename — 批量重命名书籍标题
