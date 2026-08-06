"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  Clock3,
  FileStack,
  Grid2X2,
  HardDrive,
  Loader2,
  List,
  Pencil,
  Play,
  Save,
  Sparkles,
  Star,
  Settings2,
  Heart,
} from "lucide-react";
import { fetchWork, fetchWorkComics } from "@/api/works";
import { apiPath } from "@/lib/base-path";
import {
  buildWorkReaderPath,
  selectWorkReadingTarget,
  getWorkMetadataTarget,
} from "@/lib/work-model";
import { canManageWorkLibrary } from "@/lib/work-model";
import { formatDuration, formatFileSize } from "@/lib/comic-utils";
import type { ApiComic } from "@/hooks/useComicTypes";
import type { Work, WorkComicState, WorkUnit } from "@/types/work";
import { useAuth } from "@/lib/auth-context";
import { MetadataSearch } from "@/components/MetadataSearch";
import { SeriesMetadataSearch } from "@/components/SeriesMetadataSearch";
import { fetchAccessibleLibraries, type Library } from "@/api/libraries";
import {
  setWorkCategories,
  setWorkCoverUrl,
  setWorkTags,
  toggleWorkFavorite,
  updateWorkMetadata,
  uploadWorkCover,
} from "@/api/workMetadata";
import { SimilarComics } from "@/components/Recommendations";

function unitProgress(
  unit: WorkUnit,
  fallback?: WorkComicState,
): { page: number; percentage: number; started: boolean } {
  const storedPage = unit.lastReadPage ?? fallback?.lastReadPage ?? unit.startPage;
  const rawPage =
    unit.startPage > 0 && storedPage < unit.startPage
      ? unit.startPage + storedPage
      : storedPage;
  const started =
    Boolean(unit.lastReadAt ?? fallback?.lastReadAt)
    || rawPage > unit.startPage
    || unit.readingStatus === "reading"
    || unit.readingStatus === "finished"
    || fallback?.readingStatus === "reading"
    || fallback?.readingStatus === "finished";
  if (!started || unit.pageCount <= 0) {
    return { page: unit.startPage, percentage: 0, started: false };
  }
  const pageInUnit = Math.max(
    0,
    Math.min(unit.pageCount - 1, rawPage - unit.startPage),
  );
  return {
    page: rawPage,
    percentage: Math.min(
      100,
      Math.round(((pageInUnit + 1) / unit.pageCount) * 100),
    ),
    started: true,
  };
}

function workCover(work: Work): string {
  return work.coverUrl
    || (work.coverComicId
      ? apiPath(`/api/comics/${encodeURIComponent(work.coverComicId)}/thumbnail`)
      : apiPath("/api/placeholder/400/560"));
}

function unitSection(unit: WorkUnit): string {
  const labelParts = (unit.displayLabel || "").split(/\s+\/\s+/).filter(Boolean);
  if (labelParts.length > 1) return labelParts[0];
  const pathParts = (unit.internalPath || "").split("/").filter(Boolean);
  if (pathParts.length > 2) return pathParts[0];
  return "";
}

const METADATA_FIELD_LABELS = {
  title: "标题",
  author: "作者",
  publisher: "出版社",
  year: "年份",
  language: "语言",
  genre: "类型",
  status: "状态",
  tags: "标签（逗号分隔）",
  categories: "分类标识（逗号分隔）",
  coverUrl: "封面网址",
} as const;

export default function WorkDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = String(params?.id || "");
  const [work, setWork] = useState<Work | null>(null);
  const [comics, setComics] = useState<ApiComic[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showMetadata, setShowMetadata] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [activeSection, setActiveSection] = useState("all");
  const [viewMode, setViewMode] = useState<"list" | "grid">(() => {
    if (typeof window === "undefined") return "grid";
    return localStorage.getItem("work:unitView") === "list" ? "list" : "grid";
  });
  const [libraries, setLibraries] = useState<Library[]>([]);
  const [savingMetadata, setSavingMetadata] = useState(false);
  const [draft, setDraft] = useState({
    title: "", author: "", publisher: "", year: "", description: "",
    language: "", genre: "", status: "", tags: "", categories: "", coverUrl: "",
  });
  const { user } = useAuth();
  const canManage = work
    ? canManageWorkLibrary(user, work.libraryId, libraries)
    : user?.role === "admin";

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError("");
    try {
      const result = await fetchWork(id);
      setWork(result);
      setDraft({
        title: result.title || "",
        author: result.author || "",
        publisher: result.publisher || "",
        year: result.year ? String(result.year) : "",
        description: result.description || "",
        language: result.language || "",
        genre: result.genre || "",
        status: result.status || "",
        tags: (result.tags || []).map((tag) => tag.name).join(", "),
        categories: (result.categories || []).map((category) => category.slug || category.name).join(", "),
        coverUrl: result.coverUrl || "",
      });

      const hasField = (name: keyof Work) =>
        Object.prototype.hasOwnProperty.call(result, name);
      const needsLegacyFallback =
        !hasField("continueComicId")
        || !hasField("author")
        || !hasField("description");
      if (needsLegacyFallback) {
        fetchWorkComics(result)
          .then(setComics)
          .catch(() => setComics([]));
      } else {
        setComics([]);
      }
    } catch (caught) {
      setError(
        typeof caught === "object" && caught && "message" in caught
          ? String(caught.message)
          : "作品加载失败",
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    fetchAccessibleLibraries().then(setLibraries).catch(() => setLibraries([]));
  }, []);

  useEffect(() => {
    localStorage.setItem("work:unitView", viewMode);
  }, [viewMode]);

  const saveMetadata = useCallback(async () => {
    if (!work || !canManage) return;
    setSavingMetadata(true);
    try {
      await updateWorkMetadata(work, {
        title: draft.title.trim(),
        author: draft.author.trim(),
        publisher: draft.publisher.trim(),
        year: draft.year.trim() ? Number(draft.year) : null,
        description: draft.description,
        language: draft.language.trim(),
        genre: draft.genre.trim(),
        status: draft.status.trim(),
      });
      await Promise.all([
        setWorkTags(work, draft.tags.split(/[,，]/).map((value) => value.trim()).filter(Boolean)),
        setWorkCategories(work, draft.categories.split(/[,，]/).map((value) => value.trim()).filter(Boolean)),
        draft.coverUrl.trim() ? setWorkCoverUrl(work, draft.coverUrl.trim()) : Promise.resolve(),
      ]);
      await load();
    } finally {
      setSavingMetadata(false);
    }
  }, [canManage, draft, load, work]);

  const saveTitle = useCallback(async () => {
    if (!work || !canManage || !draft.title.trim()) return;
    setSavingMetadata(true);
    try {
      await updateWorkMetadata(work, { title: draft.title.trim() });
      await load();
      setEditingTitle(false);
    } finally {
      setSavingMetadata(false);
    }
  }, [canManage, draft.title, load, work]);

  const comicMap = useMemo(
    () => new Map(comics.map((comic) => [comic.id, comic])),
    [comics],
  );
  const coverComic = work ? comicMap.get(work.coverComicId) : undefined;
  const comicStates = useMemo<Record<string, WorkComicState>>(
    () =>
      Object.fromEntries(
        comics.map((comic) => [
          comic.id,
          {
            lastReadPage: comic.lastReadPage,
            lastReadAt: comic.lastReadAt,
            readingStatus: comic.readingStatus,
          },
        ]),
      ),
    [comics],
  );
  const target = useMemo(
    () => (work ? selectWorkReadingTarget(work, comicStates) : null),
    [comicStates, work],
  );
  const metadataTarget = useMemo(
    () => (work ? getWorkMetadataTarget(work) : null),
    [work],
  );

  if (loading) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-accent" />
      </main>
    );
  }

  if (!work || error) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background px-6 text-center">
        <FileStack className="h-14 w-14 text-muted/30" />
        <p className="text-sm text-muted">{error || "作品不存在"}</p>
        <div className="flex gap-2">
          <button
            onClick={() => router.push("/books")}
            className="rounded-xl bg-accent px-4 py-2 text-sm font-medium text-white"
          >
            返回书库
          </button>
          <button
            onClick={load}
            className="rounded-xl border border-border px-4 py-2 text-sm text-muted"
          >
            重试
          </button>
        </div>
      </main>
    );
  }

  const metadata = {
    author: work.author || coverComic?.author || "",
    publisher: work.publisher || coverComic?.publisher || "",
    year: work.year ?? coverComic?.year ?? null,
    description: work.description || coverComic?.description || "",
    language: work.language || coverComic?.language || "",
    genre: work.genre || coverComic?.genre || "",
    rating: work.rating ?? coverComic?.rating ?? null,
    tags:
      work.tags?.length
        ? work.tags
        : (coverComic?.tags || []).map((tag) => ({
            name: tag.name,
            color: tag.color,
          })),
    categories:
      work.categories?.length
        ? work.categories
        : coverComic?.categories || [],
  };
  const sortedUnits = [...work.units].sort(
    (left, right) => left.sortIndex - right.sortIndex,
  );
  const sectionNames = [...new Set(sortedUnits.map(unitSection).filter(Boolean))];
  const unsectionedCount = sortedUnits.filter((unit) => !unitSection(unit)).length;
  const visibleUnits = activeSection === "all"
    ? sortedUnits
    : activeSection === "unsectioned"
      ? sortedUnits.filter((unit) => !unitSection(unit))
      : sortedUnits.filter((unit) => unitSection(unit) === activeSection);
  const completedItemCount = work.completedItemCount
    ?? sortedUnits.filter((unit) => unit.readingStatus === "finished").length;
  const overallProgress = work.itemCount > 0
    ? Math.round((completedItemCount / work.itemCount) * 100)
    : 0;

  const renderUnit = (unit: WorkUnit, index: number) => {
    const fallbackState = comicStates[unit.comicId];
    const progress = unitProgress(unit, fallbackState);
    const finished = unit.readingStatus === "finished" || progress.percentage >= 100;
    const isCurrent = target?.unit.id === unit.id && target.isContinue;
    const unitCover = unit.coverUrl || comicMap.get(unit.comicId)?.coverUrl || workCover(work);
    const title = unit.displayLabel || unit.title || `第 ${index + 1} 话`;
    const openUnit = () => router.push(buildWorkReaderPath(work, unit, unit.startPage));

    if (viewMode === "grid") {
      return (
        <button
          key={unit.id}
          type="button"
          onClick={openUnit}
          className="group overflow-hidden rounded-2xl border border-border/50 bg-card/65 text-left transition-all hover:-translate-y-0.5 hover:border-accent/30 hover:shadow-lg"
        >
          <div
            className="relative w-full overflow-hidden bg-background"
            style={{ aspectRatio: unit.coverAspectRatio && unit.coverAspectRatio > 0 ? String(unit.coverAspectRatio) : "5 / 7" }}
          >
            <img src={unitCover} alt={title} className="h-full w-full object-contain" loading="lazy" />
            <span className={`absolute left-2 top-2 rounded-full px-2 py-1 text-[10px] font-medium text-white ${finished ? "bg-emerald-500/90" : "bg-black/65"}`}>
              {finished ? "已读" : progress.started ? `${progress.percentage}%` : "未读"}
            </span>
            {isCurrent && <span className="absolute right-2 top-2 rounded-full bg-accent px-2 py-1 text-[10px] text-white">续读</span>}
            <div className="absolute inset-x-0 bottom-0 h-1.5 bg-black/30">
              <div className="h-full bg-accent" style={{ width: `${progress.percentage}%` }} />
            </div>
          </div>
          <div className="p-3">
            <p className="line-clamp-2 text-sm font-medium text-foreground group-hover:text-accent">{title}</p>
            <div className="mt-1.5 flex items-center justify-between text-[11px] text-muted">
              <span>{unit.pageCount || 0} 页</span>
              <span className="tabular-nums">{String(index + 1).padStart(3, "0")}</span>
            </div>
          </div>
        </button>
      );
    }

    return (
      <button
        key={unit.id}
        type="button"
        onClick={openUnit}
        className="group flex w-full items-center gap-3 rounded-2xl border border-border/50 bg-card/55 p-2.5 text-left transition-all hover:border-accent/30 hover:bg-card-hover"
      >
        <div
          className="relative w-14 shrink-0 overflow-hidden rounded-lg bg-background"
          style={{ aspectRatio: unit.coverAspectRatio && unit.coverAspectRatio > 0 ? String(unit.coverAspectRatio) : "5 / 7" }}
        >
          <img src={unitCover} alt={title} className="h-full w-full object-contain" loading="lazy" />
          {progress.percentage > 0 && <div className="absolute inset-x-0 bottom-0 h-1 bg-black/30"><div className="h-full bg-accent" style={{ width: `${progress.percentage}%` }} /></div>}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-xs tabular-nums text-muted/60">{String(index + 1).padStart(3, "0")}</span>
            <h3 className="truncate text-sm font-medium text-foreground group-hover:text-accent">{title}</h3>
            {isCurrent && <span className="shrink-0 rounded-full bg-accent/10 px-2 py-0.5 text-[10px] text-accent">上次读到这里</span>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-3 text-[11px] text-muted">
            <span>{unit.pageCount || 0} 页</span>
            {unit.internalPath && <span className="truncate">{unit.internalPath}</span>}
            {progress.started && <span>{progress.percentage}%</span>}
          </div>
        </div>
        {finished ? <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-400" /> : <BookOpen className="h-5 w-5 shrink-0 text-muted group-hover:text-accent" />}
      </button>
    );
  };

  return (
    <main className="min-h-screen bg-background pb-24">
      <header className="sticky top-0 z-30 border-b border-border/50 bg-background/85 backdrop-blur-xl">
        <div className="mx-auto flex max-w-6xl items-center gap-3 px-4 py-3 sm:px-6">
          <button onClick={() => router.push("/books")} className="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 text-muted hover:text-foreground" title="返回书库">
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-foreground">{work.title}</p>
            <p className="truncate text-[11px] text-muted">{work.rootPath}</p>
          </div>
          <div className="flex rounded-lg border border-border/60 bg-card/60 p-0.5" aria-label="目录显示方式">
            <button type="button" title="列表模式" onClick={() => setViewMode("list")} className={`flex h-8 w-8 items-center justify-center rounded-md ${viewMode === "list" ? "bg-accent text-white" : "text-muted"}`}><List className="h-4 w-4" /></button>
            <button type="button" title="封面网格" onClick={() => setViewMode("grid")} className={`flex h-8 w-8 items-center justify-center rounded-md ${viewMode === "grid" ? "bg-accent text-white" : "text-muted"}`}><Grid2X2 className="h-4 w-4" /></button>
          </div>
        </div>
      </header>

      <div className="mx-auto max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
        <section className="overflow-hidden rounded-3xl border border-border/50 bg-card/55 shadow-xl shadow-black/5">
          <div className="grid gap-6 p-5 sm:grid-cols-[180px_1fr] sm:p-7">
            <div
              className="relative mx-auto w-full max-w-[180px] overflow-hidden rounded-2xl bg-card shadow-lg"
              style={{ aspectRatio: work.coverAspectRatio && work.coverAspectRatio > 0 ? String(work.coverAspectRatio) : "5 / 7" }}
            >
              <img src={workCover(work)} alt={work.title} className="h-full w-full object-contain" />
              <div className="absolute inset-x-0 bottom-0 h-1.5 bg-black/30"><div className="h-full bg-accent" style={{ width: `${overallProgress}%` }} /></div>
            </div>

            <div className="flex min-w-0 flex-col justify-center">
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <span className="rounded-full bg-accent/10 px-2.5 py-1 text-accent">目录作品</span>
                <span>{work.itemCount} 个阅读单元</span>
                {sectionNames.length > 0 && <span>{sectionNames.length} 季/篇</span>}
                <span>{work.pageCount} 页</span>
              </div>

              {editingTitle ? (
                <div className="mt-3 flex max-w-xl gap-2">
                  <input value={draft.title} onChange={(event) => setDraft((value) => ({ ...value, title: event.target.value }))} className="h-11 flex-1 rounded-xl border border-border bg-background px-3 text-lg font-semibold outline-none focus:border-accent" autoFocus />
                  <button onClick={saveTitle} disabled={savingMetadata} className="flex h-11 items-center gap-1.5 rounded-xl bg-accent px-4 text-sm font-medium text-white"><Save className="h-4 w-4" />保存</button>
                </div>
              ) : (
                <div className="mt-3 flex items-start gap-2">
                  <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">{work.title}</h1>
                  {canManage && <button type="button" onClick={() => setEditingTitle(true)} className="mt-1 rounded-lg p-1.5 text-muted hover:bg-card hover:text-foreground" title="修改作品标题"><Pencil className="h-4 w-4" /></button>}
                </div>
              )}

              <div className="mt-5 grid max-w-2xl grid-cols-2 gap-3 sm:grid-cols-4">
                <div className="rounded-xl bg-background/70 p-3"><p className="text-[11px] text-muted">整部进度</p><p className="mt-1 text-lg font-semibold">{completedItemCount}/{work.itemCount}</p></div>
                <div className="rounded-xl bg-background/70 p-3"><p className="text-[11px] text-muted">完成比例</p><p className="mt-1 text-lg font-semibold">{overallProgress}%</p></div>
                <div className="rounded-xl bg-background/70 p-3"><p className="text-[11px] text-muted">总大小</p><p className="mt-1 text-lg font-semibold">{formatFileSize(work.fileSize || 0)}</p></div>
                <div className="rounded-xl bg-background/70 p-3"><p className="text-[11px] text-muted">阅读时长</p><p className="mt-1 text-lg font-semibold">{formatDuration(work.totalReadTime || 0)}</p></div>
              </div>

              <div className="mt-5 flex flex-wrap gap-2">
                {target && <button type="button" onClick={() => router.push(buildWorkReaderPath(work, target.unit, target.page))} className="flex h-10 items-center gap-2 rounded-xl bg-accent px-4 text-sm font-medium text-white shadow-lg shadow-accent/20"><BookOpen className="h-4 w-4" />{target.isContinue ? "继续阅读" : "开始阅读"}</button>}
                <button type="button" onClick={async () => { await toggleWorkFavorite(work); await load(); }} className="flex h-10 items-center gap-2 rounded-xl border border-border px-4 text-sm text-foreground hover:bg-background"><Heart className={`h-4 w-4 ${work.isFavorite ? "fill-rose-400 text-rose-400" : ""}`} />{work.isFavorite ? "取消收藏" : "收藏"}</button>
                {canManage && <button type="button" onClick={() => setShowMetadata((value) => !value)} className="flex h-10 items-center gap-2 rounded-xl border border-border px-4 text-sm text-foreground hover:bg-background"><Sparkles className="h-4 w-4 text-purple-400" />刮削与元数据</button>}
              </div>

              {(metadata.author || metadata.year || metadata.publisher || metadata.language || metadata.genre || metadata.description || metadata.tags.length > 0 || metadata.categories.length > 0 || work.externalRating != null) && (
                <div className="mt-5 max-w-3xl space-y-3 border-t border-border/50 pt-5">
                  <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted">
                    {metadata.author && <span>作者：<span className="text-foreground">{metadata.author}</span></span>}
                    {metadata.year && <span>年份：<span className="text-foreground">{metadata.year}</span></span>}
                    {metadata.publisher && <span>出版：<span className="text-foreground">{metadata.publisher}</span></span>}
                    {metadata.language && <span>语言：<span className="text-foreground">{metadata.language}</span></span>}
                    {work.externalRating != null && <span>评分：<span className="text-foreground">{work.externalRating}{work.externalRatingMax ? `/${work.externalRatingMax}` : ""}</span></span>}
                  </div>
                  {(metadata.tags.length > 0 || metadata.categories.length > 0 || metadata.genre) && <div className="flex flex-wrap gap-1.5">
                    {metadata.categories.map((category) => <span key={`category-${category.id ?? category.name}`} className="rounded-full bg-purple-500/10 px-2.5 py-1 text-xs text-purple-300">{category.icon || ""} {category.name}</span>)}
                    {metadata.tags.map((tag) => <span key={`tag-${tag.name}`} className="rounded-full bg-accent/10 px-2.5 py-1 text-xs text-accent" style={tag.color ? { color: tag.color } : undefined}>{tag.name}</span>)}
                    {!metadata.tags.length && metadata.genre ? metadata.genre.split(/[,，/]/).map((name) => name.trim()).filter(Boolean).map((name) => <span key={name} className="rounded-full bg-accent/10 px-2.5 py-1 text-xs text-accent">{name}</span>) : null}
                  </div>}
                  {metadata.description && <p className="whitespace-pre-line text-sm leading-6 text-muted">{metadata.description}</p>}
                </div>
              )}
            </div>
          </div>
        </section>

        {canManage && showMetadata && metadataTarget && (
          <section className="mt-6 rounded-2xl border border-border/50 bg-card/55 p-4 sm:p-5">
            <div className="mb-4 flex items-center justify-between"><h2 className="text-lg font-semibold text-foreground">刮削、元数据与封面</h2><button type="button" onClick={() => setShowMetadata(false)} className="text-sm text-muted hover:text-foreground">收起</button></div>
            <div className="grid gap-3 sm:grid-cols-2">
              {(["title", "author", "publisher", "year", "language", "genre", "status", "tags", "categories", "coverUrl"] as const).map((field) => <label key={field} className="space-y-1 text-xs text-muted"><span>{METADATA_FIELD_LABELS[field]}</span><input value={draft[field]} onChange={(event) => setDraft((value) => ({ ...value, [field]: event.target.value }))} className="w-full rounded-xl border border-border/60 bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent" /></label>)}
              <label className="space-y-1 text-xs text-muted sm:col-span-2"><span>简介</span><textarea value={draft.description} onChange={(event) => setDraft((value) => ({ ...value, description: event.target.value }))} rows={5} className="w-full rounded-xl border border-border/60 bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent" /></label>
            </div>
            <div className="my-4 flex flex-wrap gap-2">
              <button onClick={saveMetadata} disabled={savingMetadata} className="rounded-xl bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50">{savingMetadata ? "保存中…" : "保存元数据"}</button>
              <label className="cursor-pointer rounded-xl border border-border/60 px-4 py-2 text-sm text-muted hover:text-foreground">上传作品封面<input type="file" accept="image/*" className="hidden" onChange={async (event) => { const file = event.target.files?.[0]; if (!file) return; await uploadWorkCover(work, file); await load(); }} /></label>
            </div>
            {metadataTarget.type === "series" ? <SeriesMetadataSearch seriesId={metadataTarget.id} groupName={work.title} contentType="comic" onApplied={load} /> : <MetadataSearch comicId={metadataTarget.id} comicTitle={work.title} comicType="comic" onApplied={load} />}
          </section>
        )}

        {(sectionNames.length > 0 || unsectionedCount > 0) && (
          <nav className="mt-6 flex gap-2 overflow-x-auto pb-1" aria-label="目录分区">
            <button type="button" onClick={() => setActiveSection("all")} className={`shrink-0 rounded-full px-4 py-2 text-sm ${activeSection === "all" ? "bg-accent text-white" : "bg-card text-muted"}`}>全部 {sortedUnits.length}</button>
            {unsectionedCount > 0 && sectionNames.length > 0 && <button type="button" onClick={() => setActiveSection("unsectioned")} className={`shrink-0 rounded-full px-4 py-2 text-sm ${activeSection === "unsectioned" ? "bg-accent text-white" : "bg-card text-muted"}`}>未分篇 {unsectionedCount}</button>}
            {sectionNames.map((section) => <button type="button" key={section} onClick={() => setActiveSection(section)} className={`shrink-0 rounded-full px-4 py-2 text-sm ${activeSection === section ? "bg-accent text-white" : "bg-card text-muted"}`}>{section} {sortedUnits.filter((unit) => unitSection(unit) === section).length}</button>)}
          </nav>
        )}

        <section className="mt-6">
          <div className="mb-3 flex items-center justify-between"><div><h2 className="text-base font-semibold">阅读单元</h2><p className="mt-0.5 text-xs text-muted">卷、话、内部 ZIP/CBZ 章节和 PDF 全文统一显示</p></div><span className="text-xs text-muted">{visibleUnits.length} 项</span></div>
          <div className={viewMode === "grid" ? "grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5" : "space-y-2"}>
            {visibleUnits.map((unit) => renderUnit(unit, sortedUnits.findIndex((candidate) => candidate.id === unit.id)))}
          </div>
        </section>

        {(work.representativeComicId || work.coverComicId) && <SimilarComics comicId={work.representativeComicId || work.coverComicId} />}
      </div>
    </main>
  );
}
