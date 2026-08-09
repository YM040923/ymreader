package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UserWorkProgress is the canonical per-user cursor for a logical Work.
// RelativePage is scoped to UnitID, while AbsolutePage is scoped to ComicID.
type UserWorkProgress struct {
	UserID          string    `json:"userId"`
	WorkID          string    `json:"workId"`
	UnitID          string    `json:"unitId"`
	ComicID         string    `json:"comicId"`
	RelativePage    int       `json:"relativePage"`
	AbsolutePage    int       `json:"absolutePage"`
	UpdatedAt       time.Time `json:"updatedAt"`
	ClientSessionID string    `json:"clientSessionId"`
	LastSequence    int       `json:"lastSequence"`
}

// WorkProgressUpdate contains a server-validated Work/Unit cursor.
type WorkProgressUpdate struct {
	UserID          string
	WorkID          string
	UnitID          string
	ComicID         string
	RelativePage    int
	UnitStartPage   int
	UnitPageCount   int
	ClientSessionID string
	Sequence        int
}

// UpsertUserWorkProgressTx persists the latest cursor for a Work. A newer
// sequence in the same client session may move in either direction.
func UpsertUserWorkProgressTx(tx *sql.Tx, update WorkProgressUpdate) (UserWorkProgress, bool, error) {
	if tx == nil {
		return UserWorkProgress{}, false, fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(update.UserID) == "" || strings.TrimSpace(update.WorkID) == "" ||
		strings.TrimSpace(update.UnitID) == "" || strings.TrimSpace(update.ComicID) == "" ||
		strings.TrimSpace(update.ClientSessionID) == "" {
		return UserWorkProgress{}, false, fmt.Errorf("userId, workId, unitId, comicId and clientSessionId are required")
	}
	if update.Sequence < 0 {
		return UserWorkProgress{}, false, fmt.Errorf("sequence must not be negative")
	}
	if update.UnitStartPage < 0 || update.UnitPageCount <= 0 ||
		update.RelativePage < 0 || update.RelativePage >= update.UnitPageCount {
		return UserWorkProgress{}, false, fmt.Errorf("relative page is outside the Work unit")
	}

	var workLibraryID, rootPath string
	if err := tx.QueryRow(
		`SELECT "libraryId", "rootPath" FROM "LogicalWork" WHERE "id" = ? AND "missingSince" IS NULL`,
		update.WorkID,
	).Scan(&workLibraryID, &rootPath); err != nil {
		return UserWorkProgress{}, false, fmt.Errorf("resolve Work: %w", err)
	}

	var comicLibraryID, relativePath string
	var comicPageCount int
	if err := tx.QueryRow(
		`SELECT "libraryId", "relativePath", "pageCount" FROM "Comic" WHERE "id" = ? AND "missingSince" IS NULL`,
		update.ComicID,
	).Scan(&comicLibraryID, &relativePath, &comicPageCount); err != nil {
		return UserWorkProgress{}, false, fmt.Errorf("resolve Comic: %w", err)
	}
	if workLibraryID != comicLibraryID || !comicPathBelongsToWork(relativePath, rootPath) {
		return UserWorkProgress{}, false, fmt.Errorf("Comic does not belong to Work")
	}

	absolutePage := update.UnitStartPage + update.RelativePage
	if comicPageCount > 0 && absolutePage >= comicPageCount {
		return UserWorkProgress{}, false, fmt.Errorf("absolute page is outside the Comic")
	}

	current, err := getUserWorkProgressTx(tx, update.UserID, update.WorkID)
	if err != nil && err != sql.ErrNoRows {
		return UserWorkProgress{}, false, err
	}
	if err == nil && current.ClientSessionID == update.ClientSessionID &&
		update.Sequence <= current.LastSequence {
		return current, false, nil
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`
		INSERT INTO "UserWorkProgress" (
			"userId", "workId", "unitId", "comicId", "relativePage",
			"absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT("userId", "workId") DO UPDATE SET
			"unitId" = excluded."unitId",
			"comicId" = excluded."comicId",
			"relativePage" = excluded."relativePage",
			"absolutePage" = excluded."absolutePage",
			"updatedAt" = excluded."updatedAt",
			"clientSessionId" = excluded."clientSessionId",
			"lastSequence" = excluded."lastSequence"
	`, update.UserID, update.WorkID, update.UnitID, update.ComicID, update.RelativePage,
		absolutePage, now, update.ClientSessionID, update.Sequence); err != nil {
		return UserWorkProgress{}, false, err
	}

	return UserWorkProgress{
		UserID:          update.UserID,
		WorkID:          update.WorkID,
		UnitID:          update.UnitID,
		ComicID:         update.ComicID,
		RelativePage:    update.RelativePage,
		AbsolutePage:    absolutePage,
		UpdatedAt:       now,
		ClientSessionID: update.ClientSessionID,
		LastSequence:    update.Sequence,
	}, true, nil
}

func GetUserWorkProgress(userID, workID string) (*UserWorkProgress, error) {
	progress, err := getUserWorkProgressRow(db.QueryRow(`
		SELECT "userId", "workId", "unitId", "comicId", "relativePage",
		       "absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		FROM "UserWorkProgress"
		WHERE "userId" = ? AND "workId" = ?
	`, userID, workID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &progress, err
}

func GetUserWorkProgresses(userID string, workIDs []string) (map[string]UserWorkProgress, error) {
	result := make(map[string]UserWorkProgress, len(workIDs))
	if strings.TrimSpace(userID) == "" || len(workIDs) == 0 ||
		!workTableExists("UserWorkProgress") {
		return result, nil
	}
	args := make([]any, 0, len(workIDs)+1)
	args = append(args, userID)
	for _, workID := range workIDs {
		args = append(args, workID)
	}
	rows, err := db.Query(`
		SELECT "userId", "workId", "unitId", "comicId", "relativePage",
		       "absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		FROM "UserWorkProgress"
		WHERE "userId" = ? AND "workId" IN (`+placeholders(len(workIDs))+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		progress, scanErr := getUserWorkProgressRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[progress.WorkID] = progress
	}
	return result, rows.Err()
}

func getUserWorkProgressTx(tx *sql.Tx, userID, workID string) (UserWorkProgress, error) {
	return getUserWorkProgressRow(tx.QueryRow(`
		SELECT "userId", "workId", "unitId", "comicId", "relativePage",
		       "absolutePage", "updatedAt", "clientSessionId", "lastSequence"
		FROM "UserWorkProgress"
		WHERE "userId" = ? AND "workId" = ?
	`, userID, workID))
}

type workProgressScanner interface {
	Scan(dest ...any) error
}

func getUserWorkProgressRow(row workProgressScanner) (UserWorkProgress, error) {
	var progress UserWorkProgress
	err := row.Scan(
		&progress.UserID,
		&progress.WorkID,
		&progress.UnitID,
		&progress.ComicID,
		&progress.RelativePage,
		&progress.AbsolutePage,
		&progress.UpdatedAt,
		&progress.ClientSessionID,
		&progress.LastSequence,
	)
	return progress, err
}

func comicPathBelongsToWork(comicPath, rootPath string) bool {
	comicPath = normalizeProgressPath(comicPath)
	rootPath = normalizeProgressPath(rootPath)
	if comicPath == "" || rootPath == "" {
		return false
	}
	return strings.EqualFold(comicPath, rootPath) ||
		strings.HasPrefix(strings.ToLower(comicPath), strings.ToLower(rootPath)+"/")
}

func normalizeProgressPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	return strings.Trim(value, "/")
}
