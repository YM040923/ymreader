package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func ReplaceWorksForLibrary(libraryID string, worksInput interface{}) error {
	works, err := normalizeWorkInput(worksInput)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM "Work" WHERE "libraryId" = ?`, libraryID); err != nil {
		return err
	}
	for _, work := range works {
		if work.LibraryID == "" {
			work.LibraryID = libraryID
		}
		if work.LibraryID != libraryID {
			return fmt.Errorf("work %s belongs to library %s, want %s", work.ID, work.LibraryID, libraryID)
		}
		if strings.TrimSpace(work.SortTitle) == "" {
			work.SortTitle = BuildTitleSortKey(work.Title)
		}
		if _, err := tx.Exec(`
			INSERT INTO "Work" ("id", "libraryId", "rootRelativePath", "title", "sortTitle", "author", "description", "coverComicId", "coverUrl", "createdAt", "updatedAt")
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, work.ID, work.LibraryID, work.RootRelativePath, work.Title, work.SortTitle, work.Author, work.Description, work.CoverComicID, work.CoverURL); err != nil {
			return err
		}
		for _, unit := range work.Units {
			if unit.WorkID == "" {
				unit.WorkID = work.ID
			}
			if unit.WorkID != work.ID {
				return fmt.Errorf("unit %s belongs to work %s, want %s", unit.ID, unit.WorkID, work.ID)
			}
			if unit.Title == "" || unit.RelativePath == "" || unit.PageCount == 0 || unit.FileSize == 0 {
				fillWorkUnitFromComic(tx, &unit)
			}
			if _, err := tx.Exec(`
				INSERT INTO "WorkUnit" ("id", "workId", "comicId", "title", "displayLabel", "relativePath", "kind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "createdAt", "updatedAt")
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, unit.ID, unit.WorkID, unit.ComicID, unit.Title, unit.DisplayLabel, unit.RelativePath, unit.Kind, unit.VolumeNumber, unit.ChapterNumber, unit.SortIndex, unit.PageCount, unit.FileSize); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func normalizeWorkInput(input interface{}) ([]model.Work, error) {
	switch works := input.(type) {
	case []model.Work:
		return works, nil
	case []workmodel.DetectedWork:
		out := make([]model.Work, 0, len(works))
		for _, detected := range works {
			work := model.Work{
				ID:               detected.ID,
				LibraryID:        detected.LibraryID,
				RootRelativePath: detected.RootRelativePath,
				Title:            detected.Title,
				SortTitle:        detected.SortTitle,
			}
			for _, detectedUnit := range detected.Units {
				work.Units = append(work.Units, model.WorkUnit{
					ID:            detectedUnit.ID,
					WorkID:        detected.ID,
					ComicID:       detectedUnit.ComicID,
					Title:         detectedUnit.Title,
					DisplayLabel:  detectedUnit.DisplayLabel,
					RelativePath:  detectedUnit.RelativePath,
					Kind:          string(detectedUnit.Kind),
					VolumeNumber:  detectedUnit.VolumeNumber,
					ChapterNumber: detectedUnit.ChapterNumber,
					SortIndex:     detectedUnit.SortIndex,
					PageCount:     detectedUnit.PageCount,
					FileSize:      detectedUnit.FileSize,
				})
			}
			out = append(out, work)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported work input type %T", input)
	}
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
}

func ListWorks(libraryID string) ([]model.Work, error) {
	rows, err := db.Query(`
		SELECT w."id", w."libraryId", w."rootRelativePath", w."title", w."sortTitle", w."author", w."description", w."coverComicId", w."coverUrl",
		       COUNT(u."id") AS itemCount, w."createdAt", w."updatedAt"
		FROM "Work" w
		LEFT JOIN "WorkUnit" u ON u."workId" = w."id"
		WHERE w."libraryId" = ?
		GROUP BY w."id"
		ORDER BY w."sortTitle", w."title", w."id"
	`, libraryID)
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

func GetWorkDetail(workID string) (*model.Work, error) {
	row := db.QueryRow(`
		SELECT w."id", w."libraryId", w."rootRelativePath", w."title", w."sortTitle", w."author", w."description", w."coverComicId", w."coverUrl",
		       COUNT(u."id") AS itemCount, w."createdAt", w."updatedAt"
		FROM "Work" w
		LEFT JOIN "WorkUnit" u ON u."workId" = w."id"
		WHERE w."id" = ?
		GROUP BY w."id"
	`, workID)
	work, err := scanWorkSummary(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	units, err := listWorkUnits(workID)
	if err != nil {
		return nil, err
	}
	work.Units = units
	return &work, nil
}

func GetFirstWorkUnit(workID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? ORDER BY "sortIndex", "id" LIMIT 1`, workID))
}

func GetWorkUnit(unitID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "id" = ?`, unitID))
}

func GetAdjacentWorkUnit(workID, unitID string, direction int) (*model.WorkUnit, error) {
	var sortIndex int
	if err := db.QueryRow(`SELECT "sortIndex" FROM "WorkUnit" WHERE "workId" = ? AND "id" = ?`, workID, unitID).Scan(&sortIndex); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if direction < 0 {
		return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? AND ("sortIndex" < ? OR ("sortIndex" = ? AND "id" < ?)) ORDER BY "sortIndex" DESC, "id" DESC LIMIT 1`, workID, sortIndex, sortIndex, unitID))
	}
	if direction > 0 {
		return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "workId" = ? AND ("sortIndex" > ? OR ("sortIndex" = ? AND "id" > ?)) ORDER BY "sortIndex", "id" LIMIT 1`, workID, sortIndex, sortIndex, unitID))
	}
	return nil, nil
}

func SetUserWorkProgress(progress *model.UserWorkProgress) error {
	if progress == nil {
		return fmt.Errorf("progress is nil")
	}
	updatedAt := progress.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := db.Exec(`
		INSERT INTO "UserWorkProgress" ("userId", "workId", "unitId", "lastPage", "progress", "updatedAt")
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT("userId", "workId") DO UPDATE SET
			"unitId" = excluded."unitId",
			"lastPage" = excluded."lastPage",
			"progress" = excluded."progress",
			"updatedAt" = excluded."updatedAt"
	`, progress.UserID, progress.WorkID, progress.UnitID, progress.LastPage, progress.Progress, updatedAt)
	return err
}

func GetUserWorkProgress(userID, workID string) (*model.UserWorkProgress, error) {
	var progress model.UserWorkProgress
	err := db.QueryRow(`SELECT "userId", "workId", "unitId", "lastPage", "progress", "updatedAt" FROM "UserWorkProgress" WHERE "userId" = ? AND "workId" = ?`, userID, workID).Scan(&progress.UserID, &progress.WorkID, &progress.UnitID, &progress.LastPage, &progress.Progress, &progress.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

func GetWorkUnitByComicID(comicID string) (*model.WorkUnit, error) {
	return scanWorkUnitRow(db.QueryRow(workUnitSelectSQL+` WHERE "comicId" = ? ORDER BY "workId", "sortIndex", "id" LIMIT 1`, comicID))
}

func ListWorkSourceItems(libraryID string) ([]workmodel.SourceItem, error) {
	rows, err := db.Query(`
		SELECT c."id", c."libraryId", c."title", COALESCE(NULLIF(c."relativePath", ''), c."filename"), c."pageCount", c."fileSize"
		FROM "Comic" c
		WHERE c."libraryId" = ?
		ORDER BY c."relativePath", c."filename", c."id"
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []workmodel.SourceItem{}
	for rows.Next() {
		var item workmodel.SourceItem
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.Title, &item.RelativePath, &item.PageCount, &item.FileSize); err != nil {
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

func scanWorkSummary(scanner workScanner) (model.Work, error) {
	var work model.Work
	err := scanner.Scan(&work.ID, &work.LibraryID, &work.RootRelativePath, &work.Title, &work.SortTitle, &work.Author, &work.Description, &work.CoverComicID, &work.CoverURL, &work.ItemCount, &work.CreatedAt, &work.UpdatedAt)
	return work, err
}

const workUnitSelectSQL = `SELECT "id", "workId", "comicId", "title", "displayLabel", "relativePath", "kind", "volumeNumber", "chapterNumber", "sortIndex", "pageCount", "fileSize", "createdAt", "updatedAt" FROM "WorkUnit"`

func scanWorkUnit(scanner workScanner) (model.WorkUnit, error) {
	var unit model.WorkUnit
	err := scanner.Scan(&unit.ID, &unit.WorkID, &unit.ComicID, &unit.Title, &unit.DisplayLabel, &unit.RelativePath, &unit.Kind, &unit.VolumeNumber, &unit.ChapterNumber, &unit.SortIndex, &unit.PageCount, &unit.FileSize, &unit.CreatedAt, &unit.UpdatedAt)
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
