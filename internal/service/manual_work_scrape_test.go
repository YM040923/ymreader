package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestScrapeLibraryWorksIncludesManualWorks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scrape.db")
	if err := store.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := store.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	t.Cleanup(func() {
		store.CloseDB()
		_ = os.Remove(dbPath)
	})

	year := 2024
	if _, err := store.DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled")
		VALUES ('scrape-lib', 'scrape-lib', 'comic', '/scrape', 1);
		INSERT INTO "Comic" (
			"id", "filename", "title", "pageCount", "libraryId", "relativePath",
			"author", "description", "genre", "year", "coverImageUrl", "coverAspectRatio"
		) VALUES
			('scrape-c1', 'Work A/01.cbz', 'Work A', 3, 'scrape-lib', 'Work A/01.cbz',
			 'Author A', 'Description A', 'Action', ?, 'cover-a.jpg', 0.7),
			('scrape-c2', 'Work B/01.cbz', 'Work B', 3, 'scrape-lib', 'Work B/01.cbz',
			 'Author B', 'Description B', 'Action', ?, 'cover-b.jpg', 0.7);
		INSERT INTO "LogicalWork" (
			"id", "libraryId", "rootPath", "title", "author", "description",
			"genre", "year", "coverComicId", "coverUrl", "coverAspectRatio", "coverLocked"
		) VALUES (
			'work_manual_scrape', 'scrape-lib', 'Work B` + "\x1f" + `work_manual_scrape',
			'拆分作品', 'Author B', 'Description B', 'Action', ?, 'scrape-c2',
			'https://example.invalid/cover.jpg', 0.7, 1
		);
		INSERT INTO "WorkMembership" ("comicId", "workId", "libraryId")
		VALUES ('scrape-c2', 'work_manual_scrape', 'scrape-lib');
	`, year, year, year); err != nil {
		t.Fatalf("seed scrape library: %v", err)
	}

	comics, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"scrape-lib"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatalf("load scrape comics: %v", err)
	}
	for _, work := range BuildWorksFromComicList(comics.Comics, WorkBuildOptions{}) {
		if work.Title != "Work A" {
			continue
		}
		if _, err := store.DB().Exec(`
			INSERT INTO "LogicalWork" (
				"id", "libraryId", "rootPath", "title", "author", "description",
				"genre", "year", "coverComicId", "coverUrl", "coverAspectRatio", "coverLocked"
			) VALUES (?, 'scrape-lib', ?, 'Work A', 'Author A', 'Description A',
				'Action', ?, 'scrape-c1', 'https://example.invalid/a.jpg', 0.7, 1)
		`, work.ID, work.RootPath, year); err != nil {
			t.Fatalf("seed automatic work metadata: %v", err)
		}
	}

	result, err := ScrapeLibraryWorks("scrape-lib", false)
	if err != nil {
		t.Fatalf("ScrapeLibraryWorks failed: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("scrape total = %d, want automatic Work plus manual Work", result.Total)
	}
	if result.Success != 0 || result.Failed != 0 || result.Skipped != 2 {
		t.Fatalf("unexpected scrape result: %#v", result)
	}
}
