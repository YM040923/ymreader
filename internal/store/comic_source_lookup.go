package store

import "strings"

// ComicExistsAtLibraryPath reports whether a library-relative source already
// has a database row, independently from the row's generated ID.
func ComicExistsAtLibraryPath(libraryID, relativePath string) (bool, error) {
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return false, nil
	}
	normalized, err := normalizeIgnoredRelativePath(relativePath)
	if err != nil {
		return false, err
	}

	var exists int
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM "Comic"
			WHERE "libraryId" = ?
			  AND COALESCE(NULLIF("relativePath", ''), "filename") = ?
		)
	`, libraryID, normalized).Scan(&exists)
	return exists != 0, err
}
