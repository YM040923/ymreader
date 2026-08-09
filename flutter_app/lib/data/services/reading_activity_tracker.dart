import 'dart:async';
import 'dart:math';

import 'package:flutter/widgets.dart';

import '../api/comic_api.dart';
import 'reading_progress_outbox.dart';

typedef ReadingActivitySender = Future<ReadingProgressEvent> Function(
    ReadingProgressEvent event);

class ReadingActivityTracker with WidgetsBindingObserver {
  ReadingActivityTracker({
    ComicApi? api,
    required this.comicId,
    this.serverKey = 'default',
    this.userKey = 'default',
    ReadingActivitySender? sender,
    ReadingProgressOutbox? outbox,
    this.observeLifecycle = true,
  })  : _api = api,
        _sender = sender,
        _outbox = outbox ?? ReadingProgressOutbox();

  final ComicApi? _api;
  final String comicId;
  final String serverKey;
  final String userKey;
  final bool observeLifecycle;
  final ReadingActivitySender? _sender;
  final ReadingProgressOutbox _outbox;
  late final String _clientSessionId =
      'flutter-${DateTime.now().microsecondsSinceEpoch}-${Random.secure().nextInt(1 << 32)}';

  Timer? _activeTimer;
  Timer? _heartbeatTimer;
  Timer? _pageTimer;
  ReadingProgressEvent? _pending;
  Future<void>? _drainFuture;
  int _page = 0;
  int _totalPages = 0;
  int _activeSeconds = 0;
  int _sequence = 0;
  String? _workId;
  String? _unitId;
  int? _relativePage;
  bool _trackProgress = true;
  bool _started = false;
  bool _finalized = false;
  bool _resumed = true;
  bool _restoredOutbox = false;

  void start(
    int page,
    int totalPages, {
    bool trackProgress = true,
    String? workId,
    String? unitId,
    int? relativePage,
  }) {
    if (_started || totalPages <= 0) return;
    _started = true;
    _page = page;
    _totalPages = totalPages;
    _trackProgress = trackProgress;
    _workId = workId;
    _unitId = unitId;
    _relativePage = relativePage;
    _resumed = !observeLifecycle ||
        WidgetsBinding.instance.lifecycleState == null ||
        WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;
    if (observeLifecycle) WidgetsBinding.instance.addObserver(this);
    _activeTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (_resumed) _activeSeconds += 1;
    });
    _heartbeatTimer =
        Timer.periodic(const Duration(seconds: 10), (_) => flush());
    _queue();
    unawaited(_drain());
  }

  void updatePage(
    int page,
    int totalPages, {
    String? workId,
    String? unitId,
    int? relativePage,
    bool flushImmediately = false,
  }) {
    if (!_started || _finalized) return;
    final switchedUnit = unitId != null && _unitId != null && unitId != _unitId;
    _page = page;
    _totalPages = totalPages;
    _workId = workId ?? _workId;
    _unitId = unitId ?? _unitId;
    _relativePage = relativePage ?? _relativePage;
    _queue();
    _pageTimer?.cancel();
    if (flushImmediately || switchedUnit) {
      unawaited(flush());
    } else {
      _pageTimer = Timer(const Duration(milliseconds: 350), () => flush());
    }
  }

  void _queue({bool finalize = false}) {
    _pending = ReadingProgressEvent(
      serverKey: serverKey,
      userKey: userKey,
      comicId: comicId,
      workId: _workId,
      unitId: _unitId,
      relativePage: _relativePage,
      page: _page,
      totalPages: _totalPages,
      activeSeconds: _activeSeconds,
      clientSessionId: _clientSessionId,
      sequence: ++_sequence,
      finalize: finalize,
      trackProgress: _trackProgress,
    );
  }

  Future<void> flush({bool finalize = false}) {
    if (!_started || _finalized) return Future.value();
    _pageTimer?.cancel();
    _queue(finalize: finalize);
    return _drain();
  }

  Future<void> _drain() {
    return _drainFuture ??= _drainLoop().whenComplete(() {
      _drainFuture = null;
      if (_pending != null && !_finalized) unawaited(_drain());
    });
  }

  Future<void> _drainLoop() async {
    if (!_restoredOutbox) {
      _restoredOutbox = true;
      final stored = await _outbox.readAll();
      for (final event in stored.where((item) =>
          item.serverKey == serverKey &&
          item.userKey == userKey &&
          item.storageKey ==
              (_pending?.storageKey ??
                  '$serverKey\u0000$userKey\u0000${_workId ?? 'comic:$comicId'}'))) {
        try {
          await _send(event);
          await _outbox.remove(event);
        } catch (_) {
          if (_pending != null) await _outbox.put(_pending!);
          _pending = null;
          return;
        }
      }
    }
    while (_pending != null) {
      final event = _pending!;
      _pending = null;
      try {
        final confirmed = await _send(event);
        await _outbox.remove(event);
        if (confirmed.finalize) _finalized = true;
      } catch (_) {
        final latest = _pending ?? event;
        _pending = null;
        await _outbox.put(latest);
        return;
      }
    }
  }

  Future<ReadingProgressEvent> _send(ReadingProgressEvent event) async {
    if (_sender != null) return _sender(event);
    final api = _api;
    if (api == null) throw StateError('ComicApi is required');
    final data = await api.recordReadingActivity(
      comicId: event.comicId,
      clientSessionId: event.clientSessionId,
      page: event.page,
      totalPages: event.totalPages,
      activeSeconds: event.activeSeconds,
      sequence: event.sequence,
      finalize: event.finalize,
      trackProgress: event.trackProgress,
      workId: event.workId,
      unitId: event.unitId,
      relativePage: event.relativePage,
    );
    final progress = data['progress'];
    if (progress is Map) {
      return event.copyWith(
        page: progress['absolutePage'] as int? ?? event.page,
        relativePage: progress['relativePage'] as int? ?? event.relativePage,
      );
    }
    return event;
  }

  Future<void> finish() async {
    if (!_started || _finalized) return;
    _cancelTimers();
    await flush(finalize: true);
    if (observeLifecycle) WidgetsBinding.instance.removeObserver(this);
  }

  void dispose() {
    _cancelTimers();
    if (observeLifecycle) WidgetsBinding.instance.removeObserver(this);
    if (_started && !_finalized) {
      _queue(finalize: true);
      unawaited(_drain());
    }
  }

  void _cancelTimers() {
    _activeTimer?.cancel();
    _heartbeatTimer?.cancel();
    _pageTimer?.cancel();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _resumed = state == AppLifecycleState.resumed;
    if (!_resumed) unawaited(flush());
  }
}
