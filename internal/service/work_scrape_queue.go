package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

const (
	defaultWorkScrapeQueueCapacity = 64
	defaultWorkScrapeWorkers       = 2
	defaultWorkScrapeInterval      = 750 * time.Millisecond
)

type workScrapeJob struct {
	Work      Work
	SkipCover bool
}

type workScrapeQueueOptions struct {
	Capacity int
	Workers  int
	Interval time.Duration
	Enabled  func() bool
	Scrape   func(workScrapeJob) error
	State    func(workScrapeJob, string, error)
	Record   func(workScrapeJob, error)
}

type workScrapeQueue struct {
	jobs      chan workScrapeJob
	enabled   func() bool
	scrape    func(workScrapeJob) error
	state     func(workScrapeJob, string, error)
	record    func(workScrapeJob, error)
	interval  time.Duration
	pendingMu sync.Mutex
	pending   map[string]struct{}
	closeOnce sync.Once
	workers   sync.WaitGroup
}

func newWorkScrapeQueue(opts workScrapeQueueOptions) *workScrapeQueue {
	if opts.Capacity < 1 {
		opts.Capacity = 1
	}
	if opts.Workers < 0 {
		opts.Workers = 0
	}
	if opts.Enabled == nil {
		opts.Enabled = func() bool { return true }
	}
	if opts.Scrape == nil {
		opts.Scrape = func(workScrapeJob) error { return nil }
	}
	if opts.State == nil {
		opts.State = func(workScrapeJob, string, error) {}
	}
	if opts.Record == nil {
		opts.Record = func(workScrapeJob, error) {}
	}
	queue := &workScrapeQueue{
		jobs:     make(chan workScrapeJob, opts.Capacity),
		enabled:  opts.Enabled,
		scrape:   opts.Scrape,
		state:    opts.State,
		record:   opts.Record,
		interval: opts.Interval,
		pending:  make(map[string]struct{}),
	}
	for index := 0; index < opts.Workers; index++ {
		queue.workers.Add(1)
		go queue.worker()
	}
	return queue
}

func (q *workScrapeQueue) Enqueue(job workScrapeJob) bool {
	if q == nil || strings.TrimSpace(job.Work.ID) == "" {
		return false
	}
	if !q.enabled() {
		q.state(job, "disabled", nil)
		return false
	}
	q.pendingMu.Lock()
	if _, exists := q.pending[job.Work.ID]; exists {
		q.pendingMu.Unlock()
		return false
	}
	q.pending[job.Work.ID] = struct{}{}
	q.pendingMu.Unlock()

	select {
	case q.jobs <- job:
		q.state(job, "queued", nil)
		return true
	default:
		q.finish(job.Work.ID)
		q.state(job, "deferred", errors.New("automatic work scrape queue is full"))
		log.Printf("[auto-work-scrape] queue full; skipped work %s", job.Work.ID)
		return false
	}
}

func (q *workScrapeQueue) worker() {
	defer q.workers.Done()
	var limiter <-chan time.Time
	var ticker *time.Ticker
	if q.interval > 0 {
		ticker = time.NewTicker(q.interval)
		defer ticker.Stop()
		limiter = ticker.C
	}
	for job := range q.jobs {
		if limiter != nil {
			<-limiter
		}
		if !q.enabled() {
			q.state(job, "disabled", nil)
			q.finish(job.Work.ID)
			continue
		}
		q.state(job, "running", nil)
		err := q.safeScrape(job)
		if err != nil {
			q.state(job, "failed", err)
		} else {
			q.state(job, "success", nil)
		}
		q.record(job, err)
		q.finish(job.Work.ID)
	}
}

func (q *workScrapeQueue) safeScrape(job workScrapeJob) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("automatic work scrape panic: %v", recovered)
		}
	}()
	return q.scrape(job)
}

func (q *workScrapeQueue) finish(workID string) {
	q.pendingMu.Lock()
	delete(q.pending, workID)
	q.pendingMu.Unlock()
}

func (q *workScrapeQueue) Close() {
	if q == nil {
		return
	}
	q.closeOnce.Do(func() {
		close(q.jobs)
		q.workers.Wait()
	})
}

var defaultAutomaticWorkScrapeQueue = newWorkScrapeQueue(workScrapeQueueOptions{
	Capacity: defaultWorkScrapeQueueCapacity,
	Workers:  defaultWorkScrapeWorkers,
	Interval: defaultWorkScrapeInterval,
	Enabled:  config.IsScraperEnabled,
	Scrape:   scrapeWorkAutomatically,
	State:    recordAutomaticWorkScrapeState,
	Record:   recordAutomaticWorkScrapeOutcome,
})

func ScheduleAutomaticWorkScrape(libraryIDs, changedComicIDs []string) {
	scheduleAutomaticWorkScrape(
		libraryIDs,
		changedComicIDs,
		discoverAutomaticWorkScrapeJobs,
		defaultAutomaticWorkScrapeQueue.Enqueue,
		config.IsScraperEnabled,
		func(err error) { log.Printf("[auto-work-scrape] discovery failed: %v", err) },
	)
}

func scheduleAutomaticWorkScrape(
	libraryIDs, changedComicIDs []string,
	discover func([]string, []string) ([]workScrapeJob, error),
	enqueue func(workScrapeJob) bool,
	enabled func() bool,
	reportError func(error),
) {
	if enabled == nil || !enabled() || len(changedComicIDs) == 0 {
		return
	}
	libraries := append([]string(nil), libraryIDs...)
	changed := append([]string(nil), changedComicIDs...)
	go func() {
		jobs, err := discover(libraries, changed)
		if err != nil {
			if reportError != nil {
				reportError(err)
			}
			return
		}
		for _, job := range jobs {
			enqueue(job)
		}
	}()
}

func discoverAutomaticWorkScrapeJobs(libraryIDs, changedComicIDs []string) ([]workScrapeJob, error) {
	libraryIDs = uniqueNonEmptyStrings(libraryIDs)
	if err := store.MigrateComicSeriesToLogicalWorks(); err != nil {
		return nil, err
	}
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType:      "comic",
		SortBy:           "title",
		SortOrder:        "asc",
		Page:             0,
		PageSize:         0,
		LibraryIDs:       libraryIDs,
		FilterLibraryIDs: len(libraryIDs) > 0,
	})
	if err != nil {
		return nil, err
	}
	works := BuildWorksFromComicList(result.Comics, WorkBuildOptions{})
	if err := PersistAndApplyLogicalWorks(works); err != nil {
		return nil, err
	}
	return selectAutomaticWorkScrapeJobs(works, changedComicIDs), nil
}

func selectAutomaticWorkScrapeJobs(works []Work, changedComicIDs []string) []workScrapeJob {
	changed := make(map[string]struct{}, len(changedComicIDs))
	for _, comicID := range changedComicIDs {
		if comicID = strings.TrimSpace(comicID); comicID != "" {
			changed[comicID] = struct{}{}
		}
	}
	jobs := make([]workScrapeJob, 0)
	for _, work := range works {
		if !workContainsChangedComic(work, changed) {
			continue
		}
		metadataPoor := strings.TrimSpace(work.MetadataSource) == "" ||
			(strings.TrimSpace(work.Author) == "" && strings.TrimSpace(work.Description) == "" && strings.TrimSpace(work.Genre) == "")
		coverMissing := strings.TrimSpace(work.StoredCoverURL) == "" && !work.CoverLocked
		if !metadataPoor && !coverMissing {
			continue
		}
		jobs = append(jobs, workScrapeJob{
			Work:      work,
			SkipCover: work.CoverLocked || strings.TrimSpace(work.StoredCoverURL) != "",
		})
	}
	return jobs
}

func workContainsChangedComic(work Work, changed map[string]struct{}) bool {
	for _, comicID := range PhysicalComicIDs(work) {
		if _, ok := changed[comicID]; ok {
			return true
		}
	}
	return false
}

func scrapeWorkAutomatically(job workScrapeJob) error {
	results := SearchMetadata(job.Work.Title, nil, "zh", "comic")
	if len(results) == 0 {
		return fmt.Errorf("no metadata result for %q", job.Work.Title)
	}
	return applyAutomaticWorkMetadata(job, results[0])
}

// ScrapeWorkMetadata performs one explicit Work-level metadata scrape. It is
// intentionally separate from the automatic queue so callers can keep
// automatic scraping disabled while still offering manual actions.
func ScrapeWorkMetadata(work Work, skipCover bool) error {
	if strings.TrimSpace(work.ID) == "" {
		return fmt.Errorf("work id is required")
	}
	if work.MetadataHostType != "work" {
		work.MetadataHostType = "work"
		work.MetadataHostID = work.ID
	}
	results := SearchMetadata(work.Title, nil, "zh", "comic")
	if len(results) == 0 {
		return fmt.Errorf("no metadata result for %q", work.Title)
	}
	return applyAutomaticWorkMetadata(workScrapeJob{Work: work, SkipCover: skipCover}, results[0])
}

func applyAutomaticWorkMetadata(job workScrapeJob, meta ComicMetadata) error {
	if job.Work.MetadataHostType == "work" {
		work, err := store.GetLogicalWork(job.Work.MetadataHostID)
		if err != nil {
			return err
		}
		if work == nil {
			return fmt.Errorf("logical Work metadata host not found: %s", job.Work.MetadataHostID)
		}
		update := store.LogicalWorkMetadataUpdate{}
		changed := false
		if work.Title == "" && meta.Title != "" {
			update.Title = &meta.Title
			changed = true
		}
		if work.Author == "" && meta.Author != "" {
			update.Author = &meta.Author
			changed = true
		}
		if work.Publisher == "" && meta.Publisher != "" {
			update.Publisher = &meta.Publisher
			changed = true
		}
		if work.Description == "" && meta.Description != "" {
			update.Description = &meta.Description
			changed = true
		}
		if work.Language == "" && meta.Language != "" {
			update.Language = &meta.Language
			changed = true
		}
		if work.Genre == "" && meta.Genre != "" {
			update.Genre = &meta.Genre
			changed = true
		}
		if work.Year == nil && meta.Year != nil {
			update.Year = meta.Year
			changed = true
		}
		if work.ExternalRating == nil && meta.ExternalRating != nil {
			update.ExternalRating = meta.ExternalRating
			update.ExternalRatingMax = meta.ExternalRatingMax
			update.ExternalRatingSource = &meta.ExternalRatingSource
			updatedAt := time.Now().UTC()
			update.ExternalRatingUpdatedAt = &updatedAt
			changed = true
		}
		if changed && work.MetadataSource != "manual" && meta.Source != "" {
			update.MetadataSource = &meta.Source
		}
		if changed {
			if err := store.UpdateLogicalWorkMetadata(work.ID, update); err != nil {
				return err
			}
		}
		if meta.Genre != "" {
			if err := mergeAutomaticLogicalWorkTags(work.ID, meta.Genre); err != nil {
				return err
			}
		}
		coverProtected := job.SkipCover || work.CoverLocked
		if !coverProtected && work.CoverURL == "" && meta.CoverURL != "" {
			coverURL := meta.CoverURL
			if err := store.UpdateLogicalWorkCover(work.ID, store.LogicalWorkCoverUpdate{CoverURL: &coverURL}); err != nil {
				return err
			}
			DownloadWorkCover(work.ID, coverURL)
		}
		InvalidateAllCaches()
		return nil
	}
	if job.Work.MetadataHostType == "series" {
		detail, err := store.GetSeriesDetail(job.Work.MetadataHostID, "")
		if err != nil {
			return err
		}
		if detail == nil {
			return fmt.Errorf("series metadata host not found: %s", job.Work.MetadataHostID)
		}
		update := store.SeriesMetadataUpdate{}
		changed := false
		if detail.Series.Author == "" && meta.Author != "" {
			update.Author = &meta.Author
			changed = true
		}
		if detail.Series.Publisher == "" && meta.Publisher != "" {
			update.Publisher = &meta.Publisher
			changed = true
		}
		if detail.Series.Description == "" && meta.Description != "" {
			update.Description = &meta.Description
			changed = true
		}
		if detail.Series.Language == "" && meta.Language != "" {
			update.Language = &meta.Language
			changed = true
		}
		if detail.Series.Genre == "" && meta.Genre != "" {
			update.Genre = &meta.Genre
			changed = true
		}
		if detail.Series.Year == nil && meta.Year != nil {
			update.Year = meta.Year
			changed = true
		}
		if detail.Series.ExternalRating == nil && meta.ExternalRating != nil {
			update.ExternalRating = meta.ExternalRating
			update.ExternalRatingMax = meta.ExternalRatingMax
			update.ExternalRatingSource = &meta.ExternalRatingSource
			updatedAt := time.Now().UTC()
			update.ExternalRatingUpdatedAt = &updatedAt
			changed = true
		}
		coverProtected := job.SkipCover ||
			(detail.Series.MetadataLocked && detail.Series.MetadataSource == "manual")
		coverURL := ""
		if !coverProtected && detail.Series.StoredCoverURL == "" && meta.CoverURL != "" {
			coverURL = meta.CoverURL
			changed = true
		}
		if changed && detail.Series.MetadataSource != "manual" && meta.Source != "" {
			update.MetadataSource = &meta.Source
		}
		if changed {
			if err := store.UpdateSeriesMetadata(detail.Series.ID, update); err != nil {
				return err
			}
		}
		if meta.Genre != "" {
			if err := mergeAutomaticSeriesTags(detail.Series.ID, meta.Genre); err != nil {
				return err
			}
		}
		if coverURL != "" {
			DownloadSeriesCover(detail.Series.ID, coverURL)
		}
		InvalidateAllCaches()
		return nil
	}
	skipCover := job.SkipCover
	if existing, lookupErr := store.GetComicByID(job.Work.MetadataHostID); lookupErr == nil && existing != nil {
		skipCover = skipCover || (existing.MetadataSource == "manual" && existing.CoverImageURL != "")
	}
	_, err := ApplyMetadata(
		job.Work.MetadataHostID,
		meta,
		"zh",
		false,
		ApplyOption{
			SkipCover:              skipCover,
			PreserveMetadataSource: job.Work.MetadataSource == "manual",
		},
	)
	if err == nil {
		InvalidateAllCaches()
	}
	return err
}

func mergeAutomaticLogicalWorkTags(workID, genre string) error {
	names := make([]string, 0)
	for _, name := range strings.Split(genre, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return store.AddLogicalWorkAndComicTags([]string{workID}, nil, names)
}

func mergeAutomaticSeriesTags(seriesID, genre string) error {
	existing, err := store.GetSeriesTags(seriesID)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(existing))
	seen := make(map[string]struct{}, len(existing))
	for _, tag := range existing {
		names = append(names, tag.Name)
		seen[tag.Name] = struct{}{}
	}
	for _, name := range strings.Split(genre, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return store.SetSeriesTags(seriesID, names)
}

func recordAutomaticWorkScrapeOutcome(job workScrapeJob, scrapeErr error) {
	status := "success"
	message := ""
	if scrapeErr != nil {
		status = "failed"
		message = scrapeErr.Error()
	}
	comicID := job.Work.RepresentativeComicID
	if comicID == "" {
		ids := PhysicalComicIDs(job.Work)
		if len(ids) > 0 {
			comicID = ids[0]
		}
	}
	if comicID == "" {
		return
	}
	_ = store.InsertSyncLog(comicID, "auto_work_scrape", "scanner", "", map[string]interface{}{
		"workId": job.Work.ID,
		"status": status,
		"error":  message,
	}, nil)
}

func recordAutomaticWorkScrapeState(job workScrapeJob, status string, stateErr error) {
	if job.Work.MetadataHostType != "work" || strings.TrimSpace(job.Work.MetadataHostID) == "" {
		return
	}
	message := ""
	if stateErr != nil {
		message = stateErr.Error()
	}
	var scrapedAt *time.Time
	if status == "success" {
		now := time.Now().UTC()
		scrapedAt = &now
	}
	if err := store.UpdateLogicalWorkScrapeState(job.Work.MetadataHostID, status, message, scrapedAt); err != nil {
		log.Printf("[auto-work-scrape] failed to persist Work %s state %s: %v", job.Work.MetadataHostID, status, err)
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
