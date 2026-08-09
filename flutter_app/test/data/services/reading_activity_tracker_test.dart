import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/services/reading_activity_tracker.dart';
import 'package:nowen_reader/data/services/reading_progress_outbox.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('tracker serializes requests and eventually sends the latest page',
      () async {
    SharedPreferences.setMockInitialValues({});
    final sent = <ReadingProgressEvent>[];
    final gates = <Completer<void>>[];
    final tracker = ReadingActivityTracker(
      comicId: 'comic',
      serverKey: 'server',
      userKey: 'user',
      observeLifecycle: false,
      sender: (event) async {
        sent.add(event);
        if (event.finalize) return event;
        final gate = Completer<void>();
        gates.add(gate);
        await gate.future;
        return event;
      },
    );

    tracker.start(0, 100);
    final firstFlush = tracker.flush();
    await Future<void>.delayed(Duration.zero);
    tracker.updatePage(5, 100);
    tracker.updatePage(9, 100);
    expect(sent, hasLength(1));

    gates.first.complete();
    while (sent.length < 2) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }
    expect(sent.last.page, 9);
    gates.last.complete();
    await firstFlush;
    await tracker.finish();
  });

  test('switching work unit flushes relative page context', () async {
    SharedPreferences.setMockInitialValues({});
    final sent = <ReadingProgressEvent>[];
    final tracker = ReadingActivityTracker(
      comicId: 'comic',
      serverKey: 'server',
      userKey: 'user',
      observeLifecycle: false,
      sender: (event) async {
        sent.add(event);
        return event;
      },
    );
    tracker.start(
      120,
      200,
      workId: 'work',
      unitId: 'unit-20',
      relativePage: 0,
    );
    tracker.updatePage(
      133,
      200,
      workId: 'work',
      unitId: 'unit-21',
      relativePage: 3,
      flushImmediately: true,
    );
    await tracker.flush();

    expect(sent.last.workId, 'work');
    expect(sent.last.unitId, 'unit-21');
    expect(sent.last.relativePage, 3);
  });

  test('failed send persists the latest cursor', () async {
    SharedPreferences.setMockInitialValues({});
    final outbox = ReadingProgressOutbox();
    final tracker = ReadingActivityTracker(
      comicId: 'comic',
      serverKey: 'server',
      userKey: 'user',
      observeLifecycle: false,
      outbox: outbox,
      sender: (_) async => throw StateError('offline'),
    );
    tracker.start(0, 100, workId: 'work', unitId: 'unit', relativePage: 0);
    tracker.updatePage(
      8,
      100,
      workId: 'work',
      unitId: 'unit',
      relativePage: 8,
    );
    await tracker.finish();

    final pending = await outbox.readAll();
    expect(pending, hasLength(1));
    expect(pending.single.relativePage, 8);
  });
}
