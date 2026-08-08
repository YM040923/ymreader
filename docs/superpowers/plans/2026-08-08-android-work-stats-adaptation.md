# Android Work Stats Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the obsolete collection client, repair folder statistics, and align Android scan-rule controls with the server Work model.

**Architecture:** Add typed folder-stat models at the API boundary, keep the statistics screen responsible only for scope selection and rendering, and make scan-rule payload generation a pure function that can be regression-tested. Remove collection-only routes and source files rather than hiding them.

**Tech Stack:** Flutter 3.44, Dart 3.12, Riverpod, Dio, GoRouter, flutter_test, Android Gradle release build.

---

### Task 1: Restore a clean Android baseline

**Files:**
- Restore generated file: `flutter_app/macos/Flutter/GeneratedPluginRegistrant.swift`
- Modify: `flutter_app/pubspec.yaml`

- [ ] **Step 1: Restore the unrelated generated macOS change**

Run:

```powershell
git restore -- flutter_app/macos/Flutter/GeneratedPluginRegistrant.swift
```

Expected: `git status --short` no longer lists this generated file.

- [ ] **Step 2: Update the Android client version**

Change:

```yaml
version: 1.2.2+43
```

- [ ] **Step 3: Commit the baseline/version change**

```powershell
git add flutter_app/pubspec.yaml
git commit -m "chore(android): bump client version to 1.2.2"
```

### Task 2: Add typed folder-stat API contracts

**Files:**
- Create: `flutter_app/lib/data/models/folder_stats.dart`
- Modify: `flutter_app/lib/data/api/comic_api.dart`
- Create: `flutter_app/test/data/models/folder_stats_test.dart`

- [ ] **Step 1: Write failing model tests**

Tests must cover:

```dart
final nodes = FolderStatsResponse.fromJson([
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
      }
    ],
  }
]);
expect(nodes.roots.single.files.single.type, 'work');
```

Also verify integer parsing from `double` and numeric strings.

- [ ] **Step 2: Run the test and verify RED**

```powershell
flutter test test/data/models/folder_stats_test.dart
```

Expected: FAIL because `FolderStatsResponse` does not exist.

- [ ] **Step 3: Implement models**

Define:

```dart
enum FolderStatsScope { work, physical }

class FolderStatsResponse {
  final List<FolderStatsNode> roots;
  factory FolderStatsResponse.fromJson(dynamic json);
}

class FolderStatsNode {
  final String name;
  final String path;
  final int fileCount;
  final int totalSize;
  final int totalPages;
  final List<FolderStatsNode> children;
  final List<FolderStatsFile> files;
}

class FolderStatsFile {
  final String id;
  final String title;
  final String filename;
  final int fileSize;
  final int pageCount;
  final String type;
}

class FileStatsSummary {
  final int totalFiles;
  final int totalSize;
  final int totalPages;
}
```

`FolderStatsResponse.fromJson` must reject non-list roots with `FormatException`.

- [ ] **Step 4: Adapt API methods**

Replace the map-returning method with:

```dart
Future<FolderStatsResponse> getFolderTreeStats({
  FolderStatsScope scope = FolderStatsScope.work,
}) async {
  final res = await _dio.get(
    '/stats/folder-tree',
    queryParameters: {'scope': scope.name},
  );
  return FolderStatsResponse.fromJson(res.data);
}

Future<FileStatsSummary> getFileStats({
  FolderStatsScope scope = FolderStatsScope.work,
}) async {
  final res = await _dio.get(
    '/stats/files',
    queryParameters: {'scope': scope.name},
  );
  return FileStatsSummary.fromJson(res.data);
}
```

- [ ] **Step 5: Run tests and commit**

```powershell
flutter test test/data/models/folder_stats_test.dart
git add flutter_app/lib/data/models/folder_stats.dart flutter_app/lib/data/api/comic_api.dart flutter_app/test/data/models/folder_stats_test.dart
git commit -m "fix(android): decode Work folder statistics"
```

### Task 3: Rebuild the folder statistics screen

**Files:**
- Modify: `flutter_app/lib/features/stats/folder_tree_stats_screen.dart`
- Create: `flutter_app/test/features/stats/folder_tree_stats_screen_test.dart`

- [ ] **Step 1: Write widget tests**

Tests must verify:

```text
默认选中“按作品”
切换“按物理文件”后重新请求 physical scope
Work 文件项点击后导航到 /work/<id>
comic 文件项点击后导航到 /comic/<id>
空数组显示空状态
FormatException 显示加载错误
```

- [ ] **Step 2: Run the tests and verify RED**

```powershell
flutter test test/features/stats/folder_tree_stats_screen_test.dart
```

- [ ] **Step 3: Implement the screen**

The screen must:

- load folder tree and summary together with `Future.wait`;
- use a two-button segmented selector for `work` and `physical`;
- clear stale results before scope changes;
- render summary totals from `/stats/files`;
- render child folders and `files`;
- use Work/Comic icons and navigation according to `file.type`;
- show refresh, loading, empty and error states.

- [ ] **Step 4: Run tests and commit**

```powershell
flutter test test/features/stats/folder_tree_stats_screen_test.dart
git add flutter_app/lib/features/stats/folder_tree_stats_screen.dart flutter_app/test/features/stats/folder_tree_stats_screen_test.dart
git commit -m "feat(android): add Work-aware folder statistics"
```

### Task 4: Remove the collection client completely

**Files:**
- Modify: `flutter_app/lib/features/settings/settings_screen.dart`
- Modify: `flutter_app/lib/app/router.dart`
- Modify: `flutter_app/lib/data/api/comic_api.dart`
- Modify: `flutter_app/lib/data/providers/comic_provider.dart`
- Modify: `flutter_app/lib/data/models/comic.dart`
- Delete: `flutter_app/lib/features/collections/collections_screen.dart`
- Delete: `flutter_app/lib/features/groups/group_detail_screen.dart`
- Delete: `flutter_app/lib/features/groups/group_detail_v2_screen.dart`
- Delete: `flutter_app/lib/widgets/group_card.dart`
- Create: `flutter_app/test/app/removed_collection_routes_test.dart`

- [ ] **Step 1: Write a source-level regression test**

The test reads `lib/app/router.dart` and `lib/features/settings/settings_screen.dart` and asserts they do not contain:

```text
/collections
/group/:id
合集管理
CollectionsScreen
GroupDetailV2Screen
```

- [ ] **Step 2: Run the test and verify RED**

```powershell
flutter test test/app/removed_collection_routes_test.dart
```

- [ ] **Step 3: Remove all collection-only references**

Delete the settings tile, routes, imports, API methods, providers, `ComicGroup` model and collection-only files. Do not remove unrelated uses of collection icons for ordinary comic files.

- [ ] **Step 4: Verify no collection symbols remain**

```powershell
rg -n "ComicGroup|CollectionsScreen|GroupDetail|groupsProvider|groupsByTypeProvider|/collections|/group/:id|合集管理" flutter_app/lib
```

Expected: no matches.

- [ ] **Step 5: Run tests and commit**

```powershell
flutter test test/app/removed_collection_routes_test.dart
git add -A flutter_app
git commit -m "refactor(android): remove obsolete collection client"
```

### Task 5: Align scan-rule UI and payloads with Work

**Files:**
- Create: `flutter_app/lib/features/scan_rules/scan_rules_payload.dart`
- Modify: `flutter_app/lib/features/scan_rules/scan_rules_screen.dart`
- Create: `flutter_app/test/features/scan_rules/scan_rules_payload_test.dart`

- [ ] **Step 1: Write failing payload tests**

Expected payload:

```dart
final payload = buildScanRulesPayload(
  enabled: false,
  applyOn: 'newOnly',
  aiEnabled: true,
  minConfidence: 'medium',
  overwriteTitle: false,
  directoryOrganizeEnabled: true,
  directoryOrganizeMode: 'hardlink',
  directoryOrganizeStrategy: 'smartDir',
  hardlinkTargetDir: '/app/_nowen_organized',
);

expect(payload.containsKey('organize'), isFalse);
expect(payload['aiInfer'].containsKey('applyToGroup'), isFalse);
expect(payload['aiInfer'].containsKey('applyToComic'), isFalse);
expect(payload['aiInfer']['scope'], 'folderGroup');
```

- [ ] **Step 2: Run the test and verify RED**

```powershell
flutter test test/features/scan_rules/scan_rules_payload_test.dart
```

- [ ] **Step 3: Implement pure payload construction**

Keep legacy-compatible fixed fields (`scope: folderGroup`, `applyToComic: false`) only when required by the server decoder, but do not expose them as user controls. Never send `organize` or `applyToGroup`.

- [ ] **Step 4: Update the screen**

Remove state and UI for:

```text
_aiScope
_applyToComic
_applyToGroup
_organizeEnabled
_autoGroupByDir
_inheritMeta
识别范围
写回单卷字段
同步到所属分组
虚拟归类
```

Update copy to explain:

```text
AI 每部 Work 识别一次，结果写入作品级元数据。
单大 ZIP 保持单文件，多 CBZ 归入同一作品目录。
硬链接目标必须与源文件处于同一文件系统。
```

The recommended preset configures child actions but leaves `_enabled = false`.

Display `physicalTotal` when the result contains it.

- [ ] **Step 5: Run tests and commit**

```powershell
flutter test test/features/scan_rules/scan_rules_payload_test.dart
git add flutter_app/lib/features/scan_rules flutter_app/test/features/scan_rules
git commit -m "feat(android): align scan rules with Work model"
```

### Task 6: Full verification and signed APK

**Files:**
- Generated artifact: `flutter_app/build/app/outputs/flutter-apk/app-release.apk`

- [ ] **Step 1: Format changed Dart files**

```powershell
dart format flutter_app/lib flutter_app/test
```

- [ ] **Step 2: Run the complete test suite**

```powershell
cd flutter_app
flutter test
```

Expected: all tests pass.

- [ ] **Step 3: Run analyzer**

```powershell
flutter analyze
```

Expected: no new errors or warnings in changed files. Existing unrelated informational deprecations may remain.

- [ ] **Step 4: Build the signed release APK**

```powershell
flutter build apk --release
```

Expected artifact:

```text
flutter_app/build/app/outputs/flutter-apk/app-release.apk
```

- [ ] **Step 5: Verify APK identity and checksum**

Use Android build tools to verify package/version/signature and calculate SHA-256.

- [ ] **Step 6: Commit and push**

```powershell
git status --short
git push fork ymreader-final
```

Do not deploy or modify NAS content during Android packaging.
