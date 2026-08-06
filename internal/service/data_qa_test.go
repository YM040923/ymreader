package service

import (
	"os"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	tmp := t.TempDir() + "/test.db"
	os.Setenv("NOWEN_DB_PATH", tmp)
	if err := store.InitDB(tmp); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		store.CloseDB()
	})
}

func TestScanDataIssues_PageCountZero(t *testing.T) {
	setupTestDB(t)
	db := store.DB()

	_, err := db.Exec(`INSERT INTO "Comic" ("id","filename","title","pageCount","fileSize","addedAt","updatedAt")
		VALUES ('test1','f.cbz','Test Comic',0,100,datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatalf("insert comic: %v", err)
	}

	issues, err := ScanDataIssues()
	if err != nil {
		t.Fatalf("ScanDataIssues: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.IssueType == "PAGE_COUNT_ZERO" && iss.EntityID == "test1" {
			found = true
			if iss.Severity != "p2" {
				t.Errorf("expected severity p2, got %s", iss.Severity)
			}
			if !iss.AutoFixable {
				t.Error("expected AutoFixable=true")
			}
		}
	}
	if !found {
		t.Error("PAGE_COUNT_ZERO issue not found")
	}
}

func TestScanDataIssues_SessionOrphan(t *testing.T) {
	setupTestDB(t)
	db := store.DB()

	_, err := db.Exec(`INSERT INTO "Comic" ("id","filename","title","pageCount","fileSize","addedAt","updatedAt")
		VALUES ('comic2','f.cbz','Test',100,100,datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatalf("insert comic: %v", err)
	}

	twoHoursAgo := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO "ReadingSession" ("comicId","startedAt","duration","startPage","endPage")
		VALUES ('comic2',?,?,0,0)`, twoHoursAgo, 0)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	issues, err := ScanDataIssues()
	if err != nil {
		t.Fatalf("ScanDataIssues: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.IssueType == "SESSION_ORPHAN" {
			found = true
			if iss.Severity != "p2" {
				t.Errorf("expected severity p2, got %s", iss.Severity)
			}
		}
	}
	if !found {
		t.Error("SESSION_ORPHAN issue not found")
	}
}

func TestScanDataIssues_TotalTimeZero(t *testing.T) {
	setupTestDB(t)
	db := store.DB()

	_, err := db.Exec(`INSERT INTO "Comic" ("id","filename","title","pageCount","fileSize","totalReadTime","addedAt","updatedAt")
		VALUES ('comic3','f.cbz','Test',100,100,0,datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatalf("insert comic: %v", err)
	}

	oneHourAgo := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO "ReadingSession" ("comicId","startedAt","endedAt","duration","startPage","endPage")
		VALUES ('comic3',?,?,30,0,5)`, oneHourAgo, now)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	issues, err := ScanDataIssues()
	if err != nil {
		t.Fatalf("ScanDataIssues: %v", err)
	}

	found := false
	for _, iss := range issues {
		if iss.IssueType == "TOTAL_TIME_ZERO" && iss.EntityID == "comic3" {
			found = true
			if iss.Severity != "p2" {
				t.Errorf("expected severity p2, got %s", iss.Severity)
			}
		}
	}
	if !found {
		t.Error("TOTAL_TIME_ZERO issue not found")
	}
}

func TestBuildSummary(t *testing.T) {
	issues := []DataQAIssue{
		{Severity: "p2", IssueType: "PAGE_COUNT_ZERO", AutoFixable: true},
		{Severity: "p2", IssueType: "SESSION_ORPHAN", AutoFixable: true},
		{Severity: "p3", IssueType: "SESSION_ZERO_DURATION", AutoFixable: false},
	}
	s := BuildSummary(issues)
	if s.TotalIssues != 3 {
		t.Errorf("expected 3, got %d", s.TotalIssues)
	}
	if s.P2 != 2 {
		t.Errorf("expected p2=2, got %d", s.P2)
	}
	if s.P3 != 1 {
		t.Errorf("expected p3=1, got %d", s.P3)
	}
	if s.AutoFixable != 2 {
		t.Errorf("expected autoFixable=2, got %d", s.AutoFixable)
	}
	if s.ByType["PAGE_COUNT_ZERO"] != 1 {
		t.Errorf("expected PAGE_COUNT_ZERO=1, got %d", s.ByType["PAGE_COUNT_ZERO"])
	}
}
