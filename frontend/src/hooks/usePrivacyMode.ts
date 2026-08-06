"use client";

import { useState, useEffect, useCallback } from "react";

const STORAGE_KEY = "privacy:enabled";
const STORAGE_BLUR_KEY = "privacy:blurNSFW";

/**
 * 隐私模式 Hook
 * 使用 localStorage 保存用户偏好，不依赖后端接口
 */
export function usePrivacyMode() {
  const [enabled, setEnabledState] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem(STORAGE_KEY) === "true";
  });

  const [blurNSFW, setBlurNSFWState] = useState(() => {
    if (typeof window === "undefined") return true;
    const val = localStorage.getItem(STORAGE_BLUR_KEY);
    return val === null ? true : val === "true"; // 默认开启模糊
  });

  const setEnabled = useCallback((v: boolean) => {
    setEnabledState(v);
    localStorage.setItem(STORAGE_KEY, String(v));
  }, []);

  const setBlurNSFW = useCallback((v: boolean) => {
    setBlurNSFWState(v);
    localStorage.setItem(STORAGE_BLUR_KEY, String(v));
  }, []);

  // 监听其他标签页的变化
  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setEnabledState(e.newValue === "true");
      if (e.key === STORAGE_BLUR_KEY) setBlurNSFWState(e.newValue !== "false");
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  return { enabled, blurNSFW, setEnabled, setBlurNSFW };
}
