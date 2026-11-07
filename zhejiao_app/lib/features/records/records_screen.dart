import 'package:flutter/material.dart';
import '../../widgets/phosphor_icons.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/work_api.dart';
import '../../data/models/reading_history.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/cache_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/responsive.dart';
import '../../widgets/work_card.dart';
import '../shell/app_shell.dart';

/// 书架：顶部 书架 / 历史 / 缓存 三页；历史按日期分组，支持续看和编辑删除。
class RecordsScreen extends ConsumerStatefulWidget {
  const RecordsScreen({super.key});

  @override
  ConsumerState<RecordsScreen> createState() => _RecordsScreenState();
}

class _RecordsScreenState extends ConsumerState<RecordsScreen> {
  List<Work> _favorites = const [];
  List<ReadingHistoryItem> _history = const [];
  bool _loading = true;
  int _tab = 1;
  int _historyKind = 0;
  bool _editing = false;
  bool _initialTabSet = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final api = ref.read(workApiProvider);
      final results = await Future.wait([
        api.listWorks(const WorkQuery(
          pageSize: 200,
          favoritesOnly: true,
          sortBy: 'title',
        )),
        api.listReadingHistory(),
      ]);
      if (!mounted) return;
      setState(() {
        _favorites = (results[0] as WorkListResponse).works;
        _history = results[1] as List<ReadingHistoryItem>;
        if (!_initialTabSet) {
          _tab = _history.isNotEmpty ? 1 : 0;
          _initialTabSet = true;
        }
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _confirmDelete(ReadingHistoryItem item) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除历史记录'),
        content: Text('删除《${item.title}》的阅读历史，并清空它的阅读进度。此操作不可恢复。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    try {
      await ref.read(workApiProvider).deleteReadingHistory(
            workIds: item.isNovel ? const [] : [item.id],
            comicIds: item.isNovel ? [item.id] : const [],
          );
      await _load();
      ref.read(workListProvider.notifier).load();
      if (mounted) {
        setState(() => _editing = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('历史记录已删除')),
        );
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('删除失败，请重试')),
        );
      }
    }
  }

  Future<void> _confirmClearHistory() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('清空历史记录'),
        content: const Text('清空全部历史记录，并清空所有作品的阅读进度。此操作不可恢复。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('清空'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    try {
      await ref.read(workApiProvider).deleteReadingHistory(all: true);
      await _load();
      ref.read(workListProvider.notifier).load();
      if (mounted) {
        setState(() => _editing = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('历史记录已清空')),
        );
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('清空失败，请重试')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final serverUrl = ref.watch(authProvider).serverUrl;
    final width = MediaQuery.sizeOf(context).width;
    final columns = coverColumns(width);
    final pageHorizontal = pagePadding(width);
    final listPadding = pageHorizontal;
    final visibleHistory = _history
        .where((item) => _historyKind == 0 ? !item.isNovel : item.isNovel)
        .toList();

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        titleSpacing: pageHorizontal,
        title: Row(
          children: [
            _TopTab(
              label: '书架',
              selected: _tab == 0,
              onTap: () => setState(() {
                _tab = 0;
                _editing = false;
              }),
            ),
            _TopTab(
              label: '历史',
              selected: _tab == 1,
              onTap: () => setState(() => _tab = 1),
            ),
            _TopTab(
              label: '缓存',
              selected: _tab == 2,
              onTap: () => setState(() {
                _tab = 2;
                _editing = false;
              }),
            ),
          ],
        ),
        actions: [
          if (_tab == 1)
            IconButton(
              tooltip: _editing ? '完成' : '编辑',
              onPressed: () => setState(() => _editing = !_editing),
              icon: Icon(
                _editing
                    ? PhosphorIconsRegular.check
                    : PhosphorIconsRegular.pencilSimple,
                size: 20,
              ),
            ),
          if (_tab == 1 && _editing && visibleHistory.isNotEmpty)
            TextButton(
              onPressed: _confirmClearHistory,
              child: const Text('清空'),
            ),
          SizedBox(
            width: (pageHorizontal - 8).clamp(0.0, double.infinity),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : IndexedStack(
              index: _tab,
              children: [
                _ShelfPane(
                  works: _favorites,
                  serverUrl: serverUrl,
                  columns: columns,
                  onRefresh: _load,
                ),
                _HistoryPane(
                  items: visibleHistory,
                  kind: _historyKind,
                  editing: _editing,
                  serverUrl: serverUrl,
                  padding: listPadding,
                  onKindChanged: (value) =>
                      setState(() => _historyKind = value),
                  onRefresh: _load,
                  onDelete: _confirmDelete,
                  onResume: (item) {
                    final route =
                        item.readHref.isNotEmpty ? item.readHref : item.route;
                    context.push(route);
                  },
                ),
                const _CachePane(),
              ],
            ),
    );
  }
}

class _TopTab extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _TopTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.only(right: 22),
        child: Text(
          label,
          style: TextStyle(
            fontSize: selected ? 22 : 17,
            fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
            color: selected ? colors.primary : colors.onSurfaceVariant,
          ),
        ),
      ),
    );
  }
}

class _ShelfPane extends ConsumerWidget {
  final List<Work> works;
  final String serverUrl;
  final int columns;
  final Future<void> Function() onRefresh;

  const _ShelfPane({
    required this.works,
    required this.serverUrl,
    required this.columns,
    required this.onRefresh,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (works.isEmpty) {
      return RefreshIndicator(
        onRefresh: onRefresh,
        child: ListView(
          children: [
            const SizedBox(height: 180),
            const Center(child: Text('还没有收藏作品')),
            SizedBox(height: shellBarClearance(context)),
          ],
        ),
      );
    }
    final horizontal = pagePadding(
      MediaQuery.sizeOf(context).width,
      phone: 12,
    );
    return RefreshIndicator(
      onRefresh: onRefresh,
      child: GridView.builder(
        padding: EdgeInsets.fromLTRB(
          horizontal,
          12,
          horizontal,
          12 + shellBarClearance(context),
        ),
        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: columns,
          childAspectRatio: 0.62,
          crossAxisSpacing: MediaQuery.sizeOf(context).width >= 860 ? 16 : 8,
          mainAxisSpacing: MediaQuery.sizeOf(context).width >= 860 ? 24 : 16,
        ),
        itemCount: works.length,
        itemBuilder: (_, index) => WorkCard(
          work: works[index],
          serverUrl: serverUrl,
          showMeta: true,
          onFavorite: () async {
            await ref.read(workApiProvider).setFavorite(works[index].id, false);
            await onRefresh();
          },
        ),
      ),
    );
  }
}

class _HistoryPane extends StatelessWidget {
  final List<ReadingHistoryItem> items;
  final int kind;
  final bool editing;
  final String serverUrl;
  final double padding;
  final ValueChanged<int> onKindChanged;
  final Future<void> Function() onRefresh;
  final Future<void> Function(ReadingHistoryItem item) onDelete;
  final void Function(ReadingHistoryItem item) onResume;

  const _HistoryPane({
    required this.items,
    required this.kind,
    required this.editing,
    required this.serverUrl,
    required this.padding,
    required this.onKindChanged,
    required this.onRefresh,
    required this.onDelete,
    required this.onResume,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final groups = <String, List<ReadingHistoryItem>>{};
    for (final item in items) {
      groups.putIfAbsent(_dateLabel(item.lastReadAt), () => []).add(item);
    }
    // 平板横屏放两列：每列约 580dp，「续看」不再和文字隔着一大片空白
    final twoColumns = MediaQuery.sizeOf(context).width >= 1100;

    return Column(
      children: [
        Padding(
          padding: EdgeInsets.only(
            left: (padding - 16).clamp(0.0, double.infinity),
          ),
          child: SizedBox(
            height: 40,
            child: Row(
              children: [
                _KindTab(
                  label: '漫画',
                  selected: kind == 0,
                  onTap: () => onKindChanged(0),
                ),
                _KindTab(
                  label: '小说',
                  selected: kind == 1,
                  onTap: () => onKindChanged(1),
                ),
              ],
            ),
          ),
        ),
        Expanded(
          child: items.isEmpty
              ? RefreshIndicator(
                  onRefresh: onRefresh,
                  child: ListView(
                    children: [
                      const SizedBox(height: 150),
                      const Center(child: Text('还没有阅读历史')),
                      SizedBox(height: shellBarClearance(context)),
                    ],
                  ),
                )
              : RefreshIndicator(
                  onRefresh: onRefresh,
                  child: ListView(
                    padding: EdgeInsets.fromLTRB(
                      padding,
                      0,
                      padding,
                      16 + shellBarClearance(context),
                    ),
                    children: [
                      for (final entry in groups.entries) ...[
                        Padding(
                          padding: const EdgeInsets.fromLTRB(0, 16, 0, 6),
                          child: Text(
                            entry.key,
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              color: colors.onSurfaceVariant,
                            ),
                          ),
                        ),
                        if (!twoColumns)
                          for (final item in entry.value)
                            _HistoryRow(
                              item: item,
                              serverUrl: serverUrl,
                              editing: editing,
                              onDelete: () => onDelete(item),
                              onResume: () => onResume(item),
                            )
                        else
                          for (var i = 0; i < entry.value.length; i += 2)
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(
                                  child: _HistoryRow(
                                    item: entry.value[i],
                                    serverUrl: serverUrl,
                                    editing: editing,
                                    onDelete: () => onDelete(entry.value[i]),
                                    onResume: () => onResume(entry.value[i]),
                                  ),
                                ),
                                const SizedBox(width: 16),
                                Expanded(
                                  child: i + 1 < entry.value.length
                                      ? _HistoryRow(
                                          item: entry.value[i + 1],
                                          serverUrl: serverUrl,
                                          editing: editing,
                                          onDelete: () =>
                                              onDelete(entry.value[i + 1]),
                                          onResume: () =>
                                              onResume(entry.value[i + 1]),
                                        )
                                      : const SizedBox.shrink(),
                                ),
                              ],
                            ),
                      ],
                    ],
                  ),
                ),
        ),
      ],
    );
  }
}

class _KindTab extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _KindTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Center(
          child: Text(
            label,
            style: TextStyle(
              fontSize: 14,
              fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
              color: selected ? colors.onSurface : colors.onSurfaceVariant,
            ),
          ),
        ),
      ),
    );
  }
}

class _HistoryRow extends StatelessWidget {
  final ReadingHistoryItem item;
  final String serverUrl;
  final bool editing;
  final VoidCallback onDelete;
  final VoidCallback onResume;

  const _HistoryRow({
    required this.item,
    required this.serverUrl,
    required this.editing,
    required this.onDelete,
    required this.onResume,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final progress = item.endPage > 0
        ? '看到第 ${item.endPage} 页 · ${item.sessionCount} 场'
        : '${item.sessionCount} 场 · ${_formatDuration(item.totalReadTime)}';
    return InkWell(
      onTap: editing ? null : () => context.push(item.route),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 11),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: SizedBox(
                width: 78,
                height: 104,
                child: AuthenticatedImage(
                  imageUrl: item.resolvedCoverUrl(serverUrl),
                  fit: BoxFit.cover,
                  placeholder:
                      ColoredBox(color: colors.surfaceContainerHighest),
                  errorWidget:
                      ColoredBox(color: colors.surfaceContainerHighest),
                ),
              ),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    progress,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12.5,
                      fontWeight: FontWeight.w500,
                      color: colors.primary,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    _formatClock(item.lastReadAt),
                    style: TextStyle(fontSize: 11.5, color: colors.tertiary),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 10),
            if (editing)
              IconButton(
                tooltip: '删除这条历史',
                onPressed: onDelete,
                icon: Icon(
                  Icons.delete_outline_rounded,
                  size: 19,
                  color: colors.onSurfaceVariant,
                ),
              )
            else
              SizedBox(
                width: 68,
                height: 34,
                child: FilledButton(
                  onPressed: onResume,
                  style: FilledButton.styleFrom(
                    minimumSize: Size.zero,
                    padding: EdgeInsets.zero,
                    backgroundColor: colors.primary,
                    foregroundColor: colors.onPrimary,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(17),
                    ),
                    textStyle: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  child: const Text('续看'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _CachePane extends ConsumerWidget {
  const _CachePane();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).colorScheme;
    final entries = ref.watch(cacheEntriesProvider);
    final size = ref.watch(totalCacheSizeProvider);
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.offline_pin_outlined, size: 38, color: colors.tertiary),
          const SizedBox(height: 10),
          Text(
            '离线缓存',
            style: TextStyle(
                fontSize: 17,
                fontWeight: FontWeight.w600,
                color: colors.onSurface),
          ),
          const SizedBox(height: 6),
          Text(
            '${entries.length} 部 · $size',
            style: TextStyle(fontSize: 12, color: colors.onSurfaceVariant),
          ),
          const SizedBox(height: 18),
          SizedBox(
            width: 180,
            child: OutlinedButton(
              onPressed: () => context.push('/cache'),
              child: const Text('管理离线缓存'),
            ),
          ),
        ],
      ),
    );
  }
}

String _formatDuration(int seconds) {
  if (seconds <= 0) return '未记录时长';
  final minutes = (seconds / 60).round();
  if (minutes < 60) return '$minutes 分钟';
  final hours = minutes ~/ 60;
  final rest = minutes % 60;
  return rest == 0 ? '$hours 小时' : '$hours 小时 $rest 分';
}

String _dateLabel(String raw) {
  final date = DateTime.tryParse(raw)?.toLocal();
  if (date == null) return '更早';
  final now = DateTime.now();
  final today = DateTime(now.year, now.month, now.day);
  final day = DateTime(date.year, date.month, date.day);
  final diff = today.difference(day).inDays;
  if (diff == 0) return '今天';
  if (diff == 1) return '昨天';
  return '${date.month}月${date.day}日';
}

String _formatClock(String raw) {
  final date = DateTime.tryParse(raw)?.toLocal();
  if (date == null) return '';
  final two = (int value) => value.toString().padLeft(2, '0');
  return '${date.year}-${two(date.month)}-${two(date.day)} '
      '${two(date.hour)}:${two(date.minute)}';
}
