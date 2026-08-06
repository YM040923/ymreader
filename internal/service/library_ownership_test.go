package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/model"
)

func TestLibraryOwnershipPrefersDeepestRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "novels")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}

	ownership := NewLibraryOwnership([]model.Library{
		{ID: "parent", Name: "Parent", Type: "mixed", RootPath: root, Enabled: true},
		{ID: "child", Name: "Child", Type: "novel", RootPath: child, Enabled: true},
	})

	parentFile := filepath.Join(root, "comic.cbz")
	childFile := filepath.Join(child, "book.epub")
	if owner, ok := ownership.OwnerForPath(parentFile); !ok || owner.LibraryID != "parent" {
		t.Fatalf("parent file owner = %#v, ok=%v", owner, ok)
	}
	if owner, ok := ownership.OwnerForPath(childFile); !ok || owner.LibraryID != "child" {
		t.Fatalf("child file owner = %#v, ok=%v", owner, ok)
	}
	if ownership.IsOwnedBy("parent", childFile) {
		t.Fatal("parent library must not own content delegated to child root")
	}
}

func TestLibraryOwnershipRejectsExactRootAmbiguity(t *testing.T) {
	root := t.TempDir()
	ownership := NewLibraryOwnership([]model.Library{
		{ID: "one", Name: "One", RootPath: root, Enabled: true},
		{ID: "two", Name: "Two", RootPath: root, Enabled: true},
	})

	if !ownership.RootHasExactConflict(root) {
		t.Fatal("expected exact root conflict")
	}
	if owner, ok := ownership.OwnerForPath(filepath.Join(root, "book.epub")); ok {
		t.Fatalf("ambiguous root unexpectedly resolved to %s", owner.LibraryID)
	}
}

func TestLibraryOwnershipReservesDisabledChild(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "disabled")
	ownership := NewLibraryOwnership([]model.Library{
		{ID: "parent", Name: "Parent", RootPath: root, Enabled: true},
		{ID: "child", Name: "Child", RootPath: child, Enabled: false},
	})

	if owner, ok := ownership.OwnerForPath(filepath.Join(child, "book.epub")); !ok || owner.LibraryID != "child" {
		t.Fatalf("disabled child must keep its ownership boundary: %#v, ok=%v", owner, ok)
	}
}
