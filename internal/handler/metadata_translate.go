package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func metadataTranslationFields(target metadataTarget) map[string]string {
	fields := make(map[string]string)
	if target.EntityType == "work" {
		if target.LogicalWork != nil {
			if target.LogicalWork.Title != "" {
				fields["title"] = target.LogicalWork.Title
			}
			if target.LogicalWork.Description != "" {
				fields["description"] = target.LogicalWork.Description
			}
			if target.LogicalWork.Genre != "" {
				fields["genre"] = target.LogicalWork.Genre
			}
			if target.LogicalWork.Publisher != "" {
				fields["publisher"] = target.LogicalWork.Publisher
			}
			if target.LogicalWork.Author != "" {
				fields["author"] = target.LogicalWork.Author
			}
			return fields
		}
		if target.Title != "" {
			fields["title"] = target.Title
		}
		return fields
	}
	if target.Comic == nil {
		return fields
	}
	if target.Comic.Title != "" {
		fields["title"] = target.Comic.Title
	}
	if target.Comic.Description != "" {
		fields["description"] = target.Comic.Description
	}
	if target.Comic.Genre != "" {
		fields["genre"] = target.Comic.Genre
	}
	if target.Comic.Publisher != "" {
		fields["publisher"] = target.Comic.Publisher
	}
	if target.Comic.Author != "" {
		fields["author"] = target.Comic.Author
	}
	return fields
}

func applyMetadataTranslation(target metadataTarget, translated map[string]string) error {
	if target.EntityType == "work" {
		update := store.LogicalWorkMetadataUpdate{}
		for key, value := range translated {
			value := strings.TrimSpace(value)
			if value == "" {
				continue
			}
			switch key {
			case "title":
				update.Title = &value
			case "author":
				update.Author = &value
			case "publisher":
				update.Publisher = &value
			case "description":
				update.Description = &value
			case "genre":
				update.Genre = &value
			case "language":
				update.Language = &value
			case "status":
				update.Status = &value
			case "year":
				if parsed, err := strconv.Atoi(value); err == nil {
					update.Year = &parsed
				}
			}
		}
		return store.UpdateLogicalWorkMetadata(target.EntityID, update)
	}

	updates := make(map[string]interface{}, len(translated))
	for key, value := range translated {
		if strings.TrimSpace(value) != "" {
			updates[key] = strings.TrimSpace(value)
		}
	}
	return store.UpdateComicFields(target.EntityID, updates)
}

func translateMetadataTarget(target metadataTarget, targetLang string, engine service.TranslateEngine) (*service.TranslateFieldsResult, error) {
	fields := metadataTranslationFields(target)
	if len(fields) == 0 {
		return nil, nil
	}
	return service.TranslateMetadataFieldsMultiEngine(fields, targetLang, engine)
}

// POST /api/works/:id/translate-metadata
func (h *MetadataHandler) TranslateWork(c *gin.Context) {
	var body struct {
		TargetLang string                  `json:"targetLang"`
		Engine     service.TranslateEngine `json:"engine"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.TargetLang) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetLang is required"})
		return
	}
	targets, err := discoverMetadataTargets(c, []metadataTargetSelection{{
		ID: c.Param("id"), EntityType: "work",
	}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve Work"})
		return
	}
	if len(targets) != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
		return
	}
	result, err := translateMetadataTarget(targets[0], body.TargetLang, body.Engine)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Translation failed: " + err.Error()})
		return
	}
	if result == nil || len(result.Fields) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"translated": false,
			"fields":     gin.H{},
			"engine":     "",
		})
		return
	}
	if err := applyMetadataTranslation(targets[0], result.Fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Work"})
		return
	}
	updated, _ := store.GetLogicalWork(targets[0].EntityID)
	c.JSON(http.StatusOK, gin.H{
		"work":       updated,
		"translated": true,
		"fields":     result.Fields,
		"engine":     string(result.Engine),
		"cached":     result.Cached,
	})
}
