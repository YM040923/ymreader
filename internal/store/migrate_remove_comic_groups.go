package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     41,
		Description: "Remove legacy ComicGroup collection tables",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "ScanRuleOpLog_v41" (
				"id"        INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				"batchId"   TEXT NOT NULL DEFAULT '',
				"comicId"   TEXT NOT NULL DEFAULT '',
				"action"    TEXT NOT NULL,
				"status"    TEXT NOT NULL DEFAULT 'success',
				"fromValue" TEXT NOT NULL DEFAULT '',
				"toValue"   TEXT NOT NULL DEFAULT '',
				"message"   TEXT NOT NULL DEFAULT '',
				"createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`,
			`INSERT OR IGNORE INTO "ScanRuleOpLog_v41"
				("id","batchId","comicId","action","status","fromValue","toValue","message","createdAt")
			 SELECT "id","batchId","comicId","action","status","fromValue","toValue","message","createdAt"
			 FROM "ScanRuleOpLog";`,
			`DROP TABLE "ScanRuleOpLog";`,
			`ALTER TABLE "ScanRuleOpLog_v41" RENAME TO "ScanRuleOpLog";`,
			`CREATE INDEX IF NOT EXISTS "SROL_batchId_idx" ON "ScanRuleOpLog"("batchId");`,
			`CREATE INDEX IF NOT EXISTS "SROL_comicId_idx" ON "ScanRuleOpLog"("comicId");`,
			`CREATE INDEX IF NOT EXISTS "SROL_action_idx" ON "ScanRuleOpLog"("action");`,
			`CREATE INDEX IF NOT EXISTS "SROL_createdAt_idx" ON "ScanRuleOpLog"("createdAt" DESC);`,
			`DROP TABLE IF EXISTS "ComicGroupSeries";`,
			`DROP TABLE IF EXISTS "ComicGroupItem";`,
			`DROP TABLE IF EXISTS "ComicGroupTag";`,
			`DROP TABLE IF EXISTS "GroupCategory";`,
			`DROP TABLE IF EXISTS "ComicGroup";`,
		}, "\n"),
	})
}
