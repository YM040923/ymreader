import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// 墨 + 朱砂：封面承担颜色，界面只做纸、墨与一笔朱砂。
class AppTheme {
  AppTheme._();

  static const ink = Color(0xFF161616);
  static const sub = Color(0xFF6A6A6A);
  static const tert = Color(0xFF9A9A9A);
  static const accent = Color(0xFFD94B3D);
  static const canvas = Color(0xFFFFFFFF);
  static const surface = Color(0xFFF5F5F5);
  static const hairline = Color(0xFFE8E8E8);

  static const darkInk = Color(0xFFF2F2F2);
  static const darkSub = Color(0xFFA3A3A3);
  static const darkTert = Color(0xFF6F6F6F);
  static const darkAccent = Color(0xFFFF7A6B);
  static const darkCanvas = Color(0xFF0E0E10);
  static const darkSurface = Color(0xFF17171A);
  static const darkHairline = Color(0xFF2A2A2E);

  // 只有三档圆角：封面/标签 4，按钮/输入/行内块 8，面板/弹窗 16。
  static const double radiusXs = 4;
  static const double radiusSm = 8;
  static const double radiusMd = 8;
  static const double radiusLg = 16;
  static const double radiusXl = 16;

  static const _lightScheme = ColorScheme(
    brightness: Brightness.light,
    primary: accent,
    onPrimary: Colors.white,
    primaryContainer: Color(0xFFFBEAE7),
    onPrimaryContainer: ink,
    secondary: sub,
    onSecondary: Colors.white,
    secondaryContainer: surface,
    onSecondaryContainer: ink,
    tertiary: tert,
    onTertiary: Colors.white,
    error: Color(0xFFB3261E),
    onError: Colors.white,
    surface: canvas,
    onSurface: ink,
    onSurfaceVariant: sub,
    outline: hairline,
    outlineVariant: Color(0xFFF0F0F0),
    shadow: Colors.black,
    scrim: Colors.black,
    inverseSurface: ink,
    onInverseSurface: Colors.white,
    inversePrimary: darkAccent,
    surfaceTint: Colors.transparent,
    surfaceContainerHighest: surface,
  );

  static const _darkScheme = ColorScheme(
    brightness: Brightness.dark,
    primary: darkAccent,
    onPrimary: Color(0xFF111111),
    primaryContainer: Color(0xFF3A1E1A),
    onPrimaryContainer: darkInk,
    secondary: darkSub,
    onSecondary: Color(0xFF111111),
    secondaryContainer: darkSurface,
    onSecondaryContainer: darkInk,
    tertiary: darkTert,
    onTertiary: Color(0xFF111111),
    error: Color(0xFFFFB4AB),
    onError: Color(0xFF690005),
    surface: darkCanvas,
    onSurface: darkInk,
    onSurfaceVariant: darkSub,
    outline: darkHairline,
    outlineVariant: Color(0xFF242428),
    shadow: Colors.black,
    scrim: Colors.black,
    inverseSurface: darkInk,
    onInverseSurface: darkCanvas,
    inversePrimary: accent,
    surfaceTint: Colors.transparent,
    surfaceContainerHighest: darkSurface,
  );

  static final lightTheme = _build(_lightScheme, Brightness.light);
  static final darkTheme = _build(_darkScheme, Brightness.dark);

  static ThemeData _build(ColorScheme scheme, Brightness brightness) {
    final isLight = brightness == Brightness.light;
    final bg = isLight ? canvas : darkCanvas;
    final fg = isLight ? ink : darkInk;
    final quiet = isLight ? sub : darkSub;
    final faint = isLight ? tert : darkTert;
    final line = isLight ? hairline : darkHairline;
    final fill = isLight ? surface : darkSurface;

    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      brightness: brightness,
      scaffoldBackgroundColor: bg,
      canvasColor: bg,
      splashFactory: NoSplash.splashFactory,
      splashColor: Colors.transparent,
      // 不要 Material 涟漪，但按压必须有一层很薄的墨色反馈。
      highlightColor: fg.withValues(alpha: isLight ? 0.045 : 0.06),
      hoverColor: fg.withValues(alpha: isLight ? 0.03 : 0.04),
      textTheme: _textTheme(isLight ? ink : darkInk, quiet, faint),
      iconTheme: IconThemeData(color: fg, size: 22),
      appBarTheme: AppBarTheme(
        centerTitle: false,
        elevation: 0,
        scrolledUnderElevation: 0,
        toolbarHeight: 52,
        titleSpacing: 16,
        backgroundColor: bg,
        surfaceTintColor: Colors.transparent,
        systemOverlayStyle:
            isLight ? SystemUiOverlayStyle.dark : SystemUiOverlayStyle.light,
        titleTextStyle: TextStyle(
          fontSize: 17,
          fontWeight: FontWeight.w600,
          color: fg,
        ),
        iconTheme: IconThemeData(color: fg, size: 22),
      ),
      cardTheme: CardThemeData(
        elevation: 0,
        color: fill,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        clipBehavior: Clip.antiAlias,
        margin: EdgeInsets.zero,
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: fill,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide.none,
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          borderSide: BorderSide(color: fg, width: 1),
        ),
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        hintStyle: TextStyle(color: faint, fontSize: 14),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size(64, 48),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
          textStyle: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
          elevation: 0,
          padding: const EdgeInsets.symmetric(horizontal: 18),
          disabledBackgroundColor: fill,
          disabledForegroundColor: faint,
        ).copyWith(
          overlayColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.pressed)) {
              return (isLight ? Colors.white : Colors.black)
                  .withValues(alpha: 0.12);
            }
            if (states.contains(WidgetState.hovered) ||
                states.contains(WidgetState.focused)) {
              return (isLight ? Colors.white : Colors.black)
                  .withValues(alpha: 0.06);
            }
            return Colors.transparent;
          }),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(64, 44),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radiusSm),
          ),
          side: BorderSide(color: line),
          textStyle: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
          ),
          padding: const EdgeInsets.symmetric(horizontal: 16),
          disabledForegroundColor: faint,
        ).copyWith(
          overlayColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.pressed)) {
              return fg.withValues(alpha: 0.07);
            }
            if (states.contains(WidgetState.hovered)) {
              return fg.withValues(alpha: 0.04);
            }
            return Colors.transparent;
          }),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: fg,
          textStyle: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
          ),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          minimumSize: const Size(0, 40),
          disabledForegroundColor: faint,
        ).copyWith(
          overlayColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.pressed)) {
              return fg.withValues(alpha: 0.07);
            }
            if (states.contains(WidgetState.hovered)) {
              return fg.withValues(alpha: 0.04);
            }
            return Colors.transparent;
          }),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: ButtonStyle(
          iconSize: const WidgetStatePropertyAll(22),
          foregroundColor: WidgetStatePropertyAll(fg),
          overlayColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.pressed)) {
              return fg.withValues(alpha: 0.08);
            }
            if (states.contains(WidgetState.hovered)) {
              return fg.withValues(alpha: 0.04);
            }
            return Colors.transparent;
          }),
        ),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (!states.contains(WidgetState.selected)) {
            return isLight ? Colors.white : darkSub;
          }
          return Colors.white;
        }),
        trackColor: WidgetStateProperty.resolveWith((states) {
          if (!states.contains(WidgetState.selected)) return line;
          return isLight ? accent : darkAccent;
        }),
        trackOutlineColor: const WidgetStatePropertyAll(Colors.transparent),
        overlayColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.pressed)) {
            return scheme.primary.withValues(alpha: 0.10);
          }
          return Colors.transparent;
        }),
      ),
      segmentedButtonTheme: SegmentedButtonThemeData(
        style: ButtonStyle(
          minimumSize: const WidgetStatePropertyAll(Size(0, 32)),
          padding: const WidgetStatePropertyAll(
            EdgeInsets.symmetric(horizontal: 12),
          ),
          visualDensity: VisualDensity.compact,
          backgroundColor: const WidgetStatePropertyAll(Colors.transparent),
          foregroundColor: WidgetStateProperty.resolveWith((states) {
            return states.contains(WidgetState.selected) ? fg : quiet;
          }),
          side: WidgetStateProperty.resolveWith((states) {
            final selected = states.contains(WidgetState.selected);
            return BorderSide(
              color: selected ? (isLight ? accent : darkAccent) : line,
              width: 1,
            );
          }),
          shape: WidgetStatePropertyAll(
            RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(radiusSm),
            ),
          ),
          textStyle: const WidgetStatePropertyAll(
            TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
          ),
          overlayColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.pressed)) {
              return fg.withValues(alpha: 0.06);
            }
            return Colors.transparent;
          }),
        ),
      ),
      checkboxTheme: CheckboxThemeData(
        fillColor: WidgetStateProperty.resolveWith((states) {
          return states.contains(WidgetState.selected)
              ? scheme.primary
              : Colors.transparent;
        }),
        checkColor:
            WidgetStatePropertyAll(isLight ? Colors.white : Colors.black),
        side: BorderSide(color: line, width: 1),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusXs),
        ),
        visualDensity: VisualDensity.compact,
      ),
      radioTheme: RadioThemeData(
        fillColor: WidgetStateProperty.resolveWith((states) {
          return states.contains(WidgetState.selected) ? scheme.primary : quiet;
        }),
        overlayColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.pressed)) {
            return scheme.primary.withValues(alpha: 0.10);
          }
          return Colors.transparent;
        }),
      ),
      sliderTheme: SliderThemeData(
        trackHeight: 2,
        activeTrackColor: scheme.primary,
        inactiveTrackColor: line,
        thumbColor: scheme.primary,
        thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 6),
        overlayShape: const RoundSliderOverlayShape(overlayRadius: 16),
        overlayColor: scheme.primary.withValues(alpha: 0.12),
        showValueIndicator: ShowValueIndicator.never,
      ),
      tooltipTheme: TooltipThemeData(
        waitDuration: const Duration(milliseconds: 450),
        decoration: BoxDecoration(
          color: fg,
          borderRadius: BorderRadius.circular(radiusXs),
        ),
        textStyle: TextStyle(fontSize: 12, color: bg),
      ),
      chipTheme: ChipThemeData(
        backgroundColor: fill,
        selectedColor: fg,
        disabledColor: fill,
        labelStyle: TextStyle(fontSize: 11, color: quiet),
        secondaryLabelStyle: TextStyle(fontSize: 11, color: bg),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusXs),
        ),
        side: BorderSide.none,
        elevation: 0,
        pressElevation: 0,
        showCheckmark: false,
        padding: const EdgeInsets.symmetric(horizontal: 8),
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: bg,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        insetPadding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
        actionsPadding: const EdgeInsets.fromLTRB(8, 0, 8, 8),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusLg),
        ),
        titleTextStyle: TextStyle(
          fontSize: 17,
          fontWeight: FontWeight.w600,
          color: fg,
        ),
        contentTextStyle: TextStyle(
          fontSize: 14,
          height: 1.55,
          color: quiet,
        ),
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: bg,
        surfaceTintColor: Colors.transparent,
        showDragHandle: false,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
        ),
      ),
      dividerTheme: DividerThemeData(
        color: line,
        thickness: 0.5,
        space: 0.5,
      ),
      listTileTheme: ListTileThemeData(
        contentPadding: const EdgeInsets.symmetric(horizontal: 16),
        minVerticalPadding: 8,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
        titleTextStyle: TextStyle(
          fontSize: 14,
          fontWeight: FontWeight.w500,
          color: fg,
        ),
        subtitleTextStyle: TextStyle(fontSize: 11, color: faint),
        selectedColor: fg,
        selectedTileColor: Colors.transparent,
      ),
      popupMenuTheme: PopupMenuThemeData(
        color: bg,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        menuPadding: const EdgeInsets.symmetric(vertical: 4),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
          side: BorderSide(color: line),
        ),
        textStyle: TextStyle(fontSize: 14, color: fg),
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: fg,
        contentTextStyle: TextStyle(fontSize: 13, color: bg),
        actionTextColor: isLight ? darkAccent : accent,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(radiusSm),
        ),
      ),
      progressIndicatorTheme: ProgressIndicatorThemeData(
        color: scheme.primary,
        linearTrackColor: isLight ? hairline : darkHairline,
        circularTrackColor: Colors.transparent,
      ),
      pageTransitionsTheme: const PageTransitionsTheme(
        builders: {
          TargetPlatform.android: CupertinoPageTransitionsBuilder(),
          TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
          TargetPlatform.windows: CupertinoPageTransitionsBuilder(),
          TargetPlatform.macOS: CupertinoPageTransitionsBuilder(),
          TargetPlatform.linux: CupertinoPageTransitionsBuilder(),
        },
      ),
    );
  }

  static TextTheme _textTheme(Color fg, Color quiet, Color faint) {
    return TextTheme(
      displayLarge: TextStyle(
        fontSize: 22,
        fontWeight: FontWeight.w700,
        color: fg,
        height: 1.25,
      ),
      displayMedium: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.w700,
        color: fg,
        height: 1.3,
      ),
      displaySmall: TextStyle(
        fontSize: 17,
        fontWeight: FontWeight.w700,
        color: fg,
        height: 1.3,
      ),
      headlineLarge: TextStyle(
        fontSize: 17,
        fontWeight: FontWeight.w700,
        color: fg,
        height: 1.25,
      ),
      headlineMedium: TextStyle(
        fontSize: 17,
        fontWeight: FontWeight.w600,
        color: fg,
        height: 1.3,
      ),
      headlineSmall: TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w600,
        color: fg,
        height: 1.35,
      ),
      titleLarge: TextStyle(
        fontSize: 17,
        fontWeight: FontWeight.w600,
        color: fg,
        height: 1.3,
      ),
      titleMedium: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: fg,
        height: 1.4,
      ),
      titleSmall: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w600,
        color: fg,
        height: 1.4,
      ),
      bodyLarge: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w400,
        color: fg,
        height: 1.5,
      ),
      bodyMedium: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w400,
        color: fg,
        height: 1.5,
      ),
      bodySmall: TextStyle(
        fontSize: 11,
        fontWeight: FontWeight.w400,
        color: quiet,
        height: 1.4,
      ),
      labelLarge: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: fg,
      ),
      labelMedium: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w500,
        color: quiet,
      ),
      labelSmall: TextStyle(
        fontSize: 10,
        fontWeight: FontWeight.w500,
        color: faint,
      ),
    );
  }
}
