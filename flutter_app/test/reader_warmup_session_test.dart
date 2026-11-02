import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/api/comic_api.dart';
import 'package:nowen_reader/data/services/reader_warmup_session.dart';

class FakeWarmupApi extends ComicApi {
  FakeWarmupApi() : super(Dio());
  final requests = <({String comic, String session, int page, int count})>[];
  final ended = <String>[];
  final endedComics = <String>[];

  @override
  Future<void> warmupPages(String comicId, String sessionId,
      {required int startPage, required int count}) async {
    requests.add(
        (comic: comicId, session: sessionId, page: startPage, count: count));
  }

  @override
  Future<void> endWarmup(String comicId, String sessionId) async {
    ended.add(sessionId);
    endedComics.add(comicId);
  }
}

void main() {
  testWidgets('warmup lease renews, closes, and resumes with a fresh id',
      (tester) async {
    final api = FakeWarmupApi();
    final session = ReaderWarmupSession(api: api, comicId: 'book');
    session.start(20, 100);
    expect(api.requests.single.count, -1);
    final firstId = api.requests.single.session;
    session.prefetch(20);
    session.prefetch(21);
    expect(api.requests.length, 2);
    expect(api.requests.last.page, 21);
    expect(api.requests.last.count, 8);
    await tester.pump(const Duration(seconds: 30));
    expect(api.requests.last.count, -1);
    session.didChangeAppLifecycleState(AppLifecycleState.paused);
    expect(api.ended, [firstId]);
    final count = api.requests.length;
    await tester.pump(const Duration(minutes: 3));
    expect(api.requests.length, count);
    session.didChangeAppLifecycleState(AppLifecycleState.resumed);
    expect(api.requests.last.session, isNot(firstId));
    session.dispose();
    session.dispose();
    expect(api.ended.length, 2);
    final finalCount = api.requests.length;
    await tester.pump(const Duration(minutes: 3));
    expect(api.requests.length, finalCount);
  });

  testWidgets('跨进另一本 comic 时换绑租约：旧的收掉，新的重挂', (tester) async {
    final api = FakeWarmupApi();
    final session = ReaderWarmupSession(api: api, comicId: 'one');
    session.start(20, 100);
    final firstId = api.requests.single.session;
    expect(api.requests.single.comic, 'one');

    session.rebind('two');
    expect(api.ended, [firstId]);
    expect(api.endedComics, ['one']);

    session.start(0, 30);
    expect(api.requests.last.comic, 'two');
    expect(api.requests.last.session, isNot(firstId));
    expect(api.requests.last.page, 0);

    // 同一本 comic 再走一遍 rebind 不该把租约拆掉重来。
    session.rebind('two');
    expect(api.ended.length, 1);

    // start 可重入：反复换话不会重复注册生命周期观察者。
    session.start(5, 30);
    session.prefetch(6);
    expect(api.requests.last.comic, 'two');
    expect(api.requests.last.page, 7);

    session.dispose();
    expect(api.ended.length, 2);
    expect(api.endedComics, ['one', 'two']);
  });
}
