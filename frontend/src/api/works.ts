import { apiClient } from "@/lib/apiClient";
import type { ApiComic } from "@/hooks/useComicTypes";
import { buildWorkQuery, getWorkComicIds } from "@/lib/work-model";
import type { Work, WorkListResponse, WorkQueryOptions } from "@/types/work";

export async function fetchWorks(options: WorkQueryOptions | string = {}): Promise<WorkListResponse> {
  const query = buildWorkQuery(typeof options === "string" ? { search: options } : options);
  const response = await apiClient.get<WorkListResponse>(
    `/api/works${query ? `?${query}` : ""}`,
  );
  return {
    ...response,
    works: (response.works || []).map((work) => ({
      ...work,
      units: work.units || [],
    })),
  };
}

export async function fetchWork(id: string): Promise<Work> {
  const work = await apiClient.get<Work>(
    `/api/works/${encodeURIComponent(id)}`,
  );
  return { ...work, units: work.units || [] };
}

export async function fetchWorkComics(work: Work): Promise<ApiComic[]> {
  const uniqueIDs = [...new Set(work.units.map((unit) => unit.comicId))];
  return Promise.all(
    uniqueIDs.map((comicId) =>
      apiClient.get<ApiComic>(`/api/comics/${encodeURIComponent(comicId)}`),
    ),
  );
}

export async function batchWorkOperation(
  operation: string,
  works: Work[],
  payload: Record<string, unknown> = {},
): Promise<void> {
  const comicIds = [...new Set(works.flatMap(getWorkComicIds))];
  if (comicIds.length === 0) return;
  await apiClient.post("/api/comics/batch", {
    action: operation,
    comicIds,
    ...payload,
  });
}

export async function reorderWorks(
  works: Work[],
): Promise<void> {
  const seen = new Set<string>();
  const orders: Array<{ id: string; sortOrder: number }> = [];
  works.forEach((work, workIndex) => {
    const ids = [
      ...[...work.units]
        .sort((left, right) => left.sortIndex - right.sortIndex)
        .map((unit) => unit.comicId),
      work.representativeComicId,
      work.coverComicId,
    ].filter((id): id is string => Boolean(id));
    ids.forEach((id, unitIndex) => {
      if (seen.has(id)) return;
      seen.add(id);
      orders.push({
        id,
        sortOrder: workIndex * 10_000 + unitIndex,
      });
    });
  });
  if (orders.length === 0) return;
  await apiClient.put("/api/comics/reorder", { orders });
}

export async function fetchAllWorks(
  options: Omit<WorkQueryOptions, "page" | "pageSize"> = {},
): Promise<Work[]> {
  const pageSize = 500;
  let page = 1;
  const result: Work[] = [];
  while (true) {
    const response = await fetchWorks({ ...options, page, pageSize });
    result.push(...response.works);
    if (page >= response.totalPages || response.works.length === 0) break;
    page += 1;
  }
  return result;
}
