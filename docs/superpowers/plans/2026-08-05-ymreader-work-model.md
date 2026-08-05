# YmReader Work Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Work -> Unit -> Page manga model so NowenReader can recognize single archives, multi-chapter folders, multi-volume folders, image folders, and mixed layouts as one manga entry with detail page, table of contents, progress, and OPDS output.

**Architecture:** Keep upstream compatibility by leaving existing Comic and ComicSeries tables/routes intact. Add a focused workmodel package plus Work/WorkUnit/WorkProgress store/API layers; existing Comic remains the physical readable resource while Work becomes the user-facing manga object.

**Tech Stack:** Go backend with SQLite migrations, Gin handlers, existing React/Vite frontend, existing Flutter Android client, existing archive/comic parser services.

---

## File Structure

Create backend:
- `internal/workmodel/types.go`
- `internal/workmodel/resolver.go`
- `internal/workmodel/sorter.go`
- `internal/workmodel/detector.go`
- `internal/workmodel/resolver_test.go`
- `internal/workmodel/detector_test.go`
- `internal/store/migrate_work_model.go`
- `internal/store/work_store.go`
- `internal/store/work_store_test.go`
- `internal/service/work_service.go`
- `internal/service/work_service_test.go`
- `internal/handler/work_handler.go`
- `internal/handler/routes_works.go`
- `internal/handler/work_handler_test.go`

Modify backend:
- `internal/model/models.go`
- `internal/handler/router.go`
- `internal/handler/routes_comics.go`
- `internal/service/opds.go`
- `internal/handler/opds_handler.go`
- `docs/API.md`

Create/modify web:
- `frontend/src/types/work.ts`
- `frontend/src/api/works.ts`
- `frontend/src/components/WorkCard.tsx`
- `frontend/src/app/work/[id]/page.tsx`
- `frontend/src/app/books/page.tsx`
- `frontend/src/app/reader/[id]/page.tsx`
- `frontend/src/hooks/useReadingActivity.ts`

Create/modify Flutter:
- `flutter_app/lib/data/models/work.dart`
- `flutter_app/lib/data/api/work_api.dart`
- `flutter_app/lib/features/home/home_screen.dart`
- `flutter_app/lib/features/detail/work_detail_screen.dart`
- `flutter_app/lib/features/reader/reader_screen.dart`

---

## Task 1: Baseline Sync and Safety Branch

**Files:** git metadata only

- [ ] **Step 1: Verify branch and remotes**

```powershell
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13'
git status --short
git remote -v
git branch --show-current
```

Expected branch: `ymreader-work-model`. Working tree should be clean.

- [ ] **Step 2: Sync official upstream into fork main**

```powershell
git fetch origin --tags --prune
git fetch fork --tags --prune
git checkout main
git merge --ff-only origin/main
git push fork main
```

- [ ] **Step 3: Rebase custom branch**

```powershell
git checkout ymreader-work-model
git rebase main
git push fork ymreader-work-model --force-with-lease
```

- [ ] **Step 4: Commit only if conflict resolution changed files**

```powershell
git add -A
git commit -m "chore: sync ymreader work branch with upstream"
git push fork ymreader-work-model
```

---

## Task 2: Pure Work Structure Resolver

**Files:**
- Create: `internal/workmodel/types.go`
- Create: `internal/workmodel/resolver.go`
- Create: `internal/workmodel/sorter.go`
- Create: `internal/workmodel/resolver_test.go`

- [ ] **Step 1: Write failing resolver tests**

Create `internal/workmodel/resolver_test.go`:

```go
package workmodel

import "testing"

func TestResolveSingleArchiveAsFullWork(t *testing.T) {
	got := ResolvePath("鎭堕瓟X澶╀娇 涓嶈兘鍙嬪ソ鐩稿.zip", "")
	if got.WorkTitle != "鎭堕瓟X澶╀娇 涓嶈兘鍙嬪ソ鐩稿" {
		t.Fatalf("WorkTitle = %q", got.WorkTitle)
	}
	if got.Unit.Kind != UnitKindFull || got.Unit.DisplayLabel != "鍏ㄦ湰" {
		t.Fatalf("Unit = %#v", got.Unit)
	}
}

func TestResolveChineseChapter(t *testing.T) {
	got := ResolvePath("澶х帇楗跺懡/澶х帇楗跺懡 绗?01璇?cbz", "澶х帇楗跺懡")
	if got.WorkTitle != "澶х帇楗跺懡" {
		t.Fatalf("WorkTitle = %q", got.WorkTitle)
	}
	if got.Unit.Kind != UnitKindChapter || got.Unit.ChapterNumber == nil || *got.Unit.ChapterNumber != 1 {
		t.Fatalf("Unit = %#v", got.Unit)
	}
	if got.Unit.DisplayLabel != "绗?01璇? {
		t.Fatalf("DisplayLabel = %q", got.Unit.DisplayLabel)
	}
}

func TestResolveVolume(t *testing.T) {
	got := ResolvePath("璐ョ姮濂充富澶浜?Vol.02.cbz", "璐ョ姮濂充富澶浜?)
	if got.Unit.Kind != UnitKindVolume || got.Unit.VolumeNumber == nil || *got.Unit.VolumeNumber != 2 {
		t.Fatalf("Unit = %#v", got.Unit)
	}
	if got.Unit.DisplayLabel != "Vol.02" {
		t.Fatalf("DisplayLabel = %q", got.Unit.DisplayLabel)
	}
}

func TestResolveSpecialAfterMain(t *testing.T) {
	items := []ResolvedUnit{
		{Kind: UnitKindSpecial, DisplayLabel: "鐗瑰吀", SortKey: SortKey{KindRank: 3, Number: 0, Raw: "鐗瑰吀"}},
		{Kind: UnitKindChapter, DisplayLabel: "绗?02璇?, ChapterNumber: floatPtr(2), SortKey: SortKey{KindRank: 1, Number: 2, Raw: "绗?02璇?}},
		{Kind: UnitKindChapter, DisplayLabel: "绗?01璇?, ChapterNumber: floatPtr(1), SortKey: SortKey{KindRank: 1, Number: 1, Raw: "绗?01璇?}},
	}
	SortUnits(items)
	if items[0].DisplayLabel != "绗?01璇? || items[1].DisplayLabel != "绗?02璇? || items[2].DisplayLabel != "鐗瑰吀" {
		t.Fatalf("order = %#v", items)
	}
}
```

- [ ] **Step 2: Verify tests fail**

```powershell
go test ./internal/workmodel
```

Expected: FAIL because package/functions do not exist.

- [ ] **Step 3: Implement types**

Create `internal/workmodel/types.go`:

```go
package workmodel

type UnitKind string

const (
	UnitKindFull    UnitKind = "full"
	UnitKindChapter UnitKind = "chapter"
	UnitKindVolume  UnitKind = "volume"
	UnitKindExtra   UnitKind = "extra"
	UnitKindSpecial UnitKind = "special"
	UnitKindUnknown UnitKind = "unknown"
)

type SortKey struct {
	KindRank int
	Number   float64
	Raw      string
}

type ResolvedUnit struct {
	Kind          UnitKind
	Title         string
	DisplayLabel  string
	VolumeNumber  *float64
	ChapterNumber *float64
	SortKey       SortKey
}

type ResolvedPath struct {
	RelativePath string
	WorkTitle    string
	Unit         ResolvedUnit
}

func floatPtr(v float64) *float64 { return &v }
```

- [ ] **Step 4: Implement resolver and sorter**

Create `internal/workmodel/resolver.go` and `internal/workmodel/sorter.go` with:

```go
package workmodel

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	chapterPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:绗琝s*)?(\d+(?:\.\d+)?)\s*(?:璇潀瑭?`),
		regexp.MustCompile(`(?i)\bch(?:apter)?[\.\s_-]*(\d+(?:\.\d+)?)\b`),
	}
	volumePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:绗琝s*)?(\d+(?:\.\d+)?)\s*(?:鍗?`),
		regexp.MustCompile(`(?i)\bvol(?:ume)?[\.\s_-]*(\d+(?:\.\d+)?)\b`),
	}
	extraPattern   = regexp.MustCompile(`(?i)(鐣|extra|omake|bonus)`)
	specialPattern = regexp.MustCompile(`(?i)(鐗瑰吀|鍏憡|璇峰亣鏉搴嗗吀|璁惧畾闆唡special)`)
)

func ResolvePath(relativePath string, parentTitle string) ResolvedPath {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	base := strings.TrimSuffix(filepath.Base(clean), "/")
	stem := trimKnownExt(base)
	workTitle := strings.TrimSpace(parentTitle)
	if workTitle == "" {
		workTitle = cleanWorkTitle(stem)
	}
	unit := resolveUnit(stem, workTitle)
	return ResolvedPath{RelativePath: clean, WorkTitle: workTitle, Unit: unit}
}

func resolveUnit(stem, workTitle string) ResolvedUnit {
	labelSource := strings.TrimSpace(strings.TrimPrefix(stem, workTitle))
	labelSource = strings.Trim(labelSource, " -_鈥斺€撀?[]銆愩€?)锛堬級")
	if specialPattern.MatchString(stem) {
		return ResolvedUnit{Kind: UnitKindSpecial, Title: stem, DisplayLabel: firstNonEmpty(labelSource, stem), SortKey: SortKey{KindRank: 3, Raw: stem}}
	}
	if extraPattern.MatchString(stem) {
		return ResolvedUnit{Kind: UnitKindExtra, Title: stem, DisplayLabel: firstNonEmpty(labelSource, stem), SortKey: SortKey{KindRank: 2, Raw: stem}}
	}
	for _, re := range volumePatterns {
		if m := re.FindStringSubmatch(stem); len(m) == 2 {
			n := parseFloat(m[1])
			return ResolvedUnit{Kind: UnitKindVolume, Title: stem, DisplayLabel: firstNonEmpty(labelSource, re.FindString(stem)), VolumeNumber: &n, SortKey: SortKey{KindRank: 1, Number: n, Raw: stem}}
		}
	}
	for _, re := range chapterPatterns {
		if m := re.FindStringSubmatch(stem); len(m) == 2 {
			n := parseFloat(m[1])
			return ResolvedUnit{Kind: UnitKindChapter, Title: stem, DisplayLabel: firstNonEmpty(labelSource, re.FindString(stem)), ChapterNumber: &n, SortKey: SortKey{KindRank: 1, Number: n, Raw: stem}}
		}
	}
	return ResolvedUnit{Kind: UnitKindFull, Title: stem, DisplayLabel: "鍏ㄦ湰", SortKey: SortKey{KindRank: 0, Raw: stem}}
}

func SortUnits(items []ResolvedUnit) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].SortKey, items[j].SortKey
		if a.KindRank != b.KindRank { return a.KindRank < b.KindRank }
		if a.Number != b.Number { return a.Number < b.Number }
		return strings.Compare(a.Raw, b.Raw) < 0
	})
}

func cleanWorkTitle(stem string) string {
	out := stem
	for _, re := range append(volumePatterns, chapterPatterns...) { out = re.ReplaceAllString(out, "") }
	out = extraPattern.ReplaceAllString(out, "")
	out = specialPattern.ReplaceAllString(out, "")
	out = strings.Trim(out, " -_鈥斺€撀?[]銆愩€?)锛堬級")
	if out == "" { return stem }
	return out
}

func trimKnownExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".zip", ".cbz", ".rar", ".cbr", ".7z", ".cb7", ".pdf":
		return strings.TrimSuffix(name, filepath.Ext(name))
	default:
		return name
	}
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil { return 0 }
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" { return strings.TrimSpace(v) }
	}
	return "鍏ㄦ湰"
}
```

If Go reports duplicate packages because sorter has to be separate, move only `SortUnits` and its `sort/strings` imports to `sorter.go`.

- [ ] **Step 5: Verify and commit**

```powershell
go test ./internal/workmodel
git add internal/workmodel
git commit -m "feat: add work structure resolver"
git push fork ymreader-work-model
```

---

## Task 3: Work Detector From Existing Comic Inventory

**Files:**
- Create: `internal/workmodel/detector.go`
- Create: `internal/workmodel/detector_test.go`

- [ ] **Step 1: Write detector tests**

Create `internal/workmodel/detector_test.go`:

```go
package workmodel

import "testing"

func TestDetectSingleArchiveWork(t *testing.T) {
	works := DetectWorks([]SourceItem{{ID: "c1", LibraryID: "lib", RelativePath: "鎭堕瓟X澶╀娇 涓嶈兘鍙嬪ソ鐩稿.zip"}})
	if len(works) != 1 { t.Fatalf("len = %d", len(works)) }
	if works[0].Title != "鎭堕瓟X澶╀娇 涓嶈兘鍙嬪ソ鐩稿" { t.Fatalf("title = %q", works[0].Title) }
	if len(works[0].Units) != 1 || works[0].Units[0].Kind != UnitKindFull { t.Fatalf("units = %#v", works[0].Units) }
}

func TestDetectFolderChaptersAsOneWork(t *testing.T) {
	works := DetectWorks([]SourceItem{
		{ID: "c1", LibraryID: "lib", RelativePath: "澶х帇楗跺懡/绗?02璇?cbz"},
		{ID: "c2", LibraryID: "lib", RelativePath: "澶х帇楗跺懡/绗?01璇?cbz"},
	})
	if len(works) != 1 { t.Fatalf("len = %d", len(works)) }
	if works[0].Title != "澶х帇楗跺懡" { t.Fatalf("title = %q", works[0].Title) }
	if works[0].Units[0].DisplayLabel != "绗?01璇? || works[0].Units[1].DisplayLabel != "绗?02璇? {
		t.Fatalf("order = %#v", works[0].Units)
	}
}

func TestDetectSiblingNamedChaptersAsOneWork(t *testing.T) {
	works := DetectWorks([]SourceItem{
		{ID: "c1", LibraryID: "lib", RelativePath: "澶х帇楗跺懡 绗?01璇?cbz"},
		{ID: "c2", LibraryID: "lib", RelativePath: "澶х帇楗跺懡 绗?02璇?cbz"},
	})
	if len(works) != 1 { t.Fatalf("len = %d works=%#v", len(works), works) }
	if works[0].Title != "澶х帇楗跺懡" { t.Fatalf("title = %q", works[0].Title) }
}
```

- [ ] **Step 2: Verify failure**

```powershell
go test ./internal/workmodel
```

- [ ] **Step 3: Implement detector**

Create `internal/workmodel/detector.go`:

```go
package workmodel

import (
	"crypto/md5"
	"encoding/hex"
	"path"
	"sort"
	"strings"
)

type SourceItem struct {
	ID           string
	LibraryID    string
	RelativePath string
	Title        string
	FileSize     int64
	PageCount    int
}

type DetectedUnit struct {
	ID            string
	ComicID       string
	RelativePath  string
	Kind          UnitKind
	Title         string
	DisplayLabel  string
	VolumeNumber  *float64
	ChapterNumber *float64
	SortIndex     int
	PageCount     int
	FileSize      int64
	SortKey       SortKey
}

type DetectedWork struct {
	ID               string
	LibraryID        string
	RootRelativePath string
	Title            string
	SortTitle        string
	Units            []DetectedUnit
}

func DetectWorks(items []SourceItem) []DetectedWork {
	groups := map[string]*DetectedWork{}
	for _, item := range items {
		rel := strings.ReplaceAll(item.RelativePath, "\\", "/")
		parent := path.Dir(rel)
		if parent == "." { parent = "" }
		parentTitle := ""
		if parent != "" { parentTitle = path.Base(parent) }
		resolved := ResolvePath(rel, parentTitle)
		root := parent
		if root == "" && resolved.Unit.Kind != UnitKindFull { root = resolved.WorkTitle }
		if root == "" { root = trimKnownExt(path.Base(rel)) }
		key := item.LibraryID + "\x00" + root
		work := groups[key]
		if work == nil {
			work = &DetectedWork{ID: stableID("work", item.LibraryID, root), LibraryID: item.LibraryID, RootRelativePath: root, Title: resolved.WorkTitle, SortTitle: strings.ToLower(resolved.WorkTitle)}
			groups[key] = work
		}
		work.Units = append(work.Units, DetectedUnit{ID: stableID("unit", item.LibraryID, rel), ComicID: item.ID, RelativePath: rel, Kind: resolved.Unit.Kind, Title: resolved.Unit.Title, DisplayLabel: resolved.Unit.DisplayLabel, VolumeNumber: resolved.Unit.VolumeNumber, ChapterNumber: resolved.Unit.ChapterNumber, SortIndex: 0, PageCount: item.PageCount, FileSize: item.FileSize, SortKey: resolved.Unit.SortKey})
	}
	works := make([]DetectedWork, 0, len(groups))
	for _, work := range groups {
		sort.SliceStable(work.Units, func(i, j int) bool {
			a, b := work.Units[i].SortKey, work.Units[j].SortKey
			if a.KindRank != b.KindRank { return a.KindRank < b.KindRank }
			if a.Number != b.Number { return a.Number < b.Number }
			return a.Raw < b.Raw
		})
		for i := range work.Units { work.Units[i].SortIndex = i }
		works = append(works, *work)
	}
	sort.SliceStable(works, func(i, j int) bool { return works[i].SortTitle < works[j].SortTitle })
	return works
}

func stableID(prefix, libraryID, value string) string {
	sum := md5.Sum([]byte(libraryID + "\x00" + strings.ReplaceAll(value, "\\", "/")))
	return prefix + "_" + hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Verify and commit**

```powershell
go test ./internal/workmodel
git add internal/workmodel
git commit -m "feat: detect works from comic paths"
git push fork ymreader-work-model
```

---

## Task 4: Database Schema and Store

**Files:**
- Modify: `internal/model/models.go`
- Create: `internal/store/migrate_work_model.go`
- Create: `internal/store/work_store.go`
- Create: `internal/store/work_store_test.go`

- [ ] **Step 1: Add model structs**

Append to `internal/model/models.go`:

```go
type Work struct {
	ID               string     `json:"id"`
	LibraryID        string     `json:"libraryId"`
	RootRelativePath string     `json:"rootRelativePath"`
	Title            string     `json:"title"`
	SortTitle        string     `json:"sortTitle"`
	CoverURL         string     `json:"coverUrl"`
	CoverUnitID      string     `json:"coverUnitId"`
	Author           string     `json:"author"`
	Publisher        string     `json:"publisher"`
	Year             *int       `json:"year"`
	Description      string     `json:"description"`
	Language         string     `json:"language"`
	Genre            string     `json:"genre"`
	MetadataSource   string     `json:"metadataSource"`
	ContentType      string     `json:"contentType"`
	MetadataLocked   bool       `json:"metadataLocked"`
	ManualLocked     bool       `json:"manualLocked"`
	ItemCount        int        `json:"itemCount,omitempty"`
	LastReadAt       *time.Time `json:"lastReadAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type WorkUnit struct {
	ID            string   `json:"id"`
	WorkID        string   `json:"workId"`
	ComicID       string   `json:"comicId"`
	RelativePath  string   `json:"relativePath"`
	Title         string   `json:"title"`
	DisplayLabel  string   `json:"displayLabel"`
	UnitKind      string   `json:"unitKind"`
	VolumeNumber  *float64 `json:"volumeNumber"`
	ChapterNumber *float64 `json:"chapterNumber"`
	SortIndex     int      `json:"sortIndex"`
	PageCount     int      `json:"pageCount"`
	FileSize      int64    `json:"fileSize"`
	CoverURL      string   `json:"coverUrl"`
}

type UserWorkProgress struct {
	UserID    string     `json:"userId"`
	WorkID    string     `json:"workId"`
	UnitID    string     `json:"unitId"`
	PageIndex int        `json:"pageIndex"`
	UpdatedAt *time.Time `json:"updatedAt"`
}
```

- [ ] **Step 2: Add migration**

Create `internal/store/migrate_work_model.go` with migration version 34 creating tables `Work`, `WorkUnit`, `UserWorkProgress`, indexes `Work_library_sort_idx`, `WorkUnit_work_sort_idx`, and `WorkUnit_comic_idx`. Use the SQL from the approved design document section 12 with `ON DELETE CASCADE` foreign keys to `Library`, `Work`, and `WorkUnit`.

- [ ] **Step 3: Write store tests**

Create `internal/store/work_store_test.go` with one test that:
1. Creates test DB using the same helper pattern as `series_store_test.go`.
2. Inserts a comic library.
3. Calls `ReplaceWorksForLibrary` with one `workmodel.DetectedWork`.
4. Calls `ListWorks`.
5. Asserts title `澶х帇楗跺懡` and `ItemCount == 1`.

- [ ] **Step 4: Implement store functions**

Create `internal/store/work_store.go` exporting:

```go
func ReplaceWorksForLibrary(libraryID string, detected []workmodel.DetectedWork) error
func ListWorks(libraryIDs []string, userID string, search string) ([]model.Work, error)
func GetWorkDetail(workID string, userID string) (*model.Work, []model.WorkUnit, error)
func GetFirstWorkUnit(workID string) (*model.WorkUnit, error)
func GetWorkUnit(unitID string) (*model.WorkUnit, error)
func GetAdjacentWorkUnit(workID string, sortIndex int, delta int) (*model.WorkUnit, error)
func SetUserWorkProgress(userID, workID, unitID string, pageIndex int) error
func GetUserWorkProgress(userID, workID string) (*model.UserWorkProgress, error)
func GetWorkUnitByComicID(comicID string) (*model.WorkUnit, error)
func ListWorkSourceItems(libraryID string) ([]WorkSourceItem, error)
```

Use a transaction for `ReplaceWorksForLibrary`; delete unlocked Works for the target library; preserve rows with `manualLocked=1`; preserve metadata when `metadataLocked=1`; upsert units by `(workId, relativePath)`.

- [ ] **Step 5: Verify and commit**

```powershell
go test ./internal/store -run 'TestReplaceWorks|TestMigrations' -v
git add internal/model/models.go internal/store/migrate_work_model.go internal/store/work_store.go internal/store/work_store_test.go
git commit -m "feat: persist manga work model"
git push fork ymreader-work-model
```

---

## Task 5: Work Service and Scan Hook

**Files:**
- Create: `internal/service/work_service.go`
- Create: `internal/service/work_service_test.go`
- Modify: `internal/handler/routes_comics.go`

- [ ] **Step 1: Write service test**

Create `internal/service/work_service_test.go` using the DB setup pattern from `scanner_ownership_test.go`. Insert library `lib_work` and Comic rows `澶х帇楗跺懡/绗?02璇?cbz`, `澶х帇楗跺懡/绗?01璇?cbz`; call `RebuildWorksForLibrary("lib_work")`; assert Work title and sorted units.

- [ ] **Step 2: Implement service**

Create `internal/service/work_service.go`:

```go
package service

import (
	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func RebuildAllWorks() error {
	libs, err := store.GetScannableLibraries()
	if err != nil { return err }
	for _, lib := range libs {
		if lib.Type == "novel" { continue }
		if err := RebuildWorksForLibrary(lib.ID); err != nil { return err }
	}
	return nil
}

func RebuildWorksForLibrary(libraryID string) error {
	items, err := store.ListWorkSourceItems(libraryID)
	if err != nil { return err }
	source := make([]workmodel.SourceItem, 0, len(items))
	for _, item := range items {
		source = append(source, workmodel.SourceItem{ID: item.ID, LibraryID: item.LibraryID, RelativePath: item.RelativePath, Title: item.Title, FileSize: item.FileSize, PageCount: item.PageCount})
	}
	return store.ReplaceWorksForLibrary(libraryID, workmodel.DetectWorks(source))
}
```

- [ ] **Step 3: Hook after sync**

In `internal/handler/routes_comics.go`, add `rebuildWorksAfterScan()` next to `rebuildSeriesAfterScan()`:

```go
func rebuildWorksAfterScan() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 400 { return }
		if err := service.RebuildAllWorks(); err != nil {
			log.Printf("[works] rebuild after scan failed: %v", err)
		}
	}
}
```

Register:

```go
syncTrigger.POST("/sync", reconcileOwnershipAfterScan(), rebuildSeriesAfterScan(), rebuildWorksAfterScan(), comic.TriggerSync)
```

- [ ] **Step 4: Verify and commit**

```powershell
go test ./internal/service ./internal/handler -run 'TestRebuildWorks|Test.*Sync' -v
git add internal/service/work_service.go internal/service/work_service_test.go internal/store/work_store.go internal/handler/routes_comics.go
git commit -m "feat: rebuild works after library scan"
git push fork ymreader-work-model
```

---

## Task 6: Work HTTP API

**Files:**
- Create: `internal/handler/work_handler.go`
- Create: `internal/handler/routes_works.go`
- Modify: `internal/handler/router.go`
- Create: `internal/handler/work_handler_test.go`
- Modify: `docs/API.md`

- [ ] **Step 1: Add failing handler tests**

Create tests for:
- `GET /api/works`
- `GET /api/works/:id`
- `GET /api/works/:id/continue`
- `PUT /api/works/:id/progress`
- `GET /api/works/:id/units/:unitId/adjacent`

Use existing authenticated request helpers from `internal/handler/handler_test.go`.

- [ ] **Step 2: Implement routes**

Create `internal/handler/routes_works.go`:

```go
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/middleware"
)

func registerWorkRoutes(api *gin.RouterGroup) {
	h := NewWorkHandler()
	works := api.Group("/works")
	works.Use(middleware.AuthRequired())
	{
		works.GET("", h.List)
		works.GET("/:id", h.Get)
		works.GET("/:id/continue", h.Continue)
		works.PUT("/:id/progress", h.UpdateProgress)
		works.GET("/:id/units/:unitId/adjacent", h.Adjacent)
	}
	worksAdmin := api.Group("/works")
	worksAdmin.Use(middleware.AdminRequired())
	{
		worksAdmin.POST("/rebuild", h.Rebuild)
	}
}
```

Modify `internal/handler/router.go` to call `registerWorkRoutes(api)`.

- [ ] **Step 3: Implement handler behavior**

Create `internal/handler/work_handler.go` with methods:

```go
func NewWorkHandler() *WorkHandler
func (h *WorkHandler) List(c *gin.Context)
func (h *WorkHandler) Get(c *gin.Context)
func (h *WorkHandler) Continue(c *gin.Context)
func (h *WorkHandler) UpdateProgress(c *gin.Context)
func (h *WorkHandler) Adjacent(c *gin.Context)
func (h *WorkHandler) Rebuild(c *gin.Context)
```

Response shapes:

```json
{ "works": [] }
{ "work": {}, "units": [] }
{ "workId": "...", "unitId": "...", "comicId": "...", "pageIndex": 0 }
{ "previous": null, "next": { "unitId": "...", "comicId": "...", "pageIndex": 0 } }
{ "success": true }
```

- [ ] **Step 4: Document API**

Add `浣滃搧 Work API` to `docs/API.md` with endpoint list and the response shapes above.

- [ ] **Step 5: Verify and commit**

```powershell
go test ./internal/handler -run TestWork -v
go test ./internal/...
git add internal/handler/work_handler.go internal/handler/routes_works.go internal/handler/router.go internal/handler/work_handler_test.go docs/API.md
git commit -m "feat: expose manga work API"
git push fork ymreader-work-model
```

---

## Task 7: Web Work Library, Detail, and Reader Context

**Files:**
- Create: `frontend/src/types/work.ts`
- Create: `frontend/src/api/works.ts`
- Create: `frontend/src/components/WorkCard.tsx`
- Create: `frontend/src/app/work/[id]/page.tsx`
- Modify: `frontend/src/app/books/page.tsx`
- Modify: `frontend/src/app/reader/[id]/page.tsx`

- [ ] **Step 1: Add web types**

Create `frontend/src/types/work.ts`:

```ts
export type Work = {
  id: string
  libraryId: string
  rootRelativePath: string
  title: string
  sortTitle: string
  coverUrl: string
  coverUnitId: string
  author: string
  publisher: string
  year?: number | null
  description: string
  language: string
  genre: string
  metadataSource: string
  contentType: string
  itemCount?: number
  lastReadAt?: string | null
}

export type WorkUnit = {
  id: string
  workId: string
  comicId: string
  relativePath: string
  title: string
  displayLabel: string
  unitKind: 'full' | 'chapter' | 'volume' | 'extra' | 'special' | 'unknown'
  volumeNumber?: number | null
  chapterNumber?: number | null
  sortIndex: number
  pageCount: number
  fileSize: number
  coverUrl: string
}

export type WorkDetail = { work: Work; units: WorkUnit[] }
export type WorkContinue = { workId: string; unitId: string; comicId: string; pageIndex: number }
```

- [ ] **Step 2: Add web API**

Create `frontend/src/api/works.ts` using the existing `apiClient` style in `frontend/src/api/comics.ts`:

```ts
export async function listWorks(params?: { libraryId?: string; search?: string }): Promise<{ works: Work[] }>
export async function getWork(id: string): Promise<WorkDetail>
export async function getWorkContinue(id: string): Promise<WorkContinue>
export async function updateWorkProgress(id: string, body: { unitId: string; pageIndex: number }): Promise<{ success: true }>
export async function getWorkAdjacent(id: string, unitId: string): Promise<{ previous: WorkContinue | null; next: WorkContinue | null }>
```

- [ ] **Step 3: Add Work card**

Create `frontend/src/components/WorkCard.tsx` with an `object-contain` cover image and link to `/work/${work.id}`. Do not crop covers to square.

- [ ] **Step 4: Add detail page**

Create `frontend/src/app/work/[id]/page.tsx` showing cover, metadata, `绔嬪嵆闃呰 / 缁х画闃呰`, and sorted Unit table of contents. Unit links must include:

```text
/reader/{comicId}?workId={workId}&unitId={unitId}&page=0
```

- [ ] **Step 5: Switch books page to Work cards**

Modify `frontend/src/app/books/page.tsx` so comic libraries use `/api/works`. Keep old Comic path behind:

```ts
const useWorkModel = true
```

Do not delete old code in this task.

- [ ] **Step 6: Update reader context**

Modify `frontend/src/app/reader/[id]/page.tsx` to read `workId` and `unitId` from query params. On page changes, update both old Comic progress and new Work progress. Add previous/next/directory buttons only when Work context exists.

- [ ] **Step 7: Verify and commit**

```powershell
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13\frontend'
npm run build
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13'
git add frontend/src/types/work.ts frontend/src/api/works.ts frontend/src/components/WorkCard.tsx frontend/src/app/work frontend/src/app/books/page.tsx frontend/src/app/reader
git commit -m "feat(web): browse and read manga works"
git push fork ymreader-work-model
```

---

## Task 8: Work-Based OPDS

**Files:**
- Modify: `internal/service/opds.go`
- Modify: `internal/handler/opds_handler.go`
- Modify: `internal/handler/opds_handler_test.go`
- Modify: `docs/API.md`

- [ ] **Step 1: Add failing OPDS test**

Add `TestOPDSWorks` asserting:
- `/api/opds/works` contains one navigation entry for `澶х帇楗跺懡`.
- `/api/opds/works/{id}` contains acquisition entries for `绗?01璇漙 and `绗?02璇漙.

- [ ] **Step 2: Implement service methods**

Add:

```go
func GenerateWorksOPDSFeed(baseURL, userID string) ([]byte, error)
func GenerateWorkUnitsOPDSFeed(baseURL, userID, workID string) ([]byte, error)
```

Works feed links to work unit feeds. Unit feed links to existing Comic acquisition/download endpoints. Keep `/api/opds/all` and `/api/opds/series`.

- [ ] **Step 3: Add handler routes**

Add:

```go
opds.GET("/works", h.Works)
opds.GET("/works/:id", h.WorkUnits)
```

- [ ] **Step 4: Verify and commit**

```powershell
go test ./internal/handler -run TestOPDS -v
go test ./internal/service -run TestOPDS -v
git add internal/service/opds.go internal/handler/opds_handler.go internal/handler/opds_handler_test.go docs/API.md
git commit -m "feat: add work-based OPDS catalog"
git push fork ymreader-work-model
```

---

## Task 9: Flutter Android Work Integration

**Files:**
- Create: `flutter_app/lib/data/models/work.dart`
- Create: `flutter_app/lib/data/api/work_api.dart`
- Modify: `flutter_app/lib/features/home/home_screen.dart`
- Create: `flutter_app/lib/features/detail/work_detail_screen.dart`
- Modify: `flutter_app/lib/features/reader/reader_screen.dart`

- [ ] **Step 1: Add Dart models**

Create `Work`, `WorkUnit`, `WorkDetail`, and `WorkContinue` with `fromJson(Map<String, dynamic>)`. Field names must match backend JSON: `workId`, `unitId`, `comicId`, `pageIndex`, `displayLabel`, `unitKind`.

- [ ] **Step 2: Add Work API client**

Create methods:

```dart
Future<List<Work>> listWorks({String? libraryId, String? search})
Future<WorkDetail> getWork(String id)
Future<WorkContinue> getContinue(String id)
Future<void> updateProgress(String id, String unitId, int pageIndex)
Future<AdjacentWorkUnits> getAdjacent(String id, String unitId)
```

Use the existing API client pattern under `flutter_app/lib/data/api`.

- [ ] **Step 3: Update home**

Modify `home_screen.dart` to show Works for comic libraries. Preserve current visual card style.

- [ ] **Step 4: Add Work detail screen**

Display cover, title, description, continue button, and Unit table of contents. Unit tap opens reader with `comicId`, `workId`, `unitId`, and page.

- [ ] **Step 5: Update reader**

Reader must:
- Accept optional Work context.
- Update Work progress on page change.
- Show previous/next buttons when Work context exists.
- Open directory by returning to Work detail and positioning current Unit.

- [ ] **Step 6: Verify and commit**

```powershell
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13\flutter_app'
flutter analyze
flutter build apk --release
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13'
git add flutter_app/lib/data/models/work.dart flutter_app/lib/data/api/work_api.dart flutter_app/lib/features/home/home_screen.dart flutter_app/lib/features/detail/work_detail_screen.dart flutter_app/lib/features/reader/reader_screen.dart
git commit -m "feat(android): use work model for manga library"
git push fork ymreader-work-model
```

---

## Task 10: End-to-End Verification

**Files:**
- Create: `docs/superpowers/test-results/2026-08-05-ymreader-work-model-e2e.md`

- [ ] **Step 1: Prepare fixture library**

```powershell
$root = 'F:\meihua\.tmp\ymreader-fixture-library'
Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path "$root\澶х帇楗跺懡" | Out-Null
New-Item -ItemType Directory -Force -Path "$root\璐ョ姮濂充富澶浜? | Out-Null
```

Create tiny CBZ fixtures with two generated images per archive; do not use the real NAS library for verification.

- [ ] **Step 2: Run server and scan fixture**

```powershell
Set-Location -LiteralPath 'F:\meihua\nowen-reader-official-v1.0.13'
go run ./cmd/server
```

Add the fixture path as a comic library through UI/API, then trigger `/api/sync`.

- [ ] **Step 3: Verify required cases**

Expected:
- `鎭堕瓟X澶╀娇 涓嶈兘鍙嬪ソ鐩稿.zip` appears as one Work with detail page and `鍏ㄦ湰`.
- `澶х帇楗跺懡` appears as one Work with `绗?01璇漙, `绗?02璇漙.
- `璐ョ姮濂充富澶浜哷 appears as one Work with `Vol.01`, `Vol.02`.
- Continue reading stores Unit + Page.
- Previous/next Unit works.
- `/api/opds/works` does not flatten chapters.

- [ ] **Step 4: Record results**

Create `docs/superpowers/test-results/2026-08-05-ymreader-work-model-e2e.md`:

```markdown
# YmReader Work Model E2E Results

## Environment
- Date: 2026-08-05
- Branch: ymreader-work-model

## Cases
- Single archive: PASS/FAIL with notes
- Chapter folder: PASS/FAIL with notes
- Volume folder: PASS/FAIL with notes
- Continue reading: PASS/FAIL with notes
- Previous/next Unit: PASS/FAIL with notes
- OPDS works: PASS/FAIL with notes

## Defects Fixed During E2E
- List exact commits if fixes were needed.
```

Use actual PASS/FAIL values from the run.

- [ ] **Step 5: Commit verification**

```powershell
git add docs/superpowers/test-results internal frontend flutter_app
git commit -m "test: verify ymreader work model e2e"
git push fork ymreader-work-model
```

---

## Self-Review

### Spec coverage

- Single archive as Work: Tasks 2, 3, 7, 10.
- Multi-chapter folder: Tasks 2, 3, 10.
- Multi-volume folder: Tasks 2, 3, 10.
- Mixed layout support: Tasks 2 and 3 establish parsing/sorting; first UI displays flattened WorkUnit list with preserved volume/chapter numbers.
- Work/Unit/Page model: Tasks 4 and 5.
- Work progress by unit and page: Tasks 4, 6, 7, 9.
- Web library/detail: Task 7.
- Reader previous/next: Task 7.
- OPDS works mode: Task 8.
- Android client: Task 9.
- Avoid unrelated features: every task scopes changes to Work-related files only.
- Upstream sync: Task 1.

### Placeholder scan

The plan contains no placeholder markers. Steps include concrete files, function names, commands, and expected results. Test helper references point to exact existing files to copy setup patterns from.

### Type consistency

The plan consistently uses `Work`, `WorkUnit`, `UserWorkProgress`, `DetectedWork`, `DetectedUnit`, `SourceItem`, `UnitKind`, `workId`, `unitId`, and `comicId` across backend, web, and Flutter.

