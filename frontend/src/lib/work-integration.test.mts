// @ts-nocheck
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const read = (relative) => fs.readFileSync(path.join(root, relative), "utf8");

const routes = read("main.tsx");
assert.doesNotMatch(routes, /path="\/collections"/);
assert.doesNotMatch(routes, /path="\/group/);
assert.match(routes, /path="\/work\/:id"/);
assert.match(routes, /path="\*"/);

for (const file of [
  "app/books/page.tsx",
  "app/comic/[id]/page.tsx",
  "app/reader/[id]/page.tsx",
  "app/scraper/page.tsx",
  "lib/scraper-store.ts",
]) {
  const source = read(file);
  assert.doesNotMatch(source, /fetchGroupedComicMap|AddToCollectionDialog|CollectionPanel|openCollectionPanel/);
  assert.doesNotMatch(source, /["'`]\/group\//);
}

for (const removed of [
  "app/group/[id]/page.tsx",
  "app/collections/page.tsx",
  "lib/stores/group-scraper-actions.ts",
  "lib/stores/collection-actions.ts",
  "components/scraper/CollectionPanel.tsx",
  "components/scraper/AddToCollectionDialog.tsx",
]) {
  assert.equal(fs.existsSync(path.join(root, removed)), false, `${removed} must be removed`);
}

const scraper = read("app/scraper/page.tsx");
assert.doesNotMatch(scraper, /viewMode\s*===\s*"group"|\/api\/groups|scraperGroup|groupBatch/);
assert.doesNotMatch(read("lib/scraper-store.ts"), /group-scraper-actions|collection-actions/);
assert.doesNotMatch(read("components/scraper/DetailPanel.tsx"), /applyToGroup|应用到系列/);
assert.doesNotMatch(read("components/ScanRulesPanel.tsx"), /applyToGroup|autoGroupByDir|自动合集|新建合集|ComicGroup/);

const books = read("app/books/page.tsx");
assert.match(books, /useWorks\(/);
assert.match(books, /favoritesOnly/);
assert.match(books, /readingStatusFilter/);
assert.match(books, /uncategorized/);
assert.match(books, /untagged/);
assert.match(books, /DuplicateDetector/);
assert.match(books, /BatchToolbar/);
assert.match(books, /aiSearchResults/);
assert.match(books, /sessionStorage/);
assert.match(books, /batchWorkOperation\("delete"/);
assert.match(books, /batchWorkOperation\("favorite"/);
assert.match(books, /batchWorkOperation\("addTags"/);
assert.match(books, /batchWorkOperation\("setCategory"/);
assert.match(books, /batchWorkOperation\("setReadingStatus"/);
assert.match(books, /reorderWorks\(orderedWorks\)/);
assert.doesNotMatch(books, /updateSortOrders|physicalIdsFor|\/api\/comics\/reorder/);
const worksApi = read("api/works.ts");
assert.match(worksApi, /action:\s*operation/);
assert.match(worksApi, /getWorkComicIds/);
assert.match(worksApi, /\/api\/comics\/batch/);
assert.match(worksApi, /\/api\/comics\/reorder/);
assert.doesNotMatch(worksApi, /\/api\/works\/batch|\/api\/works\/reorder/);

const detail = read("app/work/[id]/page.tsx");
assert.match(detail, /canManageWorkLibrary/);
assert.match(detail, /updateWorkMetadata/);
assert.match(detail, /setWorkTags/);
assert.match(detail, /setWorkCategories/);
assert.match(detail, /uploadWorkCover/);
assert.match(detail, /SimilarComics/);
const metadataApi = read("api/workMetadata.ts");
assert.match(metadataApi, /getWorkMetadataTarget/);
assert.match(metadataApi, /\/apply-metadata/);
assert.match(metadataApi, /work\.units\.map/);
assert.match(metadataApi, /\/api\/comics\/batch/);
assert.match(metadataApi, /categorySlugs/);

const history = read("app/history/page.tsx");
assert.match(history, /fetchAllComics\(\{ contentType: "novel"/);
assert.match(history, /fetchAllWorks/);

for (const statsFile of ["app/stats/page.tsx", "components/StatsPanel.tsx"]) {
  const source = read(statsFile);
  assert.match(source, /view=work&includeNovels=true/);
  assert.doesNotMatch(source, /recentSessions\s*=\s*.*flatMap/);
}

for (const recommendationFile of ["app/recommendations/page.tsx", "components/Recommendations.tsx"]) {
  const source = read(recommendationFile);
  assert.match(source, /fetchAllWorks/);
  assert.match(source, /isNovel/);
  assert.doesNotMatch(source, /pageSize:\s*10000/);
}

assert.match(read("components/Navbar.tsx"), /\{onUpload && \(/);
const continueReading = read("components/ContinueReading.tsx");
assert.match(continueReading, /fetchAllWorks/);
assert.match(continueReading, /comic\.readHref \|\| comic\.detailHref/);
assert.doesNotMatch(continueReading, /`\/reader\/\$\{comic\.id\}`/);

const reader = read("app/reader/[id]/page.tsx");
assert.match(reader, /fetchWork\(readerNavigation\.workId\)/);
assert.match(reader, /findActiveWorkUnit/);
assert.match(reader, /slicePagesForUnit/);
assert.match(reader, /absolutePageForUnit\(activeUnit, currentPage\)/);
assert.match(reader, /sessionKey:\s*activeUnit\?\.id \|\| comicId/);
assert.match(reader, /volume\.id === activeUnit\?\.id/);
assert.match(reader, /buildUnitReaderPath\(readerNavigation\.workId/);
assert.match(reader, /relativePageForUnit\(activeUnit, readerNavigation\.startPage\)/);
assert.match(reader, /router\.replace\(`\/work\/\$\{encodeURIComponent\(readerNavigation\.workId\)\}`\)/);
assert.doesNotMatch(reader, /findIndex\(v => v\.comicId === comicId\)/);
assert.doesNotMatch(reader, /router\.replace\(`\/reader\/\$\{nextVolume\.comicId\}`\)/);
const readingActivity = read("hooks/useReadingActivity.ts");
assert.match(readingActivity, /sessionKey:\s*string/);
assert.match(readingActivity, /session\?\.sessionKey === sessionKey/);

console.log("work integration tests passed");
