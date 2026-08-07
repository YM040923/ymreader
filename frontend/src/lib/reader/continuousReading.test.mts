// @ts-nocheck
import assert from "node:assert/strict";
import {
  anchorFromContinuousPage,
  anchorFromUnitPage,
  buildContinuousPages,
  continuousIndexForAnchor,
  getContinuousUnitProgress,
  isAtBackwardBoundary,
  isAtForwardBoundary,
  locateContinuousPage,
  shouldAutoAdvanceChapter,
} from "./continuousReading.ts";

const units = [
  { id: "u1", comicId: "c1", title: "第 1 话", startPage: 0, pageCount: 2, sortIndex: 0 },
  { id: "u2", comicId: "c2", title: "第 2 话", startPage: 0, pageCount: 2, sortIndex: 1 },
];

const pages = buildContinuousPages(units, {
  u1: ["c1-1", "c1-2"],
  u2: ["c2-1", "c2-2"],
});

assert.deepEqual(
  pages.map((page) => [page.url, page.unitId, page.relativePage]),
  [
    ["c1-1", "u1", 0],
    ["c1-2", "u1", 1],
    ["c2-1", "u2", 0],
    ["c2-2", "u2", 1],
  ],
);
assert.equal(locateContinuousPage(pages, 2)?.unitId, "u2");
assert.equal(locateContinuousPage(pages, 99)?.relativePage, 1);

assert.deepEqual(
  getContinuousUnitProgress(pages, 3),
  { unitId: "u2", page: 1, totalPages: 2 },
);
assert.equal(getContinuousUnitProgress([], 0), null);

assert.equal(shouldAutoAdvanceChapter(true, "single"), true);
assert.equal(shouldAutoAdvanceChapter(true, "double"), true);
assert.equal(shouldAutoAdvanceChapter(true, "webtoon"), true);
assert.equal(shouldAutoAdvanceChapter(false, "single"), false);

assert.equal(isAtForwardBoundary("single", 1, 2), true);
assert.equal(isAtForwardBoundary("double", 1, 4), false);
assert.equal(isAtForwardBoundary("double", 2, 4), true);
assert.equal(isAtForwardBoundary("double", 0, 4, true), false);
assert.equal(isAtForwardBoundary("double", 3, 4, true), true);
assert.equal(isAtBackwardBoundary("single", 0, 4), true);
assert.equal(isAtBackwardBoundary("double", 1, 4, true), false);
assert.equal(isAtBackwardBoundary("double", 1, 4), true);

const twentiethChapterPages = buildContinuousPages(
  [
    { id: "u19", comicId: "c19", title: "第 19 话", startPage: 0 },
    { id: "u20", comicId: "c20", title: "第 20 话", startPage: 0 },
  ],
  {
    u19: ["19-1", "19-2", "19-3"],
    u20: ["20-1", "20-2", "20-3", "20-4", "20-5", "20-6"],
  },
);

const chapterTwentyPageFive = anchorFromContinuousPage(
  twentiethChapterPages,
  7,
);
assert.deepEqual(
  chapterTwentyPageFive,
  { unitId: "u20", comicId: "c20", relativePage: 4, absolutePage: 4 },
  "closing continuous reading must preserve the actual chapter and relative page",
);
assert.equal(
  continuousIndexForAnchor(twentiethChapterPages, chapterTwentyPageFive),
  7,
  "re-enabling continuous reading must return to the same global page",
);
assert.deepEqual(
  anchorFromUnitPage(
    { id: "u20", comicId: "c20", startPage: 12 },
    4,
  ),
  { unitId: "u20", comicId: "c20", relativePage: 4, absolutePage: 16 },
  "single/double mode must expose a stable Work Unit anchor before switching modes",
);

console.log("continuous reading tests passed");
