# YmReader 智能漫画作品模型改造设计

日期：2026-08-05
目标分支：ymreader-work-model
上游项目：cropflre/nowen-reader
Fork：YM040923/ymreader

## 1. 目标

把 NowenReader 从“一个扫描文件等于一本 Comic，再用 Series 聚合”的体验，改造成以“作品”为核心的漫画库：

```text
Library 书库
  Work 作品
    Unit 阅读单元：全本 / 第几话 / 第几卷 / 番外 / 特典
      Page 页
```

用户不需要把漫画强制整理成唯一格式。服务端应兼容常见漫画资源形态，并统一识别为一部作品及其目录。

必须支持的代表场景：

```text
D:\临时\恶魔X天使 不能友好相处.zip
```

识别为：

```text
作品：恶魔X天使 不能友好相处
目录：全本
```

点击作品应进入详情页，而不是直接进入阅读器。

## 2. 非目标

本阶段不重做以下功能：

- AI 对话、推荐、阅读洞察
- 标签、分类、用户权限体系
- 小说、有声书、电子书阅读能力
- Docker 部署方式
- 现有刮削源本身
- 阅读器 UI 大改
- 安卓端视觉风格重做

只围绕：扫描识别、作品模型、目录、阅读进度、书库展示、OPDS 与客户端对接。

## 3. 现状问题

当前 NowenReader 的主要模型是：

```text
Comic = 一个实际文件或散图文件夹
ComicSeries = 多个 Comic 的目录聚合
ComicGroup = 用户自定义合集
```

这导致：

1. 国漫每话 CBZ 会先被扫成很多本 Comic。
2. 日漫每卷 CBZ 也会先被扫成很多本 Comic。
3. Series 只是后续折叠视图，底层仍是一堆单本。
4. OPDS、搜索、统计、安卓端容易把单话平铺出来。
5. 单个大压缩包虽然能作为一本 Comic，但没有真正的作品详情页和目录语义。

## 4. 目标模型

新增主模型，暂命名为 `Work` 与 `WorkUnit`。

### Work

表示一部漫画作品。

字段建议：

```text
id
libraryId
rootRelativePath
title
sortTitle
coverUrl
coverUnitId
metadata fields：author / publisher / year / description / language / genre / source / rating
contentType：comic / novel / mixed，初期主要 comic
createdAt
updatedAt
metadataLocked
manualLocked
```

### WorkUnit

表示作品下的一个阅读单元。

字段建议：

```text
id
workId
comicId，可选，兼容旧 Comic 文件记录
relativePath
title
displayLabel
unitKind：full / chapter / volume / extra / special / unknown
volumeNumber，可空
chapterNumber，可空
sortIndex
pageCount
fileSize
coverUrl，可空
createdAt
updatedAt
```

### WorkProgress

记录用户对作品级的续读位置。

```text
userId
workId
unitId
pageIndex
updatedAt
```

重点：续读不是只记某个 Comic 的页数，而是记：

```text
哪部作品 + 哪个阅读单元 + 哪一页
```

## 5. 文件结构兼容规则

扫描器不能只支持一种格式，要做智能识别。

### 5.1 单压缩包 / 单文件

```text
恶魔X天使 不能友好相处.zip
辉夜大小姐 Vol.01.cbz
某漫画.pdf
```

默认识别为一个 Work，下面一个 Unit。

如果文件名包含明显卷号或话号：

```text
辉夜 Vol.01.cbz
大王饶命 第001话.cbz
```

且同目录下存在同名同族文件，则合并到同一个 Work。否则作为单作品 + 单 Unit。

### 5.2 作品文件夹 + 多个压缩包

```text
大王饶命/
  第001话.cbz
  第002话.cbz
```

识别为：

```text
Work：大王饶命
Unit：第001话、第002话
```

### 5.3 作品文件夹 + 多个散图章节文件夹

```text
大王饶命/
  第001话/
    001.jpg
    002.jpg
  第002话/
    001.jpg
    002.jpg
```

识别为一个 Work，多 Unit。

### 5.4 单个散图文件夹

```text
短篇漫画/
  001.jpg
  002.jpg
```

识别为一个 Work，一个 Unit：全本。

### 5.5 卷 / 话混合

```text
某漫画/
  Vol.01/
    Ch.001.cbz
    Ch.002.cbz
  Vol.02/
    Ch.003.cbz
```

识别为一个 Work，多个 Unit。初期 UI 可扁平显示，但排序要保留卷号与话号：

```text
第01卷 第001话
第01卷 第002话
第02卷 第003话
```

后续再做二级目录。

### 5.6 混合文件与文件夹

```text
漫画名/
  第001话.cbz
  第002话/
    001.jpg
    002.jpg
  番外.cbz
```

识别为一个 Work，多 Unit。

### 5.7 特殊内容

识别这些关键词：

```text
番外
特典
公告
请假条
庆典
设定集
单行本特典
extra
special
omake
bonus
```

这些归为 `extra` 或 `special`，默认排在正篇后，避免干扰正文排序。

## 6. 智能识别模块

新增独立模块，尽量减少对官方原文件的侵入。

建议路径：

```text
internal/workmodel/
  detector.go
  resolver.go
  sorter.go
  importer.go
  types.go
```

### StructureDetector

输入文件系统节点，输出候选结构：

```text
SingleFileWork
SingleImageFolderWork
FolderWithUnits
NestedVolumeChapterWork
MixedWork
StandaloneComic
```

### WorkResolver

负责推断作品名：

- 优先使用父文件夹名。
- 单文件时使用去扩展名后的文件名。
- 多个同族文件时剥离 `Vol.xx`、`Ch.xx`、`第xx话`、`第xx卷` 等后缀。

### UnitResolver

负责推断 Unit 类型、标题和编号：

- `第001话` / `Ch.001` / `Chapter 001` → chapter
- `第01卷` / `Vol.01` / `Volume 01` → volume
- `番外` / `特典` → extra / special
- 无编号单文件 → full

### Sorter

排序优先级：

1. 正篇 chapter / volume
2. 番外 extra
3. 特典 special
4. unknown

正篇内部按自然数字排序，支持中文数字、阿拉伯数字、小数话、范围话。

## 7. 与旧 Comic / Series 的关系

不物理删除旧表，避免大规模破坏和上游同步冲突。

但主流程不再依赖 Series。

策略：

```text
Comic 继续作为“底层文件记录 / 阅读资源记录”存在
Work 是用户看到的一部作品
WorkUnit 关联到底层 Comic 或实际路径
ComicSeries 保留兼容，但从主 UI 隐藏，不再作为普通漫画组织核心
ComicGroup 可保留为用户自定义收藏集
```

这样可以兼容：

- 旧 API
- 旧阅读器页图接口
- 旧缩略图逻辑
- 旧刮削逻辑的一部分
- 官方后续更新

## 8. API 设计

新增 Work API，不直接替换旧 Comic API。

```http
GET /api/works
GET /api/works/:id
GET /api/works/:id/units
GET /api/works/:id/continue
PUT /api/works/:id/progress
POST /api/works/rebuild
POST /api/works/:id/scrape-metadata
POST /api/works/:id/apply-metadata
```

阅读页初期仍可复用：

```http
GET /api/comics/:id/pages
GET /api/comics/:id/page/:pageIndex
```

但客户端进入阅读器时传入：

```text
workId
unitId
comicId
```

以便上一话、下一话和续读都能回到作品维度。

## 9. 前端改造范围

### 书库页

显示 Work 卡片，不显示散落 Comic。

单压缩包也显示为 Work 卡片。

### 详情页

详情页显示：

- 封面
- 标题
- 简介
- 标签 / 分类
- 立即阅读 / 继续阅读
- 目录 Unit 列表

### 阅读器

不重做 UI，只补对 WorkUnit 的理解：

- 下一话
- 上一话
- 返回作品详情
- 目录定位当前 Unit
- 退出或翻页时更新 WorkProgress

## 10. OPDS 设计

新增作品维度 OPDS：

```text
/api/opds/works
/api/opds/works/:id
/api/opds/works/:id/units
```

默认 OPDS 首页优先提供作品入口，而不是每话平铺。

兼容旧接口：

```text
/api/opds/all
/api/opds/series
```

但可以在设置里选择默认 OPDS 模式：

```text
works 优先
all 文件优先
legacy series
```

## 11. 安卓端改造范围

安卓端后续只改数据接入，不乱改视觉风格。

需要改：

- 首页继续阅读按 Work 合并
- 书库显示 Work 卡片
- 搜索结果显示 Work，不显示每话
- 详情页读取 Work + Unit
- 阅读器上一话 / 下一话基于 WorkUnit
- 续读用 WorkProgress

不改：

- 主视觉风格
- 已确认满意的卡片比例和布局
- 无关设置项

## 12. 数据迁移策略

首次启用 Work 模型时：

1. 不删除 Comic。
2. 扫描现有 Comic 的 `libraryId` 和 `relativePath`。
3. 根据路径和命名生成 Work / WorkUnit。
4. 如果已有 Series 元数据，可以迁移到 Work。
5. 如果已有阅读进度，迁移为 WorkProgress：
   - 单本 Comic → 对应 Work 的对应 Unit
   - Series 成员 Comic → 对应 WorkUnit
6. 迁移可重复执行，要求幂等。

## 13. 上游同步策略

Fork 结构：

```text
main                 跟随 cropflre/nowen-reader main
ymreader-work-model  用户定制分支
```

同步方式：

```bash
git fetch origin
git checkout main
git merge origin/main

git checkout ymreader-work-model
git rebase main
```

改动尽量集中在新增模块和新增 API，减少直接修改官方核心文件。

## 14. 第一阶段验收标准

用两部代表漫画测试：

```text
恶魔X天使 不能友好相处.zip
大王饶命/
  第001话.cbz
  第002话.cbz
败犬女主太多了/
  Vol.01.cbz
  Vol.02.cbz
```

必须达成：

1. 书库页只显示作品，不显示单话/单卷散本。
2. 单压缩包也有详情页。
3. 作品详情页有目录。
4. 立即阅读从第一个 Unit 第 0 页开始。
5. 继续阅读精确到 Unit + Page。
6. 上一话 / 下一话按 Unit 顺序可用。
7. OPDS works 模式不平铺每话。
8. 原有 Comic 图片读取接口可继续工作。
9. 旧 Series 不作为主入口出现。
10. 不影响小说、用户、权限、标签、分类、AI 等无关功能。

## 15. 风险

### 风险 1：直接替换主列表可能影响旧前端

解决：新增 `/api/works`，先让新页面用 Work；旧 `/api/comics` 保留。

### 风险 2：数据库迁移复杂

解决：Work 表新增，不删除旧表；迁移幂等，可重建。

### 风险 3：排序误判

解决：Sorter 单独做测试集，覆盖中文、日文、英文、阿拉伯数字、番外、特典。

### 风险 4：官方更新冲突

解决：新增模块优先，减少改原文件；每阶段小提交。

## 16. 实施顺序建议

1. 写 Work/Unit 识别器纯函数和测试。
2. 新增数据库表和 store 层。
3. 从现有 Comic 生成 Work/Unit。
4. 新增 `/api/works` 查询接口。
5. Web 书库页切换到 Work。
6. 详情页接 Work + Unit。
7. 阅读器接 WorkProgress 和 Unit 上下话。
8. OPDS works 模式。
9. 安卓端接新 API。
10. 回归测试并打包。
