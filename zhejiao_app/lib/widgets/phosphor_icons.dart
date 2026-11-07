import 'package:flutter/widgets.dart';

/// Phosphor Icons — https://phosphoricons.com (MIT).
///
/// 官方 Regular / Fill 字体随包内置，这里只保留用到的字形常量。
/// 不依赖 phosphor_flutter 包：该包的 IconData 子类在 Flutter 新版
/// （IconData 变成 final class）下无法编译。
class PhosphorIconsRegular {
  const PhosphorIconsRegular._();

  static const house = IconData(
    0xe2c2,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const compass = IconData(
    0xe1c8,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const books = IconData(
    0xe758,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const user = IconData(
    0xe4c2,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const magnifyingGlass = IconData(
    0xe30c,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const folderSimple = IconData(
    0xe25a,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const dotsThree = IconData(
    0xe1fe,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const pencilSimple = IconData(
    0xe3b4,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const slidersHorizontal = IconData(
    0xe434,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const check = IconData(
    0xe182,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
  static const arrowRight = IconData(
    0xe06c,
    fontFamily: 'Phosphor',
    matchTextDirection: true,
  );
}

class PhosphorIconsFill {
  const PhosphorIconsFill._();

  static const house = IconData(
    0xe2c2,
    fontFamily: 'PhosphorFill',
    matchTextDirection: true,
  );
  static const compass = IconData(
    0xe1c8,
    fontFamily: 'PhosphorFill',
    matchTextDirection: true,
  );
  static const books = IconData(
    0xe758,
    fontFamily: 'PhosphorFill',
    matchTextDirection: true,
  );
  static const user = IconData(
    0xe4c2,
    fontFamily: 'PhosphorFill',
    matchTextDirection: true,
  );
  static const folderSimple = IconData(
    0xe25a,
    fontFamily: 'PhosphorFill',
    matchTextDirection: true,
  );
}
