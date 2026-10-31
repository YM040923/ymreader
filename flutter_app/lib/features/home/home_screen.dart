import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api/work_api.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/comic_provider.dart';
import '../../data/providers/server_branding_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../widgets/continue_reading.dart';
import '../../widgets/directory_card.dart';
import '../../widgets/server_logo.dart';
import '../../widgets/work_card.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  final _scrollController = ScrollController();

  bool _directoryMode = false;
  String _directory = '';
  List<LibraryDirectoryNode> _directories = const [];
  List<LibraryDirectoryBreadcrumb> _breadcrumbs = const [];
  bool _loadingDirectories = false;

  Future<void> _loadDirectories(String libraryId) async {
    setState(() => _loadingDirectories = true);
    try {
      final listing = await ref
          .read(workApiProvider)
          .listLibraryDirectories(libraryId, parent: _directory);
      if (!mounted || !_directoryMode) return;
      setState(() {
        _directories = listing.directories;
        _breadcrumbs = listing.breadcrumbs;
        _loadingDirectories = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _directories = const [];
        _breadcrumbs = const [];
        _loadingDirectories = false;
      });
    }
  }

  Future<void> _toggleDirectoryMode(String libraryId) async {
    final notifier = ref.read(workListProvider.notifier);
    if (_directoryMode) {
      setState(() {
        _directoryMode = false;
        _directory = '';
        _directories = const [];
        _breadcrumbs = const [];
      });
      await notifier.load(
        query: ref
            .read(workListProvider)
            .query
            .copyWith(filterByDirectory: false, clearDirectory: true),
      );
      return;
    }
    setState(() {
      _directoryMode = true;
      _directory = '';
      _directories = const [];
      _breadcrumbs = const [];
    });
    await notifier.load(
      query: ref.read(workListProvider).query.copyWith(
            libraryIds: [libraryId],
            filterByDirectory: true,
            clearDirectory: true,
          ),
    );
    await _loadDirectories(libraryId);
  }

  Future<void> _openDirectory(String path, String libraryId) async {
    setState(() {
      _directory = path;
      _directories = const [];
    });
    await ref.read(workListProvider.notifier).load(
          query: ref.read(workListProvider).query.copyWith(
                directory: path,
                filterByDirectory: true,
                page: 1,
              ),
        );
    await _loadDirectories(libraryId);
  }

  Future<void> _selectLibrary(String? libraryId) async {    final notifier = ref.read(workListProvider.notifier);
    final query = ref.read(workListProvider).query;
    if (!_directoryMode) {
      await notifier.load(
        query: query.copyWith(libraryIds: libraryId == null ? [] : [libraryId]),
      );
      return;
    }
    if (libraryId == null) {
      setState(() {
        _directoryMode = false;
        _directory = '';
        _directories = const [];
        _breadcrumbs = const [];
      });
      await notifier.load(
        query: query.copyWith(
          libraryIds: [],
          filterByDirectory: false,
          clearDirectory: true,
        ),
      );
      return;
    }
    setState(() {
      _directory = '';
      _directories = const [];
      _breadcrumbs = const [];
    });
    await notifier.load(
      query: query.copyWith(
        libraryIds: [libraryId],
        filterByDirectory: true,
        clearDirectory: true,
        page: 1,
      ),
    );
    await _loadDirectories(libraryId);
  }

  Widget _buildDirectoryBar(ColorScheme colors, String? libraryId) {
    if (libraryId == null) return const SizedBox.shrink();
    return SizedBox(
      height: 42,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
        children: [
          _BreadcrumbChip(
            label: '全部',
            selected: _directory.isEmpty,
            onTap: () => _openDirectory('', libraryId),
          ),
          for (final crumb in _breadcrumbs) ...[
            Icon(Icons.chevron_right_rounded,
                size: 16, color: colors.onSurfaceVariant),
            _BreadcrumbChip(
              label: crumb.name,
              selected: crumb.path == _directory,
              onTap: () => _openDirectory(crumb.path, libraryId),
            ),
          ],
          if (_loadingDirectories)
            const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Center(
                child: SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            ),
        ],
      ),
    );
  }

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(workListProvider.notifier).load();
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 320) {
      ref.read(workListProvider.notifier).loadMore();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(workListProvider);
    final libraries = ref.watch(accessibleLibrariesProvider);
    final viewMode = ref.watch(viewModeProvider);
    final serverUrl = ref.watch(authProvider).serverUrl;
    final branding = ref.watch(currentServerBrandingProvider);
    final cs = Theme.of(context).colorScheme;
    final libraryList = libraries.valueOrNull;
    final activeLibraryName =
        libraryList != null && libraryList.length == 1 ? libraryList.first.name : '书库';
    final String? activeLibraryId = libraryList != null && libraryList.length == 1
        ? libraryList.first.id
        : (state.query.libraryIds.length == 1 ? state.query.libraryIds.first : null);
    final width = MediaQuery.sizeOf(context).width;
    final columns = width >= 900
        ? 7
        : width >= 650
            ? 5
            : width >= 420
                ? 4
                : 3;

    return Scaffold(
      body: RefreshIndicator(
        onRefresh: () async {
          final brandingRefresh =
              ref.read(serverBrandingProvider.notifier).refresh(force: true);
          ref.invalidate(accessibleLibrariesProvider);
          await ref.read(workListProvider.notifier).load();
          if (_directoryMode && activeLibraryId != null) {
            await _loadDirectories(activeLibraryId);
          }
          await brandingRefresh;
        },
        child: CustomScrollView(
          controller: _scrollController,
          slivers: [
            SliverAppBar(
              floating: true,
              snap: true,
              toolbarHeight:
                  56 + (MediaQuery.textScalerOf(context).scale(16) - 16) * 2,
              title: Row(
                children: [
                  ServerLogo(branding: branding, size: 30),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(branding.name,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 16)),
                        Text(activeLibraryName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                                fontSize: 11, color: cs.onSurfaceVariant)),
                      ],
                    ),
                  ),
                ],
              ),
              actions: [
                IconButton(
                  tooltip: _directoryMode ? '退出目录浏览' : '按目录浏览',
                  onPressed: activeLibraryId == null
                      ? null
                      : () => _toggleDirectoryMode(activeLibraryId),
                  icon: Icon(
                    _directoryMode
                        ? Icons.folder_open_rounded
                        : Icons.folder_outlined,
                    color: _directoryMode ? cs.primary : null,
                  ),
                ),
                IconButton(
                  tooltip: viewMode == ViewMode.grid ? '列表视图' : '封面墙',
                  onPressed: () {
                    ref.read(viewModeProvider.notifier).state =
                        viewMode == ViewMode.grid
                            ? ViewMode.list
                            : ViewMode.grid;
                  },
                  icon: Icon(
                    viewMode == ViewMode.grid
                        ? Icons.view_agenda_outlined
                        : Icons.grid_view_rounded,
                  ),
                ),
                PopupMenuButton<String>(
                  tooltip: '排序',
                  icon: const Icon(Icons.swap_vert_rounded),
                  onSelected: (value) {
                    final order = value == 'title' ? 'asc' : 'desc';
                    ref.read(workListProvider.notifier).load(
                          query: state.query.copyWith(
                            sortBy: value,
                            sortOrder: order,
                          ),
                        );
                  },
                  itemBuilder: (_) => const [
                    PopupMenuItem(value: 'title', child: Text('标题')),
                    PopupMenuItem(value: 'addedAt', child: Text('最近添加')),
                    PopupMenuItem(value: 'lastReadAt', child: Text('最近阅读')),
                    PopupMenuItem(value: 'rating', child: Text('评分')),
                  ],
                ),
              ],
            ),
            const SliverToBoxAdapter(child: ContinueReading()),
            SliverToBoxAdapter(
              child: SizedBox(
                height: 52,
                child: libraries.when(
                  loading: () => const SizedBox.shrink(),
                  error: (_, __) => const SizedBox.shrink(),
                  data: (items) => ListView(
                    scrollDirection: Axis.horizontal,
                    padding:
                        const EdgeInsets.symmetric(horizontal: 14, vertical: 7),
                    children: [
                      _LibraryChip(
                        label: '全部',
                        selected: state.query.libraryIds.isEmpty,
                        onTap: () => _selectLibrary(null),
                      ),
                      for (final library in items)
                        _LibraryChip(
                          label: library.name,
                          count: library.workCount,
                          selected: state.query.libraryIds.length == 1 &&
                              state.query.libraryIds.first == library.id,
                          onTap: () => _selectLibrary(library.id),
                        ),
                    ],
                  ),
                ),
              ),
            ),
            if (_directoryMode)
              SliverToBoxAdapter(
                child: _buildDirectoryBar(cs, activeLibraryId),
              ),
            if ((state.isLoading || _loadingDirectories) &&
                state.works.isEmpty &&
                _directories.isEmpty)
              const SliverFillRemaining(
                child: Center(child: CircularProgressIndicator()),
              )
            else if (state.error != null &&
                state.works.isEmpty &&
                _directories.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: _ErrorState(
                  message: state.error!,
                  onRetry: () => ref.read(workListProvider.notifier).load(),
                ),
              )
            else if (state.works.isEmpty && _directories.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: Center(
                  child: Text(
                    _directoryMode ? '这个目录还没有内容' : '这个书库还没有漫画',
                  ),
                ),
              )
            else if (viewMode == ViewMode.grid)
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(12, 6, 12, 20),
                sliver: SliverGrid.builder(
                  gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: columns,
                    childAspectRatio: 0.59,
                    crossAxisSpacing: 10,
                    mainAxisSpacing: 14,
                  ),
                  itemCount: _directories.length + state.works.length,
                  itemBuilder: (_, index) {
                    if (index < _directories.length) {
                      final node = _directories[index];
                      return DirectoryCard(
                        node: node,
                        serverUrl: serverUrl,
                        onTap: () =>
                            _openDirectory(node.path, activeLibraryId!),
                      );
                    }
                    final work = state.works[index - _directories.length];
                    return WorkCard(
                      work: work,
                      serverUrl: serverUrl,
                    );
                  },
                ),
              )
            else
              SliverList.builder(
                itemCount: _directories.length + state.works.length,
                itemBuilder: (_, index) {
                  if (index < _directories.length) {
                    final node = _directories[index];
                    return DirectoryCard(
                      node: node,
                      serverUrl: serverUrl,
                      isGrid: false,
                      onTap: () => _openDirectory(node.path, activeLibraryId!),
                    );
                  }
                  final work = state.works[index - _directories.length];
                  return WorkCard(
                    work: work,
                    serverUrl: serverUrl,
                    isGrid: false,
                  );
                },
              ),
            if (state.isLoadingMore)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.all(20),
                  child: Center(child: CircularProgressIndicator()),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _LibraryChip extends StatelessWidget {
  final String label;
  final int? count;
  final bool selected;
  final VoidCallback onTap;

  const _LibraryChip({
    required this.label,
    this.count,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: ChoiceChip(
        selected: selected,
        onSelected: (_) => onTap(),
        label: Text(count == null ? label : '$label  $count'),
        showCheckmark: false,
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorState({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_rounded, size: 48),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh_rounded),
              label: const Text('重试'),
            ),
          ],
        ),
      ),
    );
  }
}

class _BreadcrumbChip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _BreadcrumbChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(right: 2),
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: selected
                ? colors.primary.withValues(alpha: 0.15)
                : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
              color: selected ? colors.primary : colors.onSurfaceVariant,
            ),
          ),
        ),
      ),
    );
  }
}
