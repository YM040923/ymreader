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
  return `/api/${target.type === "series" ? "series" : "comics"}/${encodeURIComponent(target.id)}`;
}

function comicIds(work: Work): string[] {
  return [...new Set(work.units.map((unit) => unit.comicId))];
}

export async function updateWorkMetadata(work: Work, metadata: WorkMetadataUpdate) {
  const target = getWorkMetadataTarget(work);
  if (target.type === "series") {
    try {
      await apiClient.post(`${targetPath(work)}/apply-metadata`, {
        metadata: { ...metadata, source: "manual" },
        fields: Object.keys(metadata),
        overwrite: true,
        syncToVolumes: false,
      });
    } catch {
      await Promise.all([
        metadata.title
          ? apiClient.put(`/api/series/${encodeURIComponent(target.id)}`, {
              title: metadata.title,
              coverComicId: work.coverComicId,
            })
          : Promise.resolve(),
        ...comicIds(work).map((comicId) =>
          apiClient.put(`/api/comics/${encodeURIComponent(comicId)}/metadata`, metadata),
        ),
      ]);
    }
    return;
  }
  await apiClient.put(`${targetPath(work)}/metadata`, metadata);
}

export async function toggleWorkFavorite(work: Work): Promise<boolean | null> {
  const comicIds = [...new Set(work.units.map((unit) => unit.comicId))];
  if (comicIds.length === 0) return null;
  const next = !work.isFavorite;
  await apiClient.post("/api/comics/batch", {
    action: next ? "favorite" : "unfavorite",
    comicIds,
    isFavorite: next,
  });
  return next;
}

export async function setWorkTags(work: Work, tags: string[]) {
  await Promise.all(comicIds(work).map(async (comicId) => {
    const path = `/api/comics/${encodeURIComponent(comicId)}`;
    await apiClient.delete(`${path}/tags/clear-all`);
    if (tags.length > 0) await apiClient.post(`${path}/tags`, { tags });
  }));
}

export async function setWorkCategories(work: Work, categorySlugs: string[]) {
  await Promise.all(comicIds(work).map((comicId) =>
    apiClient.put(`/api/comics/${encodeURIComponent(comicId)}/categories`, {
      categorySlugs,
    }),
  ));
}

export async function setWorkCoverUrl(work: Work, coverUrl: string) {
  const target = getWorkMetadataTarget(work);
  if (target.type === "series") {
    await apiClient.post(`${targetPath(work)}/apply-metadata`, {
      metadata: { coverUrl, source: "manual" },
      fields: ["cover"],
      overwrite: true,
      syncToVolumes: false,
    });
    return;
  }
  await apiClient.post(`${targetPath(work)}/cover`, { url: coverUrl });
}

export async function uploadWorkCover(work: Work, file: File) {
  const target = getWorkMetadataTarget(work);
  const comicId = target.type === "comic"
    ? target.id
    : work.coverComicId || work.representativeComicId;
  if (!comicId) throw new Error("No comic is available for the work cover");
  const form = new FormData();
  form.append("file", file);
  await apiClient.upload(`/api/comics/${encodeURIComponent(comicId)}/cover`, form);
  if (target.type === "series") {
    await apiClient.put(`/api/series/${encodeURIComponent(target.id)}`, {
      title: work.title,
      coverComicId: comicId,
    });
  }
}
