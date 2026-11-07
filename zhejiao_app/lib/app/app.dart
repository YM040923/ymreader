import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../l10n/app_localizations.dart';
import '../data/providers/locale_provider.dart';
import '../data/providers/server_route_provider.dart';
import '../data/providers/server_branding_provider.dart';
import 'router.dart';
import '../design/theme.dart';

class NowenReaderApp extends ConsumerStatefulWidget {
  const NowenReaderApp({super.key});

  @override
  ConsumerState<NowenReaderApp> createState() => _NowenReaderAppState();
}

class _NowenReaderAppState extends ConsumerState<NowenReaderApp>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    ref
        .read(serverRouteProvider.notifier)
        .setForeground(state == AppLifecycleState.resumed);
    if (state == AppLifecycleState.resumed) {
      ref.read(serverBrandingProvider.notifier).refresh();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final router = ref.watch(routerProvider);
    final branding = ref.watch(currentServerBrandingProvider);
    final language = ref.watch(appLanguageProvider);

    return MaterialApp.router(
      title: branding.name,
      debugShowCheckedModeBanner: false,
      theme: buildReferenceTheme(brightness: Brightness.light),
      darkTheme: buildReferenceTheme(brightness: Brightness.dark),
      themeMode: ThemeMode.system,
      routerConfig: router,
      // 国际化支持
      locale: language.locale,
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: AppLocalizations.supportedLocales,
    );
  }
}
