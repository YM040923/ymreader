import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/api_client.dart';
import '../../data/api/comic_api.dart';
import '../../data/models/comic.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/comic_provider.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/comic_list_tile.dart';

/// 收藏管理状态
class _FavoritesState {
  final List<Comic> comics;
  final bool isLoading;
  final bool hasMore;
  final int page;
  final String? error;

  const _FavoritesState({
    this.comics = const [],
    this.isLoading = false,
    this.hasMore = true,
    this.page = 1,
    this.error,
  });
}

/// 收藏管理页面
class FavoritesScreen extends ConsumerStatefulWidget {
  const FavoritesScreen({super.key});

  @override
  ConsumerState<FavoritesScreen> createState() => _FavoritesScreenState();
}

class _FavoritesScreenState extends ConsumerState<FavoritesScreen> {
  final _scrollController = ScrollController();
  var _state = const _FavoritesState();
  String _sortBy = 'addedAt';
  String _sortOrder = 'desc';
  String? _typeFilter;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadFavorites());
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 300) {
      _loadMore();
    }
  }

  Future<void> _loadFavorites() async {
    setState(() {
      _state = const _FavoritesState(isLoading: true);
    });
    try {
      final api = ref.read(comicApiProvider);
      final data = await api.listComics(
        page: 1,
        limit: 20,
        sort: _sortBy,
        order: _sortOrder,
        type: _typeFilter,
        favoritesOnly: true,
      );
      final list = (data['comics'] as List<dynamic>?)
              ?.map((e) => Comic.fromJson(e))
              .toList() ??
          [];
      setState(() {
        _state = _FavoritesState(
          comics: list,
          isLoading: false,
          hasMore: list.length >= 20,
          page: 1,
        );
      });
    } catch (e) {
      setState(() {
        _state = _FavoritesState(
          isLoading: false,
          error: '加载失败: $e',
        );
      });
    }
  }

  Future<void> _loadMore() async {
    if (_state.isLoading || !_state.hasMore) return;
    final nextPage = _state.page + 1;
    setState(() {
      _state = _FavoritesState(
        comics: _state.comics,
        isLoading: true,
        hasMore: _state.hasMore,
        page: _state.page,
      );
    });
    try {
      final api = ref.read(comicApiProvider);
      final data = await api.listComics(
        page: nextPage,
        limit: 20,
        sort: _sortBy,
        order: _sortOrder,
        type: _typeFilter,
        favoritesOnly: true,
      );
      final newList = (data['comics'] as List<dynamic>?)
              ?.map((e) => Comic.fromJson(e))
              .toList() ??
          [];
      setState(() {
        _state = _FavoritesState(
          comics: [..._state.comics, ...newList],
          isLoading: false,
          hasMore: newList.length >= 20,
          page: nextPage,
        );
      });
    } catch (e) {
      setState(() {
        _state = _FavoritesState(
          comics: _state.comics,
          isLoading: false,
          hasMore: _state.hasMore,
          page: _state.page,
          error: '加载更多失败: $e',
        );
      });
    }
  }

  Future<void> _toggleFavorite(String comicId) async {
    try {
      final api = ref.read(comicApiProvider);
      await api.toggleFavorite(comicId);
      // 从列表中移除（因为取消收藏后不应再在此页面显示）
      setState(() {
        _state = _FavoritesState(
          comics: _state.comics.where((c) => c.id != comicId).toList(),
          isLoading: false,
          hasMore: _state.hasMore,
          page: _state.page,
        );
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('已取消收藏'),
            behavior: SnackBarBehavior.floating,
            duration: Duration(seconds: 2),
          ),
        );
      }
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final authState = ref.watch(authProvider);
    final serverUrl = authState.serverUrl;
    final viewMode = ref.watch(viewModeProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text('我的收藏 (${_state.comics.length})'),
        actions: [
          // 视图模式切换按钮
          IconButton(
            icon: Icon(
              viewMode == ViewMode.grid ? Icons.view_list : Icons.grid_view,
            ),
            tooltip: viewMode == ViewMode.grid ? '切换列表模式' : '切换网格模式',
            onPressed: () {
              ref.read(viewModeProvider.notifier).state =
                  viewMode == ViewMode.grid ? ViewMode.list : ViewMode.grid;
            },
          ),
          // 排序
          PopupMenuButton<String>(
            icon: const Icon(Icons.sort),
            tooltip: '排序',
            onSelected: (sort) {
              setState(() {
                _sortBy = sort;
                _sortOrder = sort == 'title' ? 'asc' : 'desc';
              });
              _loadFavorites();
            },
            itemBuilder: (_) => [
              _buildSortItem('addedAt', '添加时间'),
              _buildSortItem('title', '标题'),
              _buildSortItem('lastReadAt', '最近阅读'),
              _buildSortItem('rating', '评分'),
            ],
          ),
          // 筛选
          PopupMenuButton<String?>(
            icon: const Icon(Icons.filter_list),
            tooltip: '筛选',
            onSelected: (type) {
              setState(() => _typeFilter = type);
              _loadFavorites();
            },
            itemBuilder: (_) => [
              PopupMenuItem<String?>(
                value: null,
                child: Row(
                  children: [
                    if (_typeFilter == null) const Icon(Icons.check, size: 18),
                    if (_typeFilter == null) const SizedBox(width: 8),
                    const Text('全部'),
                  ],
                ),
              ),
              PopupMenuItem<String?>(
                value: 'comic',
                child: Row(
                  children: [
                    if (_typeFilter == 'comic') const Icon(Icons.check, size: 18),
                    if (_typeFilter == 'comic') const SizedBox(width: 8),
                    const Text('漫画'),
                  ],
                ),
              ),
              PopupMenuItem<String?>(
                value: 'novel',
                child: Row(
                  children: [
                    if (_typeFilter == 'novel') const Icon(Icons.check, size: 18),
                    if (_typeFilter == 'novel') const SizedBox(width: 8),
                    const Text('小说'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _loadFavorites,
        child: _state.comics.isEmpty && _state.isLoading
            ? const Center(child: CircularProgressIndicator())
            : _state.comics.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.favorite_border,
                            size: 64, color: cs.onSurfaceVariant),
                        const SizedBox(height: 16),
                        Text('暂无收藏',
                            style: Theme.of(context).textTheme.titleMedium),
                        const SizedBox(height: 8),
                        Text('浏览书架并添加收藏',
                            style: TextStyle(color: cs.onSurfaceVariant)),
                      ],
                    ),
                  )
                : viewMode == ViewMode.list
                    ? ListView.builder(
                        controller: _scrollController,
                        padding: const EdgeInsets.only(bottom: 16),
                        itemCount:
                            _state.comics.length + (_state.hasMore ? 1 : 0),
                        itemBuilder: (context, index) {
                          if (index >= _state.comics.length) {
                            return const Center(
                              child: Padding(
                                padding: EdgeInsets.all(16),
                                child: CircularProgressIndicator(),
                              ),
                            );
                          }
                          return ComicListTile(
                            comic: _state.comics[index],
                            serverUrl: serverUrl,
                            onTap: () => context
                                .push('/comic/${_state.comics[index].id}'),
                            onFavoriteToggle: () =>
                                _toggleFavorite(_state.comics[index].id),
                          );
                        },
                      )
                    : _buildGridView(context, serverUrl),
      ),
    );
  }

  /// 网格视图
  Widget _buildGridView(BuildContext context, String serverUrl) {
    final width = MediaQuery.of(context).size.width;
    final crossAxisCount = width > 900
        ? 6
        : width > 600
            ? 4
            : width > 400
                ? 3
                : 2;

    return GridView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.all(8),
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: crossAxisCount,
        childAspectRatio: 0.65,
        crossAxisSpacing: 8,
        mainAxisSpacing: 8,
      ),
      itemCount: _state.comics.length + (_state.hasMore ? 1 : 0),
      itemBuilder: (context, index) {
        if (index >= _state.comics.length) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(16),
              child: CircularProgressIndicator(),
            ),
          );
        }
        final comic = _state.comics[index];
        return _FavoriteGridCard(
          comic: comic,
          serverUrl: serverUrl,
          onTap: () => context.push('/comic/${comic.id}'),
          onRemove: () => _toggleFavorite(comic.id),
        );
      },
    );
  }

  PopupMenuItem<String> _buildSortItem(String value, String label) {
    return PopupMenuItem(
      value: value,
      child: Row(
        children: [
          if (_sortBy == value) const Icon(Icons.check, size: 18),
          if (_sortBy == value) const SizedBox(width: 8),
          Text(label),
        ],
      ),
    );
  }
}

/// 收藏网格卡片组件
class _FavoriteGridCard extends StatelessWidget {
  final Comic comic;
  final String serverUrl;
  final VoidCallback onTap;
  final VoidCallback onRemove;

  const _FavoriteGridCard({
    required this.comic,
    required this.serverUrl,
    required this.onTap,
    required this.onRemove,
  });

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final thumbUrl = getImageUrl(serverUrl, comic.id, thumbnail: true);

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  AuthenticatedImage(
                    imageUrl: thumbUrl,
                    fit: BoxFit.cover,
                    placeholder: Container(
                      color: cs.surfaceContainerHighest,
                      child: const Center(
                        child: Icon(Icons.image_outlined, size: 32),
                      ),
                    ),
                    errorWidget: Container(
                      color: cs.surfaceContainerHighest,
                      child: Center(
                        child: Icon(Icons.broken_image_outlined,
                            size: 32, color: cs.onSurfaceVariant),
                      ),
                    ),
                  ),
                  // 阅读进度条
                  if (comic.progress > 0)
                    Positioned(
                      bottom: 0,
                      left: 0,
                      right: 0,
                      child: LinearProgressIndicator(
                        value: comic.progress / 100,
                        minHeight: 3,
                        backgroundColor: Colors.black38,
                        valueColor: AlwaysStoppedAnimation(cs.primary),
                      ),
                    ),
                  // 收藏图标（点击取消收藏）
                  Positioned(
                    top: 4,
                    right: 4,
                    child: GestureDetector(
                      onTap: onRemove,
                      child: const Icon(
                        Icons.favorite,
                        color: Colors.red,
                        size: 20,
                        shadows: [
                          Shadow(blurRadius: 4, color: Colors.black54),
                        ],
                      ),
                    ),
                  ),
                  // 类型标识
                  if (comic.isNovel)
                    Positioned(
                      top: 4,
                      left: 4,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: cs.tertiary,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '小说',
                          style: TextStyle(
                            color: cs.onTertiary,
                            fontSize: 10,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(8, 6, 8, 6),
              child: Text(
                comic.title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      fontWeight: FontWeight.w500,
                    ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

