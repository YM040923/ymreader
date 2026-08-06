"use client";

import { useLocale, type Locale } from "@/lib/i18n";

const localeLabels: Record<Locale, string> = {
  "zh-CN": "中文",
  en: "English",
};

interface LanguageSwitcherProps {
  /** "button" = 独立圆角按钮（默认）；"inline" = 菜单内纯文本样式 */
  variant?: "button" | "inline";
}

export default function LanguageSwitcher({ variant = "button" }: LanguageSwitcherProps) {
  const { locale, setLocale } = useLocale();

  const toggle = () => {
    setLocale(locale === "zh-CN" ? "en" : "zh-CN");
  };

  if (variant === "inline") {
    return (
      <button
        onClick={toggle}
        className="flex-1 text-left text-sm text-muted hover:text-foreground transition-colors"
        title={locale === "zh-CN" ? "Switch to English" : "切换到中文"}
      >
        {localeLabels[locale]}
      </button>
    );
  }

  return (
    <button
      onClick={toggle}
      className="flex h-9 w-9 items-center justify-center rounded-xl border border-border/60 text-muted transition-colors duration-200 hover:border-border hover:text-foreground"
      title={locale === "zh-CN" ? "Switch to English" : "切换到中文"}
    >
      <span className="text-xs font-medium">{localeLabels[locale]}</span>
    </button>
  );
}
