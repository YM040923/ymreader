package store

import "testing"

func TestLibraryTypeMigrationRemovesMixedType(t *testing.T) {
	originalMigrations := Migrations
	defer func() { Migrations = originalMigrations }()

	beforeTypeRemoval := make([]Migration, 0, len(originalMigrations))
	for _, migration := range originalMigrations {
		if migration.Version < 31 {
			beforeTypeRemoval = append(beforeTypeRemoval, migration)
		}
	}

	Migrations = beforeTypeRemoval
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("run migrations before type removal: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('legacy-mixed', 'Legacy Mixed', 'mixed', '/legacy')`); err != nil {
		t.Fatalf("insert legacy mixed library: %v", err)
	}

	Migrations = originalMigrations
	if err := RunMigrations(); err != nil {
		t.Fatalf("run library type migration: %v", err)
	}

	var libraryType string
	if err := db.QueryRow(`SELECT "type" FROM "Library" WHERE "id" = 'legacy-mixed'`).Scan(&libraryType); err != nil {
		t.Fatalf("read migrated library: %v", err)
	}
	if libraryType != "comic" {
		t.Fatalf("legacy mixed library type = %q, want comic", libraryType)
	}

	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('blocked-mixed', 'Blocked Mixed', 'mixed', '/blocked')`); err == nil {
		t.Fatal("mixed library insert succeeded after migration")
	}
	if _, err := db.Exec(`UPDATE "Library" SET "type" = 'mixed' WHERE "id" = 'legacy-mixed'`); err == nil {
		t.Fatal("mixed library update succeeded after migration")
	}
}
