package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type WorkMetadataUpdate struct {
	Title       *string
	Author      *string
	Publisher   *string
	Year        *int
	Description *string
	Language    *string
	Genre       *string
	Status      *string
}

type WorkOrderUpdate struct {
	WorkID    string
	SortOrder int
}

func ReplaceWorkTags(seriesID string, comicIDs, tagNames []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagIDs, err := ensureTagIDsTx(tx, tagNames)
	if err != nil {
		return err
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		if _, err := tx.Exec(`DELETE FROM "ComicTag" WHERE "comicId" = ?`, comicID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicTag" ("comicId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, comicID, tagID); err != nil {
				return err
			}
		}
	}
	if seriesID != "" {
		if _, err := tx.Exec(`DELETE FROM "ComicSeriesTag" WHERE "seriesId" = ?`, seriesID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicSeriesTag" ("seriesId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, seriesID, tagID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE "ComicSeries" SET "updatedAt" = ? WHERE "id" = ?`, nextSeriesUpdatedAt(), seriesID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func AddWorkTags(seriesIDs, comicIDs, tagNames []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagIDs, err := ensureTagIDsTx(tx, tagNames)
	if err != nil {
		return err
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicTag" ("comicId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, comicID, tagID); err != nil {
				return err
			}
		}
	}
	for _, seriesID := range uniqueWorkIDs(seriesIDs) {
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicSeriesTag" ("seriesId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, seriesID, tagID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE "ComicSeries" SET "updatedAt" = ? WHERE "id" = ?`, nextSeriesUpdatedAt(), seriesID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func RemoveWorkTags(seriesIDs, comicIDs, tagNames []string) error {
	if len(comicIDs) == 0 || len(tagNames) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		for _, name := range tagNames {
			if _, err := tx.Exec(`
				DELETE FROM "ComicTag"
				WHERE "comicId" = ? AND "tagId" IN (SELECT "id" FROM "Tag" WHERE "name" = ?)
			`, comicID, strings.TrimSpace(name)); err != nil {
				return err
			}
		}
	}
	for _, seriesID := range uniqueWorkIDs(seriesIDs) {
		for _, name := range tagNames {
			if _, err := tx.Exec(`
				DELETE FROM "ComicSeriesTag"
				WHERE "seriesId" = ? AND "tagId" IN (SELECT "id" FROM "Tag" WHERE "name" = ?)
			`, seriesID, strings.TrimSpace(name)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE "ComicSeries" SET "updatedAt" = ? WHERE "id" = ?`, nextSeriesUpdatedAt(), seriesID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ReplaceWorkCategories(comicIDs, slugs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	categoryIDs, err := ensureCategoryIDsTx(tx, slugs)
	if err != nil {
		return err
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		if _, err := tx.Exec(`DELETE FROM "ComicCategory" WHERE "comicId" = ?`, comicID); err != nil {
			return err
		}
		for _, categoryID := range categoryIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicCategory" ("comicId", "categoryId") VALUES (?, ?) ON CONFLICT DO NOTHING`, comicID, categoryID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func SetWorkFavorite(userID string, comicIDs []string, favorite bool) error {
	_, err := BatchSetFavorite(userID, uniqueWorkIDs(comicIDs), favorite)
	return err
}

func SetWorkReadingStatus(userID string, comicIDs []string, status string) error {
	if status == "unread" {
		status = ""
	}
	return BatchSetReadingStatus(userID, uniqueWorkIDs(comicIDs), status)
}

func UpdateWorkMetadata(hostType, hostID string, update WorkMetadataUpdate) error {
	if hostType == "work" {
		locked := true
		source := "manual"
		return UpdateLogicalWorkMetadata(hostID, LogicalWorkMetadataUpdate{
			Title: update.Title, Author: update.Author, Publisher: update.Publisher,
			Year: update.Year, Description: update.Description, Language: update.Language,
			Genre: update.Genre, Status: update.Status, MetadataSource: &source, MetadataLocked: &locked,
		})
	}
	if hostType == "series" {
		locked := true
		source := "manual"
		return UpdateSeriesMetadata(hostID, SeriesMetadataUpdate{
			Title: update.Title, Author: update.Author, Publisher: update.Publisher,
			Year: update.Year, Description: update.Description, Language: update.Language,
			Genre: update.Genre, Status: update.Status, MetadataSource: &source, MetadataLocked: &locked,
		})
	}
	fields := map[string]interface{}{"metadataSource": "manual", "updatedAt": time.Now().UTC()}
	appendComicMetadataFields(fields, update)
	if update.Status != nil {
		fields["status"] = *update.Status
	}
	return UpdateComicFields(hostID, fields)
}

func GetComicWorkStatuses(comicIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(comicIDs) == 0 || !workColumnExists("Comic", "status") {
		return result, nil
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		var status string
		err := db.QueryRow(`SELECT "status" FROM "Comic" WHERE "id" = ?`, comicID).Scan(&status)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[comicID] = status
	}
	return result, nil
}

func workColumnExists(table, column string) bool {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&count)
	if count > 0 {
		return true
	}
	rows, err := db.Query(`PRAGMA table_info("` + strings.ReplaceAll(table, `"`, `""`) + `")`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue interface{}
		if rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey) == nil && name == column {
			return true
		}
	}
	return false
}

func UpdateWorkCover(hostType, hostID, coverComicID, coverURL string, aspectRatio float64) error {
	if hostType == "work" {
		locked := true
		source := "comic"
		if coverURL != "" {
			source = "remote"
		}
		update := LogicalWorkCoverUpdate{
			CoverSource: &source,
			CoverLocked: &locked,
		}
		if coverComicID != "" {
			update.CoverComicID = &coverComicID
		}
		if coverURL != "" {
			update.CoverURL = &coverURL
		}
		if aspectRatio > 0 {
			update.CoverAspectRatio = &aspectRatio
		}
		return UpdateLogicalWorkCover(hostID, update)
	}
	if hostType == "series" {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		sets := []string{`"updatedAt" = ?`, `"metadataSource" = 'manual'`, `"metadataLocked" = 1`}
		args := []interface{}{nextSeriesUpdatedAt()}
		if coverURL != "" {
			sets = append(sets, `"coverUrl" = ?`)
			args = append(args, coverURL)
		}
		if coverComicID != "" {
			sets = append(sets, `"coverComicId" = ?`)
			args = append(args, coverComicID)
		}
		if aspectRatio > 0 {
			sets = append(sets, `"coverAspectRatio" = ?`)
			args = append(args, aspectRatio)
		}
		args = append(args, hostID)
		if _, err := tx.Exec(`UPDATE "ComicSeries" SET `+strings.Join(sets, ", ")+` WHERE "id" = ?`, args...); err != nil {
			return err
		}
		return tx.Commit()
	}
	fields := map[string]interface{}{"metadataSource": "manual", "updatedAt": time.Now().UTC()}
	if coverURL != "" {
		fields["coverImageUrl"] = coverURL
	}
	if aspectRatio > 0 {
		fields["coverAspectRatio"] = aspectRatio
	}
	return UpdateComicFields(hostID, fields)
}

func DeleteWorkComics(comicIDs []string) (int64, error) {
	return BatchDeleteComics(uniqueWorkIDs(comicIDs))
}

func ReorderWorkHosts(orders []WorkOrderUpdate) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, order := range orders {
		if order.WorkID == "" {
			return fmt.Errorf("work id is required")
		}
		if _, err := tx.Exec(`
			INSERT INTO "WorkPreference" ("workId", "sortOrder", "updatedAt")
			VALUES (?, ?, ?)
			ON CONFLICT("workId") DO UPDATE SET "sortOrder" = excluded."sortOrder", "updatedAt" = excluded."updatedAt"
		`, order.WorkID, order.SortOrder, time.Now().UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetWorkSortOrders(workIDs []string) (map[string]int, error) {
	result := make(map[string]int)
	if len(workIDs) == 0 || !workTableExists("WorkPreference") {
		return result, nil
	}
	for _, workID := range uniqueWorkIDs(workIDs) {
		var order int
		err := db.QueryRow(`SELECT "sortOrder" FROM "WorkPreference" WHERE "workId" = ?`, workID).Scan(&order)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[workID] = order
	}
	return result, nil
}

func ensureTagIDsTx(tx *sql.Tx, names []string) ([]int64, error) {
	ids := []int64{}
	for _, name := range uniqueWorkIDs(names) {
		if _, err := tx.Exec(`INSERT INTO "Tag" ("name") VALUES (?) ON CONFLICT("name") DO NOTHING`, name); err != nil {
			return nil, err
		}
		var id int64
		if err := tx.QueryRow(`SELECT "id" FROM "Tag" WHERE "name" = ?`, name).Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func ensureCategoryIDsTx(tx *sql.Tx, slugs []string) ([]int64, error) {
	ids := []int64{}
	for _, slug := range uniqueWorkIDs(slugs) {
		var id int64
		err := tx.QueryRow(`SELECT "id" FROM "Category" WHERE "slug" = ?`, slug).Scan(&id)
		if err == sql.ErrNoRows {
			result, insertErr := tx.Exec(`INSERT INTO "Category" ("name", "slug", "icon", "sortOrder") VALUES (?, ?, '', 999)`, slug, slug)
			if insertErr != nil {
				return nil, insertErr
			}
			id, _ = result.LastInsertId()
		} else if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func appendComicMetadataFields(fields map[string]interface{}, update WorkMetadataUpdate) {
	if update.Title != nil {
		fields["title"] = strings.TrimSpace(*update.Title)
		fields["titleSortKey"] = BuildTitleSortKey(*update.Title)
	}
	for key, value := range map[string]*string{
		"author": update.Author, "publisher": update.Publisher, "description": update.Description,
		"language": update.Language, "genre": update.Genre,
	} {
		if value != nil {
			fields[key] = *value
		}
	}
	if update.Year != nil {
		fields["year"] = *update.Year
	}
}

func uniqueWorkIDs(values []string) []string {
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
