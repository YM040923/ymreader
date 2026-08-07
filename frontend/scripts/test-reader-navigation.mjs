import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const toolbar = readFileSync(
  new URL("../src/components/reader/ReaderToolbar.tsx", import.meta.url),
  "utf8",
);
const page = readFileSync(
  new URL("../src/app/reader/[id]/page.tsx", import.meta.url),
  "utf8",
);
const types = readFileSync(
  new URL("../src/types/reader.ts", import.meta.url),
  "utf8",
);

assert.match(toolbar, /上一话/);
assert.match(toolbar, /下一话/);
assert.match(toolbar, /目录/);
assert.match(toolbar, /连续阅读/);
assert.match(toolbar, /readerOptions\.rtl/);
assert.match(toolbar, /readerOptions\.ltr/);
assert.match(types, /continuousReading:\s*boolean/);
assert.doesNotMatch(page, /fixed bottom-20 left-1\/2 z-50 flex/);
assert.match(page, /scrollIntoView\(\{\s*block:\s*"center"/s);
assert.match(page, /shouldAutoAdvanceChapter\(readerOpts\.continuousReading, effectiveMode\)/);
assert.match(page, /isAtForwardBoundary\(/);
assert.match(page, /anchorFromContinuousPage\(/);
assert.match(page, /continuousIndexForAnchor\(/);
assert.doesNotMatch(page, /setContinuousActiveUnitId\(activeUnit\?\.id \|\| ""\)/);

console.log("reader navigation UI tests passed");
