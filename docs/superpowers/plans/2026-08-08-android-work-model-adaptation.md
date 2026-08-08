# Android Work Model Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adapt the existing customized Flutter client to the YMReader Work API without replacing its current UI, offline support, or physical comic readers.

**Architecture:** Add a Work-specific domain/API/provider layer and make it the primary online catalog. Keep `Comic` as the physical file/page model. Pass a `WorkReaderContext` into existing readers so directory navigation and progress are work-aware while legacy and offline routes remain valid.

**Tech Stack:** Flutter 3.44, Dart 3.12, Riverpod, Dio, GoRouter, SharedPreferences, Flutter test.

---

### Task 1: Establish baseline and Work domain contracts

**Files:**
- Create: `flutter_app/lib/data/models/work.dart`
- Create: `flutter_app/test/data/models/work_model_test.dart`

- [ ] **Step 1: Write failing Work parsing tests**

Cover parsing of `Work`, ordered `WorkUnit` entries, nullable metadata, `continueComicId`, `continueUnitId`, `continuePage`, cover URL and aspect ratio.

- [ ] **Step 2: Run the model test and confirm it fails**

Run:

```powershell
flutter test test/data/models/work_model_test.dart
```

Expected: failure because `work.dart` does not exist.

- [ ] **Step 3: Implement immutable Work models**

Add:

```dart
class WorkUnit {
  final String id;
  final String workId;
  final String comicId;
  final String title;
  final String displayLabel;
  final int startPage;
  final int pageCount;
  final int sortIndex;
  final String? coverUrl;
}

class Work {
  final String id;
  final String libraryId;
  final String title;
  final String coverComicId;
  final String? coverUrl;
  final List<WorkUnit> units;
  // metadata, status and continue-reading fields
}
```

Use tolerant numeric/string parsing consistent with `comic.dart`. Sort units by `sortIndex`, then display label.

- [ ] **Step 4: Run the test and confirm it passes**

- [ ] **Step 5: Commit**

```powershell
git add flutter_app/lib/data/models/work.dart flutter_app/test/data/models/work_model_test.dart
git commit -m "feat(android): add Work domain models"
```

### Task 2: Add Work and library APIs

**Files:**
- Create: `flutter_app/lib/data/api/work_api.dart`
- Create: `flutter_app/lib/data/api/library_api.dart`
- Create: `flutter_app/test/data/api/work_api_contract_test.dart`
- Modify: `flutter_app/lib/data/api/api_client.dart`

- [ ] **Step 1: Write failing query and endpoint contract tests**

Verify:

- `/works` query uses `page`, `pageSize`, `sortBy`, `sortOrder`, `search`, `libraryIds`, `favorites`, `tags`, `category`.
- `/works/:id` and `/works/:id/units` encode IDs.
- favorite, rating and reading-status mutations use Work endpoints.
- `/libraries` parses server library IDs and names.

- [ ] **Step 2: Run tests and confirm failure**

- [ ] **Step 3: Implement `WorkApi` and `LibraryApi`**

Use the shared authenticated `Dio` instance. Do not duplicate cookie or base-path handling.

- [ ] **Step 4: Add Work cover URL normalization**

Relative Work covers must resolve against the configured server. Empty Work cover falls back to `coverComicId` thumbnail.

- [ ] **Step 5: Run tests and commit**

```powershell
git commit -m "feat(android): connect Work and library APIs"
```

### Task 3: Add Work providers and legacy capability fallback

**Files:**
- Create: `flutter_app/lib/data/providers/work_provider.dart`
- Create: `flutter_app/lib/data/services/server_capabilities.dart`
- Create: `flutter_app/test/data/services/server_capabilities_test.dart`
- Modify: `flutter_app/lib/data/providers/auth_provider.dart`

- [ ] **Step 1: Write failing capability tests**

Test Work-capable, 404 legacy, unauthorized, offline and transient-error behavior. A transient error must not permanently downgrade a Work-capable server.

- [ ] **Step 2: Implement capability state**

Cache support by normalized server URL. Probe authenticated `/works?page=1&pageSize=1`; use legacy only for a confirmed 404/405.

- [ ] **Step 3: Implement Work list state/notifier**

Support refresh, pagination, search, library selection, favorites, tag/category filters and sorting.

- [ ] **Step 4: Run tests and commit**

```powershell
git commit -m "feat(android): add Work catalog state"
```

### Task 4: Replace the online home catalog with Work cards

**Files:**
- Create: `flutter_app/lib/widgets/work_card.dart`
- Modify: `flutter_app/lib/features/home/home_screen.dart`
- Modify: `flutter_app/lib/widgets/continue_reading.dart`
- Modify: `flutter_app/lib/features/favorites/favorites_screen.dart`
- Modify: `flutter_app/lib/features/search/search_screen.dart`
- Create: `flutter_app/test/widgets/work_catalog_test.dart`

- [ ] **Step 1: Write failing widget tests**

Verify:

- each Work renders once regardless of unit count;
- no “合集” or “按文件夹收起” control is rendered;
- horizontal library chips update the Work query;
- cards use Work titles and Work cover URLs;
- continue reading routes to the Work’s saved unit/page.

- [ ] **Step 2: Implement `WorkCard` using existing visual tokens**

Preserve current poster-wall proportions, typography, animation and authenticated image loading.

- [ ] **Step 3: Convert Home, Search and Favorites**

Use Work providers in Work mode. Preserve legacy widgets only behind the legacy capability branch.

- [ ] **Step 4: Convert Continue Reading**

Query Works sorted by `lastReadAt`, filter readable progress and build Work-aware reader routes.

- [ ] **Step 5: Run tests and commit**

```powershell
git commit -m "feat(android): show Work-based online library"
```

### Task 5: Add Work detail page and route

**Files:**
- Create: `flutter_app/lib/features/detail/work_detail_screen.dart`
- Modify: `flutter_app/lib/app/router.dart`
- Create: `flutter_app/test/features/detail/work_detail_test.dart`

- [ ] **Step 1: Write failing detail tests**

Verify Work metadata, scraped cover, ordered directory, immediate/continue button states and unit selection.

- [ ] **Step 2: Add `/work/:id` route**

Keep `/comic/:id` and `/group/:id` as compatibility routes. Online Work cards must navigate only to `/work/:id`.

- [ ] **Step 3: Implement detail screen**

Reuse current detail page visual hierarchy. Do not reintroduce collection/series editing.

- [ ] **Step 4: Run tests and commit**

```powershell
git commit -m "feat(android): add Work detail and directory"
```

### Task 6: Make reader navigation Work-aware

**Files:**
- Create: `flutter_app/lib/features/reader/work_reader_context.dart`
- Modify: `flutter_app/lib/app/router.dart`
- Modify: `flutter_app/lib/features/reader/reader_dispatch_screen.dart`
- Modify: `flutter_app/lib/features/reader/comic_reader_screen.dart`
- Modify: `flutter_app/lib/features/reader/pdf_reader_screen.dart`
- Modify: `flutter_app/lib/features/reader/novel_reader_screen.dart`
- Create: `flutter_app/test/data/models/work_reader_context_test.dart`

- [ ] **Step 1: Write failing route/navigation tests**

Cover route construction/parsing, current unit resolution, previous/next unit, physical/relative page conversion and return-to-detail behavior.

- [ ] **Step 2: Implement `WorkReaderContext`**

The context owns Work/unit IDs and ordered units; physical readers continue to own image/PDF/novel rendering.

- [ ] **Step 3: Wire directory, previous and next actions**

Selecting a unit replaces the physical reader target while retaining the Work context. Back returns to `/work/:id`.

- [ ] **Step 4: Synchronize progress**

Continue using physical comic activity APIs so the server’s Work projection updates naturally. Resolve Work continue position after each unit transition.

- [ ] **Step 5: Run tests and commit**

```powershell
git commit -m "feat(android): navigate reader by Work units"
```

### Task 7: Store reading layout per Work

**Files:**
- Modify: `flutter_app/lib/widgets/reader_settings_panel.dart`
- Modify: `flutter_app/lib/features/reader/comic_reader_screen.dart`
- Create: `flutter_app/lib/data/services/reader_preferences.dart`
- Create: `flutter_app/test/data/services/reader_preferences_test.dart`

- [ ] **Step 1: Write failing isolation tests**

Verify two Work IDs can retain different modes/directions while preload and image settings remain global. Verify `comic:<id>` fallback.

- [ ] **Step 2: Implement scoped preference storage**

Use `reader_options_by_work_v1` and scope keys matching the Web client:

```text
work:<workId>
comic:<comicId>
```

- [ ] **Step 3: Load preferences before reader layout initialization**

Avoid briefly rendering the previous Work’s mode.

- [ ] **Step 4: Run tests and commit**

```powershell
git commit -m "feat(android): remember reader layout per Work"
```

### Task 8: Regression, versioning, signing and APK

**Files:**
- Modify: `flutter_app/pubspec.yaml`
- Modify: `flutter_app/android/app/build.gradle.kts` only if existing signing lookup needs restoration
- Create: `docs/releases/android-work-model-2026-08-08.md`

- [ ] **Step 1: Run formatting and static checks**

```powershell
dart format --set-exit-if-changed lib test
flutter analyze
flutter test
```

- [ ] **Step 2: Run release build**

Locate the existing private signing configuration without committing secrets. Build:

```powershell
flutter build apk --release
```

- [ ] **Step 3: Verify package metadata and signature**

Use Android build tools to confirm application ID, version and signing certificate. Confirm the APK can replace the prior customized package.

- [ ] **Step 4: Copy artifact to fixed release path**

```text
F:\meihua\releases\YMReader-Android-WorkModel-<version>.apk
```

- [ ] **Step 5: Commit and push**

```powershell
git add flutter_app docs/releases
git commit -m "release(android): adapt client to YMReader Work model"
git push fork ymreader-final
```

