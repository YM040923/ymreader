import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/widgets/reader_settings_panel.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('ReaderSettings fit mode migration', () {
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
