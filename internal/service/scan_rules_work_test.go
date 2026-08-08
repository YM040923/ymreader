package service

import (
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestBuildWorkAwareDirectoryOrganizeRelPathKeepsStandaloneArchiveFlat(t *testing.T) {
	got := buildWorkAwareDirectoryOrganizeRelPath(
		"国漫/恶魔X天使 不能友好相处.zip",
		"国漫/恶魔X天使 不能友好相处",
		"smartDir",
	)
	if want := "国漫/恶魔X天使 不能友好相处.zip"; got != want {
		t.Fatalf("standalone archive target = %q, want %q", got, want)
	}
}

func TestBuildWorkAwareDirectoryOrganizeRelPathGroupsPhysicalUnitsUnderWork(t *testing.T) {
	got := buildWorkAwareDirectoryOrganizeRelPath(
		"日漫/败犬女主太多了/败犬女主太多了 - Vol. 002.cbz",
		"日漫/败犬女主太多了",
		"smartDir",
	)
	if want := "日漫/败犬女主太多了/败犬女主太多了 - Vol. 002.cbz"; got != want {
		t.Fatalf("volume target = %q, want %q", got, want)
	}
}

func TestBuildWorkAwareDirectoryOrganizeRelPathMovesRootLevelUnitsIntoWork(t *testing.T) {
	got := buildWorkAwareDirectoryOrganizeRelPath(
		"败犬女主太多了 - Vol. 002.cbz",
		"败犬女主太多了",
		"smartDir",
	)
	if want := "败犬女主太多了/败犬女主太多了 - Vol. 002.cbz"; got != want {
		t.Fatalf("root volume target = %q, want %q", got, want)
	}
}

func TestApplyInferredToLogicalWorkDoesNotRewritePhysicalComicMetadata(t *testing.T) {
	setupTestDB(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled", "scanEnabled", "createdAt", "updatedAt")
		VALUES ('library', 'Library', 'comic', '/library', 1, 1, ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		LibraryID:   "library",
		RootPath:    "日漫/作品",
		ContentType: "comic",
		Title:       "作品",
	}}); err != nil {
		t.Fatal(err)
	}
	workID := stableWorkID("library", "日漫/作品")
	// LogicalWork uses the shared Work stable-ID implementation, not Comic IDs.
	logical, err := store.GetLogicalWork(workID)
	if err != nil || logical == nil {
		t.Fatalf("logical work = %#v err=%v", logical, err)
	}
	applied, err := applyInferredToLogicalWork(Work{
		ID:       logical.ID,
		Title:    "作品",
		RootPath: "日漫/作品",
	}, &InferredTitleStructure{
		Title:      "作品 中文名",
		Author:     "作者",
		Language:   "zh",
		Confidence: "high",
	}, true, false)
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	updated, err := store.GetLogicalWork(logical.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "作品 中文名" || updated.Author != "作者" ||
		updated.MetadataSource != "ai_scan_rules" {
		t.Fatalf("updated logical work = %#v", updated)
	}
}

func TestApplyInferredToLogicalWorkPreservesExistingTitleUnlessOverwriteEnabled(t *testing.T) {
	setupTestDB(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled", "scanEnabled", "createdAt", "updatedAt")
		VALUES ('library', 'Library', 'comic', '/library', 1, 1, ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		LibraryID:   "library",
		RootPath:    "日漫/作品",
		ContentType: "comic",
		Title:       "原始标题",
	}}); err != nil {
		t.Fatal(err)
	}
	workID := stableWorkID("library", "日漫/作品")
	applied, err := applyInferredToLogicalWork(Work{
		ID:       workID,
		Title:    "原始标题",
		RootPath: "日漫/作品",
	}, &InferredTitleStructure{
		Title:      "AI 标题",
		Author:     "作者",
		Confidence: "high",
	}, false, false)
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	updated, err := store.GetLogicalWork(workID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "原始标题" || updated.Author != "作者" {
		t.Fatalf("updated logical work = %#v", updated)
	}
}
