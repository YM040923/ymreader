# Android Work 模型与文件夹统计适配设计

## 目标

让 Flutter 安卓客户端完整适配当前服务端的
`Library → Work → WorkUnit → Comic` 模型，删除已经废弃的合集功能，并修复文件夹统计页面。

## 范围

### 1. 删除合集客户端功能

- 从设置页面移除“合集管理”。
- 删除 `/collections` 与 `/group/:id` 路由。
- 删除仅服务于合集的页面、组件、Provider、API 方法和数据模型。
- 不修改收藏、标签、分类、书库和 Work 详情功能。

### 2. 文件夹统计

- `/stats/folder-tree` 按服务端当前协议直接解析 JSON 数组。
- 默认使用 `scope=work`，每部作品只统计和展示一次。
- 提供“按作品 / 按物理文件”切换。
- `/stats/files` 提供当前 scope 的摘要数据。
- 文件夹节点展示作品数、总大小和页数。
- Work 文件项进入 `/work/:id`；物理漫画进入 `/comic/:id`。
- 请求失败、空数据和刷新状态必须正常展示。

### 3. 扫描规则适配

- AI 固定按 Work 调用一次并写入 LogicalWork。
- 删除“识别范围”“写回单卷”“同步合集”和“虚拟分组”选项。
- 保留：
  - 总开关；
  - 触发时机；
  - 最低置信度；
  - 覆盖已有标题；
  - Work 感知的硬链接或移动目录整理；
  - 过滤器、预览、执行和日志。
- 推荐预设不自动打开线上总开关；它只配置子功能，是否启用由用户决定。
- 执行结果区分 Work 总数与物理文件总数。

### 4. 版本和发布

- 版本更新为 `1.2.2+43`。
- 沿用现有签名配置构建 release APK。
- 不修改服务端数据库、NAS 漫画文件或书库配置。

## 数据流

```text
Android
  ├─ GET /stats/folder-tree?scope=work|physical
  │    └─ List<FolderTreeNode>
  ├─ GET /stats/files?scope=work|physical
  │    └─ FileStatsSummary
  └─ GET/PUT/POST /scan-rules/*
       └─ Work 级扫描规则与执行结果
```

客户端不再自行推断合集或作品归属，作品身份完全以服务端 Work ID 为准。

## 错误处理

- API 返回数组或对象类型不匹配时显示明确错误，而不是运行时强制转换崩溃。
- 数值字段接受 `int`、`double` 和数字字符串。
- scope 切换时取消旧结果的展示，避免不同请求结果混合。
- 已移除的合集深链不再注册路由。

## 测试

- API 解码测试：
  - Work 文件夹树数组；
  - physical 文件夹树数组；
  - 文件统计摘要；
  - 空数组和异常字段。
- Widget 测试：
  - scope 切换；
  - Work 点击路由；
  - 空状态与错误状态。
- 扫描规则测试：
  - 请求载荷不再包含 `organize`、`applyToGroup`；
  - UI 不再出现逐文件和合集选项。
- 运行 `flutter test`、`flutter analyze` 和 release APK 构建。

## 非目标

- 不重新设计阅读器。
- 不修改服务端 Work 聚合算法。
- 不修复项目中与本次改动无关的既有 lint 信息。
- 不恢复任何合集或 ComicGroup 兼容入口。
