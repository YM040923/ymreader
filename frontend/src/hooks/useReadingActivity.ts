"use client";

import { useCallback, useEffect, useRef } from "react";
import { beaconReadingActivity, recordReadingActivity, type ReadingActivityPayload } from "@/api/reading";

interface ReadingActivityOptions {
  comicId: string;
  sessionKey?: string;
  enabled: boolean;
  page: number;
  totalPages: number;
  trackProgress?: boolean;
  workId?: string;
  unitId?: string;
  relativePage?: number;
}

interface ActiveReadingSession {
  comicId: string;
  sessionKey: string;
  clientSessionId: string;
  page: number;
  totalPages: number;
  activeSeconds: number;
  sequence: number;
  trackProgress: boolean;
  finalized: boolean;
  workId?: string;
  unitId?: string;
  relativePage?: number;
}

function createClientSessionId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `web-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function buildPayload(session: ActiveReadingSession, finalize: boolean): ReadingActivityPayload {
  session.sequence += 1;
  return {
    clientSessionId: session.clientSessionId,
    page: session.page,
    totalPages: session.totalPages,
    activeSeconds: session.activeSeconds,
    sequence: session.sequence,
    finalize,
    trackProgress: session.trackProgress,
    ...(session.workId && session.unitId && session.relativePage !== undefined
      ? {
          workId: session.workId,
          unitId: session.unitId,
          relativePage: session.relativePage,
        }
      : {}),
  };
}

export function useReadingActivity({
  comicId,
  sessionKey = comicId,
  enabled,
  page,
  totalPages,
  trackProgress = true,
  workId,
  unitId,
  relativePage,
}: ReadingActivityOptions) {
  const hasPages = totalPages > 0;
  const sessionRef = useRef<ActiveReadingSession | null>(null);
  const session = sessionRef.current;
  if (session?.comicId === comicId && session?.sessionKey === sessionKey) {
    session.page = page;
    session.totalPages = totalPages;
    session.trackProgress = trackProgress;
    session.workId = workId;
    session.unitId = unitId;
    session.relativePage = relativePage;
  }

  const flush = useCallback(async (finalize = false) => {
    const current = sessionRef.current;
    if (!current || current.finalized) return;
    try {
      await recordReadingActivity(current.comicId, buildPayload(current, finalize));
      if (finalize) current.finalized = true;
    } catch {
      // A later heartbeat or the unload beacon will retry the cumulative state.
    }
  }, []);

  useEffect(() => {
    if (!enabled || !comicId || !hasPages) return;

    const current: ActiveReadingSession = {
      comicId,
      sessionKey,
      clientSessionId: createClientSessionId(),
      page,
      totalPages,
      activeSeconds: 0,
      sequence: 0,
      trackProgress,
      finalized: false,
      workId,
      unitId,
      relativePage,
    };
    sessionRef.current = current;

    void flush();
    const activeTimer = window.setInterval(() => {
      if (document.visibilityState === "visible") current.activeSeconds += 1;
    }, 1000);
    const heartbeatTimer = window.setInterval(() => { void flush(); }, 15_000);
    const handleVisibility = () => {
      if (document.visibilityState === "hidden") void flush();
    };
    const handleBeforeUnload = () => {
      if (current.finalized) return;
      beaconReadingActivity(current.comicId, buildPayload(current, true));
    };

    document.addEventListener("visibilitychange", handleVisibility);
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.clearInterval(activeTimer);
      window.clearInterval(heartbeatTimer);
      document.removeEventListener("visibilitychange", handleVisibility);
      window.removeEventListener("beforeunload", handleBeforeUnload);
      if (!current.finalized) {
        const payload = buildPayload(current, true);
        if (!beaconReadingActivity(current.comicId, payload)) {
          void recordReadingActivity(current.comicId, payload);
        }
      }
      if (sessionRef.current === current) sessionRef.current = null;
    };
  }, [comicId, enabled, flush, hasPages, sessionKey]);

  useEffect(() => {
    if (!enabled || !sessionRef.current || sessionRef.current.finalized) return;
    const timer = window.setTimeout(() => { void flush(); }, 600);
    return () => window.clearTimeout(timer);
  }, [enabled, flush, page, relativePage, totalPages, trackProgress, unitId, workId]);

  const finish = useCallback(() => flush(true), [flush]);
  const flushNow = useCallback(() => flush(false), [flush]);
  return { finish, flush: flushNow };
}
