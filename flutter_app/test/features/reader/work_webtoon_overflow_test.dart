import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:nowen_reader/data/services/page_metrics_cache.dart';
import 'package:nowen_reader/features/reader/work_webtoon_view.dart';

// 这个用例单独成文件：PageMetricsCache 是进程内的单例，只认第一次
// ensureLoaded 时读到的那批比例，和同目录其它用例抢同一个 key 会互相盖掉。
const double _ratio = 0.6507592190889371;
const int _pageCount = 3000;

void main() {
  setUp(() async {
    SharedPreferences.setMockInitialValues({
      'reader_page_ratios_v1': '{${[
            for (var index = 0; index < _pageCount; index++)
              '"d:$index":$_ratio',
          ].join(',')}}',
    });
    await PageMetricsCache.instance.ensureLoaded();
  });

  testWidgets('长条排到几千页也不会在页缝画出溢出条', (tester) async {
    // 真机那一屏黄黑斜纹的成因：行约束原来是从累加的 starts 相减得来的，和
    // 「话名带 + 图片宽」的和孩子差最后一位浮点。页数少时误差够不到
    // precisionErrorTolerance（1e-10）看不出来，长条累计到上百万 logical px
    // 之后就跨过去了，于是页缝一条条溢出。
    final controller = WorkWebtoonController();
    await tester.pumpWidget(MaterialApp(
      home: Center(
        child: SizedBox(
          width: 1440,
          height: 600,
          child: WorkWebtoonView(
            controller: controller,
            pages: [
              for (var index = 0; index < _pageCount; index++)
                WorkWebtoonPage(
                  imageUrl: 'http://127.0.0.1:9/real-$index.jpg',
                  comicId: 'd',
                  pageIndex: index,
                  header: index % 7 == 3 ? '第 ${index ~/ 7 + 1} 话' : null,
                ),
            ],
            initialPage: 0,
            onPageChanged: (_) {},
            axis: Axis.horizontal,
          ),
        ),
      ),
    ));
    await tester.pumpAndSettle();

    // 累加误差最大的那一页，滚过去才排得出来。
    controller.jumpToPage(2646);
    await tester.pumpAndSettle();
    expect(controller.page, 2646);
    expect(tester.takeException(), isNull);
    controller.dispose();
  });
}
