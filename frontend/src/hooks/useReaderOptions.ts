"use client";

import { useState, useCallback, useEffect } from "react";
import { ReaderOptions, defaultReaderOptions } from "@/types/reader";
import {
  READER_OPTIONS_BY_WORK_KEY,
  resolveReaderOptions,
  splitReaderOptionsUpdate,
} from "@/lib/reader-options-storage";

const STORAGE_KEY = "reader-options";

function readStoredJSON(key: string): unknown {
  const stored = localStorage.getItem(key);
  return stored ? JSON.parse(stored) : undefined;
}

/**
 * 管理阅读器选项。
 * 无 scope 时读写全局默认值；有 scope 时，阅读布局字段按作品覆盖保存。
 */
export function useReaderOptions(scopeId?: string) {
  const [options, setOptions] = useState<ReaderOptions>(defaultReaderOptions);
  const scopeToken = scopeId ?? "__global__";
  const [hydratedScope, setHydratedScope] = useState<string | null>(null);

  useEffect(() => {
    try {
      setOptions(resolveReaderOptions(
        defaultReaderOptions,
        readStoredJSON(STORAGE_KEY),
        readStoredJSON(READER_OPTIONS_BY_WORK_KEY),
        scopeId,
      ));
    } catch {
      setOptions(defaultReaderOptions);
    }
    setHydratedScope(scopeToken);
  }, [scopeId, scopeToken]);

  const updateOptions = useCallback((partial: Partial<ReaderOptions>) => {
    setOptions((prev) => {
      const next = { ...prev, ...partial };
      try {
        if (!scopeId) {
          localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
          return next;
        }

        const { scoped, global } = splitReaderOptionsUpdate(partial);

        if (Object.keys(global).length > 0) {
          const storedGlobal = readStoredJSON(STORAGE_KEY);
          const globalBase =
            typeof storedGlobal === "object" && storedGlobal !== null
              ? storedGlobal
              : {};
          localStorage.setItem(
            STORAGE_KEY,
            JSON.stringify({ ...defaultReaderOptions, ...globalBase, ...global }),
          );
        }

        if (Object.keys(scoped).length > 0) {
          const storedMap = readStoredJSON(READER_OPTIONS_BY_WORK_KEY);
          const scopedMap =
            typeof storedMap === "object" && storedMap !== null
              ? storedMap as Record<string, Partial<ReaderOptions>>
              : {};
          localStorage.setItem(
            READER_OPTIONS_BY_WORK_KEY,
            JSON.stringify({
              ...scopedMap,
              [scopeId]: {
                ...(scopedMap[scopeId] || {}),
                ...scoped,
              },
            }),
          );
        }
      } catch {
        // localStorage 不可用时仍保留当前会话内的设置。
      }
      return next;
    });
  }, [scopeId]);

  return {
    options,
    updateOptions,
    loaded: hydratedScope === scopeToken,
  };
}
