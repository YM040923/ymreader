import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show FontLoader;
import 'package:flutter_test/flutter_test.dart';
import 'package:zhejiao_app/design/theme.dart';
import 'package:zhejiao_app/preview/preview_page.dart';

/// 用真实主题工厂和真实页面组件离屏渲染，不打开窗口。
///
/// 运行：flutter test --update-goldens test/design_preview_test.dart
/// 产物：test/goldens/*.png
void main() {
  setUpAll(() async {
    final file = File(r'C:\Windows\Fonts\NotoSansSC-VF.ttf');
    if (file.existsSync()) {
      final bytes = file.readAsBytesSync();
      final loader = FontLoader('Noto Sans SC')
        ..addFont(Future.value(ByteData.sublistView(bytes)));
      await loader.load();
    }
    final icons = File(
      r'C:\src\flutter\bin\cache\artifacts\material_fonts\materialicons-regular.otf',
    );
    if (icons.existsSync()) {
      final bytes = icons.readAsBytesSync();
      final loader = FontLoader('MaterialIcons')
        ..addFont(Future.value(ByteData.sublistView(bytes)));
      await loader.load();
    }
  });

  Widget app(Brightness brightness) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      locale: const Locale('zh', 'CN'),
      theme: buildZhejiaoTheme(
        brightness: brightness,
        fontFamily: 'Noto Sans SC',
        locale: const Locale('zh', 'CN'),
      ),
      home: PreviewPage(onToggleTheme: () {}),
    );
  }

  Future<void> capture(
    WidgetTester tester, {
    required Size size,
    required Brightness brightness,
    required String name,
  }) async {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(app(brightness));
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(PreviewPage),
      matchesGoldenFile('goldens/$name.png'),
    );
  }

  testWidgets('desktop light', (tester) async {
    await capture(
      tester,
      size: const Size(1440, 900),
      brightness: Brightness.light,
      name: 'preview_light_1440',
    );
  });

  testWidgets('desktop dark', (tester) async {
    await capture(
      tester,
      size: const Size(1440, 900),
      brightness: Brightness.dark,
      name: 'preview_dark_1440',
    );
  });

  testWidgets('phone light', (tester) async {
    await capture(
      tester,
      size: const Size(400, 900),
      brightness: Brightness.light,
      name: 'preview_light_400',
    );
  });

  testWidgets('phone dark', (tester) async {
    await capture(
      tester,
      size: const Size(400, 900),
      brightness: Brightness.dark,
      name: 'preview_dark_400',
    );
  });
}
