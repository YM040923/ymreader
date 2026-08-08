import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/providers/auth_provider.dart';
import '../../data/providers/comic_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../widgets/continue_reading.dart';
import '../../widgets/work_card.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  final _scrollController = ScrollController();

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
          ref.invalidate(accessibleLibrariesProvider);
          await ref.read(workListProvider.notifier).load();
        },
        child: CustomScrollView(
          controller: _scrollController,
          slivers: [
            SliverAppBar(
              floating: true,
              snap: true,
              title: const Text('书库'),
              actions: [
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
                        onTap: () => ref
                            .read(workListProvider.notifier)
                            .load(query: state.query.copyWith(libraryIds: [])),
                      ),
                      for (final library in items)
                        _LibraryChip(
                          label: library.name,
                          count: library.workCount,
                          selected: state.query.libraryIds.length == 1 &&
                              state.query.libraryIds.first == library.id,
                          onTap: () => ref.read(workListProvider.notifier).load(
                                query: state.query
                                    .copyWith(libraryIds: [library.id]),
                              ),
                        ),
                    ],
                  ),
                ),
              ),
            ),
            if (state.isLoading && state.works.isEmpty)
              const SliverFillRemaining(
                child: Center(child: CircularProgressIndicator()),
              )
            else if (state.error != null && state.works.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: _ErrorState(
                  message: state.error!,
                  onRetry: () => ref.read(workListProvider.notifier).load(),
                ),
              )
            else if (state.works.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(child: Text('这个书库还没有漫画')),
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
                  itemCount: state.works.length,
                  itemBuilder: (_, index) {
                    final work = state.works[index];
                    return WorkCard(
                      work: work,
                      serverUrl: serverUrl,
                    );
                  },
                ),
              )
            else
              SliverList.builder(
                itemCount: state.works.length,
                itemBuilder: (_, index) {
                  final work = state.works[index];
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
