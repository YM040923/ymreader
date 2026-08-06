/**
 * 文件夹刮削控制面板组件
 */
"use client";

import {
  CheckCircle,
  AlertCircle,
  Play,
  Square,
  RefreshCw,
  FolderOpen,
} from "lucide-react";
import type { MetadataFolderNode } from "@/lib/scraper-store";
import { startFolderScrape, cancelFolderScrape } from "@/lib/scraper-store";

export function FolderScrapePanel({
  folderPath,
  folderTree,
  scrapeRunning,
  scrapeProgress,
  scrapeDone,
  batchMode,
  scraperT,
}: {
  folderPath: string;
  folderTree: MetadataFolderNode[] | null;
  scrapeRunning: boolean;
  scrapeProgress: { current: number; total: number; status: string; filename: string } | null;
  scrapeDone: { total: number; success: number; failed: number } | null;
  batchMode: string;
  scraperT: Record<string, string>;
}) {
  // 查找选中的文件夹节点
  const findNode = (nodes: MetadataFolderNode[], path: string): MetadataFolderNode | null => {
    for (const n of nodes) {
      if (n.path === path) return n;
      if (n.children) {
        const found = findNode(n.children, path);
        if (found) return found;
      }
    }
    return null;
  };

  const selectedNode = folderTree ? findNode(folderTree, folderPath) : null;
  if (!selectedNode) return null;

  const metaPercent = selectedNode.fileCount > 0
    ? Math.round((selectedNode.withMeta / selectedNode.fileCount) * 100)
    : 0;

  return (
    <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-4 space-y-4">
      {/* 文件夹信息 */}
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/10 flex-shrink-0">
          <FolderOpen className="h-5 w-5 text-amber-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold text-foreground truncate">{selectedNode.name}</div>
          <div className="text-[10px] text-muted truncate">{folderPath}</div>
        </div>
      </div>

      {/* 统计信息 */}
      <div className="grid grid-cols-3 gap-2">
        <div className="rounded-lg bg-card/50 p-2 text-center">
          <div className="text-lg font-bold text-foreground">{selectedNode.fileCount}</div>
          <div className="text-[10px] text-muted">总文件</div>
        </div>
        <div className="rounded-lg bg-emerald-500/10 p-2 text-center">
          <div className="text-lg font-bold text-emerald-500">{selectedNode.withMeta}</div>
          <div className="text-[10px] text-muted">已刮削</div>
        </div>
        <div className="rounded-lg bg-amber-500/10 p-2 text-center">
          <div className="text-lg font-bold text-amber-500">{selectedNode.missingMeta}</div>
          <div className="text-[10px] text-muted">缺失</div>
        </div>
      </div>

      {/* 元数据完成度进度条 */}
      <div>
        <div className="flex items-center justify-between mb-1">
          <span className="text-[11px] text-muted">元数据完成度</span>
          <span className={`text-[11px] font-medium ${metaPercent === 100 ? "text-emerald-500" : "text-accent"}`}>
            {metaPercent}%
          </span>
        </div>
        <div className="h-2 rounded-full bg-border/30 overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-500 ${
              metaPercent === 100 ? "bg-emerald-500" : "bg-gradient-to-r from-accent to-emerald-500"
            }`}
            style={{ width: `${metaPercent}%` }}
          />
        </div>
      </div>

      {/* 文件列表预览 */}
      {selectedNode.files && selectedNode.files.length > 0 && (
        <div className="space-y-1 max-h-48 overflow-y-auto">
          <div className="text-[11px] text-muted font-medium mb-1">文件列表</div>
          {selectedNode.files.map((file) => (
            <div key={file.id} className="flex items-center gap-2 rounded-lg px-2 py-1 text-[11px] hover:bg-white/5 transition-colors">
              {file.hasMetadata ? (
                <CheckCircle className="h-3 w-3 shrink-0 text-emerald-500" />
              ) : (
                <AlertCircle className="h-3 w-3 shrink-0 text-amber-500" />
              )}
              <span className="flex-1 truncate text-muted">{file.title}</span>
              {file.metadataSource && (
                <span className="text-[9px] text-muted/50 shrink-0">{file.metadataSource}</span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* 刮削操作按钮 */}
      {!scrapeRunning ? (
        <div className="space-y-2">
          <button
            onClick={() => startFolderScrape(folderPath, "missing")}
            disabled={selectedNode.missingMeta === 0}
            className="w-full flex items-center justify-center gap-2 rounded-lg bg-accent hover:bg-accent-hover text-white py-2 text-xs font-medium transition-all disabled:opacity-50"
          >
            <Play className="h-3.5 w-3.5" />
            刮削缺失项 ({selectedNode.missingMeta})
          </button>
          <button
            onClick={() => startFolderScrape(folderPath, "all")}
            className="w-full flex items-center justify-center gap-2 rounded-lg border border-border/40 bg-card hover:bg-card-hover text-foreground py-2 text-xs font-medium transition-all"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            全部重新刮削 ({selectedNode.fileCount})
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {/* 刮削进度 */}
          {scrapeProgress && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-[11px] text-muted">
                  {scrapeProgress.status === "processing" ? "处理中..." : `${scrapeProgress.current}/${scrapeProgress.total}`}
                </span>
                <span className="text-[11px] text-accent font-medium">
                  {scrapeProgress.total > 0 ? Math.round((scrapeProgress.current / scrapeProgress.total) * 100) : 0}%
                </span>
              </div>
              <div className="h-1.5 rounded-full bg-border/30 overflow-hidden">
                <div
                  className="h-full rounded-full bg-accent transition-all"
                  style={{ width: `${scrapeProgress.total > 0 ? (scrapeProgress.current / scrapeProgress.total) * 100 : 0}%` }}
                />
              </div>
              <div className="text-[10px] text-muted mt-1 truncate">{scrapeProgress.filename}</div>
            </div>
          )}
          <button
            onClick={cancelFolderScrape}
            className="w-full flex items-center justify-center gap-2 rounded-lg bg-red-500/10 text-red-400 py-2 text-xs font-medium transition-all hover:bg-red-500/20"
          >
            <Square className="h-3.5 w-3.5" />
            停止刮削
          </button>
        </div>
      )}

      {/* 刮削完成结果 */}
      {scrapeDone && !scrapeRunning && (
        <div className="rounded-lg bg-card/50 p-3 space-y-1">
          <div className="text-xs font-medium text-foreground">刮削完成</div>
          <div className="flex items-center gap-3 text-[11px]">
            <span className="text-muted">总计: <span className="text-foreground font-medium">{scrapeDone.total}</span></span>
            <span className="text-emerald-500">成功: {scrapeDone.success}</span>
            <span className="text-red-400">失败: {scrapeDone.failed}</span>
          </div>
        </div>
      )}
    </div>
  );
}
