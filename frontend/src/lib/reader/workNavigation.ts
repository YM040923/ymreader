import type { ReaderNavigation } from "@/lib/work-model";
import type { Work, WorkUnit } from "@/types/work";

function unitEndExclusive(unit: WorkUnit): number {
  return unit.startPage + Math.max(1, unit.pageCount);
}

export function unitContainsAbsolutePage(
  unit: WorkUnit,
  absolutePage: number,
): boolean {
  return absolutePage >= unit.startPage && absolutePage < unitEndExclusive(unit);
}

export function findActiveWorkUnit(
  work: Pick<Work, "units">,
  navigation: ReaderNavigation,
  comicId: string,
): WorkUnit | null {
  const units = [...(work.units || [])].sort(
    (left, right) => left.sortIndex - right.sortIndex,
  );
  const sameComic = units.filter((unit) => unit.comicId === comicId);
  return (
    sameComic.find((unit) => unit.id === navigation.unitId)
    || (
      navigation.startPage !== null
        ? sameComic.find((unit) =>
          unitContainsAbsolutePage(unit, navigation.startPage!),
        )
        : undefined
    )
    || sameComic[0]
    || null
  );
}

export function clampAbsolutePageToUnit(
  unit: WorkUnit,
  absolutePage: number,
): number {
  const lastPage = unitEndExclusive(unit) - 1;
  return Math.max(unit.startPage, Math.min(lastPage, Math.trunc(absolutePage)));
}

export function relativePageForUnit(
  unit: WorkUnit,
  absolutePage: number,
): number {
  return clampAbsolutePageToUnit(unit, absolutePage) - unit.startPage;
}

export function absolutePageForUnit(
  unit: WorkUnit,
  relativePage: number,
): number {
  return clampAbsolutePageToUnit(
    unit,
    unit.startPage + Math.max(0, Math.trunc(relativePage)),
  );
}

export function slicePagesForUnit<T>(
  pages: T[],
  unit: WorkUnit | null,
): T[] {
  if (!unit) return pages;
  const start = Math.max(0, Math.min(pages.length, unit.startPage));
  const end = Math.max(
    start,
    Math.min(pages.length, unit.startPage + Math.max(0, unit.pageCount)),
  );
  return pages.slice(start, end);
}

export function buildUnitReaderPath(
  workId: string,
  unit: WorkUnit,
  absolutePage = unit.startPage,
): string {
  const params = new URLSearchParams();
  params.set("page", String(clampAbsolutePageToUnit(unit, absolutePage)));
  params.set("workId", workId);
  params.set("unitId", unit.id);
  return `/reader/${encodeURIComponent(unit.comicId)}?${params.toString()}`;
}
