import { useSyncExternalStore } from "react";
import {
  subscribe,
  getSnapshot,
  type ScraperState,
} from "@/lib/scraper-store";

/**
 * 订阅全局刮削状态的 hook
 *
 * 使用 useSyncExternalStore 确保状态变更时组件正确重渲染。
 * 因为状态存储在模块级单例中，页面卸载再重新打开时
 * 依然能看到正在进行的刮削进度和已完成的结果。
 */
export function useScraperStore(): ScraperState {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
