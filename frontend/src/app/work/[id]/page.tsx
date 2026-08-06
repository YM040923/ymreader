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
  HardDrive,
  Loader2,
  Play,
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
import { formatFileSize } from "@/lib/comic-utils";
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

  return (
    <main className="min-h-screen bg-background pb-20">
      <header className="sticky top-0 z-30 border-b border-border/50 bg-background/85 backdrop-blur-xl">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4 sm:h-16 sm:px-6">
          <button
            onClick={() => router.push("/books")}
            className="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 text-muted transition-colors hover:text-foreground"
            title="返回书库"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-foreground">
              {work.title}
            </p>
            <p className="truncate text-[11px] text-muted">{work.rootPath}</p>
          </div>
        </div>
      </header>

      <div className="mx-auto max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
        <section className="overflow-hidden rounded-3xl border border-border/50 bg-card/60 shadow-xl shadow-black/5">
          <div className="grid gap-6 p-5 sm:grid-cols-[210px_1fr] sm:p-7">
            <div
              className="relative mx-auto w-full max-w-[210px] overflow-hidden rounded-2xl bg-background shadow-xl"
              style={{
                aspectRatio:
                  work.coverAspectRatio && work.coverAspectRatio > 0
                    ? String(work.coverAspectRatio)
                    : "5 / 7",
              }}
            >
              <img
                src={workCover(work)}
                alt={work.title}
                className="h-full w-full object-contain"
              />
            </div>

            <div className="flex min-w-0 flex-col justify-center">
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <span className="rounded-full bg-accent/10 px-2.5 py-1 text-accent">
                  作品
                </span>
                <span>{work.itemCount} 个阅读单元</span>
                <span>{work.pageCount} 页</span>
                {work.isFavorite && <span className="text-rose-400">已收藏</span>}
              </div>

              <h1 className="mt-3 text-2xl font-bold tracking-tight text-foreground sm:text-4xl">
                {work.title}
              </h1>

              <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted">
                {metadata.author && <span>作者：<b className="font-medium text-foreground">{metadata.author}</b></span>}
                {metadata.publisher && <span>出版：<b className="font-medium text-foreground">{metadata.publisher}</b></span>}
                {metadata.year && (
                  <span className="inline-flex items-center gap-1">
                    <CalendarDays className="h-3.5 w-3.5" />
                    {metadata.year}
                  </span>
                )}
                {metadata.language && <span>语言：{metadata.language}</span>}
                {metadata.rating !== null && (
                  <span className="inline-flex items-center gap-1 text-amber-400">
                    <Star className="h-3.5 w-3.5 fill-current" />
                    {metadata.rating}
                  </span>
                )}
              </div>

              <div className="mt-5 flex flex-wrap gap-2">
                {target && (
                  <button
                    onClick={() =>
                      router.push(
                        buildWorkReaderPath(work, target.unit, target.page),
                      )
                    }
                    className="flex h-11 items-center gap-2 rounded-xl bg-accent px-5 text-sm font-semibold text-white shadow-lg shadow-accent/25 transition-transform hover:-translate-y-0.5"
                  >
                    {target.isContinue ? (
                      <Clock3 className="h-4 w-4" />
                    ) : (
                      <Play className="h-4 w-4 fill-current" />
                    )}
                    {target.isContinue ? "继续阅读" : "立即阅读"}
                  </button>
                )}
                <div className="flex h-11 items-center gap-2 rounded-xl border border-border/60 px-4 text-sm text-muted">
                  <HardDrive className="h-4 w-4" />
                  {formatFileSize(work.fileSize || 0)}
                </div>
                <button
                  type="button"
                  onClick={async () => {
                    await toggleWorkFavorite(work);
                    await load();
                  }}
                  className="flex h-11 items-center gap-2 rounded-xl border border-border/60 px-4 text-sm text-muted hover:text-foreground"
                >
                  <Heart className={`h-4 w-4 ${work.isFavorite ? "fill-rose-400 text-rose-400" : ""}`} />
                  {work.isFavorite ? "取消收藏" : "收藏"}
                </button>
                {canManage && (
                  <button
                    type="button"
                    onClick={() => setShowMetadata((value) => !value)}
                    className="flex h-11 items-center gap-2 rounded-xl border border-border/60 px-4 text-sm text-muted hover:text-foreground"
                  >
                    <Settings2 className="h-4 w-4" />
                    元数据与封面
                  </button>
                )}
              </div>

              {(metadata.tags.length > 0
                || metadata.categories.length > 0
                || metadata.genre) && (
                <div className="mt-5 flex flex-wrap gap-1.5">
                  {metadata.categories.map((category) => (
                    <span
                      key={`category-${category.id ?? category.name}`}
                      className="rounded-full bg-purple-500/10 px-2.5 py-1 text-xs text-purple-300"
                    >
                      {category.icon || ""} {category.name}
                    </span>
                  ))}
                  {metadata.tags.map((tag) => (
                    <span
                      key={`tag-${tag.name}`}
                      className="rounded-full bg-accent/10 px-2.5 py-1 text-xs text-accent"
                      style={tag.color ? { color: tag.color } : undefined}
                    >
                      {tag.name}
                    </span>
                  ))}
                  {!metadata.tags.length && metadata.genre
                    ? metadata.genre.split(/[,，/]/).map((genre) => genre.trim()).filter(Boolean).map((genre) => (
                        <span key={genre} className="rounded-full bg-accent/10 px-2.5 py-1 text-xs text-accent">
                          {genre}
                        </span>
                      ))
                    : null}
                </div>
              )}

              {metadata.description && (
                <p className="mt-5 max-w-3xl whitespace-pre-line text-sm leading-7 text-muted">
                  {metadata.description}
                </p>
              )}
            </div>
          </div>
        </section>

        {canManage && showMetadata && metadataTarget && (
          <section className="mt-7 rounded-2xl border border-border/50 bg-card/55 p-4 sm:p-5">
            <h2 className="mb-4 text-lg font-semibold text-foreground">元数据与封面</h2>
            <div className="grid gap-3 sm:grid-cols-2">
              {(["title", "author", "publisher", "year", "language", "genre", "status", "tags", "categories", "coverUrl"] as const).map((field) => (
                <label key={field} className="space-y-1 text-xs text-muted">
                  <span>{METADATA_FIELD_LABELS[field]}</span>
                  <input value={draft[field]} onChange={(event) => setDraft((value) => ({ ...value, [field]: event.target.value }))} className="w-full rounded-xl border border-border/60 bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent" />
                </label>
              ))}
              <label className="space-y-1 text-xs text-muted sm:col-span-2">
                <span>简介</span>
                <textarea value={draft.description} onChange={(event) => setDraft((value) => ({ ...value, description: event.target.value }))} rows={5} className="w-full rounded-xl border border-border/60 bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent" />
              </label>
            </div>
            <div className="my-4 flex flex-wrap gap-2">
              <button onClick={saveMetadata} disabled={savingMetadata} className="rounded-xl bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-50">{savingMetadata ? "保存中…" : "保存元数据"}</button>
              <label className="cursor-pointer rounded-xl border border-border/60 px-4 py-2 text-sm text-muted hover:text-foreground">
                上传作品封面
                <input type="file" accept="image/*" className="hidden" onChange={async (event) => { const file = event.target.files?.[0]; if (!file) return; await uploadWorkCover(work, file); await load(); }} />
              </label>
            </div>
            {metadataTarget.type === "series" ? (
              <SeriesMetadataSearch seriesId={metadataTarget.id} groupName={work.title} contentType="comic" onApplied={load} />
            ) : (
              <MetadataSearch comicId={metadataTarget.id} comicTitle={work.title} comicType="comic" onApplied={load} />
            )}
          </section>
        )}

        <section className="mt-7">
          <div className="mb-3 flex items-end justify-between">
            <div>
              <h2 className="text-lg font-semibold text-foreground">目录</h2>
              <p className="mt-0.5 text-xs text-muted">
                卷、话、内部 ZIP/CBZ 章节和 PDF 全文统一显示
              </p>
            </div>
            <span className="text-xs text-muted">{sortedUnits.length} 项</span>
          </div>

          <div className="space-y-2">
            {sortedUnits.map((unit, index) => {
              const fallbackState = comicStates[unit.comicId];
              const progress = unitProgress(unit, fallbackState);
              const isCurrent = target?.unit.id === unit.id && target.isContinue;
              const unitCover =
                unit.coverUrl
                || comicMap.get(unit.comicId)?.coverUrl
                || workCover(work);
              return (
                <button
                  key={unit.id}
                  type="button"
                  onClick={() =>
                    router.push(buildWorkReaderPath(work, unit, unit.startPage))
                  }
                  className="group flex w-full items-center gap-3 rounded-2xl border border-border/50 bg-card/55 p-2.5 text-left transition-all hover:border-accent/30 hover:bg-card-hover"
                >
                  <div className="relative h-20 w-14 shrink-0 overflow-hidden rounded-lg bg-background">
                    <img
                      src={unitCover}
                      alt={unit.displayLabel || unit.title}
                      className="h-full w-full object-contain"
                      loading="lazy"
                    />
                    {progress.percentage > 0 && (
                      <div className="absolute inset-x-0 bottom-0 h-1 bg-black/30">
                        <div
                          className="h-full bg-accent"
                          style={{ width: `${progress.percentage}%` }}
                        />
                      </div>
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-xs tabular-nums text-muted/60">
                        {String(index + 1).padStart(3, "0")}
                      </span>
                      <h3 className="truncate text-sm font-medium text-foreground group-hover:text-accent">
                        {unit.displayLabel || unit.title || `第 ${index + 1} 话`}
                      </h3>
                      {isCurrent && (
                        <span className="shrink-0 rounded-full bg-accent/10 px-2 py-0.5 text-[10px] text-accent">
                          上次读到这里
                        </span>
                      )}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-3 text-[11px] text-muted">
                      <span>{unit.pageCount || 0} 页</span>
                      {unit.internalPath && <span className="truncate">{unit.internalPath}</span>}
                      {progress.started && <span>{progress.percentage}%</span>}
                    </div>
                  </div>
                  {progress.percentage >= 100 ? (
                    <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-400" />
                  ) : (
                    <BookOpen className="h-5 w-5 shrink-0 text-muted transition-colors group-hover:text-accent" />
                  )}
                </button>
              );
            })}
          </div>
        </section>
        {(work.representativeComicId || work.coverComicId) && (
          <SimilarComics comicId={work.representativeComicId || work.coverComicId} />
        )}
      </div>
    </main>
  );
}
