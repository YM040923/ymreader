package handler

import (
	"log"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func syncMetadataToComicIDs(comicIDs []string, meta service.ComicMetadata, fieldsSet map[string]bool, overwrite, syncRating bool) (successCount, errorCount int, err error) {
	applyAll := len(fieldsSet) == 0
	shouldApply := func(field string) bool {
		return applyAll || fieldsSet[field]
	}

	ratingUpdates := map[string]interface{}{}
	if syncRating && shouldApply("rating") {
		ratingUpdates = service.BuildRatingUpdates(meta)
	}

	for _, comicID := range comicIDs {
		updates := map[string]interface{}{}
		if meta.Author != "" && shouldApply("author") {
			updates["author"] = meta.Author
		}
		if meta.Publisher != "" && shouldApply("publisher") {
			updates["publisher"] = meta.Publisher
		}
		if meta.Language != "" && shouldApply("language") {
			updates["language"] = meta.Language
		}
		if meta.Genre != "" && shouldApply("genre") {
			updates["genre"] = meta.Genre
		}
		if meta.Description != "" && shouldApply("description") {
			updates["description"] = meta.Description
		}
		if meta.Year != nil && shouldApply("year") {
			updates["year"] = *meta.Year
		}
		for key, value := range ratingUpdates {
			updates[key] = value
		}

		if !overwrite {
			updates = filterEmptyFieldsOnly(comicID, updates)
		}
		if len(updates) == 0 {
			continue
		}
		if err := store.UpdateComicFields(comicID, updates); err != nil {
			log.Printf("[API] sync metadata to item failed for %s: %v", comicID, err)
			errorCount++
		} else {
			successCount++
		}
	}
	return successCount, errorCount, nil
}

func filterEmptyFieldsOnly(comicID string, updates map[string]interface{}) map[string]interface{} {
	if len(updates) == 0 {
		return updates
	}

	var author, publisher, language, genre, description string
	var year *int
	err := store.DB().QueryRow(`
		SELECT COALESCE("author",''), COALESCE("publisher",''), COALESCE("language",''),
		       COALESCE("genre",''), COALESCE("description",''), "year"
		FROM "Comic" WHERE "id" = ?
	`, comicID).Scan(&author, &publisher, &language, &genre, &description, &year)
	if err != nil {
		return updates
	}

	filtered := map[string]interface{}{}
	for key, value := range updates {
		switch key {
		case "author":
			if author == "" {
				filtered[key] = value
			}
		case "publisher":
			if publisher == "" {
				filtered[key] = value
			}
		case "language":
			if language == "" {
				filtered[key] = value
			}
		case "genre":
			if genre == "" {
				filtered[key] = value
			}
		case "description":
			if description == "" {
				filtered[key] = value
			}
		case "year":
			if year == nil {
				filtered[key] = value
			}
		default:
			filtered[key] = value
		}
	}
	return filtered
}
