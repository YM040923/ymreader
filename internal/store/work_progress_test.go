package store

import (
	"database/sql"
	"testing"
)

func setupWorkProgressTest(t *testing.T) {
	t.Helper()
	setupTestDB(t)

	if _, err := DB().Exec(`
		INSERT INTO "User" ("id", "username", "password") VALUES
			('progress-user', 'progress-user', 'password');
		INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES
			('progress-library', 'Progress', 'comic', '/progress');
		INSERT INTO "Comic" (
			"id", "filename", "title", "pageCount", "libraryId", "relativePath"
		) VALUES
			('progress-comic', '作品.zip', '作品', 200, 'progress-library', '作品.zip'),
			('other-comic', '其他.zip', '其他', 50, 'progress-library', '其他.zip');
		INSERT INTO "LogicalWork" (
			"id", "libraryId", "rootPath", "title", "sortTitle"
		) VALUES
			('work-progress', 'progress-library', '作品.zip', '作品', '作品'),
			('work-other', 'progress-library', '其他.zip', '其他', '其他');
	`); err != nil {
		t.Fatalf("seed work progress data: %v", err)
	}
}

func upsertWorkProgressForTest(t *testing.T, update WorkProgressUpdate) (UserWorkProgress, bool, error) {
	t.Helper()
	tx, err := DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	progress, applied, err := UpsertUserWorkProgressTx(tx, update)
	if err != nil {
		return UserWorkProgress{}, false, err
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return progress, applied, nil
}

func TestWorkProgressPersistsCanonicalCursor(t *testing.T) {
	setupWorkProgressTest(t)

	progress, applied, err := upsertWorkProgressForTest(t, WorkProgressUpdate{
		UserID:          "progress-user",
		WorkID:          "work-progress",
		UnitID:          "unit-21",
		ComicID:         "progress-comic",
		RelativePage:    2,
		UnitStartPage:   120,
		UnitPageCount:   10,
		ClientSessionID: "session-a",
		Sequence:        7,
	})
	if err != nil {
		t.Fatalf("UpsertUserWorkProgressTx failed: %v", err)
	}
	if !applied {
		t.Fatal("first cursor update was not applied")
	}
	if progress.AbsolutePage != 122 || progress.RelativePage != 2 {
		t.Fatalf("canonical cursor = relative:%d absolute:%d, want 2/122", progress.RelativePage, progress.AbsolutePage)
	}

	stored, err := GetUserWorkProgress("progress-user", "work-progress")
	if err != nil {
		t.Fatalf("GetUserWorkProgress failed: %v", err)
	}
	if stored == nil || stored.UnitID != "unit-21" || stored.ComicID != "progress-comic" ||
		stored.RelativePage != 2 || stored.AbsolutePage != 122 ||
		stored.ClientSessionID != "session-a" || stored.LastSequence != 7 {
		t.Fatalf("stored cursor = %#v", stored)
	}
}

func TestWorkProgressAllowsCurrentSequenceRollbackButRejectsOlderSequence(t *testing.T) {
	setupWorkProgressTest(t)

	first := WorkProgressUpdate{
		UserID:          "progress-user",
		WorkID:          "work-progress",
		UnitID:          "unit-21",
		ComicID:         "progress-comic",
		RelativePage:    8,
		UnitStartPage:   120,
		UnitPageCount:   10,
		ClientSessionID: "session-a",
		Sequence:        10,
	}
	if _, applied, err := upsertWorkProgressForTest(t, first); err != nil || !applied {
		t.Fatalf("initial update = applied:%v err:%v", applied, err)
	}

	rollback := first
	rollback.UnitID = "unit-2"
	rollback.RelativePage = 1
	rollback.UnitStartPage = 10
	rollback.Sequence = 11
	if progress, applied, err := upsertWorkProgressForTest(t, rollback); err != nil || !applied {
		t.Fatalf("rollback update = %#v applied:%v err:%v", progress, applied, err)
	}

	late := first
	late.RelativePage = 9
	late.Sequence = 9
	progress, applied, err := upsertWorkProgressForTest(t, late)
	if err != nil {
		t.Fatalf("late update failed: %v", err)
	}
	if applied {
		t.Fatal("older sequence unexpectedly overwrote the current cursor")
	}
	if progress.UnitID != "unit-2" || progress.RelativePage != 1 || progress.AbsolutePage != 11 ||
		progress.LastSequence != 11 {
		t.Fatalf("cursor after late update = %#v", progress)
	}
}

func TestWorkProgressRejectsInvalidRelationshipsAndPagesWithoutWriting(t *testing.T) {
	setupWorkProgressTest(t)

	tests := []struct {
		name   string
		update WorkProgressUpdate
	}{
		{
			name: "comic outside work root",
			update: WorkProgressUpdate{
				UserID: "progress-user", WorkID: "work-progress", UnitID: "unit-x",
				ComicID: "other-comic", RelativePage: 0, UnitStartPage: 0, UnitPageCount: 10,
				ClientSessionID: "invalid-a", Sequence: 1,
			},
		},
		{
			name: "relative page overflow",
			update: WorkProgressUpdate{
				UserID: "progress-user", WorkID: "work-progress", UnitID: "unit-x",
				ComicID: "progress-comic", RelativePage: 10, UnitStartPage: 120, UnitPageCount: 10,
				ClientSessionID: "invalid-b", Sequence: 1,
			},
		},
		{
			name: "absolute page overflow",
			update: WorkProgressUpdate{
				UserID: "progress-user", WorkID: "work-progress", UnitID: "unit-x",
				ComicID: "progress-comic", RelativePage: 5, UnitStartPage: 198, UnitPageCount: 10,
				ClientSessionID: "invalid-c", Sequence: 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := upsertWorkProgressForTest(t, test.update); err == nil {
				t.Fatal("invalid cursor update unexpectedly succeeded")
			}
		})
	}

	var count int
	if err := DB().QueryRow(`SELECT COUNT(*) FROM "UserWorkProgress"`).Scan(&count); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid updates wrote %d cursor rows", count)
	}
}
