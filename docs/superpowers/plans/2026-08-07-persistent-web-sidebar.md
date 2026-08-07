# Persistent Web Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在公共 Web 页面之间保持桌面侧边栏持续挂载，同时让详情页和阅读器保持全屏。

**Architecture:** 在 React Router 顶层增加共享 Shell 路由，Shell 负责侧边栏与桌面左边距，子页面通过 `<Outlet />` 渲染。首页移除页面内部重复的侧边栏结构。

**Tech Stack:** React 19、React Router、TypeScript、Tailwind CSS、Vite

---

### Task 1: 建立共享 Shell 路由

**Files:**
- Modify: `frontend/src/main.tsx`

- [ ] 导入 `Outlet` 和 `DesktopSidebar`。
- [ ] 新增 `AnimatedOutlet`，只对当前子路由内容应用入场动画。
- [ ] 新增 `PersistentShellLayout`，渲染一次侧边栏和统一内容左边距。
- [ ] 将公共页面放入 Shell 路由，将详情与阅读页面放入无 Shell 路由。

### Task 2: 清理首页重复布局

**Files:**
- Modify: `frontend/src/app/page.tsx`

- [ ] 删除首页对 `DesktopSidebar` 的导入和渲染。
- [ ] 删除首页自身的桌面左边距，保留背景、顶栏和内容结构。

### Task 3: 自动与视觉验证

**Files:**
- Test: `frontend/src/main.tsx`
- Test: `frontend/src/app/page.tsx`

- [ ] 运行 `npx tsc --noEmit`。
- [ ] 运行 `npm run build`。
- [ ] 部署测试镜像并检查首页、书库、历史、设置、详情页和阅读器。
- [ ] 确认桌面端无重复边距、移动端无重复导航。

