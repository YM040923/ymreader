/**
 * 全局刮削状态管理 (模块级单例)
 *
 * 此文件现在作为 barrel 文件，统一 re-export 所有拆分后的模块。
 * 所有类型、状态、actions 已拆分到 stores/ 目录下的独立文件中：
 *
 * - stores/scraper-types.ts    — 所有类型定义
 * - stores/scraper-core.ts     — 核心 state + subscribe/notify 机制
 * - stores/scraper-batch-actions.ts — 刮削批处理 actions
 * - stores/library-actions.ts  — 书库管理 actions
 * - stores/ai-chat-actions.ts  — AI 聊天 actions
 * - stores/guide-actions.ts    — 引导教程 actions
 * - stores/folder-actions.ts   — 文件夹模式 actions
 *
 * SSE 连接和进度状态保存在模块作用域中，
 * 页面组件卸载/重新挂载不会丢失正在进行的刮削进度。
 */

/* ── 类型 re-export ── */
export type {
  ProgressItem,
  CompletedItem,
  BatchDone,
  MetadataStats,
  BatchMode,
  ScrapeScope,
  MetaFilter,
  ActiveTab,
  LibraryItemTag,
  LibraryItemCategory,
  LibraryItem,
  LibrarySortBy,
  BatchEditNameEntry,
  BatchRenameResult,
  AIChatMessage,
  AIChatQuickCommand,
  GuideStep,
  ViewMode,
  MetadataFolderFile,
  MetadataFolderNode,
  ScraperState,
} from "./stores/scraper-types";

/* ── 常量 re-export ── */

/* ── 核心 state + 订阅机制 re-export ── */
export { subscribe, getSnapshot } from "./stores/scraper-core";

/* ── 刮削批处理 actions re-export ── */
export {
  setBatchMode,
  setScrapeScope,
  setShowResults,
  setUpdateTitle,
  setSkipCover,
  loadStats,
  startBatch,
  cancelBatch,
} from "./stores/scraper-batch-actions";

/* ── 书库管理 actions re-export ── */
export {
  setActiveTab,
  setLibrarySearch,
  setLibraryMetaFilter,
  setLibraryContentType,
  setLibraryPage,
  setLibraryPageSize,
  setLibrarySort,
  setFocusedItem,
  toggleSelectItem,
  selectAllVisible,
  deselectAll,
  loadLibrary,
  startBatchSelected,
  clearSelectedMetadata,
  enterBatchEditMode,
  exitBatchEditMode,
  setBatchEditName,
  applyNameToAll,
  undoBatchEditNames,
  saveBatchRename,
  aiRename,
} from "./stores/library-actions";

/* ── AI 聊天 actions re-export ── */
export {
  toggleAIChat,
  openAIChat,
  closeAIChat,
  setAIChatInput,
  clearAIChatMessages,
  abortAIChat,
  sendAIChatMessage,
} from "./stores/ai-chat-actions";

/* ── 引导教程 actions re-export ── */
export {
  GUIDE_STEPS,
  startGuide,
  nextGuideStep,
  prevGuideStep,
  goToGuideStep,
  finishGuide,
  skipGuide,
  resetGuide,
  openHelpPanel,
  closeHelpPanel,
  setHelpSearchQuery,
  checkAutoStartGuide,
} from "./stores/guide-actions";

/* ── 文件夹模式 actions re-export ── */
export {
  setViewMode,
  setSelectedFolderPath,
  setFolderSearch,
  loadFolderTree,
  startFolderScrape,
  cancelFolderScrape,
} from "./stores/folder-actions";
