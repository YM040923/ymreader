import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:shared_preferences/shared_preferences.dart';

/// Remembers each page's aspect ratio (width / height) so a webtoon list can
/// lay pages out with their real heights before the image finishes decoding.
///
/// Without this every not-yet-loaded page has to be guessed, and the scroll
/// offset has to be corrected whenever the guess turns out to be wrong — which
/// is what makes scrolling back up jump around.
class PageMetricsCache {
  PageMetricsCache._();

  static final PageMetricsCache instance = PageMetricsCache._();

  static const _prefsKey = 'reader_page_ratios_v1';
  static const _maxEntries = 6000;

  final Map<String, double> _ratios = <String, double>{};
  bool _loaded = false;
  Future<void>? _loading;

  static String keyFor(String? comicId, int? pageIndex) {
    if (comicId == null || pageIndex == null) return '';
    return '$comicId:$pageIndex';
  }

  double? ratioFor(String? comicId, int? pageIndex) {
    if (!_loaded) return null;
    final ratio = _ratios[keyFor(comicId, pageIndex)];
    if (ratio == null || !ratio.isFinite || ratio <= 0) return null;
    return ratio;
  }

  /// Best-effort ratio for a page that has never been measured, borrowed from
  /// the nearest measured page of the same comic. Falls back to a typical
  /// manga page (0.7) rather than a full-screen box.
  double guessFor(String? comicId, int? pageIndex) {
    if (comicId != null && pageIndex != null) {
      for (var distance = 1; distance <= 12; distance++) {
        for (final candidate in [pageIndex - distance, pageIndex + distance]) {
          if (candidate < 0) continue;
          final ratio = _ratios['$comicId:$candidate'];
          if (ratio != null && ratio.isFinite && ratio > 0) return ratio;
        }
      }
    }
    return 0.7;
  }

  Future<void> ensureLoaded() {
    if (_loaded) return Future.value();
    return _loading ??= _load();
  }

  Future<void> _load() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = prefs.getString(_prefsKey);
      if (raw != null && raw.isNotEmpty) {
        final decoded = jsonDecode(raw);
        if (decoded is Map) {
          decoded.forEach((key, value) {
            if (key is String && value is num) {
              final ratio = value.toDouble();
              if (ratio.isFinite && ratio > 0) _ratios[key] = ratio;
            }
          });
        }
      }
    } catch (_) {
      // A corrupt cache only costs one round of layout corrections.
    }
    _loaded = true;
  }

  void record(String? comicId, int? pageIndex, double ratio) {
    if (comicId == null || pageIndex == null) return;
    if (!ratio.isFinite || ratio <= 0) return;
    final key = keyFor(comicId, pageIndex);
    if (_ratios[key] == ratio) return;
    _ratios[key] = ratio;
    _loaded = true;
    _schedulePersist();
  }

  bool _persistScheduled = false;

  void _schedulePersist() {
    if (_persistScheduled) return;
    _persistScheduled = true;
    Future<void>.delayed(const Duration(seconds: 2), () async {
      _persistScheduled = false;
      try {
        final prefs = await SharedPreferences.getInstance();
        if (_ratios.length > _maxEntries) {
          final excess = _ratios.length - _maxEntries;
          for (final key in _ratios.keys.take(excess).toList()) {
            _ratios.remove(key);
          }
        }
        await prefs.setString(_prefsKey, jsonEncode(_ratios));
      } catch (_) {
        // Persisting is an optimisation; never surface its failures.
      }
    });
  }

  /// Reads only the image header, so prefetching dimensions costs far less
  /// than decoding the page twice.
  static Future<double?> measure(Uint8List bytes) async {
    if (bytes.isEmpty) return null;
    ui.ImmutableBuffer? buffer;
    ui.ImageDescriptor? descriptor;
    try {
      buffer = await ui.ImmutableBuffer.fromUint8List(bytes);
      descriptor = await ui.ImageDescriptor.encoded(buffer);
      if (descriptor.width <= 0 || descriptor.height <= 0) return null;
      return descriptor.width / descriptor.height;
    } catch (_) {
      return null;
    } finally {
      descriptor?.dispose();
      buffer?.dispose();
    }
  }
}
