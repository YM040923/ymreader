"use client";

import { useEffect, useCallback, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import {
  ArrowLeft,
  Database,
  Sparkles,
  Search,
  Play,
  Square,
  CheckCircle,
  XCircle,
  AlertCircle,
  Loader2,
  RefreshCw,
  Brain,
  FileText,
  Tag,
  Clock,
  ChevronDown,
  ChevronUp,
  ChevronLeft,
  ChevronRight,
  RotateCcw,
  Library,
  Trash2,
  BookOpen,
  CheckSquare,
  Filter,
  Eye,
  X,
  User,
  Zap,
  Pencil,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  MessageCircle,
  Bot,
  HelpCircle,
  CircleHelp,
  AlertTriangle,
} from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { useAuth } from "@/lib/auth-context";
import { apiPath } from "@/lib/base-path";
import { useAIStatus } from "@/hooks/useAIStatus";
import { useScraperStore } from "@/hooks/useScraperStore";
import {
  loadStats,
  startBatch,
  cancelBatch,
  setBatchMode,
  setScrapeScope,
  setShowResults,
  setUpdateTitle,
  setSkipCover,
  loadLibrary,
  setLibrarySearch,
  setLibraryMetaFilter,
  setLibraryContentType,
  setLibraryPage,
  setLibraryPageSize,
  toggleSelectItem,
  selectAllVisible,
  deselectAll,
  startBatchSelected,
  clearSelectedMetadata,
  setFocusedItem,
  enterBatchEditMode,
  exitBatchEditMode,
  setBatchEditName,
  setLibrarySort,
  toggleAIChat,
  closeAIChat,
  openAIChat,
  startGuide,
  checkAutoStartGuide,
  openHelpPanel,
  closeHelpPanel,
  // 文件夹模式
  setViewMode,
  setSelectedFolderPath,
  setFolderSearch,
  loadFolderTree,
  // 系列模式
  // 批量在线刮削
  // 系列分页
  // 脏数据检测与清理
} from "@/lib/scraper-store";
import type { LibraryItem, BatchEditNameEntry, AIChatMessage, MetadataFolderNode, ViewMode, MetaFilter, LibrarySortBy } from "@/lib/scraper-store";
import { FolderOpen, FolderPlus, Layers, Plus, Minus, FolderTree, Folder, List } from "lucide-react";
import { useResizablePanel } from "@/hooks/useResizablePanel";
import { ResizeDivider } from "@/components/ResizeDivider";
import { useGlobalSyncEvent } from "@/hooks/useSyncEvent";

import {
  filterMetadataFolderTree,
  highlightSearchText,
  MetadataFolderTreeItem,
} from "@/components/scraper/FolderTreeItem";
import { FolderScrapePanel } from "@/components/scraper/FolderScrapePanel";
import { GuideOverlay } from "@/components/scraper/GuideOverlay";
import { HelpPanel } from "@/components/scraper/HelpPanel";
import { AIChatPanel } from "@/components/scraper/AIChatPanel";
import { BatchEditPanel } from "@/components/scraper/BatchEditPanel";
import { DetailPanel } from "@/components/scraper/DetailPanel";

/* ── 主页面 ── */
export default function ScraperPage() {
  const router = useRouter();
  const t = useTranslation();
  const { user } = useAuth();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const scraperT = (t as any).scraper || {};

  // 检查刮削功能是否启用
  const [scraperEnabled, setScraperEnabled] = useState<boolean | null>(null);
  useEffect(() => {
    fetch(apiPath("/api/site-settings"))
      .then(r => r.json())
      .then(data => setScraperEnabled(data.scraperEnabled ?? true))
      .catch(() => setScraperEnabled(true));
  }, []);

  const {
    stats,
    statsLoading,
    batchRunning,
    batchMode,
    scrapeScope,
    updateTitle,
    skipCover,
    currentProgress,
    batchDone,
    completedItems,
    showResults,
    libraryItems,
    libraryLoading,
    librarySearch,
    libraryMetaFilter,
    libraryContentType,
    libraryPage,
    libraryPageSize,
    libraryTotalPages,
    libraryTotal,
    selectedIds,
    focusedItemId,
    batchEditMode,
    batchEditNames,
    batchEditSaving,
    batchEditResults,
    aiRenameLoading,
    librarySortBy,
    librarySortOrder,
    aiChatOpen,
    aiChatMessages,
    aiChatLoading,
    aiChatInput,
    guideActive,
    guideCurrentStep,
    guideDismissed,
    helpPanelOpen,
    helpSearchQuery,
    // 合集管理
    // 文件夹模式
    viewMode,
    folderTree,
    folderTreeLoading,
    selectedFolderPath,
    folderSearch,
    folderScrapeRunning,
    folderScrapeProgress,
    folderScrapeDone,
    // 系列模式
    // 系列分页
    // 批量在线刮削
    // 脏数据检测与清理
  } = useScraperStore();

  const isAdmin = user?.role === "admin";
  const { aiConfigured } = useAIStatus();
  const [detailRefreshKey, setDetailRefreshKey] = useState(0);

  // 首次挂载加载
  useEffect(() => {
    if (!stats && !statsLoading) loadStats();
    loadLibrary();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // 监听来自详情页/其他标签页的同步事件，自动刷新列表
  useGlobalSyncEvent((event) => {
    // 刷新列表数据
    loadLibrary();
    loadStats();
    // 如果当前正在查看被修改的漫画，也刷新详情
    if (focusedItemId === event.comicId) {
      loadLibrary();
    }
  }, { ignoreSource: "scraper" });

  // 首次使用引导检测
  useEffect(() => {
    if (stats && !guideDismissed && !guideActive) {
      checkAutoStartGuide();
    }
  }, [stats, guideDismissed, guideActive]); // eslint-disable-line react-hooks/exhaustive-deps

  // 当筛选/分页/搜索变化时重新加载
  useEffect(() => {
    loadLibrary();
  }, [libraryPage, libraryPageSize, libraryMetaFilter, libraryContentType, librarySortBy, librarySortOrder]); // eslint-disable-line react-hooks/exhaustive-deps

  // 搜索防抖
  useEffect(() => {
    const timer = setTimeout(() => loadLibrary(), 300);
    return () => clearTimeout(timer);
  }, [librarySearch]); // eslint-disable-line react-hooks/exhaustive-deps

  const progressPercent = currentProgress
    ? Math.round((currentProgress.current / currentProgress.total) * 100)
    : 0;

  const metaPercent =
    stats && stats.total > 0
      ? Math.round((stats.withMetadata / stats.total) * 100)
      : 0;

  const handleSearchKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter") loadLibrary();
    },
    []
  );

  // 当前聚焦的详情项
  const focusedItem = focusedItemId
    ? libraryItems.find((item) => item.id === focusedItemId) ?? null
    : null;

  // 当前聚焦的系列
  // 系列列表筛选 + 排序 + 分页
  // 滚动引用
  const listRef = useRef<HTMLDivElement>(null);

  // 右侧面板可拖拽宽度
  const {
    width: rightPanelWidth,
    isDragging: isResizing,
    handleMouseDown: handleResizeMouseDown,
    resetWidth: resetRightPanelWidth,
  } = useResizablePanel({
    storageKey: "scraper-right-panel-width",
    defaultWidth: 520,
    minWidth: 360,
    maxWidth: 800,
    side: "right",
  });

  // 加载中（注意：必须放在所有 Hooks 调用之后再做条件渲染，避免 Hooks 顺序变化）
  if (scraperEnabled === null) {
    return (
      <div className="h-screen flex items-center justify-center bg-background">
        <Loader2 className="h-6 w-6 animate-spin text-muted" />
      </div>
    );
  }

  // 刮削功能未启用时显示提示页面
  if (scraperEnabled === false) {
    return (
      <div className="h-screen flex flex-col items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-4 max-w-md text-center px-6">
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted/20">
            <Database className="h-8 w-8 text-muted" />
          </div>
          <h1 className="text-xl font-bold text-foreground">
            {scraperT.title || "元数据刮削"}
          </h1>
          <p className="text-sm text-muted">
            内容刮削功能当前已关闭。请在「设置 → 站点设置」中启用「内容刮削」开关后再使用。
          </p>
          <button
            onClick={() => router.push("/settings?tab=site")}
            className="mt-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/90 transition-colors"
          >
            前往设置
          </button>
          <button
            onClick={() => router.push("/")}
            className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-muted hover:text-foreground hover:bg-card-hover transition-colors"
          >
            返回首页
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col bg-background">
      {/* ═══════════ Header ═══════════ */}
      <header data-guide="header" className="sticky top-0 z-50 border-b border-border/40 bg-background/80 backdrop-blur-2xl flex-shrink-0">
        <div className="mx-auto flex h-14 sm:h-16 max-w-[1800px] items-center gap-3 px-3 sm:px-6">
          <button
            onClick={() => router.push("/")}
            className="group flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl border border-border/50 text-muted transition-all hover:border-accent/40 hover:text-accent hover:bg-accent/5"
          >
            <ArrowLeft className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" />
          </button>
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 shadow-lg shadow-purple-500/20">
              <Database className="h-4 w-4 text-white" />
            </div>
            <div>
              <h1 className="text-base sm:text-lg font-bold text-foreground">
                {scraperT.title || "元数据刮削"}
              </h1>
              <p className="hidden sm:block text-xs text-muted -mt-0.5">
                {scraperT.subtitle || "自动获取封面、简介、标签等信息"}
              </p>
            </div>
          </div>

          {/* 统计信息 */}
          <div className="ml-auto flex items-center gap-3">
            {stats && (
              <div className="hidden sm:flex items-center gap-4 text-xs">
                <div className="flex items-center gap-1.5">
                  <FileText className="h-3.5 w-3.5 text-muted" />
                  <span className="text-muted">{scraperT.statsTotal || "总计"}</span>
                  <span className="font-bold text-foreground">{stats.total}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  <CheckCircle className="h-3.5 w-3.5 text-emerald-500" />
                  <span className="font-bold text-emerald-500">{stats.withMetadata}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  <AlertCircle className="h-3.5 w-3.5 text-amber-500" />
                  <span className="font-bold text-amber-500">{stats.missing}</span>
                </div>
                {/* 进度条 */}
                <div className="w-20 h-1.5 rounded-full bg-border/30 overflow-hidden">
                  <div
                    className="h-full rounded-full bg-gradient-to-r from-accent to-emerald-500 transition-all duration-700"
                    style={{ width: `${metaPercent}%` }}
                  />
                </div>
                <span className="font-medium text-accent">{metaPercent}%</span>
              </div>
            )}
            <button
              onClick={loadStats}
              disabled={statsLoading}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-muted hover:text-foreground hover:bg-card-hover transition-colors disabled:opacity-50"
            >
              <RotateCcw className={`h-3.5 w-3.5 ${statsLoading ? "animate-spin" : ""}`} />
            </button>
          </div>
        </div>
      </header>

      {/* ═══════════ 主体：左右分栏 ═══════════ */}
      <div className="flex-1 flex overflow-hidden">
        {/* ── 左侧面板：书库列表 ── */}
        <div className={`flex-1 flex flex-col min-w-0 ${isResizing ? '' : 'border-r border-border/30'}`}>
          {/* 搜索 & 筛选 */}
          <div data-guide="filter-bar" className="flex-shrink-0 p-3 sm:p-4 space-y-3 border-b border-border/20 bg-card/30">
            {/* 视图模式切换 + 搜索框 */}
            <div className="flex items-center gap-2">
              {/* 模式切换 */}
              <div className="flex rounded-lg border border-border/40 overflow-hidden flex-shrink-0">
                <button
                  onClick={() => setViewMode("list")}
                  className={`flex items-center gap-1 px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                    viewMode === "list"
                      ? "bg-accent text-white"
                      : "text-muted hover:text-foreground hover:bg-white/5"
                  }`}
                  title="列表模式"
                >
                  <List className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">列表</span>
                </button>
                <button
                  onClick={() => setViewMode("folder")}
                  className={`flex items-center gap-1 px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                    viewMode === "folder"
                      ? "bg-amber-500 text-white"
                      : "text-muted hover:text-foreground hover:bg-white/5"
                  }`}
                  title="文件夹模式"
                >
                  <FolderTree className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">文件夹</span>
                </button>
              </div>
              {/* 搜索框 */}
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted" />
                <input
                  type="text"
                  value={viewMode === "folder" ? folderSearch : librarySearch}
                  onChange={(e) => viewMode === "folder" ? setFolderSearch(e.target.value) : setLibrarySearch(e.target.value)}
                  onKeyDown={viewMode === "list" ? handleSearchKeyDown : undefined}
                  placeholder={viewMode === "folder" ? "搜索文件夹或文件名..." : false ? "搜索系列名称..." : (scraperT.libSearchPlaceholder || "搜索书名、文件名...")}
                  className="w-full rounded-xl bg-card-hover/50 pl-10 pr-4 py-2 text-sm text-foreground placeholder-muted/50 outline-none border border-border/40 focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                />
              </div>
            </div>

            {/* 筛选（仅列表模式） */}
            {viewMode === "list" && (
            <div className="flex flex-wrap items-center gap-1.5">
              {(["all", "missing", "with"] as MetaFilter[]).map((f) => (
                <button
                  key={f}
                  onClick={() => setLibraryMetaFilter(f)}
                  className={`rounded-lg px-2 py-1 text-[11px] font-medium transition-all ${
                    libraryMetaFilter === f
                      ? f === "missing" ? "bg-amber-500 text-white" : f === "with" ? "bg-emerald-500 text-white" : "bg-accent text-white"
                      : "bg-card-hover text-muted hover:text-foreground"
                  }`}
                >
                  {f === "all" && (scraperT.libFilterAll || "全部")}
                  {f === "missing" && (scraperT.libFilterMissing || "缺失")}
                  {f === "with" && (scraperT.libFilterWith || "已有")}
                </button>
              ))}

              <div className="h-3 w-px bg-border/40 mx-0.5" />

              {(["comic", "novel"] as string[]).map((ct) => (
                <button
                  key={ct}
                  onClick={() => setLibraryContentType(ct)}
                  className={`rounded-lg px-2 py-1 text-[11px] font-medium transition-all ${
                    libraryContentType === ct
                      ? "bg-purple-500 text-white"
                      : "bg-card-hover text-muted hover:text-foreground"
                  }`}
                >
                  {ct === "comic" && (scraperT.libTypeComic || "漫画")}
                  {ct === "novel" && (scraperT.libTypeNovel || "小说")}
                </button>
              ))}

              <div className="h-3 w-px bg-border/40 mx-0.5" />

              {/* 排序 */}
              {(([
                ["title", scraperT.sortByTitle || "名称"],
                ["fileSize", scraperT.sortByFileSize || "大小"],
                ["updatedAt", scraperT.sortByUpdatedAt || "更新时间"],
                ["metaStatus", scraperT.sortByMetaStatus || "刮削状态"],
              ] as [LibrarySortBy, string][]).map(([field, label]) => {
                const isActive = librarySortBy === field;
                return (
                  <button
                    key={field}
                    onClick={() => setLibrarySort(field)}
                    className={`flex items-center gap-0.5 rounded-lg px-2 py-1 text-[11px] font-medium transition-all ${
                      isActive
                        ? "bg-sky-500 text-white"
                        : "bg-card-hover text-muted hover:text-foreground"
                    }`}
                    title={`${scraperT.sortBy || "排序"}: ${label}`}
                  >
                    {label}
                    {isActive && (
                      librarySortOrder === "asc"
                        ? <ArrowUp className="h-3 w-3 ml-0.5" />
                        : <ArrowDown className="h-3 w-3 ml-0.5" />
                    )}
                    {!isActive && <ArrowUpDown className="h-2.5 w-2.5 ml-0.5 opacity-40" />}
                  </button>
                );
              }))}
            </div>
            )}

            {/* 多选操作栏 */}
            {isAdmin && viewMode === "list" && (
              <div data-guide="select-bar" className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => (selectedIds.size === libraryItems.length && libraryItems.length > 0 ? deselectAll() : selectAllVisible())}
                    className="flex items-center gap-1 rounded-lg bg-card-hover px-2 py-1 text-[11px] font-medium text-muted transition-colors hover:text-foreground"
                  >
                    <CheckSquare className="h-3 w-3" />
                    {selectedIds.size > 0 ? (scraperT.libDeselectAll || "取消") : (scraperT.libSelectAll || "全选")}
                  </button>
                  {selectedIds.size > 0 && (
                    <span className="text-[11px] text-accent font-medium">
                      {selectedIds.size} {scraperT.libItems || "项"}
                    </span>
                  )}
                </div>

                {selectedIds.size > 0 && (
                  <div className="flex items-center gap-1.5">
                    <button
                      onClick={enterBatchEditMode}
                      disabled={batchRunning || batchEditMode}
                      className="flex items-center gap-1 rounded-lg bg-purple-500/10 px-2 py-1 text-[11px] font-medium text-purple-400 transition-all disabled:opacity-50 hover:bg-purple-500/20"
                    >
                      <Pencil className="h-3 w-3" />
                      {scraperT.batchEditBtn || "批量命名"}
                    </button>
                    <button
                      onClick={startBatchSelected}
                      disabled={batchRunning}
                      className={`flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-white transition-all disabled:opacity-50 ${
                        batchMode === "ai"
                          ? "bg-gradient-to-r from-violet-500 to-purple-600"
                          : "bg-accent hover:bg-accent-hover"
                      }`}
                    >
                      <Play className="h-3 w-3" />
                      {scraperT.libScrapeSelected || "刮削"}
                    </button>
                    <button
                      onClick={clearSelectedMetadata}
                      className="flex items-center gap-1 rounded-lg bg-red-500/10 px-2 py-1 text-[11px] font-medium text-red-400 hover:bg-red-500/20 transition-colors"
                    >
                      <Trash2 className="h-3 w-3" />
                      {scraperT.libClearMeta || "清除"}
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* 书库列表 / 文件夹树 / 系列列表 */}
          {viewMode === "folder" ? (
            /* ── 文件夹树形视图 ── */
            <div className="flex-1 overflow-y-auto min-h-0 p-3">
              {folderTreeLoading ? (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-6 w-6 animate-spin text-accent" />
                </div>
              ) : !folderTree || folderTree.length === 0 ? (
                <div className="py-12 text-center text-sm text-muted">暂无文件夹层级数据</div>
              ) : (
                <div className="space-y-0.5">
                  {filterMetadataFolderTree(folderTree, folderSearch).map((node) => (
                    <MetadataFolderTreeItem
                      key={node.path}
                      node={node}
                      depth={0}
                      selectedPath={selectedFolderPath}
                      onSelect={setSelectedFolderPath}
                      searchTerm={folderSearch}
                    />
                  ))}
                </div>
              )}
            </div>
          ) : (<>
          {/* 书库列表 */}
          <div ref={listRef} data-guide="book-list" className="flex-1 overflow-y-auto min-h-0">
            {libraryLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-6 w-6 animate-spin text-accent" />
              </div>
            ) : libraryItems.length === 0 ? (
              <div className="py-12 text-center text-sm text-muted">{scraperT.libEmpty || "没有找到匹配的内容"}</div>
            ) : (
              <div className="divide-y divide-border/10">
                {libraryItems.map((item) => {
                  const isSelected = selectedIds.has(item.id);
                  const isFocused = focusedItemId === item.id;
                  return (
                    <div
                      key={item.id}
                      className={`flex items-center gap-2.5 px-3 sm:px-4 py-2.5 transition-colors cursor-pointer ${
                        isFocused
                          ? "bg-accent/10 border-l-2 border-l-accent"
                          : isSelected
                            ? "bg-accent/5"
                            : "hover:bg-card-hover/30"
                      } ${!isFocused ? "border-l-2 border-l-transparent" : ""}`}
                      onClick={() => setFocusedItem(isFocused ? null : item.id)}
                    >
                      {/* 多选框 */}
                      {isAdmin && (
                        <div
                          onClick={(e) => {
                            e.stopPropagation();
                            toggleSelectItem(item.id);
                          }}
                          className={`flex h-4.5 w-4.5 flex-shrink-0 items-center justify-center rounded border-[1.5px] transition-all cursor-pointer ${
                            isSelected ? "border-accent bg-accent" : "border-muted/40 hover:border-muted/60"
                          }`}
                        >
                          {isSelected && (
                            <svg className="h-2.5 w-2.5 text-white" viewBox="0 0 12 12" fill="none">
                              <path d="M2 6l3 3 5-5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                            </svg>
                          )}
                        </div>
                      )}

                      {/* 封面 */}
                      <div className="relative h-11 w-8 flex-shrink-0 overflow-hidden rounded-lg border border-border/30 bg-muted/10">
                        <Image
                          src={item.coverUrl || (
                            item.entityType === "work"
                              ? apiPath(`/api/opds/work-cover/${item.id}`)
                              : apiPath(`/api/comics/${item.id}/thumbnail`)
                          )}
                          alt=""
                          fill
                          className="object-cover"
                          sizes="32px"
                          unoptimized
                        />
                      </div>

                      {/* 信息 */}
                      <div className="flex-1 min-w-0">
                        {batchEditMode && batchEditNames.has(item.id) ? (
                          /* 批量编辑模式 - 内联输入框 */
                          <input
                            type="text"
                            value={batchEditNames.get(item.id)!.newTitle}
                            onChange={(e) => {
                              e.stopPropagation();
                              setBatchEditName(item.id, e.target.value);
                            }}
                            onClick={(e) => e.stopPropagation()}
                            disabled={batchEditSaving}
                            className={`w-full rounded-md px-1.5 py-0.5 text-[13px] font-medium text-foreground outline-none border transition-all disabled:opacity-50 ${
                              batchEditNames.get(item.id)!.newTitle.trim() !== batchEditNames.get(item.id)!.oldTitle
                                ? "bg-accent/5 border-accent/40 focus:border-accent"
                                : "bg-transparent border-transparent hover:border-border/40 focus:border-border/60 focus:bg-card-hover/30"
                            }`}
                          />
                        ) : (
                        <div className="text-[13px] font-medium text-foreground leading-tight overflow-x-auto whitespace-nowrap scrollbar-hide" style={{ scrollbarWidth: "none", msOverflowStyle: "none" }} title={item.title}>{item.title}</div>
                        )}
                        <div className="flex items-center gap-1.5 mt-0.5">
                          {item.author && (
                            <span className="text-[10px] text-muted/70 truncate max-w-[120px]">{item.author}</span>
                          )}
                        </div>
                      </div>

                      {/* 状态标识 */}
                      <div className="flex items-center gap-1.5 flex-shrink-0">
                        {item.contentType === "novel" ? (
                          <BookOpen className="h-3 w-3 text-blue-400" />
                        ) : (
                          <FileText className="h-3 w-3 text-orange-400" />
                        )}
                        {item.hasMetadata ? (
                          <CheckCircle className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <AlertCircle className="h-3.5 w-3.5 text-amber-500" />
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* 分页 — 固定在左侧面板底部 */}
          {libraryTotalPages >= 1 && (
            <div className="flex flex-col sm:flex-row items-center justify-between gap-2 border-t border-border/20 px-3 sm:px-4 py-2.5 flex-shrink-0">
              {/* 左侧: 总数 + 每页条数 */}
              <div className="flex items-center gap-3">
                <span className="text-[11px] text-muted whitespace-nowrap">
                  {scraperT.libTotalItems || "共"} {libraryTotal} {scraperT.libItems || "项"}
                </span>
                <div className="flex items-center gap-1">
                  <span className="text-[11px] text-muted whitespace-nowrap">{scraperT.paginationPerPage || "每页"}</span>
                  <select
                    value={libraryPageSize}
                    onChange={(e) => setLibraryPageSize(Number(e.target.value))}
                    className="rounded-md border border-border/40 bg-card-hover/50 px-1.5 py-0.5 text-[11px] text-foreground outline-none focus:border-accent/50 transition-colors cursor-pointer"
                  >
                    {[20, 50, 100].map((size) => (
                      <option key={size} value={size}>{size}</option>
                    ))}
                  </select>
                  <span className="text-[11px] text-muted whitespace-nowrap">{scraperT.paginationUnit || "条"}</span>
                </div>
              </div>

              {/* 右侧: 页码导航 + 跳转 */}
              <div className="flex items-center gap-1">
                {/* 首页 */}
                <button
                  disabled={libraryPage <= 1}
                  onClick={() => setLibraryPage(1)}
                  className="flex h-7 items-center justify-center rounded-lg px-1.5 text-[11px] text-muted hover:bg-card-hover hover:text-foreground disabled:opacity-30 transition-colors"
                  title={scraperT.paginationFirst || "首页"}
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  <ChevronLeft className="h-3.5 w-3.5 -ml-2" />
                </button>
                {/* 上一页 */}
                <button
                  disabled={libraryPage <= 1}
                  onClick={() => setLibraryPage(libraryPage - 1)}
                  className="flex h-7 w-7 items-center justify-center rounded-lg text-muted hover:bg-card-hover hover:text-foreground disabled:opacity-30 transition-colors"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                </button>

                {/* 页码按钮 */}
                {(() => {
                  const pages: (number | string)[] = [];
                  const total = libraryTotalPages;
                  const current = libraryPage;

                  if (total <= 7) {
                    for (let i = 1; i <= total; i++) pages.push(i);
                  } else {
                    pages.push(1);
                    if (current > 3) pages.push("...");
                    const start = Math.max(2, current - 1);
                    const end = Math.min(total - 1, current + 1);
                    for (let i = start; i <= end; i++) pages.push(i);
                    if (current < total - 2) pages.push("...");
                    pages.push(total);
                  }

                  return pages.map((p, idx) =>
                    typeof p === "string" ? (
                      <span key={`ellipsis-${idx}`} className="flex h-7 w-5 items-center justify-center text-[11px] text-muted">
                        ···
                      </span>
                    ) : (
                      <button
                        key={p}
                        onClick={() => setLibraryPage(p)}
                        className={`flex h-7 min-w-[28px] items-center justify-center rounded-lg px-1 text-[11px] font-medium transition-all ${
                          p === current
                            ? "bg-accent text-white shadow-sm"
                            : "text-muted hover:bg-card-hover hover:text-foreground"
                        }`}
                      >
                        {p}
                      </button>
                    )
                  );
                })()}

                {/* 下一页 */}
                <button
                  disabled={libraryPage >= libraryTotalPages}
                  onClick={() => setLibraryPage(libraryPage + 1)}
                  className="flex h-7 w-7 items-center justify-center rounded-lg text-muted hover:bg-card-hover hover:text-foreground disabled:opacity-30 transition-colors"
                >
                  <ChevronRight className="h-3.5 w-3.5" />
                </button>
                {/* 末页 */}
                <button
                  disabled={libraryPage >= libraryTotalPages}
                  onClick={() => setLibraryPage(libraryTotalPages)}
                  className="flex h-7 items-center justify-center rounded-lg px-1.5 text-[11px] text-muted hover:bg-card-hover hover:text-foreground disabled:opacity-30 transition-colors"
                  title={scraperT.paginationLast || "末页"}
                >
                  <ChevronRight className="h-3.5 w-3.5" />
                  <ChevronRight className="h-3.5 w-3.5 -ml-2" />
                </button>

                {/* 分隔 */}
                <div className="h-4 w-px bg-border/30 mx-1" />

                {/* 页码跳转 */}
                <div className="flex items-center gap-1">
                  <span className="text-[11px] text-muted whitespace-nowrap">{scraperT.paginationGoto || "跳至"}</span>
                  <input
                    type="number"
                    min={1}
                    max={libraryTotalPages}
                    defaultValue={libraryPage}
                    key={libraryPage}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        const val = parseInt((e.target as HTMLInputElement).value, 10);
                        if (!isNaN(val) && val >= 1 && val <= libraryTotalPages) {
                          setLibraryPage(val);
                        }
                      }
                    }}
                    onBlur={(e) => {
                      const val = parseInt(e.target.value, 10);
                      if (!isNaN(val) && val >= 1 && val <= libraryTotalPages && val !== libraryPage) {
                        setLibraryPage(val);
                      }
                    }}
                    className="w-12 rounded-md border border-border/40 bg-card-hover/50 px-1.5 py-0.5 text-center text-[11px] text-foreground outline-none focus:border-accent/50 transition-colors [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                  />
                  <span className="text-[11px] text-muted whitespace-nowrap">{scraperT.paginationPage || "页"}</span>
                </div>
              </div>
            </div>
          )}
        </>)}
        </div>

        {/* ── 可拖拽分隔条 ── */}
        <div className="hidden md:flex h-full">
          <ResizeDivider
            isDragging={isResizing}
            onMouseDown={handleResizeMouseDown}
            onReset={resetRightPanelWidth}
          />
        </div>

        {/* ── 右侧面板：详情 / 刮削控制 / 进度 / AI聊天 / 帮助 ── */}
        <div data-guide="scrape-panel" className="flex-shrink-0 hidden md:flex flex-col bg-card/20 overflow-hidden" style={{ width: rightPanelWidth }}>
          {helpPanelOpen ? (
            /* ── 帮助面板 ── */
            <HelpPanel
              scraperT={scraperT}
              searchQuery={helpSearchQuery}
              onClose={closeHelpPanel}
            />
          ) : aiChatOpen ? (
            /* ── AI 聊天模式 ── */
            <AIChatPanel
              messages={aiChatMessages}
              loading={aiChatLoading}
              input={aiChatInput}
              scraperT={scraperT}
              onClose={closeAIChat}
            />
          ) : batchEditMode ? (
            /* ── 批量编辑模式 ── */
            <BatchEditPanel
              entries={batchEditNames}
              scraperT={scraperT}
              saving={batchEditSaving}
              results={batchEditResults}
              aiLoading={aiRenameLoading}
              aiConfigured={aiConfigured}
              onExit={exitBatchEditMode}
            />
          ) : focusedItem ? (
            /* ── 详情模式 ── */
            <DetailPanel
              key={`${focusedItem.id}-${detailRefreshKey}`}
              item={focusedItem}
              scraperT={scraperT}
              isAdmin={isAdmin}
              onClose={() => setFocusedItem(null)}
              onRefresh={() => setDetailRefreshKey((k) => k + 1)}
            />
          ) : (
            /* ── 刮削控制 + 进度模式 ── */
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
              {/* 批量操作面板 */}
              {isAdmin && (
                <div className="rounded-xl border border-border/40 bg-card p-4 space-y-4">
                  <div className="flex items-center gap-2">
                    <Sparkles className="h-4 w-4 text-accent" />
                    <h3 className="text-sm font-semibold text-foreground">{scraperT.operationTitle || "批量刮削"}</h3>
                  </div>

                  {/* 模式选择 */}
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      disabled={batchRunning}
                      onClick={() => setBatchMode("standard")}
                      className={`flex items-center gap-2 rounded-lg border p-3 transition-all text-left ${
                        batchMode === "standard"
                          ? "border-accent/50 bg-accent/5 ring-1 ring-accent/20"
                          : "border-border/40 hover:border-border/60"
                      } disabled:opacity-50`}
                    >
                      <Search className="h-4 w-4 text-accent flex-shrink-0" />
                      <div>
                        <div className="text-xs font-medium text-foreground">{scraperT.modeStandard || "标准"}</div>
                        <div className="text-[10px] text-muted mt-0.5">{scraperT.modeStandardShort || "在线源搜索匹配"}</div>
                      </div>
                    </button>
                    <button
                      disabled={batchRunning || !aiConfigured}
                      onClick={() => setBatchMode("ai")}
                      className={`flex items-center gap-2 rounded-lg border p-3 transition-all text-left ${
                        batchMode === "ai"
                          ? "border-purple-500/50 bg-purple-500/5 ring-1 ring-purple-500/20"
                          : "border-border/40 hover:border-border/60"
                      } disabled:opacity-50`}
                      title={!aiConfigured ? (scraperT.aiNotConfiguredHint || "请先在设置中配置AI服务") : undefined}
                    >
                      <Brain className="h-4 w-4 text-purple-500 flex-shrink-0" />
                      <div>
                        <div className="text-xs font-medium text-foreground">{scraperT.modeAI || "AI 智能"}</div>
                        <div className="text-[10px] text-muted mt-0.5">
                          {!aiConfigured
                            ? (scraperT.aiNotConfiguredShort || "需配置AI")
                            : (scraperT.modeAIShort || "AI识别+搜索+补全")}
                        </div>
                      </div>
                    </button>
                  </div>

                  {/* 范围 + 选项 */}
                  <div className="flex items-center gap-2">
                    <button
                      disabled={batchRunning}
                      onClick={() => setScrapeScope("missing")}
                      className={`flex-1 flex items-center justify-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium transition-all ${
                        scrapeScope === "missing" ? "bg-accent text-white" : "bg-card-hover text-muted"
                      } disabled:opacity-50`}
                    >
                      <AlertCircle className="h-3 w-3" />
                      {scraperT.scopeMissing || "仅缺失"}
                    </button>
                    <button
                      disabled={batchRunning}
                      onClick={() => setScrapeScope("all")}
                      className={`flex-1 flex items-center justify-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium transition-all ${
                        scrapeScope === "all" ? "bg-accent text-white" : "bg-card-hover text-muted"
                      } disabled:opacity-50`}
                    >
                      <RefreshCw className="h-3 w-3" />
                      {scraperT.scopeAll || "全部"}
                    </button>
                  </div>

                  {/* 更新书名 toggle */}
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-muted">{scraperT.updateTitleLabel || "同时更新书名"}</span>
                    <button
                      disabled={batchRunning}
                      onClick={() => setUpdateTitle(!updateTitle)}
                      className={`relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors disabled:opacity-50 ${
                        updateTitle ? "bg-accent" : "bg-border"
                      }`}
                    >
                      <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition ${updateTitle ? "translate-x-4" : "translate-x-0"}`} />
                    </button>
                  </div>

                  {/* P2-A: 不替换封面 toggle */}
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-muted">{scraperT.skipCoverLabel || "不替换书籍封面"}</span>
                    <button
                      disabled={batchRunning}
                      onClick={() => setSkipCover(!skipCover)}
                      className={`relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors disabled:opacity-50 ${
                        skipCover ? "bg-accent" : "bg-border"
                      }`}
                    >
                      <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition ${skipCover ? "translate-x-4" : "translate-x-0"}`} />
                    </button>
                  </div>

                  {/* 开始/停止按钮 */}
                  {!batchRunning ? (
                    <button
                      onClick={startBatch}
                      disabled={!stats || stats.total === 0}
                      className={`w-full flex items-center justify-center gap-2 rounded-xl py-2.5 text-sm font-medium text-white transition-all shadow-lg disabled:opacity-50 ${
                        batchMode === "ai"
                          ? "bg-gradient-to-r from-violet-500 to-purple-600 shadow-purple-500/25"
                          : "bg-accent shadow-accent/25"
                      }`}
                    >
                      <Zap className="h-4 w-4" />
                      {scraperT.startBtn || "开始刮削"}
                    </button>
                  ) : (
                    <button
                      onClick={cancelBatch}
                      className="w-full flex items-center justify-center gap-2 rounded-xl bg-red-500 py-2.5 text-sm font-medium text-white shadow-lg shadow-red-500/25"
                    >
                      <Square className="h-4 w-4" />
                      {scraperT.stopBtn || "停止"}
                    </button>
                  )}
                </div>
              )}

              {/* 实时进度 */}
              {(batchRunning || batchDone) && (
                <div className="rounded-xl border border-border/40 bg-card p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      {batchRunning ? (
                        <Loader2 className="h-4 w-4 animate-spin text-accent" />
                      ) : (
                        <CheckCircle className="h-4 w-4 text-emerald-500" />
                      )}
                      <h3 className="text-sm font-semibold text-foreground">
                        {batchRunning ? (scraperT.progressTitle || "进度") : (scraperT.progressDone || "完成")}
                      </h3>
                    </div>
                    {currentProgress && batchRunning && (
                      <span className="text-xs text-muted">{currentProgress.current}/{currentProgress.total}</span>
                    )}
                  </div>

                  {/* 进度条 */}
                  {batchRunning && currentProgress && (
                    <div className="space-y-1.5">
                      <div className="h-2 rounded-full bg-border/30 overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${
                            batchMode === "ai" ? "bg-gradient-to-r from-violet-500 to-purple-500" : "bg-gradient-to-r from-accent to-emerald-500"
                          }`}
                          style={{ width: `${progressPercent}%` }}
                        />
                      </div>
                      <div className="flex justify-between text-[10px] text-muted">
                        <span>{progressPercent}%</span>
                        <span>{scraperT.progressRemaining || "剩余"} {currentProgress.total - currentProgress.current}</span>
                      </div>
                    </div>
                  )}

                  {/* 当前处理项 */}
                  {batchRunning && currentProgress && (
                    <div className="flex items-center gap-2.5 rounded-lg bg-card-hover/50 p-2.5">
                      <div className="relative h-10 w-7 flex-shrink-0 overflow-hidden rounded border border-border/30 bg-muted/10">
                        <Image
                          src={currentProgress.coverUrl || apiPath(`/api/comics/${currentProgress.comicId}/thumbnail`)}
                          alt=""
                          fill
                          className="object-cover"
                          sizes="28px"
                          unoptimized
                          onError={(event) => {
                            event.currentTarget.src = apiPath("/api/placeholder/56/80");
                          }}
                        />
                      </div>
                      <div className="flex h-7 w-7 items-center justify-center rounded bg-accent/10 flex-shrink-0">
                        {currentProgress.step === "recognize" && <Eye className="h-3.5 w-3.5 text-purple-500 animate-pulse" />}
                        {currentProgress.step === "parse" && <Brain className="h-3.5 w-3.5 text-purple-500 animate-pulse" />}
                        {currentProgress.step === "search" && <Search className="h-3.5 w-3.5 text-accent animate-pulse" />}
                        {currentProgress.step === "apply" && <CheckCircle className="h-3.5 w-3.5 text-emerald-500 animate-pulse" />}
                        {currentProgress.step === "ai-complete" && <Sparkles className="h-3.5 w-3.5 text-purple-500 animate-pulse" />}
                        {!currentProgress.step && <Clock className="h-3.5 w-3.5 text-muted animate-pulse" />}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-xs font-medium text-foreground truncate">
                          {currentProgress.displayTitle || currentProgress.workTitle || currentProgress.title || currentProgress.filename}
                        </div>
                        <div className="text-[10px] text-muted">
                          {currentProgress.step === "recognize" && (scraperT.stepRecognize || "AI 识别漫画内容...")}
                          {currentProgress.step === "parse" && (scraperT.stepParse || "AI 解析文件名...")}
                          {currentProgress.step === "search" && (scraperT.stepSearch || "在线搜索...")}
                          {currentProgress.step === "apply" && (scraperT.stepApply || "应用元数据...")}
                          {currentProgress.step === "ai-complete" && (scraperT.stepAIComplete || "AI 补全...")}
                          {!currentProgress.step && (scraperT.stepProcessing || "处理中...")}
                        </div>
                      </div>
                    </div>
                  )}

                  {/* 完成摘要 */}
                  {batchDone && (
                    <div className="grid grid-cols-3 gap-2">
                      <div className="rounded-lg bg-emerald-500/10 p-2 text-center">
                        <div className="text-base font-bold text-emerald-500">{batchDone.success}</div>
                        <div className="text-[10px] text-muted">{scraperT.resultSuccess || "成功"}</div>
                      </div>
                      <div className="rounded-lg bg-red-500/10 p-2 text-center">
                        <div className="text-base font-bold text-red-500">{batchDone.failed}</div>
                        <div className="text-[10px] text-muted">{scraperT.resultFailed || "失败"}</div>
                      </div>
                      <div className="rounded-lg bg-muted/10 p-2 text-center">
                        <div className="text-base font-bold text-muted">{batchDone.total}</div>
                        <div className="text-[10px] text-muted">{scraperT.resultTotal || "总数"}</div>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* 结果列表 */}
              {completedItems.length > 0 && (
                <div className="rounded-xl border border-border/40 bg-card overflow-hidden">
                  <button
                    onClick={() => setShowResults(!showResults)}
                    className="flex w-full items-center justify-between p-3 hover:bg-card-hover/50 transition-colors"
                  >
                    <div className="flex items-center gap-1.5">
                      <Tag className="h-3.5 w-3.5 text-accent" />
                      <span className="text-xs font-semibold text-foreground">{scraperT.resultListTitle || "结果"}</span>
                      <span className="text-[10px] text-muted">({completedItems.length})</span>
                    </div>
                    {showResults ? <ChevronUp className="h-3.5 w-3.5 text-muted" /> : <ChevronDown className="h-3.5 w-3.5 text-muted" />}
                  </button>

                  {showResults && (
                    <div className="divide-y divide-border/10 max-h-[400px] overflow-y-auto">
                      {completedItems.map((item) => (
                        <div key={item.id} className="flex items-center gap-2 px-3 py-2 hover:bg-card-hover/30 transition-colors">
                          <div className="flex-shrink-0">
                            {item.status === "success" ? (
                              <CheckCircle className="h-3.5 w-3.5 text-emerald-500" />
                            ) : item.status === "skipped" ? (
                              <AlertCircle className="h-3.5 w-3.5 text-amber-500" />
                            ) : item.status === "warning" ? (
                              <AlertCircle className="h-3.5 w-3.5 text-orange-500" />
                            ) : (
                              <XCircle className="h-3.5 w-3.5 text-red-500" />
                            )}
                          </div>
                          <div className="relative h-8 w-6 flex-shrink-0 overflow-hidden rounded border border-border/30 bg-muted/10">
                            <Image
                              src={item.coverUrl || apiPath(`/api/comics/${item.comicId}/thumbnail`)}
                              alt=""
                              fill
                              className="object-cover"
                              sizes="24px"
                              unoptimized
                              onError={(event) => {
                                event.currentTarget.src = apiPath("/api/placeholder/48/68");
                              }}
                            />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="text-xs font-medium text-foreground truncate">
                              {item.displayTitle || item.workTitle || item.title || item.matchTitle || item.filename}
                            </div>
                            <div className="flex items-center gap-1 mt-0.5">
                              {item.source && (
                                <span className="rounded bg-accent/10 px-1 py-0.5 text-[9px] text-accent">{item.source}</span>
                              )}
                              {item.message && <span className="text-[9px] text-muted truncate">{item.message}</span>}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* 空状态提示 */}
              {!batchRunning && !batchDone && completedItems.length === 0 && (
                <div className="rounded-xl border border-dashed border-border/40 bg-card/20 p-6 text-center space-y-2">
                  <div className="flex justify-center">
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500/10 to-purple-500/10">
                      <Eye className="h-6 w-6 text-purple-400" />
                    </div>
                  </div>
                  <h4 className="text-sm font-semibold text-foreground">{scraperT.rightPanelHint || "点击左侧书籍查看详情"}</h4>
                  <p className="text-xs text-muted leading-relaxed">
                    {scraperT.rightPanelDesc || "选择一本书查看元数据详情并进行精准刮削，或使用上方批量操作对全库/选中项统一刮削"}
                  </p>
                </div>
              )}

              {/* 文件夹刮削面板（文件夹模式下选中文件夹时显示） */}
              {viewMode === "folder" && selectedFolderPath && (
                <FolderScrapePanel
                  folderPath={selectedFolderPath}
                  folderTree={folderTree}
                  scrapeRunning={folderScrapeRunning}
                  scrapeProgress={folderScrapeProgress}
                  scrapeDone={folderScrapeDone}
                  batchMode={batchMode}
                  scraperT={scraperT}
                />
              )}
            </div>
          )}
        </div>
      </div>

      {/* ── 移动端批量编辑浮层 ── */}
      {batchEditMode && (
        <div className="fixed inset-0 z-50 md:hidden bg-background">
          <BatchEditPanel
            entries={batchEditNames}
            scraperT={scraperT}
            saving={batchEditSaving}
            results={batchEditResults}
            aiLoading={aiRenameLoading}
            aiConfigured={aiConfigured}
            onExit={exitBatchEditMode}
          />
        </div>
      )}

      {/* ── 移动端详情浮层 ── */}
      {focusedItem && (
        <div className="fixed inset-0 z-50 md:hidden bg-background">
          <DetailPanel
            key={`mobile-${focusedItem.id}-${detailRefreshKey}`}
            item={focusedItem}
            scraperT={scraperT}
            isAdmin={isAdmin}
            onClose={() => setFocusedItem(null)}
            onRefresh={() => setDetailRefreshKey((k) => k + 1)}
          />
        </div>
      )}

      {/* ── 移动端 AI 聊天浮层 ── */}
      {aiChatOpen && (
        <div className="fixed inset-0 z-50 md:hidden bg-background">
          <AIChatPanel
            messages={aiChatMessages}
            loading={aiChatLoading}
            input={aiChatInput}
            scraperT={scraperT}
            onClose={closeAIChat}
          />
        </div>
      )}

      {/* ── 悬浮 AI 助手按钮 ── */}
      {isAdmin && aiConfigured && !aiChatOpen && (
        <button
          onClick={openAIChat}
          data-guide="ai-chat-btn"
          className="fixed bottom-6 right-6 z-40 flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-500 to-purple-600 text-white shadow-xl shadow-purple-500/30 transition-all hover:shadow-2xl hover:shadow-purple-500/40 hover:scale-105 active:scale-95 md:hidden"
          title={scraperT.aiChatBtnLabel || "AI 助手"}
        >
          <MessageCircle className="h-5 w-5" />
        </button>
      )}

      {/* ── 桌面端悬浮 AI 助手按钮（当右侧面板不是AI聊天时显示） ── */}
      {isAdmin && aiConfigured && !aiChatOpen && (
        <button
          onClick={openAIChat}
          data-guide="ai-chat-btn"
          className="fixed bottom-6 right-6 z-40 hidden md:flex h-11 items-center gap-2 rounded-2xl bg-gradient-to-r from-violet-500 to-purple-600 px-4 text-white shadow-xl shadow-purple-500/30 transition-all hover:shadow-2xl hover:shadow-purple-500/40 hover:scale-[1.02] active:scale-[0.98]"
          title={scraperT.aiChatBtnLabel || "AI 助手"}
        >
          <Bot className="h-4 w-4" />
          <span className="text-xs font-medium">{scraperT.aiChatBtnLabel || "AI 助手"}</span>
        </button>
      )}

      {/* ── 帮助按钮（桌面端左下角） ── */}
      {isAdmin && !helpPanelOpen && (
        <button
          onClick={openHelpPanel}
          className="fixed bottom-6 left-6 z-40 hidden md:flex h-9 items-center gap-1.5 rounded-xl bg-card border border-border/50 px-3 text-muted shadow-lg transition-all hover:text-foreground hover:border-emerald-500/40 hover:shadow-xl"
          title={scraperT.helpTitle || "帮助中心"}
        >
          <CircleHelp className="h-3.5 w-3.5" />
          <span className="text-[11px] font-medium">{scraperT.helpTitle || "帮助"}</span>
        </button>
      )}

      {/* ── 移动端帮助浮层 ── */}
      {helpPanelOpen && (
        <div className="fixed inset-0 z-50 md:hidden bg-background">
          <HelpPanel
            scraperT={scraperT}
            searchQuery={helpSearchQuery}
            onClose={closeHelpPanel}
          />
        </div>
      )}

      {/* ── 引导遮罩 ── */}
      {guideActive && (
        <GuideOverlay
          scraperT={scraperT}
          currentStep={guideCurrentStep}
        />
      )}
    </div>
  );
}
