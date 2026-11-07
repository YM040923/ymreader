import 'dart:ui' show ImageFilter;

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
            expandedHeight: width >= 860 ? 380 : 340,
            toolbarHeight: 64,
            backgroundColor: colors.surface,
            surfaceTintColor: Colors.transparent,
            elevation: 0,
            scrolledUnderElevation: 0,
            leadingWidth: 64,
            leading: Padding(
              padding: const EdgeInsets.only(left: 12),
              child: IconButton(
                tooltip: '返回',
                onPressed: () => context.pop(),
                style: IconButton.styleFrom(
                  backgroundColor: colors.surface.withValues(alpha: 0.72),
                  foregroundColor: colors.onSurface,
                ),
                icon: const Icon(Icons.arrow_back_rounded, size: 22),
              ),
            ),
            actions: [
              if (canManage)
                PopupMenuButton<String>(
                  tooltip: '整理作品',
                  icon: Icon(
                    Icons.more_horiz_rounded,
                    color: colors.onSurface,
                  ),
                  style: IconButton.styleFrom(
                    backgroundColor: colors.surface.withValues(alpha: 0.72),
                  ),
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
              const SizedBox(width: 6),
              Padding(
                padding: const EdgeInsets.only(right: 12),
                child: IconButton(
                  tooltip: work.isFavorite ? '取消收藏' : '收藏',
                  onPressed: () async {
                    await ref
                        .read(workApiProvider)
                        .setFavorite(work.id, !work.isFavorite);
                    ref.invalidate(workDetailProvider(work.id));
                  },
                  style: IconButton.styleFrom(
                    backgroundColor: colors.surface.withValues(alpha: 0.72),
                    foregroundColor: work.isFavorite
                        ? colors.error
                        : colors.onSurface,
                  ),
                  icon: Icon(
                    work.isFavorite
                        ? Icons.favorite_rounded
                        : Icons.favorite_border_rounded,
                  ),
                ),
              ),
            ],
            flexibleSpace: FlexibleSpaceBar(
              collapseMode: CollapseMode.parallax,
              background: _DetailHero(
                work: work,
                serverUrl: serverUrl,
                target: target,
                horizontal: horizontal,
                wide: width >= 860,
              ),
            ),
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.fromLTRB(horizontal, 8, horizontal, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const SizedBox(height: 16),
                  SizedBox(
                    width: width >= 860 ? 320 : double.infinity,
                    child: FilledButton(
                      onPressed: target == null
                          ? null
                          : () => _openReader(work.readerRoute()),
                      style: FilledButton.styleFrom(
                        minimumSize: const Size(0, 48),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(24),
                        ),
                      ),
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


class _DetailHero extends StatelessWidget {
  const _DetailHero({
    required this.work,
    required this.serverUrl,
    required this.target,
    required this.horizontal,
    required this.wide,
  });

  final Work work;
  final String serverUrl;
  final WorkReadingTarget? target;
  final double horizontal;
  final bool wide;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final coverWidth = wide ? 140.0 : 112.0;
    final coverHeight = coverWidth * 4 / 3;
    final coverUrl = work.resolvedCoverUrl(serverUrl);
    return Stack(
      fit: StackFit.expand,
      children: [
        ImageFiltered(
          imageFilter: ImageFilter.blur(sigmaX: 28, sigmaY: 28),
          child: Transform.scale(
            scale: 1.18,
            child: AuthenticatedImage(
              imageUrl: coverUrl,
              fit: BoxFit.cover,
              placeholder: ColoredBox(color: colors.surfaceContainerHighest),
              errorWidget: ColoredBox(color: colors.surfaceContainerHighest),
            ),
          ),
        ),
        DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              colors: [
                colors.surface.withValues(alpha: 0.30),
                colors.surface.withValues(alpha: 0.72),
                colors.surface,
              ],
              stops: const [0, 0.62, 1],
            ),
          ),
        ),
        Positioned(
          left: horizontal,
          right: horizontal,
          bottom: 24,
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              SizedBox(
                width: coverWidth,
                height: coverHeight,
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(16),
                  child: AuthenticatedImage(
                    imageUrl: coverUrl,
                    fit: BoxFit.cover,
                    placeholder:
                        ColoredBox(color: colors.surfaceContainerHighest),
                    errorWidget:
                        ColoredBox(color: colors.surfaceContainerHighest),
                  ),
                ),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      work.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: text.headlineSmall?.copyWith(
                        fontSize: wide ? 30 : 26,
                        fontWeight: FontWeight.w700,
                        height: 1.15,
                      ),
                    ),
                    if (work.author.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(
                        work.author,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: text.bodySmall?.copyWith(
                          color: colors.onSurfaceVariant,
                        ),
                      ),
                    ],
                    const SizedBox(height: 10),
                    Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: [
                        _MetaChip('${work.itemCount} 话'),
                        _MetaChip('${work.pageCount} 页'),
                        if (work.status.isNotEmpty) _MetaChip(work.status),
                      ],
                    ),
                    if (target != null) ...[
                      const SizedBox(height: 12),
                      ClipRRect(
                        borderRadius: BorderRadius.circular(2),
                        child: LinearProgressIndicator(
                          value: work.progress / 100,
                          minHeight: 4,
                          color: colors.primary,
                          backgroundColor:
                              colors.onSurface.withValues(alpha: 0.12),
                        ),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        '上次读到 ${target!.unit.displayLabel} · 第${target!.relativePage + 1}页',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: text.labelSmall?.copyWith(
                          color: colors.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _MetaChip extends StatelessWidget {
  const _MetaChip(this.label);

  final String label;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: colors.surfaceContainerHighest.withValues(alpha: 0.72),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: colors.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
      ),
    );
  }
}
