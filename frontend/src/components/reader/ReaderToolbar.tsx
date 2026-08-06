"use client";

import {
  ArrowLeft,
  BookOpen,
  Columns2,
  GalleryVertical,
  ChevronLeft,
  ChevronRight,
  ArrowLeftRight,
  Maximize,
  Minimize,
  Info,
  Sun,
  Moon,
  Settings,
  Play,
  Square,
  Bookmark,
  List,
  MoreHorizontal,
} from "lucide-react";
import { useState, useRef, useCallback } from "react";
import { ComicReadingMode, ReadingDirection } from "@/types/reader";
import { useTranslation } from "@/lib/i18n";

export type ReaderTheme = "day" | "night" | "green" | "gray" | "white";

interface ReaderToolbarProps {
  visible: boolean;
  title: string;
  currentPage: number;
  totalPages: number;
  mode: ComicReadingMode;
  direction: ReadingDirection;
  isFullscreen: boolean;
  readerTheme: ReaderTheme;
  onBack: () => void;
  onPageChange: (page: number) => void;
  onModeChange: (mode: ComicReadingMode) => void;
  onDirectionChange: (dir: ReadingDirection) => void;
  onToggleFullscreen: () => void;
  onToggleTheme: () => void;
  onShowInfo?: () => void;
  onShowSettings?: () => void;
  /** 自动翻页是否激活 */
  autoPageActive?: boolean;
  /** 自动翻页间隔（秒），为 0 或未设置时不显示按钮 */
  autoPageInterval?: number;
  /** 切换自动翻页 */
  onToggleAutoPage?: () => void;
  /** 通知父组件工具栏正在被交互，不要自动隐藏 */
  onInteracting?: (interacting: boolean) => void;
  /** 当前页是否已书签 */
  isBookmarked?: boolean;
  /** 切换当前页书签 */
  onToggleBookmark?: () => void;
  /** 打开书签列表 */
  onShowBookmarks?: () => void;
  /** 书签数量 */
  bookmarkCount?: number;
  /** 打开快捷键帮助 */
  onShowShortcutsHelp?: () => void;
  isImmersive?: boolean;
  onToggleImmersive?: () => void;
  /** Experimental realistic book flip */
  realisticFlipEnabled?: boolean;
  canUseRealisticFlip?: boolean;
  onToggleRealisticFlip?: () => void;
  realisticFlipDisabledReason?: string | null;
  onShowThumbnails?: () => void;
}

export default function ReaderToolbar({
  visible,
  title,
  currentPage,
  totalPages,
  mode,
  direction,
  isFullscreen,
  readerTheme,
  onBack,
  onPageChange,
  onModeChange,
  onDirectionChange,
  onToggleFullscreen,
  onToggleTheme,
  onShowInfo,
  onShowSettings,
  autoPageActive,
  autoPageInterval,
  onToggleAutoPage,
  onInteracting,
  isBookmarked,
  onToggleBookmark,
  onShowBookmarks,
  bookmarkCount,
  onShowShortcutsHelp,
  isImmersive,
  onToggleImmersive,
  realisticFlipEnabled,
  canUseRealisticFlip,
  onToggleRealisticFlip,
  realisticFlipDisabledReason,
  onShowThumbnails,
}: ReaderToolbarProps) {
  const t = useTranslation();
  const [showPageInput, setShowPageInput] = useState(false);
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const [pageInputValue, setPageInputValue] = useState("");
  // 进度条拖动状态：拖动中只更新预览值，松手后才真正跳转
  const [isDragging, setIsDragging] = useState(false);
  const [dragValue, setDragValue] = useState(currentPage);
  const rafRef = useRef<number>(0);

  // 拖动中的页码显示值
  const displayPage = isDragging ? dragValue : currentPage;

  // 开始拖动
  const handleSliderStart = useCallback(() => {
    setIsDragging(true);
    setDragValue(currentPage);
    onInteracting?.(true);
  }, [currentPage, onInteracting]);

  // 拖动中：只更新预览值，节流触发页面跳转
  const handleSliderInput = useCallback((e: React.FormEvent<HTMLInputElement>) => {
    const val = Number((e.target as HTMLInputElement).value);
    setDragValue(val);
    // 使用 rAF 节流，拖动中也实时跳转但不会堆积
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(() => {
      onPageChange(val);
    });
  }, [onPageChange]);

  // 松手：确保最终值准确
  const handleSliderEnd = useCallback(() => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    onPageChange(dragValue);
    setIsDragging(false);
    onInteracting?.(false);
  }, [dragValue, onPageChange, onInteracting]);

  // 页码跳转
  const handlePageInputSubmit = () => {
    const num = parseInt(pageInputValue, 10);
    if (!isNaN(num) && num >= 1 && num <= totalPages) {
      onPageChange(num - 1);
    }
    setShowPageInput(false);
    setPageInputValue("");
  };

  const modeOptions: { value: ComicReadingMode; icon: React.ReactNode; label: string }[] = [
    { value: "single", icon: <BookOpen className="h-4 w-4" />, label: t.readerToolbar.single },
    { value: "double", icon: <Columns2 className="h-4 w-4" />, label: t.readerToolbar.double },
    { value: "webtoon", icon: <GalleryVertical className="h-4 w-4" />, label: t.readerToolbar.webtoon },
  ];

  return (
    <>
      {/* Top Bar */}
      <div
        className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ease-out ${
          visible
            ? "translate-y-0 opacity-100"
            : "-translate-y-full opacity-0 pointer-events-none"
        }`}
      >
        <div className="flex h-12 sm:h-14 items-center justify-between reader-toolbar-surface border-b px-3 sm:px-4">
          {/* Left: Back + Title */}
          <div className="flex items-center gap-3 min-w-0">
            <button
              onClick={onBack}
              className="reader-toolbar-button h-9 w-9 shrink-0"
            >
              <ArrowLeft className="h-5 w-5" />
            </button>
            <h1 className="truncate text-sm font-medium reader-text-primary">
              {title}
            </h1>
          </div>

          {/* Right: Bookmark + Settings + More (mobile), all buttons (desktop) */}
          <div className="flex items-center gap-1 relative">
            {/* Bookmark toggle — always visible */}
            {onToggleBookmark && (
              <button
                onClick={onToggleBookmark}
                className={`reader-toolbar-button h-9 w-9 shrink-0 ${
                  isBookmarked
                    ? "reader-toolbar-button-active !text-amber-400 hover:bg-amber-400/20"
                    : ""
                }`}
                title={isBookmarked ? t.readerToolbar.bookmarked : t.readerToolbar.bookmark}
              >
                <Bookmark className={`h-4 w-4 ${isBookmarked ? "fill-current" : ""}`} />
              </button>
            )}
            {/* Settings — always visible */}
            {onShowSettings && (
              <button
                onClick={onShowSettings}
                className="reader-toolbar-button h-9 w-9 shrink-0"
                title={t.readerToolbar.settings}
              >
                <Settings className="h-4 w-4" />
              </button>
            )}
            {/* Info — desktop only */}
            {onShowInfo && (
              <button
                onClick={onShowInfo}
                className="hidden sm:flex reader-toolbar-button h-9 w-9 shrink-0"
              >
                <Info className="h-4 w-4" />
              </button>
            )}
            {/* Bookmark list — desktop only */}
            {onShowBookmarks && bookmarkCount != null && bookmarkCount > 0 && (
              <button
                onClick={onShowBookmarks}
                className="hidden sm:flex reader-toolbar-button h-9 w-9 shrink-0"
                title={t.readerBookmarks.title}
              >
                <List className="h-4 w-4" />
              </button>
            )}
            {/* Fullscreen — desktop only */}
            <button
              onClick={onToggleFullscreen}
              className="hidden sm:flex reader-toolbar-button h-9 w-9 shrink-0"
            >
              {isFullscreen ? (
                <Minimize className="h-4 w-4" />
              ) : (
                <Maximize className="h-4 w-4" />
              )}
            </button>
            {/* Realistic flip toggle — desktop only, experimental */}
            {onToggleRealisticFlip && canUseRealisticFlip && (
              <button
                onClick={onToggleRealisticFlip}
                className={`hidden sm:flex reader-toolbar-button h-9 w-9 shrink-0 ${realisticFlipEnabled ? "reader-toolbar-button-active !text-accent" : ""}`}
                title={realisticFlipEnabled ? (t.readerToolbar?.exitRealisticFlip || "关闭真实翻页") : (t.readerToolbar?.realisticFlip || "真实翻页（实验）")}
              >
                <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>
              </button>
            )}
            {/* More menu — mobile only */}
            <button
              onClick={() => setShowMoreMenu((v) => !v)}
              className="reader-toolbar-button sm:hidden h-9 w-9 shrink-0"
              title={t.readerToolbar.more}
            >
              <MoreHorizontal className="h-5 w-5" />
            </button>
            {/* More dropdown */}
            {showMoreMenu && (
              <>
                <div className="fixed inset-0 z-[60]" onClick={() => setShowMoreMenu(false)} />
                <div className="motion-menu reader-panel-surface absolute right-0 top-full mt-1 z-[61] w-44 overflow-hidden">
                  {onShowInfo && (
                    <button
                      onClick={() => { onShowInfo(); setShowMoreMenu(false); }}
                      className="motion-button flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                    >
                      <Info className="h-4 w-4" />
                      <span>{t.readerToolbar.moreInfo}</span>
                    </button>
                  )}
                  {onShowBookmarks && (
                    <button
                      onClick={() => { onShowBookmarks(); setShowMoreMenu(false); }}
                      className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                    >
                      <List className="h-4 w-4" />
                      <span>{t.readerBookmarks.title}{bookmarkCount != null && bookmarkCount > 0 ? ` (${bookmarkCount})` : ""}</span>
                    </button>
                  )}
                  {onShowShortcutsHelp && (
                    <button
                      onClick={() => { onShowShortcutsHelp(); setShowMoreMenu(false); }}
                      className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                    >
                      <span className="font-mono text-xs">?</span>
                      <span>{t.reader.shortcuts}</span>
                    </button>
                  )}
                                    {onShowThumbnails && (
                    <button
                      onClick={() => { onShowThumbnails(); setShowMoreMenu(false); }}
                      className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                    >
                      <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
                      <span>缩略图</span>
                    </button>
                  )}
                  {onToggleImmersive && (
                    <button
                      onClick={() => { onToggleImmersive(); setShowMoreMenu(false); }}
                      className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                    >
                      <span className="font-mono text-xs">I</span>
                      <span>{isImmersive ? "退出沉浸" : "沉浸模式"}</span>
                    </button>
                  )}
{onToggleRealisticFlip && (
                    <button
                      onClick={() => { if (canUseRealisticFlip) { onToggleRealisticFlip(); setShowMoreMenu(false); } }}
                      disabled={!canUseRealisticFlip && !realisticFlipEnabled}
                      title={realisticFlipDisabledReason || undefined}
                    >
                      <span className="font-mono text-xs">~</span>
                      <span>{realisticFlipEnabled ? (t.readerToolbar?.exitRealisticFlip || "关闭真实翻页") : (t.readerToolbar?.realisticFlip || "真实翻页（实验）")}</span>
                      {realisticFlipDisabledReason && !canUseRealisticFlip && !realisticFlipEnabled && (
                        <span className="ml-auto text-[10px] text-white/35 truncate max-w-[140px]" title={realisticFlipDisabledReason}>
                          {realisticFlipDisabledReason}
                        </span>
                      )}
                    </button>
                  )}
                  <button
                    onClick={() => { onToggleFullscreen(); setShowMoreMenu(false); }}
                    className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-sm reader-text-secondary hover:bg-white/[0.06] hover:text-white transition-colors"
                  >
                    {isFullscreen ? <Minimize className="h-4 w-4" /> : <Maximize className="h-4 w-4" />}
                    <span>{isFullscreen ? t.readerToolbar.exitFullscreen : t.readerToolbar.fullscreen}</span>
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Bottom Bar */}
      <div
        className={`fixed bottom-0 left-0 right-0 z-50 transition-all duration-300 ease-out ${
          visible
            ? "translate-y-0 opacity-100"
            : "translate-y-full opacity-0 pointer-events-none"
        }`}
      >
        <div className="reader-toolbar-surface px-3 sm:px-4 pb-[calc(env(safe-area-inset-bottom)+0.75rem)] border-t">
          {/* Page Slider */}
          <div className="flex items-center gap-3 sm:gap-4 py-2 sm:py-3">
            <button
              onClick={() => onPageChange(Math.max(0, currentPage - 1))}
              disabled={currentPage === 0}
              className="reader-toolbar-button h-9 w-9 sm:h-8 sm:w-8 shrink-0 disabled:opacity-30"
            >
              <ChevronLeft className="h-5 w-5" />
            </button>

            <div className="relative flex-1">
              <input
                type="range"
                min={0}
                max={totalPages - 1}
                value={displayPage}
                onInput={handleSliderInput}
                onChange={handleSliderInput}
                onTouchStart={handleSliderStart}
                onTouchEnd={handleSliderEnd}
                onMouseDown={handleSliderStart}
                onMouseUp={handleSliderEnd}
                className="reader-slider w-full"
              />
            </div>

            <button
              onClick={() => onPageChange(Math.min(totalPages - 1, currentPage + 1))}
              disabled={currentPage >= totalPages - 1}
              className="reader-toolbar-button h-9 w-9 sm:h-8 sm:w-8 shrink-0 disabled:opacity-30"
            >
              <ChevronRight className="h-5 w-5" />
            </button>

            <span
              className="shrink-0 min-w-[60px] text-center text-xs font-mono reader-text-secondary cursor-pointer hover:text-white active:text-accent transition-colors"
              onClick={() => {
                setPageInputValue(String(displayPage + 1));
                setShowPageInput(true);
              }}
            >
              {displayPage + 1} / {totalPages}
            </span>
          </div>

          {/* 页码跳转弹窗 */}
          {showPageInput && (
            <div className="flex items-center gap-2 px-4 pb-2">
              <input
                type="number"
                min={1}
                max={totalPages}
                value={pageInputValue}
                onChange={(e) => setPageInputValue(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handlePageInputSubmit()}
                placeholder={`1-${totalPages}`}
                autoFocus
                className="w-20 rounded-lg bg-white/[0.08] px-2.5 py-1.5 text-xs text-white text-center font-mono placeholder:text-white/30 outline-none reader-focus-ring"
                onFocus={() => onInteracting?.(true)}
                onBlur={() => { onInteracting?.(false); setTimeout(() => setShowPageInput(false), 200); }}
              />
              <button
                onClick={handlePageInputSubmit}
                className="motion-button rounded-lg bg-accent/15 px-3 py-1.5 text-xs text-accent hover:bg-accent/25 transition-colors"
              >
                跳转
              </button>
            </div>
          )}

          {/* Mode & Settings */}
          <div className="flex items-center justify-between border-t reader-divider py-2 sm:py-3">
            {/* Reading Mode */}
            <div className="reader-mode-segment">
              {modeOptions.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => onModeChange(opt.value)}
                  className={`reader-mode-segment-item gap-1 sm:gap-1.5 px-2 sm:px-3 ${
                    mode === opt.value
                      ? "reader-mode-segment-item reader-mode-segment-item-active gap-1 sm:gap-1.5 px-2 sm:px-3"
                      : ""
                  }`}
                >
                  {opt.icon}
                  <span className="hidden sm:inline">{opt.label}</span>
                </button>
              ))}
            </div>

            {/* Direction Toggle — 长条模式下隐藏（长条模式固定为从上到下，无需切换方向） */}
            <div className="flex items-center gap-1 sm:gap-2">
              {mode !== "webtoon" && (
              <button
                onClick={() => {
                  const next = direction === "ltr" ? "rtl" : "ltr";
                  onDirectionChange(next);
                }}
                className={`reader-mode-segment-item gap-1 sm:gap-1.5 px-2 sm:px-3 ${
                  direction === "rtl"
                    ? "reader-toolbar-button-active !text-amber-400"
                    : ""
                }`}
              >
                <ArrowLeftRight className="h-4 w-4" />
                <span className="hidden sm:inline">{direction === "rtl" ? t.readerToolbar.rtl : t.readerToolbar.ltr}</span>
              </button>
              )}

              {/* 自动翻页按钮 */}
              {onToggleAutoPage && autoPageInterval != null && autoPageInterval > 0 && mode !== "webtoon" && (
                <button
                  onClick={onToggleAutoPage}
                  className={`reader-mode-segment-item gap-1 sm:gap-1.5 px-2 sm:px-3 ${
                    autoPageActive
                      ? "reader-toolbar-button-active !text-green-400 animate-pulse"
                      : ""
                  }`}
                  title={autoPageActive ? t.readerToolbar.autoPageStop : t.readerToolbar.autoPage}
                >
                  {autoPageActive ? (
                    <Square className="h-3.5 w-3.5 fill-current" />
                  ) : (
                    <Play className="h-3.5 w-3.5 fill-current" />
                  )}
                  <span className="hidden sm:inline">{autoPageActive ? t.readerToolbar.autoPageStop : t.readerToolbar.autoPage}</span>
                </button>
              )}

              <button
                onClick={onToggleTheme}
                className={`reader-mode-segment-item gap-1 sm:gap-1.5 px-2 sm:px-3 ${
                  readerTheme === "day"
                    ? "reader-toolbar-button-active !text-amber-400"
                    : ""
                }`}
              >
                {readerTheme === "day" ? (
                  <Sun className="h-4 w-4" />
                ) : (
                  <Moon className="h-4 w-4" />
                )}
                <span className="hidden sm:inline">{readerTheme === "day" ? t.readerToolbar.dayMode : t.readerToolbar.nightMode}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
