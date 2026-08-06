package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestSelectAutomaticWorkScrapeJobsOnlyChangedMetadataPoorWorks(t *testing.T) {
	works := []Work{
		{
			ID: "needs-metadata", Title: "Needs Metadata", MetadataHostType: "series", MetadataHostID: "series-needs",
			Units: []WorkUnit{{ComicID: "chapter-1"}, {ComicID: "chapter-2"}},
		},
		{
			ID: "rich", Title: "Rich", MetadataHostType: "comic", MetadataHostID: "rich-comic",
			MetadataSource: "bangumi", Author: "Author", Description: "Description", Genre: "Drama",
			StoredCoverURL: "https://example.test/rich.jpg",
			Units:          []WorkUnit{{ComicID: "rich-comic"}},
		},
		{
			ID: "unchanged", Title: "Unchanged", MetadataHostType: "comic", MetadataHostID: "unchanged-comic",
			Units: []WorkUnit{{ComicID: "unchanged-comic"}},
		},
	}

	jobs := selectAutomaticWorkScrapeJobs(works, []string{"chapter-2", "rich-comic", "chapter-1"})
	if len(jobs) != 1 {
		t.Fatalf("selected %d jobs, want exactly one Work job: %#v", len(jobs), jobs)
	}
	if jobs[0].Work.ID != "needs-metadata" {
		t.Fatalf("selected work %q, want needs-metadata", jobs[0].Work.ID)
	}
}

func TestSelectAutomaticWorkScrapeJobsProtectsManualCover(t *testing.T) {
	work := Work{
		ID: "manual-cover", Title: "Manual Cover", MetadataHostType: "series", MetadataHostID: "series-manual",
		MetadataSource: "manual", CoverLocked: true,
		Units: []WorkUnit{{ComicID: "manual-chapter"}},
	}

	jobs := selectAutomaticWorkScrapeJobs([]Work{work}, []string{"manual-chapter"})
	if len(jobs) != 1 {
		t.Fatalf("manual-cover metadata-poor Work should still be queued, got %#v", jobs)
	}
	if !jobs[0].SkipCover {
		t.Fatal("manual/locked cover must be protected from automatic scraping")
	}
}

func TestWorkScrapeQueueIsBoundedAndDeduplicated(t *testing.T) {
	queue := newWorkScrapeQueue(workScrapeQueueOptions{
		Capacity: 2,
		Workers:  0,
		Enabled:  func() bool { return true },
	})
	t.Cleanup(queue.Close)

	if !queue.Enqueue(workScrapeJob{Work: Work{ID: "one"}}) {
		t.Fatal("first Work was not queued")
	}
	if !queue.Enqueue(workScrapeJob{Work: Work{ID: "two"}}) {
		t.Fatal("second Work was not queued")
	}
	if queue.Enqueue(workScrapeJob{Work: Work{ID: "one"}}) {
		t.Fatal("duplicate Work must not be queued twice")
	}
	if queue.Enqueue(workScrapeJob{Work: Work{ID: "three"}}) {
		t.Fatal("queue accepted a Work beyond its bounded capacity")
	}
}

func TestWorkScrapeQueueLimitsConcurrentWorkers(t *testing.T) {
	release := make(chan struct{})
	var active atomic.Int32
	var maxActive atomic.Int32
	var completed atomic.Int32

	queue := newWorkScrapeQueue(workScrapeQueueOptions{
		Capacity: 4,
		Workers:  2,
		Enabled:  func() bool { return true },
		Scrape: func(job workScrapeJob) error {
			current := active.Add(1)
			for {
				previous := maxActive.Load()
				if current <= previous || maxActive.CompareAndSwap(previous, current) {
					break
				}
			}
			<-release
			active.Add(-1)
			completed.Add(1)
			return nil
		},
	})
	t.Cleanup(queue.Close)

	for _, id := range []string{"one", "two", "three", "four"} {
		if !queue.Enqueue(workScrapeJob{Work: Work{ID: id}}) {
			t.Fatalf("failed to enqueue %s", id)
		}
	}

	deadline := time.Now().Add(time.Second)
	for active.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if active.Load() != 2 {
		t.Fatalf("active workers = %d, want 2", active.Load())
	}
	close(release)

	deadline = time.Now().Add(time.Second)
	for completed.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if completed.Load() != 4 {
		t.Fatalf("processed %d jobs, want 4", completed.Load())
	}
	if maxActive.Load() > 2 {
		t.Fatalf("max concurrent workers = %d, want at most 2", maxActive.Load())
	}
}

func TestWorkScrapeQueueSkipsWhenScraperExplicitlyDisabled(t *testing.T) {
	var called atomic.Int32
	queue := newWorkScrapeQueue(workScrapeQueueOptions{
		Capacity: 2,
		Workers:  1,
		Enabled:  func() bool { return false },
		Scrape: func(job workScrapeJob) error {
			called.Add(1)
			return nil
		},
	})
	t.Cleanup(queue.Close)

	if queue.Enqueue(workScrapeJob{Work: Work{ID: "disabled"}}) {
		t.Fatal("explicitly disabled scraper must reject automatic jobs")
	}
	time.Sleep(20 * time.Millisecond)
	if called.Load() != 0 {
		t.Fatalf("scraper called %d times while disabled", called.Load())
	}
}

func TestScheduleAutomaticWorkScrapeSkipsDiscoveryWhenDisabled(t *testing.T) {
	var discovered atomic.Int32
	scheduleAutomaticWorkScrape(
		[]string{"library"},
		[]string{"comic"},
		func(libraryIDs, changedComicIDs []string) ([]workScrapeJob, error) {
			discovered.Add(1)
			return nil, nil
		},
		func(workScrapeJob) bool { return true },
		func() bool { return false },
		func(error) {},
	)
	time.Sleep(20 * time.Millisecond)
	if discovered.Load() != 0 {
		t.Fatal("explicitly disabled scraper started post-scan discovery")
	}
}

func TestScheduleAutomaticWorkScrapeReturnsWithoutWaitingForDiscovery(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	scheduleAutomaticWorkScrape(
		[]string{"library"},
		[]string{"comic"},
		func(libraryIDs, changedComicIDs []string) ([]workScrapeJob, error) {
			once.Do(func() { close(started) })
			<-release
			return nil, nil
		},
		func(workScrapeJob) bool {
			t.Fatal("no job should be queued")
			return false
		},
		func() bool { return true },
		func(error) {},
	)
	close(done)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background discovery did not start")
	}
	select {
	case <-done:
	default:
		t.Fatal("scheduleAutomaticWorkScrape blocked the scan caller")
	}
	close(release)
}

func TestApplyAutomaticSeriesMetadataPreservesLockedCover(t *testing.T) {
	dbPath := t.TempDir() + "/auto-work.db"
	if err := store.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.CloseDB)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if _, err := store.DB().Exec(`
		INSERT INTO "Library" ("id", "name", "type", "rootPath", "enabled", "scanEnabled", "createdAt", "updatedAt")
		VALUES ('lib', 'Library', 'comic', '/tmp', 1, 1, ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "Comic" (
			"id", "filename", "relativePath", "title", "pageCount", "fileSize",
			"type", "libraryId", "addedAt", "updatedAt"
		) VALUES ('comic-1', 'Locked/001.cbz', 'Locked/001.cbz', '001', 10, 100, 'comic', 'lib', ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "ComicSeries" (
			"id", "libraryId", "rootRelativePath", "title", "sortTitle", "coverComicId",
			"coverUrl", "metadataSource", "metadataLocked", "createdAt", "updatedAt"
		) VALUES (
			'series-locked', 'lib', 'Locked', 'Locked', 'locked', 'comic-1',
			'', 'manual', 1, ?, ?
		)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO "ComicSeriesItem" ("seriesId", "comicId", "sortIndex", "displayLabel")
		VALUES ('series-locked', 'comic-1', 0, '001')
	`); err != nil {
		t.Fatal(err)
	}

	job := workScrapeJob{
		Work:      Work{ID: "work-locked", MetadataHostType: "series", MetadataHostID: "series-locked"},
		SkipCover: false,
	}
	meta := ComicMetadata{
		Author: "Scraped Author", Description: "Scraped Description",
		CoverURL: "https://scraped.example/wrong.jpg", Source: "bangumi",
	}
	if err := applyAutomaticWorkMetadata(job, meta); err != nil {
		t.Fatal(err)
	}

	var coverURL, author string
	if err := store.DB().QueryRow(`
		SELECT "coverUrl", "author" FROM "ComicSeries" WHERE "id" = 'series-locked'
	`).Scan(&coverURL, &author); err != nil {
		t.Fatal(err)
	}
	if coverURL != "" {
		t.Fatalf("locked cover overwritten with %q", coverURL)
	}
	if author != "Scraped Author" {
		t.Fatalf("missing metadata was not filled: author=%q", author)
	}
}
