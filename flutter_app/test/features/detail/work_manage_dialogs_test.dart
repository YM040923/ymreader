import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/app/theme.dart';
import 'package:nowen_reader/data/api/work_api.dart';
import 'package:nowen_reader/data/models/work.dart';
import 'package:nowen_reader/features/detail/work_manage_dialogs.dart';

class _FakeWorkApi implements WorkApi {
  _FakeWorkApi(this.works);

  final List<Work> works;
  List<String>? mergedIds;

  @override
  Future<WorkListResponse> listWorks(
      [WorkQuery query = const WorkQuery()]) async {
    return WorkListResponse(
      works: works,
      total: works.length,
      page: 1,
      pageSize: query.pageSize,
      totalPages: 1,
    );
  }

  @override
  Future<void> mergeWorks(List<String> workIds) async {
    mergedIds = workIds;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('${invocation.memberName}');
}

Work _work(String id, String title) {
  return Work.fromJson({
    'id': id,
    'libraryId': 'default',
    'title': title,
    'coverUrl': '',
    'itemCount': 1,
    'pageCount': 2,
    'units': [
      {
        'id': 'unit-$id',
        'workId': id,
        'comicId': 'comic-$id',
        'title': title,
        'displayLabel': '第1话',
        'startPage': 0,
        'pageCount': 2,
        'sortIndex': 1,
      },
    ],
  });
}

/// A Work built from [archiveIds], one unit per entry, so a test can model a
/// single archive (every unit shares an id) or several archives.
Work _archiveWork(List<String> archiveIds) {
  return Work.fromJson({
    'id': 'work-a',
    'libraryId': 'default',
    'title': '当前作品',
    'coverUrl': '',
    'itemCount': archiveIds.length,
    'pageCount': archiveIds.length * 2,
    'units': [
      for (var index = 0; index < archiveIds.length; index++)
        {
          'id': 'unit-$index',
          'workId': 'work-a',
          'comicId': archiveIds[index],
          'title': '第$index话',
          'displayLabel': '第$index话',
          'startPage': index * 2,
          'pageCount': 2,
          'sortIndex': index + 1,
        },
    ],
  });
}

Future<void> _openSplit(WidgetTester tester, Work work) async {
  await tester.binding.setSurfaceSize(const Size(400, 800));
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    ProviderScope(
      overrides: [workApiProvider.overrideWithValue(_FakeWorkApi(const []))],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        home: Scaffold(
          body: Builder(
            builder: (context) => TextButton(
              onPressed: () => showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                builder: (_) => SplitWorkSheet(work: work),
              ),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('merge dialog lays out inside a bounded dialog surface',
      (tester) async {
    final api = _FakeWorkApi([_work('work-b', '目标作品')]);
    await tester.binding.setSurfaceSize(const Size(400, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [workApiProvider.overrideWithValue(api)],
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: MergeWorkDialog(work: _work('work-a', '当前作品')),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('合并作品'), findsOneWidget);
    expect(find.text('目标作品'), findsOneWidget);
    final confirm = find.widgetWithText(FilledButton, '确认合并');
    expect(confirm, findsOneWidget);
    expect(tester.getSize(confirm).width, greaterThan(0));
    expect(tester.getSize(confirm).width, lessThan(400));
  });

  testWidgets('merge dialog sends the target work first', (tester) async {
    final api = _FakeWorkApi([_work('work-b', '目标作品')]);
    await tester.binding.setSurfaceSize(const Size(400, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [workApiProvider.overrideWithValue(api)],
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: MergeWorkDialog(work: _work('work-a', '当前作品')),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('目标作品'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('确认合并'));
    await tester.pumpAndSettle();

    expect(api.mergedIds, ['work-b', 'work-a']);
  });

  testWidgets('split sheet lays out its footer buttons', (tester) async {
    await tester.binding.setSurfaceSize(const Size(400, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final work = _work('work-a', '当前作品');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [workApiProvider.overrideWithValue(_FakeWorkApi(const []))],
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () => showModalBottomSheet<void>(
                  context: context,
                  builder: (_) => SplitWorkSheet(work: work),
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('拆分单元'), findsOneWidget);
    final confirm = find.widgetWithText(FilledButton, '确认拆分');
    expect(confirm, findsOneWidget);
    expect(tester.getSize(confirm).width, greaterThan(0));
    expect(tester.getSize(confirm).width, lessThan(400));
  });

  testWidgets('single-archive work keeps 确认拆分 disabled without extra copy',
      (tester) async {
    await _openSplit(tester, _archiveWork(['comic-a', 'comic-a', 'comic-a']));

    await tester.tap(find.byType(CheckboxListTile).first);
    await tester.pumpAndSettle();

    final confirm = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '确认拆分'),
    );
    expect(confirm.onPressed, isNull);
    // The greyed-out button is the whole message: no explanatory line.
    expect(find.text('同一压缩包内的单元不能只拆出一部分'), findsNothing);
    expect(find.text('至少保留一个单元'), findsNothing);
  });

  testWidgets('two archives make the first unit a valid split', (tester) async {
    await _openSplit(tester, _archiveWork(['comic-a', 'comic-a', 'comic-b']));

    await tester.tap(find.byType(CheckboxListTile).last);
    await tester.pumpAndSettle();

    final confirm = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '确认拆分'),
    );
    expect(confirm.onPressed, isNotNull);
  });
}
