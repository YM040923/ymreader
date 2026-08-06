/**
 * 共享工具函数 — 文件大小格式化、小说文件判断、路由生成等
 *
 * 这些函数在多个页面中重复使用，提取到此处统一维护。
 */

/** 格式化文件大小 */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** 格式化时长（秒 → 可读字符串） */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return m > 0 ? `${h}h${m}m` : `${h}h`;
}

/** 判断是否为小说文件（基于文件后缀） */
export function isNovelFile(filename?: string): boolean {
  if (!filename) return false;
  const ext = filename.toLowerCase();
  return (
    ext.endsWith(".txt") ||
    ext.endsWith(".epub") ||
    ext.endsWith(".mobi") ||
    ext.endsWith(".azw3") ||
    ext.endsWith(".html") ||
    ext.endsWith(".htm")
  );
}

/** 获取阅读器 URL（优先使用数据库 type 字段，fallback 到文件后缀） */
export function getReaderUrl(comic: { id: string; filename?: string; type?: string }): string {
  if (comic.type === "comic") return `/reader/${comic.id}`;
  if (comic.type === "novel") return `/novel/${comic.id}`;
  return isNovelFile(comic.filename) ? `/novel/${comic.id}` : `/reader/${comic.id}`;
}

/** 获取详情页 URL */
export function getDetailUrl(comic: { id: string; filename?: string; type?: string }): string {
  if (comic.type === "comic") return `/comic/${comic.id}`;
  if (comic.type === "novel") return `/novel/${comic.id}`;
  return isNovelFile(comic.filename) ? `/novel/${comic.id}` : `/comic/${comic.id}`;
}

/** 判断漫画是否为小说类型 */
export function isNovelComic(comic: { filename?: string; type?: string }): boolean {
  if (comic.type === "comic") return false;
  if (comic.type === "novel") return true;
  return isNovelFile(comic.filename);
}

/** 自然排序键：将字符串中的数字部分补零对齐，实现数字感知排序 */
export function naturalSortKey(s: string): string {
  return s.replace(/\d+/g, (match) => match.padStart(20, "0")).toLowerCase();
}

const zhNaturalCollator =
  typeof Intl !== "undefined"
    ? new Intl.Collator("zh-Hans-CN-u-co-pinyin", {
        numeric: true,
        sensitivity: "base",
        ignorePunctuation: true,
      })
    : null;

/** 中文拼音 + 数字自然排序比较。 */
export function compareNaturalTitle(a?: string | null, b?: string | null): number {
  const left = (a || "").trim();
  const right = (b || "").trim();
  if (zhNaturalCollator) {
    return zhNaturalCollator.compare(left, right);
  }
  const keyA = naturalSortKey(left);
  const keyB = naturalSortKey(right);
  return keyA < keyB ? -1 : keyA > keyB ? 1 : 0;
}
