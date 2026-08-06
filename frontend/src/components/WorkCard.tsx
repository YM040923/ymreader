"use client";

import Link from "next/link";
import { BookOpen, Clock3, FileStack } from "lucide-react";
import { apiPath } from "@/lib/base-path";
import { buildWorkDetailPath, getWorkCoverAspectRatio } from "@/lib/work-model";
import type { Work } from "@/types/work";

interface WorkCardProps {
  work: Work;
  viewMode?: "grid" | "list";
}

function coverSource(work: Work): string {
  return work.coverUrl
    || (work.coverComicId
      ? apiPath(`/api/comics/${encodeURIComponent(work.coverComicId)}/thumbnail`)
      : apiPath("/api/placeholder/320/448"));
}

function workSubtitle(work: Work): string {
  if (work.itemCount <= 1) return `${work.pageCount || 0} 页`;
  return `${work.itemCount} 个章节 · ${work.pageCount || 0} 页`;
}

export default function WorkCard({
  work,
  viewMode = "grid",
}: WorkCardProps) {
  const href = buildWorkDetailPath(work.id);
  const cover = coverSource(work);

  if (viewMode === "list") {
    return (
      <Link
        href={href}
        className="group flex min-w-0 items-center gap-4 rounded-2xl border border-border/50 bg-card/65 p-3 transition-all hover:-translate-y-0.5 hover:border-accent/25 hover:bg-card-hover hover:shadow-lg hover:shadow-accent/5"
      >
        <div className="relative h-24 w-16 shrink-0 overflow-hidden rounded-xl bg-background">
          <img
            src={cover}
            alt={work.title}
            className="h-full w-full object-contain"
            loading="lazy"
          />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold text-foreground transition-colors group-hover:text-accent">
            {work.title}
          </h2>
          <p className="mt-1 truncate text-xs text-muted">{work.rootPath}</p>
          <div className="mt-2 flex flex-wrap items-center gap-3 text-[11px] text-muted/80">
            <span className="inline-flex items-center gap-1">
              <FileStack className="h-3.5 w-3.5" />
              {workSubtitle(work)}
            </span>
            {work.lastReadAt && (
              <span className="inline-flex items-center gap-1 text-accent/80">
                <Clock3 className="h-3.5 w-3.5" />
                有阅读记录
              </span>
            )}
          </div>
        </div>
        <BookOpen className="h-5 w-5 shrink-0 text-muted transition-colors group-hover:text-accent" />
      </Link>
    );
  }

  return (
    <Link href={href} className="group block min-w-0">
      <article className="overflow-hidden rounded-2xl border border-border/50 bg-card/65 transition-all duration-300 hover:-translate-y-1 hover:border-accent/25 hover:shadow-xl hover:shadow-accent/10">
        <div
          className="relative w-full overflow-hidden bg-gradient-to-br from-background to-card"
          style={{ aspectRatio: getWorkCoverAspectRatio(work) }}
        >
          <img
            src={cover}
            alt={work.title}
            className="h-full w-full object-contain transition-transform duration-500 group-hover:scale-[1.025]"
            loading="lazy"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/55 via-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
          <div className="absolute inset-0 flex items-center justify-center opacity-0 transition-opacity duration-300 group-hover:opacity-100">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-accent/90 text-white shadow-lg shadow-accent/30">
              <BookOpen className="h-5 w-5" />
            </div>
          </div>
          <span className="absolute bottom-2 right-2 rounded-full bg-black/65 px-2 py-1 text-[10px] font-medium text-white/90 backdrop-blur">
            {work.itemCount > 1 ? `${work.itemCount} 话/卷` : "单本"}
          </span>
        </div>
        <div className="p-3">
          <h2
            className="line-clamp-2 min-h-10 text-sm font-semibold leading-5 text-foreground transition-colors group-hover:text-accent"
            title={work.title}
          >
            {work.title}
          </h2>
          <p className="mt-1 truncate text-[11px] text-muted">
            {workSubtitle(work)}
          </p>
        </div>
      </article>
    </Link>
  );
}
