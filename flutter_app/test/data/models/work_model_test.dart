import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/models/work.dart';

void main() {
  group('Work API model', () {
    test('parses and orders units while preserving continue target', () {
      final work = Work.fromJson({
        'id': 'work-1',
        'libraryId': 'lib-jp',
        'title': '败犬女主太多了',
        'coverComicId': 'comic-1',
        'coverUrl': '/api/works/work-1/cover',
        'coverAspectRatio': 0.7,
        'itemCount': 2,
        'pageCount': 200,
        'continueComicId': 'comic-2',
        'continueUnitId': 'unit-2',
        'continuePage': 135,
        'lastReadAt': '2026-08-08T01:00:00Z',
        'tags': [
          {'id': 1, 'name': '校园', 'color': '#fff000'}
        ],
        'categories': [
          {'id': 2, 'name': '日漫', 'slug': 'jp'}
        ],
        'units': [
          {
            'id': 'unit-2',
            'workId': 'work-1',
            'comicId': 'comic-2',
            'title': '第二卷',
            'displayLabel': '第2卷',
            'startPage': 100,
            'pageCount': 100,
            'sortIndex': 2,
          },
          {
            'id': 'unit-1',
            'workId': 'work-1',
            'comicId': 'comic-1',
            'title': '第一卷',
            'displayLabel': '第1卷',
            'startPage': 0,
            'pageCount': 100,
            'sortIndex': 1,
          },
        ],
      });

      expect(work.units.map((unit) => unit.id), ['unit-1', 'unit-2']);
      expect(work.continueTarget?.unit.id, 'unit-2');
      expect(work.continueTarget?.absolutePage, 135);
      expect(work.continueTarget?.relativePage, 35);
      expect(work.tags.single.name, '校园');
      expect(work.categories.single.slug, 'jp');
      expect(work.progress, 68);
    });

    test('falls back to the first unit for immediate reading', () {
      final work = Work.fromJson({
        'id': 'work-2',
        'title': '恶魔X天使 不能友好相处',
        'units': [
          {
            'id': 'preview',
            'workId': 'work-2',
            'comicId': 'comic-preview',
            'displayLabel': '预告',
            'startPage': 0,
            'pageCount': 10,
            'sortIndex': 0,
          }
        ],
      });

      expect(work.hasReadingProgress, isFalse);
      expect(work.readingTarget?.unit.id, 'preview');
      expect(work.readingTarget?.absolutePage, 0);
      expect(
        work.readerRoute(),
        '/reader/comic-preview?page=0&workId=work-2&unitId=preview',
      );
    });
  });
}
