"use client";

import { useRouter } from "next/navigation";
import { ShieldX, ArrowLeft, Home } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

interface ForbiddenPageProps {
  /** 自定义标题，默认使用 i18n 的 forbidden */
  title?: string;
  /** 自定义描述，默认使用 i18n 的 forbiddenDesc */
  description?: string;
}

/**
 * 通用 403 无权限页面组件
 * 当用户尝试访问无权访问的书库内容时显示
 */
export default function ForbiddenPage({ title, description }: ForbiddenPageProps) {
  const router = useRouter();
  const t = useTranslation();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="flex max-w-md flex-col items-center text-center">
        <div className="mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-red-500/10">
          <ShieldX className="h-10 w-10 text-red-400" />
        </div>
        <h1 className="mb-2 text-2xl font-bold text-foreground">
          {title || t.common.forbidden}
        </h1>
        <p className="mb-8 text-muted">
          {description || t.common.forbiddenDesc}
        </p>
        <div className="flex gap-3">
          <button
            onClick={() => router.back()}
            className="flex items-center gap-2 rounded-lg border border-border/60 px-4 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-muted/10"
          >
            <ArrowLeft className="h-4 w-4" />
            {t.common.back}
          </button>
          <button
            onClick={() => router.push("/")}
            className="flex items-center gap-2 rounded-lg bg-accent px-4 py-2.5 text-sm font-medium text-white transition-colors hover:opacity-90"
          >
            <Home className="h-4 w-4" />
            {t.common.backToShelf}
          </button>
        </div>
      </div>
    </div>
  );
}
