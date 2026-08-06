package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

type LogicalWork struct {
	ID                      string
	LibraryID               string
	RootPath                string
	ContentType             string
	Title                   string
	SortTitle               string
	Author                  string
	Publisher               string
	Year                    *int
	Description             string
	Language                string
	Genre                   string
	Status                  string
	MetadataSource          string
	MetadataLocked          bool
	ExternalRating          *float64
	ExternalRatingMax       *float64
	ExternalRatingSource    string
	ExternalRatingUpdatedAt *time.Time
	CoverSource             string
	CoverComicID            string
	CoverPage               *int
	CoverURL                string
	CoverAspectRatio        float64
	CoverLocked             bool
	LayoutFingerprint       string
	SourceFingerprint       string
	ScrapeStatus            string
	LastScrapedAt           *time.Time
	ScrapeError             string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	LastSeenAt              *time.Time
	MissingSince            *time.Time
}

type LogicalWorkSeed struct {
	ID                string
	LibraryID         string
	RootPath          string
	ContentType       string
	Title             string
	CoverComicID      string
	CoverPage         *int
	CoverAspectRatio  float64
	LayoutFingerprint string
	SourceFingerprint string
}

type LogicalWorkMetadataUpdate struct {
	Title                   *string
	Author                  *string
	Publisher               *string
	Year                    *int
	Description             *string
	Language                *string
	Genre                   *string
	Status                  *string
	MetadataSource          *string
	MetadataLocked          *bool
	ExternalRating          *float64
	ExternalRatingMax       *float64
	ExternalRatingSource    *string
	ExternalRatingUpdatedAt *time.Time
	ScrapeStatus            *string
	LastScrapedAt           *time.Time
	ScrapeError             *string
}

type LogicalWorkCoverUpdate struct {
	CoverSource      *string
	CoverComicID     *string
	CoverPage        *int
	CoverURL         *string
	CoverAspectRatio *float64
	CoverLocked      *bool
}

func UpdateLogicalWorkScrapeState(workID, status, scrapeError string, scrapedAt *time.Time) error {
	if !LogicalWorkPersistenceAvailable() {
		return nil
	}
	sets := []string{`"scrapeStatus" = ?`, `"scrapeError" = ?`, `"updatedAt" = ?`}
	args := []interface{}{strings.TrimSpace(status), strings.TrimSpace(scrapeError), time.Now().UTC()}
	if scrapedAt != nil {
		sets = append(sets, `"lastScrapedAt" = ?`)
		args = append(args, *scrapedAt)
	}
	args = append(args, workID)
	_, err := db.Exec(`UPDATE "LogicalWork" SET `+strings.Join(sets, ", ")+` WHERE "id" = ?`, args...)
	return err
}

func LogicalWorkPersistenceAvailable() bool {
	return db != nil && workTableExists("LogicalWork")
}

func UpsertDetectedLogicalWorks(seeds []LogicalWorkSeed) error {
	if len(seeds) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	for _, seed := range seeds {
		root := workmodel.NormalizePath(seed.RootPath)
		if strings.TrimSpace(seed.LibraryID) == "" || root == "" {
			continue
		}
		id := workmodel.StableWorkID(seed.LibraryID, root)
		contentType := strings.TrimSpace(seed.ContentType)
		if contentType == "" {
			contentType = "comic"
		}
		title := strings.TrimSpace(seed.Title)
		if title == "" {
			title = pathBase(root)
		}
		coverSource := "auto"
		if seed.CoverComicID != "" {
			coverSource = "comic"
		}
		if _, err := tx.Exec(`
			INSERT INTO "LogicalWork" (
				"id", "libraryId", "rootPath", "contentType", "title", "sortTitle",
				"coverSource", "coverComicId", "coverPage", "coverAspectRatio",
				"layoutFingerprint", "sourceFingerprint", "createdAt", "updatedAt",
				"lastSeenAt", "missingSince"
			) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, NULL)
			ON CONFLICT("id") DO UPDATE SET
				"libraryId" = excluded."libraryId",
				"rootPath" = excluded."rootPath",
				"contentType" = excluded."contentType",
				"title" = CASE
					WHEN "LogicalWork"."metadataLocked" = 1 OR "LogicalWork"."metadataSource" != ''
						OR "LogicalWork"."title" != ?
					THEN "LogicalWork"."title" ELSE excluded."title"
				END,
				"sortTitle" = CASE
					WHEN "LogicalWork"."metadataLocked" = 1 OR "LogicalWork"."metadataSource" != ''
						OR "LogicalWork"."title" != ?
					THEN "LogicalWork"."sortTitle" ELSE excluded."sortTitle"
				END,
				"coverSource" = CASE
					WHEN "LogicalWork"."coverLocked" = 1 OR "LogicalWork"."coverUrl" != ''
					THEN "LogicalWork"."coverSource" ELSE excluded."coverSource"
				END,
				"coverComicId" = CASE
					WHEN "LogicalWork"."coverLocked" = 1 OR "LogicalWork"."coverUrl" != ''
					THEN "LogicalWork"."coverComicId" ELSE excluded."coverComicId"
				END,
				"coverPage" = CASE
					WHEN "LogicalWork"."coverLocked" = 1 OR "LogicalWork"."coverUrl" != ''
					THEN "LogicalWork"."coverPage" ELSE excluded."coverPage"
				END,
				"coverAspectRatio" = CASE
					WHEN "LogicalWork"."coverLocked" = 1 OR "LogicalWork"."coverUrl" != ''
					THEN "LogicalWork"."coverAspectRatio" ELSE excluded."coverAspectRatio"
				END,
				"layoutFingerprint" = excluded."layoutFingerprint",
				"sourceFingerprint" = excluded."sourceFingerprint",
				"updatedAt" = "LogicalWork"."updatedAt",
				"lastSeenAt" = excluded."lastSeenAt",
				"missingSince" = NULL
		`, id, seed.LibraryID, root, contentType, title, BuildTitleSortKey(title),
			coverSource, seed.CoverComicID, seed.CoverPage, seed.CoverAspectRatio,
			seed.LayoutFingerprint, seed.SourceFingerprint, now, now, now,
			pathBase(root), pathBase(root)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func GetLogicalWork(id string) (*LogicalWork, error) {
	row := db.QueryRow(`
		SELECT "id", "libraryId", "rootPath", "contentType", "title", "sortTitle",
		       "author", "publisher", "year", "description", "language", "genre", "status",
		       "metadataSource", "metadataLocked", "externalRating", "externalRatingMax",
		       "externalRatingSource", "externalRatingUpdatedAt", "coverSource",
		       "coverComicId", "coverPage", "coverUrl", "coverAspectRatio", "coverLocked",
		       "layoutFingerprint", "sourceFingerprint", "scrapeStatus", "lastScrapedAt",
		       "scrapeError", "createdAt", "updatedAt", "lastSeenAt", "missingSince"
		FROM "LogicalWork" WHERE "id" = ?
	`, id)
	work, err := scanLogicalWork(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return work, err
}

func GetLogicalWorksByIDs(ids []string) (map[string]LogicalWork, error) {
	result := make(map[string]LogicalWork, len(ids))
	ids = uniqueWorkIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	rows, err := db.Query(`
		SELECT "id", "libraryId", "rootPath", "contentType", "title", "sortTitle",
		       "author", "publisher", "year", "description", "language", "genre", "status",
		       "metadataSource", "metadataLocked", "externalRating", "externalRatingMax",
		       "externalRatingSource", "externalRatingUpdatedAt", "coverSource",
		       "coverComicId", "coverPage", "coverUrl", "coverAspectRatio", "coverLocked",
		       "layoutFingerprint", "sourceFingerprint", "scrapeStatus", "lastScrapedAt",
		       "scrapeError", "createdAt", "updatedAt", "lastSeenAt", "missingSince"
		FROM "LogicalWork" WHERE "id" IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		work, scanErr := scanLogicalWork(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[work.ID] = *work
	}
	return result, rows.Err()
}

type logicalWorkScanner interface {
	Scan(dest ...interface{}) error
}

func scanLogicalWork(scanner logicalWorkScanner) (*LogicalWork, error) {
	var work LogicalWork
	var year sql.NullInt64
	var rating, ratingMax sql.NullFloat64
	var externalUpdated, lastScraped, lastSeen, missing sql.NullTime
	var coverComic sql.NullString
	var coverPage sql.NullInt64
	if err := scanner.Scan(
		&work.ID, &work.LibraryID, &work.RootPath, &work.ContentType, &work.Title, &work.SortTitle,
		&work.Author, &work.Publisher, &year, &work.Description, &work.Language, &work.Genre, &work.Status,
		&work.MetadataSource, &work.MetadataLocked, &rating, &ratingMax,
		&work.ExternalRatingSource, &externalUpdated, &work.CoverSource,
		&coverComic, &coverPage, &work.CoverURL, &work.CoverAspectRatio, &work.CoverLocked,
		&work.LayoutFingerprint, &work.SourceFingerprint, &work.ScrapeStatus, &lastScraped,
		&work.ScrapeError, &work.CreatedAt, &work.UpdatedAt, &lastSeen, &missing,
	); err != nil {
		return nil, err
	}
	if year.Valid {
		value := int(year.Int64)
		work.Year = &value
	}
	if rating.Valid {
		value := rating.Float64
		work.ExternalRating = &value
	}
	if ratingMax.Valid {
		value := ratingMax.Float64
		work.ExternalRatingMax = &value
	}
	if externalUpdated.Valid {
		value := externalUpdated.Time
		work.ExternalRatingUpdatedAt = &value
	}
	if coverComic.Valid {
		work.CoverComicID = coverComic.String
	}
	if coverPage.Valid {
		value := int(coverPage.Int64)
		work.CoverPage = &value
	}
	if lastScraped.Valid {
		value := lastScraped.Time
		work.LastScrapedAt = &value
	}
	if lastSeen.Valid {
		value := lastSeen.Time
		work.LastSeenAt = &value
	}
	if missing.Valid {
		value := missing.Time
		work.MissingSince = &value
	}
	return &work, nil
}

func UpdateLogicalWorkMetadata(id string, update LogicalWorkMetadataUpdate) error {
	sets := []string{`"updatedAt" = ?`}
	args := []interface{}{time.Now().UTC()}
	appendValue := func(column string, value interface{}) {
		sets = append(sets, `"`+column+`" = ?`)
		args = append(args, value)
	}
	if update.Title != nil {
		title := strings.TrimSpace(*update.Title)
		appendValue("title", title)
		appendValue("sortTitle", BuildTitleSortKey(title))
	}
	if update.Author != nil {
		appendValue("author", strings.TrimSpace(*update.Author))
	}
	if update.Publisher != nil {
		appendValue("publisher", strings.TrimSpace(*update.Publisher))
	}
	if update.Year != nil {
		appendValue("year", *update.Year)
	}
	if update.Description != nil {
		appendValue("description", strings.TrimSpace(*update.Description))
	}
	if update.Language != nil {
		appendValue("language", strings.TrimSpace(*update.Language))
	}
	if update.Genre != nil {
		appendValue("genre", strings.TrimSpace(*update.Genre))
	}
	if update.Status != nil {
		appendValue("status", strings.TrimSpace(*update.Status))
	}
	if update.MetadataSource != nil {
		appendValue("metadataSource", strings.TrimSpace(*update.MetadataSource))
	}
	if update.MetadataLocked != nil {
		appendValue("metadataLocked", *update.MetadataLocked)
	}
	if update.ExternalRating != nil {
		appendValue("externalRating", *update.ExternalRating)
	}
	if update.ExternalRatingMax != nil {
		appendValue("externalRatingMax", *update.ExternalRatingMax)
	}
	if update.ExternalRatingSource != nil {
		appendValue("externalRatingSource", strings.TrimSpace(*update.ExternalRatingSource))
	}
	if update.ExternalRatingUpdatedAt != nil {
		appendValue("externalRatingUpdatedAt", *update.ExternalRatingUpdatedAt)
	}
	if update.ScrapeStatus != nil {
		appendValue("scrapeStatus", strings.TrimSpace(*update.ScrapeStatus))
	}
	if update.LastScrapedAt != nil {
		appendValue("lastScrapedAt", *update.LastScrapedAt)
	}
	if update.ScrapeError != nil {
		appendValue("scrapeError", strings.TrimSpace(*update.ScrapeError))
	}
	args = append(args, id)
	result, err := db.Exec(`UPDATE "LogicalWork" SET `+strings.Join(sets, ", ")+` WHERE "id" = ?`, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func UpdateLogicalWorkCover(id string, update LogicalWorkCoverUpdate) error {
	sets := []string{`"updatedAt" = ?`}
	args := []interface{}{time.Now().UTC()}
	appendValue := func(column string, value interface{}) {
		sets = append(sets, `"`+column+`" = ?`)
		args = append(args, value)
	}
	if update.CoverSource != nil {
		appendValue("coverSource", strings.TrimSpace(*update.CoverSource))
	}
	if update.CoverComicID != nil {
		if strings.TrimSpace(*update.CoverComicID) == "" {
			appendValue("coverComicId", nil)
		} else {
			appendValue("coverComicId", strings.TrimSpace(*update.CoverComicID))
		}
	}
	if update.CoverPage != nil {
		appendValue("coverPage", *update.CoverPage)
	}
	if update.CoverURL != nil {
		appendValue("coverUrl", strings.TrimSpace(*update.CoverURL))
	}
	if update.CoverAspectRatio != nil {
		appendValue("coverAspectRatio", *update.CoverAspectRatio)
	}
	if update.CoverLocked != nil {
		appendValue("coverLocked", *update.CoverLocked)
	}
	args = append(args, id)
	result, err := db.Exec(`UPDATE "LogicalWork" SET `+strings.Join(sets, ", ")+` WHERE "id" = ?`, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func GetLogicalWorkTags(workID string) ([]Tag, error) {
	rows, err := db.Query(`
		SELECT t."id", t."name", t."color"
		FROM "LogicalWorkTag" wt
		JOIN "Tag" t ON t."id" = wt."tagId"
		WHERE wt."workId" = ?
		ORDER BY t."name", t."id"
	`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Tag{}
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color); err != nil {
			return nil, err
		}
		result = append(result, tag)
	}
	return result, rows.Err()
}

func GetLogicalWorkCategories(workID string) ([]ComicCategoryInfo, error) {
	rows, err := db.Query(`
		SELECT c."id", c."name", c."slug", c."icon"
		FROM "LogicalWorkCategory" wc
		JOIN "Category" c ON c."id" = wc."categoryId"
		WHERE wc."workId" = ?
		ORDER BY c."sortOrder", c."id"
	`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ComicCategoryInfo{}
	for rows.Next() {
		var category ComicCategoryInfo
		if err := rows.Scan(&category.ID, &category.Name, &category.Slug, &category.Icon); err != nil {
			return nil, err
		}
		result = append(result, category)
	}
	return result, rows.Err()
}

func GetLogicalWorkTagsByWorkIDs(workIDs []string) (map[string][]Tag, error) {
	result := make(map[string][]Tag, len(workIDs))
	workIDs = uniqueWorkIDs(workIDs)
	if len(workIDs) == 0 {
		return result, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(workIDs)), ",")
	args := make([]interface{}, len(workIDs))
	for index, workID := range workIDs {
		args[index] = workID
	}
	rows, err := db.Query(`
		SELECT wt."workId", t."id", t."name", t."color"
		FROM "LogicalWorkTag" wt
		JOIN "Tag" t ON t."id" = wt."tagId"
		WHERE wt."workId" IN (`+placeholders+`)
		ORDER BY wt."workId", t."name", t."id"
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var workID string
		var tag Tag
		if err := rows.Scan(&workID, &tag.ID, &tag.Name, &tag.Color); err != nil {
			return nil, err
		}
		result[workID] = append(result[workID], tag)
	}
	return result, rows.Err()
}

func GetLogicalWorkCategoriesByWorkIDs(workIDs []string) (map[string][]ComicCategoryInfo, error) {
	result := make(map[string][]ComicCategoryInfo, len(workIDs))
	workIDs = uniqueWorkIDs(workIDs)
	if len(workIDs) == 0 {
		return result, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(workIDs)), ",")
	args := make([]interface{}, len(workIDs))
	for index, workID := range workIDs {
		args[index] = workID
	}
	rows, err := db.Query(`
		SELECT wc."workId", c."id", c."name", c."slug", c."icon"
		FROM "LogicalWorkCategory" wc
		JOIN "Category" c ON c."id" = wc."categoryId"
		WHERE wc."workId" IN (`+placeholders+`)
		ORDER BY wc."workId", c."sortOrder", c."id"
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var workID string
		var category ComicCategoryInfo
		if err := rows.Scan(&workID, &category.ID, &category.Name, &category.Slug, &category.Icon); err != nil {
			return nil, err
		}
		result[workID] = append(result[workID], category)
	}
	return result, rows.Err()
}

func SeedLogicalWorkRelations(workID string, tags []ComicTagInfo, categories []ComicCategoryInfo) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		if strings.TrimSpace(tag.Name) != "" {
			tagNames = append(tagNames, tag.Name)
		}
	}
	tagIDs, err := ensureTagIDsTx(tx, tagNames)
	if err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(`
			INSERT INTO "LogicalWorkTag" ("workId", "tagId")
			VALUES (?, ?) ON CONFLICT DO NOTHING
		`, workID, tagID); err != nil {
			return err
		}
	}
	categorySlugs := make([]string, 0, len(categories))
	for _, category := range categories {
		if strings.TrimSpace(category.Slug) != "" {
			categorySlugs = append(categorySlugs, category.Slug)
		}
	}
	categoryIDs, err := ensureCategoryIDsTx(tx, categorySlugs)
	if err != nil {
		return err
	}
	for _, categoryID := range categoryIDs {
		if _, err := tx.Exec(`
			INSERT INTO "LogicalWorkCategory" ("workId", "categoryId")
			VALUES (?, ?) ON CONFLICT DO NOTHING
		`, workID, categoryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ResolveLogicalWorkAlias(aliasType, aliasID string) (string, error) {
	var workID string
	err := db.QueryRow(`
		SELECT "workId" FROM "LogicalWorkAlias"
		WHERE "aliasType" = ? AND "aliasId" = ?
	`, strings.TrimSpace(aliasType), strings.TrimSpace(aliasID)).Scan(&workID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return workID, err
}

func MigrateComicSeriesToLogicalWorks() error {
	if !LogicalWorkPersistenceAvailable() || !workTableExists("ComicSeries") {
		return nil
	}
	rows, err := db.Query(`
		SELECT "id", "libraryId", "rootRelativePath", "title", "sortTitle",
		       "coverComicId", "coverUrl", "coverAspectRatio", "author", "description",
		       "year", "publisher", "language", "genre", "status", "metadataSource",
		       "externalRating", "externalRatingMax", "externalRatingSource",
		       "externalRatingUpdatedAt", "metadataLocked", "createdAt", "updatedAt"
		FROM "ComicSeries"
		ORDER BY "libraryId", "rootRelativePath"
	`)
	if err != nil {
		return err
	}
	type seriesMigrationRow struct {
		id, libraryID, root, title, sortTitle           string
		coverComicID, coverURL                          string
		coverAspectRatio                                float64
		author, description, publisher, language, genre string
		status, metadataSource, externalRatingSource    string
		year                                            sql.NullInt64
		externalRating, externalRatingMax               sql.NullFloat64
		externalRatingUpdatedAt                         sql.NullTime
		metadataLocked                                  bool
		createdAt, updatedAt                            time.Time
	}
	var seriesRows []seriesMigrationRow
	for rows.Next() {
		var item seriesMigrationRow
		if err := rows.Scan(
			&item.id, &item.libraryID, &item.root, &item.title, &item.sortTitle,
			&item.coverComicID, &item.coverURL, &item.coverAspectRatio, &item.author, &item.description,
			&item.year, &item.publisher, &item.language, &item.genre, &item.status, &item.metadataSource,
			&item.externalRating, &item.externalRatingMax, &item.externalRatingSource,
			&item.externalRatingUpdatedAt, &item.metadataLocked, &item.createdAt, &item.updatedAt,
		); err != nil {
			rows.Close()
			return err
		}
		seriesRows = append(seriesRows, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, series := range seriesRows {
		root := workmodel.NormalizePath(series.root)
		if root == "" {
			continue
		}
		workID := workmodel.StableWorkID(series.libraryID, root)
		coverSource := "auto"
		if series.coverURL != "" {
			coverSource = "remote"
		} else if series.coverComicID != "" {
			coverSource = "comic"
		}
		coverLocked := series.metadataLocked && series.coverURL != ""
		var year interface{}
		if series.year.Valid {
			year = series.year.Int64
		}
		var rating interface{}
		if series.externalRating.Valid {
			rating = series.externalRating.Float64
		}
		var ratingMax interface{}
		if series.externalRatingMax.Valid {
			ratingMax = series.externalRatingMax.Float64
		}
		var ratingUpdated interface{}
		if series.externalRatingUpdatedAt.Valid {
			ratingUpdated = series.externalRatingUpdatedAt.Time
		}
		if _, err := tx.Exec(`
			INSERT INTO "LogicalWork" (
				"id", "libraryId", "rootPath", "contentType", "title", "sortTitle",
				"author", "publisher", "year", "description", "language", "genre", "status",
				"metadataSource", "metadataLocked", "externalRating", "externalRatingMax",
				"externalRatingSource", "externalRatingUpdatedAt", "coverSource",
				"coverComicId", "coverUrl", "coverAspectRatio", "coverLocked",
				"createdAt", "updatedAt", "lastSeenAt"
			) VALUES (?, ?, ?, 'comic', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			          NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
			ON CONFLICT("id") DO UPDATE SET
				"title" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."title" ELSE excluded."title" END,
				"sortTitle" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."sortTitle" ELSE excluded."sortTitle" END,
				"author" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."author" ELSE excluded."author" END,
				"publisher" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."publisher" ELSE excluded."publisher" END,
				"year" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."year" ELSE excluded."year" END,
				"description" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."description" ELSE excluded."description" END,
				"language" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."language" ELSE excluded."language" END,
				"genre" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."genre" ELSE excluded."genre" END,
				"status" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."status" ELSE excluded."status" END,
				"metadataSource" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."metadataSource" ELSE excluded."metadataSource" END,
				"metadataLocked" = MAX("LogicalWork"."metadataLocked", excluded."metadataLocked"),
				"externalRating" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."externalRating" ELSE excluded."externalRating" END,
				"externalRatingMax" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."externalRatingMax" ELSE excluded."externalRatingMax" END,
				"externalRatingSource" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."externalRatingSource" ELSE excluded."externalRatingSource" END,
				"externalRatingUpdatedAt" = CASE WHEN "LogicalWork"."metadataLocked" = 1 THEN "LogicalWork"."externalRatingUpdatedAt" ELSE excluded."externalRatingUpdatedAt" END,
				"coverSource" = CASE WHEN "LogicalWork"."coverLocked" = 1 THEN "LogicalWork"."coverSource" ELSE excluded."coverSource" END,
				"coverComicId" = CASE WHEN "LogicalWork"."coverLocked" = 1 THEN "LogicalWork"."coverComicId" ELSE excluded."coverComicId" END,
				"coverUrl" = CASE WHEN "LogicalWork"."coverLocked" = 1 THEN "LogicalWork"."coverUrl" ELSE excluded."coverUrl" END,
				"coverAspectRatio" = CASE WHEN "LogicalWork"."coverLocked" = 1 THEN "LogicalWork"."coverAspectRatio" ELSE excluded."coverAspectRatio" END,
				"coverLocked" = MAX("LogicalWork"."coverLocked", excluded."coverLocked"),
				"updatedAt" = MAX("LogicalWork"."updatedAt", excluded."updatedAt"),
				"lastSeenAt" = excluded."lastSeenAt"
		`, workID, series.libraryID, root, series.title, firstNonEmptyStore(series.sortTitle, BuildTitleSortKey(series.title)),
			series.author, series.publisher, year, series.description, series.language, series.genre, series.status,
			series.metadataSource, series.metadataLocked, rating, ratingMax, series.externalRatingSource, ratingUpdated,
			coverSource, series.coverComicID, series.coverURL, series.coverAspectRatio, coverLocked,
			series.createdAt, series.updatedAt, time.Now().UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT INTO "LogicalWorkAlias" ("aliasType", "aliasId", "workId")
			VALUES ('comic-series', ?, ?)
			ON CONFLICT("aliasType", "aliasId") DO UPDATE SET "workId" = excluded."workId"
		`, series.id, workID); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO "LogicalWorkTag" ("workId", "tagId")
			SELECT ?, "tagId" FROM "ComicSeriesTag" WHERE "seriesId" = ?
		`, workID, series.id); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO "LogicalWorkTag" ("workId", "tagId")
			SELECT ?, ct."tagId"
			FROM "ComicSeriesItem" si
			JOIN "ComicTag" ct ON ct."comicId" = si."comicId"
			WHERE si."seriesId" = ?
		`, workID, series.id); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO "LogicalWorkCategory" ("workId", "categoryId")
			SELECT ?, cc."categoryId"
			FROM "ComicSeriesItem" si
			JOIN "ComicCategory" cc ON cc."comicId" = si."comicId"
			WHERE si."seriesId" = ?
		`, workID, series.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func pathBase(value string) string {
	value = strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
	if index := strings.LastIndex(value, "/"); index >= 0 {
		return value[index+1:]
	}
	return value
}

func firstNonEmptyStore(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ReplaceLogicalWorkTags(workID string, names []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagIDs, err := ensureTagIDsTx(tx, names)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM "LogicalWorkTag" WHERE "workId" = ?`, workID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(`INSERT INTO "LogicalWorkTag" ("workId", "tagId") VALUES (?, ?)`, workID, tagID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ReplaceLogicalWorkCategories(workID string, slugs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	categoryIDs, err := ensureCategoryIDsTx(tx, slugs)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM "LogicalWorkCategory" WHERE "workId" = ?`, workID); err != nil {
		return err
	}
	for _, categoryID := range categoryIDs {
		if _, err := tx.Exec(`INSERT INTO "LogicalWorkCategory" ("workId", "categoryId") VALUES (?, ?)`, workID, categoryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ReplaceLogicalWorkAndComicTags(workID string, comicIDs, names []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagIDs, err := ensureTagIDsTx(tx, names)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM "LogicalWorkTag" WHERE "workId" = ?`, workID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(`INSERT INTO "LogicalWorkTag" ("workId", "tagId") VALUES (?, ?)`, workID, tagID); err != nil {
			return err
		}
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
	_, err = tx.Exec(`UPDATE "LogicalWork" SET "updatedAt" = ? WHERE "id" = ?`, time.Now().UTC(), workID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func AddLogicalWorkAndComicTags(workIDs, comicIDs, names []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tagIDs, err := ensureTagIDsTx(tx, names)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, workID := range uniqueWorkIDs(workIDs) {
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "LogicalWorkTag" ("workId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, workID, tagID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE "LogicalWork" SET "updatedAt" = ? WHERE "id" = ?`, now, workID); err != nil {
			return err
		}
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(`INSERT INTO "ComicTag" ("comicId", "tagId") VALUES (?, ?) ON CONFLICT DO NOTHING`, comicID, tagID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func RemoveLogicalWorkAndComicTags(workIDs, comicIDs, names []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	for _, workID := range uniqueWorkIDs(workIDs) {
		for _, name := range uniqueWorkIDs(names) {
			if _, err := tx.Exec(`
				DELETE FROM "LogicalWorkTag"
				WHERE "workId" = ? AND "tagId" IN (SELECT "id" FROM "Tag" WHERE "name" = ?)
			`, workID, name); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE "LogicalWork" SET "updatedAt" = ? WHERE "id" = ?`, now, workID); err != nil {
			return err
		}
	}
	for _, comicID := range uniqueWorkIDs(comicIDs) {
		for _, name := range uniqueWorkIDs(names) {
			if _, err := tx.Exec(`
				DELETE FROM "ComicTag"
				WHERE "comicId" = ? AND "tagId" IN (SELECT "id" FROM "Tag" WHERE "name" = ?)
			`, comicID, name); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func ReplaceLogicalWorkAndComicCategories(workID string, comicIDs, slugs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	categoryIDs, err := ensureCategoryIDsTx(tx, slugs)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM "LogicalWorkCategory" WHERE "workId" = ?`, workID); err != nil {
		return err
	}
	for _, categoryID := range categoryIDs {
		if _, err := tx.Exec(`INSERT INTO "LogicalWorkCategory" ("workId", "categoryId") VALUES (?, ?)`, workID, categoryID); err != nil {
			return err
		}
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
	if _, err := tx.Exec(`UPDATE "LogicalWork" SET "updatedAt" = ? WHERE "id" = ?`, time.Now().UTC(), workID); err != nil {
		return err
	}
	return tx.Commit()
}

func MarkMissingLogicalWorks(libraryID string, visibleWorkIDs []string) error {
	now := time.Now().UTC()
	visibleWorkIDs = uniqueWorkIDs(visibleWorkIDs)
	if len(visibleWorkIDs) == 0 {
		_, err := db.Exec(`
			UPDATE "LogicalWork"
			SET "missingSince" = COALESCE("missingSince", ?), "updatedAt" = ?
			WHERE "libraryId" = ?
		`, now, now, libraryID)
		return err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(visibleWorkIDs)), ",")
	args := []interface{}{now, now, libraryID}
	for _, id := range visibleWorkIDs {
		args = append(args, id)
	}
	_, err := db.Exec(fmt.Sprintf(`
		UPDATE "LogicalWork"
		SET "missingSince" = COALESCE("missingSince", ?), "updatedAt" = ?
		WHERE "libraryId" = ? AND "id" NOT IN (%s)
	`, placeholders), args...)
	return err
}
