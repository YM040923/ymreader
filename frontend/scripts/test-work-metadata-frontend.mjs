import assert from "node:assert/strict";
import fs from "node:fs";

const read = (relativePath) =>
  fs.readFileSync(new URL(relativePath, import.meta.url), "utf8");

const librariesApi = read("../src/api/libraries.ts");
const libraryPanel = read("../src/components/LibraryManagementPanel.tsx");
const scraperTypes = read("../src/lib/stores/scraper-types.ts");
const libraryActions = read("../src/lib/stores/library-actions.ts");

assert.match(
  scraperTypes,
  /entityType:\s*"work"\s*\|\s*"comic"/,
  "LibraryItem must identify whether its id belongs to a Work or Comic",
);

assert.match(
  librariesApi,
  /export interface LibraryScrapeResult\s*\{[\s\S]*total:\s*number;[\s\S]*success:\s*number;[\s\S]*failed:\s*number;[\s\S]*skipped:\s*number;/,
  "libraries API must expose the library scrape result shape",
);
assert.match(
  librariesApi,
  /apiPath\(`\/api\/admin\/libraries\/\$\{id\}\/scrape`\)/,
  "libraries API must call the per-library manual scrape endpoint",
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
  libraryPanel,
  /const \[scrapingId,\s*setScrapingId\] = useState<string \| null>\(null\)/,
  "library scrape must have state independent from scanning",
);
assert.match(
  libraryPanel,
  /await scrapeLibrary\(id\)/,
  "library management must invoke the manual scrape API",
);
assert.match(
  libraryPanel,
  /立即刮削/,
  "library management must render an immediate scrape action",
);
assert.match(
  libraryPanel,
  /刮削完成：成功 \$\{result\.success\}，失败 \$\{result\.failed\}，跳过 \$\{result\.skipped\}，共 \$\{result\.total\}/,
  "library management must report the scrape result counts",
);

console.log("Work metadata frontend integration tests passed.");
