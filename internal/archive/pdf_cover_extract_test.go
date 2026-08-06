package archive

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPDFPagePrimaryImageFromJPEGXObject(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "cover.pdf")
	writeJPEGImagePDF(t, pdfPath, 24, 36)

	data, err := ExtractPDFPagePrimaryImage(pdfPath, 0)
	if err != nil {
		t.Fatalf("ExtractPDFPagePrimaryImage() error = %v", err)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode extracted image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	if config.Width != 24 || config.Height != 36 {
		t.Fatalf("size = %dx%d, want 24x36", config.Width, config.Height)
	}
}

func TestExtractPDFPagePrimaryImageRejectsInvalidPage(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "cover.pdf")
	writeJPEGImagePDF(t, pdfPath, 8, 12)

	if _, err := ExtractPDFPagePrimaryImage(pdfPath, 1); err == nil {
		t.Fatal("expected out-of-range page error")
	}
}

func TestDecodePNGPredictorUp(t *testing.T) {
	encoded := []byte{
		2, 10, 20, 30,
		2, 1, 2, 3,
	}
	decoded, err := decodePNGPredictor(encoded, 12, 3, 1, 2)
	if err != nil {
		t.Fatalf("decodePNGPredictor() error = %v", err)
	}
	want := []byte{10, 20, 30, 11, 22, 33}
	if !bytes.Equal(decoded, want) {
		t.Fatalf("decoded = %v, want %v", decoded, want)
	}
}

func writeJPEGImagePDF(t *testing.T, filePath string, width, height int) {
	t.Helper()

	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: 120,
				A: 255,
			})
		}
	}

	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, imageData, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)
	writeObject := func(id int, body func()) {
		offsets[id] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n", id)
		body()
		pdf.WriteString("\nendobj\n")
	}

	writeObject(1, func() {
		pdf.WriteString("<< /Type /Catalog /Pages 2 0 R >>")
	})
	writeObject(2, func() {
		pdf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	})
	writeObject(3, func() {
		fmt.Fprintf(&pdf, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", width, height)
	})
	writeObject(4, func() {
		fmt.Fprintf(&pdf, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n", width, height, jpegData.Len())
		pdf.Write(jpegData.Bytes())
		pdf.WriteString("\nendstream")
	})
	writeObject(5, func() {
		content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", width, height)
		fmt.Fprintf(&pdf, "<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
	})

	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n0 6\n")
	pdf.WriteString("0000000000 65535 f \n")
	for id := 1; id <= 5; id++ {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	if err := os.WriteFile(filePath, pdf.Bytes(), 0o644); err != nil {
		t.Fatalf("write PDF: %v", err)
	}
}
