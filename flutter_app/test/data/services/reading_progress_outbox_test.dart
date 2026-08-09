import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/services/reading_progress_outbox.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('outbox keeps only the latest event for one server user and work',
      () async {
    SharedPreferences.setMockInitialValues({});
    final outbox = ReadingProgressOutbox();
    final first = ReadingProgressEvent(
      serverKey: 'server',
      userKey: 'user',
      comicId: 'comic',
      workId: 'work',
      unitId: 'unit-1',
      relativePage: 2,
      page: 12,
      totalPages: 100,
      activeSeconds: 1,
      clientSessionId: 'session',
      sequence: 1,
    );
    await outbox.put(first);
    await outbox.put(first.copyWith(
      unitId: 'unit-2',
      relativePage: 4,
      page: 24,
      sequence: 2,
    ));

    final pending = await outbox.readAll();
    expect(pending, hasLength(1));
    expect(pending.single.unitId, 'unit-2');
    expect(pending.single.relativePage, 4);
    expect(pending.single.sequence, 2);
  });
}
