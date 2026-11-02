import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/widgets/reader_settings_panel.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 把面板挂到有界高度上，模拟底部弹窗的约束；size 用逻辑像素
Future<void> _pumpPanel(
  WidgetTester tester, {
  required String? scope,
  ReaderSettings? initial,
  Size size = const Size(390, 844),
}) async {
  tester.view.physicalSize = size * 3;
  tester.view.devicePixelRatio = 3.0;
  addTearDown(tester.view.reset);
  final settings = initial ??
      await ReaderSettings.load(
        scope: scope,
        fallbackScope:
            scope?.startsWith('work:') ?? false ? 'library:cn' : null,
      );
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(
      body: SafeArea(
        child: Align(
          alignment: Alignment.bottomCenter,
          child: SizedBox(
            width: size.width,
            height: size.height,
            child: ReaderSettingsPanel(
              settings: settings,
              scope: scope,
              onChanged: (_) {},
            ),
          ),
        ),
      ),
    ),
  ));
  // 让 _detectScopeMode 的异步探测完成
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('fit mode migration', () {
    test('new installations default to fit width', () async {
      SharedPreferences.setMockInitialValues({});

      final settings = await ReaderSettings.load();

      expect(settings.fitMode, FitMode.width);
      final prefs = await SharedPreferences.getInstance();
      expect(
        prefs.getInt('reader_settings_version'),
        ReaderSettings.settingsVersion,
      );
      expect(prefs.getInt('reader_fitMode'), FitMode.width.index);
    });

    test('old contain default migrates once to fit width', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': 1,
        'reader_fitMode': FitMode.contain.index,
      });

      final settings = await ReaderSettings.load();

      expect(settings.fitMode, FitMode.width);
    });

    test('current explicit contain preference is preserved', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': ReaderSettings.settingsVersion,
        'reader_fitMode': FitMode.contain.index,
      });

      final settings = await ReaderSettings.load();

      expect(settings.fitMode, FitMode.contain);
    });

    test('v4 之后自选的 contain 不会被后续迁移抹掉', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': 4,
        'reader_fitMode': FitMode.contain.index,
      });

      final settings = await ReaderSettings.load();

      expect(settings.fitMode, FitMode.contain,
          reason: '适应显示的迁移只属于 v4，不能跟着版本号一路重跑');
    });
  });

  group('v4 fields', () {
    test('round-trip through global storage', () async {
      SharedPreferences.setMockInitialValues({});

      await const ReaderSettings(
        brightness: 0.6,
        orientationLock: OrientationLock.landscape,
        swapTapZones: true,
        volumeKeyPaging: true,
      ).save();
      final loaded = await ReaderSettings.load();

      expect(loaded.brightness, 0.6);
      expect(loaded.orientationLock, OrientationLock.landscape);
      expect(loaded.swapTapZones, isTrue);
      expect(loaded.volumeKeyPaging, isTrue);
    });

    test('brightness is stored as an integer percent and clamped', () async {
      SharedPreferences.setMockInitialValues({});

      await const ReaderSettings(brightness: 0.1).save();
      final prefs = await SharedPreferences.getInstance();

      expect(prefs.getInt('reader_brightnessPercent'),
          (ReaderSettings.minBrightness * 100).round());
      expect((await ReaderSettings.load()).brightness,
          ReaderSettings.minBrightness);
    });

    test('missing v4 keys fall back to follow-system defaults', () async {
      SharedPreferences.setMockInitialValues({});

      final loaded = await ReaderSettings.load();

      expect(loaded.brightness, 1.0);
      expect(loaded.orientationLock, OrientationLock.followSystem);
      expect(loaded.swapTapZones, isFalse);
      expect(loaded.volumeKeyPaging, isFalse);
    });

    test('scoped writes cover every key so a Work cannot half-inherit',
        () async {
      SharedPreferences.setMockInitialValues({});

      await const ReaderSettings(
        mode: ComicReadingMode.single,
        volumeKeyPaging: true,
        brightness: 0.5,
      ).save(scope: 'work:a');
      await const ReaderSettings(
        mode: ComicReadingMode.webtoon,
        volumeKeyPaging: false,
        brightness: 1.0,
      ).save();

      final scoped = await ReaderSettings.load(scope: 'work:a');

      expect(scoped.mode, ComicReadingMode.single);
      expect(scoped.volumeKeyPaging, isTrue);
      expect(scoped.brightness, 0.5);
    });
  });

  group('连续阅读默认值', () {
    test('没存过的人默认连下一话', () async {
      SharedPreferences.setMockInitialValues({});

      final loaded = await ReaderSettings.load();

      expect(const ReaderSettings().continuousReading, isTrue);
      expect(loaded.continuousReading, isTrue);
    });

    test('明确关掉过的人保持关闭', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': ReaderSettings.settingsVersion,
        'reader_continuousReading': false,
      });

      final loaded = await ReaderSettings.load();

      expect(loaded.continuousReading, isFalse);
    });

    test('作品没存过就沿用库里存着的关闭', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': ReaderSettings.settingsVersion,
        'reader_continuousReading_library:L': false,
      });

      final loaded = await ReaderSettings.load(
          scope: 'work:W', fallbackScope: 'library:L');

      expect(loaded.continuousReading, isFalse);
    });
  });

  group('连续阅读 v5 迁移', () {
    test('旧存档里被默认值写死的关闭，升级时翻开一次并落盘', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': 4,
        'reader_continuousReading': false,
      });

      final loaded = await ReaderSettings.load();
      final prefs = await SharedPreferences.getInstance();

      expect(loaded.continuousReading, isTrue);
      expect(prefs.getBool('reader_continuousReading'), isTrue);
      expect(prefs.getInt('reader_settings_version'),
          ReaderSettings.settingsVersion);
    });

    test('作品级的旧 false 一起翻开', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': 4,
        'reader_continuousReading_work:W': false,
      });

      final loaded = await ReaderSettings.load(scope: 'work:W');
      final prefs = await SharedPreferences.getInstance();

      expect(loaded.continuousReading, isTrue);
      expect(prefs.getBool('reader_continuousReading_work:W'), isTrue);
    });

    test('翻过一次之后再关就不会被反复打开', () async {
      SharedPreferences.setMockInitialValues({
        'reader_settings_version': 4,
        'reader_continuousReading': false,
      });
      await ReaderSettings.load();

      // 迁移已把版本号推到当前值，此后用户的选择是权威。
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool('reader_continuousReading', false);

      expect((await ReaderSettings.load()).continuousReading, isFalse);
    });
  });

  group('panel persistence', () {
    testWidgets('a Work without its own keys writes through to global',
        (tester) async {
      SharedPreferences.setMockInitialValues({});

      await _pumpPanel(tester, scope: 'work:untouched');

      await tester.tap(find.text('单页'));
      await tester.pumpAndSettle();

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getInt('reader_mode'), ComicReadingMode.single.index);
      expect(
        prefs.containsKey('reader_mode_work:untouched'),
        isFalse,
        reason: '本书没有独立设置时不应新建 work 级键',
      );

      final reloaded = await ReaderSettings.load(scope: 'work:untouched');
      expect(reloaded.mode, ComicReadingMode.single);
    });

    testWidgets('toggling 音量键翻页 persists', (tester) async {
      SharedPreferences.setMockInitialValues({});

      await _pumpPanel(tester, scope: 'work:untouched');

      await tester.dragUntilVisible(
        find.text('音量键翻页'),
        find.byType(ListView),
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('音量键翻页'));
      await tester.pumpAndSettle();

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getBool('reader_volumeKeyPaging'), isTrue);
      expect((await ReaderSettings.load()).volumeKeyPaging, isTrue);
    });

    testWidgets('a Work that already has keys keeps edits local',
        (tester) async {
      SharedPreferences.setMockInitialValues({
        'reader_mode_work:book-a': ComicReadingMode.webtoon.index,
        'reader_settings_version': ReaderSettings.settingsVersion,
      });

      await _pumpPanel(tester, scope: 'work:book-a');

      await tester.tap(find.text('单页'));
      await tester.pumpAndSettle();

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getInt('reader_mode_work:book-a'),
          ComicReadingMode.single.index);
      expect(prefs.getInt('reader_mode'), isNull, reason: '仅本书生效时不应污染全局');
    });

    testWidgets('长条模式禁用音量键翻页与点击区对调', (tester) async {
      SharedPreferences.setMockInitialValues({});

      await _pumpPanel(
        tester,
        scope: null,
        initial: const ReaderSettings(mode: ComicReadingMode.webtoon),
      );

      await tester.dragUntilVisible(
        find.text('音量键翻页'),
        find.byType(ListView),
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();

      final row = find.ancestor(
        of: find.text('音量键翻页'),
        matching: find.byType(Row),
      );
      final sw = tester.widget<Switch>(
        find.descendant(of: row, matching: find.byType(Switch)),
      );
      expect(sw.onChanged, isNull, reason: '长条模式下不可用');
      expect(sw.value, isFalse);
    });

    testWidgets('对全部作品生效 clears the Work scope', (tester) async {
      SharedPreferences.setMockInitialValues({
        'reader_mode_work:book-b': ComicReadingMode.webtoon.index,
        'reader_direction_work:book-b': ReadingDirection.ttb.index,
        'reader_settings_version': ReaderSettings.settingsVersion,
      });

      await _pumpPanel(tester, scope: 'work:book-b');
      await tester.dragUntilVisible(
        find.text('对全部作品生效'),
        find.byType(ListView),
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('对全部作品生效'));
      await tester.pumpAndSettle();

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.containsKey('reader_mode_work:book-b'), isFalse);
      expect(prefs.getInt('reader_mode'), ComicReadingMode.webtoon.index,
          reason: '切换瞬间应把当前设置落到全局');
    });

    testWidgets('横屏时改为居中卡片而不是半屏抽屉', (tester) async {
      SharedPreferences.setMockInitialValues({});

      await _pumpPanel(
        tester,
        scope: null,
        size: const Size(844, 390),
        initial: const ReaderSettings(mode: ComicReadingMode.single),
      );

      expect(find.byType(DraggableScrollableSheet), findsNothing);
      expect(tester.takeException(), isNull);

      await tester.tap(find.text('长条'));
      await tester.pumpAndSettle();
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getInt('reader_mode'), ComicReadingMode.webtoon.index);
    });
  });

  test('reader layout is isolated per Work while global options are shared',
      () async {
    SharedPreferences.setMockInitialValues({});

    await const ReaderSettings(
      mode: ComicReadingMode.webtoon,
      direction: ReadingDirection.ttb,
      showPageNumber: false,
      autoPageInterval: 20,
    ).save(scope: 'work:a');
    await const ReaderSettings(
      mode: ComicReadingMode.doublePage,
      direction: ReadingDirection.rtl,
      showPageNumber: false,
      autoPageInterval: 20,
    ).save(scope: 'work:b');

    final a = await ReaderSettings.load(scope: 'work:a');
    final b = await ReaderSettings.load(scope: 'work:b');

    expect(a.mode, ComicReadingMode.webtoon);
    expect(a.direction, ReadingDirection.ttb);
    expect(b.mode, ComicReadingMode.doublePage);
    expect(b.direction, ReadingDirection.rtl);
    expect(a.showPageNumber, isFalse);
    expect(b.autoPageInterval, 20);
  });

  test('library defaults are used only when a Work has no override', () async {
    SharedPreferences.setMockInitialValues({});

    await const ReaderSettings(
      mode: ComicReadingMode.webtoon,
      direction: ReadingDirection.ttb,
      continuousReading: true,
    ).save(scope: 'library:cn');

    final inherited = await ReaderSettings.load(
      scope: 'work:book-without-override',
      fallbackScope: 'library:cn',
    );
    expect(inherited.mode, ComicReadingMode.webtoon);
    expect(inherited.direction, ReadingDirection.ttb);
    expect(inherited.continuousReading, isTrue);

    await const ReaderSettings(
      mode: ComicReadingMode.doublePage,
      direction: ReadingDirection.rtl,
    ).save(scope: 'work:book-with-override');
    final overridden = await ReaderSettings.load(
      scope: 'work:book-with-override',
      fallbackScope: 'library:cn',
    );
    expect(overridden.mode, ComicReadingMode.doublePage);
    expect(overridden.direction, ReadingDirection.rtl);
  });
}
