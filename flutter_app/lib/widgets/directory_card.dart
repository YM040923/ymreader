import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../data/api/work_api.dart';
import 'authenticated_image.dart';

class DirectoryCard extends StatelessWidget {
  final LibraryDirectoryNode node;
  final String serverUrl;
  final bool isGrid;
  final VoidCallback onTap;

  const DirectoryCard({
    super.key,
    required this.node,
    required this.serverUrl,
    required this.onTap,
    this.isGrid = true,
  });

  String get _coverUrl {
    final raw = node.coverUrl?.trim().isNotEmpty == true
        ? node.coverUrl!.trim()
        : (node.coverComicId?.isNotEmpty == true
            ? '/api/comics/${node.coverComicId}/thumbnail'
            : '');
    if (raw.isEmpty) return '';
    if (raw.startsWith('http')) return raw;
    final base = serverUrl.endsWith('/')
        ? serverUrl.substring(0, serverUrl.length - 1)
        : serverUrl;
    return '$base${raw.startsWith('/') ? '' : '/'}$raw';
  }

  @override
  Widget build(BuildContext context) {
    return isGrid ? _grid(context) : _list(context);
  }

  Widget _grid(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final cover = _coverUrl;
    return GestureDetector(
      onTap: () {
        HapticFeedback.lightImpact();
        onTap();
      },
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (cover.isEmpty)
                    ColoredBox(color: colors.surfaceContainerHighest)
                  else
                    AuthenticatedImage(
                      imageUrl: cover,
                      comicId: node.coverComicId,
                      isThumbnail: true,
                      fit: BoxFit.cover,
                      placeholder:
                          ColoredBox(color: colors.surfaceContainerHighest),
                      errorWidget:
                          ColoredBox(color: colors.surfaceContainerHighest),
                    ),
                  ColoredBox(color: Colors.black.withValues(alpha: 0.28)),
                  Center(
                    child: Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: Colors.black.withValues(alpha: 0.35),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: const Icon(Icons.folder_rounded,
                          size: 30, color: Colors.white),
                    ),
                  ),
                  Positioned(
                    right: 6,
                    bottom: 6,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 7, vertical: 3),
                      decoration: BoxDecoration(
                        color: Colors.black.withValues(alpha: 0.6),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        '${node.workCount} 部',
                        style: const TextStyle(
                            fontSize: 10, color: Colors.white),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            node.name,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 2),
          Text(
            node.hasChildren ? '文件夹' : '作品目录',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 10, color: colors.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  Widget _list(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final cover = _coverUrl;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: SizedBox(
                width: 46,
                height: 60,
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    if (cover.isEmpty)
                      ColoredBox(color: colors.surfaceContainerHighest)
                    else
                      AuthenticatedImage(
                        imageUrl: cover,
                        comicId: node.coverComicId,
                        isThumbnail: true,
                        fit: BoxFit.cover,
                        placeholder:
                            ColoredBox(color: colors.surfaceContainerHighest),
                        errorWidget:
                            ColoredBox(color: colors.surfaceContainerHighest),
                      ),
                    ColoredBox(color: Colors.black.withValues(alpha: 0.28)),
                    const Center(
                      child: Icon(Icons.folder_rounded,
                          size: 22, color: Colors.white),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    node.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 13, fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    '${node.workCount} 部作品 · ${node.hasChildren ? "文件夹" : "作品目录"}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                        fontSize: 11, color: colors.onSurfaceVariant),
                  ),
                ],
              ),
            ),
            Icon(Icons.chevron_right_rounded,
                color: colors.onSurfaceVariant),
          ],
        ),
      ),
    );
  }
}
