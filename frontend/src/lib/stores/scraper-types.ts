/**
 * 刮削状态管理 — 类型定义
 *
 * 从 scraper-store.ts 中提取的所有类型、接口和常量定义。
 */

/* ── 基础类型 ── */
export interface ProgressItem {
  type: string;
  current: number;
  total: number;
  comicId: string;
  filename: string;
  displayTitle?: string;
  title?: string;
  workTitle?: string;
  entityType?: "work" | "comic";
  libraryId?: string;
  step?: string;
  status?: string;
  source?: string;
  coverUrl?: string;
  message?: string;
  matchTitle?: string;
  resultsCount?: number;
  parsed?: {
    title?: string;
    author?: string;
    year?: number;
    language?: string;
    genre?: string;
    group?: string;
    tags?: string;
  };
}

export interface CompletedItem extends ProgressItem {
  id: string;
}

export interface BatchDone {
  type: string;
  total: number;
  success: number;
  failed: number;
}

export interface MetadataStats {
  total: number;
  withMetadata: number;
  missing: number;
}

export type BatchMode = "standard" | "ai";
export type ScrapeScope = "missing" | "all";

export type MetaFilter = "all" | "with" | "missing";
export type ActiveTab = "scrape" | "library";

/* ── 书库管理类型 ── */
export interface LibraryItemTag {
  name: string;
  color: string;
}

export interface LibraryItemCategory {
  slug: string;
  name: string;
  icon: string;
}

export interface LibraryItem {
  id: string;
  entityType: "work" | "comic";
  representativeComicId?: string;
  coverUrl?: string;
  itemCount?: number;
  title: string;
  filename: string;
  author: string;
  genre: string;
  description: string;
  year: number | null;
  publisher: string;
  language: string;
  fileSize: number;
  updatedAt: string;
  metadataSource: string;
  hasMetadata: boolean;
  contentType: string;
  tags: LibraryItemTag[];
  rating: number | null;
  isFavorite: boolean;
  categories: LibraryItemCategory[];
}

export type LibrarySortBy = "title" | "fileSize" | "updatedAt" | "metaStatus";

/* ── 批量编辑类型 ── */
export interface BatchEditNameEntry {
  comicId: string;
  filename: string;
  oldTitle: string;
  newTitle: string;
}

export interface BatchRenameResult {
  comicId: string;
  status: "success" | "failed" | "skipped";
  newTitle?: string;
  message?: string;
}

/* ── AI 聊天相关类型 ── */
export interface AIChatMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: number;
  /** 指令执行结果（仅系统消息） */
  commandResult?: {
    action: string;
    success: boolean;
    message: string;
  };
}

export interface AIChatQuickCommand {
  label: string;
  prompt: string;
  icon?: string;
}

/* ── 合集管理相关类型 ── */
export interface GuideStep {
  id: string;
  /** 高亮目标元素的 CSS 选择器 */
  targetSelector: string;
  /** 标题 key (i18n) */
  titleKey: string;
  /** 描述 key (i18n) */
  descKey: string;
  /** 弹窗位置 */
  placement: "top" | "bottom" | "left" | "right";
  /** 操作提示 key (可选) */
  actionKey?: string;
}

/* ── 文件夹模式相关类型 ── */
export type ViewMode = "list" | "folder";

export interface MetadataFolderFile {
  id: string;
  title: string;
  filename: string;
  fileSize: number;
  type: string;
  hasMetadata: boolean;
  metadataSource: string;
  author: string;
}

export interface MetadataFolderNode {
  name: string;
  path: string;
  fileCount: number;
  withMeta: number;
  missingMeta: number;
  comicCount: number;
  novelCount: number;
  totalSize: number;
  children: MetadataFolderNode[];
  files?: MetadataFolderFile[];
}

/* ── 系列模式相关类型 ── */
export interface ScraperState {
  // 统计
  stats: MetadataStats | null;
  statsLoading: boolean;
  // 运行状态
  batchRunning: boolean;
  batchMode: BatchMode;
  scrapeScope: ScrapeScope;
  updateTitle: boolean;
  skipCover: boolean;
  // 进度
  currentProgress: ProgressItem | null;
  batchDone: BatchDone | null;
  completedItems: CompletedItem[];
  showResults: boolean;
  // 书库管理
  activeTab: ActiveTab;
  libraryItems: LibraryItem[];
  libraryLoading: boolean;
  librarySearch: string;
  libraryMetaFilter: MetaFilter;
  libraryContentType: string;
  librarySortBy: LibrarySortBy;
  librarySortOrder: "asc" | "desc";
  libraryPage: number;
  libraryPageSize: number;
  libraryTotalPages: number;
  libraryTotal: number;
  selectedIds: Set<string>;
  // 详情面板
  focusedItemId: string | null;
  // 批量编辑模式
  batchEditMode: boolean;
  batchEditNames: Map<string, BatchEditNameEntry>;
  batchEditSaving: boolean;
  batchEditResults: BatchRenameResult[] | null;
  aiRenameLoading: boolean;
  // AI 聊天面板
  aiChatOpen: boolean;
  aiChatMessages: AIChatMessage[];
  aiChatLoading: boolean;
  aiChatInput: string;
  // 引导教程系统
  guideActive: boolean;
  guideCurrentStep: number;
  guideDismissed: boolean;
  helpPanelOpen: boolean;
  helpSearchQuery: string;
  // 合集管理
  // 文件夹模式
  viewMode: ViewMode;
  folderTree: MetadataFolderNode[] | null;
  folderTreeLoading: boolean;
  selectedFolderPath: string | null;
  folderSearch: string;
  folderScrapeRunning: boolean;
  folderScrapeProgress: { current: number; total: number; status: string; filename: string } | null;
  folderScrapeDone: { total: number; success: number; failed: number } | null;

}
