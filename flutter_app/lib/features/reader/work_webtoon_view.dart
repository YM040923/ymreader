import 'package:flutter/foundation.dart' show precisionErrorTolerance;
import 'package:flutter/material.dart';

import '../../widgets/authenticated_image.dart';

const double webtoonHeaderHeight = 40;

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
      final offset = _offset;
      final page = previous.pageAt(offset);
      final fraction =
          ((offset - previous.startOf(page)) / previous.extentOf(page))
              .clamp(0.0, 1.0);
      _pendingOffset =
          geometry.startOf(page) + fraction * geometry.extentOf(page);
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
  _PageGeometry(
    this.width,
    this.viewportHeight,
    List<double?> ratios,
    List<double> headerHeights,
  )   : ratios = List.unmodifiable(ratios),
        headerHeights = List.unmodifiable(headerHeights) {
    for (var index = 0; index < ratios.length; index++) {
      starts.add(starts.last +
          headerHeights[index] +
          (ratios[index] == null ? viewportHeight : imageExtent(index)));
    }
  }

  final double width;
  final double viewportHeight;
  final List<double?> ratios;
  final List<double> headerHeights;
  final starts = <double>[0];

  double imageExtent(int index) {
    final ratio = ratios[index];
    if (ratio == null || ratio <= 0) return viewportHeight;
    return width / ratio;
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
  double extentOf(int page) {
    if (ratios.isEmpty) return viewportHeight;
    final index = page.clamp(0, ratios.length - 1);
    return starts[index + 1] - starts[index];
  }

  bool sameAs(_PageGeometry other) {
    if (width != other.width ||
        viewportHeight != other.viewportHeight ||
        ratios.length != other.ratios.length ||
        headerHeights.length != other.headerHeights.length) {
      return false;
    }
    for (var i = 0; i < ratios.length; i++) {
      if (ratios[i] != other.ratios[i] ||
          headerHeights[i] != other.headerHeights[i]) {
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
  });

  final WorkWebtoonController controller;
  final List<WorkWebtoonPage> pages;
  final int initialPage;
  final ValueChanged<int> onPageChanged;
  final BoxFit fit;

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
    _ratios = List.generate(widget.pages.length,
        (index) => widget.controller._geometry?.ratios.elementAtOrNull(index));
    if (widget.controller._geometry == null) {
      widget.controller._requestedPage = widget.initialPage;
    }
  }

  @override
  void didUpdateWidget(WorkWebtoonView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final reset = oldWidget.pages.length != widget.pages.length ||
        (oldWidget.pages.isNotEmpty &&
            widget.pages.isNotEmpty &&
            oldWidget.pages.first.imageUrl != widget.pages.first.imageUrl);
    if (reset) {
      _ratios = List.filled(widget.pages.length, null);
      widget.controller._geometry = null;
      widget.controller._requestedPage = widget.initialPage;
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
    setState(() => _ratios[index] = ratio);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return LayoutBuilder(
      builder: (context, constraints) {
        final headers = [
          for (final page in widget.pages)
            page.header == null ? 0.0 : webtoonHeaderHeight,
        ];
        final geometry = _PageGeometry(
            constraints.maxWidth, constraints.maxHeight, _ratios, headers);
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
            itemCount: widget.pages.length,
            itemExtentBuilder: (index, _) => geometry.extentOf(index),
            itemBuilder: (context, index) {
              final page = widget.pages[index];
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (page.header != null)
                    Container(
                      width: double.infinity,
                      height: webtoonHeaderHeight,
                      padding:
                          const EdgeInsets.symmetric(horizontal: 16),
                      alignment: Alignment.center,
                      color: Colors.black,
                      child: Text(
                        page.header!,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                            color: colors.onSurfaceVariant.withValues(
                                alpha: 0.85)),
                      ),
                    ),
                  SizedBox(
                    height: geometry.imageExtent(index),
                    child: AuthenticatedImage(
                      key: ValueKey(page.imageUrl),
                      imageUrl: page.imageUrl,
                      comicId: page.comicId,
                      pageIndex: page.pageIndex,
                      fit: widget.fit,
                      onImageSize: (size) => _imageSize(index, size),
                      placeholder: const Center(
                          child: CircularProgressIndicator(strokeWidth: 2)),
                      errorWidget: const Center(
                        child: Icon(Icons.broken_image,
                            color: Colors.white54, size: 48),
                      ),
                    ),
                  ),
                ],
              );
            },
          ),
        );
      },
    );
  }
}
