import { apiClient } from "@/lib/apiClient";
import { getWorkMetadataTarget } from "@/lib/work-model";
import type { Work } from "@/types/work";

export interface WorkMetadataUpdate {
  title?: string;
  author?: string;
  publisher?: string;
  year?: number | null;
  description?: string;
  language?: string;
  genre?: string;
  status?: string;
}

function targetPath(work: Work): string {
  const target = getWorkMetadataTarget(work);
  return `/api/works/${encodeURIComponent(target.id)}`;
}

export async function updateWorkMetadata(work: Work, metadata: WorkMetadataUpdate) {
  await apiClient.put(`${targetPath(work)}/metadata`, metadata);
}

export async function toggleWorkFavorite(work: Work): Promise<boolean | null> {
  if (!work.id) return null;
  const next = !work.isFavorite;
  await apiClient.put(`/api/works/${encodeURIComponent(work.id)}/favorite`, {
    isFavorite: next,
  });
  return next;
}

export async function setWorkTags(work: Work, tags: string[]) {
  await apiClient.put(`/api/works/${encodeURIComponent(work.id)}/tags`, { tags });
}

export async function setWorkCategories(work: Work, categorySlugs: string[]) {
  await apiClient.put(`/api/works/${encodeURIComponent(work.id)}/categories`, {
    categorySlugs,
  });
}

export async function setWorkCoverUrl(work: Work, coverUrl: string) {
  await apiClient.put(`${targetPath(work)}/cover`, { url: coverUrl });
}

export async function uploadWorkCover(work: Work, file: File) {
  const comicId = work.coverComicId || work.representativeComicId;
  if (!comicId) throw new Error("No comic is available for the work cover");
  const form = new FormData();
  form.append("file", file);
  await apiClient.upload(`/api/comics/${encodeURIComponent(comicId)}/cover`, form);
  await apiClient.put(`/api/works/${encodeURIComponent(work.id)}/cover`, {
    coverComicId: comicId,
  });
}
