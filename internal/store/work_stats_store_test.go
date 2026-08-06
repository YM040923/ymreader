package store

import (
	"testing"
	"time"
)

func TestGetReadingSessionRecordsFiltersUserAndLibrariesWithoutChangingPhysicalIdentity(t *testing.T) {
	setupTestDB(t)
	createTestUser(t, "stats-user", "stats-user", "user")
	createTestUser(t, "other-user", "other-user", "user")
	createTestComicWithLibrary(t, "comic-1", "chapter-1.cbz", "第一话", "comic-lib")
	createTestComicWithLibrary(t, "comic-2", "chapter-2.cbz", "第二话", "comic-lib")
	createTestComicWithLibrary(t, "secret", "secret.cbz", "不可见", "secret-lib")
	if _, err := db.Exec(`UPDATE "Comic" SET "type" = 'novel', "genre" = '奇幻' WHERE "id" = 'comic-2'`); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Date(2026, time.July, 1, 10, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		comicID string
		userID  string
	}{
		{"comic-1", "stats-user"},
		{"comic-2", "stats-user"},
		{"secret", "stats-user"},
		{"comic-1", "other-user"},
	} {
		if _, err := db.Exec(`
			INSERT INTO "ReadingSession"
				("comicId", "userId", "startedAt", "duration", "startPage", "endPage")
			VALUES (?, ?, ?, 60, 1, 3)
		`, row.comicID, row.userID, startedAt); err != nil {
			t.Fatal(err)
		}
	}

	records, err := GetReadingSessionRecords("stats-user", []string{"comic-lib"}, true)
	if err != nil {
		t.Fatalf("GetReadingSessionRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %#v, want only two accessible user sessions", records)
	}
	if records[0].ComicID != "comic-2" || records[0].ContentType != "novel" || records[0].LibraryID != "comic-lib" {
		t.Fatalf("first record = %#v", records[0])
	}
	if records[1].ComicID != "comic-1" || records[1].ContentType != "comic" {
		t.Fatalf("second record = %#v", records[1])
	}
}
