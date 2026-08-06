package handler

import (
	"archive/zip"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestDeletePhysicalEpubClosesPooledReader(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)

	root := t.TempDir()
	const filename = "Robinson Crusoe.epub"
	filePath := filepath.Join(root, filename)
	writeMinimalEpub(t, filePath)

	library := &model.Library{
		ID:            "physical-delete-library",
		Name:          "Physical Delete Library",
		Type:          "novel",
		RootPath:      root,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	comicID := store.PathToID(library.ID, filename)
	if _, err := store.DB().Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
		VALUES (?, ?, 'Robinson Crusoe', 'novel', ?, ?)
	`, comicID, filename, library.ID, filename); err != nil {
		t.Fatalf("insert comic: %v", err)
	}

	// Opening the book populates the service reader pool. On Windows the
	// underlying zip.ReadCloser keeps the EPUB locked until explicitly closed.
	pages, err := service.GetComicPagesEx(comicID)
	if err != nil {
		t.Fatalf("GetComicPagesEx failed: %v", err)
	}
	if len(pages.Entries) != 1 {
		t.Fatalf("chapter count = %d, want 1", len(pages.Entries))
	}

	w := performAuthedRequest(r, http.MethodDelete, "/api/comics/"+comicID+"?deleteFiles=true", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("physical delete: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("physical EPUB still exists after delete: %v", err)
	}
	comic, err := store.GetComicByID(comicID)
	if err != nil {
		t.Fatalf("GetComicByID failed: %v", err)
	}
	if comic != nil {
		t.Fatal("database row still exists after physical delete")
	}
}

func writeMinimalEpub(t *testing.T, target string) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatalf("create epub: %v", err)
	}
	writer := zip.NewWriter(file)

	writeEntry := func(name, content string) {
		t.Helper()
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create epub entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write epub entry %s: %v", name, err)
		}
	}

	writeEntry("mimetype", "application/epub+zip")
	writeEntry("META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)
	writeEntry("OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Robinson Crusoe</dc:title></metadata>
  <manifest><item id="chapter1" href="chapter1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="chapter1"/></spine>
</package>`)
	writeEntry("OEBPS/chapter1.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 1</title></head><body><h1>Chapter 1</h1><p>Content</p></body></html>`)

	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatalf("close epub writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close epub file: %v", err)
	}
}
