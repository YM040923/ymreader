package archive

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/config"
)

var pngFileSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func init() {
	thumbnailDir := config.GetThumbnailsDir()
	if _, err := os.Stat(thumbnailDir); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[thumbnail] inspect cache directory failed: %v", err)
		}
		return
	}

	removed, err := clearLegacyPDFTextCoverCache(thumbnailDir)
	if err != nil {
		log.Printf("[thumbnail] recover stale PDF cover cache with warnings: %v", err)
	}
	if removed > 0 {
		log.Printf("[thumbnail] removed %d stale PDF placeholder cover(s); first pages will be regenerated on demand", removed)
	}
}

// clearLegacyPDFTextCoverCache removes the old generated "PDF" placeholder
// images that were cached with a .webp suffix after PDF page rendering failed.
// The normal thumbnail path will render page 0 again on the next request.
func clearLegacyPDFTextCoverCache(thumbnailDir string) (int, error) {
	entries, err := os.ReadDir(thumbnailDir)
	if err != nil {
		return 0, fmt.Errorf("read thumbnail cache: %w", err)
	}

	removed := 0
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".webp") {
			continue
		}

		cachePath := filepath.Join(thumbnailDir, entry.Name())
		data, readErr := os.ReadFile(cachePath)
		if readErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read %s: %w", entry.Name(), readErr)
			}
			continue
		}
		if !isGeneratedPDFTextCover(data) {
			continue
		}
		if removeErr := os.Remove(cachePath); removeErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("remove %s: %w", entry.Name(), removeErr)
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

// isGeneratedPDFTextCover recognizes the deterministic fallback image created
// by generateTextCover("*.pdf", ...). It deliberately checks the PNG signature,
// layout and alpha pattern so normal uploaded/metadata covers are never removed.
func isGeneratedPDFTextCover(data []byte) bool {
	if len(data) < len(pngFileSignature) || !bytes.Equal(data[:len(pngFileSignature)], pngFileSignature) {
		return false
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "png" {
		return false
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	const (
		iconTop    = 120
		iconWidth  = 160
		iconHeight = 200
		glyphScale = 4
		charWidth  = 24
	)
	if width < iconWidth || height <= iconTop+iconHeight {
		return false
	}

	iconLeft := (width - iconWidth) / 2
	textStartX := width/2 - len("PDF")*charWidth/2
	textStartY := iconTop + iconHeight/2 - (7*glyphScale)/2

	alphaAt := func(x, y int) uint8 {
		_, _, _, alpha := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
		return uint8(alpha >> 8)
	}

	// Generated cover structure: opaque background, translucent icon, thin
	// translucent border, and the single supported "P" glyph (D/F are absent).
	if alphaAt(0, 0) != 255 ||
		alphaAt(iconLeft+1, iconTop+1) != 25 ||
		alphaAt(iconLeft, iconTop+10) != 15 {
		return false
	}

	textPixels := [][2]int{
		{textStartX, textStartY},
		{textStartX + 12, textStartY},
		{textStartX, textStartY + 4},
		{textStartX + 16, textStartY + 4},
		{textStartX, textStartY + 12},
		{textStartX, textStartY + 24},
	}
	for _, point := range textPixels {
		if alphaAt(point[0], point[1]) != 180 {
			return false
		}
	}

	blankPixels := [][2]int{
		{textStartX + 16, textStartY},
		{textStartX + 4, textStartY + 4},
		{textStartX + charWidth, textStartY},
		{textStartX + 2*charWidth, textStartY},
	}
	for _, point := range blankPixels {
		if alphaAt(point[0], point[1]) != 25 {
			return false
		}
	}

	return true
}
