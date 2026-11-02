package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	if workLibraryID != comicLibraryID {
		return UserWorkProgress{}, false, fmt.Errorf("Comic does not belong to Work")
	}
	belongs, err := comicBelongsToWorkTx(tx, update.WorkID, update.ComicID, relativePath, rootPath)
	if err != nil {
		return UserWorkProgress{}, false, fmt.Errorf("resolve Work membership: %w", err)
	}
	if !belongs {
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

// comicBelongsToWorkTx accepts manual Work membership as the authority when a
// Work no longer maps to one physical directory (merged/split Works).
func comicBelongsToWorkTx(tx *sql.Tx, workID, comicID, comicPath, rootPath string) (bool, error) {
	if comicPathBelongsToWork(comicPath, rootPath) {
		return true, nil
	}
	if !WorkMembershipTableAvailable() {
		return false, nil
	}
	var exists int
	err := tx.QueryRow(
		`SELECT 1 FROM "WorkMembership" WHERE "comicId" = ? AND "workId" = ?`,
		comicID, workID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func comicPathBelongsToWork(comicPath, rootPath string) bool {
	comicPath = normalizeProgressPath(comicPath)
	rootPath = normalizeProgressPath(rootPath)
	if comicPath == "" || rootPath == "" {
		return false
	}
	comicLower := strings.ToLower(comicPath)
	rootLower := strings.ToLower(rootPath)
	if comicLower == rootLower || strings.HasPrefix(comicLower, rootLower+"/") {
		return true
	}
	// A Work detected from a single archive is named after the file without its
	// extension (splitWorkRoot), so the Comic path is that Work root plus an
	// archive suffix. Without this, every one-archive Work — the whole
	// "作品.zip" layout — could never store a reading cursor.
	if stem := strings.ToLower(strings.TrimSuffix(comicPath, filepath.Ext(comicPath))); stem != comicLower {
		return stem == rootLower
	}
	return false
}

func normalizeProgressPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	return strings.Trim(value, "/")
}

// WorkProgressSnapshot is one per-user cursor captured before a grouping
// change, so the change can be undone without losing reading positions.
type WorkProgressSnapshot struct {
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

func snapshotColumns() string {
	return `"userId", "workId", "unitId", "comicId", "relativePage", "absolutePage",
	        "updatedAt", "clientSessionId", "lastSequence"`
}

func scanWorkProgress(row workProgressScanner) (WorkProgressSnapshot, error) {
	var entry WorkProgressSnapshot
	err := row.Scan(
		&entry.UserID, &entry.WorkID, &entry.UnitID, &entry.ComicID,
		&entry.RelativePage, &entry.AbsolutePage, &entry.UpdatedAt,
		&entry.ClientSessionID, &entry.LastSequence,
	)
	return entry, err
}

func listUserWorkProgress(workIDs []string) ([]WorkProgressSnapshot, error) {
	ids := uniqueNonEmpty(workIDs)
	if len(ids) == 0 || !workTableExists("UserWorkProgress") {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT `+snapshotColumns()+`
		FROM "UserWorkProgress"
		WHERE "workId" IN (`+placeholders(len(ids))+`)
	`, anyStrings(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]WorkProgressSnapshot, 0, len(ids))
	for rows.Next() {
		entry, err := scanWorkProgress(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

// SnapshotUserWorkProgress captures every cursor attached to the given Works,
// for any user, before a merge or split rewrites their grouping.
func SnapshotUserWorkProgress(workIDs []string) ([]WorkProgressSnapshot, error) {
	return listUserWorkProgress(workIDs)
}

// WorkIDsWithCursorsForComics lists the Works that still hold a cursor for one
// of these physical Comics, including grouping that no longer appears in the
// catalogue, so restoring automatic grouping can pick those positions up.
func WorkIDsWithCursorsForComics(comicIDs []string) ([]string, error) {
	ids := uniqueNonEmpty(comicIDs)
	if len(ids) == 0 || !workTableExists("UserWorkProgress") {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT DISTINCT "workId" FROM "UserWorkProgress"
		WHERE "comicId" IN (`+placeholders(len(ids))+`)
	`, anyStrings(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0, len(ids))
	for rows.Next() {
		var workID string
		if err := rows.Scan(&workID); err != nil {
			return nil, err
		}
		result = append(result, workID)
	}
	return result, rows.Err()
}

// RemapUserWorkProgress re-points the cursors of the given Works at the Work
// that owns their Comic now. Without this, merging a read Work strands the
// cursor on a Work that left the catalogue, and 继续阅读 loses the entry.
// Returns the number of cursors that moved.
func RemapUserWorkProgress(workIDs []string) (int, error) {
	ids := uniqueNonEmpty(workIDs)
	if len(ids) == 0 || !workTableExists("UserWorkProgress") {
		return 0, nil
	}
	current, err := listUserWorkProgress(ids)
	if err != nil {
		return 0, err
	}
	if len(current) == 0 {
		return 0, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	type cursorKey struct {
		userID string
		workID string
	}
	// Only the newest cursor per (user, target Work) survives a merge: the
	// primary key allows one row each, and the newest is what the reader shows.
	pending := make(map[cursorKey]WorkProgressSnapshot, len(current))
	moved := make(map[cursorKey]struct{}, len(current))
	for _, entry := range current {
		owner, ownerErr := resolveWorkOwnerForComicTx(tx, entry.ComicID)
		if ownerErr != nil {
			return 0, ownerErr
		}
		if owner == "" || owner == entry.WorkID {
			continue
		}
		key := cursorKey{userID: entry.UserID, workID: owner}
		if existing, seen := pending[key]; !seen || entry.UpdatedAt.After(existing.UpdatedAt) {
			pending[key] = entry
		}
		moved[cursorKey{userID: entry.UserID, workID: entry.WorkID}] = struct{}{}
	}
	if len(pending) == 0 {
		return 0, nil
	}

	movedCount := 0
	for key, entry := range pending {
		var existing WorkProgressSnapshot
		scanErr := tx.QueryRow(`
			SELECT `+snapshotColumns()+`
			FROM "UserWorkProgress"
			WHERE "userId" = ? AND "workId" = ?
		`, key.userID, key.workID).Scan(
			&existing.UserID, &existing.WorkID, &existing.UnitID, &existing.ComicID,
			&existing.RelativePage, &existing.AbsolutePage, &existing.UpdatedAt,
			&existing.ClientSessionID, &existing.LastSequence,
		)
		if scanErr == nil {
			if existing.WorkID == entry.WorkID {
				continue
			}
			if !entry.UpdatedAt.After(existing.UpdatedAt) {
				// The target Work already has a fresher cursor; dropping the
				// absorbed one keeps the reading position the user last saw.
				continue
			}
		} else if scanErr != sql.ErrNoRows {
			return 0, scanErr
		}
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
		`, entry.UserID, key.workID, entry.UnitID, entry.ComicID, entry.RelativePage,
			entry.AbsolutePage, entry.UpdatedAt, entry.ClientSessionID, entry.LastSequence); err != nil {
			return 0, err
		}
		movedCount++
	}

	for _, entry := range current {
		if _, ok := moved[cursorKey{userID: entry.UserID, workID: entry.WorkID}]; !ok {
			continue
		}
		if _, err := tx.Exec(`
			DELETE FROM "UserWorkProgress" WHERE "userId" = ? AND "workId" = ?
		`, entry.UserID, entry.WorkID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return movedCount, nil
}

// RestoreUserWorkProgressTx puts the cursors captured before an operation back
// verbatim. [workIDs] is the operation's affected Work list, so a cursor the
// operation moved onto another Work is cleared even when no cursor was captured
// for that Work beforehand.
func RestoreUserWorkProgressTx(tx *sql.Tx, workIDs []string, snapshot []WorkProgressSnapshot) error {
	if tx == nil || !workTableExists("UserWorkProgress") {
		return nil
	}
	ids := make([]string, 0, len(workIDs)+len(snapshot))
	ids = append(ids, workIDs...)
	for _, entry := range snapshot {
		ids = append(ids, entry.WorkID)
	}
	ids = uniqueNonEmpty(ids)
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(`
		DELETE FROM "UserWorkProgress" WHERE "workId" IN (`+placeholders(len(ids))+`)
	`, anyStrings(ids)...); err != nil {
		return err
	}
	for _, entry := range snapshot {
		if strings.TrimSpace(entry.UserID) == "" || strings.TrimSpace(entry.WorkID) == "" {
			continue
		}
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
		`, entry.UserID, entry.WorkID, entry.UnitID, entry.ComicID, entry.RelativePage,
			entry.AbsolutePage, entry.UpdatedAt, entry.ClientSessionID, entry.LastSequence); err != nil {
			return err
		}
	}
	return nil
}

// resolveWorkOwnerForComicTx answers "which Work shows this archive now":
// explicit manual membership wins, then the deepest directory Work that
// contains the comic path. An empty result means nothing claims it.
func resolveWorkOwnerForComicTx(tx *sql.Tx, comicID string) (string, error) {
	if strings.TrimSpace(comicID) == "" {
		return "", nil
	}
	if WorkMembershipTableAvailable() {
		var manualWorkID string
		err := tx.QueryRow(`SELECT "workId" FROM "WorkMembership" WHERE "comicId" = ?`, comicID).
			Scan(&manualWorkID)
		if err == nil && strings.TrimSpace(manualWorkID) != "" {
			return manualWorkID, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
	}
	var libraryID, relativePath string
	if err := tx.QueryRow(`
		SELECT "libraryId", "relativePath" FROM "Comic" WHERE "id" = ?
	`, comicID).Scan(&libraryID, &relativePath); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	rows, err := tx.Query(`
		SELECT "id", "rootPath" FROM "LogicalWork"
		WHERE "libraryId" = ? AND "missingSince" IS NULL
		ORDER BY LENGTH("rootPath") DESC
	`, libraryID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type candidate struct {
		id, rootPath string
	}
	candidates := make([]candidate, 0, 8)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.rootPath); err != nil {
			return "", err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	for _, item := range candidates {
		if comicPathBelongsToWork(relativePath, item.rootPath) {
			return item.id, nil
		}
	}
	return "", nil
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func anyStrings(values []string) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

