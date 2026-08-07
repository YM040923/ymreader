import type { WorkUnit } from "@/types/work";

export type ContinuousReaderMode = "single" | "double" | "webtoon";

export interface ContinuousPage {
  url: string;
  unitId: string;
  comicId: string;
  unitTitle: string;
  relativePage: number;
  absolutePage: number;
}

export interface ReadingAnchor {
  unitId: string;
  comicId: string;
  relativePage: number;
  absolutePage: number;
}

export function buildContinuousPages(
  units: Array<Pick<WorkUnit, "id" | "comicId" | "title" | "startPage">>,
  pagesByUnit: Record<string, string[]>,
): ContinuousPage[] {
  return units.flatMap((unit) =>
    (pagesByUnit[unit.id] || []).map((url, relativePage) => ({
      url,
      unitId: unit.id,
      comicId: unit.comicId,
      unitTitle: unit.title,
      relativePage,
      absolutePage: unit.startPage + relativePage,
    })),
  );
}

export function anchorFromContinuousPage(
  pages: ContinuousPage[],
  pageIndex: number,
): ReadingAnchor | null {
  const page = locateContinuousPage(pages, pageIndex);
  if (!page) return null;
  return {
    unitId: page.unitId,
    comicId: page.comicId,
    relativePage: page.relativePage,
    absolutePage: page.absolutePage,
  };
}

export function anchorFromUnitPage(
  unit: Pick<WorkUnit, "id" | "comicId" | "startPage">,
  relativePage: number,
): ReadingAnchor {
  const safeRelativePage = Math.max(0, Math.trunc(relativePage));
  return {
    unitId: unit.id,
    comicId: unit.comicId,
    relativePage: safeRelativePage,
    absolutePage: unit.startPage + safeRelativePage,
  };
}

export function continuousIndexForAnchor(
  pages: ContinuousPage[],
  anchor: ReadingAnchor | null,
): number {
  if (!anchor || pages.length === 0) return -1;
  const exact = pages.findIndex(
    (page) => page.unitId === anchor.unitId && page.relativePage === anchor.relativePage,
  );
  if (exact >= 0) return exact;
  const sameUnit = pages.findIndex((page) => page.unitId === anchor.unitId);
  if (sameUnit >= 0) {
    const unitPages = pages.filter((page) => page.unitId === anchor.unitId);
    const relative = Math.max(0, Math.min(unitPages.length - 1, anchor.relativePage));
    return pages.findIndex(
      (page) => page.unitId === anchor.unitId && page.relativePage === relative,
    );
  }
  return Math.max(0, Math.min(pages.length - 1, anchor.absolutePage));
}

export function locateContinuousPage(
  pages: ContinuousPage[],
  pageIndex: number,
): ContinuousPage | null {
  if (pages.length === 0) return null;
  const clamped = Math.max(0, Math.min(pages.length - 1, Math.trunc(pageIndex)));
  return pages[clamped] || null;
}

export interface ContinuousUnitProgress {
  unitId: string;
  page: number;
  totalPages: number;
}

export function getContinuousUnitProgress(
  pages: ContinuousPage[],
  pageIndex: number,
): ContinuousUnitProgress | null {
  const current = locateContinuousPage(pages, pageIndex);
  if (!current) return null;
  const totalPages = pages.reduce(
    (count, page) => count + (page.unitId === current.unitId ? 1 : 0),
    0,
  );
  return {
    unitId: current.unitId,
    page: current.relativePage,
    totalPages,
  };
}

export function shouldAutoAdvanceChapter(
  continuousReading: boolean,
  mode: ContinuousReaderMode,
): boolean {
  return continuousReading && (
    mode === "single"
    || mode === "double"
    || mode === "webtoon"
  );
}

function spreadStart(
  currentPage: number,
  coverAlone: boolean,
): number {
  if (coverAlone) {
    if (currentPage <= 0) return 0;
    return currentPage % 2 === 1 ? currentPage : currentPage - 1;
  }
  return currentPage % 2 === 0 ? currentPage : currentPage - 1;
}

export function isAtForwardBoundary(
  mode: ContinuousReaderMode,
  currentPage: number,
  totalPages: number,
  coverAlone = false,
): boolean {
  if (totalPages <= 0) return true;
  if (mode !== "double") return currentPage >= totalPages - 1;
  const currentSpread = spreadStart(
    Math.max(0, Math.min(totalPages - 1, Math.trunc(currentPage))),
    coverAlone,
  );
  const step = coverAlone && currentSpread === 0 ? 1 : 2;
  return currentSpread + step >= totalPages;
}

export function isAtBackwardBoundary(
  mode: ContinuousReaderMode,
  currentPage: number,
  totalPages: number,
  coverAlone = false,
): boolean {
  if (totalPages <= 0) return true;
  if (mode !== "double") return currentPage <= 0;
  return spreadStart(
    Math.max(0, Math.min(totalPages - 1, Math.trunc(currentPage))),
    coverAlone,
  ) <= 0;
}
