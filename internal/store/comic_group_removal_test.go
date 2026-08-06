package store

import "testing"

func TestComicGroupMigrationDropsOnlyComicGroupingTables(t *testing.T) {
	setupTestDB(t)

	for _, table := range []string{
		"ComicGroupItem",
		"ComicGroupSeries",
		"ComicGroupTag",
		"GroupCategory",
		"ComicGroup",
	} {
		var count int
		if err := DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("legacy comic grouping table %s still exists", table)
		}
	}

	for _, table := range []string{
		"ComicSeries",
		"ComicSeriesItem",
		"UserGroup",
		"UserGroupMember",
		"GroupLibraryAccess",
	} {
		var count int
		if err := DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("required table %s was removed", table)
		}
	}

	var legacyLogColumnCount int
	if err := DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('ScanRuleOpLog') WHERE name = 'groupId'`,
	).Scan(&legacyLogColumnCount); err != nil {
		t.Fatal(err)
	}
	if legacyLogColumnCount != 0 {
		t.Error("legacy ComicGroup groupId column remains in ScanRuleOpLog")
	}
}

func TestComicGroupRemovalMigrationPreservesPermissionGroups(t *testing.T) {
	setupTestDB(t)

	if _, err := DB().Exec(`
		CREATE TABLE "ComicGroup" ("id" INTEGER PRIMARY KEY, "name" TEXT);
		CREATE TABLE "ComicGroupItem" ("groupId" INTEGER, "comicId" TEXT);
		CREATE TABLE "ComicGroupSeries" ("groupId" INTEGER, "seriesId" TEXT);
		CREATE TABLE "ComicGroupTag" ("groupId" INTEGER, "tagId" INTEGER);
		CREATE TABLE "GroupCategory" ("groupId" INTEGER, "categoryId" INTEGER);
		INSERT INTO "ComicGroup" ("id", "name") VALUES (1, 'legacy collection');
		INSERT INTO "UserGroup" ("id", "name", "description") VALUES ('permission-group', 'Readers', '');
		INSERT INTO "ScanRuleOpLog"
			("batchId","comicId","action","status","message")
			VALUES ('batch-before-cleanup','comic-1','ai_infer','success','preserve me');
	`); err != nil {
		t.Fatal(err)
	}

	var removal Migration
	for _, migration := range Migrations {
		if migration.Version == 41 {
			removal = migration
			break
		}
	}
	if removal.Version == 0 {
		t.Fatal("ComicGroup removal migration is not registered")
	}
	for _, statement := range splitSQL(removal.SQL) {
		if _, err := DB().Exec(statement); err != nil {
			t.Fatalf("execute removal migration: %v", err)
		}
	}

	var permissionGroupCount int
	if err := DB().QueryRow(
		`SELECT COUNT(*) FROM "UserGroup" WHERE "id" = 'permission-group'`,
	).Scan(&permissionGroupCount); err != nil {
		t.Fatal(err)
	}
	if permissionGroupCount != 1 {
		t.Fatal("ComicGroup migration removed UserGroup permission data")
	}

	var preservedLogCount int
	if err := DB().QueryRow(
		`SELECT COUNT(*) FROM "ScanRuleOpLog"
		 WHERE "batchId" = 'batch-before-cleanup' AND "message" = 'preserve me'`,
	).Scan(&preservedLogCount); err != nil {
		t.Fatal(err)
	}
	if preservedLogCount != 1 {
		t.Fatal("ComicGroup migration discarded unrelated scan-rule logs")
	}

	for _, table := range []string{"UserGroupMember", "GroupLibraryAccess", "ComicSeries", "ComicSeriesItem"} {
		var count int
		if err := DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("required table %s was removed", table)
		}
	}
}
