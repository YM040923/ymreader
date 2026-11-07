import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/widgets/responsive.dart';

/// 平板适配的守门测试：手机档必须逐字节等于改造前的取值，
/// 宽屏档才允许收窄和加列。
void main() {
  group('coverColumns', () {
    test('手机屏保持两列大封面', () {
      expect(coverColumns(360), 2);
      expect(coverColumns(392), 2);
      expect(coverColumns(419), 2);
    });

    test('跨过断点才加列', () {
      expect(coverColumns(420), 2);
      expect(coverColumns(649), 3);
      expect(coverColumns(650), 3);
      expect(coverColumns(899), 4);
      expect(coverColumns(900), 4);
      expect(coverColumns(1097), 5);
      expect(coverColumns(1112), 6);
      expect(coverColumns(1200), 6);
      // 页面栏宽封顶 1200，再宽也不加列。
      expect(coverColumns(1600), 6);
    });
  });

  group('centeredColumnPadding', () {
    test('屏宽容不下内容列时退回 base，手机上零变化', () {
      expect(centeredColumnPadding(360, contentWidth: kListColumnWidth), 16);
      expect(centeredColumnPadding(839, contentWidth: kListColumnWidth), 16);
      expect(centeredColumnPadding(360, contentWidth: kListColumnWidth, base: 20), 20);
    });

    test('宽屏上把内容收成居中的一条列', () {
      // 1097 - 840 = 257，一半约 128.5
      expect(
        centeredColumnPadding(1097, contentWidth: kListColumnWidth),
        closeTo(128.5, 0.01),
      );
      // 恰好等于内容列宽时仍取 base，不会算出 0
      expect(centeredColumnPadding(840, contentWidth: kListColumnWidth), 16);
    });

    test('正文列比列表列更窄，所以收得更狠', () {
      final reading = centeredColumnPadding(1097, contentWidth: kReadingColumnWidth);
      final list = centeredColumnPadding(1097, contentWidth: kListColumnWidth);
      expect(reading, greaterThan(list));
    });
  });

  testWidgets('ReadingColumn 在手机宽度下不改变子节点尺寸', (tester) async {
    late Size childSize;
    await tester.pumpWidget(
      Directionality(
        textDirection: TextDirection.ltr,
        child: Center(
          child: SizedBox(
            width: 360,
            height: 600,
            child: ReadingColumn(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  childSize = Size(constraints.maxWidth, constraints.maxHeight);
                  return const SizedBox.expand();
                },
              ),
            ),
          ),
        ),
      ),
    );
    expect(childSize.width, 360);
  });

  testWidgets('ReadingColumn 在宽屏上把子节点限到 maxWidth', (tester) async {
    late Size childSize;
    await tester.pumpWidget(
      Directionality(
        textDirection: TextDirection.ltr,
        child: Center(
          child: SizedBox(
            width: 1097,
            height: 600,
            child: ReadingColumn(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  childSize = Size(constraints.maxWidth, constraints.maxHeight);
                  return const SizedBox.expand();
                },
              ),
            ),
          ),
        ),
      ),
    );
    expect(childSize.width, kReadingColumnWidth);
  });
}
