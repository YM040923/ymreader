package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

// WorkSourceItem is the physical library item used as work detection input.
type WorkSourceItem struct {
	ID           string
	LibraryID    string
	RelativePath string
	Title        string
	FileSize     int64
	PageCount    int
}

func ReplaceWorksForLibrary(libraryID string, detected []workmodel.DetectedWork) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, work := range detected {
		if work.LibraryID == "" {
			work.LibraryID = libraryID
		}
		if work.LibraryID != libraryID {
			return fmt.Errorf("work %s belongs to library %s, want %s", work.ID, work.LibraryID, libraryID)
		}
		if strings.TrimSpace(work.SortTitle) == "" {
			work.SortTitle = BuildTitleSortKey(work.Title)
		}
		manualLocked, err := detectedWorkIsManualLocked(tx, libraryID, work)
		if err != nil {
			return err
		}
		if manualLocked {
			continue
		}
		if err := upsertDetectedWork(tx, libraryID, work); err != nil {
			return err
		}
		seen := make([]string, 0, len(work.Units))
		for _, unit := range work.Units {
			if unit.ID == "" {
				return fmt.Errorf("unit for work %s has empty id", work.ID)
			}
			if unit.RelativePath == "" {
				return fmt.Errorf("unit %s for work %s has empty relative path", unit.ID, work.ID)
			}
			seen = append(seen, unit.RelativePath)
			if err := upsertDetectedWorkUnit(tx, work.ID, unit); err != nil {
				return err
			}
		}
		if err := deleteStaleUnits(tx, work.ID, seen); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM "Work" WHERE "libraryId" = ? AND "manualLocked" = 0 AND "id" NOT IN (`+placeholdersFromDetected(detected)+`)`, argsLibraryAndWorkIDs(libraryID, detected)...); err != nil {
		return err
	}
	return tx.Commit()
}

func detectedWorkIsManualLocked(tx *sql.Tx, libraryID string, work workmodel.DetectedWork) (bool, error) {
	var count int
	err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM "Work"
		WHERE "manualLocked" = 1
		  AND (
			"id" = ?
			OR ("libraryId" = ? AND "rootRelativePath" = ?)
		  )
	`, work.ID, libraryID, work.RootRelativePath).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func placeholdersFromDetected(detected []workmodel.DetectedWork) string {
	if len(detected) == 0 {
		return `''`
	}
	return strings.TrimRight(strings.Repeat("?,", len(detected)), ",")
}

func argsLibraryAndWorkIDs(libraryID string, detected []workmodel.DetectedWork) []interface{} {
	args := []interface{}{libraryID}
	for _, work := range detected {
		args = append(args, work.ID)
	}
	return args
}

func upsertDetectedWork(tx *sql.Tx, libraryID string, work workmodel.DetectedWork) error {
	_, err := tx.Exec(`
		INSERT INTO "Work" ("id", "libraryId", "rootRelativePath", "title", "sortTitle", "contentType", "createdAt", "updatedAt")
		VALUES (?, ?, ?, ?, ?, 'comic', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT("id") DO UPDATE SET
			"libraryId" = excluded."libraryId",
			"rootRelativePath" = excluded."rootRelativePath",
			"title" = CASE WHEN "Work"."metadataLocked" = 1 THEN "Work"."title" ELSE excluded."title" END,
			"sortTitle" = CASE WHEN "Work"."metadataLocked" = 1 THEN "Work"."sortTitle" ELSE excluded."sortTitle" END,
			"contentType" = CASE WHEN "Work"."metadataLocked" = 1 THEN "Work"."contentType" ELSE excluded."contentType" END,
			"updatedAt" = CURRENT_TIMESTAMP
	`, work.ID, libraryID, work.RootRelativePath, work.Title, work.SortTitle)
	return err
}

func upsertDetectedWorkUnit(tx *sql.Tx, workID string, detectedUnit workmodel.DetectedUnit) error {
	unit := model.WorkUnit{
		ID:            detectedUnit.ID,
		WorkID:        workID,
		ComicID:       detectedUnit.ComicID,
		RelativePath:  detectedUnit.RelativePath,
		Title:         detectedUnit.Title,
		DisplayLabel:  detectedUnit.DisplayLabel,
		UnitKind:      string(detectedUnit.Kind),
		VolumeNumber:  detectedUnit.VolumeNumber,
		ChapterNumber: detectedUnit.ChapterNumber,
		SortIndex:     detectedUnit.SortIndex,
		PageCount:     detectedUnit.PageCount,
		FileSize:      detectedUnit.FileSize,
		CoverURL:      BuildComicCoverURL(detectedUnit.ComicID),
	}
	if unit.Title == "" || unit.PageCount == 0 || unit.FileSize == 0 {
		fillWorkUnitFromComic(tx, &unit)
	}
	_, err := tx.Exec(`
		INSERT INTO "WorkUnit" ("id", "workId", "comicId", "relativePath", "title", "displayLabel", "unitKind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "coverUrl")
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT("workId", "relativePath") DO UPDATE SET
			"id" = excluded."id",
			"comicId" = excluded."comicId",
			"title" = excluded."title",
			"displayLabel" = excluded."displayLabel",
			"unitKind" = excluded."unitKind",
			"volumeNumber" = excluded."volumeNumber",
			"chapterNumber" = excluded."chapterNumber",
			"sortIndex" = excluded."sortIndex",
			"pageCount" = excluded."pageCount",
			"fileSize" = excluded."fileSize",
			"coverUrl" = excluded."coverUrl"
	`, unit.ID, unit.WorkID, unit.ComicID, unit.RelativePath, unit.Title, unit.DisplayLabel, unit.UnitKind, unit.VolumeNumber, unit.ChapterNumber, unit.SortIndex, unit.PageCount, unit.FileSize, unit.CoverURL)
	return err
}

func deleteStaleUnits(tx *sql.Tx, workID string, relativePaths []string) error {
	if len(relativePaths) == 0 {
		_, err := tx.Exec(`DELETE FROM "WorkUnit" WHERE "workId" = ?`, workID)
		return err
	}
	args := []interface{}{workID}
	for _, rel := range relativePaths {
		args = append(args, rel)
	}
	_, err := tx.Exec(`DELETE FROM "WorkUnit" WHERE "workId" = ? AND "relativePath" NOT IN (`+strings.TrimRight(strings.Repeat("?,", len(relativePaths)), ",")+`)`, args...)
	return err
}

func fillWorkUnitFromComic(tx *sql.Tx, unit *model.WorkUnit) {
	if unit.ComicID == "" {
		return
	}
	var title, rel string
	var pageCount int
	var fileSize int64
	if err := tx.QueryRow(`SELECT "title", COALESCE(NULLIF("relativePath", ''), "filename"), "pageCount", "fileSize" FROM "Comic" WHERE "id" = ?`, unit.ComicID).Scan(&title, &rel, &pageCount, &fileSize); err != nil {
		return
	}
	if unit.Title == "" {
		unit.Title = title
	}
	if unit.RelativePath == "" {
		unit.RelativePath = rel
	}
	if unit.PageCount == 0 {
		unit.PageCount = pageCount
	}
	if unit.FileSize == 0 {
		unit.FileSize = fileSize
	}
	if unit.CoverURL == "" {
		unit.CoverURL = BuildComicCoverURL(unit.ComicID)
	}
}

func ListWorks(libraryIDs []string, userID string, search string) ([]model.Work, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if len(libraryIDs) > 0 {
		where = append(where, `w."libraryId" IN (`+strings.TrimRight(strings.Repeat("?,", len(libraryIDs)), ",")+`)`)
		for _, id := range libraryIDs {
			args = append(args, id)
		}
	}
	if strings.TrimSpace(search) != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		where = append(where, `(LOWER(w."title") LIKE ? OR LOWER(w."sortTitle") LIKE ? OR LOWER(w."author") LIKE ? OR LOWER(w."publisher") LIKE ? OR LOWER(w."genre") LIKE ?)`)
		args = append(args, like, like, like, like, like)
	}
	query := workSummarySelectSQL + `
		FROM "Work" w
		LEFT JOIN "WorkUnit" u ON u."workId" = w."id"
		LEFT JOIN "UserWorkProgress" p ON p."workId" = w."id" AND p."userId" = ?
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY w."id"
		ORDER BY w."sortTitle", w."title", w."id"`
	args = append([]interface{}{userID}, args...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	works := []model.Work{}
	for rows.Next() {
		work, err := scanWorkSummary(rows)
		if err != nil {
			return nil, err
		}
		works = append(works, work)
	}
	return works, rows.Err()
}

func GetWorkDetail(workID, userID string) (*model.Work, []model.WorkUnit, error) {
	row := db.QueryRow(workSummarySelectSQL+`
		FROM "Work" w
		LEFT JOIN "WorkUnit" u ON u."workId" = w."id"
		LEFT JOIN "UserWorkProgress" p ON p."workId" = w."id" AND p."userId" = ?
		WHERE w."id" = ?
		GROUP BY w."id"`, userID, workID)
	work, err := scanWorkSummary(row)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	units, err := listWorkUnits(workID)
	if err != nil {
		return nil, nil, err
	}
	return &work, units, nil
}

func GetFirstWorkUnit(workID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? ORDER BY "sortIndex", "id" LIMIT 1`, workID))
}

func GetWorkUnit(unitID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "id" = ?`, unitID))
}

func GetAdjacentWorkUnit(workID string, sortIndex int, delta int) (*model.WorkUnit, error) {
	if delta < 0 {
		return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? AND "sortIndex" < ? ORDER BY "sortIndex" DESC, "id" DESC LIMIT 1`, workID, sortIndex))
	}
	if delta > 0 {
		return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? AND "sortIndex" > ? ORDER BY "sortIndex", "id" LIMIT 1`, workID, sortIndex))
	}
	return nil, nil
}

func SetUserWorkProgress(userID, workID, unitID string, pageIndex int) error {
	updatedAt := time.Now().UTC()
	_, err := db.Exec(`
		INSERT INTO "UserWorkProgress" ("userId", "workId", "unitId", "pageIndex", "updatedAt")
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT("userId", "workId") DO UPDATE SET
			"unitId" = excluded."unitId",
			"pageIndex" = excluded."pageIndex",
			"updatedAt" = excluded."updatedAt"
	`, userID, workID, unitID, pageIndex, updatedAt)
	return err
}

func GetUserWorkProgress(userID, workID string) (*model.UserWorkProgress, error) {
	var progress model.UserWorkProgress
	var updatedAt time.Time
	err := db.QueryRow(`SELECT "userId", "workId", "unitId", "pageIndex", "updatedAt" FROM "UserWorkProgress" WHERE "userId" = ? AND "workId" = ?`, userID, workID).Scan(&progress.UserID, &progress.WorkID, &progress.UnitID, &progress.PageIndex, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	progress.UpdatedAt = &updatedAt
	return &progress, nil
}

func GetWorkUnitByComicID(comicID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "comicId" = ? ORDER BY "workId", "sortIndex", "id" LIMIT 1`, comicID))
}

func ListWorkSourceItems(libraryID string) ([]WorkSourceItem, error) {
	rows, err := db.Query(`
		SELECT c."id", c."libraryId", COALESCE(NULLIF(c."relativePath", ''), c."filename"), c."title", c."fileSize", c."pageCount"
		FROM "Comic" c
		WHERE c."libraryId" = ?
		ORDER BY c."relativePath", c."filename", c."id"
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []WorkSourceItem{}
	for rows.Next() {
		var item WorkSourceItem
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.RelativePath, &item.Title, &item.FileSize, &item.PageCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func listWorkUnits(workID string) ([]model.WorkUnit, error) {
	rows, err := db.Query(workUnitSelectSQL+` WHERE "workId" = ? ORDER BY "sortIndex", "id"`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []model.WorkUnit{}
	for rows.Next() {
		unit, err := scanWorkUnit(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}

type workScanner interface {
	Scan(dest ...interface{}) error
}

const workSummarySelectSQL = `
	SELECT w."id", w."libraryId", w."rootRelativePath", w."title", w."sortTitle", w."coverUrl", w."coverUnitId",
	       w."author", w."publisher", w."year", w."description", w."language", w."genre", w."metadataSource", w."contentType",
	       w."metadataLocked", w."manualLocked", COUNT(u."id") AS itemCount, MAX(p."updatedAt") AS lastReadAt, w."createdAt", w."updatedAt" `

func scanWorkSummary(scanner workScanner) (model.Work, error) {
	var work model.Work
	var lastReadAt sql.NullTime
	err := scanner.Scan(&work.ID, &work.LibraryID, &work.RootRelativePath, &work.Title, &work.SortTitle, &work.CoverURL, &work.CoverUnitID, &work.Author, &work.Publisher, &work.Year, &work.Description, &work.Language, &work.Genre, &work.MetadataSource, &work.ContentType, &work.MetadataLocked, &work.ManualLocked, &work.ItemCount, &lastReadAt, &work.CreatedAt, &work.UpdatedAt)
	if lastReadAt.Valid {
		work.LastReadAt = &lastReadAt.Time
	}
	return work, err
}

const workUnitSelectSQL = `SELECT "id", "workId", "comicId", "relativePath", "title", "displayLabel", "unitKind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "coverUrl" FROM "WorkUnit"`

func scanWorkUnit(scanner workScanner) (model.WorkUnit, error) {
	var unit model.WorkUnit
	err := scanner.Scan(&unit.ID, &unit.WorkID, &unit.ComicID, &unit.RelativePath, &unit.Title, &unit.DisplayLabel, &unit.UnitKind, &unit.VolumeNumber, &unit.ChapterNumber, &unit.SortIndex, &unit.PageCount, &unit.FileSize, &unit.CoverURL)
	return unit, err
}

func scanWorkUnitRow(row *sql.Row) (*model.WorkUnit, error) {
	unit, err := scanWorkUnit(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &unit, nil
}
