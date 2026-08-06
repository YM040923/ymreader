/**
 * 漫画相关的共享类型定义
 * 被所有 useComic*.ts hooks 和 api/comics.ts 共用
 */

export interface ApiComicTag {
  name: string;
  color: string;
}

export interface ApiComic {
  id: string;
  title: string;
  titleSortKey?: string;
  filename: string;
  pageCount: number;
  fileSize: number;
  addedAt: string;
  lastReadPage: number;
  lastReadAt: string | null;
  isFavorite: boolean;
  rating: number | null;
  coverUrl: string;
  coverAspectRatio: number;
  sortOrder: number;
  totalReadTime: number;
  tags: ApiComicTag[];
  categories: { id: number; name: string; slug: string; icon: string }[];
  // 元数据字段
  author: string;
  publisher: string;
  year: number | null;
  description: string;
  language: string;
  genre: string;
  metadataSource: string;
  type: string; // 内容类型："comic" | "novel"
  readingStatus?: string; // 用户级阅读状态

  // External rating from scraping sources
  externalRating?: number; // 外部评分原始分数
  externalRatingMax?: number; // 满分值（如 10, 100）
  externalRatingSource?: string; // 评分来源（"anilist", "bangumi"）
  externalRatingUpdatedAt?: string; // 评分更新时间
}

export interface ComicsResponse {
  comics: ApiComic[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface ApiCategory {
  id: number;
  name: string;
  slug: string;
  icon: string;
  count: number;
}
