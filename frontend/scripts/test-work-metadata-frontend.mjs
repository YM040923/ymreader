import assert from "node:assert/strict";
import fs from "node:fs";

const read = (relativePath) =>
  fs.readFileSync(new URL(relativePath, import.meta.url), "utf8");

const librariesApi = read("../src/api/libraries.ts");
const libraryPanel = read("../src/components/LibraryManagementPanel.tsx");
const scraperTypes = read("../src/lib/stores/scraper-types.ts");
const libraryActions = read("../src/lib/stores/library-actions.ts");
const batchActions = read("../src/lib/stores/scraper-batch-actions.ts");

assert.match(
  scraperTypes,
  /entityType:\s*"work"\s*\|\s*"comic"/,
  "LibraryItem must identify whether its id belongs to a Work or Comic",
);

assert.doesNotMatch(
  librariesApi,
  /scrapeLibrary/,
  "the frontend must not keep a second synchronous library scrape client",
);

assert.match(
  libraryActions,
  /targets:\s*selectedItems\.map\(\(item\)\s*=>\s*\(\{\s*id:\s*item\.id,\s*entityType:\s*item\.entityType,\s*\}\)\)/s,
  "batch-selected requests must include id/entityType targets",
);
assert.match(
  libraryActions,
  /comicIds:\s*selectedItems\.map\(\(item\)\s*=>\s*item\.id\)/,
  "batch-selected requests must retain comicIds compatibility",
);

assert.match(
  batchActions,
  /export async function startLibraryScrape\(/,
  "library scraping must use the shared metadata task",
);
assert.match(
  batchActions,
  /apiPath\("\/api\/metadata\/batch-selected"\)/,
  "the shared metadata task must use batch-selected",
);
assert.match(
  libraryPanel,
  /startLibraryScrape/,
  "library management must invoke the shared metadata task",
);
assert.match(
  libraryPanel,
  /立即刮削/,
  "library management must render an immediate scrape action",
);
assert.match(
  libraryPanel,
  /scanningId !== null \|\| batchRunning/,
  "scanning and shared scraping must be mutually exclusive",
);
assert.match(
  libraryPanel,
  /删除书库/,
  "library management must keep the delete action",
);

console.log("Work metadata frontend integration tests passed.");
