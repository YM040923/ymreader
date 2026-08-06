# Work 写 API

所有接口都需要登录，并要求当前用户是管理员或对 Work 所属的每一个
`Library` 拥有 `canManage` 权限。目录、多卷、多话和内部 ZIP 章节都会先
解析为唯一的物理 Comic ID 集合；同一物理 Comic 只执行一次操作。

## 单 Work

| 方法 | 路径 | 请求体 |
| --- | --- | --- |
| `PUT` | `/api/works/:id/favorite` | `{"isFavorite":true}` |
| `PUT` | `/api/works/:id/reading-status` | `{"status":"reading"}`；清除状态使用 `unread` |
| `PUT` | `/api/works/:id/tags` | `{"tags":["标签"]}`，替换整部作品标签 |
| `PUT` | `/api/works/:id/categories` | `{"categorySlugs":["action"]}`，替换整部作品分类 |
| `PUT` | `/api/works/:id/metadata` | Work 元数据字段 |
| `PUT` | `/api/works/:id/cover` | `{"url":"https://...","coverComicId":"comic-id","coverAspectRatio":0.68}` |
| `DELETE` | `/api/works/:id` | 删除 Work 的全部物理 Comic 记录 |

元数据和封面写入 `metadataHostType/metadataHostId` 指定的 Series 或
Comic；收藏、阅读状态、标签和分类会覆盖全部物理 Units。

## 批量操作

`POST /api/works/batch`

```json
{
  "workIds": ["work_xxx"],
  "action": "favorite|unfavorite|setReadingStatus|addTags|removeTags|setCategory|delete",
  "isFavorite": true,
  "readingStatus": "reading",
  "tags": ["标签"],
  "categorySlugs": ["action"]
}
```

也兼容 `comicIds` 输入，会先映射到 Work。权限检查在任何写入前一次性完成，
跨书库请求只要有一个来源无 `canManage` 就整体拒绝。

## 自定义排序

`PUT /api/works/reorder`

```json
{
  "orders": [
    {"id":"work_xxx","sortOrder":10},
    {"id":"work_yyy","sortOrder":20}
  ]
}
```

`id` 也可传物理 Comic ID。排序持久化在稳定的 WorkPreference 宿主，不依赖
代表 Comic 或 Series 是否发生迁移；随后使用 `/api/works?sortBy=custom`
即可读取。

## Work 级统计

- `GET /api/works/tags`
- `GET /api/works/categories`

统计以聚合后的 Work 数量计数，不会因为一部作品包含多话或多卷而膨胀。
