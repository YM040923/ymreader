package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     40,
		Description: "Add canonical work model tables",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "Work" (
				"id"               TEXT NOT NULL PRIMARY KEY,
				"libraryId"        TEXT NOT NULL,
				"rootRelativePath" TEXT NOT NULL DEFAULT '',
				"title"            TEXT NOT NULL,
				"sortTitle"        TEXT NOT NULL DEFAULT '',
				"coverUrl"         TEXT NOT NULL DEFAULT '',
				"coverUnitId"      TEXT NOT NULL DEFAULT '',
				"author"           TEXT NOT NULL DEFAULT '',
				"publisher"        TEXT NOT NULL DEFAULT '',
				"year"             INTEGER,
				"description"      TEXT NOT NULL DEFAULT '',
				"language"         TEXT NOT NULL DEFAULT '',
				"genre"            TEXT NOT NULL DEFAULT '',
				"metadataSource"   TEXT NOT NULL DEFAULT '',
				"contentType"      TEXT NOT NULL DEFAULT 'comic',
				"metadataLocked"   BOOLEAN NOT NULL DEFAULT 0,
				"manualLocked"     BOOLEAN NOT NULL DEFAULT 0,
				"createdAt"        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt"        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT "Work_libraryId_fkey" FOREIGN KEY ("libraryId")
					REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "Work_library_sort_idx" ON "Work"("libraryId", "sortTitle", "title", "id");`,
			`CREATE TABLE IF NOT EXISTS "WorkUnit" (
				"id"            TEXT NOT NULL PRIMARY KEY,
				"workId"        TEXT NOT NULL,
				"comicId"       TEXT NOT NULL,
				"relativePath"  TEXT NOT NULL DEFAULT '',
				"title"         TEXT NOT NULL,
				"displayLabel"  TEXT NOT NULL DEFAULT '',
				"unitKind"      TEXT NOT NULL DEFAULT '',
				"volumeNumber"  REAL,
				"chapterNumber" REAL,
				"sortIndex"     INTEGER NOT NULL DEFAULT 0,
				"pageCount"     INTEGER NOT NULL DEFAULT 0,
				"fileSize"      INTEGER NOT NULL DEFAULT 0,
				"coverUrl"      TEXT NOT NULL DEFAULT '',
				CONSTRAINT "WorkUnit_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "WorkUnit_comicId_fkey" FOREIGN KEY ("comicId")
					REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("workId", "relativePath")
			);`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_work_sort_idx" ON "WorkUnit"("workId", "sortIndex", "id");`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_comic_idx" ON "WorkUnit"("comicId");`,
			`CREATE TABLE IF NOT EXISTS "UserWorkProgress" (
				"userId"    TEXT NOT NULL,
				"workId"    TEXT NOT NULL,
				"unitId"    TEXT NOT NULL,
				"pageIndex" INTEGER NOT NULL DEFAULT 0,
				"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY ("userId", "workId"),
				CONSTRAINT "UWP_userId_fkey" FOREIGN KEY ("userId")
					REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_unitId_fkey" FOREIGN KEY ("unitId")
					REFERENCES "WorkUnit" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "UserWorkProgress_workId_idx" ON "UserWorkProgress"("workId");`,
			`CREATE INDEX IF NOT EXISTS "UserWorkProgress_unitId_idx" ON "UserWorkProgress"("unitId");`,
		}, "\n"),
	})
	Migrations = append(Migrations, Migration{
		Version:     41,
		Description: "Repair canonical work model schema for databases with legacy migration 40",
		SQL: strings.Join([]string{
			`PRAGMA foreign_keys = OFF;`,
			`CREATE TABLE IF NOT EXISTS "Work" (
				"id"               TEXT NOT NULL PRIMARY KEY,
				"libraryId"        TEXT NOT NULL,
				"rootRelativePath" TEXT NOT NULL DEFAULT '',
				"title"            TEXT NOT NULL,
				"sortTitle"        TEXT NOT NULL DEFAULT '',
				"coverUrl"         TEXT NOT NULL DEFAULT '',
				"coverUnitId"      TEXT NOT NULL DEFAULT '',
				"author"           TEXT NOT NULL DEFAULT '',
				"publisher"        TEXT NOT NULL DEFAULT '',
				"year"             INTEGER,
				"description"      TEXT NOT NULL DEFAULT '',
				"language"         TEXT NOT NULL DEFAULT '',
				"genre"            TEXT NOT NULL DEFAULT '',
				"metadataSource"   TEXT NOT NULL DEFAULT '',
				"contentType"      TEXT NOT NULL DEFAULT 'comic',
				"metadataLocked"   BOOLEAN NOT NULL DEFAULT 0,
				"manualLocked"     BOOLEAN NOT NULL DEFAULT 0,
				"createdAt"        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt"        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT "Work_libraryId_fkey" FOREIGN KEY ("libraryId")
					REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`ALTER TABLE "Work" ADD COLUMN "rootRelativePath" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "sortTitle" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "coverUrl" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "coverUnitId" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "author" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "publisher" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "year" INTEGER;`,
			`ALTER TABLE "Work" ADD COLUMN "description" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "language" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "genre" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "metadataSource" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "contentType" TEXT NOT NULL DEFAULT 'comic';`,
			`ALTER TABLE "Work" ADD COLUMN "metadataLocked" BOOLEAN NOT NULL DEFAULT 0;`,
			`ALTER TABLE "Work" ADD COLUMN "manualLocked" BOOLEAN NOT NULL DEFAULT 0;`,
			`ALTER TABLE "Work" ADD COLUMN "createdAt" DATETIME NOT NULL DEFAULT '';`,
			`ALTER TABLE "Work" ADD COLUMN "updatedAt" DATETIME NOT NULL DEFAULT '';`,
			`CREATE INDEX IF NOT EXISTS "Work_library_sort_idx" ON "Work"("libraryId", "sortTitle", "title", "id");`,

			`CREATE TABLE IF NOT EXISTS "WorkUnit" (
				"id"            TEXT NOT NULL PRIMARY KEY,
				"workId"        TEXT NOT NULL,
				"comicId"       TEXT NOT NULL,
				"relativePath"  TEXT NOT NULL DEFAULT '',
				"title"         TEXT NOT NULL,
				"displayLabel"  TEXT NOT NULL DEFAULT '',
				"unitKind"      TEXT NOT NULL DEFAULT '',
				"volumeNumber"  REAL,
				"chapterNumber" REAL,
				"sortIndex"     INTEGER NOT NULL DEFAULT 0,
				"pageCount"     INTEGER NOT NULL DEFAULT 0,
				"fileSize"      INTEGER NOT NULL DEFAULT 0,
				"coverUrl"      TEXT NOT NULL DEFAULT '',
				CONSTRAINT "WorkUnit_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "WorkUnit_comicId_fkey" FOREIGN KEY ("comicId")
					REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("workId", "relativePath")
			);`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "relativePath" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "displayLabel" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "unitKind" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "volumeNumber" REAL;`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "chapterNumber" REAL;`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "sortIndex" INTEGER NOT NULL DEFAULT 0;`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "pageCount" INTEGER NOT NULL DEFAULT 0;`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "fileSize" INTEGER NOT NULL DEFAULT 0;`,
			`ALTER TABLE "WorkUnit" ADD COLUMN "coverUrl" TEXT NOT NULL DEFAULT '';`,

			`CREATE TABLE IF NOT EXISTS "UserWorkProgress" (
				"userId"    TEXT NOT NULL,
				"workId"    TEXT NOT NULL,
				"unitId"    TEXT NOT NULL DEFAULT '',
				"pageIndex" INTEGER NOT NULL DEFAULT 0,
				"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY ("userId", "workId"),
				CONSTRAINT "UWP_userId_fkey" FOREIGN KEY ("userId")
					REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_unitId_fkey" FOREIGN KEY ("unitId")
					REFERENCES "WorkUnit" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`ALTER TABLE "UserWorkProgress" ADD COLUMN "unitId" TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE "UserWorkProgress" ADD COLUMN "updatedAt" DATETIME NOT NULL DEFAULT '';`,

			`DROP TABLE IF EXISTS "WorkUnit_migration_41";`,
			`CREATE TABLE "WorkUnit_migration_41" (
				"id"            TEXT NOT NULL PRIMARY KEY,
				"workId"        TEXT NOT NULL,
				"comicId"       TEXT NOT NULL,
				"relativePath"  TEXT NOT NULL DEFAULT '',
				"title"         TEXT NOT NULL,
				"displayLabel"  TEXT NOT NULL DEFAULT '',
				"unitKind"      TEXT NOT NULL DEFAULT '',
				"volumeNumber"  REAL,
				"chapterNumber" REAL,
				"sortIndex"     INTEGER NOT NULL DEFAULT 0,
				"pageCount"     INTEGER NOT NULL DEFAULT 0,
				"fileSize"      INTEGER NOT NULL DEFAULT 0,
				"coverUrl"      TEXT NOT NULL DEFAULT '',
				CONSTRAINT "WorkUnit_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "WorkUnit_comicId_fkey" FOREIGN KEY ("comicId")
					REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("workId", "relativePath")
			);`,
			`INSERT OR IGNORE INTO "WorkUnit_migration_41" ("id", "workId", "comicId", "relativePath", "title", "displayLabel", "unitKind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "coverUrl")
			 SELECT "id", "workId", "comicId", "relativePath", "title", "displayLabel", "unitKind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "coverUrl"
			 FROM "WorkUnit"
			 GROUP BY "workId", "relativePath";`,
			`DROP TABLE "WorkUnit";`,
			`ALTER TABLE "WorkUnit_migration_41" RENAME TO "WorkUnit";`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_work_sort_idx" ON "WorkUnit"("workId", "sortIndex", "id");`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_comic_idx" ON "WorkUnit"("comicId");`,

			`DROP TABLE IF EXISTS "UserWorkProgress_migration_41";`,
			`CREATE TABLE "UserWorkProgress_migration_41" (
				"userId"    TEXT NOT NULL,
				"workId"    TEXT NOT NULL,
				"unitId"    TEXT NOT NULL,
				"pageIndex" INTEGER NOT NULL DEFAULT 0,
				"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY ("userId", "workId"),
				CONSTRAINT "UWP_userId_fkey" FOREIGN KEY ("userId")
					REFERENCES "User" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "UWP_unitId_fkey" FOREIGN KEY ("unitId")
					REFERENCES "WorkUnit" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`INSERT OR IGNORE INTO "UserWorkProgress_migration_41" ("userId", "workId", "unitId", "pageIndex", "updatedAt")
			 SELECT p."userId", p."workId", COALESCE(NULLIF(p."unitId", ''), (SELECT u."id" FROM "WorkUnit" u WHERE u."workId" = p."workId" ORDER BY u."sortIndex", u."id" LIMIT 1), ''), p."pageIndex", p."updatedAt"
			 FROM "UserWorkProgress" p;`,
			`DROP TABLE "UserWorkProgress";`,
			`ALTER TABLE "UserWorkProgress_migration_41" RENAME TO "UserWorkProgress";`,
			`CREATE INDEX IF NOT EXISTS "UserWorkProgress_workId_idx" ON "UserWorkProgress"("workId");`,
			`CREATE INDEX IF NOT EXISTS "UserWorkProgress_unitId_idx" ON "UserWorkProgress"("unitId");`,
			`PRAGMA foreign_keys = ON;`,
		}, "\n"),
	})
}
