import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/api_client.dart';
import '../../data/api/library_api.dart';
import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/responsive.dart';
import 'work_manage_dialogs.dart';

class WorkDetailScreen extends ConsumerWidget {
  final String workId;

  const WorkDetailScreen({super.key, required this.workId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).colorScheme;
    final asyncWork = ref.watch(workDetailProvider(workId));
    return asyncWork.when(
      loading: () => const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (error, _) {
        debugPrint('[work-detail] $workId load failed: $error');
        return Scaffold(
          appBar: AppBar(),
          body: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.cloud_off_rounded,
                  size: 40,
                  color: colors.onSurfaceVariant,
                ),
                const SizedBox(height: 12),
                Text(
                  apiErrorLabel(error),
                  textAlign: TextAlign.center,
                  style: TextStyle(color: colors.onSurfaceVariant),
                ),
                const SizedBox(height: 18),
                ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 160),
                  child: FilledButton.icon(
                    onPressed: () => ref.invalidate(workDetailProvider(workId)),
                    icon: const Icon(Icons.refresh_rounded, size: 18),
                    label: const Text('重试'),
                  ),
                ),
              ],
            ),
          ),
        );
      },
      data: (work) => _WorkDetailBody(work: work),
    );
  }
}

enum _DirectoryViewMode { list, covers }

class _WorkDetailBody extends ConsumerStatefulWidget {
  final Work work;

  const _WorkDetailBody({required this.work});

  @override
  ConsumerState<_WorkDetailBody> createState() => _WorkDetailBodyState();
}

class _WorkDetailBodyState extends ConsumerState<_WorkDetailBody> {
  _DirectoryViewMode _directoryMode = _DirectoryViewMode.list;

  Work get work => widget.work;

  Future<void> _openReader(String route) async {
    await context.push(route);
    if (!mounted) return;
    ref.invalidate(workDetailProvider(work.id));
    ref.read(workListProvider.notifier).load();
  }

  Future<void> _mergeWork() async {
    final targetId = await showDialog<String>(
      context: context,
      builder: (_) => MergeWorkDialog(work: work),
    );
    if (!mounted || targetId == null || targetId.isEmpty) return;
    // Refresh the list first and only then drop the cached Work details: this
    // screen still shows the Work that the merge absorbed, and invalidating it
    // before the list round trip surfaces a raw 404 for a Work that no longer
    // exists. The family-wide invalidate also clears the cached failure of the
    // absorbed Work so it reloads once the server has it again.
    await ref.read(workListProvider.notifier).load();
    if (!mounted) return;
    ref.invalidate(workDetailProvider);
    context.go('/work/${Uri.encodeComponent(targetId)}');
  }

  Future<void> _splitWork() async {
    final newWorkId = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (_) => SplitWorkSheet(work: work),
    );
    if (!mounted || newWorkId == null || newWorkId.isEmpty) return;
    await ref.read(workListProvider.notifier).load();
    if (!mounted) return;
    ref.invalidate(workDetailProvider);
    context.go('/work/${Uri.encodeComponent(newWorkId)}');
  }

  Future<void> _restoreAutoGrouping() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('恢复自动分组'),
        content: const Text(
          '清除本作品所在目录组的手动合并/拆分关系，按目录重新分组。文件、阅读进度和已保存的元数据不会被删除。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          // The app-wide button theme asks for an infinite minimum width,
          // which would drop this action onto its own full-width row.
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 160),
            child: FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('确认恢复'),
            ),
          ),
        ],
      ),
    );
    if (!mounted || confirmed != true) return;
    try {
      await ref.read(workApiProvider).restoreWorkAutoGrouping(work.id);
      ref.invalidate(workDetailProvider);
      await ref.read(workListProvider.notifier).load();
      if (mounted) context.go('/');
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('恢复失败：${apiErrorLabel(error)}')),
        );
      }
    }
  }

  Future<void> _undoLastOperation() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('撤销上次整理'),
        content: const Text(
          '只恢复最近一次合并或拆分前的作品归属。文件、阅读进度和已保存的元数据不会被删除。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          // The app-wide button theme asks for an infinite minimum width,
          // which would drop this action onto its own full-width row.
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 160),
            child: FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('确认撤销'),
            ),
          ),
        ],
      ),
    );
    if (!mounted || confirmed != true) return;
    try {
      await ref.read(workApiProvider).undoLastWorkOperation(work.id);
      ref.invalidate(workDetailProvider);
      await ref.read(workListProvider.notifier).load();
      if (mounted) context.go('/');
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('撤销失败：${apiErrorLabel(error)}')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final width = MediaQuery.sizeOf(context).width;
    final horizontal = pagePadding(width);
    final serverUrl = ref.watch(authProvider).serverUrl;
    final libraries = ref.watch(accessibleLibrariesProvider).valueOrNull ??
        const <LibrarySummary>[];
    final canManage = ref.watch(authProvider).user?.isAdmin == true ||
        libraries.any(
            (library) => library.id == work.libraryId && library.canManage);
    final target = work.readingTarget;
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            pinned: true,
            title: const Text('作品详情'),
            actions: [
              if (canManage)
                PopupMenuButton<String>(
                  tooltip: '整理作品',
                  onSelected: (value) {
                    switch (value) {
                      case 'merge':
                        _mergeWork();
                      case 'split':
                        _splitWork();
                      case 'restore':
                        _restoreAutoGrouping();
                      case 'undo':
                        _undoLastOperation();
                    }
                  },
                  itemBuilder: (context) => [
                    const PopupMenuItem(
                      value: 'merge',
                      child: ListTile(
                        leading: Icon(Icons.merge_type_rounded),
                        title: Text('合并作品'),
                        contentPadding: EdgeInsets.zero,
                      ),
                    ),
                    if (work.units.length > 1)
                      const PopupMenuItem(
                        value: 'split',
                        child: ListTile(
                          leading: Icon(Icons.call_split_rounded),
                          title: Text('拆分单元'),
                          contentPadding: EdgeInsets.zero,
                        ),
                      ),
                    if (work.manualMembership)
                      const PopupMenuItem(
                        value: 'undo',
                        child: ListTile(
                          leading: Icon(Icons.undo_rounded),
                          title: Text('撤销上次整理'),
                          contentPadding: EdgeInsets.zero,
                        ),
                      ),
                    if (work.manualMembership)
                      const PopupMenuItem(
                        value: 'restore',
                        child: ListTile(
                          leading: Icon(Icons.settings_backup_restore_rounded),
                          title: Text('恢复自动分组'),
                          contentPadding: EdgeInsets.zero,
                        ),
                      ),
                  ],
                ),
              IconButton(
                tooltip: work.isFavorite ? '取消收藏' : '收藏',
                onPressed: () async {
                  await ref
                      .read(workApiProvider)
                      .setFavorite(work.id, !work.isFavorite);
                  ref.invalidate(workDetailProvider(work.id));
                },
                icon: Icon(
                  work.isFavorite
                      ? Icons.favorite_rounded
                      : Icons.favorite_border_rounded,
                ),
              ),
            ],
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.fromLTRB(horizontal, 8, horizontal, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(4),
                        child: SizedBox(
                          width: 112,
                          height: 149,
                          child: AuthenticatedImage(
                            imageUrl: work.resolvedCoverUrl(serverUrl),
                            fit: BoxFit.cover,
                            placeholder: ColoredBox(
                              color: colors.surfaceContainerHighest,
                            ),
                            errorWidget: ColoredBox(
                              color: colors.surfaceContainerHighest,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: SizedBox(
                          height: 149,
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                work.title,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontSize: 22,
                                  fontWeight: FontWeight.w700,
                                  height: 1.25,
                                ),
                              ),
                              if (work.author.isNotEmpty) ...[
                                const SizedBox(height: 6),
                                Text(
                                  work.author,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 12,
                                    color: colors.onSurfaceVariant,
                                  ),
                                ),
                              ],
                              const SizedBox(height: 4),
                              Text(
                                '${work.itemCount}话 · ${work.pageCount}页',
                                style: TextStyle(
                                  fontSize: 11,
                                  color: colors.tertiary,
                                ),
                              ),
                              if (target != null) ...[
                                const SizedBox(height: 12),
                                Text(
                                  '上次读到 ${target.unit.displayLabel} · 第${target.relativePage + 1}页',
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 11,
                                    color: colors.onSurfaceVariant,
                                  ),
                                ),
                                const SizedBox(height: 8),
                                ClipRRect(
                                  borderRadius: BorderRadius.circular(1),
                                  child: LinearProgressIndicator(
                                    value: work.progress / 100,
                                    minHeight: 2,
                                    color: colors.primary,
                                    backgroundColor: colors.outlineVariant,
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  SizedBox(
                    width: width >= 860 ? 320 : double.infinity,
                    child: FilledButton(
                      onPressed: target == null
                          ? null
                          : () => _openReader(work.readerRoute()),
                      child: Text(
                        target == null
                            ? '开始阅读'
                            : '继续阅读 · ${target.unit.displayLabel}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ),
                  if (work.description.isNotEmpty) ...[
                    const SizedBox(height: 20),
                    ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 760),
                      child: Text(
                        work.description,
                        style: TextStyle(
                          fontSize: 14,
                          height: 1.55,
                          color: colors.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ],
                  const SizedBox(height: 24),
                  Row(
                    children: [
                      const Text(
                        '目录',
                        style: TextStyle(
                            fontSize: 17, fontWeight: FontWeight.w700),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        '${work.units.length} 话',
                        style: TextStyle(
                          fontSize: 11,
                          color: colors.tertiary,
                        ),
                      ),
                      const Spacer(),
                      _DirectoryTab(
                        label: '封面',
                        selected: _directoryMode == _DirectoryViewMode.covers,
                        onTap: () => setState(
                            () => _directoryMode = _DirectoryViewMode.covers),
                      ),
                      const SizedBox(width: 16),
                      _DirectoryTab(
                        label: '列表',
                        selected: _directoryMode == _DirectoryViewMode.list,
                        onTap: () => setState(
                            () => _directoryMode = _DirectoryViewMode.list),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                ],
              ),
            ),
          ),
          if (work.units.isEmpty)
            const SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.all(30),
                child: Center(child: Text('暂无可阅读目录')),
              ),
            )
          else if (_directoryMode == _DirectoryViewMode.list)
            SliverPadding(
              padding: EdgeInsets.symmetric(horizontal: horizontal),
              sliver: SliverList.builder(
                itemCount: work.units.length,
                itemBuilder: (_, index) {
                  final unit = work.units[index];
                  final isCurrent = unit.id == work.continueUnitId;
                  final label = unit.displayLabel.isEmpty
                      ? unit.title
                      : unit.displayLabel;
                  return InkWell(
                    borderRadius: BorderRadius.circular(8),
                    onTap: () => _openReader(
                      work.readerRoute(
                        target: WorkReadingTarget(
                          unit: unit,
                          absolutePage: unit.startPage,
                          isContinue: false,
                        ),
                      ),
                    ),
                    child: SizedBox(
                      height: 56,
                      child: Row(
                        children: [
                          SizedBox(
                            width: 14,
                            child: isCurrent
                                ? Center(
                                    child: Container(
                                      width: 6,
                                      height: 6,
                                      decoration: BoxDecoration(
                                        color: colors.primary,
                                        shape: BoxShape.circle,
                                      ),
                                    ),
                                  )
                                : null,
                          ),
                          Expanded(
                            child: Text(
                              label,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 14,
                                fontWeight: isCurrent
                                    ? FontWeight.w600
                                    : FontWeight.w500,
                              ),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Text(
                            '${unit.pageCount} 页',
                            style: TextStyle(
                              fontSize: 11,
                              color: colors.tertiary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
          if (work.units.isNotEmpty &&
              _directoryMode == _DirectoryViewMode.covers)
            SliverPadding(
              padding: EdgeInsets.fromLTRB(horizontal, 4, horizontal, 16),
              sliver: SliverGrid.builder(
                gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: width >= 860
                      ? coverColumns(width)
                      : coverColumns(width).clamp(3, 5),
                  childAspectRatio: 0.60,
                  crossAxisSpacing: width >= 860 ? 16 : 8,
                  mainAxisSpacing: width >= 860 ? 24 : 16,
                ),
                itemCount: work.units.length,
                itemBuilder: (_, index) {
                  final unit = work.units[index];
                  final isCurrent = unit.id == work.continueUnitId;
                  return InkWell(
                    borderRadius: BorderRadius.circular(4),
                    onTap: () => _openReader(
                      work.readerRoute(
                        target: WorkReadingTarget(
                          unit: unit,
                          absolutePage: unit.startPage,
                          isContinue: false,
                        ),
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(
                          child: Stack(
                            fit: StackFit.expand,
                            children: [
                              ClipRRect(
                                borderRadius: BorderRadius.circular(4),
                                child: AuthenticatedImage(
                                  imageUrl: unit.resolvedCoverUrl(serverUrl),
                                  comicId: unit.comicId,
                                  pageIndex: unit.resolvedCoverPage,
                                  fit: BoxFit.cover,
                                  placeholder: ColoredBox(
                                    color: colors.surfaceContainerHighest,
                                  ),
                                  errorWidget: AuthenticatedImage(
                                    imageUrl: work.resolvedCoverUrl(serverUrl),
                                    comicId: work.coverComicId,
                                    isThumbnail: true,
                                    fit: BoxFit.cover,
                                  ),
                                ),
                              ),
                              if (isCurrent)
                                Positioned(
                                  top: 8,
                                  right: 8,
                                  child: Container(
                                    width: 10,
                                    height: 10,
                                    decoration: BoxDecoration(
                                      color: colors.primary,
                                      shape: BoxShape.circle,
                                      border: Border.all(
                                        color: colors.surface,
                                        width: 2,
                                      ),
                                    ),
                                  ),
                                ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          unit.displayLabel.isEmpty
                              ? unit.title
                              : unit.displayLabel,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        Text(
                          '${unit.pageCount} 页',
                          style: TextStyle(
                            fontSize: 11,
                            color: colors.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
          const SliverToBoxAdapter(child: SizedBox(height: 32)),
        ],
      ),
    );
  }
}

class _DirectoryTab extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _DirectoryTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Text(
        label,
        style: TextStyle(
          fontSize: 12,
          fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
          color: selected ? colors.onSurface : colors.tertiary,
        ),
      ),
    );
  }
}
