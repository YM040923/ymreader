import 'package:flutter/cupertino.dart' show CupertinoPageTransitionsBuilder;
import 'package:flutter/material.dart';

import 'tokens.dart';

/// Fushi 的默认 M3E 变体：饱和度拉满，但色相不跑偏。
const _schemeVariant = DynamicSchemeVariant.vibrant;

/// 折角新客户端的主题工厂。
///
/// - 默认种子是 M3 baseline 紫（Fushi 默认）；
/// - Android 有系统取色时直接吃系统色，桌面上退回种子；
/// - 所有出口都过一遍表面阶梯；
/// - 圆角、字阶、控件高度、动效全部走 [ZSpace] / [ZRadius] / [ZType] / [ZMotion]。
ThemeData buildZhejiaoTheme({
  required Brightness brightness,
  ColorScheme? scheme,
  String? fontFamily,
  Locale? locale,
  bool applyLadder = true,
}) {
  final base = scheme ??
      ColorScheme.fromSeed(
        seedColor: const Color(0xFF6750A4),
        brightness: brightness,
        dynamicSchemeVariant: _schemeVariant,
      );
  final colors = applyLadder ? applyZSurfaceLadder(base) : base;
  final text = buildZTextTheme(
    color: colors.onSurface,
    fontFamily: fontFamily,
    locale: locale,
  );
  final isLight = brightness == Brightness.light;

  final controlShape = RoundedRectangleBorder(
    borderRadius: ZRadius.controlAll,
  );

  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    colorScheme: colors,
    fontFamily: fontFamily,
    scaffoldBackgroundColor: colors.surface,
    canvasColor: colors.surface,
    textTheme: text,
    visualDensity: VisualDensity.standard,
    splashFactory: InkRipple.splashFactory,
    appBarTheme: AppBarTheme(
      backgroundColor: colors.surface,
      foregroundColor: colors.onSurface,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      toolbarHeight: 52,
      titleTextStyle: text.titleLarge?.copyWith(
        fontWeight: FontWeight.w600,
        fontSize: 17,
      ),
    ),
    cardTheme: CardThemeData(
      color: colors.surfaceContainerLow,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(borderRadius: ZRadius.groupAll),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size(0, ZDensity.controlHeight),
        padding: const EdgeInsets.symmetric(horizontal: 20),
        elevation: 0,
        shape: controlShape,
        textStyle: text.labelLarge?.copyWith(fontWeight: FontWeight.w600),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(0, ZDensity.controlHeight),
        padding: const EdgeInsets.symmetric(horizontal: 20),
        shape: controlShape,
        side: BorderSide(color: colors.outlineVariant),
        textStyle: text.labelLarge?.copyWith(fontWeight: FontWeight.w600),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        minimumSize: const Size(0, ZDensity.compactControlHeight),
        padding: const EdgeInsets.symmetric(horizontal: 12),
        shape: controlShape,
        textStyle: text.labelLarge?.copyWith(fontWeight: FontWeight.w600),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: colors.surfaceContainerHighest,
      contentPadding:
          const EdgeInsets.symmetric(horizontal: ZSpace.rowHorizontal, vertical: 14),
      border: OutlineInputBorder(
        borderRadius: ZRadius.controlAll,
        borderSide: BorderSide.none,
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: ZRadius.controlAll,
        borderSide: BorderSide.none,
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: ZRadius.controlAll,
        borderSide: BorderSide(color: colors.onSurface, width: 1),
      ),
      hintStyle: text.bodyMedium?.copyWith(color: colors.onSurfaceVariant),
    ),
    chipTheme: ChipThemeData(
      backgroundColor: colors.surfaceContainerHigh,
      selectedColor: colors.secondaryContainer,
      surfaceTintColor: Colors.transparent,
      side: BorderSide.none,
      shape: RoundedRectangleBorder(borderRadius: ZRadius.chipAll),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      labelStyle: text.labelMedium,
      secondaryLabelStyle: text.labelMedium?.copyWith(
        color: colors.onSecondaryContainer,
      ),
    ),
    dividerTheme: DividerThemeData(
      color: colors.outlineVariant.withValues(alpha: 0.6),
      thickness: 1,
      space: 1,
    ),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: colors.inverseSurface,
      contentTextStyle: text.bodyMedium?.copyWith(
        color: colors.onInverseSurface,
        fontWeight: FontWeight.w500,
      ),
      actionTextColor: colors.inversePrimary,
      elevation: 3,
      insetPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(24)),
      ),
    ),
    tooltipTheme: TooltipThemeData(
      waitDuration: const Duration(milliseconds: 400),
      exitDuration: const Duration(milliseconds: 100),
      constraints: const BoxConstraints(minHeight: 24),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: colors.inverseSurface,
        borderRadius: const BorderRadius.all(Radius.circular(4)),
      ),
      textStyle: text.bodySmall?.copyWith(
        color: colors.onInverseSurface,
        fontWeight: FontWeight.w500,
      ),
    ),
    scrollbarTheme: ScrollbarThemeData(
      thickness: const WidgetStatePropertyAll(3),
      radius: const Radius.circular(3),
      thumbColor: WidgetStateProperty.resolveWith((states) {
        if (states.contains(WidgetState.dragged)) {
          return colors.onSurfaceVariant.withValues(alpha: 0.8);
        }
        if (states.contains(WidgetState.hovered)) {
          return colors.onSurfaceVariant.withValues(alpha: 0.6);
        }
        return colors.onSurfaceVariant.withValues(alpha: 0.38);
      }),
    ),
    pageTransitionsTheme: const PageTransitionsTheme(
      builders: <TargetPlatform, PageTransitionsBuilder>{
        TargetPlatform.android: ZoomPageTransitionsBuilder(),
        TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
        TargetPlatform.macOS: CupertinoPageTransitionsBuilder(),
        TargetPlatform.windows: FadeForwardsPageTransitionsBuilder(),
        TargetPlatform.linux: FadeForwardsPageTransitionsBuilder(),
      },
    ),
    textSelectionTheme: TextSelectionThemeData(
      cursorColor: colors.primary,
      selectionColor: colors.primary.withValues(alpha: 0.24),
      selectionHandleColor: colors.primary,
    ),
    extensions: <ThemeExtension<dynamic>>[
      ZGlassColors(
        background: colors.surfaceContainer.withValues(alpha: isLight ? 0.78 : 0.72),
        border: colors.outlineVariant.withValues(alpha: isLight ? 0.55 : 0.35),
      ),
    ],
  );
}

/// 玻璃/chrome 的额外颜色。真正模糊由 liquid_glass_widgets 在设备上完成，
/// 这里只提供 token，预览与无玻璃环境用半透明面板兜底。
class ZGlassColors extends ThemeExtension<ZGlassColors> {
  const ZGlassColors({required this.background, required this.border});

  final Color background;
  final Color border;

  @override
  ZGlassColors copyWith({Color? background, Color? border}) => ZGlassColors(
        background: background ?? this.background,
        border: border ?? this.border,
      );

  @override
  ZGlassColors lerp(ThemeExtension<ZGlassColors>? other, double t) {
    if (other is! ZGlassColors) return this;
    return ZGlassColors(
      background: Color.lerp(background, other.background, t)!,
      border: Color.lerp(border, other.border, t)!,
    );
  }
}

/// Fushi 参考截图（docs/design/2026-10-ui-motion-redesign）里实际使用的
/// 蓝青色主题。M0 照这套颜色复刻，后续再接入 seed/系统取色。
const referenceLightScheme = ColorScheme(
  brightness: Brightness.light,
  primary: Color(0xFF874F42),
  onPrimary: Color(0xFFFFFFFF),
  primaryContainer: Color(0xFFF6D9D3),
  onPrimaryContainer: Color(0xFF3A1713),
  secondary: Color(0xFF7A5B57),
  onSecondary: Color(0xFFFFFFFF),
  secondaryContainer: Color(0xFFC8E8F7),
  onSecondaryContainer: Color(0xFF103A45),
  tertiary: Color(0xFF8A6A2E),
  onTertiary: Color(0xFFFFFFFF),
  tertiaryContainer: Color(0xFFF7E2AD),
  onTertiaryContainer: Color(0xFF3A2A00),
  error: Color(0xFFBA1A1A),
  onError: Color(0xFFFFFFFF),
  surface: Color(0xFFFAF5F2),
  onSurface: Color(0xFF2B1A18),
  onSurfaceVariant: Color(0xFF7A5B57),
  outline: Color(0xFF9A7A74),
  outlineVariant: Color(0xFFE9D3CE),
  shadow: Color(0xFF000000),
  scrim: Color(0xFF000000),
  inverseSurface: Color(0xFF3A2E2B),
  onInverseSurface: Color(0xFFF8EEEB),
  inversePrimary: Color(0xFFFFB4A8),
  surfaceTint: Color(0x00000000),
  surfaceContainerLowest: Color(0xFFFFFFFF),
  surfaceContainerLow: Color(0xFFFEF3F1),
  surfaceContainer: Color(0xFFFBE9E5),
  surfaceContainerHigh: Color(0xFFF7E1DC),
  surfaceContainerHighest: Color(0xFFF1D6D0),
  surfaceDim: Color(0xFFE8D6D1),
  surfaceBright: Color(0xFFFFFFFF),
);

const referenceDarkScheme = ColorScheme(
  brightness: Brightness.dark,
  primary: Color(0xFFFFB4A8),
  onPrimary: Color(0xFF562016),
  primaryContainer: Color(0xFF73342A),
  onPrimaryContainer: Color(0xFFFFDAD3),
  secondary: Color(0xFFB4CBD6),
  onSecondary: Color(0xFF1F333C),
  secondaryContainer: Color(0xFF0F4E5E),
  onSecondaryContainer: Color(0xFFC8E8F7),
  tertiary: Color(0xFFE2C47A),
  onTertiary: Color(0xFF3A2A00),
  tertiaryContainer: Color(0xFF56451A),
  onTertiaryContainer: Color(0xFFF7E2AD),
  error: Color(0xFFFFB4AB),
  onError: Color(0xFF690005),
  surface: Color(0xFF1A1110),
  onSurface: Color(0xFFF2DED9),
  onSurfaceVariant: Color(0xFFD6BDB7),
  outline: Color(0xFF9F8580),
  outlineVariant: Color(0xFF4F3A36),
  shadow: Color(0xFF000000),
  scrim: Color(0xFF000000),
  inverseSurface: Color(0xFFF2DED9),
  onInverseSurface: Color(0xFF3A2E2B),
  inversePrimary: Color(0xFF874F42),
  surfaceTint: Color(0x00000000),
  surfaceContainerLowest: Color(0xFF140C0B),
  surfaceContainerLow: Color(0xFF221716),
  surfaceContainer: Color(0xFF2A1D1B),
  surfaceContainerHigh: Color(0xFF352624),
  surfaceContainerHighest: Color(0xFF42302D),
  surfaceDim: Color(0xFF1A1110),
  surfaceBright: Color(0xFF4A3632),
);

/// 复刻参考截图用的入口。
ThemeData buildReferenceTheme({
  required Brightness brightness,
  String? fontFamily,
}) =>
    buildZhejiaoTheme(
      brightness: brightness,
      scheme: brightness == Brightness.light
          ? referenceLightScheme
          : referenceDarkScheme,
      fontFamily: fontFamily,
      locale: const Locale('zh', 'CN'),
      applyLadder: false,
    );

extension ZThemeX on BuildContext {
  ZGlassColors get zGlass =>
      Theme.of(this).extension<ZGlassColors>() ??
      const ZGlassColors(background: Colors.white70, border: Colors.white24);
}
