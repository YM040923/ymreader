import 'dart:async';
import 'dart:typed_data';
import 'dart:ui' show FontFeature;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:photo_view/photo_view.dart';

import '../../data/api/api_client.dart';
import '../../data/api/comic_api.dart';
import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/work_provider.dart';
import '../../data/services/page_metrics_cache.dart';
import '../../data/services/reader_warmup_session.dart';
import '../../data/services/reading_activity_tracker.dart';
import '../../widgets/authenticated_image.dart';
import '../../widgets/reader_settings_panel.dart';
import 'novel_reader_screen.dart';
import 'work_reader_context.dart';
import 'work_webtoon_view.dart';

class _ContinuousPage {
  final WorkUnit unit;
  final int relativePage;

  const _ContinuousPage(this.unit, this.relativePage);

  int get physicalPage => unit.startPage + relativePage;
}

/// One row of the webtoon list, whether or not the reader has a Work context.
class _WebtoonEntry {
  final WorkUnit? unit;
  final int relativePage;
  final int physicalPage;
  final String comicId;
  final String? header;

  const _WebtoonEntry({
    required this.unit,
    required this.relativePage,
    required this.physicalPage,
    required this.comicId,
    this.header,
  });
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
  final WorkWebtoonController _continuousScrollController =
      WorkWebtoonController();
  int _currentPage = 0;
  int _totalPages = 0;
  int _physicalTotalPages = 0;
  bool _showOverlay = false;
  bool _loading = true;
  String? _activeUnitId;
  bool _switchingActivity = false;
  bool _favorite = false;
  List<_ContinuousPage> _continuousPageCache = const [];
  List<_WebtoonEntry> _webtoonEntriesCache = const [];

  late final ComicApi _api;
  late ReadingActivityTracker _activity;
  late ReaderWarmupSession _warmup;

  // 设置
  ReaderSettings _settings = const ReaderSettings();
  Timer? _autoReaderTimer;

  int _previewOffsetFor(ReaderSettings settings) {
    final previous = _activeReaderContext?.previousUnit;
    if (previous == null ||
        !settings.continuousReading ||
        settings.mode == ComicReadingMode.webtoon) {
      return 0;
    }
    return 1;
  }

  int get _previewOffset => _previewOffsetFor(_settings);

  @override
  void initState() {
    super.initState();
    _currentPage = widget.workContext?.toRelativePage(widget.initialPage) ??
        widget.initialPage;
    _activeUnitId = widget.workContext?.currentUnit.id;
    _favorite = widget.workContext?.work.isFavorite ?? false;
    _continuousPageCache = _buildContinuousPages();
    final initialOffset = widget.workContext?.previousUnit == null ||
            !_settings.continuousReading ||
            _settings.mode == ComicReadingMode.webtoon
        ? 0
        : 1;
    _pageController = PageController(initialPage: _currentPage + initialOffset);
    // 提前缓存 API 引用
    _api = ref.read(comicApiProvider);
    final auth = ref.read(authProvider);
    _activity = ReadingActivityTracker(
      api: _api,
      comicId: widget.comicId,
      serverKey: auth.serverUrl,
      userKey: auth.user?.id ?? 'anonymous',
    );
    _warmup = ReaderWarmupSession(api: _api, comicId: widget.comicId);
    // 全屏沉浸模式
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _loadSettings();
    _loadPages();
  }

  @override
  void dispose() {
    _overlayTimer?.cancel();
    _autoReaderTimer?.cancel();
    // 恢复系统UI与方向锁定
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _restoreOrientation();
    _activity.dispose();
    _warmup.dispose();
    _pageController.dispose();
    super.dispose();
  }

  /// 拦截返回操作，尽量在退出前完成最后一次活动同步
  Future<void> _onWillPop() async {
    _warmup.dispose();
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
    if (mounted) {
      final offset = _previewOffsetFor(s);
      _pageController.dispose();
      _pageController = PageController(initialPage: _currentPage + offset);
      setState(() => _settings = s);
    }
    _applyOrientation(s.orientationLock);
    _restartAutoReader();
  }

  /// 方向锁定 — 只影响阅读器内，退出时恢复
  void _applyOrientation(OrientationLock lock) {
    if (kIsWeb) return;
    try {
      switch (lock) {
        case OrientationLock.portrait:
          SystemChrome.setPreferredOrientations(
              [DeviceOrientation.portraitUp, DeviceOrientation.portraitDown]);
        case OrientationLock.landscape:
          SystemChrome.setPreferredOrientations([
            DeviceOrientation.landscapeLeft,
            DeviceOrientation.landscapeRight
          ]);
        case OrientationLock.followSystem:
          SystemChrome.setPreferredOrientations(DeviceOrientation.values);
      }
    } catch (_) {
      // 桌面/网页端不支持，忽略即可
    }
  }

  void _restoreOrientation() {
    if (kIsWeb) return;
    try {
      SystemChrome.setPreferredOrientations(DeviceOrientation.values);
    } catch (_) {}
  }

  /// 自动阅读：按间隔推进，翻到本话末自动接下一话，全书结束就停
  void _restartAutoReader() {
    _autoReaderTimer?.cancel();
    _autoReaderTimer = null;
    final seconds = _settings.autoPageInterval;
    if (seconds <= 0) return;
    _autoReaderTimer =
        Timer.periodic(Duration(seconds: seconds), (_) => _autoStep());
  }

  void _autoStep() {
    if (!mounted) return;
    if (_settings.mode == ComicReadingMode.webtoon &&
        _continuousScrollController.hasClients) {
      final position = _continuousScrollController.position;
      final target = position.pixels + position.viewportDimension * 0.9;
      if (target < position.maxScrollExtent - 2) {
        position.animateTo(
          target,
          duration: const Duration(milliseconds: 900),
          curve: Curves.linear,
        );
        return;
      }
    }
    final readerContext = _activeReaderContext;
    final atLastPage = _currentPage >= _totalPages - 1;
    final hasNextUnit = readerContext?.nextUnit != null;
    if (atLastPage && !hasNextUnit) {
      _autoReaderTimer?.cancel();
      _autoReaderTimer = null;
      return;
    }
    _turnPage(1);
  }

  /// 音量键翻页 — 下键前进、上键后退
  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }
    if (!_settings.volumeKeyPaging) return KeyEventResult.ignored;
    if (_settings.mode == ComicReadingMode.webtoon) {
      return KeyEventResult.ignored;
    }
    if (event.physicalKey == PhysicalKeyboardKey.audioVolumeDown) {
      _turnPage(1);
      return KeyEventResult.handled;
    }
    if (event.physicalKey == PhysicalKeyboardKey.audioVolumeUp) {
      _turnPage(-1);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
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
      final unit = _activeReaderContext?.currentUnit;
      _activity.start(
        _physicalPage(_currentPage),
        _physicalTotalPages,
        workId: widget.workContext?.work.id,
        unitId: unit?.id,
        relativePage: unit == null ? null : _currentPage,
      );
      _warmup.start(_physicalPage(_currentPage), _physicalTotalPages);
      unawaited(_prefetchPages(_currentPage));
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _onPageChanged(int page) {
    setState(() => _currentPage = page);
    _trackPage(page);
  }

  void _trackPage(int relativePage, {bool flushImmediately = false}) {
    if (_switchingActivity) return;
    final unit = _activeReaderContext?.currentUnit;
    _activity.updatePage(
      _physicalPage(relativePage),
      unit == null ? _physicalTotalPages : _trackerTotalPages(unit),
      workId: widget.workContext?.work.id,
      unitId: unit?.id,
      relativePage: unit == null ? null : relativePage,
      flushImmediately: flushImmediately,
    );
    unawaited(_prefetchPages(relativePage));
  }

  /// 下一话要提前铺多少张：一屏长条通常 2-3 张，留出滚动余量取 8。
  static const int _aheadPages = 8;

  Future<void> _prefetchPages(int relativePage) async {
    final unit = _activeReaderContext?.currentUnit;
    // 跨进下一话后 comicId 就变了，预热必须跟着走。早先这里是「不是本话就
    // 整段跳过」，于是换过一次话之后一张都不再预取。
    final comicId = unit?.comicId ?? widget.comicId;
    final serverUrl = ref.read(authProvider).serverUrl;
    final basePage = unit?.startPage ?? 0;
    // Both directions: scrolling back up is as common as reading forward, and
    // a page whose height is known before it appears never moves the list.
    for (final offset in const [0, 1, 2, 3, -1, -2, -3]) {
      final nextRelative = relativePage + offset;
      if (!mounted || _currentPage != relativePage) return;
      if (nextRelative < 0 || nextRelative >= _totalPages) continue;
      final physicalPage = basePage + nextRelative;
      final bytes = await _warmPage(serverUrl, comicId, physicalPage);
      if (offset == 0 &&
          bytes != null &&
          mounted &&
          _currentPage == relativePage) {
        _warmup.prefetch(physicalPage);
      }
    }
    // Make the chapter hand-off seamless: the first pages of the next unit and
    // the last page of the previous one are warmed before they are reached.
    if (!mounted || _currentPage != relativePage) return;
    final readerContext = _activeReaderContext;
    if (readerContext == null) return;
    if (relativePage >= _totalPages - _aheadPages) {
      final next = readerContext.nextUnit;
      if (next != null && next.id != readerContext.currentUnit.id) {
        for (var index = 0;
            index < next.pageCount.clamp(0, _aheadPages);
            index++) {
          if (!mounted || _currentPage != relativePage) return;
          // 换页时这段会从头再走一遍，已下好的直接从内存缓存返回，
          // 正在下的与图片组件共用同一个 in-flight，不会重复请求。
          await _warmPage(serverUrl, next.comicId, next.startPage + index);
        }
      }
    }
    if (relativePage <= 1) {
      final previous = readerContext.previousUnit;
      if (previous != null && previous.pageCount > 0) {
        await _warmPage(
          serverUrl,
          previous.comicId,
          previous.endPage,
        );
      }
    }
  }

  Future<Uint8List?> _warmPage(
    String serverUrl,
    String comicId,
    int physicalPage,
  ) async {
    try {
      final bytes = await loadAuthenticatedImageBytes(
        getImageUrl(serverUrl, comicId, page: physicalPage),
        comicId: comicId,
        pageIndex: physicalPage,
      );
      if (PageMetricsCache.instance.ratioFor(comicId, physicalPage) == null) {
        final measured = await PageMetricsCache.measure(bytes);
        if (measured != null) {
          PageMetricsCache.instance.record(comicId, physicalPage, measured);
        }
      }
      return bytes;
    } catch (_) {
      // A failed speculative load must not interrupt the visible page.
      return null;
    }
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

  int _trackerTotalPages(WorkUnit unit) {
    if (unit.comicId == widget.comicId) return _physicalTotalPages;
    return unit.endPage + 1;
  }

  void _switchActiveUnit(WorkUnit unit, int relativePage) {
    setState(() {
      _activeUnitId = unit.id;
      _currentPage = relativePage;
      _totalPages = unit.pageCount;
    });
    // 服务端读提前也是「一本 comic 一份租约」，跨话时换绑重挂，
    // 否则租约还挂在进来时那一本上，后面的话全靠客户端一张张拉。
    _warmup.rebind(unit.comicId);
    _warmup.start(unit.startPage + relativePage, _trackerTotalPages(unit));
    if (_activity.comicId == unit.comicId) {
      _trackPage(relativePage, flushImmediately: true);
      return;
    }
    unawaited(_replaceActivityForUnit(unit, relativePage));
  }

  Future<void> _replaceActivityForUnit(
    WorkUnit unit,
    int relativePage,
  ) async {
    if (_switchingActivity) return;
    _switchingActivity = true;
    await _activity.finish();
    if (!mounted || _activeUnitId != unit.id) {
      _switchingActivity = false;
      return;
    }
    final auth = ref.read(authProvider);
    _activity = ReadingActivityTracker(
      api: _api,
      comicId: unit.comicId,
      serverKey: auth.serverUrl,
      userKey: auth.user?.id ?? 'anonymous',
    );
    _activity.start(
      unit.startPage + relativePage,
      _trackerTotalPages(unit),
      workId: widget.workContext?.work.id,
      unitId: unit.id,
      relativePage: relativePage,
    );
    _switchingActivity = false;
  }

  void _toggleOverlay() {
    setState(() => _showOverlay = !_showOverlay);
    _restartOverlayTimer();
  }

  Timer? _overlayTimer;

  void _restartOverlayTimer() {
    _overlayTimer?.cancel();
    if (!_showOverlay) return;
    _overlayTimer = Timer(const Duration(seconds: 4), () {
      if (mounted && _showOverlay) {
        setState(() => _showOverlay = false);
      }
    });
  }

  /// 弹出目录/设置这类覆盖层前先把阅读器的顶栏底栏收掉，
  /// 否则横屏居中卡片四周会露出两套控件，画面很脏。
  void _hideOverlay() {
    _overlayTimer?.cancel();
    if (_showOverlay) setState(() => _showOverlay = false);
  }

  /// Single tap: left third goes back a page, right third goes forward, the
  /// middle shows the bars — the gesture set every domestic comic reader uses.
  void _handleReaderTap(TapUpDetails details) {
    if (_showOverlay) {
      setState(() => _showOverlay = false);
      _overlayTimer?.cancel();
      return;
    }
    if (_settings.mode == ComicReadingMode.webtoon) {
      _toggleOverlay();
      return;
    }
    final size = MediaQuery.sizeOf(context);
    // 左右（或上下）点击区对调：把翻页方向取反，中间呼出菜单不变
    final step = _settings.swapTapZones ? -1 : 1;
    if (_settings.direction == ReadingDirection.ttb) {
      // 上下翻页：上三分之一回退，下三分之一前进。
      final dy = details.localPosition.dy;
      if (dy < size.height / 3) {
        _turnPage(-step);
      } else if (dy > size.height * 2 / 3) {
        _turnPage(step);
      } else {
        _toggleOverlay();
      }
      return;
    }
    final dx = details.localPosition.dx;
    if (dx < size.width / 3) {
      _turnPage(-step);
    } else if (dx > size.width * 2 / 3) {
      _turnPage(step);
    } else {
      _toggleOverlay();
    }
  }

  void _turnPage(int delta) {
    final readerContext = _activeReaderContext;
    if (_settings.mode == ComicReadingMode.doublePage) {
      final target = _currentPage + delta * 2;
      if (target < 0) {
        final previous = readerContext?.previousUnit;
        if (previous != null) {
          _goToUnit(previous, page: previous.pageCount - 1);
        }
        return;
      }
      if (target >= _totalPages) {
        final next = readerContext?.nextUnit;
        if (next != null) _goToUnit(next);
        return;
      }
      setState(() => _currentPage = target);
      _trackPage(target);
      return;
    }
    final target = _currentPage + delta;
    if (target < 0) {
      final previous = readerContext?.previousUnit;
      if (previous != null) {
        _goToUnit(previous, page: previous.pageCount - 1);
      }
      return;
    }
    if (target >= _totalPages) {
      final next = readerContext?.nextUnit;
      if (next != null) _goToUnit(next);
      return;
    }
    if (_settings.mode == ComicReadingMode.webtoon) {
      final entries = _webtoonEntriesCache;
      final index = _webtoonIndexFor(
        entries,
        readerContext?.currentUnit.id ?? _activeUnitId,
        target,
      );
      if (index != null && _continuousScrollController.hasClients) {
        _continuousScrollController.jumpToPage(index);
      }
      setState(() => _currentPage = target);
      _trackPage(target);
      return;
    }
    _pageController.animateToPage(
      target + _previewOffset,
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
    );
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
    if (previous.mode != s.mode ||
        previous.continuousReading != s.continuousReading) {
      final offset = _previewOffsetFor(s);
      _pageController.dispose();
      _pageController = PageController(initialPage: _currentPage + offset);
    }
    setState(() => _settings = s);
    if (previous.orientationLock != s.orientationLock) {
      _applyOrientation(s.orientationLock);
    }
    if (previous.autoPageInterval != s.autoPageInterval) {
      _restartAutoReader();
    }
    final enteredWebtoon = s.mode == ComicReadingMode.webtoon &&
        (previous.mode != ComicReadingMode.webtoon ||
            (s.continuousReading && !previous.continuousReading));
    if (enteredWebtoon) {
      final unit = _activeReaderContext?.currentUnit;
      if (unit != null) {
        final index = _webtoonIndexFor(
              _webtoonEntriesCache,
              unit.id,
              _currentPage,
            ) ??
            _continuousIndexFor(unit, _currentPage);
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          _continuousScrollController.jumpToPage(index);
        });
      }
    }
  }

  void _showSettings() {
    _hideOverlay();
    ReaderSettingsPanel.show(
      context,
      settings: _settings,
      onChanged: _onSettingsChanged,
      scope: widget.workContext != null
          ? 'work:${widget.workContext!.work.id}'
          : 'comic:${widget.comicId}',
    );
  }

  Future<void> _toggleFavorite() async {
    final work = widget.workContext?.work;
    if (work == null) return;
    final next = !_favorite;
    setState(() => _favorite = next);
    try {
      await ref.read(workApiProvider).setFavorite(work.id, next);
      ref.invalidate(workDetailProvider(work.id));
    } catch (_) {
      if (!mounted) return;
      setState(() => _favorite = !next);
    }
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
      child: Focus(
        autofocus: true,
        canRequestFocus: true,
        onKeyEvent: _handleKeyEvent,
        child: Scaffold(
          backgroundColor: Colors.black,
          body: Stack(
            children: [
              // 主体 — 根据阅读模式切换
              GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTapUp: _handleReaderTap,
                child: _settings.mode == ComicReadingMode.webtoon
                    ? _buildWebtoonView(serverUrl)
                    : _settings.mode == ComicReadingMode.doublePage
                        ? _buildDoublePageView(serverUrl)
                        : _buildPageView(serverUrl),
              ),

              // 阅读亮度：只压暗画面，不动系统设置，也不吃手势
              if (_settings.brightness < 1.0)
                Positioned.fill(
                  child: IgnorePointer(
                    child: ColoredBox(
                      color: Colors.black
                          .withValues(alpha: (1 - _settings.brightness) * 0.85),
                    ),
                  ),
                ),

              // 顶部 & 底部覆盖层
              if (_showOverlay) ...[
                _buildTopOverlay(),
                _buildBottomOverlay(),
              ],
            ],
          ),
        ),
      ),
    );
  }

  /// 本话末尾多出来的那一格：原来只放一根转圈，划过去看到的是一片黑。
  /// 现在直接把下一话第一页铺上，落定后换成正式路由，交接看不出接缝。
  Widget _nextUnitPreview(String serverUrl) {
    final next = _activeReaderContext?.nextUnit;
    if (next == null || next.pageCount <= 0) {
      return const Center(child: CircularProgressIndicator());
    }
    return AuthenticatedImage(
      imageUrl: getImageUrl(serverUrl, next.comicId, page: next.startPage),
      comicId: next.comicId,
      pageIndex: next.startPage,
      fit: BoxFit.contain,
      placeholder: const Center(child: CircularProgressIndicator()),
      errorWidget: const Center(
        child: Icon(Icons.broken_image, color: Colors.white54, size: 48),
      ),
    );
  }

  Widget _previousUnitPreview(String serverUrl) {
    final previous = _activeReaderContext?.previousUnit;
    if (previous == null || previous.pageCount <= 0) {
      return const Center(child: CircularProgressIndicator());
    }
    return AuthenticatedImage(
      imageUrl:
          getImageUrl(serverUrl, previous.comicId, page: previous.endPage),
      comicId: previous.comicId,
      pageIndex: previous.endPage,
      fit: BoxFit.contain,
      placeholder: const Center(child: CircularProgressIndicator()),
      errorWidget: const Center(
        child: Icon(Icons.broken_image, color: Colors.white54, size: 48),
      ),
    );
  }

  /// 单页翻页模式
  Widget _buildPageView(String serverUrl) {
    final readerContext = _activeReaderContext;
    final hasPrev =
        _settings.continuousReading && readerContext?.previousUnit != null;
    final hasNext =
        _settings.continuousReading && readerContext?.nextUnit != null;
    final offset = hasPrev ? 1 : 0;
    return PageView.builder(
      controller: _pageController,
      itemCount: _totalPages + offset + (hasNext ? 1 : 0),
      onPageChanged: (page) {
        if (hasPrev && page == 0) {
          final previous = readerContext?.previousUnit;
          if (previous != null) {
            _goToUnit(previous, page: previous.pageCount - 1);
          }
          return;
        }
        final pageIndex = page - offset;
        if (pageIndex >= _totalPages && readerContext?.nextUnit != null) {
          _goToUnit(readerContext!.nextUnit!);
          return;
        }
        _onPageChanged(pageIndex);
      },
      reverse: _settings.direction == ReadingDirection.rtl,
      scrollDirection: _settings.direction == ReadingDirection.ttb
          ? Axis.vertical
          : Axis.horizontal,
      itemBuilder: (context, index) {
        if (hasPrev && index == 0) {
          return _previousUnitPreview(serverUrl);
        }
        final pageIndex = index - offset;
        if (pageIndex >= _totalPages) {
          return _nextUnitPreview(serverUrl);
        }
        final physicalPage = _physicalPage(pageIndex);
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
    if (_settings.continuousReading && readerContext?.previousUnit != null) {
      pageGroups.insert(0, const [-2]);
    }
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
        if (firstPage == -2 && readerContext?.previousUnit != null) {
          final previous = readerContext!.previousUnit!;
          _goToUnit(previous, page: previous.pageCount - 1);
          return;
        }
        if (firstPage == -1 && readerContext?.nextUnit != null) {
          _goToUnit(readerContext!.nextUnit!);
          return;
        }
        setState(() => _currentPage = firstPage);
        _trackPage(firstPage);
      },
      itemBuilder: (context, groupIndex) {
        final pages = pageGroups[groupIndex];
        if (pages.first == -2) {
          return _previousUnitPreview(serverUrl);
        }
        if (pages.first == -1) {
          return _nextUnitPreview(serverUrl);
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
    final entries = _buildWebtoonEntries();
    _webtoonEntriesCache = entries;
    if (entries.isEmpty) {
      return const Center(
        child: Icon(Icons.broken_image, color: Colors.white54, size: 48),
      );
    }
    final initialIndex = _webtoonIndexFor(
          entries,
          _activeUnitId,
          _currentPage,
        ) ??
        0;
    return WorkWebtoonView(
      controller: _continuousScrollController,
      // 长条模式的方向真正生效：上→下竖向铺，左/右→横向铺。
      axis: _settings.direction == ReadingDirection.ttb
          ? Axis.vertical
          : Axis.horizontal,
      reverse: _settings.direction == ReadingDirection.rtl,
      pages: [
        for (final entry in entries)
          WorkWebtoonPage(
            imageUrl: getImageUrl(
              serverUrl,
              entry.comicId,
              page: entry.physicalPage,
            ),
            comicId: entry.comicId,
            pageIndex: entry.physicalPage,
            header: entry.header,
          ),
      ],
      initialPage: initialIndex,
      fit:
          _settings.fitMode == FitMode.width ? BoxFit.fitWidth : BoxFit.contain,
      onPageChanged: (index) => _onWebtoonIndexChanged(entries, index),
    );
  }

  List<_WebtoonEntry> _buildWebtoonEntries() {
    final readerContext = _activeReaderContext;
    final continuous = _settings.continuousReading && readerContext != null;
    if (continuous) {
      final entries = <_WebtoonEntry>[];
      String? previousUnitId;
      for (final page in _continuousPages) {
        final unit = page.unit;
        entries.add(_WebtoonEntry(
          unit: unit,
          relativePage: page.relativePage,
          physicalPage: page.physicalPage,
          comicId: unit.comicId,
          header: unit.id == previousUnitId ? null : _unitHeader(unit),
        ));
        previousUnitId = unit.id;
      }
      return entries;
    }
    final unit = readerContext?.currentUnit;
    final comicId = unit?.comicId ?? widget.comicId;
    final basePage = unit?.startPage ?? 0;
    final count =
        unit != null ? unit.pageCount.clamp(0, _totalPages) : _totalPages;
    return [
      for (var index = 0; index < count; index++)
        _WebtoonEntry(
          unit: unit,
          relativePage: index,
          physicalPage: basePage + index,
          comicId: comicId,
          header: index == 0 && unit != null ? _unitHeader(unit) : null,
        ),
    ];
  }

  String _unitHeader(WorkUnit unit) =>
      unit.displayLabel.isEmpty ? unit.title : unit.displayLabel;

  int? _webtoonIndexFor(
    List<_WebtoonEntry> entries,
    String? unitId,
    int relativePage,
  ) {
    if (entries.isEmpty) return null;
    for (var index = 0; index < entries.length; index++) {
      final entry = entries[index];
      if (entry.unit?.id == unitId && entry.relativePage == relativePage) {
        return index;
      }
    }
    for (var index = 0; index < entries.length; index++) {
      if (entries[index].unit?.id != unitId) continue;
      final target = index + relativePage;
      if (target < entries.length && entries[target].unit?.id == unitId) {
        return target;
      }
      return index;
    }
    return 0;
  }

  void _onWebtoonIndexChanged(List<_WebtoonEntry> entries, int index) {
    if (index < 0 || index >= entries.length) return;
    final entry = entries[index];
    final unit = entry.unit;
    if (unit == null) {
      if (_currentPage != entry.relativePage) {
        setState(() => _currentPage = entry.relativePage);
        _trackPage(entry.relativePage);
      }
      return;
    }
    if (_activeUnitId != unit.id) {
      _switchActiveUnit(unit, entry.relativePage);
    } else if (_currentPage != entry.relativePage) {
      setState(() {
        _currentPage = entry.relativePage;
        _totalPages = unit.pageCount;
      });
      _trackPage(entry.relativePage);
    }
  }

  /// 顶部工具栏 — 与后续章节一致：只有返回、章节名和工具入口。
  void _openDetail() {
    final work = widget.workContext?.work;
    if (work != null) {
      context.push(work.detailRoute());
      return;
    }
    context.push('/comic/${Uri.encodeComponent(widget.comicId)}');
  }

  Widget _buildTopOverlay() {
    final readerContext = _activeReaderContext;
    final chapter = readerContext == null
        ? ''
        : readerContext.currentUnit.displayLabel.isEmpty
            ? readerContext.currentUnit.title
            : readerContext.currentUnit.displayLabel;
    return Positioned(
      top: 0,
      left: 12,
      right: 12,
      child: SafeArea(
        bottom: false,
        child: Padding(
          padding: const EdgeInsets.only(top: 2),
          // 顶栏空白处同样吃掉点击，避免漏到翻页热区
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () {},
            child: ClipRRect(
              borderRadius: BorderRadius.circular(16),
              child: Container(
                height: 46,
                padding: const EdgeInsets.symmetric(horizontal: 2),
                decoration: BoxDecoration(
                  color: const Color(0xF20E0E10),
                  borderRadius: BorderRadius.circular(16),
                  border: Border.all(
                    color: Colors.white.withValues(alpha: 0.08),
                  ),
                ),
                child: Row(
                  children: [
                    _ReaderChromeButton(
                      icon: Icons.arrow_back_ios_new_rounded,
                      tooltip: '返回',
                      onPressed: _onWillPop,
                    ),
                    Expanded(
                      child: Text(
                        chapter,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    _ReaderChromeButton(
                      icon: Icons.info_outline_rounded,
                      tooltip: '详情',
                      onPressed: _openDetail,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  /// 底部进度条
  Future<void> _goToUnit(WorkUnit unit, {int page = 0}) async {
    final readerContext = _activeReaderContext;
    if (readerContext == null) return;
    if (_settings.continuousReading &&
        _settings.mode == ComicReadingMode.webtoon &&
        _continuousScrollController.hasClients) {
      final index = _continuousIndexFor(unit, page);
      _continuousScrollController.jumpToPage(index);
      return;
    }
    await _activity.finish();
    if (mounted) {
      context.replace(
        readerContext.routeFor(unit, absolutePage: unit.startPage + page),
      );
    }
  }

  void _showWorkDirectory() {
    final readerContext = _activeReaderContext;
    if (readerContext == null) return;
    _hideOverlay();
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF17171A),
      showDragHandle: false,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * 0.72,
          child: Column(
            children: [
              const SizedBox(height: 16),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        readerContext.work.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 17,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    Text(
                      '${readerContext.currentIndex + 1}/'
                      '${readerContext.work.units.length}',
                      style: const TextStyle(
                        color: Colors.white54,
                        fontSize: 11,
                        fontFeatures: [FontFeature.tabularFigures()],
                      ),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: ListView.builder(
                  itemCount: readerContext.work.units.length,
                  itemBuilder: (_, index) {
                    final unit = readerContext.work.units[index];
                    final selected = unit.id == readerContext.currentUnit.id;
                    return GestureDetector(
                      behavior: HitTestBehavior.opaque,
                      onTap: () {
                        Navigator.of(sheetContext).pop();
                        _goToUnit(unit);
                      },
                      child: SizedBox(
                        height: 56,
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 16),
                          child: Row(
                            children: [
                              SizedBox(
                                width: 14,
                                child: selected
                                    ? Center(
                                        child: Container(
                                          width: 6,
                                          height: 6,
                                          decoration: const BoxDecoration(
                                            color: Color(0xFFFF7A6B),
                                            shape: BoxShape.circle,
                                          ),
                                        ),
                                      )
                                    : null,
                              ),
                              Expanded(
                                child: Text(
                                  unit.displayLabel.isEmpty
                                      ? unit.title
                                      : unit.displayLabel,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    color: selected
                                        ? Colors.white
                                        : Colors.white70,
                                    fontSize: 14,
                                    fontWeight: selected
                                        ? FontWeight.w600
                                        : FontWeight.w500,
                                  ),
                                ),
                              ),
                              const SizedBox(width: 12),
                              Text(
                                '${unit.pageCount} 页',
                                style: const TextStyle(
                                  color: Colors.white38,
                                  fontSize: 11,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
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
    final pageLabel = _settings.showPageNumber ? '${_currentPage + 1}' : '';
    return Positioned(
      bottom: 0,
      left: 12,
      right: 12,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (widget.workContext != null) _buildFavoritePill(),
              // 卡片空白处必须吃掉点击，否则会漏到底下的翻页热区
              GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () {},
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(16),
                  child: Container(
                    decoration: BoxDecoration(
                      color: const Color(0xF20E0E10),
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(
                        color: Colors.white.withValues(alpha: 0.08),
                      ),
                    ),
                    padding: const EdgeInsets.fromLTRB(8, 4, 8, 2),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        SizedBox(
                          height: 46,
                          child: Row(
                            children: [
                              _ToolbarAction(
                                icon: Icons.menu_book_rounded,
                                label: '目录',
                                onPressed: readerContext == null
                                    ? null
                                    : _showWorkDirectory,
                              ),
                              // 章节名只在顶栏出现一次，这里两端收边
                              const Spacer(),
                              _ToolbarAction(
                                icon: Icons.tune_rounded,
                                label: '设置',
                                onPressed: _showSettings,
                              ),
                            ],
                          ),
                        ),
                        Row(
                          children: [
                            _ChapterStepButton(
                              icon: Icons.first_page_rounded,
                              tooltip: '上一话',
                              onPressed: readerContext?.previousUnit == null
                                  ? null
                                  : () =>
                                      _goToUnit(readerContext!.previousUnit!),
                            ),
                            SizedBox(
                              width: 30,
                              child: Text(
                                pageLabel,
                                textAlign: TextAlign.center,
                                style: const TextStyle(
                                  color: Colors.white70,
                                  fontSize: 11,
                                  fontFeatures: [FontFeature.tabularFigures()],
                                ),
                              ),
                            ),
                            Expanded(
                              child: SliderTheme(
                                data: SliderTheme.of(context).copyWith(
                                  trackHeight: 2,
                                  activeTrackColor: const Color(0xFFFF7A6B),
                                  inactiveTrackColor: Colors.white24,
                                  thumbColor: const Color(0xFFFF7A6B),
                                  thumbShape: const RoundSliderThumbShape(
                                    enabledThumbRadius: 6,
                                  ),
                                  overlayShape: const RoundSliderOverlayShape(
                                    overlayRadius: 16,
                                  ),
                                  overlayColor: Colors.white24,
                                ),
                                child: Slider(
                                  value: _currentPage
                                      .toDouble()
                                      .clamp(0, sliderMax),
                                  min: 0,
                                  max: sliderMax,
                                  onChanged: (v) {
                                    final page = v.toInt();
                                    if (_settings.mode ==
                                        ComicReadingMode.webtoon) {
                                      final entries = _webtoonEntriesCache;
                                      final target = _webtoonIndexFor(
                                        entries,
                                        readerContext?.currentUnit.id ??
                                            _activeUnitId,
                                        page,
                                      );
                                      if (target != null &&
                                          _continuousScrollController
                                              .hasClients) {
                                        _continuousScrollController
                                            .jumpToPage(target);
                                      }
                                      setState(() => _currentPage = page);
                                    } else if (_settings.mode ==
                                        ComicReadingMode.doublePage) {
                                      setState(() => _currentPage = page);
                                      _trackPage(page);
                                    } else {
                                      _pageController.jumpToPage(
                                        page + _previewOffset,
                                      );
                                    }
                                    _restartOverlayTimer();
                                  },
                                ),
                              ),
                            ),
                            SizedBox(
                              width: 30,
                              child: Text(
                                '$_totalPages',
                                textAlign: TextAlign.center,
                                style: const TextStyle(
                                  color: Colors.white70,
                                  fontSize: 11,
                                  fontFeatures: [FontFeature.tabularFigures()],
                                ),
                              ),
                            ),
                            _ChapterStepButton(
                              icon: Icons.last_page_rounded,
                              tooltip: '下一话',
                              onPressed: readerContext?.nextUnit == null
                                  ? null
                                  : () => _goToUnit(readerContext!.nextUnit!),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildFavoritePill() {
    return Padding(
      padding: const EdgeInsets.only(left: 4, bottom: 10),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(16),
          onTap: _toggleFavorite,
          child: Ink(
            decoration: BoxDecoration(
              color: const Color(0xF20E0E10),
              borderRadius: BorderRadius.circular(16),
              border: Border.all(
                color: Colors.white.withValues(alpha: 0.10),
                width: 1,
              ),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  _favorite
                      ? Icons.bookmark_rounded
                      : Icons.bookmark_border_rounded,
                  size: 16,
                  color: _favorite ? const Color(0xFFFF7A6B) : Colors.white70,
                ),
                const SizedBox(width: 6),
                Text(
                  _favorite ? '已收藏' : '收藏',
                  style: TextStyle(
                    color: _favorite ? const Color(0xFFFF7A6B) : Colors.white70,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// A compact, high-contrast icon button for chrome drawn over artwork.
class _ReaderChromeButton extends StatelessWidget {
  const _ReaderChromeButton({
    required this.icon,
    required this.tooltip,
    required this.onPressed,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      onPressed: onPressed,
      tooltip: tooltip,
      iconSize: 21,
      splashRadius: 22,
      icon: Icon(icon, color: Colors.white),
    );
  }
}

/// Icon plus a short label, used for the two ends of the toolbar card.
class _ToolbarAction extends StatelessWidget {
  const _ToolbarAction({
    required this.icon,
    required this.label,
    required this.onPressed,
  });

  final IconData icon;
  final String label;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    final color = onPressed == null ? Colors.white38 : Colors.white70;
    return InkWell(
      onTap: onPressed,
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 19, color: color),
            const SizedBox(width: 5),
            Text(
              label,
              style: TextStyle(
                color: color,
                fontSize: 12,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Previous/next chapter control on the progress row.
class _ChapterStepButton extends StatelessWidget {
  const _ChapterStepButton({
    required this.icon,
    required this.tooltip,
    required this.onPressed,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      onPressed: onPressed,
      tooltip: tooltip,
      iconSize: 20,
      visualDensity: VisualDensity.compact,
      splashRadius: 18,
      icon: Icon(
        icon,
        color: onPressed == null ? Colors.white24 : Colors.white70,
      ),
    );
  }
}
