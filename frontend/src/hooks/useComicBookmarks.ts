import { useState, useEffect, useCallback } from "react";

export interface ComicBookmark {
  comicId: string;
  pageIndex: number;
  createdAt: number;
}

const STORAGE_KEY_PREFIX = "nowen-reader:comic-bookmarks:";

function getStorageKey(comicId: string) {
  return `${STORAGE_KEY_PREFIX}${comicId}`;
}

function loadBookmarks(comicId: string): ComicBookmark[] {
  try {
    const raw = localStorage.getItem(getStorageKey(comicId));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) return parsed as ComicBookmark[];
    return [];
  } catch {
    return [];
  }
}

function saveBookmarks(comicId: string, bookmarks: ComicBookmark[]) {
  localStorage.setItem(getStorageKey(comicId), JSON.stringify(bookmarks));
}

export function useComicBookmarks(comicId: string) {
  const [bookmarks, setBookmarks] = useState<ComicBookmark[]>([]);

  useEffect(() => {
    setBookmarks(loadBookmarks(comicId));
  }, [comicId]);

  const isBookmarked = useCallback(
    (pageIndex: number) => bookmarks.some((b) => b.pageIndex === pageIndex),
    [bookmarks]
  );

  const toggleBookmark = useCallback(
    (pageIndex: number) => {
      setBookmarks((prev) => {
        const exists = prev.some((b) => b.pageIndex === pageIndex);
        let next: ComicBookmark[];
        if (exists) {
          next = prev.filter((b) => b.pageIndex !== pageIndex);
        } else {
          next = [...prev, { comicId, pageIndex, createdAt: Date.now() }].sort(
            (a, b) => a.pageIndex - b.pageIndex
          );
        }
        saveBookmarks(comicId, next);
        return next;
      });
    },
    [comicId]
  );

  const removeBookmark = useCallback(
    (pageIndex: number) => {
      setBookmarks((prev) => {
        const next = prev.filter((b) => b.pageIndex !== pageIndex);
        saveBookmarks(comicId, next);
        return next;
      });
    },
    [comicId]
  );

  return { bookmarks, isBookmarked, toggleBookmark, removeBookmark };
}
