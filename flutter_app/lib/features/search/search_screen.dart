import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../data/api/api_client.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/comic_list_tile.dart';
import '../../data/models/comic.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/comic_provider.dart';
import '../../data/api/comic_api.dart';
import '../../widgets/animations.dart';

/// 搜索页面 — 极简优雅风格
class SearchScreen extends ConsumerStatefulWidget {
  const SearchScreen({super.key});

  @override
  ConsumerState<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends ConsumerState<SearchScreen> {
  final _searchCtrl = TextEditingController();
  final _focusNode = FocusNode();
  List<Comic> _results = [];
  bool _searching = false;
  bool _hasSearched = false;
  String? _selectedTag;
  String? _selectedCategory;

  List<Tag> _tags = [];
  List<Category> _categories = [];

  @override
  void initState() {
    super.initState();
    _loadFilters();
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  Future<void> _loadFilters() async {
    final tagsAsync = ref.read(tagsProvider.future);
    final catsAsync = ref.read(categoriesProvider.future);
    try {
      final tags = await tagsAsync;
      final cats = await catsAsync;
      if (mounted) {
        setState(() {
          _tags = tags;
          _categories = cats;
        });
      }
    } catch (_) {}
  }

  Future<void> _search() async {
    final query = _searchCtrl.text.trim();
    if (query.isEmpty && _selectedTag == null && _selectedCategory == null) return;

    _focusNode.unfocus();
    setState(() {
      _searching = true;
      _hasSearched = true;
    });
    try {
      final api = ref.read(comicApiProvider);
      final data = await api.listComics(
        search: query.isNotEmpty ? query : null,
        tag: _selectedTag,
        category: _selectedCategory,
      );
      final list = (data['comics'] as List<dynamic>?)
              ?.map((e) => Comic.fromJson(e))
              .toList() ??
          [];
      setState(() {
        _results = list;
        _searching = false;
      });
    } catch (_) {
      setState(() => _searching = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final serverUrl = ref.watch(authProvider).serverUrl;
    final cs = Theme.of(context).colorScheme;
    final viewMode = ref.watch(viewModeProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('搜索'),
        actions: [
          IconButton(
            icon: Icon(
              viewMode == ViewMode.grid
                  ? Icons.view_agenda_outlined
                  : Icons.grid_view_rounded,
            ),
            tooltip: viewMode == ViewMode.grid ? '列表视图' : '网格视图',
            onPressed: () {
              ref.read(viewModeProvider.notifier).state =
                  viewMode == ViewMode.grid ? ViewMode.list : ViewMode.grid;
            },
          ),
        ],
      ),
      body: Column(
        children: [
          // ─── 搜索栏 ───
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 4, 20, 8),
            child: TextField(
              controller: _searchCtrl,
              focusNode: _focusNode,
              decoration: InputDecoration(
                hintText: '搜索漫画、小说...',
                prefixIcon: Icon(Icons.search_rounded,
                    color: cs.onSurfaceVariant.withOpacity(0.5)),
                suffixIcon: _searchCtrl.text.isNotEmpty
                    ? IconButton(
                        icon: Icon(Icons.close_rounded,
                            size: 20, color: cs.onSurfaceVariant.withOpacity(0.5)),
                        onPressed: () {
                          _searchCtrl.clear();
                          setState(() {});
                        },
                      )
                    : null,
              ),
              onSubmitted: (_) => _search(),
              onChanged: (_) => setState(() {}),
            ),
          ),

          // ─── 标签筛选 ───
          if (_tags.isNotEmpty)
            SizedBox(
              height: 38,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 20),
                itemCount: _tags.length,
                separatorBuilder: (_, __) => const SizedBox(width: 8),
                itemBuilder: (_, i) {
                  final tag = _tags[i];
                  final selected = _selectedTag == tag.name;
                  return FilterChip(
                    label: Text(tag.name, style: const TextStyle(fontSize: 12)),
                    selected: selected,
                    onSelected: (v) {
                      HapticFeedback.selectionClick();
                      setState(() => _selectedTag = v ? tag.name : null);
                      _search();
                    },
                    visualDensity: VisualDensity.compact,
                    showCheckmark: false,
                    selectedColor: cs.primary.withOpacity(0.12),
                    labelStyle: TextStyle(
                      color: selected ? cs.primary : cs.onSurfaceVariant,
                      fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                    ),
                  );
                },
              ),
            ),

          // ─── 分类筛选 ───
          if (_categories.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: SizedBox(
                height: 38,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 20),
                  itemCount: _categories.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (_, i) {
                    final cat = _categories[i];
                    final selected = _selectedCategory == cat.slug;
                    return FilterChip(
                      label: Text(cat.name, style: const TextStyle(fontSize: 12)),
                      selected: selected,
                      onSelected: (v) {
                        HapticFeedback.selectionClick();
                        setState(() => _selectedCategory = v ? cat.slug : null);
                        _search();
                      },
                      visualDensity: VisualDensity.compact,
                      showCheckmark: false,
                      selectedColor: cs.secondary.withOpacity(0.12),
                      labelStyle: TextStyle(
                        color: selected ? cs.secondary : cs.onSurfaceVariant,
                        fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                      ),
                    );
                  },
                ),
              ),
            ),

          const SizedBox(height: 12),

          // ─── 搜索按钮 ───
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: SizedBox(
              height: 46,
              child: FilledButton(
                onPressed: _searching ? null : _search,
                style: FilledButton.styleFrom(
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                child: _searching
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                            strokeWidth: 2.5, color: Colors.white))
                    : const Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(Icons.search_rounded, size: 20),
                          SizedBox(width: 8),
                          Text('搜索', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
                        ],
                      ),
              ),
            ),
          ),

          const SizedBox(height: 12),

          // ─── 搜索结果 ───
          Expanded(
            child: _results.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          _hasSearched
                              ? Icons.search_off_rounded
                              : Icons.search_rounded,
                          size: 48,
                          color: cs.onSurfaceVariant.withOpacity(0.2),
                        ),
                        const SizedBox(height: 16),
                        Text(
                          _hasSearched ? '没有找到相关内容' : '输入关键词或选择标签搜索',
                          style: TextStyle(
                            color: cs.onSurfaceVariant.withOpacity(0.5),
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                  )
                : Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // 结果数量
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 4, 20, 8),
                        child: Text(
                          '找到 ${_results.length} 个结果',
                          style: TextStyle(
                            fontSize: 12,
                            color: cs.onSurfaceVariant.withOpacity(0.5),
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      Expanded(
                        child: viewMode == ViewMode.list
                            ? ListView.builder(
                                itemCount: _results.length,
                                itemBuilder: (context, index) {
                                  final comic = _results[index];
                                  return StaggeredFadeSlide(
                                    index: index,
                                    child: ComicListTile(
                                      comic: comic,
                                      serverUrl: serverUrl,
                                      onTap: () =>
                                          context.push('/comic/${comic.id}'),
                                    ),
                                  );
                                },
                              )
                            : _buildSearchGrid(context, serverUrl),
                      ),
                    ],
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchGrid(BuildContext context, String serverUrl) {
    final width = MediaQuery.of(context).size.width;
    final crossAxisCount = width > 900
        ? 6
        : width > 600
            ? 4
            : width > 400
                ? 3
                : 2;

    return GridView.builder(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: crossAxisCount,
        childAspectRatio: 0.62,
        crossAxisSpacing: 12,
        mainAxisSpacing: 14,
      ),
      itemCount: _results.length,
      itemBuilder: (context, index) {
        final comic = _results[index];
        return _SearchGridCard(
          comic: comic,
          serverUrl: serverUrl,
          onTap: () => context.push('/comic/${comic.id}'),
        );
      },
    );
  }
}

/// 搜索结果网格卡片
class _SearchGridCard extends StatelessWidget {
  final Comic comic;
  final String serverUrl;
  final VoidCallback onTap;

  const _SearchGridCard({
    required this.comic,
    required this.serverUrl,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final thumbUrl = getImageUrl(serverUrl, comic.id, thumbnail: true);

    return GestureDetector(
      onTap: onTap,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withOpacity(0.08),
                    blurRadius: 12,
                    offset: const Offset(0, 4),
                  ),
                ],
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    AuthenticatedImage(
                      imageUrl: thumbUrl,
                      fit: BoxFit.cover,
                      placeholder: Container(
                        color: cs.surfaceContainerHighest,
                        child: Center(
                          child: Icon(Icons.image_outlined,
                              size: 28, color: cs.onSurfaceVariant.withOpacity(0.3)),
                        ),
                      ),
                      errorWidget: Container(
                        color: cs.surfaceContainerHighest,
                        child: Center(
                          child: Icon(Icons.broken_image_outlined,
                              size: 28, color: cs.onSurfaceVariant.withOpacity(0.3)),
                        ),
                      ),
                    ),
                    if (comic.isFavorite)
                      Positioned(
                        top: 6,
                        right: 6,
                        child: Container(
                          width: 26,
                          height: 26,
                          decoration: BoxDecoration(
                            color: Colors.black.withOpacity(0.3),
                            borderRadius: BorderRadius.circular(7),
                          ),
                          child: const Icon(
                            Icons.favorite_rounded,
                            color: Color(0xFFFF6B6B),
                            size: 14,
                          ),
                        ),
                      ),
                    if (comic.isNovel)
                      Positioned(
                        top: 6,
                        left: 6,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 7, vertical: 3),
                          decoration: BoxDecoration(
                            color: Colors.black.withOpacity(0.5),
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: const Text(
                            '小说',
                            style: TextStyle(
                              color: Colors.white,
                              fontSize: 10,
                              fontWeight: FontWeight.w600,
                              letterSpacing: 0.5,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(2, 8, 2, 0),
            child: Text(
              comic.title,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w600,
                    fontSize: 12,
                    height: 1.3,
                    color: cs.onSurface,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}

