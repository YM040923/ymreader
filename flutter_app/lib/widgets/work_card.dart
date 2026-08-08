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

  const WorkCard({
    super.key,
    required this.work,
    required this.serverUrl,
    this.isGrid = true,
    this.onFavorite,
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
              borderRadius: BorderRadius.circular(12),
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
                        minHeight: 3,
                        backgroundColor: Colors.black26,
                        color: colors.primary,
                      ),
                    ),
                  if (onFavorite != null)
                    Positioned(
                      top: 6,
                      right: 6,
                      child: IconButton.filledTonal(
                        visualDensity: VisualDensity.compact,
                        iconSize: 17,
                        onPressed: onFavorite,
                        icon: Icon(
                          work.isFavorite
                              ? Icons.favorite_rounded
                              : Icons.favorite_border_rounded,
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
            style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 2),
          Text(
            '${work.itemCount}话/卷 · ${work.pageCount}页',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 10, color: colors.onSurfaceVariant),
          ),
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
        borderRadius: BorderRadius.circular(8),
        child: SizedBox(width: 52, height: 72, child: _cover(colors)),
      ),
      title: Text(
        work.title,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(fontWeight: FontWeight.w600),
      ),
      subtitle: Text(
        [
          if (work.author.isNotEmpty) work.author,
          '${work.itemCount}话/卷',
          '${work.pageCount}页',
        ].join(' · '),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: onFavorite == null
          ? const Icon(Icons.chevron_right_rounded)
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
