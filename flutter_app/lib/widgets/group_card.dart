import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../data/models/comic.dart';
import '../data/api/api_client.dart';

/// 合集卡片组件 — 紧凑 mini 风格
class GroupCard extends ConsumerWidget {
  final ComicGroup group;
  final bool isGrid;

  const GroupCard({
    super.key,
    required this.group,
    this.isGrid = true,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cs = Theme.of(context).colorScheme;
    final serverUrl = ref.watch(serverUrlProvider);

    if (isGrid) {
      return _buildGridCard(context, cs, serverUrl);
    }
    return _buildListCard(context, cs, serverUrl);
  }

  /// 紧凑网格卡片 — 1:1 宽高比，更小的字体和间距
  Widget _buildGridCard(BuildContext context, ColorScheme cs, String serverUrl) {
    return GestureDetector(
      onTap: () => context.push('/group/${group.id}'),
      child: Container(
        decoration: BoxDecoration(
          color: cs.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(10),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 封面区域 — 1:1 宽高比
            AspectRatio(
              aspectRatio: 1,
              child: _buildCover(cs, serverUrl),
            ),
            // 信息区域 — 紧凑
            Padding(
              padding: const EdgeInsets.fromLTRB(6, 4, 6, 6),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    group.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w500,
                      color: cs.onSurface,
                    ),
                  ),
                  Text(
                    '${group.comicCount} 本',
                    style: TextStyle(
                      fontSize: 10,
                      color: cs.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// 列表卡片 — 紧凑行
  Widget _buildListCard(BuildContext context, ColorScheme cs, String serverUrl) {
    return GestureDetector(
      onTap: () => context.push('/group/${group.id}'),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: cs.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          children: [
            // 封面 — 更小
            ClipRRect(
              borderRadius: BorderRadius.circular(6),
              child: SizedBox(
                width: 44,
                height: 44,
                child: _buildCover(cs, serverUrl),
              ),
            ),
            const SizedBox(width: 10),
            // 信息
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    group.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                      color: cs.onSurface,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${group.comicCount} 本${group.author.isNotEmpty ? " · ${group.author}" : ""}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 11,
                      color: cs.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            // 箭头
            Icon(
              Icons.chevron_right_rounded,
              size: 18,
              color: cs.onSurfaceVariant.withOpacity(0.5),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCover(ColorScheme cs, String serverUrl) {
    if (group.coverUrl.isNotEmpty) {
      return Image.network(
        '$serverUrl${group.coverUrl}',
        fit: BoxFit.cover,
        errorBuilder: (_, __, ___) => _buildPlaceholder(cs),
      );
    }
    return _buildPlaceholder(cs);
  }

  Widget _buildPlaceholder(ColorScheme cs) {
    return Container(
      color: cs.primaryContainer.withOpacity(0.3),
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.collections_bookmark_outlined,
              size: 24,
              color: cs.primary.withOpacity(0.5),
            ),
            const SizedBox(height: 2),
            Text(
              '${group.comicCount}',
              style: TextStyle(
                fontSize: 10,
                fontWeight: FontWeight.w600,
                color: cs.primary.withOpacity(0.6),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
