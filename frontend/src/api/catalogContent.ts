import { apiClient } from "@/lib/apiClient";
import type { ApiComic, ComicsResponse } from "@/hooks/useComicTypes";

export async function fetchAllComics(options: {
  contentType?: "comic" | "novel";
  sortBy?: string;
  sortOrder?: string;
  libraryIds?: string[];
} = {}): Promise<ApiComic[]> {
  const pageSize = 500;
  let page = 1;
  const result: ApiComic[] = [];
  while (true) {
    const params = new URLSearchParams({
      page: String(page),
      pageSize: String(pageSize),
    });
    if (options.contentType) params.set("contentType", options.contentType);
    if (options.sortBy) params.set("sortBy", options.sortBy);
    if (options.sortOrder) params.set("sortOrder", options.sortOrder);
    if (options.libraryIds?.length) params.set("libraryIds", options.libraryIds.join(","));
    const response = await apiClient.get<ComicsResponse>(`/api/comics?${params}`);
    result.push(...(response.comics || []));
    if (page >= response.totalPages || !response.comics?.length) break;
    page += 1;
  }
  return result;
}
