package store

import "strings"

func init() {
	Migrations = append(Migrations, Migration{
		Version:     49,
		Description: "Add manual Work membership for user-defined merges and splits",
		SQL: `CREATE TABLE IF NOT EXISTS "WorkMembership" (
		        "comicId" TEXT NOT NULL PRIMARY KEY,
		        "workId" TEXT NOT NULL,
		        "libraryId" TEXT NOT NULL,
		        "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		        "updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		        CONSTRAINT "WorkMembership_comicId_fkey" FOREIGN KEY ("comicId")
		          REFERENCES "Comic" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
		        CONSTRAINT "WorkMembership_workId_fkey" FOREIGN KEY ("workId")
		          REFERENCES "LogicalWork" ("id") ON DELETE CASCADE ON UPDATE CASCADE
		      );
		      CREATE INDEX IF NOT EXISTS "WorkMembership_workId_idx" ON "WorkMembership"("workId");
		      CREATE INDEX IF NOT EXISTS "WorkMembership_libraryId_idx" ON "WorkMembership"("libraryId");`,
	})
}

type WorkMembershipEntry struct {
	ComicID   string
	WorkID    string
	LibraryID string
}

func WorkMembershipTableAvailable() bool {
	if db == nil {
		return false
	}
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='WorkMembership'`).Scan(&name)
	return err == nil
}

func GetWorkMemberships(libraryIDs []string, filterLibraries bool) (map[string]string, error) {
	if !WorkMembershipTableAvailable() {
		return map[string]string{}, nil
	}
	query := `SELECT "comicId", "workId" FROM "WorkMembership"`
	args := []interface{}{}
	if filterLibraries {
		if len(libraryIDs) == 0 {
			return map[string]string{}, nil
		}
		placeholders := make([]string, len(libraryIDs))
		for index, id := range libraryIDs {
			placeholders[index] = "?"
			args = append(args, id)
		}
		query += ` WHERE "libraryId" IN (` + strings.Join(placeholders, ",") + `)`
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var comicID, workID string
		if err := rows.Scan(&comicID, &workID); err != nil {
			return nil, err
		}
		result[comicID] = workID
	}
	return result, rows.Err()
}

func SetWorkMemberships(entries []WorkMembershipEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, entry := range entries {
		if strings.TrimSpace(entry.ComicID) == "" ||
			strings.TrimSpace(entry.WorkID) == "" ||
			strings.TrimSpace(entry.LibraryID) == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO "WorkMembership" ("comicId", "workId", "libraryId", "createdAt", "updatedAt")
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT("comicId") DO UPDATE SET
				"workId" = excluded."workId",
				"libraryId" = excluded."libraryId",
				"updatedAt" = CURRENT_TIMESTAMP
		`, entry.ComicID, entry.WorkID, entry.LibraryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ManualWorkIDPrefix marks Work IDs that exist only as user-defined groupings.
const ManualWorkIDPrefix = "work_manual_"

// manualWorkRootMarker separates a manual Work's display root from the
// synthetic suffix that keeps (libraryId, rootPath) unique. LogicalWork only
// allows one row per directory, while several manual Works may share the same
// source directory.
const manualWorkRootMarker = "\x1f"

func manualWorkRootKey(rootPath, workID string) string {
	if !strings.HasPrefix(workID, ManualWorkIDPrefix) {
		return rootPath
	}
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return manualWorkRootMarker + workID
	}
	if strings.HasSuffix(rootPath, manualWorkRootMarker+workID) {
		return rootPath
	}
	return rootPath + manualWorkRootMarker + workID
}

// DisplayWorkRootPath strips the synthetic uniqueness suffix from a manual
// Work's stored root path.
func DisplayWorkRootPath(workID, rootPath string) string {
	if !strings.HasPrefix(workID, ManualWorkIDPrefix) {
		return rootPath
	}
	suffix := manualWorkRootMarker + workID
	if strings.HasSuffix(rootPath, suffix) {
		return rootPath[:len(rootPath)-len(suffix)]
	}
	return rootPath
}

// EnsureLogicalWorkRecord creates a stable LogicalWork row for an explicit Work
// ID. Existing metadata is never overwritten.
func EnsureLogicalWorkRecord(work LogicalWork) error {
	logicalWorkWriteMu.Lock()
	defer logicalWorkWriteMu.Unlock()
	work.RootPath = manualWorkRootKey(work.RootPath, work.ID)
	title := strings.TrimSpace(work.Title)
	if title == "" {
		title = pathBase(DisplayWorkRootPath(work.ID, work.RootPath))
	}
	coverSource := "auto"
	if work.CoverComicID != "" {
		coverSource = "comic"
	}
	_, err := db.Exec(`
		INSERT INTO "LogicalWork" (
			"id", "libraryId", "rootPath", "contentType", "title", "sortTitle",
			"coverSource", "coverComicId", "coverPage", "coverAspectRatio",
			"createdAt", "updatedAt", "lastSeenAt", "missingSince"
		) VALUES (?, ?, ?, 'comic', ?, ?, ?, NULLIF(?, ''), NULL, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL)
		ON CONFLICT("id") DO NOTHING
	`, work.ID, work.LibraryID, work.RootPath, title, BuildTitleSortKey(title),
		coverSource, work.CoverComicID, work.CoverAspectRatio)
	return err
}
