package handler

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func (h *WorkHandler) ScrapeMetadata(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	var body struct {
		Query   string   `json:"query"`
		Sources []string `json:"sources"`
		Lang    string   `json:"lang"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Query == "" {
		body.Query = targets[0].Work.Title
	}
	if body.Lang == "" {
		body.Lang = "zh"
	}
	body.Sources = filterSeriesMetadataSources(body.Sources, "comic")
	results := service.SearchMetadata(body.Query, body.Sources, body.Lang, "comic")
	if results == nil {
		results = []service.ComicMetadata{}
	}
	c.JSON(http.StatusOK, gin.H{"results": results, "detectedContentType": "comic"})
}

func (h *WorkHandler) ApplyScrapedMetadata(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	target := targets[0]
	persisted, err := store.GetLogicalWork(target.Work.ID)
	if err != nil || persisted == nil {
		writeWorkMutationError(c, firstError(err, errors.New("logical Work not found")))
		return
	}
	var body struct {
		Metadata      service.ComicMetadata `json:"metadata"`
		Fields        []string              `json:"fields"`
		Overwrite     bool                  `json:"overwrite"`
		SyncTags      bool                  `json:"syncTags"`
		SyncToVolumes bool                  `json:"syncToVolumes"`
		SyncRating    bool                  `json:"syncRating"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	fields := make(map[string]bool, len(body.Fields))
	for _, field := range body.Fields {
		fields[field] = true
	}
	applyAll := len(fields) == 0
	shouldApply := func(field string) bool { return applyAll || fields[field] }
	meta := body.Metadata
	update := store.LogicalWorkMetadataUpdate{}
	changed := false

	if meta.Title != "" && shouldApply("title") && (body.Overwrite || persisted.Title == "") {
		update.Title = &meta.Title
		changed = true
	}
	if meta.Author != "" && shouldApply("author") && (body.Overwrite || persisted.Author == "") {
		update.Author = &meta.Author
		changed = true
	}
	if meta.Description != "" && shouldApply("description") && (body.Overwrite || persisted.Description == "") {
		update.Description = &meta.Description
		changed = true
	}
	if meta.Genre != "" && shouldApply("genre") && (body.Overwrite || persisted.Genre == "") {
		update.Genre = &meta.Genre
		changed = true
	}
	if meta.Publisher != "" && shouldApply("publisher") && (body.Overwrite || persisted.Publisher == "") {
		update.Publisher = &meta.Publisher
		changed = true
	}
	if meta.Language != "" && shouldApply("language") && (body.Overwrite || persisted.Language == "") {
		update.Language = &meta.Language
		changed = true
	}
	if meta.Year != nil && shouldApply("year") && (body.Overwrite || persisted.Year == nil) {
		update.Year = meta.Year
		changed = true
	}
	if meta.ExternalRating != nil && shouldApply("rating") {
		update.ExternalRating = meta.ExternalRating
		update.ExternalRatingMax = meta.ExternalRatingMax
		update.ExternalRatingSource = &meta.ExternalRatingSource
		now := time.Now().UTC()
		update.ExternalRatingUpdatedAt = &now
		changed = true
	}
	if changed {
		locked := true
		update.MetadataLocked = &locked
		if meta.Source != "" {
			update.MetadataSource = &meta.Source
		}
		if err := store.UpdateLogicalWorkMetadata(target.Work.ID, update); err != nil {
			writeWorkMutationError(c, err)
			return
		}
	}

	if meta.Genre != "" && shouldApply("tags") {
		if err := store.AddLogicalWorkAndComicTags(
			[]string{target.Work.ID},
			target.ComicIDs,
			splitAndTrim(meta.Genre),
		); err != nil {
			writeWorkMutationError(c, err)
			return
		}
	}
	if meta.CoverURL != "" && shouldApply("cover") {
		locked := true
		source := "remote"
		if err := store.UpdateLogicalWorkCover(target.Work.ID, store.LogicalWorkCoverUpdate{
			CoverURL: &meta.CoverURL, CoverSource: &source, CoverLocked: &locked,
		}); err != nil {
			writeWorkMutationError(c, err)
			return
		}
		go service.DownloadWorkCover(target.Work.ID, meta.CoverURL)
	}

	var syncSuccess, syncErrors int
	if body.SyncToVolumes {
		syncSuccess, syncErrors, _ = syncMetadataToComicIDs(
			target.ComicIDs, meta, fields, body.Overwrite, body.SyncRating,
		)
	}
	completeWorkMutation(c, gin.H{
		"success": true, "workId": target.Work.ID,
		"syncSuccess": syncSuccess, "syncErrors": syncErrors,
	})
}

func (h *WorkHandler) AIRecognize(c *gin.Context) {
	targets, ok := h.requireManageTargets(c, []string{c.Param("id")})
	if !ok {
		return
	}
	target := targets[0]
	if len(target.ComicIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Work has no readable units"})
		return
	}
	var body struct {
		Lang       string `json:"lang"`
		TargetLang string `json:"targetLang"`
	}
	_ = c.ShouldBindJSON(&body)
	lang := body.Lang
	if lang == "" {
		lang = body.TargetLang
	}
	if lang == "" {
		lang = "zh"
	}
	cfg := service.LoadAIConfig()
	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not configured"})
		return
	}
	firstComic, err := store.GetComicByID(target.ComicIDs[0])
	if err != nil || firstComic == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "First readable unit is unavailable"})
		return
	}
	var coverData []byte
	if data, _, _, thumbnailErr := service.GetComicThumbnail(firstComic.ID); thumbnailErr == nil {
		coverData = data
	}
	pageImages := make([][]byte, 0, 2)
	for page := 0; page < 2; page++ {
		image, imageErr := service.GetPageImage(firstComic.ID, page)
		if imageErr == nil && image != nil && len(image.Data) > 0 {
			pageImages = append(pageImages, image.Data)
		}
	}
	if len(coverData) == 0 && len(pageImages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to load readable unit images"})
		return
	}
	recognized, err := service.AIRecognizeComicContent(cfg, coverData, pageImages, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI recognition failed: " + err.Error()})
		return
	}
	meta, err := service.AICompleteMetadata(cfg, firstComic.Filename, target.Work.Title, coverData, lang)
	if err != nil {
		log.Printf("[works] AI metadata completion failed for %s: %v", target.Work.ID, err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "recognized": recognized, "metadata": meta})
}

func firstError(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
