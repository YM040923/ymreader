# Realtime Reading Progress Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Android、Web 与服务端可靠同步作品级“话/卷 + 页”阅读游标，支持正常回退、断网补传和返回详情页即时刷新。

**Architecture:** 扩展现有 reading activity 请求，使客户端提交 `workId/unitId/relativePage`，服务端校验并计算绝对页，在同一事务中写入 Comic 状态和新增的 `UserWorkProgress`。Android 使用串行、可持久化的最新事件 outbox，同步成功后刷新 Work 详情、首页与继续阅读模块；旧客户端继续走现有 Comic 进度回退路径。

**Tech Stack:** Go 1.23、Gin、SQLite、Flutter/Dart、Riverpod、Dio、SharedPreferences、Next.js/TypeScript、Go/Flutter 单元与集成测试。

---

### Task 1: 服务端作品游标存储

**Files:**
- Modify: `internal/store/db.go`
- Modify: `internal/store/migrate.go`
- Create: `internal/store/work_progress.go`
- Create: `internal/store/work_progress_test.go`

- [ ] 写失败测试：合法的 `workId/unitId/relativePage` 写入后可按用户和 Work 读取。
- [ ] 写失败测试：同一 Work 的较新 sequence 可正常回退到较小页码，旧 sequence 不得覆盖。
- [ ] 写失败测试：Unit 不属于 Work、Comic 不匹配、页码越界时事务不写入。
- [ ] 新增 `UserWorkProgress` 表和索引，字段与设计文档一致。
- [ ] 实现 `UpsertUserWorkProgressTx` 与 `GetUserWorkProgress`。
- [ ] 运行 `go test ./internal/store -run 'WorkProgress|ReadingProgress' -count=1`。
- [ ] 提交 `feat(progress): persist user work cursors`。

### Task 2: 统一 reading activity 事务

**Files:**
- Modify: `internal/store/comic_stats.go`
- Modify: `internal/store/reading_progress_test.go`
- Modify: `internal/handler/stats.go`
- Modify: `internal/handler/handler_test.go`

- [ ] 写失败测试：activity 携带 Work 上下文时，服务端根据 Unit 计算 canonical absolute page，而不是信任客户端 absolute page。
- [ ] 写失败测试：同一大 ZIP 切换内部 Unit 时记录正确话和绝对页。
- [ ] 写失败测试：不带 Work 上下文的旧请求保持原行为。
- [ ] 扩展 `readingActivityRequest`，加入 `workId/unitId/relativePage`。
- [ ] 在单事务中更新 ReadingSession、Comic、UserComicState 与 UserWorkProgress。
- [ ] 返回 canonical progress JSON。
- [ ] 运行 handler/store 相关测试。
- [ ] 提交 `feat(progress): validate work-aware reading activity`。

### Task 3: Work 查询优先使用显式游标

**Files:**
- Modify: `internal/service/work.go`
- Modify: `internal/service/work_test.go`
- Modify: `internal/store/comic_query.go`

- [ ] 写失败测试：存在 UserWorkProgress 时 `continueUnitId/continuePage` 必须使用显式游标。
- [ ] 写失败测试：允许显式游标回退到较前 Unit。
- [ ] 写失败测试：没有显式游标时继续使用现有 Comic 推导。
- [ ] 将用户 Work 游标注入 Work 构建流程。
- [ ] 保留惰性兼容旧数据，不批量改写漫画记录。
- [ ] 运行 `go test ./internal/service ./internal/store -count=1`。
- [ ] 提交 `feat(work): project explicit reading cursors`。

### Task 4: Android 可靠同步器与本地 outbox

**Files:**
- Modify: `flutter_app/lib/data/api/comic_api.dart`
- Rewrite: `flutter_app/lib/data/services/reading_activity_tracker.dart`
- Create: `flutter_app/lib/data/services/reading_progress_outbox.dart`
- Create: `flutter_app/test/data/services/reading_activity_tracker_test.dart`
- Create: `flutter_app/test/data/services/reading_progress_outbox_test.dart`

- [ ] 写失败测试：快速翻页期间最多一个请求在途，最终只确认最新页。
- [ ] 写失败测试：切换 Unit 时立即 flush，提交 `workId/unitId/relativePage`。
- [ ] 写失败测试：网络失败后只持久化每个 server/user/work 的最新事件。
- [ ] 写失败测试：应用恢复时重试，服务端成功确认后删除 outbox。
- [ ] 扩展 `recordReadingActivity` 返回 canonical progress。
- [ ] 将 tracker 改为串行 drain loop，页面防抖 350ms、心跳 10 秒。
- [ ] finish 必须等待最后事件完成或落入 outbox，不再静默丢失。
- [ ] 运行 Flutter 单元测试。
- [ ] 提交 `feat(android): make reading progress durable`。

### Task 5: Android 阅读器页码统一与详情即时刷新

**Files:**
- Modify: `flutter_app/lib/features/reader/comic_reader_screen.dart`
- Modify: `flutter_app/lib/features/reader/work_reader_context.dart`
- Modify: `flutter_app/lib/features/reader/reader_dispatch_screen.dart`
- Modify: `flutter_app/lib/features/detail/work_detail_screen.dart`
- Modify: `flutter_app/lib/data/providers/work_provider.dart`
- Modify: `flutter_app/lib/widgets/continue_reading.dart`
- Create: `flutter_app/test/features/reader/work_progress_mapping_test.dart`

- [ ] 写失败测试：大 ZIP 第 20 话第 5 页提交的是该 Unit 的 relativePage=4，由服务端映射为正确 absolutePage。
- [ ] 写失败测试：主动返回第 2 话后继续阅读允许回退。
- [ ] 所有阅读模式统一通过 `WorkReaderContext` 生成进度事件。
- [ ] 同一 physical comic 切换内部 Unit 时不再创建会提交错误页码的孤立 tracker。
- [ ] reader 正常退出后 invalidate Work 详情、Work 列表和继续阅读 provider。
- [ ] 详情按钮显示 `继续阅读 · <话/卷> · 第N页`。
- [ ] 同步失败只显示轻量“待同步”状态。
- [ ] 运行 widget/model 测试。
- [ ] 提交 `fix(android): refresh canonical continue reading state`。

### Task 6: Web 阅读器使用同一作品游标

**Files:**
- Modify: `frontend/src/app/reader/[id]/page.tsx`
- Modify: `frontend/src/api/works.ts`
- Create: `frontend/src/lib/reading-progress.ts`
- Create: `frontend/src/lib/reading-progress.test.ts`

- [ ] 写失败测试：有 Work 上下文时 payload 包含 work/unit/relativePage。
- [ ] 写失败测试：无 Work 上下文时保持旧 payload。
- [ ] Web 切话、退出和后台化时 flush。
- [ ] 服务端响应后使 Work 查询缓存失效。
- [ ] 运行前端测试和类型检查。
- [ ] 提交 `feat(web): sync work-aware reading progress`。

### Task 7: 回归、迁移、版本与部署

**Files:**
- Modify: `flutter_app/pubspec.yaml`
- Modify: `CHANGELOG.md`
- Modify: `docs/superpowers/specs/2026-08-09-realtime-reading-progress-design.md`

- [ ] 用测试复现线上“1308 被 8 覆盖”的序列，并验证修复后错误 Unit 页码被拒绝。
- [ ] 运行 `go test ./...`。
- [ ] 运行 Flutter `flutter analyze` 与全部测试。
- [ ] 运行前端测试、lint、typecheck 和生产构建。
- [ ] 构建新 Docker 镜像，在 NAS 测试容器执行数据库迁移与 API 冒烟测试。
- [ ] 构建并签名新版 APK，提升版本号。
- [ ] 更新 NAS 6680 容器前备份数据库，部署后验证健康、旧进度、合法回退和实时刷新。
- [ ] 推送 `ymreader-final` 到 GitHub并记录镜像标签、APK 路径和 SHA256。
- [ ] 提交 `release: realtime reading progress`。
