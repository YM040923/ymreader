package store

import "testing"

func TestGetFileStatsReturnsNonZeroAverages(t *testing.T) {
	setupTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO "Comic"
			("id", "filename", "relativePath", "title", "type", "fileSize", "pageCount")
		VALUES
			('stats-a', 'a.cbz', 'a.cbz', 'A', 'comic', 100, 10),
			('stats-b', 'b.cbz', 'b.cbz', 'B', 'comic', 300, 30)
	`); err != nil {
		t.Fatal(err)
	}

	stats, err := GetFileStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.AvgFileSize != 200 || stats.AvgPageCount != 20 {
		t.Fatalf(
			"averages = size:%d pages:%d, want size:200 pages:20",
			stats.AvgFileSize,
			stats.AvgPageCount,
		)
	}
}
