import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/api/api_client.dart';
import '../../data/api/metadata_api.dart';
import '../../data/api/comic_api.dart';
import '../../data/models/comic.dart';
import '../../widgets/authenticated_image.dart';
import '../../data/providers/auth_provider.dart';

// ============================================================
// 数据源定义
// ============================================================

class _SourceDef {
  final String id;
  final String name;
  final String icon;
  final Color color;

  const _SourceDef(this.id, this.name, this.icon, this.color);
}

const _comicSources = [
  _SourceDef('anilist', 'AniList', '🅰', Color(0xFF3B82F6)),
  _SourceDef('bangumi', 'Bangumi', '🅱', Color(0xFFEC4899)),
  _SourceDef('mangadex', 'MangaDex', '📖', Color(0xFFF97316)),
  _SourceDef('mangaupdates', 'MangaUpdates', '📋', Color(0xFF8B5CF6)),
  _SourceDef('kitsu', 'Kitsu', '🦊', Color(0xFFF59E0B)),
];

const _novelSources = [
  _SourceDef('googlebooks', 'Google Books', '📚', Color(0xFF10B981)),
  _SourceDef('bangumi_novel', 'Bangumi', '🅱', Color(0xFFEC4899)),
  _SourceDef('anilist_novel', 'AniList', '🅰', Color(0xFF3B82F6)),
];

// ============================================================
// 元数据刮削页面（漫画详情 → 入口）
// ============================================================

class MetadataScreen extends ConsumerStatefulWidget {
  final String comicId;
  const MetadataScreen({super.key, required this.comicId});

  @override
  ConsumerState<MetadataScreen> createState() => _MetadataScreenState();
}

class _MetadataScreenState extends ConsumerState<MetadataScreen>
    with SingleTickerProviderStateMixin {
  Comic? _comic;
  bool _loading = true;
  late TabController _tabCtrl;

  // ---- 搜索 Tab 状态 ----
  late TextEditingController _searchCtrl;
  List<MetadataResult> _searchResults = [];
  bool _searching = false;
  int? _applyingIndex;
  int? _appliedIndex;
  String? _searchError;
  List<String> _enabledSources = [];
  bool _showSourceFilter = false;

  // ---- 扫描 Tab 状态 ----
  bool _scanning = false;
  String? _scanResult;
  String? _scanError;

  // ---- 编辑 Tab 状态 ----
  final _authorCtrl = TextEditingController();
  final _publisherCtrl = TextEditingController();
  final _yearCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final _langCtrl = TextEditingController();
  final _genreCtrl = TextEditingController();
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _tabCtrl = TabController(length: 3, vsync: this);
    _searchCtrl = TextEditingController();
    _loadComic();
  }

  @override
  void dispose() {
    _tabCtrl.dispose();
    _searchCtrl.dispose();
    _authorCtrl.dispose();
    _publisherCtrl.dispose();
    _yearCtrl.dispose();
    _descCtrl.dispose();
    _langCtrl.dispose();
    _genreCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadComic() async {
    setState(() => _loading = true);
    try {
      final api = ref.read(comicApiProvider);
      final data = await api.getComic(widget.comicId);
      final comic = Comic.fromJson(data);
      setState(() {
        _comic = comic;
        _loading = false;
        _searchCtrl.text = comic.title;
        // 根据类型设置默认数据源
        final isNovel = comic.isNovel;
        _enabledSources = (isNovel ? _novelSources : _comicSources)
            .map((s) => s.id)
            .toList();
        // 填充编辑表单
        _authorCtrl.text = comic.author ?? '';
        _publisherCtrl.text = comic.publisher ?? '';
        _yearCtrl.text = comic.year?.toString() ?? '';
        _descCtrl.text = comic.description ?? '';
        _langCtrl.text = comic.language ?? '';
        _genreCtrl.text = comic.genre ?? '';
      });
    } catch (_) {
      setState(() => _loading = false);
    }
  }

  /// 可用的数据源列表
  List<_SourceDef> get _availableSources {
    if (_comic == null) return _comicSources;
    return _comic!.isNovel ? _novelSources : _comicSources;
  }

  // ============================================================
  // 在线搜索
  // ============================================================

  Future<void> _doSearch() async {
    if (_searchCtrl.text.trim().isEmpty || _enabledSources.isEmpty) return;
    setState(() {
      _searching = true;
      _searchError = null;
      _searchResults = [];
      _appliedIndex = null;
    });
    try {
      final api = ref.read(metadataApiProvider);
      final isNovel =
          _comic != null && _comic!.isNovel;
      final results = await api.searchMetadata(
        query: _searchCtrl.text.trim(),
        sources: _enabledSources,
        lang: 'zh',
        contentType: isNovel ? 'novel' : 'comic',
      );
      setState(() {
        _searchResults = results;
        _searching = false;
        if (results.isEmpty) {
          _searchError = '未找到结果';
        }
      });
    } catch (e) {
      setState(() {
        _searching = false;
        _searchError = '搜索失败：$e';
      });
    }
  }

  Future<void> _applyResult(int index) async {
    setState(() => _applyingIndex = index);
    try {
      final api = ref.read(metadataApiProvider);
      await api.applyMetadata(
        comicId: widget.comicId,
        metadata: _searchResults[index],
        lang: 'zh',
        overwrite: true,
      );
      setState(() {
        _applyingIndex = null;
        _appliedIndex = index;
      });
      _loadComic(); // 刷新数据
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('✅ 元数据已应用'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      setState(() => _applyingIndex = null);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('应用失败：$e'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    }
  }

  // ============================================================
  // 自动扫描
  // ============================================================

  Future<void> _doScan() async {
    if (_comic == null) return;
    setState(() {
      _scanning = true;
      _scanResult = null;
      _scanError = null;
    });
    try {
      final api = ref.read(metadataApiProvider);
      final isNovel = _comic!.isNovel;
      final data = isNovel
          ? await api.scanNovelMetadata(comicId: widget.comicId, lang: 'zh')
          : await api.scanMetadata(comicId: widget.comicId, lang: 'zh');

      final source = data['source'] as String?;
      if (source != null && source != 'none') {
        setState(() {
          _scanning = false;
          _scanResult = '✅ 元数据已从 ${_sourceLabel(source)} 应用';
        });
        _loadComic();
      } else {
        setState(() {
          _scanning = false;
          _scanError = data['message'] as String? ?? '未找到可用的元数据';
        });
      }
    } catch (e) {
      setState(() {
        _scanning = false;
        _scanError = '扫描失败：$e';
      });
    }
  }

  String _sourceLabel(String source) {
    switch (source) {
      case 'comicinfo':
        return 'ComicInfo.xml';
      case 'epub_opf':
        return 'EPUB OPF';
      case 'anilist':
        return 'AniList';
      case 'bangumi':
        return 'Bangumi';
      case 'mangadex':
        return 'MangaDex';
      case 'mangaupdates':
        return 'MangaUpdates';
      case 'kitsu':
        return 'Kitsu';
      case 'googlebooks':
        return 'Google Books';
      default:
        return source;
    }
  }

  // ============================================================
  // 手动编辑保存
  // ============================================================

  Future<void> _saveMetadata() async {
    setState(() => _saving = true);
    try {
      final api = ref.read(metadataApiProvider);
      await api.updateMetadata(
        comicId: widget.comicId,
        author: _authorCtrl.text.trim().isEmpty ? null : _authorCtrl.text.trim(),
        publisher: _publisherCtrl.text.trim().isEmpty ? null : _publisherCtrl.text.trim(),
        year: int.tryParse(_yearCtrl.text.trim()),
        description: _descCtrl.text.trim().isEmpty ? null : _descCtrl.text.trim(),
        language: _langCtrl.text.trim().isEmpty ? null : _langCtrl.text.trim(),
        genre: _genreCtrl.text.trim().isEmpty ? null : _genreCtrl.text.trim(),
      );
      setState(() => _saving = false);
      _loadComic();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('✅ 元数据已保存'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      setState(() => _saving = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('保存失败：$e'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    }
  }

  // ============================================================
  // UI 构建
  // ============================================================

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final serverUrl = ref.watch(authProvider).serverUrl;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(title: const Text('元数据刮削')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    final comic = _comic;
    if (comic == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('元数据刮削')),
        body: const Center(child: Text('加载失败')),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('元数据刮削'),
        bottom: TabBar(
          controller: _tabCtrl,
          tabs: const [
            Tab(icon: Icon(Icons.search), text: '在线搜索'),
            Tab(icon: Icon(Icons.document_scanner), text: '自动扫描'),
            Tab(icon: Icon(Icons.edit_note), text: '手动编辑'),
          ],
        ),
      ),
      body: Column(
        children: [
          // 顶部：漫画信息卡片
          _buildComicHeader(comic, serverUrl, cs),
          const Divider(height: 1),
          // Tab 内容
          Expanded(
            child: TabBarView(
              controller: _tabCtrl,
              children: [
                _buildSearchTab(cs),
                _buildScanTab(cs),
                _buildEditTab(cs),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// 漫画信息头部卡片
  Widget _buildComicHeader(Comic comic, String serverUrl, ColorScheme cs) {
    final thumbUrl = getImageUrl(serverUrl, comic.id, thumbnail: true);
    return Container(
      padding: const EdgeInsets.all(12),
      color: cs.surfaceContainerLow,
      child: Row(
        children: [
          // 缩略图
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: SizedBox(
              width: 56,
              height: 76,
              child: AuthenticatedImage(
                imageUrl: thumbUrl,
                fit: BoxFit.cover,
                errorWidget: Container(
                  color: cs.surfaceContainerHighest,
                  child: Icon(Icons.image, color: cs.outline),
                ),
              ),
            ),
          ),
          const SizedBox(width: 12),
          // 信息
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  comic.title,
                  style: Theme.of(context).textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    if (comic.metadataSource != null &&
                        comic.metadataSource!.isNotEmpty) ...[
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: cs.primaryContainer,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '来源: ${_sourceLabel(comic.metadataSource!)}',
                          style: TextStyle(
                              fontSize: 10, color: cs.onPrimaryContainer),
                        ),
                      ),
                      const SizedBox(width: 6),
                    ],
                    Text(
                      comic.isNovel ? '📖 小说' : '📚 漫画',
                      style: TextStyle(fontSize: 11, color: cs.outline),
                    ),
                  ],
                ),
                if (comic.author != null && comic.author!.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      '作者: ${comic.author}',
                      style: TextStyle(fontSize: 11, color: cs.outline),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ============================================================
  // Tab 1: 在线搜索
  // ============================================================

  Widget _buildSearchTab(ColorScheme cs) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // 搜索栏
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _searchCtrl,
                decoration: InputDecoration(
                  hintText: '搜索元数据...',
                  prefixIcon: const Icon(Icons.search, size: 20),
                  isDense: true,
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
                ),
                onSubmitted: (_) => _doSearch(),
              ),
            ),
            const SizedBox(width: 8),
            // 搜索按钮
            FilledButton.icon(
              onPressed:
                  (_searching || _searchCtrl.text.trim().isEmpty || _enabledSources.isEmpty)
                      ? null
                      : _doSearch,
              icon: _searching
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    )
                  : const Icon(Icons.search, size: 18),
              label: const Text('搜索'),
              style: FilledButton.styleFrom(
                minimumSize: const Size(0, 44),
                padding: const EdgeInsets.symmetric(horizontal: 12),
              ),
            ),
            const SizedBox(width: 4),
            // 数据源筛选按钮
            IconButton.filled(
              onPressed: () =>
                  setState(() => _showSourceFilter = !_showSourceFilter),
              icon: Icon(
                Icons.filter_list,
                size: 20,
                color: _showSourceFilter ? cs.onPrimary : cs.onSurfaceVariant,
              ),
              style: IconButton.styleFrom(
                backgroundColor:
                    _showSourceFilter ? cs.primary : cs.surfaceContainerHighest,
                minimumSize: const Size(44, 44),
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),

        // 数据源筛选面板
        if (_showSourceFilter) ...[
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: cs.surfaceContainerLow,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: cs.outlineVariant.withOpacity(0.5)),
            ),
            child: Wrap(
              spacing: 6,
              runSpacing: 6,
              children: _availableSources.map((src) {
                final enabled = _enabledSources.contains(src.id);
                return FilterChip(
                  avatar: Text(src.icon, style: const TextStyle(fontSize: 14)),
                  label: Text(src.name,
                      style: const TextStyle(fontSize: 12)),
                  selected: enabled,
                  onSelected: (_) {
                    setState(() {
                      if (enabled) {
                        _enabledSources.remove(src.id);
                      } else {
                        _enabledSources.add(src.id);
                      }
                    });
                  },
                  selectedColor: src.color.withOpacity(0.2),
                  checkmarkColor: src.color,
                  visualDensity: VisualDensity.compact,
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                );
              }).toList(),
            ),
          ),
          const SizedBox(height: 8),
        ],

        // 错误提示
        if (_searchError != null) ...[
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: cs.errorContainer,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(Icons.error_outline, size: 16, color: cs.error),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    _searchError!,
                    style: TextStyle(fontSize: 13, color: cs.onErrorContainer),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 8),
        ],

        // 已应用提示（扫描成功）
        if (_appliedIndex == -1) ...[
          _buildSuccessBanner(
              _comic != null && _comic!.isNovel
                  ? '小说元数据应用成功'
                  : '已从 ComicInfo.xml 应用元数据'),
          const SizedBox(height: 8),
        ],

        // 搜索结果列表
        ..._searchResults.asMap().entries.map((entry) {
          final i = entry.key;
          final result = entry.value;
          return _buildResultCard(result, i, cs);
        }),
      ],
    );
  }

  /// 搜索结果卡片
  Widget _buildResultCard(MetadataResult result, int index, ColorScheme cs) {
    final isApplied = _appliedIndex == index;
    final isApplying = _applyingIndex == index;
    final sourceDef = [..._comicSources, ..._novelSources]
        .where((s) => s.id == result.source)
        .firstOrNull;

    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 封面图
            if (result.coverUrl != null && result.coverUrl!.isNotEmpty)
              ClipRRect(
                borderRadius: BorderRadius.circular(6),
                child: Image.network(
                  result.coverUrl!,
                  width: 48,
                  height: 64,
                  fit: BoxFit.cover,
                  errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                ),
              ),
            if (result.coverUrl != null && result.coverUrl!.isNotEmpty)
              const SizedBox(width: 10),

            // 信息
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 标题 + 来源标签
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          result.title ?? '未知',
                          style: const TextStyle(
                              fontWeight: FontWeight.w600, fontSize: 14),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 6),
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: (sourceDef?.color ?? cs.primary)
                              .withOpacity(0.15),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          sourceDef?.name ?? result.source,
                          style: TextStyle(
                            fontSize: 10,
                            fontWeight: FontWeight.w500,
                            color: sourceDef?.color ?? cs.primary,
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (result.author != null && result.author!.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        '作者: ${result.author}',
                        style: TextStyle(fontSize: 12, color: cs.outline),
                      ),
                    ),
                  if (result.year != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 1),
                      child: Text(
                        '${result.year}'
                        '${result.publisher != null ? ' · ${result.publisher}' : ''}',
                        style: TextStyle(fontSize: 11, color: cs.outline),
                      ),
                    ),
                  if (result.description != null &&
                      result.description!.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Text(
                        result.description!,
                        style:
                            TextStyle(fontSize: 11, color: cs.onSurfaceVariant),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  // 类型标签
                  if (result.genre != null && result.genre!.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Wrap(
                        spacing: 4,
                        runSpacing: 4,
                        children: result.genre!
                            .split(',')
                            .take(5)
                            .map((g) => Container(
                                  padding: const EdgeInsets.symmetric(
                                      horizontal: 5, vertical: 1),
                                  decoration: BoxDecoration(
                                    color: cs.surfaceContainerHighest,
                                    borderRadius: BorderRadius.circular(4),
                                  ),
                                  child: Text(g.trim(),
                                      style: TextStyle(
                                          fontSize: 10, color: cs.outline)),
                                ))
                            .toList(),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 8),

            // 应用按钮
            FilledButton.tonal(
              onPressed: (_applyingIndex != null) ? null : () => _applyResult(index),
              style: FilledButton.styleFrom(
                minimumSize: const Size(0, 36),
                padding: const EdgeInsets.symmetric(horizontal: 10),
                backgroundColor:
                    isApplied ? Colors.green.withOpacity(0.15) : null,
              ),
              child: isApplying
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : isApplied
                      ? const Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(Icons.check, size: 16, color: Colors.green),
                            SizedBox(width: 4),
                            Text('已应用',
                                style: TextStyle(
                                    fontSize: 12, color: Colors.green)),
                          ],
                        )
                      : const Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(Icons.download, size: 16),
                            SizedBox(width: 4),
                            Text('应用', style: TextStyle(fontSize: 12)),
                          ],
                        ),
            ),
          ],
        ),
      ),
    );
  }

  // ============================================================
  // Tab 2: 自动扫描
  // ============================================================

  Widget _buildScanTab(ColorScheme cs) {
    final comic = _comic;
    final isNovel = comic != null && comic.isNovel;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // 说明
        Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: cs.surfaceContainerLow,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.auto_fix_high, size: 20, color: cs.primary),
                  const SizedBox(width: 8),
                  Text(
                    '自动刮削流程',
                    style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 15,
                        color: cs.onSurface),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              if (isNovel) ...[
                _buildStepItem(cs, '1', '尝试从 EPUB OPF 提取本地元数据'),
                _buildStepItem(cs, '2', '在线搜索兜底（Google Books / AniList / Bangumi）'),
              ] else ...[
                _buildStepItem(cs, '1', '尝试从 ComicInfo.xml 提取本地元数据'),
                _buildStepItem(cs, '2', '在线搜索兜底（AniList / Bangumi / MangaDex 等）'),
              ],
              _buildStepItem(cs, '3', '自动应用最佳匹配结果'),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // 扫描按钮
        SizedBox(
          width: double.infinity,
          child: FilledButton.icon(
            onPressed: _scanning ? null : _doScan,
            icon: _scanning
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : Icon(isNovel ? Icons.menu_book : Icons.document_scanner),
            label: Text(_scanning
                ? '正在扫描...'
                : isNovel
                    ? '扫描小说元数据'
                    : '扫描漫画元数据'),
          ),
        ),
        const SizedBox(height: 16),

        // 扫描结果
        if (_scanResult != null)
          _buildSuccessBanner(_scanResult!),

        if (_scanError != null)
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: cs.errorContainer,
              borderRadius: BorderRadius.circular(10),
            ),
            child: Row(
              children: [
                Icon(Icons.warning_amber_rounded,
                    size: 20, color: cs.error),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    _scanError!,
                    style: TextStyle(fontSize: 13, color: cs.onErrorContainer),
                  ),
                ),
              ],
            ),
          ),

        // 当前元数据展示
        const SizedBox(height: 24),
        Text('当前元数据',
            style: TextStyle(
                fontWeight: FontWeight.w600,
                fontSize: 14,
                color: cs.onSurface)),
        const SizedBox(height: 8),
        _buildCurrentMetadata(cs),
      ],
    );
  }

  Widget _buildStepItem(ColorScheme cs, String step, String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 20,
            height: 20,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: cs.primary.withOpacity(0.15),
              shape: BoxShape.circle,
            ),
            child: Text(step,
                style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: cs.primary)),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: TextStyle(fontSize: 13, color: cs.onSurfaceVariant)),
          ),
        ],
      ),
    );
  }

  /// 当前元数据信息卡
  Widget _buildCurrentMetadata(ColorScheme cs) {
    final comic = _comic;
    if (comic == null) return const SizedBox.shrink();

    final fields = <MapEntry<IconData, MapEntry<String, String?>>>[
      MapEntry(Icons.person, MapEntry('作者', comic.author)),
      MapEntry(Icons.business, MapEntry('出版社', comic.publisher)),
      MapEntry(Icons.calendar_today, MapEntry('年份', comic.year?.toString())),
      MapEntry(Icons.language, MapEntry('语言', comic.language)),
      MapEntry(Icons.category, MapEntry('类型', comic.genre)),
    ];

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            ...fields.where((f) => f.value.value != null && f.value.value!.isNotEmpty).map((f) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(
                  children: [
                    Icon(f.key, size: 16, color: cs.outline),
                    const SizedBox(width: 8),
                    Text('${f.value.key}：',
                        style: TextStyle(
                            fontSize: 12, color: cs.onSurfaceVariant)),
                    Expanded(
                      child: Text(f.value.value!,
                          style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w500,
                              color: cs.onSurface)),
                    ),
                  ],
                ),
              );
            }),
            if (comic.description != null && comic.description!.isNotEmpty) ...[
              const Divider(height: 16),
              SizedBox(
                width: double.infinity,
                child: Text(
                  comic.description!,
                  style: TextStyle(fontSize: 12, color: cs.onSurfaceVariant),
                  maxLines: 4,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
            if (fields.every(
                (f) => f.value.value == null || f.value.value!.isEmpty) &&
                (comic.description == null || comic.description!.isEmpty))
              Center(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      Icon(Icons.info_outline, size: 32, color: cs.outline),
                      const SizedBox(height: 8),
                      Text('暂无元数据',
                          style: TextStyle(color: cs.outline, fontSize: 13)),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  // ============================================================
  // Tab 3: 手动编辑
  // ============================================================

  Widget _buildEditTab(ColorScheme cs) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // 两列布局
        Row(
          children: [
            Expanded(
              child: _buildTextField('作者', _authorCtrl, Icons.person),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _buildTextField('出版社', _publisherCtrl, Icons.business),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _buildTextField('年份', _yearCtrl, Icons.calendar_today,
                  keyboardType: TextInputType.number),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _buildTextField('语言', _langCtrl, Icons.language),
            ),
          ],
        ),
        const SizedBox(height: 12),
        _buildTextField('类型（逗号分隔）', _genreCtrl, Icons.category),
        const SizedBox(height: 12),
        _buildTextField('简介', _descCtrl, Icons.description,
            maxLines: 5),
        const SizedBox(height: 20),

        // 保存按钮
        FilledButton.icon(
          onPressed: _saving ? null : _saveMetadata,
          icon: _saving
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: Colors.white),
                )
              : const Icon(Icons.save),
          label: Text(_saving ? '保存中...' : '保存元数据'),
        ),
      ],
    );
  }

  Widget _buildTextField(
      String label, TextEditingController ctrl, IconData icon,
      {int maxLines = 1, TextInputType? keyboardType}) {
    return TextField(
      controller: ctrl,
      maxLines: maxLines,
      keyboardType: keyboardType,
      decoration: InputDecoration(
        labelText: label,
        prefixIcon: Icon(icon, size: 20),
        isDense: true,
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
      ),
    );
  }

  /// 成功横幅
  Widget _buildSuccessBanner(String message) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.green.withOpacity(0.1),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.green.withOpacity(0.3)),
      ),
      child: Row(
        children: [
          const Icon(Icons.check_circle, size: 20, color: Colors.green),
          const SizedBox(width: 10),
          Expanded(
            child: Text(message,
                style: const TextStyle(fontSize: 13, color: Colors.green)),
          ),
        ],
      ),
    );
  }
}

// ============================================================
// 批量刮削对话框（可从设置页调用）
// ============================================================

class BatchMetadataDialog extends ConsumerStatefulWidget {
  const BatchMetadataDialog({super.key});

  @override
  ConsumerState<BatchMetadataDialog> createState() =>
      _BatchMetadataDialogState();
}

class _BatchMetadataDialogState extends ConsumerState<BatchMetadataDialog> {
  bool _running = false;
  bool _done = false;
  String _mode = 'missing';
  int _total = 0;
  int _current = 0;
  int _success = 0;
  int _failed = 0;
  String? _currentFilename;
  String? _currentStatus;
  StreamSubscription? _subscription;

  @override
  void dispose() {
    _subscription?.cancel();
    super.dispose();
  }

  void _start() {
    final api = ref.read(metadataApiProvider);
    setState(() {
      _running = true;
      _done = false;
      _total = 0;
      _current = 0;
      _success = 0;
      _failed = 0;
    });

    _subscription = api
        .batchScrape(mode: _mode, lang: 'zh')
        .listen(
      (event) {
        setState(() {
          switch (event.type) {
            case 'start':
              _total = event.total ?? 0;
              break;
            case 'progress':
              _current = event.current ?? _current;
              _currentFilename = event.filename;
              _currentStatus = event.status;
              if (event.status == 'success') _success++;
              if (event.status == 'failed' || event.status == 'skipped') {
                _failed++;
              }
              break;
            case 'complete':
              _running = false;
              _done = true;
              _success = event.success ?? _success;
              _failed = event.failed ?? _failed;
              break;
          }
        });
      },
      onError: (e) {
        setState(() {
          _running = false;
          _done = true;
        });
      },
      onDone: () {
        setState(() {
          _running = false;
          _done = true;
        });
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;

    return AlertDialog(
      title: const Row(
        children: [
          Icon(Icons.auto_fix_high, size: 22),
          SizedBox(width: 8),
          Text('批量刮削元数据', style: TextStyle(fontSize: 18)),
        ],
      ),
      content: SizedBox(
        width: double.maxFinite,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!_running && !_done) ...[
              // 模式选择
              Text('刮削模式',
                  style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                      color: cs.onSurface)),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(
                    value: 'missing',
                    label: Text('仅缺失', style: TextStyle(fontSize: 12)),
                    icon: Icon(Icons.find_replace, size: 18),
                  ),
                  ButtonSegment(
                    value: 'all',
                    label: Text('全部重刮', style: TextStyle(fontSize: 12)),
                    icon: Icon(Icons.refresh, size: 18),
                  ),
                ],
                selected: {_mode},
                onSelectionChanged: (v) =>
                    setState(() => _mode = v.first),
              ),
              const SizedBox(height: 12),
              Text(
                _mode == 'missing'
                    ? '仅对缺少元数据的条目进行在线搜索，已有元数据的不受影响。'
                    : '重新对所有条目进行元数据搜索，已有数据将被覆盖。',
                style: TextStyle(fontSize: 12, color: cs.outline),
              ),
            ],

            if (_running || _done) ...[
              // 进度条
              if (_total > 0) ...[
                LinearProgressIndicator(
                  value: _total > 0 ? _current / _total : null,
                  backgroundColor: cs.surfaceContainerHighest,
                ),
                const SizedBox(height: 8),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text('$_current / $_total',
                        style: TextStyle(fontSize: 12, color: cs.outline)),
                    Row(
                      children: [
                        Text('✅ $_success',
                            style: const TextStyle(
                                fontSize: 12, color: Colors.green)),
                        const SizedBox(width: 8),
                        Text('❌ $_failed',
                            style: const TextStyle(
                                fontSize: 12, color: Colors.red)),
                      ],
                    ),
                  ],
                ),
              ],
              const SizedBox(height: 8),
              // 当前文件
              if (_currentFilename != null && _running)
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: cs.surfaceContainerLow,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    children: [
                      if (_currentStatus == 'success')
                        const Icon(Icons.check_circle,
                            size: 16, color: Colors.green)
                      else if (_currentStatus == 'failed' ||
                          _currentStatus == 'skipped')
                        const Icon(Icons.cancel, size: 16, color: Colors.red)
                      else
                        SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: cs.primary,
                          ),
                        ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _currentFilename!,
                          style: TextStyle(fontSize: 11, color: cs.outline),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                ),
              // 完成提示
              if (_done) ...[
                const SizedBox(height: 12),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.green.withOpacity(0.1),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.check_circle_outline,
                          color: Colors.green),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          '批量刮削完成：成功 $_success，失败 $_failed',
                          style: const TextStyle(
                              fontSize: 13, color: Colors.green),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ],
        ),
      ),
      actions: [
        if (!_running)
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(_done ? '关闭' : '取消'),
          ),
        if (!_running && !_done)
          FilledButton.icon(
            onPressed: _start,
            icon: const Icon(Icons.rocket_launch, size: 18),
            label: const Text('开始刮削'),
          ),
      ],
    );
  }
}
