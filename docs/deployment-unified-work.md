# YMReader Unified Work 独立测试部署

本流程只面向以下隔离测试实例：

- NAS：`192.168.2.9`
- SSH：`22`
- Web：`http://192.168.2.9:17680`
- 镜像：`ymreader:unified-work-clean`
- 容器：`ymreader-unified-work-clean`
- 根目录：`/vol3/1000/ymreader-unified-work-clean`

脚本会永久拒绝端口 `6680`，不会读取、停止、替换或修改正式容器。

## 持久化布局

所有 Docker `VOLUME` 都使用显式 bind mount，不会生成落到 `vol1` 的匿名卷：

```text
/vol3/1000/ymreader-unified-work-clean/
  data/                 -> /data
  cache/                -> /app/.cache
  comics/               -> /app/comics
    fixture/
      A-folder-chapters/
      B-multi-cbz/
      C-multi-zip/
      D-single-large-zip/
      E-single-large-cbz-wrapper/
      F-mixed-folder-archive/
      G-pdf/
    _default-empty/       -> 防止默认 COMICS_DIR 把七库再次作为父目录扫描
  novels/               -> /app/novels
  build/source/         -> NAS 构建源码
  staging/              -> 临时上传包
  compose.yaml
```

容器日志使用 Docker `json-file` 轮转：

```text
max-size: 10m
max-file: 3
```

## 1. 本地生成夹具

```powershell
python scripts\bootstrap_unified_work_fixture.py
```

默认生成到：

```text
.tmp/unified_work_fixture
```

夹具不是七部不同漫画，而是**同一本《统一格式对照漫画》的七份平行副本**。标题、作者、简介、语言、Genre、章节标题、六张页面图片、章节封面和对应 `ComicInfo.xml` 元数据保持一致，只改变物理整理方式：

| 代码 | 独立书库 | 物理格式 |
|---|---|---|
| A | `A-folder-chapters` | 作品目录/章节图片文件夹 |
| B | `B-multi-cbz` | 作品目录/多个 CBZ |
| C | `C-multi-zip` | 作品目录/多个 ZIP |
| D | `D-single-large-zip` | 单大 ZIP 内章节 |
| E | `E-single-large-cbz-wrapper` | 单大 CBZ，内部带单一作品根目录 |
| F | `F-mixed-folder-archive` | 文件夹章节与 CBZ 混合 |
| G | `G-pdf` | 相同六张页面组成的 PDF |

七份副本分别放入七个测试书库，防止同库标题聚合把对照样本合并。G 保留相同 PDF 文档元数据，并在 `.fixture-metadata/ComicInfo.xml` 保存规范元数据副本；PDF 没有可移植的章节边界，因此明确允许它显示为一个“全文” Unit，其余 Work 展示语义必须一致。

## 2. 部署计划预览

默认只做 dry-run，不连接 NAS，也不运行 Docker：

```powershell
python scripts\deploy_unified_work_nas.py
```

检查输出中的端口、镜像、容器和四个 bind mount。SSH 密码不写入代码、命令行或配置文件，由系统 `ssh`/`scp` 在执行时交互询问。

## 3. 主代理批准后执行

只有同时提供 `--execute` 和固定确认串才会连接 NAS：

```powershell
python scripts\deploy_unified_work_nas.py `
  --execute `
  --confirm-target ymreader-unified-work-clean@192.168.2.9:17680
```

可选参数：

```text
--ssh-user ymzwh
--ssh-port 22
--puid 1000
--pgid 1000
--admin-username admin
```

管理员密码不提供明文默认值，也不接受命令行参数。正常交互执行时无需预先设置，脚本会通过 `getpass` 隐藏输入。CI 或无人值守环境必须由秘密管理系统注入 `YMREADER_ADMIN_PASSWORD`。

不要把密码写入 Git、文档、脚本、compose、命令行参数或 shell 历史。

部署脚本会：

1. 在本地打包当前工作树，因此包含尚未提交但已完成的代理修改；
2. 只上传到固定 `vol3` 测试根目录；
3. 在 NAS 上构建 `ymreader:unified-work-clean`；
4. 只替换同名测试容器；
5. 清空该隔离测试实例自己的 `data`、`cache`、`comics/fixture`、`novels`，避免旧测试数据污染验收；
6. 启动后创建管理员；
7. 根据 `fixture-manifest.json` 创建 A-G 七个独立漫画书库并逐库触发扫描。

## 4. 端到端验收

使用夹具清单执行完整的有状态验收：

```powershell
python scripts\verify_unified_work_container.py `
  --base-url http://192.168.2.9:17680 `
  --manifest .tmp\unified_work_fixture\fixture-manifest.json `
  --comparison-report .tmp\unified_work_comparison_report.json
```

验收脚本同样只从 `YMREADER_ADMIN_PASSWORD` 或交互式隐藏输入读取密码。

验收内容：

- 健康检查、管理员登录、匿名访问拒绝、管理权限；
- `Work → Unit → Page` 的详情、目录、页列表和真实页面渲染；
- A-G 七个书库各自恰好识别为一个 Work；
- 同一作品在文件夹、多个 CBZ、多个 ZIP、单大 ZIP、单大 CBZ、混合格式和 PDF 下的平行比较；
- Work ID 稳定、Unit 顺序、物理 Comic 数量、内部虚拟 Unit；
- 标题、作者、简介、语言、Genre、封面内容、封面来源可用性和封面比例；
- 章节标题、自然顺序、Unit 数、每 Unit 页数、物理与规范化起止页；
- 全部六张页面的顺序和内容指纹；PDF 允许渲染字节不同，但视觉指纹必须在阈值内；
- Work 详情 API 和 `/work/:id` 详情页面能力；
- `/api/comics?seriesView=true` 一作品一卡，以及搜索和书库筛选；
- 创建唯一测试标签和分类，并绑定到 Work 的 Comic metadata host；
- 验证 `/api/works` 与 `seriesView` 的标签、分类筛选都只返回一个 Work；
- 验证 Work 搜索与 OPDS 搜索保留同一标签/分类，且不会按 Unit 重复；
- OPDS `all`、`works`、`series`、`recent`、`favorites`、`search`；
- OPDS Work 详情、Unit 条目、虚拟 Unit PSE 流和重复下载链接；
- 连续阅读同一 Work 的两个不同物理 Unit；
- PDF 在同一全文 Unit 内验证首尾页；
- 继续阅读精确指向最后 Unit 和最后页；
- 阅读历史将两个 Unit 投影为一个 Work；
- `totalComicsRead` 只增加一个 Work，而不是增加两本；
- `/api/groups`、`/api/collections` 等合集 API 返回 `404`；
- 前端构建产物不再包含合集列表和详情路由。

比较报告是机器可读 JSON，包含：

- 每个变体的原始 Work DTO、Unit DTO、封面和页面指纹；
- 搜索、`seriesView`、OPDS、详情页、继续阅读、历史和统计结果；
- A 对 B-F 的严格规范化语义比较；
- A 对 G 的公共语义比较；
- 每种格式允许的 acquisition MIME；
- PDF 单 Unit、PDF 渲染字节等明确允许差异；
- 所有未列入允许清单的差异和最终 `pass`/`fail` 状态。

任何非允许差异都会同时写入报告并令验收脚本返回非零退出码。

验收会在隔离数据库中短暂写入阅读进度、阅读会话和收藏状态；收藏会在结束时恢复。只做只读检查时：

```powershell
python scripts\verify_unified_work_container.py `
  --base-url http://192.168.2.9:17680 `
  --manifest .tmp\unified_work_fixture\fixture-manifest.json `
  --read-only
```

## 安全边界

- 两个脚本都会拒绝 `6680`。
- 部署脚本锁定 `/vol3/1000/ymreader-unified-work-clean`。
- 删除和替换只允许发生在该固定根目录的 `build`、`staging`、`comics/fixture`，以及同名测试容器。
- 不清理其他镜像、容器、卷、书库或漫画。
- 不在 compose 中声明匿名卷。
- 本文档中的部署命令必须等主代理明确通知后才能执行。
