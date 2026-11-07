import 'dart:async';

import 'package:flutter/foundation.dart'
    show precisionErrorTolerance, kDebugMode;
import 'package:flutter/material.dart';

import '../../data/services/page_metrics_cache.dart';
import '../../widgets/authenticated_image.dart';

/// The chapter band that introduces each unit, measured along the scroll axis.
const double webtoonHeaderExtent = 40;

/// How long one page runs along the scroll axis. A vertical strip pins the
/// image to the viewport width, a horizontal one to the viewport height, so
/// the same page reads longer across a wide screen when it runs sideways.
double webtoonRowExtent(Axis axis, double crossExtent, double ratio) =>
    axis == Axis.vertical ? crossExtent / ratio : crossExtent * ratio;

/// A page index plus an offset within that page survives changes in image size.
class WorkWebtoonController extends ScrollController {
  WorkWebtoonController() : super(keepScrollOffset: false);

  _PageGeometry? _geometry;
  double? _pendingOffset;
  double _savedOffset = 0;
  int? _requestedPage;

  int get page => _geometry?.pageAt(_offset) ?? 0;
  double get _offset =>
      _pendingOffset ?? (hasClients ? position.pixels : _savedOffset);

  void jumpToPage(int page) {
    _requestedPage = page;
    final geometry = _geometry;
    if (geometry == null || !hasClients) return;
    final offset = geometry.startOf(page);
    _requestedPage = null;
    _pendingOffset = null;
    jumpTo(offset.clamp(position.minScrollExtent, position.maxScrollExtent));
  }

  void _updateGeometry(_PageGeometry geometry) {
    final previous = _geometry;
    if (_requestedPage != null) {
      _pendingOffset = geometry.startOf(_requestedPage!);
      _requestedPage = null;
    } else if (previous != null && !previous.sameAs(geometry)) {
      if (previous.axis != geometry.axis ||
          previous.crossExtent != geometry.crossExtent ||
          previous.viewportExtent != geometry.viewportExtent ||
          previous.ratios.length != geometry.ratios.length) {
        // Rotation, an axis flip or a changed page count: keep the page under
        // the finger.
        final offset = _offset;
        final page = previous.pageAt(offset);
        final fraction =
            ((offset - previous.startOf(page)) / previous.extentOf(page))
                .clamp(0.0, 1.0);
        _pendingOffset =
            geometry.startOf(page) + fraction * geometry.extentOf(page);
      } else {
        // A page above the viewport learned its real height. Move the scroll
        // offset by exactly that delta so what is on screen stays still —
        // re-anchoring to a page would visibly jump while dragging.
        final offset = _offset;
        final anchor = previous.pageAt(offset);
        final delta = geometry.startOf(anchor) - previous.startOf(anchor);
        if (delta.abs() > 0.5) {
          _pendingOffset = (offset + delta).clamp(0.0, double.infinity);
          if (kDebugMode) {
            debugPrint('[webtoon] keep anchor page=$anchor delta=${delta.toStringAsFixed(1)}');
          }
        }
      }
    }
    _geometry = geometry;
  }

  @override
  ScrollPosition createScrollPosition(ScrollPhysics physics,
          ScrollContext context, ScrollPosition? oldPosition) =>
      _WebtoonPosition(
        controller: this,
        physics: physics,
        context: context,
        initialPixels: _pendingOffset ?? _savedOffset,
        oldPosition: oldPosition,
      );

  @override
  void detach(ScrollPosition position) {
    _savedOffset = position.pixels;
    super.detach(position);
  }
}

class _WebtoonPosition extends ScrollPositionWithSingleContext {
  _WebtoonPosition({
    required this.controller,
    required super.physics,
    required super.context,
    required super.initialPixels,
    super.oldPosition,
  }) : super(keepScrollOffset: false);

  final WorkWebtoonController controller;

  @override
  bool applyContentDimensions(double minScrollExtent, double maxScrollExtent) {
    final pending = controller._pendingOffset;
    if (pending != null) {
      controller._pendingOffset = null;
      final target = pending.clamp(minScrollExtent, maxScrollExtent);
      if ((target - pixels).abs() > precisionErrorTolerance) {
        correctPixels(target);
        return false;
      }
    }
    return super.applyContentDimensions(minScrollExtent, maxScrollExtent);
  }
}

class _PageGeometry {
  _PageGeometry({
    required this.axis,
    required this.crossExtent,
    required this.viewportExtent,
    required List<double?> ratios,
    required List<double> headerExtents,
  })  : ratios = List.unmodifiable(ratios),
        headerExtents = List.unmodifiable(headerExtents) {
    for (var index = 0; index < ratios.length; index++) {
      starts.add(starts.last + extentOf(index));
    }
  }

  /// Vertical strips scroll top-down and fill the width; horizontal strips
  /// scroll sideways and fill the height.
  final Axis axis;

  /// Size perpendicular to scrolling — the side the image is pinned to.
  final double crossExtent;

  /// Size along scrolling — also the fallback row size for an unknown image.
  final double viewportExtent;
  final List<double?> ratios;
  final List<double> headerExtents;
  final starts = <double>[0];

  double imageExtent(int index) {
    final ratio = ratios[index];
    if (ratio == null || ratio <= 0) return viewportExtent;
    return webtoonRowExtent(axis, crossExtent, ratio);
  }

  /// Full row size along the scroll axis.
  ///
  /// The list hands this to `itemExtentBuilder`, so it becomes the tight
  /// constraint the row is laid out inside — which means it has to be the very
  /// expression [_row] adds its children up to. The previous version derived it
  /// by subtracting two accumulated page offsets; on a long strip the running
  /// total is big enough that the rounding in that subtraction leaves the
  /// children a few ulp wider than their constraint, and past
  /// `precisionErrorTolerance` Flutter calls that a real overflow and paints
  /// yellow/black hazard stripes at every page seam.
  double extentOf(int index) {
    if (ratios.isEmpty) return viewportExtent;
    final clamped = index.clamp(0, ratios.length - 1);
    return headerExtents[clamped] + imageExtent(clamped);
  }

  int pageAt(double offset) {
    var low = 0;
    var high = ratios.length;
    while (low + 1 < high) {
      final mid = (low + high) ~/ 2;
      if (starts[mid] <= offset) {
        low = mid;
      } else {
        high = mid;
      }
    }
    return low;
  }

  double startOf(int page) =>
      ratios.isEmpty ? 0 : starts[page.clamp(0, ratios.length - 1)];

  bool sameAs(_PageGeometry other) {
    if (axis != other.axis ||
        crossExtent != other.crossExtent ||
        viewportExtent != other.viewportExtent ||
        ratios.length != other.ratios.length ||
        headerExtents.length != other.headerExtents.length) {
      return false;
    }
    for (var i = 0; i < ratios.length; i++) {
      if (ratios[i] != other.ratios[i] ||
          headerExtents[i] != other.headerExtents[i]) {
        return false;
      }
    }
    return true;
  }
}

class WorkWebtoonPage {
  final String imageUrl;
  final String? comicId;
  final int? pageIndex;
  final String? header;

  const WorkWebtoonPage({
    required this.imageUrl,
    this.comicId,
    this.pageIndex,
    this.header,
  });
}

/// Explicit row extents remain nonzero during download, decode and recycling.
class WorkWebtoonView extends StatefulWidget {
  const WorkWebtoonView({
    super.key,
    required this.controller,
    required this.pages,
    required this.initialPage,
    required this.onPageChanged,
    this.fit = BoxFit.fitWidth,
    this.axis = Axis.vertical,
    this.reverse = false,
  });

  final WorkWebtoonController controller;
  final List<WorkWebtoonPage> pages;
  final int initialPage;
  final ValueChanged<int> onPageChanged;
  final BoxFit fit;

  /// Long strips run vertically; a manga page column runs horizontally.
  final Axis axis;

  /// Right-to-left reading for horizontal strips.
  final bool reverse;

  @override
  State<WorkWebtoonView> createState() => _WorkWebtoonViewState();
}

class _WorkWebtoonViewState extends State<WorkWebtoonView> {
  late List<double?> _ratios;
  int? _reportedPage;
  bool _reportScheduled = false;

  @override
  void initState() {
    super.initState();
    _ratios = _seedRatios();
    if (widget.controller._geometry == null) {
      widget.controller._requestedPage = widget.initialPage;
    }
    unawaited(_warmMetricCache());
  }

  List<double?> _seedRatios() {
    final cache = PageMetricsCache.instance;
    return List.generate(widget.pages.length, (index) {
      final existing =
          widget.controller._geometry?.ratios.elementAtOrNull(index);
      if (existing != null && existing > 0) return existing;
      final page = widget.pages[index];
      return cache.ratioFor(page.comicId, page.pageIndex) ??
          cache.guessFor(page.comicId, page.pageIndex);
    });
  }

  Future<void> _warmMetricCache() async {
    await PageMetricsCache.instance.ensureLoaded();
    if (!mounted) return;
    var changed = false;
    for (var index = 0; index < widget.pages.length; index++) {
      final page = widget.pages[index];
      final cached =
          PageMetricsCache.instance.ratioFor(page.comicId, page.pageIndex);
      if (cached != null && cached != _ratios[index]) {
        _ratios[index] = cached;
        changed = true;
      }
    }
    if (changed) setState(() {});
  }

  @override
  void didUpdateWidget(WorkWebtoonView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final reset = oldWidget.pages.length != widget.pages.length ||
        (oldWidget.pages.isNotEmpty &&
            widget.pages.isNotEmpty &&
            oldWidget.pages.first.imageUrl != widget.pages.first.imageUrl);
    if (reset) {
      widget.controller._geometry = null;
      widget.controller._requestedPage = widget.initialPage;
      _ratios = _seedRatios();
      _reportedPage = null;
    }
  }

  void _schedulePageReport() {
    if (_reportScheduled) return;
    _reportScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _reportScheduled = false;
      if (!mounted ||
          !widget.controller.hasClients ||
          widget.pages.isEmpty) {
        return;
      }
      final page = widget.controller.page;
      if (_reportedPage != page) {
        _reportedPage = page;
        widget.onPageChanged(page);
      }
    });
  }

  void _imageSize(int index, Size size) {
    if (!mounted ||
        index >= _ratios.length ||
        size.width <= 0 ||
        size.height <= 0) {
      return;
    }
    final ratio = size.width / size.height;
    if (_ratios[index] == ratio) return;
    final page = widget.pages[index];
    final wasGuess =
        PageMetricsCache.instance.ratioFor(page.comicId, page.pageIndex) == null;
    PageMetricsCache.instance.record(page.comicId, page.pageIndex, ratio);
    if (kDebugMode && wasGuess) {
      debugPrint('[webtoon] first measure row=$index page=${page.pageIndex}');
    }
    setState(() => _ratios[index] = ratio);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return LayoutBuilder(
      builder: (context, constraints) {
        final vertical = widget.axis == Axis.vertical;
        final headers = [
          for (final page in widget.pages)
            page.header == null ? 0.0 : webtoonHeaderExtent,
        ];
        final geometry = _PageGeometry(
          axis: widget.axis,
          crossExtent: vertical ? constraints.maxWidth : constraints.maxHeight,
          viewportExtent:
              vertical ? constraints.maxHeight : constraints.maxWidth,
          ratios: _ratios,
          headerExtents: headers,
        );
        widget.controller._updateGeometry(geometry);
        _schedulePageReport();
        return NotificationListener<ScrollNotification>(
          onNotification: (_) {
            _schedulePageReport();
            return false;
          },
          child: ListView.builder(
            padding: EdgeInsets.zero,
            controller: widget.controller,
            scrollDirection: widget.axis,
            reverse: widget.reverse,
            itemCount: widget.pages.length,
            itemExtentBuilder: (index, _) => geometry.extentOf(index),
            itemBuilder: (context, index) => _row(geometry, index, colors),
          ),
        );
      },
    );
  }

  /// One page: the chapter band that introduces it, then the image itself.
  Widget _row(_PageGeometry geometry, int index, ColorScheme colors) {
    final page = widget.pages[index];
    final image = AuthenticatedImage(
      key: ValueKey(page.imageUrl),
      imageUrl: page.imageUrl,
      comicId: page.comicId,
      pageIndex: page.pageIndex,
      // 横排是按高等待铺，盒子宽就是按真实宽高比算出来的，contain 已等价铺满。
      fit: geometry.axis == Axis.vertical ? widget.fit : BoxFit.contain,
      onImageSize: (size) => _imageSize(index, size),
      placeholder: Container(
        color: const Color(0xFF0B0B0F),
        alignment: Alignment.center,
        child: const SizedBox(
          width: 18,
          height: 18,
          child: CircularProgressIndicator(
            strokeWidth: 1.5,
            color: Colors.white24,
          ),
        ),
      ),
      errorWidget: const Center(
        child: Icon(Icons.broken_image, color: Colors.white38, size: 32),
      ),
    );
    if (geometry.axis == Axis.vertical) {
      return Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (page.header != null) _band(geometry, page.header!, colors),
          SizedBox(height: geometry.imageExtent(index), child: image),
        ],
      );
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (page.header != null) _band(geometry, page.header!, colors),
        SizedBox(
          width: geometry.imageExtent(index),
          height: geometry.crossExtent,
          child: image,
        ),
      ],
    );
  }

  Widget _band(_PageGeometry geometry, String label, ColorScheme colors) {
    final text = Text(
      label,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
      textAlign: TextAlign.center,
      style: TextStyle(
          color: colors.onSurfaceVariant.withValues(alpha: 0.85)),
    );
    if (geometry.axis == Axis.vertical) {
      return Container(
        width: double.infinity,
        height: webtoonHeaderExtent,
        padding: const EdgeInsets.symmetric(horizontal: 16),
        alignment: Alignment.center,
        color: Colors.black,
        child: text,
      );
    }
    // 横排时话名占一条竖着的窄带，文字转 90° 顺着滚动方向排。
    return Container(
      width: webtoonHeaderExtent,
      height: geometry.crossExtent,
      alignment: Alignment.center,
      color: Colors.black,
      child: RotatedBox(quarterTurns: 1, child: text),
    );
  }
}
