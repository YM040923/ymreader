/**
 * NSFW 判断工具
 * 基于标签名、分类名、标题判断是否为成人内容
 */

// 集中维护的 NSFW 关键词列表
const NSFW_KEYWORDS = [
  "r18", "r-18", "nsfw", "adult", "18+",
  "hentai", "ero", "erotic",
  "成人", "無修正", "无修正",
  "尻", "乳", "巨根", "触手",
  "incest", "lolicon", "shotacon",
];

/**
 * 判断标签列表中是否包含 NSFW 标签
 */
export function hasNSFWTag(tags: { name: string }[]): boolean {
  return tags.some((tag) =>
    NSFW_KEYWORDS.some((kw) => tag.name.toLowerCase().includes(kw))
  );
}

/**
 * 判断标题是否包含 NSFW 关键词（弱匹配，仅作 fallback）
 */
export function hasNSFWTitle(title: string): boolean {
  const lower = title.toLowerCase();
  return NSFW_KEYWORDS.some((kw) => lower.includes(kw));
}

/**
 * 综合判断漫画是否为 NSFW 内容
 * 优先标签，其次标题
 * 支持 tags 为 string[] 或 { name: string }[] 两种格式
 */
export function isNSFW(comic: {
  tags?: (string | { name: string })[];
  tagData?: { name: string }[];
  title?: string;
  filename?: string;
}): boolean {
  // 优先使用 tagData（对象数组）
  if (comic.tagData && comic.tagData.length > 0) {
    if (hasNSFWTag(comic.tagData)) return true;
  }
  // 兼容 tags 为 string[] 的情况
  if (comic.tags && comic.tags.length > 0) {
    const tagNames = comic.tags.map((t) => typeof t === "string" ? t : t.name);
    if (tagNames.some((name) => NSFW_KEYWORDS.some((kw) => name.toLowerCase().includes(kw)))) return true;
  }
  if (comic.title && hasNSFWTitle(comic.title)) return true;
  if (comic.filename && hasNSFWTitle(comic.filename)) return true;
  return false;
}
