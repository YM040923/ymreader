import assert from "node:assert/strict";
import fs from "node:fs";

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), "utf8");

const types = read("../src/lib/stores/scraper-types.ts");
const batchActions = read("../src/lib/stores/scraper-batch-actions.ts");
const scraperPage = read("../src/app/scraper/page.tsx");
const librariesApi = read("../src/api/libraries.ts");
const libraryPanel = read("../src/components/LibraryManagementPanel.tsx");

assert.match(types, /displayTitle\?:\s*string;/, "progress must carry a normalized Work title");
assert.match(types, /libraryId\?:\s*string;/, "library scrape progress must identify its library");

assert.match(batchActions, /function normalizeProgressItem\(/, "batch progress must be normalized");
assert.match(
  batchActions,
  /libraryItems\.find\(\(item\)\s*=>\s*item\.id\s*===\s*data\.comicId\)\?\.title/,
  "Work progress must resolve its title from the metadata library",
);
assert.match(
  batchActions,
  /export async function startLibraryScrape\(/,
  "library scraping must use the shared scraper task",
);
assert.match(
  batchActions,
  /fetchLibraryScrapeTargets\(libraryId\)/,
  "the shared library task must load Work targets",
);
assert.match(
  batchActions,
  /apiPath\("\/api\/metadata\/batch-selected"\)/,
  "the shared library task must use the unified batch-selected stream",
);

assert.match(
  scraperPage,
  /currentProgress\.displayTitle\s*\|\|/,
  "current progress must display the normalized Work title",
);
assert.match(
  scraperPage,
  /item\.displayTitle\s*\|\|/,
  "completed results must display the normalized Work title",
);

for (const field of ["workCount", "unitCount", "fileCount"]) {
  assert.match(librariesApi, new RegExp(`${field}\\??:\\s*number;`), `Library must expose ${field}`);
}
assert.match(
  librariesApi,
  /export async function fetchLibraryScrapeTargets\(/,
  "library API must expose Work targets for the unified task",
);

assert.doesNotMatch(
  libraryPanel,
  /scrapeLibrary/,
  "the panel must not wait for the independent synchronous scrape endpoint",
);
assert.match(libraryPanel, /startLibraryScrape/, "the panel must start the unified scraper task");
assert.match(
  libraryPanel,
  /const scrapeRunning = batchRunning && currentProgress\?\.libraryId === lib\.id/,
  "each card must derive scrape state from the shared task",
);

for (const label of ["作品数", "章节/卷数", "文件数", "新增文件"]) {
  assert.match(libraryPanel, new RegExp(label), `comic cards must show ${label}`);
}
for (const action of ["立即扫描", "编辑书库", "禁用书库", "启用书库", "删除书库"]) {
  assert.match(libraryPanel, new RegExp(action), `the ${action} menu action must remain visible`);
}

assert.match(
  libraryPanel,
  /menuOpen=\{openMenuId === lib\.id\}/,
  "an open action menu must escape card clipping",
);
assert.match(
  libraryPanel,
  /disabled=\{scanningId !== null \|\| batchRunning\}/,
  "scan and scrape controls must be mutually exclusive",
);
assert.ok(
  (libraryPanel.match(/\{lib\.type !== "novel" && \(/g) || []).length >= 2,
  "novel libraries must hide Work scraping in the card and menu",
);

console.log("Final Work metadata UI integration tests passed.");
