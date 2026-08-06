// @ts-nocheck
import assert from "node:assert/strict";
import {
  buildWorkQuery,
  buildWorkDetailPath,
  buildWorkReaderPath,
  canManageWorks,
  collapseComicIdsToWorks,
  getWorkCoverAspectRatio,
  getWorkMetadataTarget,
  getWorkComicIds,
  canManageWorkLibrary,
  mergeNovelAndWorkItems,
  indexWorksByComicId,
  parseReaderNavigation,
  selectWorkReadingTarget,
  workToComic,
} from "./work-model.ts";

const work = {
  id: "work_demo",
  libraryId: "library_demo",
  title: "测试作品",
  rootPath: "测试作品.zip",
  coverComicId: "comic_archive",
  coverUrl: "/api/comics/comic_archive/thumbnail",
  itemCount: 2,
  pageCount: 20,
  fileSize: 1024,
  lastReadAt: "2026-08-05T10:00:00Z",
  units: [
    {
      id: "unit_1",
      workId: "work_demo",
      comicId: "comic_archive",
      title: "第001话",
      displayLabel: "第001话",
      internalPath: "第001话",
      startPage: 0,
      pageCount: 8,
      sortIndex: 0,
    },
    {
      id: "unit_2",
      workId: "work_demo",
      comicId: "comic_archive",
      title: "第002话",
      displayLabel: "第002话",
      internalPath: "第002话",
      startPage: 8,
      pageCount: 12,
      sortIndex: 1,
    },
  ],
};

assert.equal(buildWorkDetailPath(work.id), "/work/work_demo");
assert.equal(
  buildWorkReaderPath(work, work.units[1]),
  "/reader/comic_archive?page=8&workId=work_demo&unitId=unit_2",
);

assert.deepEqual(
  parseReaderNavigation("?workId=work_demo&unitId=unit_2&page=8"),
  { workId: "work_demo", unitId: "unit_2", startPage: 8 },
);

assert.deepEqual(
  getWorkComicIds({
    ...work,
    representativeComicId: "comic_archive",
    units: [
      ...work.units,
      { ...work.units[0], id: "unit_3", comicId: "comic_other" },
    ],
  }),
  ["comic_archive", "comic_other"],
);
assert.equal(
  canManageWorkLibrary(
    { role: "user" },
    "library_demo",
    [{ id: "library_demo", canManage: true }],
  ),
  true,
);
assert.equal(
  canManageWorkLibrary(
    { role: "user" },
    "library_demo",
    [{ id: "other", canManage: true }],
  ),
  false,
);
assert.deepEqual(
  mergeNovelAndWorkItems(
    [{ id: "novel-1", type: "novel", title: "Novel" }],
    [{ id: "work-1", title: "Work" }],
  ).map((item) => item.id),
  ["novel-1", "work-1"],
);

const target = selectWorkReadingTarget(work, {
  comic_archive: {
    lastReadPage: 11,
    lastReadAt: "2026-08-05T10:00:00Z",
    readingStatus: "reading",
  },
});

assert.equal(target?.unit.id, "unit_2");
assert.equal(target?.page, 11);
assert.equal(target?.isContinue, true);

const serverTarget = selectWorkReadingTarget(
  {
    ...work,
    continueComicId: "comic_archive",
    continuePage: 13,
    continueUnitId: "unit_2",
  },
  {},
);
assert.equal(serverTarget?.unit.id, "unit_2");
assert.equal(serverTarget?.page, 13);
assert.equal(serverTarget?.isContinue, true);
assert.equal(
  workToComic({
    ...work,
    continueComicId: "comic_archive",
    continuePage: 13,
    continueUnitId: "unit_2",
  }).readHref,
  "/reader/comic_archive?page=13&workId=work_demo&unitId=unit_2",
);
assert.equal(
  workToComic({
    ...work,
    units: [
      work.units[0],
      {
        ...work.units[1],
        comicId: "comic_second",
        startPage: 0,
      },
    ],
    continueComicId: "comic_second",
    continuePage: 5,
    continueUnitId: "unit_2",
  }).progress,
  70,
);

const unreadTarget = selectWorkReadingTarget(
  { ...work, lastReadAt: undefined },
  {},
);
assert.equal(unreadTarget?.unit.id, "unit_1");
assert.equal(unreadTarget?.page, 0);
assert.equal(unreadTarget?.isContinue, false);

assert.equal(
  buildWorkQuery({
    search: "  测试  ",
    tags: ["剧情", "恋爱"],
    favoritesOnly: true,
    sortBy: "addedAt",
    sortOrder: "desc",
    page: 3,
    pageSize: 48,
    category: "manga",
    readingStatus: "reading",
    metaFilter: "missing",
    uncategorized: true,
    untagged: true,
    libraryIds: ["lib-a", "lib-b"],
  }),
  "search=%E6%B5%8B%E8%AF%95&tags=%E5%89%A7%E6%83%85%2C%E6%81%8B%E7%88%B1&favorites=true&category=manga&readingStatus=reading&metaFilter=missing&uncategorized=true&untagged=true&libraryIds=lib-a%2Clib-b&sortBy=addedAt&sortOrder=desc&page=3&pageSize=48",
);

const secondWork = {
  ...work,
  id: "work_second",
  metadataHostType: "series",
  metadataHostId: "series-2",
  representativeComicId: "comic-2",
  coverComicId: "comic-2",
  coverAspectRatio: 0.72,
  units: [{ ...work.units[0], id: "unit-2", comicId: "comic-2" }],
};
const comicIndex = indexWorksByComicId([work, secondWork]);
assert.equal(comicIndex.get("comic_archive")?.id, "work_demo");
assert.equal(comicIndex.get("comic-2")?.id, "work_second");
assert.deepEqual(
  collapseComicIdsToWorks(["comic_archive", "comic_archive", "comic-2"], comicIndex)
    .map((item) => item.id),
  ["work_demo", "work_second"],
);

assert.equal(getWorkCoverAspectRatio(secondWork), "0.72");
assert.equal(getWorkCoverAspectRatio({ ...work, coverAspectRatio: 0 }), "5 / 7");
assert.equal(canManageWorks({ role: "user" }, [{ canManage: false }, { canManage: true }]), true);
assert.equal(canManageWorks({ role: "user" }, [{ canManage: false }]), false);
assert.equal(canManageWorks({ role: "admin" }, []), true);

assert.deepEqual(
  getWorkMetadataTarget({
    ...secondWork,
    metadataHostType: "series",
    metadataHostId: "series-2",
  }),
  { type: "series", id: "series-2" },
);
assert.deepEqual(
  getWorkMetadataTarget({
    ...work,
    metadataHostType: "comic",
    metadataHostId: "comic_archive",
  }),
  { type: "comic", id: "comic_archive" },
);

console.log("work-model tests passed");
