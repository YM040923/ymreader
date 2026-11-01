import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/providers/locale_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('an untouched install shows Chinese regardless of the system locale',
      () async {
    SharedPreferences.setMockInitialValues({});
    expect(await loadStoredAppLanguage(), AppLanguage.chinese);
    expect(AppLanguage.chinese.locale?.languageCode, 'zh');
  });

  test('following the system hands the locale back to the platform', () {
    expect(AppLanguage.system.locale, isNull);
    expect(AppLanguage.english.locale?.languageCode, 'en');
  });

  test('a picked language persists and is read back on the next start',
      () async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await container.read(appLanguageProvider.notifier).select(AppLanguage.english);

    expect(container.read(appLanguageProvider), AppLanguage.english);
    expect(await loadStoredAppLanguage(), AppLanguage.english);
  });

  test('an unrecognised stored value falls back to Chinese', () async {
    SharedPreferences.setMockInitialValues({'app_language': 'klingon'});
    expect(await loadStoredAppLanguage(), AppLanguage.chinese);
  });
}
