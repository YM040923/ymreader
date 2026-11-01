import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Interface language, as offered in Settings.
///
/// The app's copy is Chinese-first: only the shell (navigation, a few settings
/// rows) has English strings, so following the system locale on an English
/// device produced a half-translated interface. Chinese is therefore the
/// default and the other two modes are opt-in.
enum AppLanguage {
  system,
  chinese,
  english;

  /// `null` lets `WidgetsBinding` resolve the platform locale instead.
  Locale? get locale {
    switch (this) {
      case AppLanguage.system:
        return null;
      case AppLanguage.chinese:
        return const Locale('zh');
      case AppLanguage.english:
        return const Locale('en');
    }
  }

  static AppLanguage fromName(String? name) => AppLanguage.values.firstWhere(
        (value) => value.name == name,
        orElse: () => AppLanguage.chinese,
      );
}

const _languageKey = 'app_language';

/// Reads the stored choice before the first frame, so the shell never renders
/// in one language and then switches.
Future<AppLanguage> loadStoredAppLanguage() async {
  final prefs = await SharedPreferences.getInstance();
  return AppLanguage.fromName(prefs.getString(_languageKey));
}

class AppLanguageNotifier extends StateNotifier<AppLanguage> {
  AppLanguageNotifier(super.initial);

  Future<void> select(AppLanguage language) async {
    state = language;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_languageKey, language.name);
  }
}

final appLanguageProvider =
    StateNotifierProvider<AppLanguageNotifier, AppLanguage>(
  (ref) => AppLanguageNotifier(AppLanguage.chinese),
);
