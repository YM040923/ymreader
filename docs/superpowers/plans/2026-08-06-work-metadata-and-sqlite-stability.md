# Work Metadata and SQLite Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stabilize Work reads and provide Work-level manual metadata scraping, including a per-library “立即刮削” action.

**Architecture:** Keep SQLite WAL with concurrent reads, remove all writes from Work GET handlers, and serialize retryable Work persistence writes. Scans persist Work layouts but never start online scraping; manual library scraping operates once per Work.

**Tech Stack:** Go, Gin, SQLite/modernc, React, TypeScript, Vite, Docker.

---

### Task 1: Make Work reads pure

**Files:**
- Modify: `internal/handler/work_handler.go`
- Modify: `internal/service/work_persistence.go`
- Test: `internal/handler/work_handler_test.go`

- [ ] Add a failing handler regression test that makes the LogicalWork table read-only and loads Work list/detail/units.
- [ ] Split “persist and apply” into read-only metadata overlay and explicit persistence.
- [ ] Remove migration and persistence calls from `loadWorkCatalog`.
- [ ] Run focused handler/service tests.

### Task 2: Restore concurrent reads and serialize Work writes

**Files:**
- Modify: `internal/store/db.go`
- Modify: `internal/store/logical_work.go`
- Modify: `internal/store/db_test.go`
- Test: `internal/store/logical_work_test.go`

- [ ] Replace the single-connection regression test with a concurrent-read pool test.
- [ ] Add a failing concurrent Work-write/read regression test.
- [ ] Add a shared Work write mutex and retry helper around Work write transactions.
- [ ] Keep WAL and per-connection busy timeout.
- [ ] Run store tests.

### Task 3: Stop automatic online scraping

**Files:**
- Modify: `internal/service/scanner.go`
- Modify: `internal/service/work_scrape_queue.go`
- Test: `internal/service/work_scrape_queue_test.go`
- Test: scanner tests adjacent to `scanner.go`

- [ ] Add a failing test proving scan completion does not schedule automatic scraping.
- [ ] Remove automatic scrape scheduling from all scan paths.
- [ ] Keep explicit/manual Work scraping services available.
- [ ] Persist affected Work layouts after scans and invalidate Work cache.
- [ ] Run service tests.

### Task 4: Expose Work-level metadata library API

**Files:**
- Modify: `internal/handler/metadata_library_handler.go`
- Modify: `internal/handler/routes_metadata.go`
- Test: `internal/handler/metadata_library_handler_test.go`

- [ ] Add failing tests for comic Work pagination/filtering and novel Comic fallback.
- [ ] Return one metadata item per Work for comics.
- [ ] Preserve the current novel response model.
- [ ] Add stable entity type and Work fields required by the frontend.
- [ ] Run handler tests.

### Task 5: Add manual per-library Work scraping

**Files:**
- Modify: `internal/handler/library_handler.go`
- Modify: `internal/handler/routes_library.go`
- Create: `internal/service/manual_work_scrape.go`
- Test: `internal/handler/library_handler_test.go`
- Test: `internal/service/manual_work_scrape_test.go`

- [ ] Add failing tests proving one job per Work and library permission checks.
- [ ] Add `POST /api/admin/libraries/:id/scrape` and managed-user equivalent.
- [ ] Process missing-metadata Works sequentially with progress/result counters.
- [ ] Prevent duplicate concurrent runs for the same library.
- [ ] Run handler/service tests.

### Task 6: Update metadata and library management UI

**Files:**
- Modify: `frontend/src/lib/scraper-store.ts`
- Modify: `frontend/src/app/scraper/page.tsx`
- Modify: `frontend/src/api/libraries.ts`
- Modify: `frontend/src/components/LibraryManagementPanel.tsx`
- Test: frontend integration/source tests

- [ ] Add failing source/integration tests for Work-based comic metadata loading.
- [ ] Route comic actions to Work APIs and novel actions to Comic APIs.
- [ ] Add “立即刮削” with disabled/running/result states.
- [ ] Ensure cards and counts represent Works, not Units.
- [ ] Run frontend tests, typecheck, and build.

### Task 7: Error classification and deployment

**Files:**
- Modify: Work handler error helpers and frontend API client/error views.
- Modify: Docker build metadata only as required.

- [ ] Add failing tests for 404 versus `database_busy` 503.
- [ ] Implement finite retry for busy reads.
- [ ] Run focused Go tests, full frontend build, and Go build.
- [ ] Build a new image.
- [ ] Replace only `ymreader-unified-work-clean` on port `17680` using `--mount`.
- [ ] Verify login, repeated Work reads, known Work details/pages, manual scrape UI/API, and absence of automatic scraping.

