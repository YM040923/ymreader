package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     44,
		Description: "Add persistent LogicalWork metadata and relations",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "LogicalWork" (
				"id" TEXT NOT NULL PRIMARY KEY,
				"libraryId" TEXT NOT NULL,
				"rootPath" TEXT NOT NULL,
				"contentType" TEXT NOT NULL DEFAULT 'comic',
				"title" TEXT NOT NULL DEFAULT '',
				"sortTitle" TEXT NOT NULL DEFAULT '',
				"author" TEXT NOT NULL DEFAULT '',
				"publisher" TEXT NOT NULL DEFAULT '',
				"year" INTEGER,
				"description" TEXT NOT NULL DEFAULT '',
				"language" TEXT NOT NULL DEFAULT '',
				"genre" TEXT NOT NULL DEFAULT '',
				"status" TEXT NOT NULL DEFAULT '',
				"metadataSource" TEXT NOT NULL DEFAULT '',
				"metadataLocked" BOOLEAN NOT NULL DEFAULT 0,
				"externalRating" REAL,
				"externalRatingMax" REAL,
				"externalRatingSource" TEXT NOT NULL DEFAULT '',
				"externalRatingUpdatedAt" DATETIME,
				"coverSource" TEXT NOT NULL DEFAULT 'auto',
				"coverComicId" TEXT,
				"coverPage" INTEGER,
				"coverUrl" TEXT NOT NULL DEFAULT '',
				"coverAspectRatio" REAL NOT NULL DEFAULT 0,
				"coverLocked" BOOLEAN NOT NULL DEFAULT 0,
				"layoutFingerprint" TEXT NOT NULL DEFAULT '',
				"sourceFingerprint" TEXT NOT NULL DEFAULT '',
				"scrapeStatus" TEXT NOT NULL DEFAULT '',
				"lastScrapedAt" DATETIME,
				"scrapeError" TEXT NOT NULL DEFAULT '',
				"createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"lastSeenAt" DATETIME,
				"missingSince" DATETIME,
				CONSTRAINT "LogicalWork_libraryId_fkey" FOREIGN KEY ("libraryId")
					REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "LogicalWork_coverComicId_fkey" FOREIGN KEY ("coverComicId")
					REFERENCES "Comic" ("id") ON DELETE SET NULL ON UPDATE CASCADE,
				UNIQUE ("libraryId", "rootPath")
			);`,
			`CREATE INDEX IF NOT EXISTS "LogicalWork_libraryId_idx" ON "LogicalWork"("libraryId");`,
			`CREATE INDEX IF NOT EXISTS "LogicalWork_sortTitle_idx" ON "LogicalWork"("sortTitle", "title", "id");`,
			`CREATE INDEX IF NOT EXISTS "LogicalWork_updatedAt_idx" ON "LogicalWork"("updatedAt" DESC);`,
			`CREATE INDEX IF NOT EXISTS "LogicalWork_scrapeStatus_idx" ON "LogicalWork"("scrapeStatus", "updatedAt");`,
			`CREATE TABLE IF NOT EXISTS "LogicalWorkTag" (
				"workId" TEXT NOT NULL,
				"tagId" INTEGER NOT NULL,
				PRIMARY KEY ("workId", "tagId"),
				CONSTRAINT "LogicalWorkTag_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "LogicalWork" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "LogicalWorkTag_tagId_fkey" FOREIGN KEY ("tagId")
					REFERENCES "Tag" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "LogicalWorkTag_tagId_idx" ON "LogicalWorkTag"("tagId");`,
			`CREATE TABLE IF NOT EXISTS "LogicalWorkCategory" (
				"workId" TEXT NOT NULL,
				"categoryId" INTEGER NOT NULL,
				PRIMARY KEY ("workId", "categoryId"),
				CONSTRAINT "LogicalWorkCategory_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "LogicalWork" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "LogicalWorkCategory_categoryId_fkey" FOREIGN KEY ("categoryId")
					REFERENCES "Category" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "LogicalWorkCategory_categoryId_idx" ON "LogicalWorkCategory"("categoryId");`,
			`CREATE TABLE IF NOT EXISTS "LogicalWorkAlias" (
				"aliasType" TEXT NOT NULL,
				"aliasId" TEXT NOT NULL,
				"workId" TEXT NOT NULL,
				"createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY ("aliasType", "aliasId"),
				CONSTRAINT "LogicalWorkAlias_workId_fkey" FOREIGN KEY ("workId")
					REFERENCES "LogicalWork" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "LogicalWorkAlias_workId_idx" ON "LogicalWorkAlias"("workId");`,
		}, "\n"),
	})
}
