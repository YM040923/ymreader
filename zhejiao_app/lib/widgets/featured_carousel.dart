import 'dart:math';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../data/api/work_api.dart';
import '../data/models/work.dart';
import '../data/providers/auth_provider.dart';
import 'authenticated_image.dart';
import 'responsive.dart';

/// 首页精选轮播。
///
/// 不用 PageView 铺页：槽位式封面扇。所有卡片的位置/缩放/倾斜只由
/// 「它离中心几个槽位」这个连续值决定，左右是同一个函数，所以静止和
/// 拖动过程中都保持镜像对称；槽位间距按目标可见缝反推，不是靠页面边锚点。
class FeaturedCarousel extends ConsumerStatefulWidget {
  final int refreshTick;

  const FeaturedCarousel({super.key, this.refreshTick = 0});

  @override
  ConsumerState<FeaturedCarousel> createState() => _FeaturedCarouselState();
}

class _FeaturedCarouselState extends ConsumerState<FeaturedCarousel>
    with SingleTickerProviderStateMixin {
  /// 连续位置：整数表示某张卡在中心，小数表示正在拖动/吸附。
  late final AnimationController _position = AnimationController.unbounded(
    vsync: this,
    value: 0,
  );
  List<Work> _works = const [];

  @override
  void initState() {
    super.initState();
    _position.addListener(_onPositionChanged);
    _load();
  }

  void _onPositionChanged() {
    if (mounted) setState(() {});
  }

  @override
  void didUpdateWidget(covariant FeaturedCarousel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.refreshTick != widget.refreshTick) {
      _load();
    }
  }

  Future<void> _load() async {
    try {
      final response = await ref.read(workApiProvider).listWorks(
            const WorkQuery(
              pageSize: 50,
              sortBy: 'addedAt',
              sortOrder: 'desc',
            ),
          );
      if (!mounted) return;
      final pool = response.works.toList()..shuffle(Random());
      setState(() => _works = pool.take(10).toList());
    } catch (_) {
      if (mounted) setState(() => _works = const []);
    }
  }

  void _animateTo(double target) {
    if (!mounted) return;
    if ((_position.value - target).abs() < 0.001) return;
    _position.animateTo(
      target,
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  void _onDragUpdate(DragUpdateDetails details, double slotDistance) {
    _position.stop();
    // 手指左移（dx<0）→ position 增大 → 下一张进入中心
    _position.value -= details.delta.dx / slotDistance;
  }

  void _onDragEnd(DragEndDetails details) {
    final velocity = details.velocity.pixelsPerSecond.dx;
    double target;
    if (velocity < -320) {
      target = _position.value.floorToDouble() + 1;
    } else if (velocity > 320) {
      target = _position.value.ceilToDouble() - 1;
    } else {
      target = _position.value.roundToDouble();
    }
    _animateTo(target);
  }

  @override
  void dispose() {
    _position.removeListener(_onPositionChanged);
    _position.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_works.isEmpty) return const SizedBox.shrink();

    final serverUrl = ref.watch(authProvider).serverUrl;
    final colors = Theme.of(context).colorScheme;
    final width = MediaQuery.sizeOf(context).width;
    final wide = width >= 860;
    final horizontal = pagePadding(width);
    final carouselHeight = wide ? 360.0 : 208.0;
    final slideWidth = wide ? 260.0 : 156.0;
    final slideHeight = wide ? 336.0 : 208.0;

    final gap = wide ? 24.0 : 14.0;
    final innerScale = wide ? 0.74 : 0.72;
    const outerScale = 0.58;
    const innerAngle = 0.30;
    const outerAngle = 0.52;

    // 用旋转后的投影宽度反推槽位中心距，保证静止时两段可见缝都等于 gap。
    final innerHalf = slideWidth * innerScale * cos(innerAngle) / 2;
    final outerHalf = slideWidth * outerScale * cos(outerAngle) / 2;
    final d1 = slideWidth / 2 + gap + innerHalf;
    final d2 = d1 + innerHalf + gap + outerHalf;

    final count = _works.length;
    final maxSide = count <= 1 ? 0 : (wide ? 2 : 1);
    final base = _position.value.floor();
    final frac = _position.value - base;
    final current = ((_position.value.round() % count) + count) % count;

    final entries = <_FanEntry>[];
    for (var k = -maxSide - 1; k <= maxSide + 1; k++) {
      final offset = k - frac;
      if (offset.abs() > maxSide + 0.75) continue;
      final index = ((base + k) % count + count) % count;
      entries.add(_FanEntry(offset: offset, work: _works[index]));
    }
    // 外侧先画，中心最后画：主卡永远在最上层。
    entries.sort(
      (a, b) => b.offset.abs().compareTo(a.offset.abs()),
    );

    return Padding(
      padding: EdgeInsets.fromLTRB(horizontal, 14, horizontal, 0),
      child: Column(
        children: [
          SizedBox(
            width: double.infinity,
            height: carouselHeight,
            child: RawGestureDetector(
              behavior: HitTestBehavior.opaque,
              gestures: count <= 1
                  ? const <Type, GestureRecognizerFactory>{}
                  : <Type, GestureRecognizerFactory>{
                      _CarouselDragRecognizer:
                          GestureRecognizerFactoryWithHandlers<
                              _CarouselDragRecognizer>(
                        () => _CarouselDragRecognizer(),
                        (recognizer) {
                          recognizer
                            ..dragStartBehavior = DragStartBehavior.start
                            ..onStart = ((_) {
                              _position.stop();
                            })
                            ..onUpdate = ((details) {
                              _onDragUpdate(details, d1);
                            })
                            ..onEnd = _onDragEnd;
                        },
                      ),
                    },
              child: Stack(
                alignment: Alignment.center,
                clipBehavior: Clip.none,
                children: [
                  for (final entry in entries)
                    _buildCard(
                      context: context,
                      entry: entry,
                      cardKey: count >= (wide ? 7 : 5)
                          ? ValueKey('fan-${entry.work.id}')
                          : null,
                      serverUrl: serverUrl,
                      wide: wide,
                      slideWidth: slideWidth,
                      slideHeight: slideHeight,
                      innerScale: innerScale,
                      outerScale: outerScale,
                      innerAngle: innerAngle,
                      outerAngle: outerAngle,
                      d1: d1,
                      d2: d2,
                    ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              for (var i = 0; i < count; i++)
                AnimatedContainer(
                  duration: const Duration(milliseconds: 180),
                  curve: Curves.easeOut,
                  width: i == current ? 16 : 6,
                  height: 6,
                  margin: const EdgeInsets.symmetric(horizontal: 3),
                  decoration: BoxDecoration(
                    color: i == current
                        ? colors.onSurface
                        : colors.onSurfaceVariant.withValues(alpha: 0.30),
                    borderRadius: BorderRadius.circular(3),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildCard({
    required BuildContext context,
    required _FanEntry entry,
    required Key? cardKey,
    required String serverUrl,
    required bool wide,
    required double slideWidth,
    required double slideHeight,
    required double innerScale,
    required double outerScale,
    required double innerAngle,
    required double outerAngle,
    required double d1,
    required double d2,
  }) {
    final offset = entry.offset;
    final distance = offset.abs();
    final sign = offset == 0 ? 0.0 : offset.sign;

    final scale = (distance <= 1
            ? 1 + (innerScale - 1) * distance
            : innerScale +
                (outerScale - innerScale) * (distance - 1).clamp(0, 1))
        .toDouble();
    final angle = (distance <= 1
            ? innerAngle * distance
            : innerAngle +
                (outerAngle - innerAngle) * (distance - 1).clamp(0, 1))
        .toDouble();
    final opacity = (distance <= 1
            ? 1 - 0.10 * distance
            : (0.90 - 0.12 * (distance - 1)).clamp(0.0, 1.0))
        .toDouble();

    final x = (distance <= 1
            ? sign * distance * d1
            : sign * (d1 + (distance - 1).clamp(0, 1) * (d2 - d1)))
        .toDouble();
    final active = distance < 0.5;

    return Transform.translate(
      key: cardKey,
      offset: Offset(x, 0),
      child: Transform(
        transform: Matrix4.identity()
          ..setEntry(3, 2, 0.0015)
          ..rotateY(sign * angle),
        alignment: Alignment.center,
        child: Transform.scale(
          scale: scale,
          alignment: Alignment.center,
          child: Opacity(
            opacity: opacity,
            child: SizedBox(
              width: slideWidth,
              height: slideHeight,
              child: _Slide(
                work: entry.work,
                serverUrl: serverUrl,
                active: active,
                wide: wide,
                slideWidth: slideWidth,
                slideHeight: slideHeight,
                onTap: () {
                  HapticFeedback.lightImpact();
                  if (active) {
                    context.push(entry.work.detailRoute());
                  } else {
                    _animateTo((_position.value + offset).roundToDouble());
                  }
                },
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 横向拖动只在“明显是横向”时才接管，纵向优先让给页面滚动。
class _CarouselDragRecognizer extends HorizontalDragGestureRecognizer {
  Offset? _downPosition;
  Offset? _lastPosition;

  @override
  void addAllowedPointer(PointerDownEvent event) {
    _downPosition = event.position;
    _lastPosition = event.position;
    super.addAllowedPointer(event);
  }

  @override
  void handleEvent(PointerEvent event) {
    if (event is PointerMoveEvent) {
      _lastPosition = event.position;
    }
    super.handleEvent(event);
  }

  @override
  bool hasSufficientGlobalDistanceToAccept(
    PointerDeviceKind pointerDeviceKind,
    double? deviceTouchSlop,
  ) {
    final down = _downPosition;
    final last = _lastPosition;
    if (down == null || last == null) {
      return super.hasSufficientGlobalDistanceToAccept(
        pointerDeviceKind,
        deviceTouchSlop,
      );
    }
    final moved = last - down;
    final slop = computeHitSlop(pointerDeviceKind, gestureSettings);
    return moved.dx.abs() > moved.dy.abs() * 1.4 && moved.dx.abs() > slop;
  }
}

class _FanEntry {
  final double offset;
  final Work work;

  const _FanEntry({required this.offset, required this.work});
}

class _Slide extends StatelessWidget {
  final Work work;
  final String serverUrl;
  final bool active;
  final bool wide;
  final double slideWidth;
  final double slideHeight;
  final VoidCallback onTap;

  const _Slide({
    required this.work,
    required this.serverUrl,
    required this.active,
    required this.wide,
    required this.slideWidth,
    required this.slideHeight,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final meta = [
      if (work.author.isNotEmpty) work.author,
      '${work.itemCount}话 · ${work.pageCount}页',
    ].join(' · ');

    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: active ? 0.24 : 0.12),
            blurRadius: active ? 24 : 14,
            offset: Offset(0, active ? 10 : 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(12),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Stack(
            fit: StackFit.expand,
            children: [
              AuthenticatedImage(
                imageUrl: work.resolvedCoverUrl(serverUrl),
                fit: BoxFit.cover,
                placeholder: ColoredBox(color: colors.surfaceContainerHighest),
                errorWidget: ColoredBox(color: colors.surfaceContainerHighest),
              ),
              if (active)
                Positioned(
                  left: 0,
                  right: 0,
                  bottom: 0,
                  child: Container(
                    padding: EdgeInsets.fromLTRB(
                      wide ? 12 : 10,
                      wide ? 28 : 22,
                      wide ? 12 : 10,
                      wide ? 10 : 8,
                    ),
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [
                          Colors.transparent,
                          Colors.black.withValues(alpha: 0.78),
                        ],
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          work.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: wide ? 18 : 16,
                            fontWeight: FontWeight.w700,
                            height: 1.2,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          meta,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: Colors.white.withValues(alpha: 0.78),
                            fontSize: wide ? 12 : 11,
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
  }
}
