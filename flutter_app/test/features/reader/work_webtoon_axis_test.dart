import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:nowen_reader/data/services/page_metrics_cache.dart';
import 'package:nowen_reader/features/reader/work_webtoon_view.dart';

/// Three pages of the same comic, each 1.5 times wider than it is tall.
List<WorkWebtoonPage> _pages({String? headerOn}) => [
      for (var index = 0; index < 3; index++)
        WorkWebtoonPage(
          imageUrl: 'http://127.0.0.1:9/page-$index.jpg',
          comicId: 'c',
          pageIndex: index,
          header: index == 0 ? headerOn : null,
        ),
    ];

/// Navigator 发的是紧约束，所以必须用 Center 把 400×200 这块屏幕让出来。
Widget _stage({
  required WorkWebtoonController controller,
  required Axis axis,
  bool reverse = false,
  String? headerOn,
}) =>
    MaterialApp(
      home: Center(
        child: SizedBox(
          width: 400,
          height: 200,
          child: WorkWebtoonView(
            controller: controller,
            pages: _pages(headerOn: headerOn),
            initialPage: 0,
            onPageChanged: (_) {},
            axis: axis,
            reverse: reverse,
          ),
        ),
      ),
    );

void main() {
  setUp(() async {
    SharedPreferences.setMockInitialValues({
      'reader_page_ratios_v1': '{"c:0":1.5,"c:1":1.5,"c:2":1.5}',
    });
    await PageMetricsCache.instance.ensureLoaded();
  });

  test('webtoonRowExtent 竖排钉宽、横排钉高', () {
    // 同一张 1.5 的页面：竖排在 400 宽里占 266.7，横排在 200 高里占 300。
    expect(webtoonRowExtent(Axis.vertical, 400, 1.5), closeTo(266.667, 0.01));
    expect(webtoonRowExtent(Axis.horizontal, 200, 1.5), closeTo(300, 0.01));
  });

  testWidgets('竖向长条沿高度滚动，行长等于屏幕宽 ÷ 宽高比', (tester) async {
    final controller = WorkWebtoonController();
    await tester.pumpWidget(_stage(
        controller: controller, axis: Axis.vertical));
    await tester.pump();

    final list = tester.widget<ListView>(find.byType(ListView));
    expect(list.scrollDirection, Axis.vertical);
    // 3 行 × 266.67 − 视口 200
    expect(controller.position.maxScrollExtent, closeTo(600, 0.5));
    controller.dispose();
  });

  testWidgets('横向长条沿宽度滚动，行长等于屏幕高 × 宽高比', (tester) async {
    final controller = WorkWebtoonController();
    await tester.pumpWidget(_stage(
        controller: controller, axis: Axis.horizontal));
    await tester.pump();

    final list = tester.widget<ListView>(find.byType(ListView));
    expect(list.scrollDirection, Axis.horizontal);
    expect(list.reverse, isFalse);
    // 3 行 × 300 − 视口 400
    expect(controller.position.maxScrollExtent, closeTo(500, 0.5));
    controller.dispose();
  });

  testWidgets('右→左时横向长条反向滚动，话名带子也算进行长', (tester) async {
    final controller = WorkWebtoonController();
    await tester.pumpWidget(_stage(
      controller: controller,
      axis: Axis.horizontal,
      reverse: true,
      headerOn: '第 1 话',
    ));
    await tester.pump();

    final list = tester.widget<ListView>(find.byType(ListView));
    expect(list.reverse, isTrue);
    // 第一行多了 40 的话名带：(300+40) + 300 + 300 − 400
    expect(controller.position.maxScrollExtent, closeTo(540, 0.5));
    expect(find.byType(RotatedBox), findsOneWidget);
    controller.dispose();
  });

  testWidgets('换成横排时停在原页面上，不跳回开头', (tester) async {
    final controller = WorkWebtoonController();
    await tester.pumpWidget(
        _stage(controller: controller, axis: Axis.vertical));
    await tester.pump();
    controller.jumpToPage(1);
    await tester.pump();
    expect(controller.page, 1);
    expect(controller.position.pixels, closeTo(266.667, 0.5));

    await tester.pumpWidget(
        _stage(controller: controller, axis: Axis.horizontal));
    await tester.pump();
    // 竖排第 1 页的页首落到横排第 1 页的页首：300，而不是回到 0。
    expect(controller.page, 1);
    expect(controller.position.pixels, closeTo(300, 0.5));
    controller.dispose();
  });
}
