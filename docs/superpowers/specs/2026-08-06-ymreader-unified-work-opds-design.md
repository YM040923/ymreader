# YMReader Unified Work and OPDS Design

## Goal

Make `Work` the only user-visible manga entity while retaining `Comic` as the
physical file/page resource. Remove the runtime dependency on `ComicSeries`,
make scanning optionally scrape newly discovered Works by default, and expose
standards-compatible OPDS feeds for chapter and continuous reading.

## Domain model

```text
Work (one visible manga)
  ├─ Unit (volume, chapter, extra, or full text)
  │    └─ Comic (physical ZIP/CBZ/PDF/folder resource)
  └─ Work metadata (title, cover, tags, categories, scrape state)
```

`Comic` rows remain because page rendering, downloads, checksums, file watching,
and reading sessions need stable physical resource IDs. Comic list endpoints
must not be used by shelves, dashboard, search, history, statistics, or OPDS
catalog listings.

## Persistent Work metadata

A new `LogicalWork` table stores stable Work IDs and user-visible metadata.
`LogicalWorkUnitOverride` stores optional manual unit order/section information.
The existing deterministic Work builder remains responsible for detecting file
layouts. After detection, persistent metadata is overlaid by Work ID.

Existing `ComicSeries` metadata is migrated into matching LogicalWork rows using
`libraryId + normalized root path`. Runtime Series generation and Series APIs
are removed after migration. `UserGroup` permission functionality is unrelated
and remains intact.

## OPDS

- All catalog, detail, cover, stream, and download resources support `HEAD`.
- `/api/opds/works` is the canonical Work navigation feed.
- `/api/opds/works/:id` is the Unit acquisition feed.
- Every Unit has a standard acquisition link.
- A physical archive/PDF Unit downloads the original file when it spans the
  whole resource.
- A Unit inside a large archive is exposed through a virtual CBZ endpoint that
  writes only that Unit's pages without creating a permanent duplicate.
- Each Work also exposes a continuous virtual publication that concatenates its
  Units into one page stream. PSE links remain available as an optimization.
- Legacy `/api/opds/series` routes and root entries are removed.

## Automatic scraping

Automatic metadata scraping defaults to enabled for new installations and new
libraries. A completed scan enqueues only new or materially changed Works that
lack reliable metadata or a user-selected cover. Manual/locked covers are never
overwritten. Scraping runs in a bounded background queue and does not block the
scanner.

## Cleanup

Remove YMReader format fixtures, sample manga, obsolete test libraries, stale
test containers/images, and superseded build directories. Cleanup is limited to
paths and Docker objects positively identified as YMReader test artifacts.
Formal service port `6680` and real manga roots are never modified.

## Verification

- Unit tests for Work grouping, Work metadata migration, scrape eligibility,
  OPDS GET/HEAD parity, virtual CBZ contents, and continuous page mapping.
- Handler tests proving Series routes are absent and Work routes remain.
- Frontend integration checks proving no runtime Series links/components.
- Docker deployment health check on port `17680`.

