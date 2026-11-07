"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ReaderTheme } from "@/components/reader/ReaderToolbar";
import { useImagePreloader } from "@/hooks/useImagePreloader";

interface HorizontalStripViewProps {
  pages: string[];
  currentPage: number;
  onPageChange: (page: number) => void;
  onTapCenter: () => void;
  useRealData?: boolean;
  readerTheme?: ReaderTheme;
  /** 横向长条的阅读方向：rtl = 从右往左（日漫），ltr = 从左往右 */
  direction?: "ltr" | "rtl";
  onBoundaryReached?: (direction: "next" | "prev") => void;
  nextVolumeTitle?: string;
  imageFilter?: string;
  comicId?: string;
  preloadCount?: number;
}

const SCROLL_REPORT_THROTTLE = 100;
/** 未加载时按常见漫画比例给一个占位宽度，避免整条塌陷 */
const DEFAULT_ASPECT = 0.7;

/**
 * 横向长条：页面按高度铺满，横向连续滚动。
 * 与安卓端的“横向长条”一致：rtl 时从右往左排，滚轮/触控板纵向滚动会转成横向滚动。
 */
export default function HorizontalStripView({
  pages,
  currentPage,
  onPageChange,
  onTapCenter,
  readerTheme = "night",
  direction = "rtl",
  onBoundaryReached,
  nextVolumeTitle,
  imageFilter = "",
  comicId,
  preloadCount = 5,
}: HorizontalStripViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const pageRefs = useRef<(HTMLDivElement | null)[]>([]);
  const lastReportedPage = useRef(currentPage);
  const lastReportTime = useRef(0);
  const boundaryLock = useRef(false);
  const [loaded, setLoaded] = useState<Record<number, boolean>>({});
  // 每页按容器高度折算出的整数像素宽度；用整数可避免相邻页累加
  // 小数误差后露出底色，形成竖直发丝缝。
  const [widths, setWidths] = useState<Record<number, number>>({});
  // 每页原始宽高比，供窗口尺寸变化（含全屏切换）后重算像素宽度
  const ratiosRef = useRef<Record<number, number>>({});
  const isDay = readerTheme === "day";
  // 已加载页的平均宽高比，用来给未加载页一个接近真实的占位宽度
  const aspectRef = useRef(DEFAULT_ASPECT);
  const aspectSamplesRef = useRef(0);
  const [, setLayoutTick] = useState(0);
  // 外部跳页的目标：等目标页及其前面的占位宽度稳定后再补一次居中
  const pendingTarget = useRef<number | null>(null);
  const didMountRef = useRef(false);

  useImagePreloader(pages, currentPage, preloadCount, comicId);

  const centerPageAt = useCallback((index: number, behavior: ScrollBehavior) => {
    const container = containerRef.current;
    const el = pageRefs.current[index];
    if (!container || !el) return;
    const containerRect = container.getBoundingClientRect();
    const rect = el.getBoundingClientRect();
    const delta =
      rect.left + rect.width / 2 - (containerRect.left + containerRect.width / 2);
    container.scrollTo({ left: container.scrollLeft + delta, behavior });
  }, []);

  // 外部翻页（键盘、滑条、上一话）时把目标页滚到中间
  useEffect(() => {
    if (!didMountRef.current) {
      didMountRef.current = true;
      pendingTarget.current = currentPage;
      centerPageAt(currentPage, "auto");
      return;
    }
    if (currentPage === lastReportedPage.current) return;
    pendingTarget.current = currentPage;
    centerPageAt(currentPage, "smooth");
    lastReportedPage.current = currentPage;
  }, [centerPageAt, currentPage]);

  // 图片陆续加载后占位宽度会变，目标页可能偏出去；在目标页本身加载完成前持续回正
  useEffect(() => {
    const target = pendingTarget.current;
    if (target == null) return;
    centerPageAt(target, "auto");
    if (loaded[target]) pendingTarget.current = null;
  }, [centerPageAt, loaded]);

  // 窗口尺寸或全屏状态变化时，容器高度变了，按高度铺满的页宽必须跟着重算，
  // 否则盒子比屏幕矮，图片只能按宽度收敛，上下就会露出留白。
  // 按当前容器高度，把所有已知比例的页重算成整数像素宽。
  // 图片加载、窗口缩放、全屏切换都会走这里，保证盒子高度始终等于容器高度。
  const recomputeWidths = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;
    const top = container.clientHeight;
    if (top <= 0) return;
    const next: Record<number, number> = {};
    for (const [key, ratio] of Object.entries(ratiosRef.current)) {
      next[Number(key)] = Math.round(top * ratio);
    }
    setWidths((prev) => {
      const changed = Object.keys(next).some((k) => prev[Number(k)] !== next[Number(k)]);
      return changed ? { ...prev, ...next } : prev;
    });
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const recompute = () => recomputeWidths();
    const observer = new ResizeObserver(recompute);
    observer.observe(container);
    window.addEventListener("resize", recompute);
    // 全屏切换后 innerHeight 更新有延迟，补两拍
    const onFullscreen = () => {
      recompute();
      setTimeout(recompute, 60);
      setTimeout(recompute, 240);
    };
    document.addEventListener("fullscreenchange", onFullscreen);
    document.addEventListener("webkitfullscreenchange", onFullscreen as EventListener);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", recompute);
      document.removeEventListener("fullscreenchange", onFullscreen);
      document.removeEventListener("webkitfullscreenchange", onFullscreen as EventListener);
    };
  }, [recomputeWidths]);

  // 滚动时反推当前页
  const handleScroll = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;

    // 离开两端后解除边界锁
    const maxScroll = Math.max(0, container.scrollWidth - container.clientWidth);
    const position = Math.abs(container.scrollLeft);
    if (position > 4 && position < maxScroll - 4) {
      boundaryLock.current = false;
    }

    const now = Date.now();
    if (now - lastReportTime.current < SCROLL_REPORT_THROTTLE) return;
    lastReportTime.current = now;

    const center = container.getBoundingClientRect().left + container.clientWidth / 2;
    let bestIndex = 0;
    let bestDistance = Number.POSITIVE_INFINITY;
    pageRefs.current.forEach((el, index) => {
      if (!el) return;
      const r = el.getBoundingClientRect();
      const distance = Math.abs(r.left + r.width / 2 - center);
      if (distance < bestDistance) {
        bestDistance = distance;
        bestIndex = index;
      }
    });

    if (bestIndex !== lastReportedPage.current) {
      lastReportedPage.current = bestIndex;
      onPageChange(bestIndex);
    }
  }, [onPageChange]);

  // 滚轮：纵向转横向；已经贴边还继续往同方向滚，才翻上一话 / 下一话
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const onWheel = (event: WheelEvent) => {
      if (event.deltaY === 0) return;
      if (Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      event.preventDefault();
      pendingTarget.current = null;

      const maxScroll = Math.max(0, el.scrollWidth - el.clientWidth);
      const position = Math.abs(el.scrollLeft);
      const forward = event.deltaY > 0;
      const atEnd = position >= maxScroll - 2;
      const atStart = position <= 2;
      const atForwardEdge = direction === "rtl" ? atEnd : atStart;
      const atBackEdge = direction === "rtl" ? atStart : atEnd;

      if (onBoundaryReached && !boundaryLock.current) {
        if (atForwardEdge && forward) {
          boundaryLock.current = true;
          onBoundaryReached("next");
          return;
        }
        if (atBackEdge && !forward) {
          boundaryLock.current = true;
          onBoundaryReached("prev");
          return;
        }
      }

      el.scrollBy({
        left: direction === "rtl" ? -event.deltaY : event.deltaY,
        behavior: "auto",
      });
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [direction, onBoundaryReached]);

  // 点击：中间切换工具栏，两侧翻页（rtl 时左侧是下一页）
  const handleClick = (event: React.MouseEvent<HTMLDivElement>) => {
    pendingTarget.current = null;
    const rect = event.currentTarget.getBoundingClientRect();
    const ratio = (event.clientX - rect.left) / rect.width;
    if (ratio > 0.3 && ratio < 0.7) {
      onTapCenter();
      return;
    }
    const container = containerRef.current;
    if (!container) return;

    // 用点击这一刻屏幕上居中的页做基准，避免平滑滚动途中 currentPage 还没更新
    const center = rect.left + rect.width / 2;
    let baseIndex = 0;
    let bestDistance = Number.POSITIVE_INFINITY;
    pageRefs.current.forEach((el, index) => {
      if (!el) return;
      const r = el.getBoundingClientRect();
      const distance = Math.abs(r.left + r.width / 2 - center);
      if (distance < bestDistance) {
        bestDistance = distance;
        baseIndex = index;
      }
    });

    const forward = direction === "rtl" ? ratio < 0.3 : ratio > 0.7;
    const targetIndex = forward ? baseIndex + 1 : baseIndex - 1;
    if (targetIndex < 0) {
      onBoundaryReached?.("prev");
      return;
    }
    if (targetIndex > pages.length - 1) {
      onBoundaryReached?.("next");
      return;
    }
    const el = pageRefs.current[targetIndex];
    if (!el) return;
    const containerRect = container.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    const delta = r.left + r.width / 2 - (containerRect.left + containerRect.width / 2);
    container.scrollTo({ left: container.scrollLeft + delta, behavior: "smooth" });
  };

  return (
    <div
      ref={containerRef}
      className={`h-dvh w-full select-none overflow-x-auto overflow-y-hidden scrollbar-hide ${
        isDay ? "bg-gray-100" : "bg-black"
      }`}
      style={{ direction: direction === "rtl" ? "rtl" : "ltr" }}
      onScroll={handleScroll}
      onClick={handleClick}
    >
      <div
        className="flex h-full items-stretch"
        style={{ width: "max-content" }}
      >
        {pages.map((src, index) => (
          <div
            key={`${index}-${src}`}
            ref={(el) => {
              pageRefs.current[index] = el;
            }}
            className="relative flex h-full shrink-0 items-center justify-center overflow-hidden"
            style={{
              width: widths[index]
                ? `${widths[index]}px`
                : `${Math.round(aspectRef.current * 1000) / 10}vh`,
            }}
          >
            {!loaded[index] && (
              <div
                className={`absolute inset-0 animate-pulse ${
                  isDay ? "bg-black/[0.06]" : "bg-white/[0.06]"
                }`}
              />
            )}
            <img
              src={src}
              alt={`第 ${index + 1} 页`}
              loading={Math.abs(index - currentPage) < 3 ? "eager" : "lazy"}
              draggable={false}
              onLoad={(event) => {
                const img = event.currentTarget;
                if (img.naturalWidth > 0 && img.naturalHeight > 0) {
                  const ratio = Math.min(
                    1.6,
                    Math.max(0.25, img.naturalWidth / img.naturalHeight),
                  );
                  // 只用前几张校准占位比例，避免阅读中不断改变整条布局
                  if (aspectSamplesRef.current < 8) {
                    aspectSamplesRef.current += 1;
                    aspectRef.current =
                      aspectRef.current * 0.6 + ratio * 0.4;
                    setLayoutTick((tick) => tick + 1);
                  }
                  // 按容器高度折算成整数像素宽，相邻页相加不会累积小数误差
                  const pageRatio = img.naturalWidth / img.naturalHeight;
                  ratiosRef.current[index] = pageRatio;
                  // 图片可能早于布局加载完（此时容器高度还是 0），
                  // 所以统一走一次重算，而不是在这里死记一个宽度。
                  requestAnimationFrame(() => recomputeWidths());
                }
                setLoaded((prev) =>
                  prev[index] ? prev : { ...prev, [index]: true }
                );
              }}
              className="h-full w-full max-w-none select-none object-contain"

              style={imageFilter ? { filter: imageFilter } : undefined}
            />
          </div>
        ))}

        <div className="flex h-full w-52 shrink-0 flex-col items-center justify-center gap-3 text-center">
          <span className={`text-xs ${isDay ? "text-gray-400" : "text-white/40"}`}>
            已到本话末尾
          </span>
          {nextVolumeTitle && onBoundaryReached && (
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                onBoundaryReached("next");
              }}
              className="rounded-xl bg-accent px-4 py-2 text-xs font-medium text-white shadow-sm shadow-accent/25 transition-colors hover:bg-accent/90"
            >
              {nextVolumeTitle} →
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
