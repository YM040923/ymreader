# YMReader Unified Work and OPDS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a Work-only user model, compatible OPDS chapter/continuous reading, default automatic scraping, and a clean NAS test deployment.

**Architecture:** Keep deterministic Work detection and physical Comic resources. Add persistent Work metadata, migrate ComicSeries data, and remove Series runtime APIs after migration. OPDS exposes Work navigation, Unit acquisitions, virtual CBZ resources, and continuous page streams.

**Tech Stack:** Go 1.23, Gin, SQLite, React 19, TypeScript, Vite, Docker.

---

### Task 1: OPDS HTTP and acquisition compatibility

**Files:**
- Modify: `internal/handler/routes_metadata.go`
- Modify: `internal/handler/opds_handler.go`
- Modify: `internal/service/opds.go`
- Test: `internal/handler/opds_handler_test.go`
- Test: `internal/handler/opds_work_handler_test.go`

- [ ] Add failing tests that issue `HEAD` to catalog/detail/cover/stream routes and expect the same status and content type as `GET`.
- [ ] Add failing tests requiring every Unit entry to contain a standard OPDS acquisition link.
- [ ] Add failing tests for a virtual Unit CBZ containing only the Unit page range.
- [ ] Register HEAD handlers and implement bodyless feed/media responses.
- [ ] Add virtual Unit download and continuous Work publication endpoints.
- [ ] Run `go test ./internal/handler ./internal/service -count=1`.
- [ ] Commit the OPDS changes.

### Task 2: Default and post-scan Work scraping

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/service/scanner.go`
- Modify: `internal/service/metadata_scraper.go`
- Modify: `internal/store/site_settings.go`
- Test: scanner/config/service tests adjacent to those files

- [ ] Add failing tests proving automatic scraping defaults to enabled.
- [ ] Add failing tests proving only new/changed metadata-poor Works are queued.
- [ ] Add failing tests proving manual covers are not overwritten.
- [ ] Implement a bounded Work scrape queue triggered after successful scans.
- [ ] Persist queue outcomes and leave scanning non-blocking.
- [ ] Run focused tests and commit.

### Task 3: Persistent Work metadata and Series migration

**Files:**
- Modify: `internal/store/db.go`
- Modify: `internal/store/migrate.go`
- Create: `internal/store/logical_work.go`
- Modify: `internal/service/work.go`
- Modify: `internal/handler/work_handler.go`
- Modify: `internal/handler/work_write_handler.go`
- Test: `internal/store/logical_work_test.go`
- Test: `internal/handler/work_write_handler_test.go`

- [ ] Add failing migration and metadata-overlay tests.
- [ ] Create LogicalWork persistence keyed by deterministic Work ID.
- [ ] Migrate matching ComicSeries metadata and cover state.
- [ ] Rewrite Work mutations to write LogicalWork directly.
- [ ] Verify existing Work IDs, reading progress, and physical Comic IDs remain stable.
- [ ] Run store/service/handler tests and commit.

### Task 4: Remove Series runtime

**Files:**
- Modify: route registration under `internal/handler`
- Delete or detach runtime Series handlers/stores after migration
- Modify: `frontend/src/main.tsx`
- Delete: `frontend/src/app/series/[id]/page.tsx`
- Remove: `frontend/src/components/SeriesMetadataSearch.tsx`
- Modify: Work metadata UI and scraper integrations
- Test: `frontend/src/lib/work-integration.test.mts`

- [ ] Add failing tests asserting Series routes and frontend links are absent.
- [ ] Remove Series runtime endpoints and OPDS aliases.
- [ ] Move remaining scraping UI to Work APIs.
- [ ] Keep UserGroup permission functionality.
- [ ] Run frontend integration/build and Go route tests.
- [ ] Commit.

### Task 5: Cleanup and deploy

**Targets:**
- NAS test root: `/vol3/1000/ymreader-unified-work-clean`
- Test service: `17680`

- [ ] Inventory fixture files, libraries, containers, images, and build directories.
- [ ] Delete only positively identified format fixtures and sample manga.
- [ ] Remove obsolete YMReader test containers/images while retaining the current rollback image until the new container is healthy.
- [ ] Run `go test ./... -count=1`, frontend integration tests, and `npm run build`.
- [ ] Build a new Docker image on the NAS.
- [ ] Replace only the `17680` test container and verify health/version.
- [ ] Confirm real manga mounts and library counts remain present.

