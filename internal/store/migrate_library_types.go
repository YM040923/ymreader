package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     31,
		Description: "Remove mixed library type and restrict libraries to comic or novel",
		SQL: strings.Join([]string{
			// 历史 mixed 书库不直接丢弃：小说内容数量更多时迁移为 novel，
			// 其余情况（漫画更多、数量相同或空书库）迁移为 comic。
			`UPDATE "Library"
			 SET "type" = CASE
			 	WHEN (
			 		SELECT COUNT(*) FROM "Comic" c
			 		WHERE c."libraryId" = "Library"."id" AND c."type" = 'novel'
			 	) > (
			 		SELECT COUNT(*) FROM "Comic" c
			 		WHERE c."libraryId" = "Library"."id" AND c."type" = 'comic'
			 	)
			 	THEN 'novel'
			 	ELSE 'comic'
			 END
			 WHERE "type" NOT IN ('comic', 'novel');`,
			`CREATE TRIGGER IF NOT EXISTS "Library_type_insert_check"
			 BEFORE INSERT ON "Library"
			 WHEN NEW."type" NOT IN ('comic', 'novel') BEGIN
			 	SELECT RAISE(ABORT, 'library type must be comic or novel');
			 END;`,
			`CREATE TRIGGER IF NOT EXISTS "Library_type_update_check"
			 BEFORE UPDATE OF "type" ON "Library"
			 WHEN NEW."type" NOT IN ('comic', 'novel') BEGIN
			 	SELECT RAISE(ABORT, 'library type must be comic or novel');
			 END;`,
		}, "\n"),
	})

	Migrations = append(Migrations, Migration{
		Version:     32,
		Description: "Keep record-only deletions hidden from automatic library scans",
		SQL: strings.Join([]string{
			`CREATE TABLE IF NOT EXISTS "LibraryIgnoredContent" (
				"libraryId" TEXT NOT NULL,
				"relativePath" TEXT NOT NULL,
				"createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY ("libraryId", "relativePath"),
				CONSTRAINT "LibraryIgnoredContent_libraryId_fkey" FOREIGN KEY ("libraryId")
					REFERENCES "Library" ("id") ON DELETE CASCADE ON UPDATE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS "LibraryIgnoredContent_libraryId_idx" ON "LibraryIgnoredContent"("libraryId");`,
			`CREATE TRIGGER IF NOT EXISTS "Comic_ignored_content_before_insert"
			 BEFORE INSERT ON "Comic"
			 WHEN EXISTS (
			 	SELECT 1 FROM "LibraryIgnoredContent" ignored
			 	WHERE ignored."libraryId" = COALESCE(NEW."libraryId", '')
			 	  AND ignored."relativePath" = COALESCE(NULLIF(NEW."relativePath", ''), NEW."filename")
			 ) BEGIN
			 	SELECT RAISE(IGNORE);
			 END;`,
		}, "\n"),
	})
}
