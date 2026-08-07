# YMReader Final Work Scrape, Library, and OPDS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 Work 级元数据刮削、恢复完整书库管理、实现按物理布局自适应的 OPDS，并优化作品封面加载。

**Architecture:** 以现有 `service.Work` 和持久化 `LogicalWork` 为唯一作品模型。新增共享的元数据目标发现与执行层，所有入口调用同一任务；OPDS 顶层只发布 Work，详情根据 Work 的物理来源决定发布物理 acquisition 或必要的虚拟 Unit。

**Tech Stack:** Go、Gin、SQLite、React、TypeScript、Atom/OPDS 1.2、Docker。

---

### Task 1: 固定 Work 级元数据统计与目标发现

**Files:**
- Modify: `internal/handler/metadata_batch_handler.go`
- Modify: `internal/handler/metadata_library_handler.go`
- Create: `internal/handler/metadata_work_batch_test.go`

- [ ] 编写失败测试：两话同作品在漫画统计中只算一个目标，小说仍逐书计算。
- [ ] 运行目标测试并确认因当前 Comic 级统计而失败。
- [ ] 提取共享目标发现函数，返回 Work/Comic 统一目标。
- [ ] 让 Stats、Batch、AIBatch 和 BatchSelected 复用目标发现。
- [ ] 运行测试并提交。

### Task 2: 统一 Work 级批处理和取消语义

**Files:**
- Modify: `internal/handler/metadata_batch_handler.go`
- Modify: `internal/handler/manual_work_scrape.go`
- Modify: `internal/service/work_scrape_queue.go`
- Modify: `internal/handler/routes_metadata.go`
- Test: `internal/handler/metadata_work_batch_test.go`
- Test: `internal/service/work_scrape_queue_test.go`

- [ ] 编写失败测试：漫画搜索使用 Work 标题、进度按 Work 输出、取消后不再写入。
- [ ] 确认测试失败。
- [ ] 实现统一 Work 执行器和 SSE 事件模型。
- [ ] 将书库立即刮削接入同一队列和状态查询。
- [ ] 验证扫描/刮削互斥、取消和 SQLite 重试。
- [ ] 运行测试并提交。

### Task 3: 修复元数据前端与书库管理

**Files:**
- Modify: `frontend/src/lib/stores/scraper-types.ts`
- Modify: `frontend/src/lib/stores/scraper-batch-actions.ts`
- Modify: `frontend/src/app/scraper/page.tsx`
- Modify: `frontend/src/api/libraries.ts`
- Modify: `frontend/src/components/LibraryManagementPanel.tsx`
- Modify: `internal/handler/library.go`
- Modify: `internal/store/library_store.go`
- Test: `frontend/scripts/test-work-metadata-frontend.mjs`
- Create: `frontend/scripts/test-library-management-work-counts.mjs`

- [ ] 编写失败测试：前端进度按 Work 展示、书库删除菜单存在、漫画统计字段分离。
- [ ] 确认测试失败。
- [ ] 更新 SSE 类型和任务状态展示。
- [ ] 增加 workCount/unitCount/fileCount/lastScanAddedWorks DTO。
- [ ] 恢复并验证删除、编辑、启用/禁用、扫描菜单。
- [ ] 运行前端测试并提交。

### Task 4: 固定 OPDS 自适应发布行为

**Files:**
- Modify: `internal/handler/opds_work_handler_test.go`
- Modify: `internal/service/opds_test.go`
- Modify: `internal/handler/opds_handler.go`
- Modify: `internal/service/opds.go`

- [ ] 编写失败测试：多物理 CBZ 返回全部 Unit 且没有连续整部项。
- [ ] 编写失败测试：单个内部章节 ZIP 只返回一个原始 acquisition。
- [ ] 编写失败测试：散图 Unit 仍可使用虚拟 CBZ/PSE。
- [ ] 编写失败测试：分页、唯一 ID、HEAD、GET 和 Range 正确。
- [ ] 确认测试均因当前实现失败。
- [ ] 实现自适应 acquisition 策略并删除 WorkContinuous 路由。
- [ ] 运行测试并提交。

### Task 5: 优化 OPDS 元数据与封面

**Files:**
- Modify: `internal/handler/opds_handler.go`
- Modify: `internal/service/opds.go`
- Modify: `internal/service/work.go`
- Modify: `internal/archive/thumbnail.go`
- Test: `internal/handler/opds_work_handler_test.go`
- Test: `internal/service/opds_test.go`

- [ ] 编写失败测试：image 与 thumbnail URL 分离。
- [ ] 编写失败测试：封面请求按 Work ID 直接查询，不加载全部 Work。
- [ ] 编写失败测试：本地缩略图长期缓存，远程封面不重定向。
- [ ] 确认测试失败。
- [ ] 实现预生成缩略图、直接权限查询和版本化缓存。
- [ ] 运行测试并提交。

### Task 6: 全量回归与测试部署

**Files:**
- Modify: `scripts/verify_unified_work_container.py`
- Modify: `docs/API.md`
- Modify: `frontend/public/api-doc.html`

- [ ] 更新真实格式验证：多 CBZ、整体 ZIP、平铺 CBZ、散图目录、PDF。
- [ ] 运行全部 Go 测试、前端测试和构建。
- [ ] 运行 SQLite 并发请求验证并确认没有 SQLITE_BUSY。
- [ ] 构建全新版本镜像。
- [ ] 仅替换 `17680` 测试容器，保持真实漫画只读挂载。
- [ ] 验证登录、书库管理、元数据页面、OPDS XML、封面缓存和阅读链接。
- [ ] 确认 `6680` 未被操作并提交最终版本。
