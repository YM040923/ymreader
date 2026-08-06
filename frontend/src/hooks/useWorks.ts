"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { fetchWorks } from "@/api/works";
import { LIBRARY_ACCESS_CHANGED_EVENT } from "@/hooks/useComicList";
import type { Work, WorkListResponse, WorkQueryOptions } from "@/types/work";
import { buildWorkQuery } from "@/lib/work-model";

const cache = new Map<string, { data: WorkListResponse; timestamp: number }>();
const CACHE_TTL = 30_000;

export function invalidateWorksCache() {
  cache.clear();
}

export function useWorks(options: WorkQueryOptions = {}) {
  const [works, setWorks] = useState<Work[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetching, setFetching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const controller = useRef<AbortController | null>(null);
  const query = buildWorkQuery(options);

  const load = useCallback(async (force = false) => {
    controller.current?.abort();
    controller.current = new AbortController();
    const cached = cache.get(query);
    if (!force && cached && Date.now() - cached.timestamp < CACHE_TTL) {
      setWorks(cached.data.works);
      setTotal(cached.data.total);
      setTotalPages(cached.data.totalPages);
      setLoading(false);
    }
    setFetching(true);
    setError(null);
    try {
      const data = await fetchWorks(options);
      if (controller.current.signal.aborted) return;
      cache.set(query, { data, timestamp: Date.now() });
      setWorks(data.works);
      setTotal(data.total);
      setTotalPages(data.totalPages);
    } catch (reason) {
      if (reason instanceof Error && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "Failed to fetch works");
    } finally {
      if (!controller.current.signal.aborted) {
        setLoading(false);
        setFetching(false);
      }
    }
  }, [query]);

  useEffect(() => {
    void load();
    return () => controller.current?.abort();
  }, [load]);

  useEffect(() => {
    const refresh = () => void load(true);
    window.addEventListener(LIBRARY_ACCESS_CHANGED_EVENT, refresh);
    return () => window.removeEventListener(LIBRARY_ACCESS_CHANGED_EVENT, refresh);
  }, [load]);

  const refetch = useCallback(async () => {
    invalidateWorksCache();
    await load(true);
  }, [load]);

  return { works, setWorks, loading, fetching, error, total, totalPages, refetch };
}
