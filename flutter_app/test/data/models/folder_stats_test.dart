import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/models/folder_stats.dart';

void main() {
  group('FolderStatsResponse', () {
    test('parses Work tree files from a raw API array', () {
      final response = FolderStatsResponse.fromJson([
        {
          'name': '日漫',
          'path': '日漫',
          'fileCount': 1,
          'totalSize': 300,
          'totalPages': 30,
          'files': [
            {
              'id': 'work-1',
              'title': '作品',
              'filename': '作品',
              'fileSize': 300,
              'pageCount': 30,
              'type': 'work',
            },
          ],
          'children': <dynamic>[],
        },
      ]);

      expect(response.roots, hasLength(1));
      expect(response.roots.single.name, '日漫');
      expect(response.roots.single.files.single.id, 'work-1');
      expect(response.roots.single.files.single.type, 'work');
    });

    test('accepts numeric values represented as doubles and strings', () {
      final response = FolderStatsResponse.fromJson([
        {
          'name': '国漫',
          'fileCount': '2',
          'totalSize': 512.8,
          'totalPages': '100',
          'files': [
            {
              'id': 'comic-1',
              'fileSize': '256',
              'pageCount': 50.0,
              'type': 'comic',
            },
          ],
        },
      ]);
      final summary = FileStatsSummary.fromJson({
        'totalFiles': '2',
        'totalSize': 512.8,
        'totalPages': '100',
      });

      expect(response.roots.single.fileCount, 2);
      expect(response.roots.single.totalSize, 512);
      expect(response.roots.single.files.single.pageCount, 50);
      expect(summary.totalFiles, 2);
      expect(summary.totalSize, 512);
      expect(summary.totalPages, 100);
    });

    test('rejects the removed object-wrapped tree contract', () {
      expect(
        () => FolderStatsResponse.fromJson({'tree': <dynamic>[]}),
        throwsA(isA<FormatException>()),
      );
    });
  });
}
