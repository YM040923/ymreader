"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

/**
 * 外壳 chrome（左侧栏 / 顶栏）的收起状态。
 * 持久化到 localStorage，并同步到 <html data-*>，让 CSS 变量驱动布局宽度。
 */
const STORAGE_KEY = "shell-chrome-v1";

export interface ShellChromeValue {
  sidebarCollapsed: boolean;
  topbarHidden: boolean;
  toggleSidebar: () => void;
  toggleTopbar: () => void;
  showTopbar: () => void;
}

interface ShellChromeState {
  sidebarCollapsed: boolean;
  topbarHidden: boolean;
}

const DEFAULT_STATE: ShellChromeState = {
  sidebarCollapsed: false,
  topbarHidden: false,
};

const ShellChromeContext = createContext<ShellChromeValue | null>(null);

function readStored(): ShellChromeState {
  if (typeof window === "undefined") return DEFAULT_STATE;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_STATE;
    const parsed = JSON.parse(raw) as Partial<ShellChromeState>;
    return {
      sidebarCollapsed: parsed.sidebarCollapsed === true,
      topbarHidden: parsed.topbarHidden === true,
    };
  } catch {
    return DEFAULT_STATE;
  }
}

export function ShellChromeProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<ShellChromeState>(readStored);

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.sidebarCollapsed = state.sidebarCollapsed ? "true" : "false";
    root.dataset.topbarHidden = state.topbarHidden ? "true" : "false";
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      /* 隐私模式等写不进去时忽略 */
    }
  }, [state]);

  const toggleSidebar = useCallback(() => {
    setState((prev) => ({ ...prev, sidebarCollapsed: !prev.sidebarCollapsed }));
  }, []);

  const toggleTopbar = useCallback(() => {
    setState((prev) => ({ ...prev, topbarHidden: !prev.topbarHidden }));
  }, []);

  const showTopbar = useCallback(() => {
    setState((prev) => (prev.topbarHidden ? { ...prev, topbarHidden: false } : prev));
  }, []);

  const value = useMemo<ShellChromeValue>(
    () => ({ ...state, toggleSidebar, toggleTopbar, showTopbar }),
    [state, toggleSidebar, toggleTopbar, showTopbar],
  );

  return (
    <ShellChromeContext.Provider value={value}>{children}</ShellChromeContext.Provider>
  );
}

export function useShellChrome(): ShellChromeValue {
  const ctx = useContext(ShellChromeContext);
  if (ctx) return ctx;
  return {
    ...DEFAULT_STATE,
    toggleSidebar: () => {},
    toggleTopbar: () => {},
    showTopbar: () => {},
  };
}
