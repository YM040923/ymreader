import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const types = readFileSync(
  new URL("../src/lib/stores/scraper-types.ts", import.meta.url),
  "utf8",
);
const page = readFileSync(
  new URL("../src/app/scraper/page.tsx", import.meta.url),
  "utf8",
);
const detailPanel = readFileSync(
  new URL("../src/components/scraper/DetailPanel.tsx", import.meta.url),
  "utf8",
);

assert.match(types, /entityId\?:\s*string/);
assert.match(page, /\/api\/placeholder\/\d+\/\d+/);
assert.match(page, /onError=/);
assert.match(
  detailPanel,
  /isWork\s*\?\s*`\/api\/works\/\$\{encodeURIComponent\(item\.id\)\}\/translate-metadata`/,
);
assert.match(detailPanel, /data-testid="metadata-edit-translate"/);

console.log("scraper cover tests passed");
