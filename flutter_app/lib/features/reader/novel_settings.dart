import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 小说阅读主题
enum NovelTheme { night, day, green, gray, white }

/// 字体
enum NovelFont { system, serif, sans, kai, mono }

/// 页边距
enum NovelPadding { compact, standard, wide }

/// 翻页模式
enum NovelPageMode { scroll, swipe }

/// 自动滚动速度
enum AutoScrollSpeed { slow, medium, fast }

/// 书签
class NovelBookmark {
  final int chapterIndex;
  final String chapterTitle;
  final int timestamp;

  const NovelBookmark({
    required this.chapterIndex,
    required this.chapterTitle,
    required this.timestamp,
  });

  Map<String, dynamic> toJson() => {
        'chapterIndex': chapterIndex,
        'chapterTitle': chapterTitle,
        'timestamp': timestamp,
      };

  factory NovelBookmark.fromJson(Map<String, dynamic> json) => NovelBookmark(
        chapterIndex: json['chapterIndex'] as int,
        chapterTitle: json['chapterTitle'] as String,
        timestamp: json['timestamp'] as int,
      );
}

/// 小说阅读设置
class NovelSettings {
  final double fontSize;
  final double lineHeight;
  final NovelTheme theme;
  final NovelFont font;
  final NovelPadding padding;
  final NovelPageMode pageMode;
  final int autoScrollSpeed; // 1=慢 2=中 3=快

  const NovelSettings({
    this.fontSize = 18,
    this.lineHeight = 1.8,
    this.theme = NovelTheme.night,
    this.font = NovelFont.system,
    this.padding = NovelPadding.standard,
    this.pageMode = NovelPageMode.scroll,
    this.autoScrollSpeed = 2,
  });

  NovelSettings copyWith({
    double? fontSize,
    double? lineHeight,
    NovelTheme? theme,
    NovelFont? font,
    NovelPadding? padding,
    NovelPageMode? pageMode,
    int? autoScrollSpeed,
  }) {
    return NovelSettings(
      fontSize: fontSize ?? this.fontSize,
      lineHeight: lineHeight ?? this.lineHeight,
      theme: theme ?? this.theme,
      font: font ?? this.font,
      padding: padding ?? this.padding,
      pageMode: pageMode ?? this.pageMode,
      autoScrollSpeed: autoScrollSpeed ?? this.autoScrollSpeed,
    );
  }

  static Future<NovelSettings> load() async {
    final prefs = await SharedPreferences.getInstance();
    return NovelSettings(
      fontSize: prefs.getDouble('novel_fontSize') ?? 18,
      lineHeight: prefs.getDouble('novel_lineHeight') ?? 1.8,
      theme: NovelTheme.values[
          (prefs.getInt('novel_theme') ?? 0).clamp(0, NovelTheme.values.length - 1)],
      font: NovelFont.values[
          (prefs.getInt('novel_font') ?? 0).clamp(0, NovelFont.values.length - 1)],
      padding: NovelPadding.values[
          (prefs.getInt('novel_padding') ?? 1).clamp(0, NovelPadding.values.length - 1)],
      pageMode: NovelPageMode.values[
          (prefs.getInt('novel_pageMode') ?? 0).clamp(0, NovelPageMode.values.length - 1)],
      autoScrollSpeed: (prefs.getInt('novel_autoScrollSpeed') ?? 2).clamp(1, 3),
    );
  }

  Future<void> save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setDouble('novel_fontSize', fontSize);
    await prefs.setDouble('novel_lineHeight', lineHeight);
    await prefs.setInt('novel_theme', theme.index);
    await prefs.setInt('novel_font', font.index);
    await prefs.setInt('novel_padding', padding.index);
    await prefs.setInt('novel_pageMode', pageMode.index);
    await prefs.setInt('novel_autoScrollSpeed', autoScrollSpeed);
  }

  /// 水平内边距
  double get horizontalPadding {
    switch (padding) {
      case NovelPadding.compact:
        return 12;
      case NovelPadding.standard:
        return 24;
      case NovelPadding.wide:
        return 40;
    }
  }

  /// 字体族
  String? get fontFamily {
    switch (font) {
      case NovelFont.system:
        return null;
      case NovelFont.serif:
        return 'serif';
      case NovelFont.sans:
        return 'sans-serif';
      case NovelFont.kai:
        return 'KaiTi';
      case NovelFont.mono:
        return 'monospace';
    }
  }

  Color get backgroundColor {
    switch (theme) {
      case NovelTheme.night:
        return const Color(0xFF18181B);
      case NovelTheme.day:
        return const Color(0xFFFFFBEB);
      case NovelTheme.green:
        return const Color(0xFFC7EDCC);
      case NovelTheme.gray:
        return const Color(0xFFE0E0E0);
      case NovelTheme.white:
        return const Color(0xFFFFFFFF);
    }
  }

  Color get textColor {
    switch (theme) {
      case NovelTheme.night:
        return const Color(0xFFE0E0E0);
      case NovelTheme.day:
        return const Color(0xFF1A1A1A);
      case NovelTheme.green:
        return const Color(0xFF1A3A1A);
      case NovelTheme.gray:
        return const Color(0xFF1A1A1A);
      case NovelTheme.white:
        return const Color(0xFF333333);
    }
  }

  Color get secondaryTextColor {
    switch (theme) {
      case NovelTheme.night:
        return const Color(0xFF888888);
      case NovelTheme.day:
        return const Color(0xFF666666);
      case NovelTheme.green:
        return const Color(0xFF4A6A4A);
      case NovelTheme.gray:
        return const Color(0xFF555555);
      case NovelTheme.white:
        return const Color(0xFF999999);
    }
  }

  bool get isDark => theme == NovelTheme.night;
}

/// 搜索结果
class SearchResult {
  final int chapterIndex;
  final String chapterTitle;
  final String matchText;
  final int matchCount;

  const SearchResult({
    required this.chapterIndex,
    required this.chapterTitle,
    required this.matchText,
    required this.matchCount,
  });
}

/// 书签工具类
class BookmarkManager {
  static const _key = 'novel_bookmarks_';

  static Future<List<NovelBookmark>> load(String comicId) async {
    final prefs = await SharedPreferences.getInstance();
    final json = prefs.getString('$_key$comicId');
    if (json == null) return [];
    try {
      final list = jsonDecode(json) as List;
      return list.map((e) => NovelBookmark.fromJson(e)).toList();
    } catch (_) {
      return [];
    }
  }

  static Future<void> save(String comicId, List<NovelBookmark> bookmarks) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      '$_key$comicId',
      jsonEncode(bookmarks.map((b) => b.toJson()).toList()),
    );
  }
}
