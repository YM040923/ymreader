import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:photo_view/photo_view.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

import '../../data/api/api_client.dart';
import '../../data/api/comic_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/services/reading_activity_tracker.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/reader_settings_panel.dart';
import 'novel_reader_screen.dart';
import 'work_reader_context.dart';

class _ContinuousPage {
  final WorkUnit unit;
  final int relativePage;

  const _ContinuousPage(this.unit, this.relativePage);

  int get physicalPage => unit.startPage + relativePage;
}

/// 漫画阅读器
class ComicReaderScreen extends ConsumerStatefulWidget {
  final String comicId;
  final int initialPage;
  final WorkReaderContext? workContext;

  const ComicReaderScreen({
    super.key,
    required this.comicId,
    this.initialPage = 0,
    this.workContext,
  });

  @override
  ConsumerState<ComicReaderScreen> createState() => _ComicReaderScreenState();
}

class _ComicReaderScreenState extends ConsumerState<ComicReaderScreen> {
  late PageController _pageController;
  final ScrollController _scrollController = ScrollController();
  final ItemScrollController _continuousScrollController =
      ItemScrollController();
  final ItemPositionsListener _continuousPositions =
      ItemPositionsListener.create();
  int _currentPage = 0;
  int _totalPages = 0;
  int _physicalTotalPages = 0;
  bool _showOverlay = false;
  bool _loading = true;
  String? _activeUnitId;
  List<_ContinuousPage> _continuousPageCache = const [];

  late final ComicApi _api;
  late ReadingActivityTracker _activity;

  // 设置
  ReaderSettings _settings = const ReaderSettings();

  @override
  void initState() {
    super.initState();
    _currentPage = widget.workContext?.toRelativePage(widget.initialPage) ??
        widget.initialPage;
    _activeUnitId = widget.workContext?.currentUnit.id;
    _continuousPageCache = _buildContinuousPages();
    _pageController = PageController(initialPage: _currentPage);
    // 提前缓存 API 引用
    _api = ref.read(comicApiProvider);
    _activity = ReadingActivityTracker(api: _api, comicId: widget.comicId);
    _continuousPositions.itemPositions.addListener(
      _onContinuousPositionChanged,
    );
    // 全屏沉浸模式
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _loadSettings();
    _loadPages();
  }

  @override
  void dispose() {
    // 恢复系统UI
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _activity.dispose();
    _continuousPositions.itemPositions.removeListener(
      _onContinuousPositionChanged,
    );
    _pageController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  /// 拦截返回操作，尽量在退出前完成最后一次活动同步
  Future<void> _onWillPop() async {
    await _activity.finish();
    if (!mounted) return;
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    } else if (widget.workContext != null) {
      context.go(widget.workContext!.work.detailRoute());
    }
  }

  Future<void> _loadSettings() async {
    final scope = widget.workContext != null
        ? 'work:${widget.workContext!.work.id}'
        : 'comic:${widget.comicId}';
    final libraryId = widget.workContext?.work.libraryId ?? '';
    final s = await ReaderSettings.load(
      scope: scope,
      fallbackScope: libraryId.isEmpty ? null : 'library:$libraryId',
    );
    if (mounted) setState(() => _settings = s);
  }

  Future<void> _loadPages() async {
    try {
      final data = await _api.getPages(widget.comicId);
      if (!mounted) return;

      // 如果是小说类型，自动跳转到小说阅读器
      final isNovel = data['isNovel'] == true;
      if (isNovel) {
        // 恢复系统UI后跳转（使用 Navigator 直接跳转，绕过 GoRouter）
        SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
        if (mounted) {
          Navigator.of(context).pushReplacement(
            MaterialPageRoute(
              builder: (_) => NovelReaderScreen(
                comicId: widget.comicId,
                initialChapter: widget.initialPage,
              ),
            ),
          );
        }
        return;
      }

      setState(() {
        _physicalTotalPages = data['totalPages'] ?? 0;
        final unitCount = widget.workContext?.currentUnit.pageCount ?? 0;
        final available = (_physicalTotalPages -
                (widget.workContext?.currentUnit.startPage ?? 0))
            .clamp(0, _physicalTotalPages);
        _totalPages =
            unitCount > 0 ? unitCount.clamp(0, available) : _physicalTotalPages;
        if (_totalPages > 0) {
          _currentPage = _currentPage.clamp(0, _totalPages - 1);
        }
        _loading = false;
      });
      _activity.start(_physicalPage(_currentPage), _physicalTotalPages);
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _onPageChanged(int page) {
    setState(() => _currentPage = page);
    _activity.updatePage(_physicalPage(page), _physicalTotalPages);
  }

  int _physicalPage(int relativePage) =>
      (_activeReaderContext?.currentUnit.startPage ?? 0) + relativePage;

  WorkReaderContext? get _activeReaderContext {
    final context = widget.workContext;
    if (context == null) return null;
    return WorkReaderContext(
      work: context.work,
      unitId: _activeUnitId ?? context.currentUnit.id,
    );
  }

  List<_ContinuousPage> _buildContinuousPages() {
    final context = widget.workContext;
    if (context == null) return const [];
    return [
      for (final unit in context.work.units)
        for (var page = 0; page < unit.pageCount; page++)
          _ContinuousPage(unit, page),
    ];
  }

  List<_ContinuousPage> get _continuousPages => _continuousPageCache;

  int _continuousIndexFor(WorkUnit unit, int relativePage) {
    var offset = 0;
    for (final candidate
        in widget.workContext?.work.units ?? const <WorkUnit>[]) {
      if (candidate.id == unit.id) {
        return offset + relativePage.clamp(0, candidate.pageCount - 1);
      }
      offset += candidate.pageCount;
    }
    return 0;
  }

  void _onContinuousPositionChanged() {
    if (!_settings.continuousReading ||
        _settings.mode != ComicReadingMode.webtoon) {
      return;
    }
    final positions = _continuousPositions.itemPositions.value
        .where((position) =>
            position.itemTrailingEdge > 0 && position.itemLeadingEdge < 1)
        .toList()
      ..sort((a, b) => a.itemLeadingEdge.compareTo(b.itemLeadingEdge));
    if (positions.isEmpty) return;
    final pages = _continuousPages;
    final index = positions.first.index;
    if (index < 0 || index >= pages.length) return;
    final page = pages[index];
    if (_activeUnitId != page.unit.id) {
      _switchActiveUnit(page.unit, page.relativePage);
    } else if (_currentPage != page.relativePage) {
      setState(() {
        _currentPage = page.relativePage;
        _totalPages = page.unit.pageCount;
      });
      _activity.updatePage(
        page.physicalPage,
        _trackerTotalPages(page.unit),
      );
    }
  }

  int _trackerTotalPages(WorkUnit unit) {
    if (unit.comicId == widget.comicId) return _physicalTotalPages;
    return unit.endPage + 1;
  }

  void _switchActiveUnit(WorkUnit unit, int relativePage) {
    unawaited(_activity.finish());
    _activity = ReadingActivityTracker(api: _api, comicId: unit.comicId);
    _activity.start(
      unit.startPage + relativePage,
      _trackerTotalPages(unit),
    );
    setState(() {
      _activeUnitId = unit.id;
      _currentPage = relativePage;
      _totalPages = unit.pageCount;
    });
  }

  void _toggleOverlay() {
    setState(() => _showOverlay = !_showOverlay);
  }

  void _onSettingsChanged(ReaderSettings s) {
    final previous = _settings;
    final activeContext = _activeReaderContext;
    final mustLeaveContinuousRoute = previous.continuousReading &&
        (!s.continuousReading || s.mode != ComicReadingMode.webtoon) &&
        activeContext != null &&
        activeContext.currentUnit.id != widget.workContext?.currentUnit.id;
    if (mustLeaveContinuousRoute) {
      final unit = activeContext.currentUnit;
      final absolutePage = unit.startPage + _currentPage;
      setState(() => _settings = s);
      unawaited(_activity.finish());
      context.replace(
        activeContext.routeFor(unit, absolutePage: absolutePage),
      );
      return;
    }
    if (previous.mode != s.mode) {
      _pageController.dispose();
      _pageController = PageController(initialPage: _currentPage);
    }
    setState(() => _settings = s);
  }

  void _showSettings() {
    ReaderSettingsPanel.show(
      context,
      settings: _settings,
      onChanged: _onSettingsChanged,
      scope: widget.workContext != null
          ? 'work:${widget.workContext!.work.id}'
          : 'comic:${widget.comicId}',
    );
  }

  PhotoViewComputedScale _getInitialScale() {
    switch (_settings.fitMode) {
      case FitMode.width:
        return PhotoViewComputedScale.covered;
      case FitMode.height:
        return PhotoViewComputedScale.contained;
      case FitMode.contain:
        return PhotoViewComputedScale.contained;
    }
  }

  @override
  Widget build(BuildContext context) {
    final serverUrl = ref.watch(authProvider).serverUrl;

    if (_loading) {
      return const Scaffold(
        backgroundColor: Colors.black,
        body: Center(child: CircularProgressIndicator()),
      );
    }

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        await _onWillPop();
      },
      child: Scaffold(
        backgroundColor: Colors.black,
        body: Stack(
          children: [
            // 主体 — 根据阅读模式切换
            GestureDetector(
              onTap: _toggleOverlay,
              child: _settings.mode == ComicReadingMode.webtoon
                  ? _buildWebtoonView(serverUrl)
                  : _settings.mode == ComicReadingMode.doublePage
                      ? _buildDoublePageView(serverUrl)
                      : _buildPageView(serverUrl),
            ),

            // 顶部 & 底部覆盖层
            if (_showOverlay) ...[
              _buildTopOverlay(),
              _buildBottomOverlay(),
            ],
          ],
        ),
      ),
    );
  }

  /// 单页翻页模式
  Widget _buildPageView(String serverUrl) {
    final readerContext = _activeReaderContext;
    final hasNext =
        _settings.continuousReading && readerContext?.nextUnit != null;
    return PageView.builder(
      controller: _pageController,
      itemCount: _totalPages + (hasNext ? 1 : 0),
      onPageChanged: (page) {
        if (page >= _totalPages && readerContext?.nextUnit != null) {
          _goToUnit(readerContext!.nextUnit!);
          return;
        }
        _onPageChanged(page);
      },
      reverse: _settings.direction == ReadingDirection.rtl,
      scrollDirection: _settings.direction == ReadingDirection.ttb
          ? Axis.vertical
          : Axis.horizontal,
      itemBuilder: (context, index) {
        if (index >= _totalPages) {
          return const Center(child: CircularProgressIndicator());
        }
        final physicalPage = _physicalPage(index);
        final imageUrl =
            getImageUrl(serverUrl, widget.comicId, page: physicalPage);
        return PhotoView(
          imageProvider: AuthenticatedImageProvider(
            imageUrl,
            comicId: widget.comicId,
            pageIndex: physicalPage,
          ),
          minScale: PhotoViewComputedScale.contained,
          maxScale: PhotoViewComputedScale.covered * 3,
          initialScale: _getInitialScale(),
          backgroundDecoration: const BoxDecoration(color: Colors.black),
          loadingBuilder: (_, event) => Center(
            child: CircularProgressIndicator(
              value: event?.expectedTotalBytes != null
                  ? event!.cumulativeBytesLoaded / event.expectedTotalBytes!
                  : null,
            ),
          ),
          errorBuilder: (_, __, ___) => const Center(
            child: Icon(Icons.broken_image, color: Colors.white54, size: 48),
          ),
        );
      },
    );
  }

  /// 双页模式 — 横屏时左右各显示一页，竖屏时自动回退到单页
  Widget _buildDoublePageView(String serverUrl) {
    // 计算双页对：根据 doubleCoverAlone 决定第 1 页是否单独显示
    final isLandscape =
        MediaQuery.of(context).orientation == Orientation.landscape;
    if (!isLandscape) {
      // 竖屏回退到单页模式
      return _buildPageView(serverUrl);
    }

    // 构建双页列表
    //  - doubleCoverAlone=true : [0], [1,2], [3,4], ...   （封面单页，日漫见开页对齐）
    //  - doubleCoverAlone=false: [0,1], [2,3], [4,5], ... （首页起两两配对，欧美漫合适）
    final List<List<int>> pageGroups = [];
    if (_settings.doubleCoverAlone) {
      if (_totalPages > 0) {
        pageGroups.add([0]); // 封面单独一页
      }
      for (int i = 1; i < _totalPages; i += 2) {
        if (i + 1 < _totalPages) {
          pageGroups.add([i, i + 1]);
        } else {
          pageGroups.add([i]);
        }
      }
    } else {
      for (int i = 0; i < _totalPages; i += 2) {
        if (i + 1 < _totalPages) {
          pageGroups.add([i, i + 1]);
        } else {
          pageGroups.add([i]);
        }
      }
    }
    final readerContext = _activeReaderContext;
    if (_settings.continuousReading && readerContext?.nextUnit != null) {
      pageGroups.add(const [-1]);
    }

    // 找到当前页对应的 group index
    int currentGroupIndex = 0;
    for (int g = 0; g < pageGroups.length; g++) {
      if (pageGroups[g].contains(_currentPage)) {
        currentGroupIndex = g;
        break;
      }
    }

    final doublePageController = PageController(initialPage: currentGroupIndex);

    return PageView.builder(
      controller: doublePageController,
      itemCount: pageGroups.length,
      reverse: _settings.direction == ReadingDirection.rtl,
      onPageChanged: (groupIndex) {
        final firstPage = pageGroups[groupIndex].first;
        if (firstPage < 0 && readerContext?.nextUnit != null) {
          _goToUnit(readerContext!.nextUnit!);
          return;
        }
        setState(() => _currentPage = firstPage);
        _activity.updatePage(
          _physicalPage(firstPage),
          _physicalTotalPages,
        );
      },
      itemBuilder: (context, groupIndex) {
        final pages = pageGroups[groupIndex];
        if (pages.first < 0) {
          return const Center(child: CircularProgressIndicator());
        }
        if (pages.length == 1) {
          // 单页（封面或最后一页）
          final physicalPage = _physicalPage(pages[0]);
          final imageUrl =
              getImageUrl(serverUrl, widget.comicId, page: physicalPage);
          return PhotoView(
            imageProvider: AuthenticatedImageProvider(
              imageUrl,
              comicId: widget.comicId,
              pageIndex: physicalPage,
            ),
            minScale: PhotoViewComputedScale.contained,
            maxScale: PhotoViewComputedScale.covered * 3,
            initialScale: PhotoViewComputedScale.contained,
            backgroundDecoration: const BoxDecoration(color: Colors.black),
            loadingBuilder: (_, event) => Center(
              child: CircularProgressIndicator(
                value: event?.expectedTotalBytes != null
                    ? event!.cumulativeBytesLoaded / event.expectedTotalBytes!
                    : null,
              ),
            ),
            errorBuilder: (_, __, ___) => const Center(
              child: Icon(Icons.broken_image, color: Colors.white54, size: 48),
            ),
          );
        }

        // 双页并排
        final leftPage =
            _settings.direction == ReadingDirection.rtl ? pages[1] : pages[0];
        final rightPage =
            _settings.direction == ReadingDirection.rtl ? pages[0] : pages[1];
        final physicalLeftPage = _physicalPage(leftPage);
        final physicalRightPage = _physicalPage(rightPage);
        final leftUrl =
            getImageUrl(serverUrl, widget.comicId, page: physicalLeftPage);
        final rightUrl =
            getImageUrl(serverUrl, widget.comicId, page: physicalRightPage);

        // 双页贴合：把左页右对齐、右页左对齐，让两张图在屏幕正中央拼合，去除中间缝
        final noGap = _settings.doublePageNoGap;
        return Row(
          children: [
            Expanded(
              child: AuthenticatedImage(
                imageUrl: leftUrl,
                comicId: widget.comicId,
                pageIndex: physicalLeftPage,
                fit: BoxFit.contain,
                alignment: noGap ? Alignment.centerRight : Alignment.center,
                placeholder: const Center(child: CircularProgressIndicator()),
                errorWidget: const Center(
                  child:
                      Icon(Icons.broken_image, color: Colors.white54, size: 48),
                ),
              ),
            ),
            Expanded(
              child: AuthenticatedImage(
                imageUrl: rightUrl,
                comicId: widget.comicId,
                pageIndex: physicalRightPage,
                fit: BoxFit.contain,
                alignment: noGap ? Alignment.centerLeft : Alignment.center,
                placeholder: const Center(child: CircularProgressIndicator()),
                errorWidget: const Center(
                  child:
                      Icon(Icons.broken_image, color: Colors.white54, size: 48),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  /// 长条滚动模式（Webtoon）
  Widget _buildWebtoonView(String serverUrl) {
    if (_settings.continuousReading && widget.workContext != null) {
      final pages = _continuousPages;
      final currentUnit = _activeReaderContext!.currentUnit;
      final initialIndex = _continuousIndexFor(currentUnit, _currentPage);
      return ScrollablePositionedList.builder(
        itemScrollController: _continuousScrollController,
        itemPositionsListener: _continuousPositions,
        initialScrollIndex: initialIndex,
        itemCount: pages.length,
        itemBuilder: (context, index) {
          final page = pages[index];
          final previousUnit = index == 0 ? null : pages[index - 1].unit;
          return Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (previousUnit?.id != page.unit.id)
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 10,
                  ),
                  color: Colors.black,
                  child: Text(
                    page.unit.displayLabel.isEmpty
                        ? page.unit.title
                        : page.unit.displayLabel,
                    textAlign: TextAlign.center,
                    style: const TextStyle(color: Colors.white70),
                  ),
                ),
              AuthenticatedImage(
                imageUrl: getImageUrl(
                  serverUrl,
                  page.unit.comicId,
                  page: page.physicalPage,
                ),
                comicId: page.unit.comicId,
                pageIndex: page.physicalPage,
                fit: _settings.fitMode == FitMode.width
                    ? BoxFit.fitWidth
                    : BoxFit.contain,
                placeholder: SizedBox(
                  height: MediaQuery.of(context).size.height,
                  child: const Center(child: CircularProgressIndicator()),
                ),
                errorWidget: const SizedBox(
                  height: 200,
                  child: Center(
                    child: Icon(
                      Icons.broken_image,
                      color: Colors.white54,
                      size: 48,
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      );
    }
    return NotificationListener<ScrollNotification>(
      onNotification: (notification) {
        if (notification is ScrollUpdateNotification) {
          // 根据滚动位置估算当前页码
          final viewportHeight = notification.metrics.viewportDimension;
          if (viewportHeight > 0) {
            final page = (notification.metrics.pixels / viewportHeight).floor();
            if (page != _currentPage && page >= 0 && page < _totalPages) {
              setState(() => _currentPage = page);
              _activity.updatePage(
                _physicalPage(page),
                _physicalTotalPages,
              );
            }
          }
        }
        return false;
      },
      child: ListView.builder(
        controller: _scrollController,
        itemCount: _totalPages,
        itemBuilder: (context, index) {
          final physicalPage = _physicalPage(index);
          final imageUrl =
              getImageUrl(serverUrl, widget.comicId, page: physicalPage);
          return AuthenticatedImage(
            imageUrl: imageUrl,
            comicId: widget.comicId,
            pageIndex: physicalPage,
            fit: _settings.fitMode == FitMode.width
                ? BoxFit.fitWidth
                : BoxFit.contain,
            placeholder: SizedBox(
              height: MediaQuery.of(context).size.height,
              child: const Center(child: CircularProgressIndicator()),
            ),
            errorWidget: SizedBox(
              height: 200,
              child: const Center(
                child:
                    Icon(Icons.broken_image, color: Colors.white54, size: 48),
              ),
            ),
          );
        },
      ),
    );
  }

  /// 顶部工具栏
  Widget _buildTopOverlay() {
    return Positioned(
      top: 0,
      left: 0,
      right: 0,
      child: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Colors.black87, Colors.transparent],
          ),
        ),
        child: SafeArea(
          bottom: false,
          child: Row(
            children: [
              // 返回按钮
              IconButton(
                icon: const Icon(Icons.arrow_back, color: Colors.white),
                onPressed: _onWillPop,
              ),
              // 页码
              Expanded(
                child: _settings.showPageNumber
                    ? Text(
                        '${_currentPage + 1} / $_totalPages',
                        style: const TextStyle(color: Colors.white),
                        textAlign: TextAlign.center,
                      )
                    : const SizedBox.shrink(),
              ),
              // 设置按钮
              IconButton(
                icon: const Icon(Icons.settings, color: Colors.white),
                tooltip: '阅读设置',
                onPressed: _showSettings,
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// 底部进度条
  Future<void> _goToUnit(WorkUnit unit) async {
    final readerContext = _activeReaderContext;
    if (readerContext == null) return;
    if (_settings.continuousReading &&
        _settings.mode == ComicReadingMode.webtoon &&
        _continuousScrollController.isAttached) {
      final index = _continuousIndexFor(unit, 0);
      await _continuousScrollController.scrollTo(
        index: index,
        duration: const Duration(milliseconds: 280),
        curve: Curves.easeOutCubic,
      );
      return;
    }
    await _activity.finish();
    if (mounted) {
      context.replace(
        readerContext.routeFor(unit, absolutePage: unit.startPage),
      );
    }
  }

  void _showWorkDirectory() {
    final readerContext = _activeReaderContext;
    if (readerContext == null) return;
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * 0.72,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(18, 0, 18, 12),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        readerContext.work.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(sheetContext)
                            .textTheme
                            .titleMedium
                            ?.copyWith(fontWeight: FontWeight.w700),
                      ),
                    ),
                    Text('${readerContext.currentIndex + 1}/'
                        '${readerContext.work.units.length}'),
                  ],
                ),
              ),
              const Divider(height: 1),
              Expanded(
                child: ListView.builder(
                  itemCount: readerContext.work.units.length,
                  itemBuilder: (_, index) {
                    final unit = readerContext.work.units[index];
                    final selected = unit.id == readerContext.currentUnit.id;
                    return ListTile(
                      selected: selected,
                      leading: CircleAvatar(child: Text('${index + 1}')),
                      title: Text(
                        unit.displayLabel.isEmpty
                            ? unit.title
                            : unit.displayLabel,
                      ),
                      subtitle: Text('${unit.pageCount}页'),
                      trailing: selected
                          ? const Icon(Icons.play_arrow_rounded)
                          : null,
                      onTap: () {
                        Navigator.of(sheetContext).pop();
                        _goToUnit(unit);
                      },
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBottomOverlay() {
    final readerContext = _activeReaderContext;
    final sliderMax =
        (_totalPages - 1).toDouble().clamp(0, double.infinity).toDouble();
    return Positioned(
      bottom: 0,
      left: 0,
      right: 0,
      child: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.bottomCenter,
            end: Alignment.topCenter,
            colors: [Colors.black87, Colors.transparent],
          ),
        ),
        child: SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 10),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (readerContext != null)
                  Row(
                    children: [
                      IconButton(
                        tooltip: '???',
                        onPressed: readerContext.previousUnit == null
                            ? null
                            : () => _goToUnit(readerContext.previousUnit!),
                        icon: const Icon(Icons.skip_previous_rounded),
                        color: Colors.white,
                        disabledColor: Colors.white30,
                      ),
                      Expanded(
                        child: TextButton.icon(
                          onPressed: _showWorkDirectory,
                          icon: const Icon(Icons.list_alt_rounded),
                          label: Text(
                            readerContext.currentUnit.displayLabel,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          style: TextButton.styleFrom(
                            foregroundColor: Colors.white,
                          ),
                        ),
                      ),
                      IconButton(
                        tooltip: '???',
                        onPressed: readerContext.nextUnit == null
                            ? null
                            : () => _goToUnit(readerContext.nextUnit!),
                        icon: const Icon(Icons.skip_next_rounded),
                        color: Colors.white,
                        disabledColor: Colors.white30,
                      ),
                    ],
                  ),
                Row(
                  children: [
                    Text('${_currentPage + 1}',
                        style: const TextStyle(color: Colors.white)),
                    Expanded(
                      child: Slider(
                        value: _currentPage.toDouble().clamp(0, sliderMax),
                        min: 0,
                        max: sliderMax,
                        onChanged: (v) {
                          final page = v.toInt();
                          if (_settings.mode == ComicReadingMode.webtoon) {
                            if (_settings.continuousReading &&
                                readerContext != null &&
                                _continuousScrollController.isAttached) {
                              _continuousScrollController.jumpTo(
                                index: _continuousIndexFor(
                                  readerContext.currentUnit,
                                  page,
                                ),
                              );
                            } else {
                              final viewportH =
                                  MediaQuery.of(context).size.height;
                              _scrollController.jumpTo(page * viewportH);
                              setState(() => _currentPage = page);
                            }
                          } else {
                            _pageController.jumpToPage(page);
                          }
                        },
                      ),
                    ),
                    Text('$_totalPages',
                        style: const TextStyle(color: Colors.white)),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
