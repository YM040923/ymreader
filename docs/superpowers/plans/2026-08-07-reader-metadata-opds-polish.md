# YMReader Reader Metadata OPDS Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成中文元数据、刮削封面、Web 阅读下栏/连续阅读及 OPDS 多 CBZ 的最终收尾。

**Architecture:** 保持现有 Work/Unit 模型，分别在元数据排序层、Progress DTO、Reader 导航层和 OPDS feed/acquisition 层修复。所有行为先由回归测试定义，再做最小实现。

**Tech Stack:** Go/Gin/SQLite、React 19/TypeScript/Vite、OPDS Atom、Docker。

---

### Task 1: 中文元数据候选

**Files:**
- Modify: `internal/service/metadata_anilist.go`
- Modify: `internal/service/metadata_http.go`
- Modify: `internal/service/metadata_mangadex.go`
- Modify: `internal/service/metadata_kitsu.go`
- Modify: `internal/service/metadata_relevance.go`
- Test: `internal/service/metadata_language_test.go`

- [ ] 写失败测试：中文语言键选择、MangaDex 中文别名、中文候选排序、AniList 查询结构。
- [ ] 运行 `go test ./internal/service -run 'MetadataLanguage|MetadataRelevance|AniListQuery' -count=1`，确认测试因缺少行为失败。
- [ ] 实现语言归一化、中文别名选择、来源/中文完整度评分和 AniList 查询修复。
- [ ] 重新运行目标测试和 `go test ./internal/service -count=1`。

### Task 2: 刮削进度封面

**Files:**
- Modify: `internal/handler/metadata_targets.go`
- Modify: `internal/handler/metadata_selected_handler.go`
- Modify: `frontend/src/lib/stores/scraper-types.ts`
- Modify: `frontend/src/lib/stores/scraper-batch-actions.ts`
- Modify: `frontend/src/app/scraper/page.tsx`
- Test: `internal/handler/metadata_progress_test.go`
- Test: `frontend/scripts/test-scraper-cover.mjs`

- [ ] 写失败测试：Work 进度返回 Work cover/代表 Comic ID，前端不再将 Work ID 拼到 Comic thumbnail。
- [ ] 运行目标 Go/Node 测试并确认失败。
- [ ] 补齐 DTO，增加统一封面解析与图片失败占位。
- [ ] 重新运行测试。

### Task 3: 阅读下栏与连续阅读

**Files:**
- Modify: `frontend/src/components/reader/ReaderToolbar.tsx`
- Modify: `frontend/src/app/reader/[id]/page.tsx`
- Modify: `frontend/src/components/reader/WebtoonView.tsx`
- Create: `frontend/src/lib/reader/continuousReading.ts`
- Test: `frontend/scripts/test-reader-navigation.mjs`

- [ ] 写失败测试：明确上下话/目录按钮、目录自动定位、长条与连续阅读状态独立、下一 Unit 页面拼接。
- [ ] 运行 Node 测试确认失败。
- [ ] 将章节导航合并进下栏，增加独立连续阅读开关。
- [ ] 实现长条模式的下一 Unit 预取、拼接和进度上下文切换。
- [ ] 运行测试和前端构建。

### Task 4: OPDS 多 CBZ

**Files:**
- Modify: `internal/service/opds.go`
- Modify: `internal/handler/opds_work_handler.go`
- Modify: `internal/handler/opds_handler.go`
- Test: `internal/handler/opds_work_handler_test.go`

- [ ] 写失败测试：多 CBZ Work 入口返回 Unit 导航，每个 Unit acquisition 指向真实 CBZ，整体 ZIP 保持单文件。
- [ ] 运行 `go test ./internal/handler -run 'OPDS.*Work|OPDS.*Multi' -count=1` 并确认失败。
- [ ] 修正 feed 与 acquisition 链接构造。
- [ ] 验证 GET、HEAD、Range 及认证。

### Task 5: 集成、部署与真实验证

**Files:**
- Modify: `frontend/package.json`（仅在增加测试脚本时）

- [ ] 运行 `gofmt`、目标测试、`go test ./...`。
- [ ] 运行前端 Node 回归测试、`npm run lint`、`npm run build`。
- [ ] 构建新镜像并替换 `ymreader-unified-work-clean`，保持正式 `6680` 停止。
- [ ] 在 `17680` 点击验证刮削页、Web 阅读器及 OPDS 真实漫画。
- [ ] 检查容器日志、健康状态和 Git diff 后提交。

