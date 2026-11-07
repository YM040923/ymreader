import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:material_color_utilities/material_color_utilities.dart';

/// 折角新客户端的设计 token。
///
/// 数字来自 Fushi 的 `FushiDesignTokens` / `FushiTypeScale` / `FushiMotion`
/// （GPL-3.0，见仓库根 THIRD_PARTY_NOTICES）。这里只保留折角需要的子集：
/// 间距、圆角、密度、M3 Expressive 字阶、表面阶梯、动效。
abstract final class ZSpace {
  static const double page = 20;
  static const double rowHorizontal = 16;
  static const double rowVertical = 12;
  static const double card = 20;
  static const double gap = 8;
  static const double section = 32;
}

abstract final class ZRadius {
  static const double group = 20;
  static const double card = 12;
  static const double control = 12;
  static const double chip = 6;
  static const double menu = 16;
  static const double dialog = 28;
  static const double sheet = 28;

  static const BorderRadius groupAll = BorderRadius.all(Radius.circular(group));
  static const BorderRadius cardAll = BorderRadius.all(Radius.circular(card));
  static const BorderRadius controlAll =
      BorderRadius.all(Radius.circular(control));
  static const BorderRadius chipAll = BorderRadius.all(Radius.circular(chip));
  static const BorderRadius menuAll = BorderRadius.all(Radius.circular(menu));
  static const BorderRadius dialogAll =
      BorderRadius.all(Radius.circular(dialog));
  static const BorderRadius sheetTop =
      BorderRadius.vertical(top: Radius.circular(sheet));
}

abstract final class ZDensity {
  static const double listMinHeight = 56;
  static const double compactListMinHeight = 44;
  static const double controlHeight = 48;
  static const double compactControlHeight = 36;
}

/// CJK 正文的最小行高倍数。M3 的 1.33 行高按拉丁字母设计，汉字/假名需要更高的行。
const double kCjkMinLineHeight = 1.4;

class ZTypeSpec {
  const ZTypeSpec(
    this.size,
    this.weight,
    this.height, [
    this.letterSpacing,
    this.emphasizedWeight = FontWeight.w500,
  ]);

  final double size;
  final FontWeight weight;
  final double height;
  final double? letterSpacing;
  final FontWeight emphasizedWeight;

  TextStyle applyTo(TextStyle base, {bool emphasized = false}) {
    final weight = emphasized ? emphasizedWeight : this.weight;
    return base.copyWith(
      fontSize: size,
      fontWeight: _platformWeight(weight),
      height: height < kCjkMinLineHeight && size <= 22
          ? kCjkMinLineHeight
          : height,
      letterSpacing: 0,
    );
  }

  static FontWeight _platformWeight(FontWeight weight) {
    if (weight != FontWeight.w500) return weight;
    switch (defaultTargetPlatform) {
      case TargetPlatform.windows:
      case TargetPlatform.linux:
        return FontWeight.w600;
      case TargetPlatform.android:
      case TargetPlatform.iOS:
      case TargetPlatform.macOS:
      case TargetPlatform.fuchsia:
        return weight;
    }
  }
}

/// M3 baseline 字阶 + M3 Expressive 的字重档位。
abstract final class ZType {
  static const displayLarge = ZTypeSpec(57, FontWeight.w400, 64 / 57, -0.25);
  static const displayMedium = ZTypeSpec(45, FontWeight.w400, 52 / 45);
  static const displaySmall = ZTypeSpec(36, FontWeight.w400, 44 / 36);
  static const headlineLarge = ZTypeSpec(32, FontWeight.w400, 40 / 32);
  static const headlineMedium = ZTypeSpec(28, FontWeight.w400, 36 / 28);
  static const headlineSmall = ZTypeSpec(24, FontWeight.w400, 32 / 24);
  static const titleLarge = ZTypeSpec(22, FontWeight.w400, 28 / 22);
  static const titleMedium = ZTypeSpec(
    16,
    FontWeight.w500,
    24 / 16,
    0.15,
    FontWeight.w700,
  );
  static const titleSmall = ZTypeSpec(
    14,
    FontWeight.w500,
    20 / 14,
    0.1,
    FontWeight.w700,
  );
  static const bodyLarge = ZTypeSpec(16, FontWeight.w400, 24 / 16, 0.5);
  static const bodyMedium = ZTypeSpec(14, FontWeight.w400, 20 / 14, 0.25);
  static const bodySmall = ZTypeSpec(12, FontWeight.w400, 16 / 12, 0.4);
  static const labelLarge = ZTypeSpec(
    14,
    FontWeight.w500,
    20 / 14,
    0.1,
    FontWeight.w700,
  );
  static const labelMedium = ZTypeSpec(
    12,
    FontWeight.w500,
    16 / 12,
    0.5,
    FontWeight.w700,
  );
  static const labelSmall = ZTypeSpec(
    11,
    FontWeight.w500,
    16 / 11,
    0.5,
    FontWeight.w700,
  );
}

TextTheme buildZTextTheme({
  required Color color,
  String? fontFamily,
  Locale? locale,
}) {
  final base = TextStyle(
    color: color,
    fontFamily: fontFamily,
    locale: locale,
    textBaseline: TextBaseline.alphabetic,
  );
  TextStyle style(ZTypeSpec spec) => spec.applyTo(base);
  return TextTheme(
    displayLarge: style(ZType.displayLarge),
    displayMedium: style(ZType.displayMedium),
    displaySmall: style(ZType.displaySmall),
    headlineLarge: style(ZType.headlineLarge),
    headlineMedium: style(ZType.headlineMedium),
    headlineSmall: style(ZType.headlineSmall),
    titleLarge: style(ZType.titleLarge),
    titleMedium: style(ZType.titleMedium),
    titleSmall: style(ZType.titleSmall),
    bodyLarge: style(ZType.bodyLarge),
    bodyMedium: style(ZType.bodyMedium),
    bodySmall: style(ZType.bodySmall),
    labelLarge: style(ZType.labelLarge),
    labelMedium: style(ZType.labelMedium),
    labelSmall: style(ZType.labelSmall),
  );
}

/// 表面阶梯：亮色 100 / 99.5 / 96.5 / 93.5 / 90.5 / 87，
/// 深色 3 / 5 / 9.5 / 14 / 19 / 24。
///
/// 只改 tone，色相与彩度沿用原方案。相邻层至少 1.07，页面到卡片 1.16，
/// 避免「页面底、卡片、菜单糊成一片」。
const List<double> _lightSurfaceTones = <double>[
  100,
  99.5,
  96.5,
  93.5,
  90.5,
  87,
];
const List<double> _darkSurfaceTones = <double>[3, 5, 9.5, 14, 19, 24];

ColorScheme applyZSurfaceLadder(ColorScheme scheme) {
  final tones =
      scheme.brightness == Brightness.light ? _lightSurfaceTones : _darkSurfaceTones;
  final anchor = Hct.fromInt(scheme.surfaceContainer.toARGB32());
  Color at(double tone) =>
      Color(Hct.from(anchor.hue, anchor.chroma, tone).toInt());
  return scheme.copyWith(
    surfaceContainerLowest: at(tones[0]),
    surface: at(tones[1]),
    surfaceContainerLow: at(tones[2]),
    surfaceContainer: at(tones[3]),
    surfaceContainerHigh: at(tones[4]),
    surfaceContainerHighest: at(tones[5]),
    surfaceBright: at(scheme.brightness == Brightness.light ? tones[1] : 26),
    surfaceDim: at(scheme.brightness == Brightness.light ? 85 : 4),
  );
}

abstract final class ZMotion {
  static const micro = Duration(milliseconds: 90);
  static const short = Duration(milliseconds: 180);
  static const medium = Duration(milliseconds: 280);
  static const long = Duration(milliseconds: 360);
  static const longReverse = Duration(milliseconds: 240);
  static const staggerStep = Duration(milliseconds: 35);
  static const staggerMaxItems = 8;

  static const enter = Cubic(0.05, 0.7, 0.1, 1.0);
  static const exit = Cubic(0.3, 0.0, 0.8, 0.15);
  static const standard = Cubic(0.2, 0.0, 0.0, 1.0);
  static const release = Cubic(0.18, 1.25, 0.4, 1.0);
}

/// 内容栏：手机 16，平板 20，超过 1200 后居中。
double zPagePadding(double width) {
  if (width < 600) return 16;
  final centered = (width - 1200) / 2;
  return centered <= 0 ? ZSpace.page : centered + ZSpace.page;
}

/// 封面网格列数：手机两列大封面，1200dp 六列，超过 1200 不再加列。
int zCoverColumns(double width) {
  final content = width > 1200 ? 1200.0 : width;
  final columns = ((content - 32) / 180).floor();
  return columns.clamp(2, 8);
}
