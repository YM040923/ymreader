package service

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

const maxCoverDownloadBytes = 20 << 20

var (
	seriesCoverDownload sync.Map // seriesID -> chan struct{}
	workCoverDownload   sync.Map // workID -> chan struct{}
)

// ============================================================
// Apply metadata to comic
// ============================================================

// BuildRatingUpdates 构建外部评分的更新字段 map，避免在多处重复相同的逻辑。
// 如果 metadata 中没有评分信息，返回 nil。
func BuildRatingUpdates(meta ComicMetadata) map[string]interface{} {
	if meta.ExternalRating == nil {
		return nil
	}
	updates := map[string]interface{}{
		"externalRating":       *meta.ExternalRating,
		"externalRatingSource": meta.ExternalRatingSource,
	}
	if meta.ExternalRatingMax != nil {
		updates["externalRatingMax"] = *meta.ExternalRatingMax
	}
	if meta.ExternalRatingUpdatedAt != nil {
		updates["externalRatingUpdatedAt"] = *meta.ExternalRatingUpdatedAt
	} else {
		updates["externalRatingUpdatedAt"] = time.Now().UTC()
	}
	return updates
}

// ApplyMetadata updates comic fields in DB from metadata.
func ApplyMetadata(comicID string, meta ComicMetadata, lang string, overwrite bool, opts ...ApplyOption) (*store.ComicListItem, error) {
	// 解析可选参数
	opt := ApplyOption{}
	if len(opts) > 0 {
		opt = opts[0]
	}

	existing, err := store.GetComicByID(comicID)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("comic not found: %s", comicID)
	}

	updates := map[string]interface{}{}

	shouldUpdate := func(current string) bool {
		return overwrite || current == ""
	}

	if meta.Title != "" && shouldUpdate(existing.Title) {
		updates["title"] = meta.Title
	}
	if meta.Author != "" && shouldUpdate(existing.Author) {
		updates["author"] = meta.Author
	}
	if meta.Publisher != "" && shouldUpdate(existing.Publisher) {
		updates["publisher"] = meta.Publisher
	}
	if meta.Description != "" && shouldUpdate(existing.Description) {
		updates["description"] = meta.Description
	}
	if meta.Language != "" && shouldUpdate(existing.Language) {
		updates["language"] = meta.Language
	}
	if meta.Genre != "" && shouldUpdate(existing.Genre) {
		updates["genre"] = meta.Genre
	}
	if meta.Year != nil {
		if overwrite || existing.Year == nil {
			updates["year"] = *meta.Year
		}
	}
	if meta.Source != "" && !opt.PreserveMetadataSource {
		updates["metadataSource"] = meta.Source
	}
	// P2-A: 当 skipCover 为 true 时，跳过封面更新
	if meta.CoverURL != "" && !opt.SkipCover {
		updates["coverImageUrl"] = meta.CoverURL
	}
	// External rating
	for k, v := range BuildRatingUpdates(meta) {
		updates[k] = v
	}

	if len(updates) > 0 {
		if err := store.UpdateComicFields(comicID, updates); err != nil {
			return nil, fmt.Errorf("update comic fields: %w", err)
		}
	}

	// Download cover image as thumbnail（仅在不跳过封面时）。
	// 先清理旧缓存，避免前端在后台下载完成前继续命中旧封面。
	if meta.CoverURL != "" && !opt.SkipCover {
		archive.ClearThumbnailCache(comicID)
		go func() {
			if err := cacheCoverAsThumbnail(comicID, meta.CoverURL); err != nil {
				log.Printf("[metadata] Cover cache failed for %s: %v", comicID, err)
			}
		}()
	}

	// Add genres as tags
	if meta.Genre != "" {
		genres := strings.Split(meta.Genre, ",")
		var tagNames []string
		for _, g := range genres {
			g = strings.TrimSpace(g)
			if g != "" {
				tagNames = append(tagNames, g)
			}
		}
		if len(tagNames) > 0 {
			_ = store.AddTagsToComic(comicID, tagNames)
		}
	}

	return store.GetComicByID(comicID)
}

// downloadCoverAsThumbnail fetches a cover URL and saves as WebP thumbnail.
func downloadCoverAsThumbnail(comicID, coverURL string) {
	if err := cacheCoverAsThumbnail(comicID, coverURL); err != nil {
		log.Printf("[metadata] Cover cache failed for %s: %v", comicID, err)
	}
}

func cacheCoverAsThumbnail(comicID, coverURL string) error {
	// Bangumi 等源可能返回 http:// URL，Go HTTP 客户端会跟随重定向，
	// 但显式转为 https 更安全
	coverURL = strings.Replace(coverURL, "http://", "https://", 1)

	thumbDir := config.GetThumbnailsDir()
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "NowenReader/1.0")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if err != nil {
			return err
		}
		return fmt.Errorf("download cover returned HTTP %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	imgData, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverDownloadBytes+1))
	if err != nil || len(imgData) == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("empty cover response")
	}
	if len(imgData) > maxCoverDownloadBytes {
		return fmt.Errorf("cover download too large")
	}

	thumbPath := filepath.Join(thumbDir, archive.ThumbnailCacheName(comicID))
	webpData, _, err := archive.ResizeImageToWebP(imgData, config.GetThumbnailWidth(), config.GetThumbnailHeight(), 85)
	if err != nil {
		return fmt.Errorf("convert cover: %w", err)
	}
	archive.ClearThumbnailCache(comicID)
	if err := os.WriteFile(thumbPath, webpData, 0644); err != nil {
		return err
	}
	log.Printf("[metadata] Cover cached for %s", comicID)
	return nil
}

func DownloadSeriesCover(seriesID, coverURL string) {
	if seriesID == "" || coverURL == "" {
		return
	}
	coverURL = strings.Replace(coverURL, "http://", "https://", 1)
	if err := store.UpdateSeriesMetadata(seriesID, store.SeriesMetadataUpdate{CoverURL: &coverURL}); err != nil {
		log.Printf("[metadata] Series cover URL save failed for %s: %v", seriesID, err)
		return
	}

	thumbDir := config.GetThumbnailsDir()
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return
	}
	cachePath := filepath.Join(thumbDir, archive.SeriesCoverCacheName(seriesID))
	ch, loaded := seriesCoverDownload.LoadOrStore(seriesID, make(chan struct{}))
	done := ch.(chan struct{})
	if loaded {
		<-done
		return
	}
	defer func() {
		close(done)
		seriesCoverDownload.Delete(seriesID)
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, coverURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "NowenReader/1.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	defer resp.Body.Close()
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverDownloadBytes+1))
	if err != nil || len(imgData) == 0 || len(imgData) > maxCoverDownloadBytes {
		return
	}
	if imageConfig, _, decodeErr := image.DecodeConfig(bytes.NewReader(imgData)); decodeErr == nil && imageConfig.Height > 0 {
		aspectRatio := float64(imageConfig.Width) / float64(imageConfig.Height)
		_ = store.UpdateSeriesMetadata(seriesID, store.SeriesMetadataUpdate{CoverAspectRatio: &aspectRatio})
	}
	webpData, _, err := archive.ResizeImageToWebP(imgData, config.GetThumbnailWidth(), config.GetThumbnailHeight(), 85)
	if err != nil {
		return
	}
	archive.ClearSeriesCoverCache(seriesID)
	if err := os.WriteFile(cachePath, webpData, 0644); err != nil {
		return
	}
	log.Printf("[metadata] Series cover cached locally for %s", seriesID)
}

func DownloadWorkCover(workID, coverURL string) {
	if workID == "" || coverURL == "" {
		return
	}
	coverURL = strings.Replace(coverURL, "http://", "https://", 1)
	if err := store.UpdateLogicalWorkCover(workID, store.LogicalWorkCoverUpdate{CoverURL: &coverURL}); err != nil {
		log.Printf("[metadata] Work cover URL save failed for %s: %v", workID, err)
		return
	}

	thumbDir := config.GetThumbnailsDir()
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return
	}
	cachePath := filepath.Join(thumbDir, archive.WorkCoverCacheName(workID))
	channel, loaded := workCoverDownload.LoadOrStore(workID, make(chan struct{}))
	done := channel.(chan struct{})
	if loaded {
		<-done
		return
	}
	defer func() {
		close(done)
		workCoverDownload.Delete(workID)
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequest(http.MethodGet, coverURL, nil)
	if err != nil {
		return
	}
	request.Header.Set("User-Agent", "NowenReader/1.0")
	response, err := client.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			response.Body.Close()
		}
		return
	}
	defer response.Body.Close()
	imageData, err := io.ReadAll(io.LimitReader(response.Body, maxCoverDownloadBytes+1))
	if err != nil || len(imageData) == 0 || len(imageData) > maxCoverDownloadBytes {
		return
	}
	if imageConfig, _, decodeErr := image.DecodeConfig(bytes.NewReader(imageData)); decodeErr == nil && imageConfig.Height > 0 {
		aspectRatio := float64(imageConfig.Width) / float64(imageConfig.Height)
		_ = store.UpdateLogicalWorkCover(workID, store.LogicalWorkCoverUpdate{CoverAspectRatio: &aspectRatio})
	}
	webpData, _, err := archive.ResizeImageToWebP(
		imageData,
		config.GetThumbnailWidth(),
		config.GetThumbnailHeight(),
		85,
	)
	if err != nil {
		return
	}
	archive.ClearWorkCoverCache(workID)
	if err := os.WriteFile(cachePath, webpData, 0o644); err != nil {
		return
	}
	log.Printf("[metadata] Work cover cached locally for %s", workID)
}

// ============================================================
// HTTP helpers
// ============================================================

// maxRetries429 是遇到 HTTP 429 时的最大重试次数
const maxRetries429 = 3

// retryAfterFromHeader 从 Retry-After 头中解析等待秒数，默认返回 fallback
