import 'package:flutter/material.dart';

import 'design/theme.dart';
import 'preview/preview_page.dart';

void main() {
  runApp(const ZhejiaoPreviewApp());
}

/// M0 设计预览壳。
///
/// 真机上 Android 12+ 后续接系统壁纸色；Windows 没有系统取色时用
/// Fushi 默认的 M3 baseline 紫作为种子。两个平台都走同一套表面阶梯。
class ZhejiaoPreviewApp extends StatefulWidget {
  const ZhejiaoPreviewApp({
    super.key,
    this.fontFamily,
    this.initialBrightness = Brightness.light,
  });

  final String? fontFamily;
  final Brightness initialBrightness;

  @override
  State<ZhejiaoPreviewApp> createState() => _ZhejiaoPreviewAppState();
}

class _ZhejiaoPreviewAppState extends State<ZhejiaoPreviewApp> {
  late ThemeMode _mode = widget.initialBrightness == Brightness.dark
      ? ThemeMode.dark
      : ThemeMode.light;

  void _toggle() {
    setState(() {
      _mode = _mode == ThemeMode.dark ? ThemeMode.light : ThemeMode.dark;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '折角',
      debugShowCheckedModeBanner: false,
      locale: const Locale('zh', 'CN'),
      themeMode: _mode,
      theme: buildZhejiaoTheme(
        brightness: Brightness.light,
        fontFamily: widget.fontFamily,
        locale: const Locale('zh', 'CN'),
      ),
      darkTheme: buildZhejiaoTheme(
        brightness: Brightness.dark,
        fontFamily: widget.fontFamily,
        locale: const Locale('zh', 'CN'),
      ),
      home: PreviewPage(onToggleTheme: _toggle),
    );
  }
}
