# API 文档

NowenReader 提供完整的 RESTful API，所有功能均可通过 API 调用。

> 🔒 = 需要认证（登录用户） &emsp; 🔒管理员 = 需要管理员权限

## Base Path 与请求地址

本文档中的接口路径统一写为业务路径 `/api/...`。当服务配置了 `BASE_PATH` 时，实际请求地址为：

```text
服务地址 + BASE_PATH + API 路径
```

示例：

```text
根路径部署：
http://localhost:6680/api/health
http://localhost:6680/api/opds

BASE_PATH=/reader：
https://example.com/reader/api/health
https://example.com/reader/api/opds
```

客户端的服务器地址应包含 Base Path，例如 `https://example.com/reader`，后续再拼接 `/api/...`。不要同时让反向代理剥离 `/reader`，也不需要额外暴露根路径 `/api`。

## 🔐 认证

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| POST | `/api/auth/register` | 注册（限流） |
| POST | `/api/auth/login` | 登录（限流） |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/auth/me` | 当前用户信息 |
| GET | `/api/auth/users` | 用户列表 🔒管理员 |
| POST | `/api/auth/users` | 创建用户 🔒管理员 |
| PUT | `/api/auth/users` | 更新用户 🔒管理员 |
| DELETE | `/api/auth/users` | 删除用户 🔒管理员 |
| GET | `/api/auth/api-keys` | 当前用户的 API Key 列表 🔒浏览器会话 |
| POST | `/api/auth/api-keys` | 创建 API Key 🔒浏览器会话 |
| DELETE | `/api/auth/api-keys/:id` | 撤销一个 API Key 🔒浏览器会话 |
| DELETE | `/api/auth/api-keys` | 撤销当前用户全部 API Key 🔒浏览器会话 |
| GET | `/api/admin/users/:id/api-keys` | 查看指定用户的 API Key 元数据 🔒管理员浏览器会话 |
| DELETE | `/api/admin/users/:id/api-keys` | 撤销指定用户全部 API Key 🔒管理员浏览器会话 |

### API Key 认证

除登录、注册等公开接口外，需要认证的接口支持浏览器 Session Cookie 或 API Key。API Key 通过请求头发送：

```http
Authorization: Bearer nwr_<key-id>_<secret>
```

- API Key 绑定到创建它的用户，不单独保存角色、用户组或书库权限。
- 每次请求都使用用户当前的角色与 `canView`、`canDownload`、`canManage` 权限；管理员 Key 拥有该管理员当时的完整权限。
- 用户权限、用户组、角色或书库授权发生变化后，已有 Key 会立即按新权限生效。
- 请求只要携带 `Authorization` 头，就优先按 Bearer Key 认证；Key 错误、过期或已撤销时返回 `401`，不会退回 Cookie。
- 不支持通过 URL 查询参数传递 API Key。
- API Key 管理接口仅接受浏览器 Session Cookie。API Key 本身不能查看、创建或撤销 Key。
- 服务端仅保存 Key 的 SHA-256 摘要；完整 Key 只在创建成功的响应中返回一次。
- 删除用户时会同时删除该用户的全部 API Key。修改密码不会自动撤销 Key，可使用全部撤销接口主动失效。

OPDS 1.2 接口还支持 HTTP Basic Auth，方便不支持 Bearer 请求头的目录客户端：

```text
用户名：API Key 所属用户的用户名
密码：完整 API Key（nwr_...）
```

Basic Auth 仅用于 `/api/opds` 及其子路径；用户名必须与 API Key 所属用户一致。不要填写账户登录密码，也不支持把 API Key 放入 URL 查询参数。在不可信局域网或公网使用时必须通过 HTTPS 传输。

#### 创建 API Key

```http
POST /api/auth/api-keys
Content-Type: application/json

{
  "name": "家庭自动化",
  "currentPassword": "当前账户密码",
  "expiresInDays": 365
}
```

`name` 为 1-64 个字符。`expiresInDays` 省略时默认 365 天，传 `0` 表示永不过期，最大为 3650 天。创建成功返回 `201`：

```json
{
  "apiKey": {
    "id": "uuid",
    "userId": "uuid",
    "name": "家庭自动化",
    "keyPrefix": "nwr_12345678...",
    "expiresAt": "2027-07-14T00:00:00Z",
    "lastUsedAt": null,
    "revokedAt": null,
    "createdAt": "2026-07-14T00:00:00Z"
  },
  "key": "nwr_<key-id>_<secret>"
}
```

列表接口只返回 `apiKey` 元数据，不会再次返回完整 `key` 或摘要。单个撤销成功返回 `204`。

撤销当前用户全部 Key 需要再次验证密码：

```http
DELETE /api/auth/api-keys
Content-Type: application/json

{"currentPassword": "当前账户密码"}
```

管理员接口只能查看指定用户的 Key 元数据或全部撤销，不能替其他用户创建 Key。

## 📚 漫画

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/comics` | 列表（按用户可访问书库过滤，搜索/筛选/分页/排序/FTS5 全文搜索）🔒 |
| GET | `/api/works` | 统一作品列表：一部作品一条，包含卷/话/内部章节 Units 🔒 |
| GET | `/api/works/:id` | 统一作品详情及 Units 🔒 |
| GET | `/api/works/:id/units` | 按自然顺序返回作品目录中的卷、话及压缩包内部章节 Units 🔒 |
| GET | `/api/catalog/items` | 兼容逻辑作品列表（目录作品与散本统一分页）🔒 |
| GET | `/api/comics/:id` | 详情（无权限返回 403）🔒 |
| PUT | `/api/comics/:id/favorite` | 切换收藏 🔒 |
| PUT | `/api/comics/:id/rating` | 更新评分 🔒 |
| PUT | `/api/comics/:id/progress` | 更新阅读进度 🔒 |
| PUT | `/api/comics/:id/reading-status` | 设置阅读状态 🔒 |
| PUT | `/api/comics/:id/metadata` | 编辑元数据 🔒管理员 |
| DELETE | `/api/comics/:id/delete` | 删除漫画（含磁盘文件） 🔒管理员 |
| POST | `/api/comics/batch` | 批量操作 🔒管理员 |
| POST | `/api/comics/cleanup` | 清理无效条目 🔒管理员 |
| POST | `/api/comics/redetect-types` | 漫画类型重检测 🔒管理员 |
| PUT | `/api/comics/reorder` | 自定义排序 🔒管理员 |
| GET | `/api/comics/duplicates` | 重复检测 |


### 漫画列表查询

```
GET /api/comics?libraryIds=lib-a,lib-b&contentType=comic&page=1&pageSize=24
Authorization: Bearer <token>
```

| 查询参数 | 类型 | 必填 | 说明 |
|:---|:---|:---:|:---|
| `search` | string | 否 | 按标题/文件名全文搜索 |
| `tags` | string | 否 | 逗号分隔的标签名，匹配任意标签 |
| `favorites` | string | 否 | `true` 时仅返回当前用户收藏 |
| `sortBy` | string | 否 | 排序字段，默认 `title`；标题排序使用中文拼音 + 数字自然排序键 |
| `sortOrder` | string | 否 | `asc` / `desc`，默认 `asc` |
| `category` | string | 否 | 分类 slug；`uncategorized` 表示无分类 |
| `contentType` | string | 否 | `comic` / `novel` |
| `readingStatus` | string | 否 | `want` / `reading` / `finished` / `shelved` |
| `libraryIds` | string | 否 | 逗号分隔的书库 ID。管理员按传入书库过滤；普通用户会与自身可访问书库取交集，不会越权 |
| `uncategorized` | string | 否 | `true` 时仅返回无分类作品 |
| `untagged` | string | 否 | `true` 时仅返回无标签作品 |
| `page` | int | 否 | 页码，默认 `0` |
| `pageSize` | int | 否 | 每页数量，默认 `0`（不分页） |
| `seriesView` | string | 否 | `true` 时通过统一 Work 模型返回一部作品一条的兼容 DTO；其他值或省略时返回普通物理漫画记录 |

- 需要登录。
- 普通用户即使不传 `libraryIds`，也只返回自己可访问书库中的作品。
- 普通用户传入无权限书库 ID 时会被过滤掉；交集为空时返回空列表。
- 没有任何书库访问权限的普通用户返回空列表，不会退化成全库查询。
- `sortBy=title` 时会按服务端维护的 `titleSortKey` 排序，效果上 `第2卷` 在 `第10卷` 前，常见中文标题按拼音顺序排列。
- `seriesView=true` 是旧客户端兼容入口，响应与 `/api/works` 使用同一统一 Work DTO、权限过滤和分页语义；客户端应使用返回的 `id` 进入 `/api/works/:id`，并通过 Unit 的真实 `comicId` 读取物理内容。
- 新客户端应直接使用 `/api/works`；需要兼容目录作品与散本选择器的场景可使用 `/api/catalog/items`。

### 统一作品 Work API

`/api/works` 是漫画客户端、书库页面和协议适配层使用的统一作品视图。它不会改变底层物理 `Comic`，而是在读取时建立：

```text
Work（作品） -> Unit（卷/话/内部章节） -> Page（页）
```

- 目录多话、多卷压缩包、单 ZIP/CBZ 内部章节和 PDF 都返回 Work。
- 同一 Work 在列表中只出现一次；`itemCount` 是 Unit 数量。
- `GET /api/works/:id` 返回同一个 Work 及其自然排序后的 `units`。
- Unit 的 `comicId` 始终指向真实物理 Comic；`internalPath` 非空表示该 Unit 位于单个 ZIP/CBZ 内部。
- 内部 Unit 使用 `startPage` 和 `pageCount` 表示其在物理 Comic 页面序列中的窗口。
- Work ID 和 Unit ID 是稳定逻辑 ID，不能传给旧的 `/api/comics/:id`、下载或页面图片接口；读取物理内容仍使用 Unit 的 `comicId`。
- Work API 使用普通 `canView` 书库权限；OPDS Work 目录另行要求 `canDownload`，两者不会互相回退。

### 兼容逻辑作品列表

```http
GET /api/catalog/items?contentType=comic&search=作品名&page=1&pageSize=12&sortBy=title&sortOrder=asc
Authorization: Bearer <token>
```

该兼容接口返回逻辑作品分页；新客户端应优先使用 `/api/works`：

- 至少包含两个有效漫画阅读单元的目录作品返回一条 `kind=series` 记录，成员不会重复作为散本返回。
- 单成员目录关系退回普通 `kind=comic` 记录，不会导致作品消失。
- 小说始终逐本返回 `kind=comic`，不接入目录作品。
- 搜索目录作品标题或任一成员标题/文件名时，返回对应目录作品。
- 普通用户始终按当前 `canView` 权限过滤；传入的 `libraryIds` 会与可访问书库取交集，交集为空时返回空列表。

| 查询参数 | 类型 | 必填 | 说明 |
|:---|:---|:---:|:---|
| `contentType` | string | 否 | `comic` / `novel`，默认 `comic` |
| `search` | string | 否 | 按逻辑作品标题、成员标题或文件名搜索 |
| `libraryIds` | string | 否 | 逗号分隔的书库 ID，普通用户不能借此扩大权限范围 |
| `page` | int | 否 | 页码，从 `1` 开始，默认 `1` |
| `pageSize` | int | 否 | 每页数量，默认 `24`，最大 `100` |
| `sortBy` | string | 否 | 当前仅支持 `title`，默认 `title` |
| `sortOrder` | string | 否 | `asc` / `desc`，默认 `asc` |

响应示例：

```json
{
  "items": [
    {
      "id": "ser_xxx",
      "kind": "series",
      "title": "目录作品",
      "coverUrl": "/api/comics/comic_xxx/thumbnail",
      "itemCount": 15,
      "libraryId": "lib_xxx"
    },
    {
      "id": "comic_xxx",
      "kind": "comic",
      "title": "散本漫画",
      "coverUrl": "/api/comics/comic_xxx/thumbnail",
      "itemCount": 1,
      "libraryId": "lib_xxx"
    }
  ],
  "page": 1,
  "pageSize": 12,
  "total": 2,
  "totalPages": 1
}
```

`id` 是对应实体的真实 ID，不要添加或解析虚拟前缀。

### 设置阅读状态

```
PUT /api/comics/:id/reading-status
Authorization: Bearer <token>

{
  "status": "want" | "reading" | "finished" | "shelved" | ""
}
```

- 需要登录（任何角色）
- 需要对该漫画有访问权限（书库权限校验）
- 状态保存到当前用户的 UserComicState，不更新全局 Comic 表
- 多用户之间阅读状态互不影响
- 空字符串 `""` 表示清除状态
- 第一版前端不暴露 `shelved` 状态

### 按阅读状态筛选列表

```
GET /api/comics?readingStatus=want
GET /api/comics?readingStatus=reading
GET /api/comics?readingStatus=finished
```

- 按当前用户的 UserComicState.readingStatus 过滤
- 与 search、tags、category、contentType、favorites 等条件可自由组合
- 不传该参数时不按阅读状态过滤

## 📂 目录作品

目录作品由漫画书库中的目录结构自动识别，用于把同一根目录下的多个阅读单元组织为一个作品。以下接口全部需要认证；读取接口按 `canView` 过滤，修改接口要求目标书库的 `canManage` 权限。

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/series` | 获取目录作品列表，支持 `libraryIds` 和 `search` |
| GET | `/api/series/preview?libraryId=:libraryId` | 预览书库的目录作品识别结果，需要目标书库 `canManage` |
| POST | `/api/series/rebuild?libraryId=:libraryId` | 重建指定书库的目录作品，需要目标书库 `canManage` |
| POST | `/api/series/rebuild` | 重建全部书库的目录作品，仅管理员 |
| GET | `/api/series/:id` | 获取目录作品详情、篇章和未分篇阅读单元 |
| PUT | `/api/series/:id` | 修改目录作品标题、封面或人工锁定状态，需要 `canManage` |
| PUT | `/api/series/:id/structure` | 修改阅读单元所属篇章及顺序，需要 `canManage` |
| POST | `/api/series/:id/re-detect` | 解除人工锁定并重新自动识别，需要 `canManage` |
| POST | `/api/series/:id/scrape-metadata` | 从在线数据源搜索目录作品元数据，仅管理员且需启用刮削 |
| POST | `/api/series/:id/apply-metadata` | 应用刮削元数据，可同步标签和元数据到全部阅读单元，仅管理员 |
| POST | `/api/series/:id/ai-recognize` | 使用首个阅读单元的封面和页面进行 AI 识别，仅管理员 |
| DELETE | `/api/series/:id` | 删除目录作品关系，不删除作品记录或磁盘文件，需要 `canManage` |

### 获取目录作品列表

```http
GET /api/series?libraryIds=lib-a,lib-b&search=日月
Authorization: Bearer <token>
```

| 查询参数 | 类型 | 必填 | 说明 |
|:---|:---|:---:|:---|
| `libraryIds` | string | 否 | 逗号分隔的书库 ID；普通用户只会查询与自身 `canView` 书库的交集 |
| `search` | string | 否 | 按目录作品标题搜索 |

管理员不传 `libraryIds` 时返回全部可识别目录作品；普通用户不传时返回其可查看书库中的目录作品。普通用户没有可查看书库，或传入书库与授权范围交集为空时，返回 `{"series":[]}`。

列表响应：

```json
{
  "series": [
    {
      "id": "ser_xxx",
      "libraryId": "lib_xxx",
      "contentType": "comic",
      "rootRelativePath": "作品目录",
      "title": "作品标题",
      "sortTitle": "作品排序标题",
      "coverComicId": "comic_xxx",
      "coverUrl": "/api/comics/comic_xxx/thumbnail",
      "author": "作者",
      "description": "作品简介",
      "year": 2026,
      "publisher": "出版社",
      "language": "zh",
      "genre": "冒险,奇幻",
      "status": "",
      "externalRating": 8.5,
      "externalRatingMax": 10,
      "externalRatingSource": "anilist",
      "metadataLocked": true,
      "tags": [{"id": 1, "name": "冒险", "color": ""}],
      "itemCount": 15,
      "sectionCount": 2,
      "completedItemCount": 3,
      "totalReadTime": 3600,
      "fileSize": 104857600,
      "lastReadAt": "2026-07-21T12:00:00Z",
      "isFavorite": false,
      "manualLocked": false,
      "canManage": true,
      "createdAt": "2026-07-21T10:00:00Z",
      "updatedAt": "2026-07-21T12:00:00Z"
    }
  ]
}
```

### 获取目录作品详情

```http
GET /api/series/:id
Authorization: Bearer <token>
```

返回 `{ "series": SeriesSummary, "sections": SeriesSection[], "unsectioned": SeriesItem[] }`。`sections[].items` 是已归入篇章的阅读单元，`unsectioned` 是未分篇单元；每个单元包含普通漫画对象 `comic`、`sectionId`、`sortIndex` 和 `displayLabel`。不存在返回 `404`，无目标书库查看权限返回 `403`。

`series` 同时返回目录作品级作者、简介、年份、出版社、语言、类型、状态、外部评分及标签。`contentType` 由所属书库类型决定；混合书库才按成员多数计算，用于选择正确的漫画或小说刮削源。存在刮削封面时，`coverUrl` 指向目录作品自己的缓存封面；否则继续回退到指定成员或首个阅读单元的封面。

### 预览与重建

```http
GET /api/series/preview?libraryId=lib_xxx
POST /api/series/rebuild?libraryId=lib_xxx
```

- `preview` 只运行识别并返回 `{ "libraryId": "...", "seriesCount": 3, "candidates": [...] }`，不会写入目录作品关系。
- 指定 `libraryId` 重建时要求该书库的 `canManage` 权限；省略 `libraryId` 表示重建全部书库，仅管理员可用。
- 重建成功返回 `{"success":true}`。

### 修改目录作品

```http
PUT /api/series/:id
Content-Type: application/json

{
  "title": "新标题",
  "coverComicId": "comic_xxx",
  "manualLocked": true
}
```

三个字段均可省略。空标题或空 `coverComicId` 不会清除现有值；成功返回 `{"success":true}`。

### 目录作品元数据刮削

在线搜索：

```http
POST /api/series/:id/scrape-metadata
Content-Type: application/json

{
  "query": "作品标题",
  "sources": ["anilist", "bangumi"],
  "lang": "zh",
  "contentType": "comic"
}
```

`query` 为空时使用目录作品当前标题；`lang` 默认为 `zh`。服务端以 `SeriesSummary.contentType` 为准选择并校验数据源：漫画书库固定使用漫画源，小说书库固定使用小说源，混合书库才按成员多数判断；请求中的 `contentType` 仅为兼容字段，不能覆盖服务端判断。成功返回：

```json
{"results":[ComicMetadata],"detectedContentType":"comic"}
```

应用搜索结果：

```http
POST /api/series/:id/apply-metadata
Content-Type: application/json

{
  "metadata": {
    "title": "作品标题",
    "author": "作者",
    "description": "简介",
    "genre": "冒险,奇幻",
    "publisher": "出版社",
    "language": "zh",
    "year": 2026,
    "coverUrl": "https://example.com/cover.jpg",
    "externalRating": 8.5,
    "externalRatingMax": 10,
    "externalRatingSource": "anilist",
    "source": "anilist"
  },
  "fields": ["author", "description", "genre", "publisher", "language", "year", "cover", "tags", "rating"],
  "overwrite": true,
  "syncTags": true,
  "syncToVolumes": true,
  "syncRating": false
}
```

- `fields` 为空时应用全部字段；否则只应用列出的字段。支持 `title`、`author`、`description`、`genre`、`publisher`、`language`、`year`、`cover`、`tags` 和 `rating`。
- `overwrite=false` 时仅填充目录作品及阅读单元的空字段。
- 成功应用后会启用独立的 `metadataLocked`，避免后续目录识别覆盖刮削标题和元数据，但不会锁住目录结构；新增或移除阅读单元仍可由扫描自动刷新。
- `syncTags=true` 将目录作品标签增量同步到全部阅读单元。
- `syncToVolumes=true` 将作者、简介、类型、出版社、语言和年份同步到全部阅读单元；`syncRating=true` 时同时同步外部评分。
- 刮削封面保存到目录作品自己的缩略图缓存，不覆盖成员作品封面。

AI 识别：

```http
POST /api/series/:id/ai-recognize
Content-Type: application/json

{"lang":"zh"}
```

服务端使用首个阅读单元的封面及前两页识别作品，并返回 `{"success":true,"recognized":...,"metadata":...}`。该接口要求已配置云端 AI。三个接口均仅限管理员并要求启用刮削功能。应用接口成功时的 `series` 字段为更新后的 `SeriesSummary`。

调整阅读单元结构：

```http
PUT /api/series/:id/structure
Content-Type: application/json

{
  "items": [
    {"comicId": "comic_a", "sectionId": "section_a", "sortIndex": 0},
    {"comicId": "comic_b", "sectionId": "", "sortIndex": 1}
  ]
}
```

`sectionId` 为空表示不归入篇章。服务端会校验篇章和阅读单元是否属于当前目录作品，并在更新后自动设置 `manualLocked=true`，避免后续自动识别覆盖人工顺序。

`POST /api/series/:id/re-detect` 会先解除人工锁定，再重建该目录作品所在书库。`DELETE /api/series/:id` 只删除目录作品、篇章和成员关系，返回：

```json
{"success":true,"filesDeleted":false,"comicsDeleted":false}
```

如果物理目录仍满足自动识别条件，后续扫描、重建或自动刷新可能再次创建该目录作品。

## 🏷️ 标签 & 分类

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/tags` | 标签列表 |
| PUT | `/api/tags/color` | 更新标签颜色 🔒管理员 |
| PUT | `/api/tags/rename` | 重命名标签 🔒管理员 |
| DELETE | `/api/tags` | 删除标签 🔒管理员 |
| POST | `/api/tags/merge` | 合并标签 🔒管理员 |
| POST | `/api/tags/translate` | 标签翻译 🔒管理员 |
| POST | `/api/comics/:id/tags` | 添加标签 🔒管理员 |
| DELETE | `/api/comics/:id/tags` | 移除标签 🔒管理员 |
| DELETE | `/api/comics/:id/tags/clear-all` | 清除所有标签 🔒管理员 |
| POST | `/api/comics/:id/translate-metadata` | 翻译漫画元数据 🔒管理员 |
| GET | `/api/categories` | 分类列表 |
| POST | `/api/categories` | 初始化分类 🔒管理员 |
| POST | `/api/categories/create` | 创建分类 🔒管理员 |
| PUT | `/api/categories/reorder` | 分类排序 🔒管理员 |
| PUT | `/api/categories/:slug` | 更新分类 🔒管理员 |
| DELETE | `/api/categories/:slug` | 删除分类 🔒管理员 |
| POST | `/api/comics/:id/categories` | 添加分类 🔒管理员 |
| PUT | `/api/comics/:id/categories` | 设置分类 🔒管理员 |
| DELETE | `/api/comics/:id/categories` | 移除分类 🔒管理员 |

## Unified Works and removed ComicGroup API

The manga collection (`ComicGroup`) feature has been removed. `/api/groups` and all child paths are no longer registered and return `404`. Manga aggregation now uses the Work model. `ComicSeries` remains the directory relationship and stable metadata host; `UserGroup`, `UserGroupMember`, and `GroupLibraryAccess` remain unchanged for permissions.

## 🖼️ 图片 & 内容

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/comics/:id/pages` | 页面列表 |
| GET | `/api/comics/:id/page/:pageIndex` | 页面图片 |
| GET | `/api/comics/:id/thumbnail` | 缩略图 |
| POST | `/api/comics/:id/cover` | 更新封面 🔒管理员 |
| GET | `/api/comics/:id/pdf` | PDF 文件流式传输 |
| GET | `/api/comics/:id/chapter/:chapterIndex` | 小说章节内容 |
| GET | `/api/comics/:id/epub-resource/*resourcePath` | EPUB 资源 |
| GET | `/api/comics/:id/embedded-images` | 嵌入图片列表 |
| GET | `/api/comics/:id/embedded-image/:index` | 单个嵌入图片 |
| POST | `/api/comics/:id/warmup` | 页面预热 |
| POST | `/api/comics/:id/warmup-done` | 预热完成 |
| POST | `/api/thumbnails/manage` | 缩略图管理 🔒管理员 |

### 页面与小说目录

`GET /api/comics/:id/pages` 返回漫画页面或小说章节的平铺列表。小说章节继续使用稳定的 `index` 作为阅读进度、书签、搜索和章节内容接口的定位值，同时提供可选的目录层级字段：

- `level`：目录深度，顶级为 `0`。
- `parentIndex`：父目录节点在同一 `pages` 数组中的 `index`；顶级节点不返回。
- `hasChildren`：存在子目录时为 `true`；叶子节点不返回。

客户端可以按层级展示或折叠目录，但不应重新排列数组或改变章节 `index`。

```json
{
  "comicId": "string",
  "title": "string",
  "totalPages": 2,
  "pages": [
    {
      "index": 0,
      "name": "chapter-0001.html",
      "url": "/api/comics/string/chapter/0",
      "title": "第一卷",
      "level": 0,
      "hasChildren": true
    },
    {
      "index": 1,
      "name": "chapter-0002.html",
      "url": "/api/comics/string/chapter/1",
      "title": "第一章",
      "level": 1,
      "parentIndex": 0
    }
  ],
  "isNovel": true,
  "isPdf": false
}
```

## 🌐 元数据

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET/POST | `/api/metadata/search` | 搜索元数据 |
| POST | `/api/metadata/apply` | 应用元数据 |
| POST | `/api/metadata/scan` | 扫描 ComicInfo.xml |
| POST | `/api/metadata/novel-scan` | 扫描小说元数据 |
| POST | `/api/metadata/batch` | 批量获取元数据 |
| POST | `/api/metadata/translate-batch` | 批量翻译元数据 |
| GET | `/api/metadata/stats` | 元数据统计 🔒管理员 |
| POST | `/api/metadata/ai-batch` | AI 批量处理 🔒管理员 |
| GET | `/api/metadata/library` | 库信息 🔒管理员 |
| POST | `/api/metadata/batch-selected` | 批量选择 🔒管理员 |
| POST | `/api/metadata/clear` | 清除元数据 🔒管理员 |
| POST | `/api/metadata/batch-rename` | 批量重命名 🔒管理员 |
| POST | `/api/metadata/ai-rename` | AI 重命名 🔒管理员 |
| POST | `/api/metadata/ai-chat` | AI 对话 🔒管理员 |
| GET | `/api/metadata/folder-tree` | 文件夹树 🔒管理员 |
| POST | `/api/metadata/batch-folder` | 批量文件夹 🔒管理员 |

## 🤖 AI

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/ai/status` | AI 服务状态 |
| GET/PUT | `/api/ai/settings` | AI 设置 |
| GET | `/api/ai/models` | 可用模型列表 |
| POST | `/api/ai/test` | 测试 AI 连接 |
| GET/DELETE | `/api/ai/usage` | AI 用量统计 |
| GET/PUT/DELETE | `/api/ai/prompts` | 提示词模板 |
| POST | `/api/ai/chat` | AI 对话 |
| POST | `/api/ai/semantic-search` | 语义搜索 |
| POST | `/api/ai/reading-insight` | 阅读洞察报告 |
| POST | `/api/ai/batch-suggest-tags` | 批量标签建议 |
| POST | `/api/ai/suggest-category` | 分类建议 |
| POST | `/api/ai/batch-suggest-category` | 批量分类建议 |
| POST | `/api/ai/verify-duplicates` | AI 重复验证 |
| POST | `/api/ai/recommend-goal` | AI 推荐阅读目标 |

### AI 批量标签与分类候选选择

`/api/ai/batch-suggest-tags` 与 `/api/ai/batch-suggest-category` 均返回 SSE 流，每次最多处理 30 本作品。请求体支持以下两种互斥模式：

```json
{
  "comicIds": ["comic_xxx"],
  "targetLang": "zh",
  "apply": false
}
```

```json
{
  "selector": {
    "scope": "missing",
    "contentType": "comic",
    "libraryIds": ["lib_xxx"],
    "limit": 30
  },
  "targetLang": "zh",
  "apply": false
}
```

- `comicIds` 保留用于显式选择具体作品，超过 30 个时只处理前 30 个。
- `selector.scope`：标签接口支持 `missing` / `all`；分类接口支持 `uncategorized` / `missing` / `all`。默认分别为 `missing` 和 `uncategorized`。
- `selector.contentType`：可选值为 `comic` / `novel`；省略时处理两种内容。
- `selector.libraryIds`：可选书库范围，始终与当前用户可查看书库取交集，不能扩大权限。
- `selector.limit`：默认和最大值均为 `30`。
- `selector` 模式先发送 `selection` SSE 数据，其中包含 `eligible` 候选总数和 `selected` 本次处理数；随后发送逐本结果及最终 `done` 数据。

### AI 漫画级功能

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| POST | `/api/comics/:id/ai-summary` | 生成摘要 🔒 |
| POST | `/api/comics/:id/ai-parse-filename` | 解析文件名 🔒 |
| POST | `/api/comics/:id/ai-infer-title` | AI 推断标题 🔒 |
| POST | `/api/comics/:id/ai-suggest-tags` | 标签建议 🔒 |
| POST | `/api/comics/:id/ai-analyze-cover` | 封面分析 🔒 |
| POST | `/api/comics/:id/ai-complete-metadata` | 完善元数据 🔒 |
| POST | `/api/comics/:id/ai-chapter-recap` | 章节回顾 🔒 |
| POST | `/api/comics/:id/ai-chapter-summary` | 章节摘要 🔒 |
| POST | `/api/comics/:id/ai-chapter-summaries` | 批量章节摘要 🔒 |
| POST | `/api/comics/:id/ai-translate-page` | 页面翻译 🔒 |

## 📊 阅读统计 & 目标 & 导出

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/stats` | 阅读统计 |
| GET | `/api/stats/yearly` | 年度阅读报告 |
| POST | `/api/reading/:id/activity` | 幂等记录阅读进度与活跃时长（推荐）🔒 |
| POST | `/api/stats/session` | 开始阅读会话 |
| PUT | `/api/stats/session` | 结束阅读会话 |
| POST | `/api/stats/session/end` | 结束会话（sendBeacon 兜底） |
| GET | `/api/stats/enhanced` | 增强统计数据 |
| GET | `/api/stats/files` | 文件统计 |
| GET | `/api/stats/folder-tree` | 文件夹树统计 |
| GET | `/api/goals` | 获取目标进度 |
| POST | `/api/goals` | 设定阅读目标 🔒管理员 |
| DELETE | `/api/goals` | 删除阅读目标 🔒管理员 |
| GET | `/api/export/json` | JSON 全量导出 |
| GET | `/api/export/csv/sessions` | CSV 会话导出 |
| GET | `/api/export/csv/comics` | CSV 漫画列表导出 |

### 记录阅读活动

```http
POST /api/reading/:id/activity
Authorization: Bearer <token>
Content-Type: application/json

{
  "clientSessionId": "每次打开阅读器生成的唯一 ID",
  "page": 19,
  "totalPages": 190,
  "activeSeconds": 30,
  "sequence": 3,
  "finalize": false,
  "trackProgress": true
}
```

- `page` 使用从 `0` 开始的索引，服务端会按作品实际页数或章节数限制范围。
- `activeSeconds` 是本次客户端会话累计的有效阅读秒数，不是本次请求新增的秒数。页面不可见或应用进入后台时客户端应停止累计。
- `sequence` 在同一 `clientSessionId` 内单调递增。重复或乱序请求不会重复累计时长，也不会让阅读进度倒退。
- `finalize=true` 表示客户端主动结束本次会话；即使结束请求丢失，之前的心跳仍会保留已有时长和进度。
- `trackProgress=false` 时只记录阅读时长，不修改当前用户的阅读进度。
- 同一用户的 `clientSessionId` 必须只属于一个作品。接口需要当前用户拥有该作品所在书库的查看权限。
- 成功返回 `{"success": true}`。旧的 `/api/stats/session` 开始/结束接口继续保留用于兼容旧客户端，新客户端应使用本接口。

## 📡 OPDS & 推荐 & 其他

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/opds` | OPDS 根目录 |
| GET | `/api/opds/all` | 全部可下载 Work，一作品一条 |
| GET | `/api/opds/works` | 统一 Work 导航（规范入口） |
| GET | `/api/opds/works/:id` | Work 的 Unit acquisition Feed |
| GET | `/api/opds/recent` | 最近更新 |
| GET | `/api/opds/favorites` | 收藏列表 |
| GET | `/api/opds/series` | 兼容入口，与 Work 导航一致 |
| GET | `/api/opds/series/:id` | Work ID 优先；旧 ComicSeries ID 兼容 |
| GET | `/api/opds/series/:id/cover` | 旧 ComicSeries 封面兼容接口 |
| GET | `/api/opds/search.xml` | OpenSearch 搜索描述 |
| GET | `/api/opds/search` | OPDS 搜索 |
| GET | `/api/opds/cover/:id` | OPDS 漫画封面 |
| GET | `/api/opds/work-cover/:id` | Work 独立封面（与 Work API 元数据封面一致） |
| GET | `/api/opds/unit-cover/:id?page=N` | 内部 Unit 独立封面（物理 Comic 的绝对页） |
| GET | `/api/opds/stream/:id` | OPDS-PSE 1.2 逐页 JPEG |
| GET/HEAD | `/api/opds/download/:id/:filename` | 下载原始文件，支持字节范围请求 |
| GET/HEAD | `/api/opds/download/:id` | 兼容旧版下载地址 |
| GET | `/api/recommendations` | 个性化推荐 |
| GET | `/api/recommendations/similar/:id` | 相似推荐 |
| POST | `/api/recommendations/ai-reasons` | AI 推荐理由 |
| GET/HEAD | `/api/health` | 健康检查；HEAD 仅返回状态和响应头 |
| GET/PUT | `/api/site-settings` | 站点设置 |
| POST | `/api/upload` | 文件上传 🔒（管理员或目标书库 canManage） |

### OPDS 1.2

- **认证**：支持浏览器 Session Cookie、`Authorization: Bearer <API Key>`，以及“用户名 + API Key”形式的 HTTP Basic Auth。
- **统一 Work 目录**：`/api/opds/all`、`/api/opds/works` 和 `/api/opds/series` 都先按 Work 聚合并返回 `kind=navigation`，一部作品只出现一个条目。三个入口使用同一权限范围和同一 Work ID；条目统一链接到 `/api/opds/works/:workId`。
- **Unit Feed**：`/api/opds/works/:workId` 返回 `kind=acquisition`，每个卷、话、PDF 全文或 ZIP/CBZ 内部章节对应一个 Unit 条目。Unit Atom ID 使用稳定 Unit ID，不会因多个内部章节共用同一个物理 Comic 而重复。
- **物理内容链接**：Unit 的 acquisition、封面和 PSE stream 继续使用真实 `comicId`，不会把 Work ID 或 Unit ID 伪装成可下载文件 ID。单 ZIP/CBZ 的多个内部 Unit 因而可以共享同一个原文件下载地址，同时拥有不同的逐页窗口。
- **文件夹 Unit**：散图文件夹没有可直接下载的单一原文件，因此只发布基于真实 `comicId` 的 PSE stream，不伪造 acquisition 下载链接；ZIP、CBZ、PDF 等文件型 Unit 同时发布原文件 acquisition 和 PSE。
- **内容范围**：`all`、`works`、`series`、`recent`、`favorites` 和 `search` 均使用统一 Work 模型，只聚合所属 `Library.type=comic`、书库已启用且 `Comic.type=comic` 的内容；`Library.type=novel` 的小说书库整体不接入 OPDS。`recent` 按 Work 内最新 `addedAt` 排序，`favorites` 保留“任一 Unit/Series 收藏即收藏 Work”的语义，`search` 可命中 Work 元数据或 Unit 标题。
- **元数据一致性**：OPDS 与 `/api/works` 使用同一 Work 加载和 `ApplySeriesMetadata` 流程，作品标题、作者、简介、语言、出版社、年份、标签和封面保持一致；Atom `updated` 使用 Comic/ComicSeries 的实际更新时间。
- **文件格式**：CBZ/ZIP、CBR/RAR、CB7/7Z、PDF、EPUB、MOBI、AZW3、TXT 和 HTML/HTM。文件格式只决定 acquisition 媒体类型，不改变书库类型边界。
- **分段下载**：下载接口支持 `HEAD` 和 HTTP Range。合法范围请求返回 `206 Partial Content`、`Content-Range` 与分段 `Content-Length`，便于客户端读取大型 CBZ/PDF 的尾部目录。
- **下载发现**：Feed 中的 acquisition URL 以经过转义的真实文件名结尾，并通过 Atom `length` 属性提供文件字节数。旧版不带文件名的下载地址继续可用。
- **媒体类型**：CBZ/ZIP、CBR/RAR、CB7/7Z 和 PDF 分别使用 `application/vnd.comicbook+zip`、`application/x-cbr`、`application/x-cb7` 和 `application/pdf`；EPUB、MOBI、AZW3、TXT 和 HTML/HTM 分别使用 `application/epub+zip`、`application/x-mobipocket-ebook`、`application/vnd.amazon.mobi8-ebook`、`text/plain` 和 `text/html`。
- **权限**：OPDS 是获取目录，只返回当前用户拥有 `canDownload` 权限的书库内容。公开书库或仅有 `canView` 权限不会自动获得 OPDS 下载权限。
- **Series 兼容**：`/api/opds/series` 不再建立另一套聚合结果，而是统一 Work 目录的兼容别名。`/api/opds/series/:id` 收到 Work ID 时返回同一 Unit Feed；收到旧 ComicSeries ID 时仍按旧结构返回，避免已有客户端书签立即失效。
- **直链保护**：Work 列表、Unit Feed、封面、PSE 和下载都以当前用户的 `canDownload` 书库集合为边界。没有任何下载权限时返回空目录，不会退回全库；公开或仅可查看书库不会泄漏。封面、PSE 与下载接口还会再次校验漫画、书库状态和下载权限。
- **OPDS-PSE 1.2**：可逐页阅读的漫画条目提供 `rel=http://vaemendis.net/opds-pse/stream` 链接和 `pse:count` 页数。普通 CBZ/ZIP/PDF Unit 保留物理文件 acquisition；单个 ZIP/CBZ 内拆出的虚拟 Unit 仅发布带 `startPage/pageCount` 的 PSE，不重复发布多个指向同一整包文件的下载链接。
- **封面**：Work 条目使用 `/api/opds/work-cover/:workId`；普通物理 Unit 使用 Comic 封面；ZIP/CBZ 内部 Unit 使用 `/api/opds/unit-cover/:comicId?page=:startPage`，因此不同章节不会全部指向同一张整包封面。
- **逐页范围**：PSE 链接只提供给 `Comic.type=comic`、`pageCount>0` 且可按图片页面解析的 CBZ/ZIP、CBR/RAR、CB7/7Z、PDF，以及已识别为图片漫画的 EPUB、MOBI、AZW3。文本小说、TXT 和 HTML 仍可通过原始 acquisition 链接下载，但不会发布 PSE 链接。
- **内部 Unit 页窗口**：单 ZIP/CBZ 内部 Unit 的 PSE 模板会附带 `startPage` 和 `pageCount`。客户端仍从 `page=0` 开始请求；服务端将其映射到物理 Comic 的 `startPage + page`，并禁止越过该 Unit 的 `pageCount`。
- **逐页图片**：`/api/opds/stream/:id` 固定返回 `image/jpeg`。JPEG 原页在无需缩小时直接返回；其他图片格式和 PDF 渲染结果会保持宽高比转换为 JPEG，不放大、不裁剪。
- **逐页缓存**：转换结果按作品、源文件版本、页码和宽度缓存在页面缓存目录中，支持 `ETag` 和私有缓存。源文件大小或修改时间变化后不会继续命中旧版本缓存。
- **阅读位置**：PSE 链接可包含当前用户独立的 `pse:lastRead` 和 `pse:lastReadDate`。`lastRead` 从 1 开始；内部 Unit 会先把物理 Comic 进度换算为 Unit 内相对页码，未开始阅读时省略。OPDS-PSE 没有标准进度回写接口，页面请求本身不会更新阅读进度，以免把客户端预加载误记为已阅读。
- **Feed 类型**：`/api/opds`、`all`、`works`、`series`、`recent`、`favorites` 和 `search` 返回 `kind=navigation`；Work Unit Feed 返回 `kind=acquisition`。
- **搜索发现**：根目录通过 `rel=search` 指向 `/api/opds/search.xml`，搜索模板使用 `/api/opds/search?q={searchTerms}`。
- **分页**：`all`、`works`、`series`、Work Unit Feed、`recent`、`favorites` 和 `search` 支持 `page`、`pageSize`。默认每页 100 条，`pageSize` 最大 500；响应包含 OpenSearch 统计及 `first`、`last`、`previous`、`next` 链接。
- **收藏隔离**：`favorites` 读取当前用户的 `UserComicState`，不会混用其他用户或旧的全局收藏字段。
- **获取方式**：条目提供受认证保护的标准 acquisition 文件链接，不声明 `open-access`；符合逐页条件的漫画会同时提供 OPDS-PSE 1.2 链接。

搜索参数：

| 参数 | 类型 | 必填 | 说明 |
|:---|:---|:---:|:---|
| `q` | string | 是 | 匹配漫画标题或作者 |
| `page` | integer | 否 | 页码，从 1 开始 |
| `pageSize` | integer | 否 | 每页数量，默认 100，最大 500 |

### `GET /api/opds/stream/:id`

- **认证**：与其他 OPDS 接口一致，支持 Session、Bearer API Key 和“用户名 + API Key”形式的 HTTP Basic Auth。
- **权限**：需要作品所在漫画书库的 `canDownload` 权限。仅有 `canView` 权限返回 `403`。
- **响应**：成功返回 `image/jpeg`；不存在、不支持逐页读取或页码越界返回 `404`；参数错误返回 `400`。

| 参数 | 位置 | 类型 | 必填 | 说明 |
|:---|:---|:---|:---:|:---|
| `id` | path | string | 是 | 漫画 ID |
| `page` | query | integer | 是 | 从 0 开始的页面索引 |
| `width` | query | integer | 否 | 最大输出宽度，范围 1～4096；省略或传 0 使用默认上限，只缩小、不放大 |
| `startPage` | query | integer | 否 | 内部 Unit 在物理 Comic 中的起始页；通常由 OPDS Feed 自动提供 |
| `pageCount` | query | integer | 否 | 内部 Unit 的页数边界；传入后 `page >= pageCount` 返回 `404` |

### `POST /api/upload`

- **认证**: 登录用户。管理员可上传到任意启用书库；普通用户必须传 `libraryId` 且对该书库拥有 `canManage` 权限。
- **Content-Type**: `multipart/form-data`

#### 参数

| 参数 | 类型 | 必填 | 说明 |
|:---|:---|:---:|:---|
| `files` | File[] | 必填 | 上传文件列表（表单字段名必须为 `files`） |
| `category` | string | 可选 | `comic` 或 `novel`，帮助后端判断歧义扩展名（如 `.azw3`） |
| `libraryId` | string | 普通用户必填 | 目标书库 ID；普通用户必须拥有该书库 `canManage` 权限。不传时仅管理员可使用旧目录逻辑 |

#### 行为

**传入 `libraryId` 时**：

1. 查询目标 Library，校验存在、`enabled=true`、`rootPath` 非空
2. 文件写入 `Library.rootPath`
3. 按 `Library.type` 校验文件格式：
   - `comic`：仅允许归档类（`.zip` `.cbz` `.rar` `.cbr` `.7z` 等）
   - `novel`：仅允许电子书（`.txt` `.epub` `.mobi` `.azw3` `.html` `.htm` `.pdf`）
   - `mixed`：允许全部支持格式

**不传 `libraryId` 时**：

- 仅管理员可用，完全兼容旧逻辑
- 漫画文件写入 `comicsDir`，小说文件写入 `novelsDir`
- 根据 `category` 和文件扩展名自动判断目标目录

**上传成功后**：

- 接口只负责文件落盘，**不直接写入数据库**
- 不直接触发扫描
- 入库依赖现有 `POST /api/sync` 扫描流程（上传成功后通常自动触发）

#### 错误响应

| HTTP 状态码 | 场景 |
|:---:|:---|
| 400 | 没有上传文件 / `libraryId` 不存在 / Library 已禁用 / Library rootPath 为空 |
| 401/403 | 未登录 / 普通用户未传 `libraryId` / 对目标书库无 `canManage` 权限 |
| 200（单文件级别） | 文件已存在 / 不支持的格式 / 文件类型与书库类型不匹配（按单文件报告在 `results` 中） |

#### 响应示例

**全部成功**：

```json
{
  "message": "Successfully uploaded 2 file(s)",
  "results": [
    { "filename": "vol01.zip", "success": true },
    { "filename": "vol02.zip", "success": true }
  ],
  "successCount": 2,
  "totalCount": 2,
  "libraryId": "abc123"
}
```

**部分失败**：

```json
{
  "message": "Uploaded 1 of 2 file(s), 1 failed",
  "results": [
    { "filename": "vol01.zip", "success": true },
    { "filename": "notes.txt", "success": false, "error": "File type not allowed for comic library" }
  ],
  "successCount": 1,
  "totalCount": 2,
  "libraryId": "abc123"
}
```

| POST | `/api/cache` | 缓存管理 🔒管理员 |
| POST | `/api/sync` | 触发文件同步 🔒管理员 |
| GET | `/api/browse-dirs` | 浏览服务器目录 🔒管理员 |
| GET | `/api/logs` | 错误日志 🔒管理员 |
| GET | `/api/logs/stats` | 日志统计 🔒管理员 |
| GET | `/api/logs/export` | 导出日志 🔒管理员 |
| DELETE | `/api/logs` | 清理日志 🔒管理员 |

## 🎨 站点设置

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/site-settings/icon` | 获取站点图标 |
| POST | `/api/site-settings/icon` | 上传站点图标 🔒管理员 |
| DELETE | `/api/site-settings/icon` | 删除站点图标 🔒管理员 |

## 📋 扫描规则 🔒管理员

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/scan-rules` | 获取扫描规则 |
| PUT | `/api/scan-rules` | 更新扫描规则 |
| POST | `/api/scan-rules/apply` | 应用规则 |
| POST | `/api/scan-rules/preview` | 预览规则效果 |
| POST | `/api/scan-rules/restore-titles` | 恢复原标题 |
| GET | `/api/scan-rules/logs` | 规则应用日志 |
| GET | `/api/scan-rules/progress` | 应用进度 |

## 💾 存储管理 🔒管理员

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/admin/storage` | 存储概览 |
| GET | `/api/admin/storage/database` | 数据库信息 |
| GET | `/api/admin/storage/history` | 历史记录 |
| POST | `/api/admin/storage/cache/clear` | 清除缓存 |
| POST | `/api/admin/storage/db/checkpoint` | 数据库检查点 |
| POST | `/api/admin/storage/db/analyze` | 数据库分析 |
| POST | `/api/admin/storage/db/vacuum` | 数据库清理 |
| POST | `/api/admin/storage/db/integrity` | 数据库完整性检查 |
| PUT | `/api/admin/storage/threshold` | 更新阈值 |

## 🌍 翻译引擎

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/translate/engines` | 翻译引擎列表 |
| GET | `/api/translate/config` | 翻译配置 |
| GET | `/api/translate/health` | 引擎健康检查 |
| GET | `/api/translate/cache/stats` | 缓存统计 |
| PUT | `/api/translate/config` | 更新翻译配置 🔒管理员 |
| DELETE | `/api/translate/cache` | 清除翻译缓存 🔒管理员 |
| POST | `/api/translate/test` | 测试翻译引擎 🔒管理员 |

## 🔄 元数据同步 🔒管理员

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/sync/status` | 同步状态 |
| GET | `/api/sync/history` | 同步历史 |
| GET | `/api/sync/diff/:id` | 差异对比 |
| POST | `/api/sync/push` | 推送同步 |
| POST | `/api/sync/revert` | 回滚同步 |

## ⚙️ 系统

## 📖 书库管理 🔒管理员

| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/admin/libraries` | 获取所有书库列表 |
| POST | `/api/admin/libraries` | 创建书库 |
| PUT | `/api/admin/libraries/:id` | 更新书库 |
| DELETE | `/api/admin/libraries/:id` | 删除书库 |
| POST | `/api/admin/libraries/:id/scan` | 扫描指定书库 |
| GET | `/api/admin/libraries/ownership-preview` | 预览目录归属与重复记录 |
| POST | `/api/admin/libraries/ownership-reconcile` | 合并重复记录并修复书库归属 |
| POST | `/api/admin/libraries/:id/delete-preview` | 删除书库预览（不删除源文件） |
| GET | `/api/libraries/accessible` | 获取当前用户可访问书库 🔒 |
| POST | `/api/libraries/:id/scan` | 扫描当前用户可管理的书库 🔒（需要 canManage） |
| GET | `/api/admin/users/:id/library-access` | 获取用户书库访问权限 |
| PUT | `/api/admin/users/:id/library-access` | 设置用户书库访问权限 |
| GET | `/api/admin/user-groups/:id/library-access` | 获取权限组书库访问权限 |
| PUT | `/api/admin/user-groups/:id/library-access` | 设置权限组书库访问权限 |

> 普通用户只能访问被授权的书库。列表接口按用户可访问书库自动过滤，详情/图片/PDF/章节/OPDS 等资源接口无权限返回 403。
>
> `POST /api/admin/libraries/:id/scan` 与 `POST /api/libraries/:id/scan` 会以一次完整扫描得到的磁盘内容为准：新增文件会入库，已从该书库移除的文件记录会立即清理并刷新缓存；响应中的 `added`、`removed` 分别表示新增和清理数量。如果任一根目录缺失、不是目录或遍历时发生权限错误，本次扫描不会删除旧记录，避免 NAS、Docker 挂载或权限异常造成误删。

### 书库字段

书库对象包含多目录和权限相关字段：

```json
{
  "id": "string",
  "name": "string",
  "type": "comic|novel|mixed",
  "rootPath": "string",
  "rootPaths": ["string"],
  "enabled": true,
  "sortOrder": 0,
  "defaultAccess": "public|private",
  "scanEnabled": true,
  "lastScanAt": null,
  "lastScanAdded": 0,
  "lastScanTotal": 0,
  "comicCount": 0
}
```

- `rootPath` 是主目录；`rootPaths` 包含主目录和额外目录。
- 文件解析按漫画记录的 `libraryId + relativePath` 在该书库所有根目录内查找，不再按全局文件名唯一定位。
- 书库内通过 `libraryId + relativePath` 去重，不同书库允许相同文件名。
- 不同书库不能配置完全相同的物理根目录；创建或更新时返回 `409 Conflict` 和 `conflicts` 数组。
- 父子根目录可以共存，文件归属于匹配路径最深的书库；父书库扫描时自动跳过子书库目录。
- 禁用或关闭自动扫描的子书库仍保留目录所有权，防止内容被父书库以不同权限重新收录。

### 书库归属巡检与修复

```http
GET /api/admin/libraries/ownership-preview
```

仅按真实物理路径检查，不会将不同目录中的同名文件或相同 MD5 文件误判为同一条记录。响应包括完全相同的根目录冲突，以及需要移动或合并的记录：

```json
{
  "issueCount": 1,
  "duplicateRows": 1,
  "canReconcile": true,
  "rootConflicts": [],
  "issues": [
    {
      "physicalPath": "/books/novels/book.epub",
      "targetLibraryId": "novels",
      "targetLibraryName": "小说",
      "targetRelativePath": "book.epub",
      "targetId": "string",
      "action": "merge",
      "resolvable": true,
      "records": []
    }
  ]
}
```

确认修复：

```http
POST /api/admin/libraries/ownership-reconcile
Content-Type: application/json

{
  "confirm": true,
  "rootOwners": {
    "/books/novels": "novels"
  }
}
```

修复在数据库事务中合并标签、分类、用户阅读状态、阅读会话和元数据日志，不删除或移动源文件。存在完全相同的根目录冲突时，必须通过 `rootOwners` 明确选择每个目录保留在哪个书库；记录合并完成后再修改其他书库的重复路径。未提供完整选择时返回 `409 Conflict`。

### 当前用户可访问书库

```
GET /api/libraries/accessible
```

响应中仅包含当前用户可查看的启用书库，并附带该用户是否可管理：

```json
{
  "libraries": [
    {
      "id": "string",
      "name": "string",
      "type": "comic|novel|mixed",
      "enabled": true,
      "defaultAccess": "public|private",
      "comicCount": 0,
      "canManage": true
    }
  ]
}
```

### 用户/权限组书库权限

用户和权限组的书库权限均为三列权限矩阵：

- `canView`: 可查看书库内容。
- `canDownload`: 可下载书库内容。
- `canManage`: 可上传、管理该书库内容。
- 保存时 `canDownload` 或 `canManage` 会自动包含 `canView`。
- 兼容旧请求体 `{ "libraryIds": ["lib-id"] }`，等价于对这些书库设置 `canView=true`、`canDownload=false`、`canManage=false`。

#### 获取用户书库权限

```
GET /api/admin/users/:id/library-access
```

```json
{
  "userId": "string",
  "libraries": [
    {
      "id": "string",
      "name": "string",
      "type": "comic|novel|mixed",
      "rootPath": "string",
      "rootPaths": ["string"],
      "canView": true,
      "canDownload": false,
      "canManage": false
    }
  ]
}
```

#### 设置用户书库权限

```
PUT /api/admin/users/:id/library-access
```

```json
{
  "libraryAccess": [
    {
      "libraryId": "string",
      "canView": true,
      "canDownload": false,
      "canManage": false
    }
  ]
}
```

#### 获取/设置权限组书库权限

```
GET /api/admin/user-groups/:id/library-access
PUT /api/admin/user-groups/:id/library-access
```

响应和请求体与用户书库权限相同，响应顶层字段为 `groupId`。


| 方法 | 路径 | 说明 |
|:---|:---|:---|
| GET | `/api/system/pdf-renderer` | PDF 渲染器状态 |
