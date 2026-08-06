// @ts-nocheck
import assert from "node:assert/strict";
import {
  absolutePageForUnit,
  buildUnitReaderPath,
  clampAbsolutePageToUnit,
  findActiveWorkUnit,
  relativePageForUnit,
  slicePagesForUnit,
} from "./workNavigation.ts";

const work = {
  id: "work_archive",
  pageCount: 10,
  units: [
    {
      id: "unit_1",
      workId: "work_archive",
      comicId: "comic_archive",
      title: "第 1 话",
      displayLabel: "第 1 话",
      startPage: 0,
      pageCount: 4,
      sortIndex: 0,
    },
    {
      id: "unit_2",
      workId: "work_archive",
      comicId: "comic_archive",
      title: "第 2 话",
      displayLabel: "第 2 话",
      startPage: 4,
      pageCount: 6,
      sortIndex: 1,
    },
  ],
};

assert.equal(
  findActiveWorkUnit(work, {
    workId: work.id,
    unitId: "unit_2",
    startPage: 4,
  }, "comic_archive")?.id,
  "unit_2",
);
assert.equal(
  findActiveWorkUnit(work, {
    workId: work.id,
    unitId: "",
    startPage: 7,
  }, "comic_archive")?.id,
  "unit_2",
);

const physicalPages = Array.from({ length: 10 }, (_, index) => `page-${index}`);
assert.deepEqual(
  slicePagesForUnit(physicalPages, work.units[1]),
  ["page-4", "page-5", "page-6", "page-7", "page-8", "page-9"],
);
assert.equal(clampAbsolutePageToUnit(work.units[1], 2), 4);
assert.equal(clampAbsolutePageToUnit(work.units[1], 99), 9);
assert.equal(relativePageForUnit(work.units[1], 7), 3);
assert.equal(absolutePageForUnit(work.units[1], 3), 7);

assert.equal(
  buildUnitReaderPath(work.id, work.units[1], 7),
  "/reader/comic_archive?page=7&workId=work_archive&unitId=unit_2",
);

console.log("reader work navigation tests passed");
