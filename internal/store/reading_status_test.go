package store

import "testing"

// ============================================================
// TestSetUserReadingStatus_BasicCRUD
// ============================================================
func TestSetUserReadingStatus_BasicCRUD(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	createTestUser(t, "user-a", "alice", "user")
	createTestUser(t, "user-b", "bob", "user")
	createTestComicWithLibrary(t, "comic-rs-1", "rs1.cbz", "RS Comic 1", "default")

	// User A sets reading status to "reading"
	if err := SetUserReadingStatus("user-a", "comic-rs-1", "reading"); err != nil {
		t.Fatalf("SetUserReadingStatus(user-a, reading) failed: %v", err)
	}

	// User A queries: should see "reading"
	comic, err := GetComicByIDForUser("comic-rs-1", "user-a")
	if err != nil {
		t.Fatalf("GetComicByIDForUser(user-a) failed: %v", err)
	}
	if comic == nil {
		t.Fatal("GetComicByIDForUser(user-a) returned nil")
	}
	if comic.ReadingStatus != "reading" {
		t.Errorf("user-a readingStatus = %q, want %q", comic.ReadingStatus, "reading")
	}

	// User B queries same comic: should see empty (no UserComicState for user-b)
	comicB, err := GetComicByIDForUser("comic-rs-1", "user-b")
	if err != nil {
		t.Fatalf("GetComicByIDForUser(user-b) failed: %v", err)
	}
	if comicB == nil {
		t.Fatal("GetComicByIDForUser(user-b) returned nil")
	}
	if comicB.ReadingStatus != "" {
		t.Errorf("user-b readingStatus = %q, want empty", comicB.ReadingStatus)
	}
}

// ============================================================
// TestSetUserReadingStatus_UpdateAndClear
// ============================================================
func TestSetUserReadingStatus_UpdateAndClear(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	createTestUser(t, "user-c", "charlie", "user")
	createTestComicWithLibrary(t, "comic-rs-2", "rs2.cbz", "RS Comic 2", "default")

	// Set to "want"
	SetUserReadingStatus("user-c", "comic-rs-2", "want")
	comic, _ := GetComicByIDForUser("comic-rs-2", "user-c")
	if comic.ReadingStatus != "want" {
		t.Errorf("after set want: got %q", comic.ReadingStatus)
	}

	// Update to "finished"
	SetUserReadingStatus("user-c", "comic-rs-2", "finished")
	comic, _ = GetComicByIDForUser("comic-rs-2", "user-c")
	if comic.ReadingStatus != "finished" {
		t.Errorf("after set finished: got %q", comic.ReadingStatus)
	}

	// Clear (empty string)
	SetUserReadingStatus("user-c", "comic-rs-2", "")
	comic, _ = GetComicByIDForUser("comic-rs-2", "user-c")
	if comic.ReadingStatus != "" {
		t.Errorf("after clear: got %q", comic.ReadingStatus)
	}
}

// ============================================================
// TestSetUserReadingStatus_IndependentUsers
// ============================================================
func TestSetUserReadingStatus_IndependentUsers(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	createTestUser(t, "user-d", "dave", "user")
	createTestUser(t, "user-e", "eve", "user")
	createTestComicWithLibrary(t, "comic-rs-3", "rs3.cbz", "RS Comic 3", "default")

	// User D: reading
	SetUserReadingStatus("user-d", "comic-rs-3", "reading")
	// User E: finished
	SetUserReadingStatus("user-e", "comic-rs-3", "finished")

	comicD, _ := GetComicByIDForUser("comic-rs-3", "user-d")
	if comicD.ReadingStatus != "reading" {
		t.Errorf("user-d: got %q, want reading", comicD.ReadingStatus)
	}

	comicE, _ := GetComicByIDForUser("comic-rs-3", "user-e")
	if comicE.ReadingStatus != "finished" {
		t.Errorf("user-e: got %q, want finished", comicE.ReadingStatus)
	}

	// User E clears their status - should not affect User D
	SetUserReadingStatus("user-e", "comic-rs-3", "")
	comicD2, _ := GetComicByIDForUser("comic-rs-3", "user-d")
	if comicD2.ReadingStatus != "reading" {
		t.Errorf("user-d after user-e clear: got %q, want reading", comicD2.ReadingStatus)
	}
}

// ============================================================
// TestSetUserReadingStatus_GlobalFieldNotPolluted
// ============================================================
func TestSetUserReadingStatus_GlobalFieldNotPolluted(t *testing.T) {
	setupTestDB(t)
	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	createTestUser(t, "user-f", "frank", "user")
	createTestComicWithLibrary(t, "comic-rs-4", "rs4.cbz", "RS Comic 4", "default")

	// Set Comic table global readingStatus to "want" (old data)
	_, err := db.Exec(`UPDATE "Comic" SET "readingStatus" = ? WHERE "id" = ?`, "want", "comic-rs-4")
	if err != nil {
		t.Fatalf("Set global readingStatus failed: %v", err)
	}

	// User F has no UserComicState yet - GetComicByIDForUser should return empty, not "want"
	comic, _ := GetComicByIDForUser("comic-rs-4", "user-f")
	if comic.ReadingStatus != "" {
		t.Errorf("user-f with no UserComicState: got %q, want empty (global field should not leak)", comic.ReadingStatus)
	}

	// User F sets "finished"
	SetUserReadingStatus("user-f", "comic-rs-4", "finished")
	comic, _ = GetComicByIDForUser("comic-rs-4", "user-f")
	if comic.ReadingStatus != "finished" {
		t.Errorf("user-f after set finished: got %q", comic.ReadingStatus)
	}

	// Global Comic.readingStatus should still be "want"
	var globalStatus string
	db.QueryRow(`SELECT "readingStatus" FROM "Comic" WHERE "id" = ?`, "comic-rs-4").Scan(&globalStatus)
	if globalStatus != "want" {
		t.Errorf("global Comic.readingStatus = %q, want %q (should not be modified)", globalStatus, "want")
	}
}

