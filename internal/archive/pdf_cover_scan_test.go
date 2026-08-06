package archive

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFirstEmbeddedJPEGLargePDF(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "large-manga.pdf")

	tiny := encodeSolidJPEG(t, 64, 64)
	cover := encodeSolidJPEG(t, 1200, 1800)

	file, err := os.Create(pdfPath)
	if err != nil {
		t.Fatalf("create PDF fixture: %v", err)
	}
	if _, err := file.WriteString("%PDF-1.7\n"); err != nil {
		t.Fatalf("write PDF header: %v", err)
	}
	if _, err := file.Write(tiny); err != nil {
		t.Fatalf("write tiny JPEG: %v", err)
	}
	if _, err := file.Write(make([]byte, 2<<20)); err != nil {
		t.Fatalf("write PDF padding: %v", err)
	}
	if _, err := file.Write(cover); err != nil {
		t.Fatalf("write cover JPEG: %v", err)
	}
	// 模拟 40 MB 以上的漫画 PDF；封面仍位于文件前部，扫描器应提前返回。
	if err := file.Truncate(45 << 20); err != nil {
		t.Fatalf("truncate PDF fixture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close PDF fixture: %v", err)
	}

	extracted, err := ExtractFirstEmbeddedJPEG(pdfPath)
	if err != nil {
		t.Fatalf("extract embedded cover: %v", err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(extracted))
	if err != nil {
		t.Fatalf("decode extracted JPEG config: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	if config.Width != 1200 || config.Height != 1800 {
		t.Fatalf("dimensions = %dx%d, want 1200x1800", config.Width, config.Height)
	}
}

func TestExtractFirstEmbeddedJPEGRejectsTinyImages(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "icons-only.pdf")
	data := append([]byte("%PDF-1.7\n"), encodeSolidJPEG(t, 64, 64)...)
	if err := os.WriteFile(pdfPath, data, 0o644); err != nil {
		t.Fatalf("write PDF fixture: %v", err)
	}

	if _, err := ExtractFirstEmbeddedJPEG(pdfPath); err == nil {
		t.Fatal("expected tiny embedded JPEG to be rejected")
	}
}

func encodeSolidJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 80, G: 120, B: 180, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return buffer.Bytes()
}
