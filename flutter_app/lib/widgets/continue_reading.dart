import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../data/api/work_api.dart';
import '../data/models/work.dart';
import '../data/providers/auth_provider.dart';
import 'animations.dart';
import 'authenticated_image.dart';
import 'responsive.dart';

/// 轮播下面的横向“继续阅读”长条。每本只露封面、话号和进度，
/// 不再用大面板，避免和顶部轮播抢层级。
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
    final width = MediaQuery.sizeOf(context).width;
    final horizontal = pagePadding(width);

    return Padding(
      padding: EdgeInsets.fromLTRB(horizontal, 14, horizontal, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text(
                '继续阅读',
                style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => context.push('/records'),
                child: Text(
                  '全部',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: colors.primary,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          LayoutBuilder(
            builder: (context, constraints) {
              final itemWidth =
                  ((constraints.maxWidth - 20) / 2.35).clamp(132.0, 210.0);
              final itemHeight = itemWidth * 0.74;
              return SizedBox(
                height: itemHeight,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: _works.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (_, index) {
                    final work = _works[index];
                    final target = work.continueTarget;
                    final label = target == null
                        ? '${work.progress}%'
                        : '${target.unit.displayLabel} · ${work.progress}%';
                    return PressableScale(
                      scaleDown: 0.97,
                      onTap: () async {
                        HapticFeedback.lightImpact();
                        await context.push(work.readerRoute());
                        if (mounted) await _load();
                      },
                      child: SizedBox(
                        width: itemWidth,
                        height: itemHeight,
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(6),
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
                                child: Container(
                                  padding:
                                      const EdgeInsets.fromLTRB(8, 18, 8, 7),
                                  decoration: BoxDecoration(
                                    gradient: LinearGradient(
                                      begin: Alignment.topCenter,
                                      end: Alignment.bottomCenter,
                                      colors: [
                                        Colors.transparent,
                                        Colors.black.withValues(alpha: 0.80),
                                      ],
                                    ),
                                  ),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        work.title,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: const TextStyle(
                                          color: Colors.white,
                                          fontSize: 14,
                                          fontWeight: FontWeight.w600,
                                          height: 1.2,
                                        ),
                                      ),
                                      const SizedBox(height: 2),
                                      Text(
                                        label,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: TextStyle(
                                          color: Colors.white
                                              .withValues(alpha: 0.78),
                                          fontSize: 11.5,
                                          height: 1.2,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
