package store

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
)

func TestRecordOnlyDeletionTombstoneBlocksRescan(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	lib := &model.Library{
		ID:            "ignore-test-library",
		Name:          "Ignore Test Library",
		Type:          "novel",
		RootPath:      t.TempDir(),
		RootPaths:     []string{},
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := CreateLibrary(lib); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	const (
		comicID     = "ignored-epub"
		relativePath = "books/Robinson Crusoe.epub"
	)
	insertComic := func(id string) error {
		_, err := db.Exec(`
			INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath")
			VALUES (?, ?, ?, 'novel', ?, ?)
		`, id, relativePath, "Robinson Crusoe", lib.ID, relativePath)
		return err
	}

	if err := insertComic(comicID); err != nil {
		t.Fatalf("insert initial comic: %v", err)
	}
	identities, err := GetComicSourceIdentities([]string{comicID})
	if err != nil {
		t.Fatalf("GetComicSourceIdentities failed: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("identities length = %d, want 1", len(identities))
	}
	if identities[0].RelativePath != relativePath {
		t.Fatalf("relative path = %q, want %q", identities[0].RelativePath, relativePath)
	}

	if err := AddIgnoredLibraryContents(identities); err != nil {
		t.Fatalf("AddIgnoredLibraryContents failed: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM "Comic" WHERE "id" = ?`, comicID); err != nil {
		t.Fatalf("delete comic row: %v", err)
	}

	// The scanner uses INSERT ... ON CONFLICT DO NOTHING. The BEFORE INSERT
	// tombstone trigger must ignore the row even though the physical file still
	// exists and the generated ID may be new.
	if err := insertComic("rescanned-epub"); err != nil {
		t.Fatalf("rescanned insert returned error: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Comic" WHERE "libraryId" = ? AND "relativePath" = ?`, lib.ID, relativePath).Scan(&count); err != nil {
		t.Fatalf("count blocked rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rescanned row count = %d, want 0", count)
	}

	if err := RemoveIgnoredLibraryContents(identities); err != nil {
		t.Fatalf("RemoveIgnoredLibraryContents failed: %v", err)
	}
	if err := insertComic("restored-epub"); err != nil {
		t.Fatalf("insert after removing tombstone: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Comic" WHERE "libraryId" = ? AND "relativePath" = ?`, lib.ID, relativePath).Scan(&count); err != nil {
		t.Fatalf("count restored rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored row count = %d, want 1", count)
	}
}

func TestNormalizeIgnoredRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "windows separators", input: `books\\novel.epub`, want: "books/novel.epub"},
		{name: "folder marker", input: `images/chapter/`, want: "images/chapter/"},
		{name: "clean segments", input: `books/./classic/../novel.epub`, want: "books/novel.epub"},
		{name: "absolute", input: `/etc/passwd`, wantErr: true},
		{name: "parent traversal", input: `../outside.epub`, wantErr: true},
		{name: "empty", input: `   `, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeIgnoredRelativePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeIgnoredRelativePath(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeIgnoredRelativePath(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeIgnoredRelativePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
