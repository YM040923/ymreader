package archive

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestIsGeneratedPDFTextCover(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "pdf.webp")
	pdfData, _, err := generateTextCover("book.pdf", "pdf-book", dir, pdfPath)
	if err != nil {
		t.Fatalf("generate PDF text cover: %v", err)
	}
	if !isGeneratedPDFTextCover(pdfData) {
		t.Fatal("generated PDF fallback was not recognized")
	}

	txtPath := filepath.Join(dir, "txt.webp")
	txtData, _, err := generateTextCover("book.txt", "txt-book", dir, txtPath)
	if err != nil {
		t.Fatalf("generate TXT text cover: %v", err)
	}
	if isGeneratedPDFTextCover(txtData) {
		t.Fatal("TXT fallback was incorrectly recognized as PDF fallback")
	}

	var unrelated bytes.Buffer
	if err := png.Encode(&unrelated, image.NewRGBA(image.Rect(0, 0, 400, 560))); err != nil {
		t.Fatalf("encode unrelated PNG: %v", err)
	}
	if isGeneratedPDFTextCover(unrelated.Bytes()) {
		t.Fatal("unrelated PNG was incorrectly recognized as PDF fallback")
	}
}

func TestClearLegacyPDFTextCoverCache(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "pdf_400x560.webp")
	if _, _, err := generateTextCover("book.pdf", "pdf-book", dir, pdfPath); err != nil {
		t.Fatalf("generate PDF text cover: %v", err)
	}

	txtPath := filepath.Join(dir, "txt_400x560.webp")
	if _, _, err := generateTextCover("book.txt", "txt-book", dir, txtPath); err != nil {
		t.Fatalf("generate TXT text cover: %v", err)
	}

	otherPath := filepath.Join(dir, "other_400x560.webp")
	var unrelated bytes.Buffer
	if err := png.Encode(&unrelated, image.NewRGBA(image.Rect(0, 0, 400, 560))); err != nil {
		t.Fatalf("encode unrelated PNG: %v", err)
	}
	if err := os.WriteFile(otherPath, unrelated.Bytes(), 0o644); err != nil {
		t.Fatalf("write unrelated cover: %v", err)
	}

	removed, err := clearLegacyPDFTextCoverCache(dir)
	if err != nil {
		t.Fatalf("clear cache: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(pdfPath); !os.IsNotExist(err) {
		t.Fatalf("PDF fallback still exists, stat err = %v", err)
	}
	for _, path := range []string{txtPath, otherPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("non-PDF cover %s was removed: %v", filepath.Base(path), err)
		}
	}
}
