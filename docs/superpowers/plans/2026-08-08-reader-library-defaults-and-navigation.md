# 阅读器默认设置与导航修复 Implementation Plan

> **For agentic workers:** Execute task-by-task with regression tests and verification checkpoints.

**Goal:** 修复阅读返回、跨话页码、详情目录、首页收藏、书库默认阅读设置、连续阅读和版本显示。

**Architecture:** 客户端保留 Work → WorkUnit 模型；阅读设置采用作品 > 书库 > 全局的本地回退链；阅读器切换话数时使用新的状态键强制重建，避免旧控制器污染新话。

**Tech Stack:** Flutter、Riverpod、go_router、SharedPreferences、Flutter test。

---

### Task 1: 固化数据与设置行为测试

**Files:**
- Modify: `flutter_app/test/data/models/work_reader_context_test.dart`
- Modify: `flutter_app/test/widgets/reader_settings_test.dart`
- Create: `flutter_app/test/data/models/reader_navigation_test.dart`

- [ ] 添加测试：目标话默认相对页为 0，且不会继承上一话页码。
- [ ] 添加测试：作品设置优先于书库设置，书库设置优先于全局设置。
- [ ] 添加测试：WorkUnit 封面 URL 可正确生成。
- [ ] 运行新增测试确认先失败。

### Task 2: 修复返回栈和跨话状态

**Files:**
- Modify: `flutter_app/lib/features/reader/comic_reader_screen.dart`
- Modify: `flutter_app/lib/features/reader/reader_dispatch_screen.dart`
- Modify: `flutter_app/lib/features/reader/work_reader_context.dart`

- [ ] 阅读器返回改为 pop 当前阅读路由。
- [ ] 路由切换时按 `comicId/unitId/initialPage` 重建控制器和活动跟踪器。
- [ ] `_goToUnit` 明确传递目标话 `startPage`，目标相对页为 0。
- [ ] 运行导航回归测试。

### Task 3: 恢复详情页两种目录模式

**Files:**
- Modify: `flutter_app/lib/features/detail/work_detail_screen.dart`
- Modify: `flutter_app/lib/data/models/work.dart`
- Modify: `flutter_app/test/data/models/work_model_test.dart`

- [ ] 增加列表/封面网格切换状态和按钮。
- [ ] 网格模式使用章节自己的封面，逐级回退到首图/作品封面。
- [ ] 两种模式都使用同一阅读目标路由。
- [ ] 运行详情页模型和 Widget 测试。

### Task 4: 移除首页收藏按钮

**Files:**
- Modify: `flutter_app/lib/widgets/work_card.dart`
- Modify: `flutter_app/lib/features/home/home_screen.dart`

- [ ] 删除首页 WorkCard 的收藏覆盖层和列表尾部按钮。
- [ ] 保留详情页收藏操作。
- [ ] 运行首页 smoke test。

### Task 5: 实现书库级默认设置

**Files:**
- Modify: `flutter_app/lib/widgets/reader_settings_panel.dart`
- Modify: `flutter_app/lib/features/settings/settings_screen.dart`
- Modify: `flutter_app/lib/app/router.dart`
- Create: `flutter_app/lib/features/settings/library_reader_defaults_screen.dart`

- [ ] ReaderSettings.load 增加 fallbackScope。
- [ ] 设置页展示可访问漫画库并打开独立设置面板。
- [ ] 阅读器按 Work.libraryId 使用书库设置作为回退。
- [ ] 自动播放默认改为关闭并移除阅读器入口。

### Task 6: 实现连续阅读

**Files:**
- Modify: `flutter_app/lib/widgets/reader_settings_panel.dart`
- Modify: `flutter_app/lib/features/reader/comic_reader_screen.dart`
- Modify: `flutter_app/lib/features/reader/work_reader_context.dart`
- Modify: `flutter_app/test/data/models/reader_navigation_test.dart`

- [ ] 增加作品级 continuousReading 设置。
- [ ] 长条模式按 WorkUnit 顺序加载并追加下一话。
- [ ] 单页/双页模式到边界时切换下一话。
- [ ] 更新当前话、物理页和作品进度。
- [ ] 开关关闭时不跨话。

### Task 7: 修复版本显示并发布

**Files:**
- Modify: `flutter_app/pubspec.yaml`
- Modify: `flutter_app/lib/features/settings/settings_screen.dart`
- Modify: `flutter_app/pubspec.lock`

- [ ] 版本提升到 `1.2.1+42`。
- [ ] 设置页读取实际 package version。
- [ ] 运行 `flutter test`、`flutter analyze`、release build、apksigner verify。
- [ ] 复制 `F:\meihua\YMReader-Android-1.2.1+42.apk`。
- [ ] 提交并推送 `ymreader-final`。
