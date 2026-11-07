import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../data/models/work.dart';
import 'animations.dart';
import 'authenticated_image.dart';

class WorkCard extends StatelessWidget {
  final Work work;
  final String serverUrl;
  final bool isGrid;
  final VoidCallback? onFavorite;
  final bool showMeta;

  const WorkCard({
    super.key,
    required this.work,
    required this.serverUrl,
    this.isGrid = true,
    this.onFavorite,
    this.showMeta = false,
  });

  @override
  Widget build(BuildContext context) {
    return isGrid ? _buildGrid(context) : _buildList(context);
  }

  Widget _buildGrid(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return PressableScale(
      onTap: () {
        HapticFeedback.lightImpact();
        context.push(work.detailRoute());
      },
      scaleDown: 0.97,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  _cover(colors),
                  if (work.hasReadingProgress)
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: LinearProgressIndicator(
                        value: work.progress / 100,
                        minHeight: 2,
                        backgroundColor: Colors.black26,
                        color: colors.primary,
                      ),
                    ),
                  if (onFavorite != null)
                    Positioned(
                      top: 6,
                      right: 6,
                      child: GestureDetector(
                        onTap: onFavorite,
                        child: Container(
                          width: 28,
                          height: 28,
                          decoration: BoxDecoration(
                            color: Colors.black.withValues(alpha: 0.38),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Icon(
                            work.isFavorite
                                ? Icons.favorite_rounded
                                : Icons.favorite_border_rounded,
                            size: 16,
                            color: Colors.white,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            work.title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              fontSize: 15,
              height: 1.2,
              fontWeight: FontWeight.w600,
            ),
          ),
          if (showMeta) ...[
            const SizedBox(height: 3),
            Text(
              work.itemCount > 0 && work.completedItemCount >= work.itemCount
                  ? '已读至最新话'
                  : work.completedItemCount > 0
                      ? '已读 ${work.completedItemCount}/${work.itemCount}话'
                      : work.progress > 0
                          ? '已读 ${work.progress}%'
                          : '${work.itemCount}话',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: work.hasReadingProgress
                    ? colors.primary
                    : colors.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildList(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return ListTile(
      onTap: () => context.push(work.detailRoute()),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 5),
      leading: ClipRRect(
        borderRadius: BorderRadius.circular(4),
        child: SizedBox(width: 40, height: 53, child: _cover(colors)),
      ),
      title: Text(
        work.title,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
      ),
      subtitle: Text(
        [
          if (work.author.isNotEmpty) work.author,
          '${work.itemCount}话/卷',
          '${work.pageCount}页',
        ].join(' · '),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(fontSize: 11, color: colors.onSurfaceVariant),
      ),
      trailing: onFavorite == null
          ? Icon(Icons.chevron_right_rounded,
              size: 18, color: colors.onSurfaceVariant)
          : IconButton(
              onPressed: onFavorite,
              icon: Icon(
                work.isFavorite
                    ? Icons.favorite_rounded
                    : Icons.favorite_border_rounded,
                color: work.isFavorite ? colors.primary : null,
              ),
            ),
    );
  }

  Widget _cover(ColorScheme colors) {
    final url = work.resolvedCoverUrl(serverUrl);
    if (url.isEmpty) {
      return ColoredBox(
        color: colors.surfaceContainerHighest,
        child: Icon(Icons.menu_book_rounded, color: colors.onSurfaceVariant),
      );
    }
    return AuthenticatedImage(
      imageUrl: url,
      comicId: work.coverComicId.isEmpty ? null : work.coverComicId,
      isThumbnail: true,
      fit: BoxFit.cover,
      placeholder: ColoredBox(color: colors.surfaceContainerHighest),
      errorWidget: ColoredBox(
        color: colors.surfaceContainerHighest,
        child:
            Icon(Icons.broken_image_outlined, color: colors.onSurfaceVariant),
      ),
    );
  }
}
