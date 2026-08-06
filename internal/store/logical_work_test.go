package store

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func TestLogicalWorkMigrationCreatesPersistentWorkTables(t *testing.T) {
	setupTestDB(t)

	for _, table := range []string{
		"LogicalWork",
		"LogicalWorkTag",
		"LogicalWorkCategory",
		"LogicalWorkAlias",
	} {
		var count int
		if err := DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing persistent Work table %s", table)
		}
	}
}

func TestMigrateComicSeriesToLogicalWorksPreservesMetadataCoverAndRelations(t *testing.T) {
	setupTestDB(t)

	if _, err := DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled")
		VALUES ('logical-library', 'Logical', 'comic', '/logical', 1);
		INSERT INTO "Comic" (
			"id", "filename", "title", "type", "libraryId", "relativePath",
			"author", "publisher", "language", "genre", "coverImageUrl"
		) VALUES
			('logical-ch1', '作品/第001话.cbz', '第一话', 'comic', 'logical-library', '作品/第001话.cbz',
			 '章节作者', '章节出版社', 'zh-CN', '动作', ''),
			('logical-ch2', '作品/第002话.cbz', '第二话', 'comic', 'logical-library', '作品/第002话.cbz',
			 '', '', '', '', '');
		INSERT INTO "ComicSeries" (
			"id", "libraryId", "rootRelativePath", "title", "sortTitle",
			"coverComicId", "coverUrl", "coverAspectRatio", "author", "description",
			"year", "publisher", "language", "genre", "status", "metadataSource",
			"externalRating", "externalRatingMax", "externalRatingSource",
			"metadataLocked", "manualLocked"
		) VALUES (
			'ser_logical', 'logical-library', '作品', '刮削后的作品名', '刮削后的作品名',
			'logical-ch1', 'https://example.test/cover.jpg', 0.7, '作品作者', '作品简介',
			2024, '作品出版社', 'zh-CN', '漫画,动作', 'ongoing', 'bangumi',
			8.8, 10, 'bangumi', 1, 0
		);
		INSERT INTO "ComicSeriesItem" ("seriesId", "comicId", "sortIndex", "displayLabel") VALUES
			('ser_logical', 'logical-ch1', 0, '第001话'),
			('ser_logical', 'logical-ch2', 1, '第002话');
		INSERT INTO "Tag" ("name") VALUES ('作品标签');
		INSERT INTO "ComicSeriesTag" ("seriesId", "tagId")
			SELECT 'ser_logical', "id" FROM "Tag" WHERE "name" = '作品标签';
		INSERT INTO "Category" ("name", "slug") VALUES ('国漫', 'mainland');
		INSERT INTO "ComicCategory" ("comicId", "categoryId")
			SELECT 'logical-ch1', "id" FROM "Category" WHERE "slug" = 'mainland';
	`); err != nil {
		t.Fatal(err)
	}

	if err := MigrateComicSeriesToLogicalWorks(); err != nil {
		t.Fatalf("MigrateComicSeriesToLogicalWorks failed: %v", err)
	}

	workID := workmodel.StableWorkID("logical-library", "作品")
	work, err := GetLogicalWork(workID)
	if err != nil {
		t.Fatal(err)
	}
	if work == nil {
		t.Fatal("migrated LogicalWork not found")
	}
	if work.Title != "刮削后的作品名" || work.Author != "作品作者" || work.Description != "作品简介" {
		t.Fatalf("series metadata was not preserved: %+v", work)
	}
	if work.CoverURL != "https://example.test/cover.jpg" || work.CoverComicID != "logical-ch1" || work.CoverAspectRatio != 0.7 {
		t.Fatalf("series cover was not preserved: %+v", work)
	}
	if !work.MetadataLocked || !work.CoverLocked {
		t.Fatalf("migrated locked metadata/cover must remain protected: %+v", work)
	}

	alias, err := ResolveLogicalWorkAlias("comic-series", "ser_logical")
	if err != nil {
		t.Fatal(err)
	}
	if alias != workID {
		t.Fatalf("series alias = %q, want %q", alias, workID)
	}
	tags, err := GetLogicalWorkTags(workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "作品标签" {
		t.Fatalf("migrated tags = %+v", tags)
	}
	categories, err := GetLogicalWorkCategories(workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Slug != "mainland" {
		t.Fatalf("migrated categories = %+v", categories)
	}

	if err := MigrateComicSeriesToLogicalWorks(); err != nil {
		t.Fatalf("idempotent migration failed: %v", err)
	}
}

func TestUpsertDetectedLogicalWorkDoesNotOverwriteLockedMetadataOrCover(t *testing.T) {
	setupTestDB(t)
	if _, err := DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled")
		VALUES ('upsert-library', 'Upsert', 'comic', '/upsert', 1);
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath") VALUES
			('auto-cover', '作品/第001话.cbz', '第一话', 'comic', 'upsert-library', '作品/第001话.cbz'),
			('new-auto-cover', '作品/第002话.cbz', '第二话', 'comic', 'upsert-library', '作品/第002话.cbz');
	`); err != nil {
		t.Fatal(err)
	}

	workID := workmodel.StableWorkID("upsert-library", "作品")
	if err := UpsertDetectedLogicalWorks([]LogicalWorkSeed{{
		ID: "work_wrong_id", LibraryID: "upsert-library", RootPath: "作品",
		Title: "扫描标题", CoverComicID: "auto-cover",
	}}); err != nil {
		t.Fatal(err)
	}
	manualTitle := "手工标题"
	manualAuthor := "手工作者"
	locked := true
	if err := UpdateLogicalWorkMetadata(workID, LogicalWorkMetadataUpdate{
		Title: &manualTitle, Author: &manualAuthor, MetadataLocked: &locked,
	}); err != nil {
		t.Fatal(err)
	}
	manualCover := "https://example.test/manual.jpg"
	if err := UpdateLogicalWorkCover(workID, LogicalWorkCoverUpdate{
		CoverURL: &manualCover, CoverLocked: &locked, CoverSource: stringPointer("uploaded"),
	}); err != nil {
		t.Fatal(err)
	}

	if err := UpsertDetectedLogicalWorks([]LogicalWorkSeed{{
		ID: workID, LibraryID: "upsert-library", RootPath: "作品",
		Title: "新的扫描标题", CoverComicID: "new-auto-cover",
	}}); err != nil {
		t.Fatal(err)
	}
	work, err := GetLogicalWork(workID)
	if err != nil {
		t.Fatal(err)
	}
	if work.Title != manualTitle || work.Author != manualAuthor {
		t.Fatalf("locked metadata was overwritten: %+v", work)
	}
	if work.CoverURL != manualCover || work.CoverSource != "uploaded" || !work.CoverLocked {
		t.Fatalf("locked cover was overwritten: %+v", work)
	}
}

func stringPointer(value string) *string { return &value }
