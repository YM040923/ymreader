import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown } from "lucide-react";

export type ToolbarSelectOption = { value: string; label: string };

type Props = {
  value: string;
  options: ToolbarSelectOption[];
  onChange: (value: string) => void;
  ariaLabel?: string;
  className?: string;
};

const MENU_WIDTH = 132;
const MENU_GAP = 4;

/**
 * 工具栏下拉：按钮与站点一致，弹层渲染到 body，避免被工具栏的
 * overflow-x-auto 容器裁掉；样式使用卡片底色/描边，替代原生 select 系统菜单。
 */
export default function ToolbarSelect({
  value,
  options,
  onChange,
  ariaLabel,
  className = "",
}: Props) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState({ left: 0, top: 0 });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const placeMenu = useCallback(() => {
    const rect = buttonRef.current?.getBoundingClientRect();
    if (!rect) return;
    const width = Math.max(MENU_WIDTH, rect.width);
    const left = Math.min(
      Math.max(8, rect.right - width),
      window.innerWidth - width - 8
    );
    const top = Math.min(rect.bottom + MENU_GAP, window.innerHeight - 8);
    setPosition({ left, top });
  }, []);

  useLayoutEffect(() => {
    if (open) placeMenu();
  }, [open, placeMenu]);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (
        !buttonRef.current?.contains(target) &&
        !menuRef.current?.contains(target)
      ) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    const onViewportChange = () => setOpen(false);
    window.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("resize", onViewportChange);
    window.addEventListener("scroll", onViewportChange, true);
    return () => {
      window.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("resize", onViewportChange);
      window.removeEventListener("scroll", onViewportChange, true);
    };
  }, [open]);

  const current = options.find((option) => option.value === value) ?? options[0];

  return (
    <div className={`relative ${className}`}>
      <button
        ref={buttonRef}
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className={`flex h-8 items-center gap-1 rounded-lg px-2.5 text-xs font-medium transition-colors ${
          open
            ? "bg-foreground/[0.06] text-foreground"
            : "bg-card text-muted hover:text-foreground"
        }`}
      >
        <span className="whitespace-nowrap">{current?.label}</span>
        <ChevronDown
          className={`h-3.5 w-3.5 text-muted transition-transform duration-150 ${
            open ? "rotate-180" : ""
          }`}
        />
      </button>

      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={menuRef}
            role="listbox"
            aria-label={ariaLabel}
            style={{ left: position.left, top: position.top, minWidth: MENU_WIDTH }}
            className="fixed z-[80] overflow-hidden rounded-lg border border-border/60 bg-card py-1 shadow-xl shadow-black/25"
          >
            {options.map((option) => {
              const selected = option.value === value;
              return (
                <button
                  key={option.value}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  onClick={() => {
                    onChange(option.value);
                    setOpen(false);
                  }}
                  className={`flex w-full items-center px-3 py-1.5 text-left text-xs transition-colors ${
                    selected
                      ? "bg-accent/15 text-accent"
                      : "text-muted hover:bg-foreground/5 hover:text-foreground"
                  }`}
                >
                  {option.label}
                </button>
              );
            })}
          </div>,
          document.body
        )}
    </div>
  );
}
