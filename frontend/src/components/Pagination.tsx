"use client";

import { useState, useMemo } from "react";
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

const PAGE_SIZE_OPTIONS = [20, 50, 100];

export function Pagination({
  currentPage,
  totalPages,
  totalItems,
  pageSize,
  onPageChange,
  onPageSizeChange,
  t,
}: {
  currentPage: number;
  totalPages: number;
  totalItems: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  t: ReturnType<typeof useTranslation>;
}) {
  const [jumpInput, setJumpInput] = useState("");

  // Generate visible page numbers with ellipsis
  const pageNumbers = useMemo(() => {
    const pages: (number | "...")[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1);
      if (currentPage > 3) pages.push("...");
      const start = Math.max(2, currentPage - 1);
      const end = Math.min(totalPages - 1, currentPage + 1);
      for (let i = start; i <= end; i++) pages.push(i);
      if (currentPage < totalPages - 2) pages.push("...");
      pages.push(totalPages);
    }
    return pages;
  }, [currentPage, totalPages]);

  if (totalPages <= 1 && totalItems <= PAGE_SIZE_OPTIONS[0]) return null;

  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-4 pb-2">
      {/* Left: total info + page size selector */}
      <div className="flex items-center gap-3 text-xs text-muted">
        <span>
          {t.tagManager?.total || "共"} <span className="font-medium text-foreground tabular-nums">{totalItems}</span> {t.tagManager?.items || "项"}
        </span>
        <div className="flex items-center gap-1.5">
          <span>{t.tagManager?.perPage || "每页"}</span>
          <select
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            className="rounded-md border border-border/50 bg-card px-1.5 py-0.5 text-xs text-foreground outline-none focus:border-accent/50"
          >
            {PAGE_SIZE_OPTIONS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Right: page navigation */}
      <div className="flex items-center gap-1">
        {/* First page */}
        <button
          onClick={() => onPageChange(1)}
          disabled={currentPage === 1}
          className="flex h-7 w-7 items-center justify-center rounded-lg text-muted transition-colors hover:bg-card hover:text-foreground disabled:opacity-30 disabled:pointer-events-none"
          title={t.home?.firstPage || "首页"}
        >
          <ChevronsLeft className="h-3.5 w-3.5" />
        </button>
        {/* Prev page */}
        <button
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage === 1}
          className="flex h-7 w-7 items-center justify-center rounded-lg text-muted transition-colors hover:bg-card hover:text-foreground disabled:opacity-30 disabled:pointer-events-none"
          title={t.home?.prevPage || "上一页"}
        >
          <ChevronLeft className="h-3.5 w-3.5" />
        </button>

        {/* Page numbers */}
        {pageNumbers.map((p, i) =>
          p === "..." ? (
            <span key={`dots-${i}`} className="px-1 text-xs text-muted select-none">…</span>
          ) : (
            <button
              key={p}
              onClick={() => onPageChange(p as number)}
              className={`flex h-7 min-w-[1.75rem] items-center justify-center rounded-lg px-1.5 text-xs font-medium transition-colors tabular-nums ${
                currentPage === p
                  ? "bg-accent text-white"
                  : "text-muted hover:bg-card hover:text-foreground"
              }`}
            >
              {p}
            </button>
          )
        )}

        {/* Next page */}
        <button
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage === totalPages}
          className="flex h-7 w-7 items-center justify-center rounded-lg text-muted transition-colors hover:bg-card hover:text-foreground disabled:opacity-30 disabled:pointer-events-none"
          title={t.home?.nextPage || "下一页"}
        >
          <ChevronRight className="h-3.5 w-3.5" />
        </button>
        {/* Last page */}
        <button
          onClick={() => onPageChange(totalPages)}
          disabled={currentPage === totalPages}
          className="flex h-7 w-7 items-center justify-center rounded-lg text-muted transition-colors hover:bg-card hover:text-foreground disabled:opacity-30 disabled:pointer-events-none"
          title={t.home?.lastPage || "末页"}
        >
          <ChevronsRight className="h-3.5 w-3.5" />
        </button>

        {/* Jump to page */}
        {totalPages > 5 && (
          <div className="ml-2 flex items-center gap-1">
            <input
              type="text"
              value={jumpInput}
              onChange={(e) => setJumpInput(e.target.value.replace(/\D/g, ""))}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  const p = parseInt(jumpInput);
                  if (p >= 1 && p <= totalPages) {
                    onPageChange(p);
                    setJumpInput("");
                  }
                }
              }}
              placeholder={t.home?.pageInputPlaceholder || "页码"}
              className="w-12 rounded-md border border-border/50 bg-card px-1.5 py-0.5 text-center text-xs text-foreground outline-none focus:border-accent/50"
            />
            <button
              onClick={() => {
                const p = parseInt(jumpInput);
                if (p >= 1 && p <= totalPages) {
                  onPageChange(p);
                  setJumpInput("");
                }
              }}
              className="rounded-md bg-accent/10 px-2 py-0.5 text-xs font-medium text-accent hover:bg-accent/20"
            >
              {t.home?.goToPage || "跳转"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
