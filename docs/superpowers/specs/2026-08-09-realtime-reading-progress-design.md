# 实时阅读进度与继续阅读设计

## 目标

让 Android、Web 和服务端始终使用同一个“作品级阅读游标”：准确记录用户最后实际阅读到的作品、话/卷和页；允许用户主动回看前文时正常回退；网络中断后自动补传；返回详情页后立即显示最新“继续阅读”位置。

## 已确认根因

1. Android `WorkDetailScreen` 使用缓存的 `FutureProvider`。阅读器返回后没有失效并重新获取作品详情，按钮继续显示进入阅读前的数据。
2. 当前进度只按 `comicId + page` 写入，再由服务端从 `Comic` 推导所属 `WorkUnit`。同一个大 ZIP 的多话共用一个 `comicId`，客户端在章节切换时一旦提交话内相对页码，服务端无法识别错误。
3. 线上数据库已经出现可复现证据：同一作品先正确记录到绝对页 1308，随后被客户端提交的 7、13、8 覆盖。
4. `ReadingActivityTracker` 的网络写入是无等待并发请求；失败被静默吞掉，退出时的最后一次写入也没有可靠重试。
5. 当前 Work 继续阅读状态完全依赖多个 Comic 状态中时间最新的一条，缺少明确、可验证的作品级游标。

## 产品规则

- “继续阅读”表示最后一次真实阅读位置，允许正常回退。
- 回退必须来自明确的用户阅读事件，章节切换产生的错误相对页不得覆盖正确位置。
- 页面变化后 500ms 内尝试同步；切换话/卷、退出阅读器、应用进入后台时立即同步。
- 在线请求失败时保存在本机；再次联网、启动应用或恢复前台时自动重试。
- 服务端确认后，详情页、首页继续阅读和统计使用同一个作品级位置。
- 不删除或破坏现有 `/api/reading/:id/activity`，旧客户端继续兼容。

## 方案比较

### 方案 A：只修改 Android

刷新详情页，并给现有请求增加重试。改动小，但服务端仍通过 `comicId + page` 猜测话/卷，无法根治大 ZIP 的相对页错误。

### 方案 B：只修改服务端

服务端增加作品游标，但旧 Android 仍不提交 `workId/unitId`，并且详情页缓存不刷新，用户仍会看到延迟。

### 方案 C：端到端作品游标（采用）

Android 明确提交 `workId + unitId + relativePage`；服务端验证归属关系并计算绝对页，在同一事务中写入 Comic 状态和作品游标；客户端持久化待同步事件并在返回详情页时主动刷新。

## 参考实现

- Kavita GPL-3.0，commit `2c003dc3de50e9dc6d95892013fabeb6b8783f2f`，`UI/Web/src/app/_services/reader.service.ts`：进度请求明确携带 `libraryId/seriesId/volumeId/chapterId/pageNum`。借鉴其“层级身份与页码一起提交”，不复制代码。
- Mihon Apache-2.0，commit `2506b049642af2211c1ef81e7369f752363f655d`，`app/src/main/java/eu/kanade/tachiyomi/data/track/komga/KomgaApi.kt`：通过专用 read-progress API 同步连续章节位置。借鉴其服务端返回并确认规范化进度的边界设计，不复制代码。

## 服务端设计

### 数据表

新增 `UserWorkProgress`：

- `userId`
- `workId`
- `unitId`
- `comicId`
- `relativePage`
- `absolutePage`
- `updatedAt`
- `clientSessionId`
- `lastSequence`

主键为 `(userId, workId)`。

### API

扩展现有：

```http
POST /api/reading/:comicId/activity
```

新增可选字段：

```json
{
  "workId": "work_xxx",
  "unitId": "unit_xxx",
  "relativePage": 12
}
```

服务端流程：

1. 校验 Work、Unit、Comic 的关系。
2. 校验 `relativePage` 位于 Unit 页数范围内。
3. 使用 Unit 的 `startPage` 计算 canonical `absolutePage`。
4. 继续使用客户端 session sequence 防止同一会话乱序覆盖。
5. 在同一事务中更新 `UserComicState`、`Comic`、`ReadingSession` 和 `UserWorkProgress`。
6. 返回规范化结果：

```json
{
  "success": true,
  "progress": {
    "workId": "work_xxx",
    "unitId": "unit_xxx",
    "comicId": "comic_xxx",
    "relativePage": 12,
    "absolutePage": 1064,
    "updatedAt": "..."
  }
}
```

无 Work 上下文时保持现有行为。

### Work 查询

`GET /api/works` 和 `GET /api/works/:id` 优先读取 `UserWorkProgress`。旧用户没有作品游标时，才回退到现有 Comic 推导逻辑，并可惰性回填。

## Android 设计

### 统一进度对象

阅读器内部使用：

```text
comicId
workId
unitId
relativePage
absolutePage
totalPages
sequence
```

任何阅读模式都只能通过同一个入口更新该对象。

### 串行同步

- 同一阅读会话最多只有一个在途请求。
- 新页事件覆盖尚未发送的旧事件。
- 250–500ms 防抖后发送。
- 切换 Unit、退出、后台化时立即 flush。
- 服务端响应后更新本地 canonical 游标。

### 持久化待同步队列

仅保存每个 `server + user + work` 的最新一条事件，避免大量队列：

- 请求失败时持久化。
- 应用启动、恢复前台、网络恢复、打开对应作品时重试。
- 服务端确认后删除。

### UI 刷新

- 阅读器正常返回前等待最后一次 flush。
- 返回后失效 `workDetailProvider(workId)`。
- 同时刷新首页 Work 列表与继续阅读模块。
- 详情页按钮显示：
  - `立即阅读`
  - `继续阅读 · 第X话/第X卷 · 第Y页`
- 同步失败时只显示一个不打扰阅读的小型“待同步”状态；恢复后自动消失。

## Web 适配

Web 阅读器在有 Work 上下文时提交同样的 `workId/unitId/relativePage`，避免 Web 与 Android 再次产生不同的游标规则。

## 错误与兼容

- 非法 Unit 或页码返回 400，不写入任何进度。
- 网络失败不会丢失最后位置。
- 旧客户端继续按 Comic 进度工作。
- 合法的主动回退拥有更晚的服务端确认时间，因此可覆盖旧位置。
- 同一会话的旧 sequence 永远不能覆盖新 sequence。

## 验证标准

1. 大 ZIP 从第 20 话第 5 页切到第 21 话第 3 页，详情页立即显示第 21 话第 3 页。
2. 主动返回第 2 话阅读，继续阅读允许回退到第 2 话。
3. 网络断开阅读后退出，恢复网络能自动补传最后位置。
4. 快速翻页、切话、切模式和返回不会产生比当前 Unit 起始页更小的错误绝对页。
5. Android 返回详情页、首页继续阅读、Web 详情页三处位置一致。
6. 旧客户端请求和 OPDS 进度不被破坏。
