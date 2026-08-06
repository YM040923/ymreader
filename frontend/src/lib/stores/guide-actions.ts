/**
 * 刮削状态管理 — 引导教程 Actions
 *
 * 包含：引导步骤定义、引导流程控制、帮助面板等。
 */

import { getState, notify } from "./scraper-core";
import type { GuideStep } from "./scraper-types";

/** 引导步骤定义（与i18n key对应） */
export const GUIDE_STEPS: GuideStep[] = [
  {
    id: "welcome",
    targetSelector: "[data-guide='header']",
    titleKey: "guideWelcomeTitle",
    descKey: "guideWelcomeDesc",
    placement: "bottom",
  },
  {
    id: "filter",
    targetSelector: "[data-guide='filter-bar']",
    titleKey: "guideFilterTitle",
    descKey: "guideFilterDesc",
    placement: "bottom",
    actionKey: "guideFilterAction",
  },
  {
    id: "list",
    targetSelector: "[data-guide='book-list']",
    titleKey: "guideListTitle",
    descKey: "guideListDesc",
    placement: "right",
    actionKey: "guideListAction",
  },
  {
    id: "select",
    targetSelector: "[data-guide='select-bar']",
    titleKey: "guideSelectTitle",
    descKey: "guideSelectDesc",
    placement: "bottom",
    actionKey: "guideSelectAction",
  },
  {
    id: "scrape-panel",
    targetSelector: "[data-guide='scrape-panel']",
    titleKey: "guideScrapeTitle",
    descKey: "guideScrapeDesc",
    placement: "left",
    actionKey: "guideScrapeAction",
  },
  {
    id: "ai-chat",
    targetSelector: "[data-guide='ai-chat-btn']",
    titleKey: "guideAIChatTitle",
    descKey: "guideAIChatDesc",
    placement: "top",
    actionKey: "guideAIChatAction",
  },
];

export function startGuide() {
  const state = getState();
  state.guideActive = true;
  state.guideCurrentStep = 0;
  state.aiChatOpen = false;
  state.focusedItemId = null;
  state.batchEditMode = false;
  state.helpPanelOpen = false;
  notify();
}

export function nextGuideStep() {
  const state = getState();
  if (state.guideCurrentStep < GUIDE_STEPS.length - 1) {
    state.guideCurrentStep++;
  } else {
    finishGuide();
  }
  notify();
}

export function prevGuideStep() {
  const state = getState();
  if (state.guideCurrentStep > 0) {
    state.guideCurrentStep--;
    notify();
  }
}

export function goToGuideStep(step: number) {
  if (step >= 0 && step < GUIDE_STEPS.length) {
    getState().guideCurrentStep = step;
    notify();
  }
}

export function finishGuide() {
  const state = getState();
  state.guideActive = false;
  state.guideCurrentStep = 0;
  state.guideDismissed = true;
  try {
    localStorage.setItem("scraper-guide-dismissed", "true");
  } catch { /* ignore */ }
  notify();
}

export function skipGuide() {
  const state = getState();
  state.guideActive = false;
  state.guideCurrentStep = 0;
  state.guideDismissed = true;
  try {
    localStorage.setItem("scraper-guide-dismissed", "true");
  } catch { /* ignore */ }
  notify();
}

export function resetGuide() {
  const state = getState();
  state.guideDismissed = false;
  state.guideActive = false;
  state.guideCurrentStep = 0;
  try {
    localStorage.removeItem("scraper-guide-dismissed");
  } catch { /* ignore */ }
  notify();
}

export function openHelpPanel() {
  const state = getState();
  state.helpPanelOpen = true;
  state.aiChatOpen = false;
  state.focusedItemId = null;
  state.batchEditMode = false;
  notify();
}

export function closeHelpPanel() {
  getState().helpPanelOpen = false;
  notify();
}

export function setHelpSearchQuery(query: string) {
  getState().helpSearchQuery = query;
  notify();
}

/**
 * 检查是否应该自动启动引导（首次使用检测）
 */
export function checkAutoStartGuide() {
  const state = getState();
  if (!state.guideDismissed && state.stats && state.stats.total > 0) {
    state.guideActive = true;
    state.guideCurrentStep = 0;
    state.aiChatOpen = false;
    state.focusedItemId = null;
    state.batchEditMode = false;
    state.helpPanelOpen = false;
    notify();
  }
}
