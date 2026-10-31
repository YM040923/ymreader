"use client";

import { FolderOpen, ChevronRight } from "lucide-react";
import { apiPath } from "@/lib/base-path";
import type { LibraryDirectoryNode } from "@/api/libraries";

interface DirectoryCardProps {
  node: LibraryDirectoryNode;
  viewMode?: "grid" | "list";
  onOpen: (node: LibraryDirectoryNode) => void;
}

function folderCover(node: LibraryDirectoryNode): string | null {
  if (node.coverUrl) return apiPath(node.coverUrl);
  if (node.coverComicId) {
    return apiPath(`/api/comics/${encodeURIComponent(node.coverComicId)}/thumbnail`);
  }
  return null;
}

export default function DirectoryCard({ node, viewMode = "grid", onOpen }: DirectoryCardProps) {
  const cover = folderCover(node);

  if (viewMode === "list") {
    return (
      <button
        type="button"
        onClick={() => onOpen(node)}
        className="group flex min-w-0 w-full items-center gap-4 rounded-2xl border border-border/50 bg-card/65 p-3 text-left transition-all hover:-translate-y-0.5 hover:border-accent/25 hover:bg-card-hover hover:shadow-lg hover:shadow-accent/5"
      >
        <div className="relative flex h-24 w-16 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-background">
          {cover ? (
            <img src={cover} alt="" className="h-full w-full object-cover opacity-80" loading="lazy" />
          ) : null}
          <FolderOpen className="absolute h-6 w-6 text-accent" />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold text-foreground transition-colors group-hover:text-accent">
            {node.name}
          </h2>
          <p className="mt-1 truncate text-xs text-muted">{node.workCount} 部作品</p>
        </div>
        <ChevronRight className="h-5 w-5 shrink-0 text-muted transition-colors group-hover:text-accent" />
      </button>
    );
  }

  return (
    <button type="button" onClick={() => onOpen(node)} className="group block min-w-0 w-full text-left">
      <article className="overflow-hidden rounded-2xl border border-border/50 bg-card/65 transition-all duration-300 hover:-translate-y-1 hover:border-accent/25 hover:shadow-xl hover:shadow-accent/10">
        <div className="relative w-full overflow-hidden bg-gradient-to-br from-background to-card" style={{ aspectRatio: "3 / 4" }}>
          {cover ? (
            <img
              src={cover}
              alt=""
              className="h-full w-full object-cover opacity-75 blur-[1px] transition-transform duration-500 group-hover:scale-[1.025] group-hover:opacity-85"
              loading="lazy"
            />
          ) : null}
          <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-black/10 to-transparent" />
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-black/35 text-white/90 backdrop-blur-sm transition-colors group-hover:bg-accent/80">
              <FolderOpen className="h-7 w-7" />
            </div>
          </div>
          <span className="absolute bottom-2 right-2 rounded-full bg-black/65 px-2 py-1 text-[10px] font-medium text-white/90 backdrop-blur">
            {node.workCount} 部
          </span>
        </div>
        <div className="p-3">
          <h2
            className="line-clamp-2 min-h-10 text-sm font-semibold leading-5 text-foreground transition-colors group-hover:text-accent"
            title={node.name}
          >
            {node.name}
          </h2>
          <p className="mt-1 truncate text-[11px] text-muted">
            {node.hasChildren ? "文件夹" : "作品目录"}
          </p>
        </div>
      </article>
    </button>
  );
}
