import type { ApiComic } from "@/hooks/useComicTypes";

export interface WorkUnit {
  id: string;
  workId: string;
  comicId: string;
  title: string;
  displayLabel: string;
  relativePath?: string;
  internalPath?: string;
  startPage: number;
  pageCount: number;
  sortIndex: number;
  coverUrl?: string;
  coverAspectRatio?: number;
  lastReadPage?: number;
  lastReadAt?: string | null;
  readingStatus?: string;
}

export interface Work {
  id: string;
  libraryId: string;
  title: string;
  rootPath: string;
  seriesId?: string;
  metadataHostType?: "series" | "comic";
  metadataHostId?: string;
  representativeComicId?: string;
  coverComicId: string;
  coverUrl?: string;
  itemCount: number;
  pageCount: number;
  fileSize: number;
  lastReadAt?: string | null;
  addedAt?: string;
  updatedAt?: string;
  sortOrder?: number;
  author?: string;
  publisher?: string;
  year?: number | null;
  description?: string;
  language?: string;
  genre?: string;
  status?: string;
  metadataSource?: string;
  externalRating?: number;
  externalRatingMax?: number;
  externalRatingSource?: string;
  tags?: Array<{ id?: number; name: string; color?: string }>;
  categories?: Array<{ id?: number; name: string; slug?: string; icon?: string }>;
  coverAspectRatio?: number;
  isFavorite?: boolean;
  rating?: number | null;
  readingStatus?: string;
  continueComicId?: string;
  continuePage?: number;
  continueUnitId?: string;
  units: WorkUnit[];
}

export interface WorkQueryOptions {
  search?: string;
  tags?: string[];
  favoritesOnly?: boolean;
  category?: string;
  readingStatus?: string;
  metaFilter?: string;
  uncategorized?: boolean;
  untagged?: boolean;
  libraryIds?: string[];
  sortBy?: string;
  sortOrder?: string;
  page?: number;
  pageSize?: number;
}

export interface WorkListResponse {
  works: Work[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export type WorkComicState = Pick<
  ApiComic,
  "lastReadPage" | "lastReadAt" | "readingStatus"
>;

export interface WorkReadingTarget {
  unit: WorkUnit;
  page: number;
  isContinue: boolean;
}
