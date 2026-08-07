package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func TestRetryWorkScrapeSQLiteRetriesBusyWrites(t *testing.T) {
	var attempts atomic.Int32
	err := retryWorkScrapeSQLite(context.Background(), func() error {
		if attempts.Add(1) < 3 {
			return errors.New("SQLITE_BUSY: database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts=%d, want 3", attempts.Load())
	}
}

func TestManualWorkScrapeTaskIsObservableAndCancelableBeforeWrite(t *testing.T) {
	started := make(chan struct{})
	releaseSearch := make(chan struct{})
	var searched atomic.Int32
	var applied atomic.Int32

	manager := newManualWorkScrapeTaskManager(manualWorkScrapeTaskOptions{
		Workers: 1,
		Search: func(ctx context.Context, work Work) ([]ComicMetadata, error) {
			searched.Add(1)
			close(started)
			<-releaseSearch
			return []ComicMetadata{{Title: "Scraped", Source: "test"}}, nil
		},
		Apply: func(ctx context.Context, job workScrapeJob, meta ComicMetadata) error {
			applied.Add(1)
			return nil
		},
	})
	t.Cleanup(manager.Close)

	task, err := manager.Start("library-1", []Work{
		{ID: "work-1", LibraryID: "library-1", Title: "Work One"},
		{ID: "work-2", LibraryID: "library-1", Title: "Work Two"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("manual scrape task did not start")
	}
	running, ok := manager.Get(task.ID)
	if !ok || running.Status != "running" || running.CurrentEntityType != "work" || running.CurrentEntityID != "work-1" {
		t.Fatalf("task is not observable while running: %#v ok=%v", running, ok)
	}
	if !manager.Cancel(task.ID) {
		t.Fatal("running task was not canceled")
	}
	close(releaseSearch)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, _ := manager.Get(task.ID)
		if snapshot.Status == "canceled" {
			if searched.Load() != 1 {
				t.Fatalf("cancel started %d searches, want only the in-flight search", searched.Load())
			}
			if applied.Load() != 0 {
				t.Fatalf("cancel allowed %d metadata writes", applied.Load())
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("task did not reach canceled state")
}

func TestManualWorkScrapeTaskUsesLibraryOperationMutex(t *testing.T) {
	if !tryBeginLibraryOperation("busy-library") {
		t.Fatal("failed to reserve library operation for scan simulation")
	}
	defer endLibraryOperation("busy-library")

	manager := newManualWorkScrapeTaskManager(manualWorkScrapeTaskOptions{Workers: 1})
	t.Cleanup(manager.Close)
	if _, err := manager.Start("busy-library", []Work{{ID: "work-1", Title: "Work"}}, true); err == nil {
		t.Fatal("scrape task started while the same library was reserved by a scan")
	}
}

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

func TestApplyAutomaticLogicalWorkMetadataPreservesLockedCover(t *testing.T) {
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
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		ID: workmodel.StableWorkID("lib", "Locked"), LibraryID: "lib", RootPath: "Locked",
		Title: "Locked", CoverComicID: "comic-1",
	}}); err != nil {
		t.Fatal(err)
	}
	manualCover := "https://manual.example/locked.jpg"
	locked := true
	if err := store.UpdateLogicalWorkCover(
		workmodel.StableWorkID("lib", "Locked"),
		store.LogicalWorkCoverUpdate{CoverURL: &manualCover, CoverLocked: &locked},
	); err != nil {
		t.Fatal(err)
	}

	job := workScrapeJob{
		Work: Work{
			ID: workmodel.StableWorkID("lib", "Locked"), LibraryID: "lib", RootPath: "Locked",
			MetadataHostType: "work", MetadataHostID: workmodel.StableWorkID("lib", "Locked"),
			Units: []WorkUnit{{ComicID: "comic-1"}},
		},
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
		SELECT "coverUrl", "author" FROM "LogicalWork" WHERE "id" = ?
	`, job.Work.ID).Scan(&coverURL, &author); err != nil {
		t.Fatal(err)
	}
	if coverURL != manualCover {
		t.Fatalf("locked cover overwritten with %q", coverURL)
	}
	if author != "Scraped Author" {
		t.Fatalf("missing metadata was not filled: author=%q", author)
	}
}
