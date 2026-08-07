package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
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

type ManualWorkScrapeTask struct {
	ID                string `json:"id"`
	LibraryID         string `json:"libraryId"`
	Status            string `json:"status"`
	Total             int    `json:"total"`
	Current           int    `json:"current"`
	Success           int    `json:"success"`
	Failed            int    `json:"failed"`
	Skipped           int    `json:"skipped"`
	CurrentEntityType string `json:"currentEntityType,omitempty"`
	CurrentEntityID   string `json:"currentEntityId,omitempty"`
	CurrentTitle      string `json:"currentTitle,omitempty"`
	Error             string `json:"error,omitempty"`
	StartedAt         string `json:"startedAt"`
	FinishedAt        string `json:"finishedAt,omitempty"`
}

type manualWorkScrapeTaskOptions struct {
	Workers int
	Search  func(context.Context, Work) ([]ComicMetadata, error)
	Apply   func(context.Context, workScrapeJob, ComicMetadata) error
}

type manualWorkScrapeTaskState struct {
	snapshot ManualWorkScrapeTask
	cancel   context.CancelFunc
}

type manualWorkScrapeTaskManager struct {
	mu        sync.RWMutex
	tasks     map[string]*manualWorkScrapeTaskState
	workers   chan struct{}
	search    func(context.Context, Work) ([]ComicMetadata, error)
	apply     func(context.Context, workScrapeJob, ComicMetadata) error
	closed    atomic.Bool
	closeOnce sync.Once
}

var manualWorkScrapeTaskSequence atomic.Uint64

func newManualWorkScrapeTaskManager(opts manualWorkScrapeTaskOptions) *manualWorkScrapeTaskManager {
	if opts.Workers < 1 {
		opts.Workers = 1
	}
	if opts.Search == nil {
		opts.Search = func(ctx context.Context, work Work) ([]ComicMetadata, error) {
			query := strings.TrimSpace(strings.Join([]string{work.Title, work.Author}, " "))
			results := SearchMetadata(query, nil, "zh", "comic")
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return results, nil
		}
	}
	if opts.Apply == nil {
		opts.Apply = func(ctx context.Context, job workScrapeJob, meta ComicMetadata) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return retryWorkScrapeSQLite(ctx, func() error {
				return applyAutomaticWorkMetadata(job, meta)
			})
		}
	}
	return &manualWorkScrapeTaskManager{
		tasks:   make(map[string]*manualWorkScrapeTaskState),
		workers: make(chan struct{}, opts.Workers),
		search:  opts.Search,
		apply:   opts.Apply,
	}
}

func (m *manualWorkScrapeTaskManager) Start(libraryID string, works []Work, force bool) (ManualWorkScrapeTask, error) {
	if m == nil || m.closed.Load() {
		return ManualWorkScrapeTask{}, fmt.Errorf("work scrape task manager is closed")
	}
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return ManualWorkScrapeTask{}, fmt.Errorf("library id is required")
	}
	if !tryBeginLibraryOperation(libraryID) {
		return ManualWorkScrapeTask{}, fmt.Errorf("library operation is already running")
	}
	now := time.Now().UTC()
	taskID := fmt.Sprintf("work-scrape-%d-%d", now.UnixNano(), manualWorkScrapeTaskSequence.Add(1))
	ctx, cancel := context.WithCancel(context.Background())
	task := ManualWorkScrapeTask{
		ID: taskID, LibraryID: libraryID, Status: "queued", Total: len(works),
		StartedAt: now.Format(time.RFC3339Nano),
	}
	m.mu.Lock()
	m.tasks[taskID] = &manualWorkScrapeTaskState{snapshot: task, cancel: cancel}
	m.mu.Unlock()
	go m.run(ctx, taskID, libraryID, append([]Work(nil), works...), force)
	return task, nil
}

func (m *manualWorkScrapeTaskManager) run(ctx context.Context, taskID, libraryID string, works []Work, force bool) {
	defer endLibraryOperation(libraryID)
	defer func() {
		if recovered := recover(); recovered != nil {
			m.finishTask(taskID, "failed", fmt.Sprintf("work scrape task panic: %v", recovered))
		}
	}()
	m.updateTask(taskID, func(task *ManualWorkScrapeTask) { task.Status = "running" })
	for _, work := range works {
		if ctx.Err() != nil {
			m.finishTask(taskID, "canceled", "")
			return
		}
		metadataPoor := strings.TrimSpace(work.MetadataSource) == "" &&
			strings.TrimSpace(work.Author) == "" &&
			strings.TrimSpace(work.Description) == "" &&
			strings.TrimSpace(work.Genre) == "" &&
			work.Year == nil
		coverMissing := strings.TrimSpace(work.StoredCoverURL) == "" &&
			strings.TrimSpace(work.CoverComicID) == "" &&
			!work.CoverLocked
		if !force && !metadataPoor && !coverMissing {
			m.updateTask(taskID, func(task *ManualWorkScrapeTask) {
				task.Current++
				task.Skipped++
			})
			continue
		}
		m.updateTask(taskID, func(task *ManualWorkScrapeTask) {
			task.CurrentEntityType = "work"
			task.CurrentEntityID = work.ID
			task.CurrentTitle = work.Title
		})
		select {
		case m.workers <- struct{}{}:
		case <-ctx.Done():
			m.finishTask(taskID, "canceled", "")
			return
		}
		results, searchErr := m.search(ctx, work)
		if searchErr == nil && len(results) == 0 {
			searchErr = fmt.Errorf("no metadata result for %q", work.Title)
		}
		var scrapeErr error
		if searchErr != nil {
			scrapeErr = searchErr
		} else if ctx.Err() != nil {
			scrapeErr = ctx.Err()
		} else {
			scrapeErr = m.apply(ctx, workScrapeJob{
				Work: work, SkipCover: work.CoverLocked || strings.TrimSpace(work.StoredCoverURL) != "",
			}, results[0])
		}
		<-m.workers
		if ctx.Err() != nil {
			m.finishTask(taskID, "canceled", "")
			return
		}
		m.updateTask(taskID, func(task *ManualWorkScrapeTask) {
			task.Current++
			if scrapeErr != nil {
				task.Failed++
				task.Error = scrapeErr.Error()
			} else {
				task.Success++
			}
		})
	}
	m.finishTask(taskID, "completed", "")
}

func (m *manualWorkScrapeTaskManager) Get(taskID string) (ManualWorkScrapeTask, bool) {
	if m == nil {
		return ManualWorkScrapeTask{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.tasks[taskID]
	if !ok {
		return ManualWorkScrapeTask{}, false
	}
	return state.snapshot, true
}

func (m *manualWorkScrapeTaskManager) Cancel(taskID string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	state, ok := m.tasks[taskID]
	if ok {
		state.cancel()
	}
	m.mu.RUnlock()
	return ok
}

func (m *manualWorkScrapeTaskManager) updateTask(taskID string, update func(*ManualWorkScrapeTask)) {
	m.mu.Lock()
	if state := m.tasks[taskID]; state != nil {
		update(&state.snapshot)
	}
	m.mu.Unlock()
}

func (m *manualWorkScrapeTaskManager) finishTask(taskID, status, message string) {
	m.updateTask(taskID, func(task *ManualWorkScrapeTask) {
		task.Status = status
		task.Error = message
		task.CurrentEntityType = ""
		task.CurrentEntityID = ""
		task.CurrentTitle = ""
		task.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	})
}

func (m *manualWorkScrapeTaskManager) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		m.mu.RLock()
		cancels := make([]context.CancelFunc, 0, len(m.tasks))
		for _, task := range m.tasks {
			cancels = append(cancels, task.cancel)
		}
		m.mu.RUnlock()
		for _, cancel := range cancels {
			cancel()
		}
	})
}

var defaultManualWorkScrapeTasks = newManualWorkScrapeTaskManager(manualWorkScrapeTaskOptions{
	Workers: defaultWorkScrapeWorkers,
})

func StartManualWorkScrapeTask(libraryID string, works []Work, force bool) (ManualWorkScrapeTask, error) {
	return defaultManualWorkScrapeTasks.Start(libraryID, works, force)
}

func GetManualWorkScrapeTask(taskID string) (ManualWorkScrapeTask, bool) {
	return defaultManualWorkScrapeTasks.Get(taskID)
}

func CancelManualWorkScrapeTask(taskID string) bool {
	return defaultManualWorkScrapeTasks.Cancel(taskID)
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
		coverMissing := strings.TrimSpace(work.StoredCoverURL) == "" &&
			strings.TrimSpace(work.CoverComicID) == "" &&
			!work.CoverLocked
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
	return ScrapeWorkMetadataContext(context.Background(), work, "", skipCover)
}

func ScrapeWorkMetadataContext(ctx context.Context, work Work, query string, skipCover bool) error {
	if strings.TrimSpace(work.ID) == "" {
		return fmt.Errorf("work id is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if work.MetadataHostType != "work" {
		work.MetadataHostType = "work"
		work.MetadataHostID = work.ID
	}
	if strings.TrimSpace(query) == "" {
		query = strings.TrimSpace(strings.Join([]string{work.Title, work.Author}, " "))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	results := SearchMetadata(query, nil, "zh", "comic")
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no metadata result for %q", query)
	}
	return ApplyWorkMetadataContext(ctx, work, skipCover, results[0])
}

func ApplyWorkMetadataContext(ctx context.Context, work Work, skipCover bool, meta ComicMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return retryWorkScrapeSQLite(ctx, func() error {
		return applyAutomaticWorkMetadata(workScrapeJob{Work: work, SkipCover: skipCover}, meta)
	})
}

func retryWorkScrapeSQLite(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = operation()
		if err == nil || !store.IsSQLiteBusyError(err) {
			return err
		}
		delay := time.Duration(25*(1<<attempt)) * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		}
	}
	return err
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
