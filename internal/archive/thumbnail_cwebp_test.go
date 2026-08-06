package archive

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResizeWithCwebpStreamsOriginalImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake cwebp shell executable is only used on Unix test runners")
	}

	dir := t.TempDir()
	capturePath := filepath.Join(dir, "captured-input")
	executablePath := filepath.Join(dir, "fake-cwebp")
	script := "#!/bin/sh\ncat > \"$CAPTURE_PATH\"\nprintf 'fake-webp-output'\n"
	if err := os.WriteFile(executablePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake cwebp: %v", err)
	}
	t.Setenv("CAPTURE_PATH", capturePath)

	img := image.NewRGBA(image.Rect(0, 0, 32, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 180, G: 90, B: 30, A: 255})
		}
	}
	var source bytes.Buffer
	if err := jpeg.Encode(&source, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source JPEG: %v", err)
	}

	output, err := resizeWithCwebp(executablePath, source.Bytes(), 400, 560, 80)
	if err != nil {
		t.Fatalf("resize with fake cwebp: %v", err)
	}
	if string(output) != "fake-webp-output" {
		t.Fatalf("output = %q", output)
	}

	captured, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured input: %v", err)
	}
	if !bytes.Equal(captured, source.Bytes()) {
		t.Fatal("cwebp input was transformed before being streamed; large JPEGs must remain compressed")
	}
}
