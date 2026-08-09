import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../widgets/authenticated_image.dart';

class WorkDetailScreen extends ConsumerWidget {
  final String workId;

  const WorkDetailScreen({super.key, required this.workId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncWork = ref.watch(workDetailProvider(workId));
    return asyncWork.when(
      loading: () => const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (error, _) => Scaffold(
        appBar: AppBar(),
        body: Center(
          child: FilledButton.icon(
            onPressed: () => ref.invalidate(workDetailProvider(workId)),
            icon: const Icon(Icons.refresh_rounded),
            label: Text('加载失败，点击重试\n$error'),
          ),
        ),
      ),
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
  _DirectoryViewMode _directoryMode = _DirectoryViewMode.covers;

  Work get work => widget.work;

  Future<void> _openReader(String route) async {
    await context.push(route);
    if (!mounted) return;
    ref.invalidate(workDetailProvider(work.id));
    ref.read(workListProvider.notifier).load();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final serverUrl = ref.watch(authProvider).serverUrl;
    final target = work.readingTarget;
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            pinned: true,
            expandedHeight: 330,
            actions: [
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
            flexibleSpace: FlexibleSpaceBar(
              background: Stack(
                fit: StackFit.expand,
                children: [
                  AuthenticatedImage(
                    imageUrl: work.resolvedCoverUrl(serverUrl),
                    fit: BoxFit.cover,
                    placeholder:
                        ColoredBox(color: colors.surfaceContainerHighest),
                    errorWidget:
                        ColoredBox(color: colors.surfaceContainerHighest),
                  ),
                  const DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [
                          Colors.transparent,
                          Colors.black87,
                        ],
                      ),
                    ),
                  ),
                  Positioned(
                    left: 20,
                    right: 20,
                    bottom: 24,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          work.title,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 25,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                        if (work.author.isNotEmpty) ...[
                          const SizedBox(height: 5),
                          Text(
                            work.author,
                            style: const TextStyle(color: Colors.white70),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(18, 18, 18, 10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton.icon(
                      onPressed: target == null
                          ? null
                          : () => _openReader(work.readerRoute()),
                      icon: Icon(
                        work.hasReadingProgress
                            ? Icons.play_arrow_rounded
                            : Icons.auto_stories_rounded,
                      ),
                      label: Text(work.hasReadingProgress && target != null
                          ? '继续阅读 · ${target.unit.displayLabel} · 第${target.relativePage + 1}页'
                          : '立即阅读'),
                    ),
                  ),
                  const SizedBox(height: 14),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      Chip(label: Text('${work.itemCount}话/卷')),
                      Chip(label: Text('${work.pageCount}页')),
                      for (final category in work.categories)
                        Chip(label: Text(category.name)),
                    ],
                  ),
                  if (work.description.isNotEmpty) ...[
                    const SizedBox(height: 14),
                    Text(
                      work.description,
                      style: TextStyle(
                        height: 1.55,
                        color: colors.onSurfaceVariant,
                      ),
                    ),
                  ],
                  const SizedBox(height: 20),
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          '目录',
                          style:
                              Theme.of(context).textTheme.titleLarge?.copyWith(
                                    fontWeight: FontWeight.w700,
                                  ),
                        ),
                      ),
                      SegmentedButton<_DirectoryViewMode>(
                        segments: const [
                          ButtonSegment(
                            value: _DirectoryViewMode.covers,
                            icon: Icon(Icons.grid_view_rounded),
                            tooltip: '封面目录',
                          ),
                          ButtonSegment(
                            value: _DirectoryViewMode.list,
                            icon: Icon(Icons.view_list_rounded),
                            tooltip: '文字目录',
                          ),
                        ],
                        selected: {_directoryMode},
                        showSelectedIcon: false,
                        onSelectionChanged: (value) =>
                            setState(() => _directoryMode = value.first),
                      ),
                    ],
                  ),
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
            SliverList.separated(
              itemCount: work.units.length,
              separatorBuilder: (_, __) =>
                  const Divider(height: 1, indent: 18, endIndent: 18),
              itemBuilder: (_, index) {
                final unit = work.units[index];
                final isCurrent = unit.id == work.continueUnitId;
                return ListTile(
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 18, vertical: 3),
                  leading: CircleAvatar(
                    child: Text('${index + 1}'),
                  ),
                  title: Text(
                    unit.displayLabel.isEmpty ? unit.title : unit.displayLabel,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  subtitle: Text('${unit.pageCount}页'),
                  trailing: isCurrent
                      ? Icon(Icons.bookmark_rounded, color: colors.primary)
                      : const Icon(Icons.chevron_right_rounded),
                  onTap: () => _openReader(
                    work.readerRoute(
                      target: WorkReadingTarget(
                        unit: unit,
                        absolutePage: unit.startPage,
                        isContinue: false,
                      ),
                    ),
                  ),
                );
              },
            ),
          if (work.units.isNotEmpty &&
              _directoryMode == _DirectoryViewMode.covers)
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(18, 4, 18, 16),
              sliver: SliverGrid.builder(
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 3,
                  childAspectRatio: 0.62,
                  crossAxisSpacing: 10,
                  mainAxisSpacing: 14,
                ),
                itemCount: work.units.length,
                itemBuilder: (_, index) {
                  final unit = work.units[index];
                  final isCurrent = unit.id == work.continueUnitId;
                  return InkWell(
                    borderRadius: BorderRadius.circular(10),
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
                                borderRadius: BorderRadius.circular(10),
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
                                  top: 6,
                                  right: 6,
                                  child: DecoratedBox(
                                    decoration: BoxDecoration(
                                      color: colors.primary,
                                      shape: BoxShape.circle,
                                    ),
                                    child: const Padding(
                                      padding: EdgeInsets.all(5),
                                      child: Icon(
                                        Icons.bookmark_rounded,
                                        size: 15,
                                        color: Colors.white,
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
                          '${unit.pageCount}页',
                          style: TextStyle(
                            fontSize: 10,
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
