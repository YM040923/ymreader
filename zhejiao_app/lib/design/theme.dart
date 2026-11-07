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
}) {
  final base = scheme ??
      ColorScheme.fromSeed(
        seedColor: const Color(0xFF6750A4),
        brightness: brightness,
        dynamicSchemeVariant: _schemeVariant,
      );
  final colors = applyZSurfaceLadder(base);
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

extension ZThemeX on BuildContext {
  ZGlassColors get zGlass =>
      Theme.of(this).extension<ZGlassColors>() ??
      const ZGlassColors(background: Colors.white70, border: Colors.white24);
}
