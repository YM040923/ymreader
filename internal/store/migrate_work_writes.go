package store

func init() {
	Migrations = append(Migrations, Migration{
		Version:     42,
		Description: "Persist custom ordering for unified works",
		SQL: `CREATE TABLE IF NOT EXISTS "WorkPreference" (
			"workId" TEXT NOT NULL PRIMARY KEY,
			"sortOrder" INTEGER NOT NULL DEFAULT 0,
			"updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
	})
}
