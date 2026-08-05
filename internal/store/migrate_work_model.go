package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     40,
		Description: "Add canonical work model tables",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "Work" (
				"id"           TEXT NOT NULL PRIMARY KEY,
				"libraryId"        TEXT NOT NULL,
				"rootRelativePath" TEXT NOT NULL DEFAULT '',
				"title"            TEXT NOT NULL,
				"sortTitle"    TEXT NOT NULL DEFAULT '',
				"author"       TEXT NOT NULL DEFAULT '',
				"description"  TEXT NOT NULL DEFAULT '',
				"coverComicId" TEXT NOT NULL DEFAULT '',
				"coverUrl"     TEXT NOT NULL DEFAULT '',
				"createdAt"    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt"    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT "Work_libraryId_fkey" FOREIGN KEY ("libraryId")
					REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("libraryId", "sortTitle", "id")
			);`,
			`CREATE INDEX IF NOT EXISTS "Work_libraryId_idx" ON "Work"("libraryId");`,
			`CREATE INDEX IF NOT EXISTS "Work_sortTitle_idx" ON "Work"("sortTitle", "title", "id");`,
			`CREATE TABLE IF NOT EXISTS "WorkUnit" (
				"id"           TEXT NOT NULL PRIMARY KEY,
				"workId"       TEXT NOT NULL,
				"comicId"      TEXT NOT NULL,
				"title"         TEXT NOT NULL,
				"displayLabel"  TEXT NOT NULL DEFAULT '',
				"relativePath"  TEXT NOT NULL DEFAULT '',
				"kind"          TEXT NOT NULL DEFAULT '',
				"volumeNumber"  REAL,
				"chapterNumber" REAL,
				"sortIndex"     INTEGER NOT NULL DEFAULT 0,
				"pageCount"    INTEGER NOT NULL DEFAULT 0,
				"fileSize"     INTEGER NOT NULL DEFAULT 0,
				"createdAt"    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt"    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT "WorkUnit_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "Work" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "WorkUnit_comicId_fkey" FOREIGN KEY ("comicId")
					REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("workId", "comicId")
			);`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_work_sort_idx" ON "WorkUnit"("workId", "sortIndex", "id");`,
			`CREATE INDEX IF NOT EXISTS "WorkUnit_comicId_idx" ON "WorkUnit"("comicId");`,
			`CREATE TABLE IF NOT EXISTS "UserWorkProgress" (
				"userId"    TEXT NOT NULL,
				"workId"    TEXT NOT NULL,
				"unitId"    TEXT NOT NULL,
				"lastPage"  INTEGER NOT NULL DEFAULT 0,
				"progress"  REAL NOT NULL DEFAULT 0,
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
}
