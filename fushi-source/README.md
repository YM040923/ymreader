# 折角 · Fushi / Mihon 原生源

Fushi 的漫画“原生源”机制就是 Mihon 扩展。这里不是 Komga/OPDS 通用桥，而是直接读折角后端原生接口的源：

- 列表 / 搜索：`/api/works`（支持 `libraryId`、`favorites`、`tags`、`sortBy` / `sortOrder`）
- 详情：`/api/works/:id`
- 章节：`/api/works/:id/units`
- 页面图：`/api/comics/:comicId/page/:absolutePage`
- 认证：`Authorization: Bearer <API Key>`，或者源内置浏览器登录留下的 `nowen_session` Cookie
- 映射：Work → 漫画，Unit → 章节，页面按 Unit 的 `startPage + index` 取图。

## 成品

`dist/android/zhejiao-fushi-source-v1.6.3.apk`

安装后在 Fushi 的扩展列表里会出现 `Tachiyomi: 折角`（源名「折角」），首次安装需要信任。

## 筛选

搜索页的筛选器里有：

- **内容**：全部 / 国漫 / 日漫（读 `/api/libraries/accessible`，新增库会自动出现）
- **收藏**：全部 / 仅收藏
- **排序**：标题、最近添加、最近更新、上次阅读、评分
- **标签**：服务端全部标签，可多选，命中任意一个即返回

库和标签在第一次打开筛选器时异步拉取；如果只看到「点一下「重置」加载库和标签」，点一次重置就会载入。

Fushi 发现页顶部的「热门推荐」取自每个来源热门行的第一条。本源的热门行每次请求都会打乱顺序，所以刷新发现页时首条会变化，不会一直停在按标题排第一的那本。注意 Hero 的页数等于启用的来源数量：只装了一个源时它只有一页，不会在作品之间自动轮播；刷新才会换。

## Windows / macOS

**同一个 APK，不需要单独打 Windows 包。**

Fushi 桌面版的扩展页（漫画 → 扩展）有「安装 APK / 本地安装」入口，选 `zhejiao-fushi-source-v1.6.3.apk` 即可；桌面端会把它交给自带的 M-Extension-Server（Java sidecar）加载，不要求系统另装 JDK 或 Android 工具。装完后同样在源的设置里填服务器地址和 API Key（或先用源的内置浏览器登录折角）。

- Windows 没有 ATS 限制，HTTP 可以直接用；
- macOS 同样走 sidecar；
- iOS 没有 Mihon 扩展运行时，不能用这个 APK，需要走 OPDS 下载或另做 Fushi 内置源。

## 配置

打开源设置：

1. 服务器地址：`http://47.102.212.49:6680`（可改成局域网地址或 HTTPS 反代地址）
2. API Key：在折角里创建（需要当前密码），填到这里。

也可以不填 API Key，直接用源的内置浏览器登录折角；登录后的 `nowen_session` Cookie 会被源客户端复用。
注意：没有 Key 时源不会发送空 `Authorization: Bearer `——服务端看到 Authorization 头会优先按 API Key 校验、不再回退 Cookie，空 Bearer 会让图片全部 401。

## 重新构建

这个 APK 用 [Keiyoushi extensions-source](https://github.com/keiyoushi/extensions-source) 的构建系统生成。步骤：

```bash
git clone --depth 1 --filter=blob:none --sparse https://github.com/keiyoushi/extensions-source.git
cd extensions-source
git sparse-checkout set gradle core compiler common
cp -r /path/to/fushi-source/nowen src/zh/nowen
# 本机没有 compileSdk 37 时，把 gradle/kei.versions.toml 的
# android-sdk-compile / android-sdk-target 改成 36
./gradlew :src:zh:nowen:assembleRelease
```

产物：`src/zh/nowen/build/outputs/apk/release/tachiyomi-zh.nowen-v1.6.3.apk`（约 64KB）。

## 已知限制

- **平台**：Mihon 扩展运行时在 Android / Fushi 桌面 sidecar 上可用；iOS 没有 Mihon 扩展，需要走 OPDS 下载或另外做 Fushi 内置源。
- **进度**：阅读进度保存在 Fushi 本地，不回写折角的 `UserWorkProgress`；Mihon 扩展接口没有通用的服务端进度回调。
- **HTTP**：服务器目前是纯 HTTP；部分 Android 网络策略 / iOS ATS 会拦明文，远程使用建议加 HTTPS 反代。
