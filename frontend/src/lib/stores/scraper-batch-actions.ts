import { fetchLibraryScrapeTargets } from "@/api/libraries";
import { apiPath } from "@/lib/base-path";
/**
 * 刮削状态管理 — 刮削批处理 Actions
 *
 * 包含：批量刮削启动/停止、统计加载、进度管理等。
 */

import { getState, notify } from "./scraper-core";
import type {
  BatchMode,
  ScrapeScope,
  CompletedItem,
  ProgressItem,
} from "./scraper-types";

let abortController: AbortController | null = null;

function normalizeProgressItem(
  data: ProgressItem,
  knownTitles: Map<string, string> = new Map(),
): ProgressItem {
  const state = getState();
  const displayTitle =
    data.workTitle ||
    data.title ||
    knownTitles.get(data.entityId || "") ||
    knownTitles.get(data.comicId) ||
    state.libraryItems.find((item) => item.id === data.comicId)?.title ||
    data.filename;

  return {
    ...data,
    displayTitle,
    libraryId: data.libraryId || state.currentProgress?.libraryId,
  };
}

async function consumeBatchResponse(
  res: Response,
  knownTitles: Map<string, string> = new Map(),
) {
  if (!res.ok) {
    const errData = await res.json().catch(() => ({ error: "Request failed" }));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }

  const reader = res.body?.getReader();
  if (!reader) throw new Error("Empty scrape response");

  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n\n");
    buffer = lines.pop() || "";

    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      try {
        const data = JSON.parse(line.slice(6));
        const state = getState();
        if (data.type === "complete") {
          state.batchDone = data;
          notify();
          continue;
        }
        if (data.type !== "progress") continue;

        const progress = normalizeProgressItem(data, knownTitles);
        state.currentProgress = progress;
        if (
          progress.status === "success" ||
          progress.status === "failed" ||
          progress.status === "skipped"
        ) {
          state.completedItems = [
            ...state.completedItems,
            { ...progress, id: `${progress.comicId}-${Date.now()}` },
          ];
        }
        notify();
      } catch {
        /* skip malformed SSE event */
      }
    }
  }
}

function setTaskError(error: Error, title = "刮削任务", libraryId?: string) {
  const state = getState();
  state.batchDone = { type: "complete", success: 0, failed: 1, total: 1 };
  state.completedItems = [{
    type: "progress",
    current: 0,
    total: 1,
    comicId: "",
    filename: title,
    displayTitle: title,
    libraryId,
    status: "failed",
    message: error.message,
    id: `error-${Date.now()}`,
  } as CompletedItem];
  notify();
}

export function setBatchMode(mode: BatchMode) {
  const state = getState();
  if (state.batchRunning) return;
  state.batchMode = mode;
  notify();
}

export function setScrapeScope(scope: ScrapeScope) {
  const state = getState();
  if (state.batchRunning) return;
  state.scrapeScope = scope;
  notify();
}

export function setShowResults(show: boolean) {
  getState().showResults = show;
  notify();
}

export function setUpdateTitle(enabled: boolean) {
  const state = getState();
  if (state.batchRunning) return;
  state.updateTitle = enabled;
  notify();
}

export function setSkipCover(enabled: boolean) {
  const state = getState();
  if (state.batchRunning) return;
  state.skipCover = enabled;
  notify();
}

export async function loadStats() {
  const state = getState();
  state.statsLoading = true;
  notify();
  try {
    const res = await fetch(apiPath("/api/metadata/stats"));
    if (res.ok) {
      getState().stats = await res.json();
    }
  } catch {
    // ignore
  } finally {
    getState().statsLoading = false;
    notify();
  }
}

export async function startBatch() {
  const state = getState();
  if (state.batchRunning) return;

  state.batchRunning = true;
  state.currentProgress = null;
  state.batchDone = null;
  state.completedItems = [];
  state.showResults = true;
  notify();

  const abort = new AbortController();
  abortController = abort;

  const endpoint =
    state.batchMode === "ai" ? "/api/metadata/ai-batch" : "/api/metadata/batch";
  const lang = navigator.language.startsWith("zh") ? "zh" : "en";

  try {
    const res = await fetch(apiPath(endpoint), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        mode: state.scrapeScope,
        lang,
        updateTitle: state.updateTitle,
        skipCover: state.skipCover,
      }),
      signal: abort.signal,
    });
    await consumeBatchResponse(res);
  } catch (err) {
    if ((err as Error).name !== "AbortError") {
      setTaskError(err as Error);
    }
  } finally {
    const current = getState();
    current.batchRunning = false;
    abortController = null;
    notify();
    loadStats();
  }
}

export async function startLibraryScrape(libraryId: string, libraryTitle: string) {
  const state = getState();
  if (state.batchRunning) return;

  state.batchRunning = true;
  state.currentProgress = {
    type: "progress",
    current: 0,
    total: 0,
    comicId: "",
    filename: libraryTitle,
    displayTitle: libraryTitle,
    libraryId,
    entityType: "work",
    step: "search",
    status: "running",
  };
  state.batchDone = null;
  state.completedItems = [];
  state.showResults = true;
  notify();

  const abort = new AbortController();
  abortController = abort;
  const lang = navigator.language.startsWith("zh") ? "zh" : "en";

  try {
    const targets = await fetchLibraryScrapeTargets(libraryId);
    const knownTitles = new Map(targets.map((target) => [target.id, target.title]));
    if (targets.length === 0) {
      const current = getState();
      current.currentProgress = null;
      current.batchDone = { type: "complete", total: 0, success: 0, failed: 0 };
      notify();
      return;
    }

    const current = getState();
    current.currentProgress = {
      ...current.currentProgress!,
      total: targets.length,
    };
    notify();

    const res = await fetch(apiPath("/api/metadata/batch-selected"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        targets: targets.map((target) => ({ id: target.id, entityType: "work" })),
        comicIds: targets.map((target) => target.id),
        lang,
        updateTitle: state.updateTitle,
        mode: state.batchMode,
        skipCover: state.skipCover,
      }),
      signal: abort.signal,
    });
    await consumeBatchResponse(res, knownTitles);
  } catch (err) {
    if ((err as Error).name !== "AbortError") {
      setTaskError(err as Error, libraryTitle, libraryId);
      throw err;
    }
  } finally {
    const current = getState();
    current.batchRunning = false;
    abortController = null;
    notify();
    loadStats();
  }
}

export function cancelBatch() {
  abortController?.abort();
  getState().batchRunning = false;
  notify();
}
