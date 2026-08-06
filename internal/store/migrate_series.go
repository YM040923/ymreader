package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     33,
		Description: "Add hierarchical comic series, sections and item relationships",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "ComicSeries" (
				"id" TEXT NOT NULL PRIMARY KEY,
				"libraryId" TEXT NOT NULL,
				"rootRelativePath" TEXT NOT NULL,
				"title" TEXT NOT NULL,
				"sortTitle" TEXT NOT NULL DEFAULT '',
				"coverComicId" TEXT NOT NULL DEFAULT '',
				"detectionSource" TEXT NOT NULL DEFAULT 'directory',
				"manualLocked" BOOLEAN NOT NULL DEFAULT 0,
				"createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT "ComicSeries_libraryId_fkey" FOREIGN KEY ("libraryId") REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("libraryId", "rootRelativePath")
			);`,
			`CREATE INDEX IF NOT EXISTS "ComicSeries_libraryId_idx" ON "ComicSeries"("libraryId");`,
			`CREATE INDEX IF NOT EXISTS "ComicSeries_sortTitle_idx" ON "ComicSeries"("sortTitle");`,
			`CREATE TABLE IF NOT EXISTS "ComicSeriesSection" (
				"id" TEXT NOT NULL PRIMARY KEY,
				"seriesId" TEXT NOT NULL,
				"title" TEXT NOT NULL,
				"relativePath" TEXT NOT NULL,
				"kind" TEXT NOT NULL DEFAULT 'season',
				"seasonNumber" INTEGER,
				"sortIndex" INTEGER NOT NULL DEFAULT 0,
				"detectionSource" TEXT NOT NULL DEFAULT 'directory',
				"manualLocked" BOOLEAN NOT NULL DEFAULT 0,
				CONSTRAINT "ComicSeriesSection_seriesId_fkey" FOREIGN KEY ("seriesId") REFERENCES "ComicSeries" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				UNIQUE ("seriesId", "relativePath")
			);`,
			`CREATE INDEX IF NOT EXISTS "ComicSeriesSection_seriesId_idx" ON "ComicSeriesSection"("seriesId", "sortIndex");`,
			`CREATE TABLE IF NOT EXISTS "ComicSeriesItem" (
				"seriesId" TEXT NOT NULL,
				"sectionId" TEXT,
				"comicId" TEXT NOT NULL,
				"sortIndex" INTEGER NOT NULL DEFAULT 0,
				"displayLabel" TEXT NOT NULL DEFAULT '',
				PRIMARY KEY ("seriesId", "comicId"),
				CONSTRAINT "ComicSeriesItem_seriesId_fkey" FOREIGN KEY ("seriesId") REFERENCES "ComicSeries" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
				CONSTRAINT "ComicSeriesItem_sectionId_fkey" FOREIGN KEY ("sectionId") REFERENCES "ComicSeriesSection" ("id") ON DELETE SET NULL ON UPDATE CASCADE,
				CONSTRAINT "ComicSeriesItem_comicId_fkey" FOREIGN KEY ("comicId") REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS "ComicSeriesItem_comicId_key" ON "ComicSeriesItem"("comicId");`,
			`CREATE INDEX IF NOT EXISTS "ComicSeriesItem_series_sort_idx" ON "ComicSeriesItem"("seriesId", "sectionId", "sortIndex");`,
		}, "\n"),
	})
}
