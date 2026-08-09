import type { WorkUnit } from "@/types/work";

export interface WorkReadingContext {
  workId: string;
  unitId: string;
  relativePage: number;
}

export function buildWorkReadingContext(
  workId: string | null | undefined,
  unit: WorkUnit | null | undefined,
  relativePage: number,
): WorkReadingContext | undefined {
  if (!workId || !unit || relativePage < 0 || relativePage >= unit.pageCount) {
    return undefined;
  }
  return {
    workId,
    unitId: unit.id,
    relativePage,
  };
}
