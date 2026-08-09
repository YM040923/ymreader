import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../data/api/work_api.dart';
import '../data/models/work.dart';
import '../data/providers/auth_provider.dart';
import 'authenticated_image.dart';

class ContinueReading extends ConsumerStatefulWidget {
  const ContinueReading({super.key});

  @override
  ConsumerState<ContinueReading> createState() => _ContinueReadingState();
}

class _ContinueReadingState extends ConsumerState<ContinueReading> {
  List<Work> _works = const [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final response = await ref.read(workApiProvider).listWorks(
            const WorkQuery(
              pageSize: 12,
              sortBy: 'lastReadAt',
              sortOrder: 'desc',
            ),
          );
      if (!mounted) return;
      setState(() {
        _works = response.works
            .where((work) => work.hasReadingProgress)
            .take(8)
            .toList();
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading || _works.isEmpty) return const SizedBox.shrink();
    final colors = Theme.of(context).colorScheme;
    final serverUrl = ref.watch(authProvider).serverUrl;
    return Padding(
      padding: const EdgeInsets.only(top: 10, bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Text(
              '继续阅读',
              style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
            ),
          ),
          SizedBox(
            height: 166,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              itemCount: _works.length,
              separatorBuilder: (_, __) => const SizedBox(width: 12),
              itemBuilder: (_, index) {
                final work = _works[index];
                return GestureDetector(
                  onTap: () async {
                    HapticFeedback.lightImpact();
                    await context.push(work.readerRoute());
                    if (mounted) await _load();
                  },
                  child: SizedBox(
                    width: 102,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(10),
                            child: Stack(
                              fit: StackFit.expand,
                              children: [
                                AuthenticatedImage(
                                  imageUrl: work.resolvedCoverUrl(serverUrl),
                                  fit: BoxFit.cover,
                                  placeholder: ColoredBox(
                                    color: colors.surfaceContainerHighest,
                                  ),
                                  errorWidget: ColoredBox(
                                    color: colors.surfaceContainerHighest,
                                  ),
                                ),
                                Positioned(
                                  left: 0,
                                  right: 0,
                                  bottom: 0,
                                  child: LinearProgressIndicator(
                                    value: work.progress / 100,
                                    minHeight: 3,
                                    color: colors.primary,
                                    backgroundColor: Colors.black26,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          work.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
