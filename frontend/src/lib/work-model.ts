import type {
  Work,
  WorkComicState,
  WorkReadingTarget,
  WorkQueryOptions,
  WorkUnit,
} from "@/types/work";
import type { Comic } from "@/types/comic";
import type { ApiComic } from "@/hooks/useComicTypes";

export function buildWorkQuery(options: WorkQueryOptions = {}): string {
  const params = new URLSearchParams();
  if (options.search?.trim()) params.set("search", options.search.trim());
  if (options.tags?.length) params.set("tags", options.tags.join(","));
  if (options.favoritesOnly) params.set("favorites", "true");
  if (options.category) params.set("category", options.category);
  if (options.readingStatus) params.set("readingStatus", options.readingStatus);
  if (options.metaFilter) params.set("metaFilter", options.metaFilter);
  if (options.uncategorized) params.set("uncategorized", "true");
  if (options.untagged) params.set("untagged", "true");
  if (options.libraryIds?.length) params.set("libraryIds", options.libraryIds.join(","));
  if (options.sortBy) params.set("sortBy", options.sortBy);
  if (options.sortOrder) params.set("sortOrder", options.sortOrder);
  if (options.page) params.set("page", String(options.page));
  if (options.pageSize) params.set("pageSize", String(options.pageSize));
  return params.toString();
}

export function indexWorksByComicId(works: Work[]): Map<string, Work> {
  const result = new Map<string, Work>();
  for (const work of works) {
    if (work.representativeComicId) result.set(work.representativeComicId, work);
    if (work.coverComicId) result.set(work.coverComicId, work);
    for (const unit of work.units || []) result.set(unit.comicId, work);
  }
  return result;
}

export function collapseComicIdsToWorks(
  comicIds: string[],
  index: Map<string, Work>,
): Work[] {
  const seen = new Set<string>();
  const result: Work[] = [];
  for (const comicId of comicIds) {
    const work = index.get(comicId);
    if (work && !seen.has(work.id)) {
      seen.add(work.id);
      result.push(work);
    }
  }
  return result;
}

export function getWorkCoverAspectRatio(work: Pick<Work, "coverAspectRatio">): string {
  return work.coverAspectRatio && work.coverAspectRatio > 0
    ? String(work.coverAspectRatio)
    : "5 / 7";
}

export function canManageWorks(
  user: { role?: string } | null | undefined,
  libraries: Array<{ canManage?: boolean }>,
): boolean {
  return user?.role === "admin" || libraries.some((library) => library.canManage);
}

export function canManageWorkLibrary(
  user: { role?: string } | null | undefined,
  libraryId: string,
  libraries: Array<{ id: string; canManage?: boolean }>,
): boolean {
  return user?.role === "admin"
    || libraries.some((library) => library.id === libraryId && library.canManage);
}

export function getWorkComicIds(
  work: Pick<Work, "units" | "representativeComicId" | "coverComicId">,
): string[] {
  return [...new Set([
    ...(work.units || []).map((unit) => unit.comicId),
    work.representativeComicId,
    work.coverComicId,
  ].filter((id): id is string => Boolean(id)))];
}

export function mergeNovelAndWorkItems<
  N extends { id: string },
  W extends { id: string },
>(novels: N[], works: W[]): Array<N | W> {
  return [...novels, ...works];
}

export function getWorkMetadataTarget(
  work: Pick<Work, "id">,
): { type: "work"; id: string } {
  return { type: "work", id: work.id };
}

export function workToComic(work: Work): Comic {
  const readingTarget = selectWorkReadingTarget(work, {});
  const target = readingTarget
    ? buildWorkReaderPath(work, readingTarget.unit, readingTarget.page)
    : buildWorkDetailPath(work.id);
  return {
    id: work.id,
    workId: work.id,
    title: work.title,
    titleSortKey: work.title,
    coverUrl: work.coverUrl || (
      work.coverComicId
        ? `/api/comics/${encodeURIComponent(work.coverComicId)}/thumbnail`
        : "/api/placeholder/320/448"
    ),
    coverAspectRatio: work.coverAspectRatio,
    tags: (work.tags || []).map((tag) => tag.name),
    tagData: (work.tags || []).map((tag) => ({ name: tag.name, color: tag.color || "" })),
    categories: (work.categories || []).map((category, index) => ({
      id: category.id || index,
      name: category.name,
      slug: category.slug || category.name,
      icon: category.icon || "",
    })),
    author: work.author,
    publisher: work.publisher,
    year: work.year || undefined,
    description: work.description,
    language: work.language,
    genre: work.genre,
    metadataSource: work.metadataSource,
    pageCount: work.pageCount,
    fileSize: work.fileSize,
    addedAt: work.addedAt,
    lastRead: work.lastReadAt || undefined,
    lastReadAt: work.lastReadAt,
    progress:
      work.continueUnitId && work.continuePage !== undefined
        ? calculateWorkProgress(work, work.continueUnitId, work.continuePage)
        : 0,
    isFavorite: work.isFavorite,
    rating: work.rating || undefined,
    lastReadPage: work.continuePage,
    sortOrder: work.sortOrder,
    filename: work.rootPath,
    readingStatus: work.readingStatus,
    type: "comic",
    externalRating: work.externalRating,
    externalRatingMax: work.externalRatingMax,
    externalRatingSource: work.externalRatingSource,
    metadataHostType: work.metadataHostType,
    metadataHostId: work.metadataHostId,
    representativeComicId: work.representativeComicId || work.coverComicId,
    detailHref: buildWorkDetailPath(work.id),
    readHref: target,
  };
}

export function calculateWorkProgress(
  work: Pick<Work, "units" | "pageCount">,
  unitId: string,
  absolutePage: number,
): number {
  const units = [...(work.units || [])].sort(
    (left, right) => left.sortIndex - right.sortIndex,
  );
  const index = units.findIndex((unit) => unit.id === unitId);
  if (index < 0 || work.pageCount <= 0) return 0;
  const unit = units[index];
  const completedBefore = units
    .slice(0, index)
    .reduce((sum, item) => sum + Math.max(0, item.pageCount), 0);
  const relativePage = Math.max(
    0,
    Math.min(
      Math.max(0, unit.pageCount - 1),
      Math.trunc(absolutePage) - unit.startPage,
    ),
  );
  return Math.min(
    100,
    Math.max(
      0,
      Math.round(
        ((completedBefore + relativePage + 1) / work.pageCount) * 100,
      ),
    ),
  );
}

export function apiComicToComic(api: ApiComic): Comic {
  return {
    id: api.id,
    title: api.title,
    titleSortKey: api.titleSortKey,
    coverUrl: api.coverUrl,
    coverAspectRatio: api.coverAspectRatio,
    tags: (api.tags || []).map((tag) => tag.name),
    tagData: api.tags || [],
    categories: api.categories || [],
    author: api.author,
    publisher: api.publisher,
    year: api.year || undefined,
    description: api.description,
    language: api.language,
    genre: api.genre,
    metadataSource: api.metadataSource,
    pageCount: api.pageCount,
    fileSize: api.fileSize,
    addedAt: api.addedAt,
    lastReadAt: api.lastReadAt,
    lastRead: api.lastReadAt || undefined,
    lastReadPage: api.lastReadPage,
    totalReadTime: api.totalReadTime,
    isFavorite: api.isFavorite,
    rating: api.rating || undefined,
    readingStatus: api.readingStatus,
    filename: api.filename,
    type: api.type,
    detailHref: api.type === "novel" ? `/novel/${api.id}` : `/comic/${api.id}`,
    readHref: api.type === "novel" ? `/novel/${api.id}` : `/reader/${api.id}`,
  };
}

export interface ReaderNavigation {
  workId: string;
  unitId: string;
  startPage: number | null;
}

export function buildWorkDetailPath(workId: string): string {
  return `/work/${encodeURIComponent(workId)}`;
}

export function buildWorkReaderPath(
  work: Pick<Work, "id">,
  unit: WorkUnit,
  page = unit.startPage,
): string {
  const params = new URLSearchParams();
  params.set("page", String(Math.max(0, Math.trunc(page))));
  params.set("workId", work.id);
  params.set("unitId", unit.id);
  return `/reader/${encodeURIComponent(unit.comicId)}?${params.toString()}`;
}

export function parseReaderNavigation(search: string): ReaderNavigation {
  const params = new URLSearchParams(search);
  const rawPage = params.get("page");
  const parsedPage = rawPage === null ? null : Number.parseInt(rawPage, 10);
  return {
    workId: params.get("workId")?.trim() || "",
    unitId: params.get("unitId")?.trim() || "",
    startPage:
      parsedPage !== null && Number.isFinite(parsedPage) && parsedPage >= 0
        ? parsedPage
        : null,
  };
}

function hasStarted(state?: WorkComicState): boolean {
  if (!state) return false;
  return Boolean(state.lastReadAt)
    || state.lastReadPage > 0
    || state.readingStatus === "reading"
    || state.readingStatus === "finished";
}

function timestamp(state?: WorkComicState): number {
  if (!state?.lastReadAt) return 0;
  const value = Date.parse(state.lastReadAt);
  return Number.isFinite(value) ? value : 0;
}

function unitContainsPage(unit: WorkUnit, page: number): boolean {
  const end = unit.pageCount > 0
    ? unit.startPage + unit.pageCount
    : unit.startPage + 1;
  return page >= unit.startPage && page < end;
}

export function selectWorkReadingTarget(
  work: Work,
  comicStates: Record<string, WorkComicState | undefined>,
): WorkReadingTarget | null {
  const units = [...(work.units || [])].sort(
    (left, right) => left.sortIndex - right.sortIndex,
  );
  if (units.length === 0) return null;

  if (work.continueComicId) {
    const continuePage = Math.max(0, work.continuePage || 0);
    const matchingUnits = units.filter(
      (unit) => unit.comicId === work.continueComicId,
    );
    const unit =
      matchingUnits.find((candidate) => candidate.id === work.continueUnitId)
      || matchingUnits.find((candidate) =>
        unitContainsPage(candidate, continuePage),
      )
      || matchingUnits[0];
    if (unit) {
      return { unit, page: continuePage, isContinue: true };
    }
  }

  const unitWithProgress = units
    .filter((unit) =>
      unit.lastReadPage !== undefined
      && (
        unit.lastReadPage > 0
        || Boolean(unit.lastReadAt)
        || unit.readingStatus === "reading"
        || unit.readingStatus === "finished"
      ),
    )
    .sort((left, right) =>
      Date.parse(right.lastReadAt || "") - Date.parse(left.lastReadAt || ""),
    )[0];
  if (unitWithProgress) {
    return {
      unit: unitWithProgress,
      page: Math.max(
        unitWithProgress.startPage,
        unitWithProgress.lastReadPage || unitWithProgress.startPage,
      ),
      isContinue: true,
    };
  }

  const startedComicIds = [...new Set(units.map((unit) => unit.comicId))]
    .filter((comicId) => hasStarted(comicStates[comicId]))
    .sort((left, right) => {
      const timeDifference =
        timestamp(comicStates[right]) - timestamp(comicStates[left]);
      if (timeDifference !== 0) return timeDifference;
      return (
        (comicStates[right]?.lastReadPage || 0)
        - (comicStates[left]?.lastReadPage || 0)
      );
    });

  if (startedComicIds.length === 0) {
    return { unit: units[0], page: units[0].startPage, isContinue: false };
  }

  const comicId = startedComicIds[0];
  const state = comicStates[comicId]!;
  const page = Math.max(0, state.lastReadPage || 0);
  const comicUnits = units.filter((unit) => unit.comicId === comicId);
  const unit =
    comicUnits.find((candidate) => unitContainsPage(candidate, page))
    || comicUnits[0]
    || units[0];
  const unitEnd = unit.pageCount > 0
    ? unit.startPage + unit.pageCount - 1
    : page;

  return {
    unit,
    page: Math.max(unit.startPage, Math.min(page, unitEnd)),
    isContinue: true,
  };
}
