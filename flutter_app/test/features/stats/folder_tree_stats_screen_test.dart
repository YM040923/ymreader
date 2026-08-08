import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/models/folder_stats.dart';
import 'package:nowen_reader/features/stats/folder_tree_stats_screen.dart';

void main() {
  testWidgets('defaults to Work scope and switches to physical scope', (
    tester,
  ) async {
    final requested = <FolderStatsScope>[];

    await tester.pumpWidget(
      MaterialApp(
        home: FolderTreeStatsScreen(
          loadStats: (scope) async {
            requested.add(scope);
            return FolderStatsScreenData(
              tree: const FolderStatsResponse(),
              summary: const FileStatsSummary(),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(requested, [FolderStatsScope.work]);
    expect(find.text('按作品'), findsOneWidget);

    await tester.tap(find.text('按物理文件'));
    await tester.pumpAndSettle();

    expect(requested, [
      FolderStatsScope.work,
      FolderStatsScope.physical,
    ]);
  });

  testWidgets('opens Work files with the injected navigation callback', (
    tester,
  ) async {
    FolderStatsFile? opened;
    const file = FolderStatsFile(
      id: 'work-1',
      title: '测试作品',
      filename: '测试作品',
      fileSize: 300,
      pageCount: 30,
      type: 'work',
    );

    await tester.pumpWidget(
      MaterialApp(
        home: FolderTreeStatsScreen(
          loadStats: (_) async => const FolderStatsScreenData(
            tree: FolderStatsResponse(
              roots: [
                FolderStatsNode(
                  name: '日漫',
                  path: '日漫',
                  fileCount: 1,
                  totalSize: 300,
                  totalPages: 30,
                  files: [file],
                ),
              ],
            ),
            summary: FileStatsSummary(
              totalFiles: 1,
              totalSize: 300,
              totalPages: 30,
            ),
          ),
          onOpenFile: (value) => opened = value,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('测试作品'));
    expect(opened?.id, 'work-1');
    expect(opened?.type, 'work');
  });

  testWidgets('shows empty and error states without crashing', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: FolderTreeStatsScreen(
          loadStats: (_) async => const FolderStatsScreenData(
            tree: FolderStatsResponse(),
            summary: FileStatsSummary(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('暂无统计数据'), findsOneWidget);

    await tester.pumpWidget(
      MaterialApp(
        home: FolderTreeStatsScreen(
          key: UniqueKey(),
          loadStats: (_) async => throw const FormatException('bad tree'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('加载失败'), findsOneWidget);
  });
}
