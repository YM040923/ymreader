# 持久化 Web 侧边栏设计

## 目标

在桌面端公共管理页面之间切换时，让 `DesktopSidebar` 保持挂载并持续显示；作品详情、漫画阅读器、小说阅读器和开发预览页面保持全屏，不显示侧边栏。

## 架构

- 在 `frontend/src/main.tsx` 增加路由级 `PersistentShellLayout`。
- Shell 自己渲染一次 `DesktopSidebar`，并为内容区统一提供 `lg:ml-[220px] xl:ml-[240px]`。
- 使用 React Router 的 `<Outlet />` 渲染当前公共页面；路径变化只重新挂载内容区，不重新挂载侧边栏。
- 首页删除自身的 `DesktopSidebar` 和重复左边距。

## 路由边界

纳入持久侧边栏：

- `/`
- `/books`
- `/recommendations`
- `/history`
- `/stats`
- `/logs`
- `/settings`
- `/scraper`
- `/tag-manager`
- `/data-admin`
- `/data-qa`

保持无侧边栏：

- `/work/:id`
- `/comic/:id`
- `/series/:id`
- `/reader/:id`
- `/novel/:id`
- `/dev/book-flip`

## 交互要求

- 桌面宽度 `lg` 以上显示侧边栏。
- 移动端继续使用现有 `MobileBottomNav`，不增加重复导航。
- 路由切换保留侧边栏实例，避免站点图标、账户信息和导航重新加载闪烁。
- 当前路径高亮继续由 `DesktopSidebar` 内的 `useLocation()` 驱动。
- 不增加折叠按钮、本地存储或新设置项。

## 验证

- TypeScript 类型检查和 Vite 生产构建通过。
- 桌面端首页、书库、历史、设置之间跳转时侧边栏持续存在。
- 作品详情和阅读器不出现侧边栏，也不残留左边距。
- 移动端不出现桌面侧边栏。

