/**
 * 文件夹树搜索/筛选辅助函数 + 文件夹树节点组件
 */
"use client";

import { useState } from "react";
import {
  ChevronRight,
  FolderOpen,
  Folder,
} from "lucide-react";
import type { MetadataFolderNode } from "@/lib/scraper-store";

/* ── 文件夹树搜索/筛选辅助函数 ── */
export function filterMetadataFolderTree(
  nodes: MetadataFolderNode[],
  search: string
): MetadataFolderNode[] {
  if (!search) return nodes;
  const searchLower = search.toLowerCase();

  function matchNode(node: MetadataFolderNode): MetadataFolderNode | null {
    const nameMatch = node.name.toLowerCase().includes(searchLower);
    const matchedFiles = (node.files || []).filter(
      (f) =>
        f.title.toLowerCase().includes(searchLower) ||
        f.filename.toLowerCase().includes(searchLower)
    );
    const matchedChildren: MetadataFolderNode[] = [];
    for (const child of node.children || []) {
      const matched = matchNode(child);
      if (matched) matchedChildren.push(matched);
    }
    if (matchedChildren.length > 0 || matchedFiles.length > 0 || (nameMatch && node.fileCount > 0)) {
      return {
        ...node,
        children: matchedChildren,
        files: matchedFiles.length > 0 ? matchedFiles : node.files,
      };
    }
    return null;
  }

  return nodes.map(matchNode).filter(Boolean) as MetadataFolderNode[];
}

export function highlightSearchText(text: string, search: string) {
  if (!search) return text;
  const idx = text.toLowerCase().indexOf(search.toLowerCase());
  if (idx === -1) return text;
  return (
    <>
      {text.slice(0, idx)}
      <span className="bg-accent/30 text-accent font-medium rounded px-0.5">
        {text.slice(idx, idx + search.length)}
      </span>
      {text.slice(idx + search.length)}
    </>
  );
}

/* ── 文件夹树节点组件 ── */
export function MetadataFolderTreeItem({
  node,
  depth,
  selectedPath,
  onSelect,
  searchTerm = "",
}: {
  node: MetadataFolderNode;
  depth: number;
  selectedPath: string | null;
  onSelect: (path: string | null) => void;
  searchTerm?: string;
}) {
  const [expanded, setExpanded] = useState(depth < 1 || !!searchTerm);
  const hasChildren = node.children && node.children.length > 0;
  const isSelected = selectedPath === node.path;
  const isExpanded = searchTerm ? true : expanded;

  const metaPercent = node.fileCount > 0 ? Math.round((node.withMeta / node.fileCount) * 100) : 0;

  return (
    <div>
      <div
        className={`flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left transition-colors cursor-pointer ${
          isSelected
            ? "bg-accent/10 border-l-2 border-l-accent"
            : "hover:bg-white/5 border-l-2 border-l-transparent"
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={() => {
          onSelect(isSelected ? null : node.path);
          if (hasChildren) setExpanded(!expanded);
        }}
      >
        {/* 展开/收起箭头 */}
        <span className="flex h-4 w-4 shrink-0 items-center justify-center">
          {hasChildren ? (
            <ChevronRight
              className={`h-3.5 w-3.5 text-muted transition-transform ${
                isExpanded ? "rotate-90" : ""
              }`}
            />
          ) : (
            <span className="h-1 w-1 rounded-full bg-muted/30" />
          )}
        </span>

        {/* 文件夹图标 */}
        {isExpanded && hasChildren ? (
          <FolderOpen className="h-4 w-4 shrink-0 text-amber-400" />
        ) : (
          <Folder className="h-4 w-4 shrink-0 text-amber-400/60" />
        )}

        {/* 文件夹名 */}
        <span className="flex-1 truncate text-xs font-medium text-foreground">
          {highlightSearchText(node.name, searchTerm)}
        </span>

        {/* 元数据完成度 */}
        <div className="flex items-center gap-1.5 shrink-0">
          <div className="h-1.5 w-10 rounded-full bg-border/30 overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${
                metaPercent === 100 ? "bg-emerald-500" : metaPercent > 0 ? "bg-accent/60" : "bg-amber-500/40"
              }`}
              style={{ width: `${metaPercent}%` }}
            />
          </div>
          <span className={`text-[10px] font-medium ${
            metaPercent === 100 ? "text-emerald-500" : metaPercent > 0 ? "text-accent" : "text-amber-500"
          }`}>
            {node.withMeta}/{node.fileCount}
          </span>
        </div>
      </div>

      {/* 子节点 */}
      {isExpanded && hasChildren && (
        <div>
          {node.children.map((child) => (
            <MetadataFolderTreeItem
              key={child.path}
              node={child}
              depth={depth + 1}
              selectedPath={selectedPath}
              onSelect={onSelect}
              searchTerm={searchTerm}
            />
          ))}
        </div>
      )}
    </div>
  );
}
