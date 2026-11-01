package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestManualWorkDeleteSpansDirectories(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delete-scope.db")
	if err := store.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	t.Cleanup(func() {
		store.CloseDB()
		_ = os.Remove(dbPath)
	})
	if _, err := store.DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled")
		VALUES ('delete-scope-lib', 'delete-scope-lib', 'comic', '/scope', 1);
		INSERT INTO "Comic" ("id", "filename", "title", "libraryId", "relativePath") VALUES
			('scope-a', 'A/01.cbz', 'A', 'delete-scope-lib', 'A/01.cbz'),
			('scope-b', 'B/01.cbz', 'B', 'delete-scope-lib', 'B/01.cbz');
		INSERT INTO "LogicalWork" ("id", "libraryId", "rootPath", "title", "sortTitle")
		VALUES ('work_manual_scope', 'delete-scope-lib', 'A', 'Scope', 'Scope');
	`); err != nil {
		t.Fatalf("seed delete scope: %v", err)
	}

	spans, err := ManualWorkDeleteSpansDirectories([]string{"scope-a", "scope-b"})
	if err != nil {
		t.Fatalf("manual scope check failed: %v", err)
	}
	if spans {
		t.Fatal("independent Works must not require expanded delete confirmation")
	}

	if _, err := store.DB().Exec(`
		INSERT INTO "WorkMembership" ("comicId", "workId", "libraryId") VALUES
			('scope-a', 'work_manual_scope', 'delete-scope-lib'),
			('scope-b', 'work_manual_scope', 'delete-scope-lib')
	`); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	spans, err = ManualWorkDeleteSpansDirectories([]string{"scope-a", "scope-b"})
	if err != nil {
		t.Fatalf("manual scope check failed: %v", err)
	}
	if !spans {
		t.Fatal("a merged manual Work across directories must require confirmation")
	}
}
