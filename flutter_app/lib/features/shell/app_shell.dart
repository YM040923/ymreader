import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:liquid_glass_widgets/liquid_glass_widgets.dart';
import '../../widgets/phosphor_icons.dart';

import '../../l10n/app_localizations.dart';

/// 悬浮胶囊占用的高度：胶囊 54 + 下留白 10 + 12 呼吸，再让出系统手势区。
double shellBarClearance(BuildContext context) =>
    80 + MediaQuery.paddingOf(context).bottom;

/// 底栏：液态玻璃胶囊，浮在页面内容之上，滚动时折射底下封面。
class AppShell extends StatelessWidget {
  final Widget child;
  const AppShell({super.key, required this.child});

  static int _indexOf(BuildContext context) {
    final location = GoRouterState.of(context).matchedLocation;
    if (location.startsWith('/discover')) return 1;
    if (location.startsWith('/records')) return 2;
    if (location.startsWith('/settings')) return 3;
    return 0;
  }

  void _go(BuildContext context, int index) {
    HapticFeedback.lightImpact();
    switch (index) {
      case 0:
        context.go('/');
      case 1:
        context.go('/discover');
      case 2:
        context.go('/records');
      case 3:
        context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context) {
    final index = _indexOf(context);
    final l10n = AppLocalizations.of(context);
    final isLight = Theme.of(context).brightness == Brightness.light;
    // 底栏玻璃分别做「浅底深字」和「深底浅字」，不再让灰字压在亮封面上。
    final barSelected =
        isLight ? const Color(0xFFB53A2E) : const Color(0xFFFF9A8C);
    final barUnselected =
        isLight ? const Color(0xFF3B3B3B) : const Color(0xFFEDEDED);
    final items = [
      (PhosphorIconsRegular.house, PhosphorIconsFill.house, l10n.navHome),
      (
        PhosphorIconsRegular.compass,
        PhosphorIconsFill.compass,
        l10n.navDiscover
      ),
      (PhosphorIconsRegular.books, PhosphorIconsFill.books, l10n.navRecords),
      (PhosphorIconsRegular.user, PhosphorIconsFill.user, l10n.navMine),
    ];

    return LiquidGlassScope(
      child: Stack(
        children: [
          Positioned.fill(child: GlassBackgroundSource(child: child)),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: SafeArea(
              top: false,
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 560),
                  child: GlassTabBar.bottom(
                    selectedIndex: index,
                    onTabSelected: (i) => _go(context, i),
                    tabs: [
                      for (final item in items)
                        GlassTab(
                          label: item.$3,
                          icon: Icon(item.$1),
                          activeIcon: Icon(item.$2),
                          glowColor: barSelected.withValues(alpha: 0.32),
                        ),
                    ],
                    horizontalPadding: 16,
                    verticalPadding: 8,
                    barHeight: 58,
                    iconSize: 22,
                    iconLabelSpacing: 3,
                    labelFontSize: 11.5,
                    textStyle: const TextStyle(
                      fontSize: 11.5,
                      height: 1.0,
                      letterSpacing: 0.1,
                      fontWeight: FontWeight.w600,
                      decoration: TextDecoration.none,
                    ),
                    selectedIconColor: barSelected,
                    unselectedIconColor: barUnselected,
                    selectedLabelColor: barSelected,
                    unselectedLabelColor: barUnselected,
                    selectedLabelStyle: const TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                    ),
                    unselectedLabelStyle: const TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w600,
                    ),
                    indicatorColor:
                        barSelected.withValues(alpha: isLight ? 0.14 : 0.20),
                    // 默认胶囊的折射率 1.59、厚度 30 太像一颗厚玻璃珠；
                    // 这里换成接近空气的薄透镜，只留一点弯折和边缘高光。
                    settings: LiquidGlassSettings(
                      thickness: 12,
                      blur: 7,
                      chromaticAberration: 0.02,
                      lightIntensity: 0.38,
                      refractiveIndex: 1.22,
                      saturation: 1.15,
                      glassColor: isLight
                          ? const Color(0xCCFFFFFF)
                          : const Color(0xB3000000),
                    ),
                    maskingQuality: MaskingQuality.high,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
