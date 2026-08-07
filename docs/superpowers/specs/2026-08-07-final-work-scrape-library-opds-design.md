# YMReader 最终版 Work 刮削、书库管理与 OPDS 设计

## 目标

在不改变真实漫画文件、不操作正式环境 `6680` 的前提下，让测试环境 `17680` 的漫画业务统一使用：

```text
Library -> Work -> Unit -> Page
```

`Comic` 仅表示物理文件或物理目录，不再作为作品级统计、刮削、OPDS 顶层展示的实体。

## 一、元数据刮削

- 漫画书库按 `Work` 统计和刮削，一部作品一个任务。
- 小说保持按物理书籍 `Comic` 处理。
- `/api/metadata/stats`、`/api/metadata/batch`、`/api/metadata/ai-batch`、`/api/metadata/batch-selected` 使用同一目标发现逻辑。
- `missing` 对漫画检查 `LogicalWork.MetadataSource` 与作品封面，不检查章节 Comic。
- 漫画搜索词只使用 Work 标题和作者，不使用章节文件名。
- SSE 进度和结果统一返回 `entityType`、`entityId`、`title`；兼容保留旧字段，但前端不再依赖 `filename` 展示作品。
- 书库“立即刮削”创建同一套后台任务，元数据页面可观察、停止和查看结果。
- 同一时刻扫描与刮削互斥；请求取消后停止后续搜索和写入。

## 二、书库管理

- 漫画书库显示 `workCount`、`unitCount`、`fileCount`。
- 小说书库继续显示书籍/文件数量。
- `lastScanAdded` 保留物理新增文件语义，同时新增 `lastScanAddedWorks` 表示新增作品。
- 恢复原项目书库菜单中的删除、编辑、启用/禁用和扫描功能。
- 删除书库只删除数据库索引和派生缓存，不删除源文件。
- 自动扫描与自动刮削彻底分离；扫描后不触发在线刮削。

## 三、OPDS

顶层目录始终一部 Work 一条 Navigation Entry。点击 Work 后返回自适应 Acquisition Feed：

1. **一个作品目录中有多个 CBZ/ZIP/PDF 文件**
   - 每个物理文件成为一个 Unit acquisition。
   - 完整返回所有卷/话，保持稳定 Unit ID 和自然排序。
   - 不生成“连续阅读（整部）”。

2. **单个完整 ZIP/CBZ**
   - 即使内部存在章节目录，OPDS 只发布原始完整文件。
   - Nowen Web/API 仍可保留内部 Unit 目录。
   - 不为 OPDS 动态拆分为每话虚拟 CBZ。

3. **散图章节目录**
   - 每个章节 Unit 使用 PSE 或按需虚拟 CBZ。
   - 单个平铺图片目录作为一个全文 Unit。

4. **PDF**
   - 直接发布原始 PDF。

OPDS 不再发布 Work 级动态巨型 CBZ，不混合“整部”和章节两种 acquisition。所有下载、封面和流式接口必须支持正确的认证、HEAD、GET 和 Range 语义。

## 四、OPDS 元数据与封面

- Work Navigation Entry 使用 Work 标题、作者、简介、出版社、语言、标签、分类和作品封面。
- Unit Entry 仅追加卷/话标题、页数、排序和 Unit 封面，不单独在线刮削。
- 高清封面与缩略图使用不同 URL。
- 缩略图在封面下载/更新时预生成，保持比例、不裁剪，使用兼容性良好的 JPEG。
- 封面查询按 Work ID 直接读取 LogicalWork 并验证权限，不为每张封面重建完整 Work 索引。
- 本地封面使用 ETag、Last-Modified 和版本化 URL 长期缓存。
- 远程封面先下载到本地，OPDS 不把客户端重定向到第三方站点。

## 五、兼容与安全边界

- 保留原项目未涉及本次目标的功能和 API。
- `/api/opds/series` 只作为旧书签兼容入口，不恢复旧 Series 聚合逻辑。
- 不创建假漫画、测试 CBZ、`Manga/manga/漫画` 空目录。
- 不移动、删除或修改真实漫画。
- 只构建和部署 `17680`，绝不操作 `6680`。

## 六、验收标准

- 大陆漫画元数据页面总数约为 Work 数，而不是 6069。
- 一部多话漫画只产生一个在线刮削任务。
- 书库“立即刮削”与元数据页面显示同一任务。
- 书库删除按钮可见且只删除索引。
- 多 CBZ Work 的 OPDS 详情完整列出全部 Unit。
- 单个完整 ZIP 的 OPDS 详情只发布一个原始文件。
- OPDS 不出现“连续阅读（整部）”。
- 封面列表首屏使用缩略图，重复访问命中缓存。
- 后端、前端、集成和真实只读验证全部通过后才构建测试镜像。
