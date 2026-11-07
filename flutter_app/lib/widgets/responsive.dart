import 'package:flutter/material.dart';

/// 平板 / 宽屏适配的统一口径。手机（<420dp）下每个取值都与改造前一致，
/// 改动只在屏宽跨过断点后才生效。

/// 平板横屏的整页栏宽：页头、轮播、继续阅读、网格、历史列表共用同一条边线，
/// 不再出现「页头贴屏幕边、区块各自居中」的错位。
const double kPageMaxWidth = 1200;

/// 页面内容的左右内边距：宽屏时把内容收进 1200 宽的居中栏，
/// 手机（centered <= 0）保持原来的边距。
double pagePadding(double width, {double phone = 16}) {
  // 手机竖屏保持原来的边距（网格 12、页头 16）。
  if (width < 600) return phone;
  // 平板（600dp 起）统一 16，屏宽超过 1200 后再把整页收成居中栏。
  final centered = (width - kPageMaxWidth) / 2;
  return centered <= 0 ? 16 : centered + 16;
}

/// 封面网格列数。首页、搜索、收藏、作品详情目录共用一份，
/// 按目标卡宽 180dp 反推列数：1500dp 平板 → 6 列（单卡约 190dp），
/// 手机 400dp → 2 列，与改造前一致。
int coverColumns(double width) {
  final content = width > kPageMaxWidth ? kPageMaxWidth : width;
  final columns = ((content - 32) / 180).floor();
  return columns.clamp(2, 8);
}

/// 正文列宽：简介这类连续文本。14sp 中文约 45 字/行，再宽就读不动了。
const double kReadingColumnWidth = 640;

/// 列表列宽：设置项、统计卡片这类"一行一条"的内容。
const double kListColumnWidth = 840;

/// 宽屏时把单列内容限宽并居中；屏宽不够时原样铺开，手机上零变化。
class ReadingColumn extends StatelessWidget {
  const ReadingColumn({
    super.key,
    required this.child,
    this.maxWidth = kReadingColumnWidth,
  });

  final Widget child;
  final double maxWidth;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.topCenter,
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxWidth),
        child: child,
      ),
    );
  }
}

/// 整页列表的左右内边距：让内容收成一条居中的列，同时保住 ListView 原有的
/// 滚动与下拉刷新结构——不用把整页重包一层 Center。
/// 手机上 (width - contentWidth) / 2 是负数，取 base，与改造前完全一致。
double centeredColumnPadding(
  double width, {
  required double contentWidth,
  double base = 16,
}) {
  final centered = (width - contentWidth) / 2;
  return centered <= base ? base : centered;
}
