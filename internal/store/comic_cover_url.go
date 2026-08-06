package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
)

// BuildComicCoverURL 构造漫画缩略图 URL。
//
// 实现说明：
//   - 当 thumbnail 缓存文件存在时，附带其 ModTime 作为查询参数（?v=<unix>），
//     这样用户在卷详情页"使用内页作为封面"覆盖缓存文件后，URL 会自动变化，
//     从而绕过浏览器/Service Worker 缓存，避免封面已更新但列表页仍显示旧占位图的问题。
//   - 当文件不存在或 stat 失败时，返回不带版本号的基础 URL，由 thumbnail 接口按需生成。
func BuildComicCoverURL(comicID string) string {
	base := config.JoinBasePath(fmt.Sprintf("/api/comics/%s/thumbnail", comicID))
	tw := config.GetThumbnailWidth()
	th := config.GetThumbnailHeight()
	cacheName := fmt.Sprintf("%s_%dx%d.webp", comicID, tw, th)
	cachePath := filepath.Join(config.GetThumbnailsDir(), cacheName)
	if info, err := os.Stat(cachePath); err == nil {
		return fmt.Sprintf("%s?v=%d", base, info.ModTime().Unix())
	}
	return base
}

func BuildSeriesCoverURL(seriesID string) string {
	coverID := "series_" + seriesID
	base := config.JoinBasePath(fmt.Sprintf("/api/comics/%s/thumbnail", coverID))
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.SeriesCoverCacheName(seriesID))
	if info, err := os.Stat(cachePath); err == nil {
		return fmt.Sprintf("%s?v=%d", base, info.ModTime().Unix())
	}
	return base
}

func BuildLogicalWorkCoverURL(workID string) string {
	base := config.JoinBasePath(fmt.Sprintf("/api/opds/work-cover/%s", workID))
	cachePath := filepath.Join(config.GetThumbnailsDir(), archive.WorkCoverCacheName(workID))
	if info, err := os.Stat(cachePath); err == nil {
		return fmt.Sprintf("%s?v=%d", base, info.ModTime().Unix())
	}
	return base
}
