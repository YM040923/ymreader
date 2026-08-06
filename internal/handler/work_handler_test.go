package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestWorkRoutesRequireAuthenticationIncludingUnits(t *testing.T) {
	r := setupTestRouter(t)
	for _, path := range []string{"/api/works", "/api/works/work-id", "/api/works/work-id/units"} {
		response := performRequest(r, http.MethodGet, path, nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s status=%d, want 401", path, response.Code)
		}
	}
}

func TestWorkAPIListsFullWorkThenReturnsDetailAndUnits(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-main", "Main", "private")
	createWorkTestComics(t, "lib-main", []workTestComic{
		{ID: "chapter-10", Path: "恶魔X天使 不能友好相处/第010话/", Title: "第010话"},
		{ID: "chapter-2", Path: "恶魔X天使 不能友好相处/第002话/", Title: "第002话"},
		{ID: "chapter-1", Path: "恶魔X天使 不能友好相处/第001话/", Title: "第001话"},
	})

	response := performAuthedRequest(r, http.MethodGet, "/api/works?page=1&pageSize=1", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var list struct {
		Works      []service.Work `json:"works"`
		Total      int            `json:"total"`
		Page       int            `json:"page"`
		PageSize   int            `json:"pageSize"`
		TotalPages int            `json:"totalPages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Works) != 1 || list.TotalPages != 1 {
		t.Fatalf("unexpected list response: %#v", list)
	}
	work := list.Works[0]
	if work.ItemCount != 3 || len(work.Units) != 3 {
		t.Fatalf("list work is incomplete: %#v", work)
	}

	response = performAuthedRequest(r, http.MethodGet, "/api/works/"+work.ID, nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", response.Code, response.Body.String())
	}
	var detail service.Work
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ID != work.ID || len(detail.Units) != 3 {
		t.Fatalf("unexpected detail: %#v", detail)
	}

	response = performAuthedRequest(r, http.MethodGet, "/api/works/"+work.ID+"/units", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("units status=%d body=%s", response.Code, response.Body.String())
	}
	var units struct {
		WorkID string             `json:"workId"`
		Units  []service.WorkUnit `json:"units"`
		Total  int                `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &units); err != nil {
		t.Fatal(err)
	}
	if units.WorkID != work.ID || units.Total != 3 || len(units.Units) != 3 {
		t.Fatalf("unexpected units response: %#v", units)
	}
}

func TestWorkReadEndpointsDoNotWriteLogicalWorkTables(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-read-only-work", "Read only", "private")
	createWorkTestComics(t, "lib-read-only-work", []workTestComic{
		{ID: "read-only-1", Path: "只读作品/第001话", Title: "第001话"},
		{ID: "read-only-2", Path: "只读作品/第002话", Title: "第002话"},
	})

	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"lib-read-only-work"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("works=%#v", works)
	}
	workID := works[0].ID

	for _, table := range []string{"LogicalWork", "LogicalWorkTag", "LogicalWorkCategory"} {
		trigger := `CREATE TRIGGER "deny_` + table + `_insert" BEFORE INSERT ON "` + table + `" BEGIN SELECT RAISE(ABORT, 'read endpoint attempted write'); END`
		if _, err := store.DB().Exec(trigger); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB().Exec(`
		CREATE TRIGGER "deny_LogicalWork_update"
		BEFORE UPDATE ON "LogicalWork"
		BEGIN SELECT RAISE(ABORT, 'read endpoint attempted write'); END
	`); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	for _, path := range []string{
		"/api/works",
		"/api/works/" + workID,
		"/api/works/" + workID + "/units",
	} {
		response := performAuthedRequest(r, http.MethodGet, path, nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestWorkAPIFilterKeepsEveryUnitOfMatchingWork(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-filter", "Filter", "private")
	createWorkTestComics(t, "lib-filter", []workTestComic{
		{ID: "filter-1", Path: "大王饶命/第001话/", Title: "开学第一天"},
		{ID: "filter-2", Path: "大王饶命/第002话/", Title: "灵气复苏"},
	})

	response := performAuthedRequest(r, http.MethodGet, "/api/works?search=灵气复苏", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var list struct {
		Works []service.Work `json:"works"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Works) != 1 || len(list.Works[0].Units) != 2 {
		t.Fatalf("filter truncated the matching work: %#v", list)
	}
}

func TestWorkAPIEnforcesLibraryPermissionsAndRequestedLibraryIntersection(t *testing.T) {
	r := setupTestRouter(t)
	_ = registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-allowed", "Allowed", "private")
	createWorkTestLibrary(t, "lib-secret", "Secret", "private")
	createWorkTestComics(t, "lib-allowed", []workTestComic{{ID: "allowed-1", Path: "可见漫画/第001话/", Title: "第001话"}})
	createWorkTestComics(t, "lib-secret", []workTestComic{{ID: "secret-1", Path: "隐藏漫画/第001话/", Title: "第001话"}})

	userCookie := createWorkTestUserSession(t, "reader", "reader-session")
	if err := store.SetUserLibraryAccess("reader", []store.LibraryAccessReq{{LibraryID: "lib-allowed", CanView: true}}); err != nil {
		t.Fatal(err)
	}

	response := performAuthedRequest(r, http.MethodGet, "/api/works", nil, userCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var list struct {
		Works []service.Work `json:"works"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Works) != 1 || list.Works[0].LibraryID != "lib-allowed" {
		t.Fatalf("permission leak or missing work: %#v", list)
	}

	response = performAuthedRequest(r, http.MethodGet, "/api/works?libraryIds=lib-secret", nil, userCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("requested inaccessible library status=%d body=%s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 0 || len(list.Works) != 0 {
		t.Fatalf("inaccessible requested library leaked works: %#v", list)
	}

	secretID := service.BuildWorksFromComicList([]store.ComicListItem{{
		ID: "secret-1", Title: "第001话", Filename: "隐藏漫画/第001话/",
		LibraryID: "lib-secret", RelativePath: "隐藏漫画/第001话/",
	}}, service.WorkBuildOptions{})[0].ID
	response = performAuthedRequest(r, http.MethodGet, "/api/works/"+secretID, nil, userCookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("inaccessible detail status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestFilterWorksUsesAggregatedTagsAndCategories(t *testing.T) {
	works := []service.Work{
		{
			ID: "a", Title: "国漫作品",
			Tags:       []store.ComicTagInfo{{Name: "热血"}},
			Categories: []store.ComicCategoryInfo{{Name: "国漫", Slug: "cn"}},
		},
		{ID: "b", Title: "其他作品", Tags: []store.ComicTagInfo{}, Categories: []store.ComicCategoryInfo{}},
	}
	filtered := filterWorks(works, workFilters{tags: []string{"热血"}, category: "cn"})
	if len(filtered) != 1 || filtered[0].ID != "a" {
		t.Fatalf("filtered=%#v", filtered)
	}
	filtered = filterWorks(works, workFilters{untagged: true, uncategorized: true})
	if len(filtered) != 1 || filtered[0].ID != "b" {
		t.Fatalf("untagged/uncategorized=%#v", filtered)
	}
}

func TestSortWorksPreservesAddedUpdatedAndCustomModes(t *testing.T) {
	source := []service.Work{
		{ID: "a", Title: "A", AddedAt: "2025-02-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", SortOrder: 20},
		{ID: "b", Title: "B", AddedAt: "2025-01-01T00:00:00Z", UpdatedAt: "2026-03-01T00:00:00Z", SortOrder: 10},
	}
	tests := []struct {
		sortBy string
		want   []string
	}{
		{sortBy: "addedAt", want: []string{"b", "a"}},
		{sortBy: "updatedAt", want: []string{"a", "b"}},
		{sortBy: "custom", want: []string{"b", "a"}},
	}
	for _, test := range tests {
		works := append([]service.Work(nil), source...)
		sortWorks(works, test.sortBy, "asc")
		got := []string{works[0].ID, works[1].ID}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("sortBy=%s got=%#v want=%#v", test.sortBy, got, test.want)
		}
	}
}

func TestSortWorksOrdersChineseNumberedTitlesNaturally(t *testing.T) {
	works := []service.Work{
		{ID: "10", Title: "作品十"},
		{ID: "2", Title: "作品二"},
		{ID: "11", Title: "作品十一"},
		{ID: "1", Title: "作品一"},
	}
	sortWorks(works, "title", "asc")
	got := []string{works[0].ID, works[1].ID, works[2].ID, works[3].ID}
	want := []string{"1", "2", "10", "11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestComicsSeriesViewReturnsUnifiedWorkDTO(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-series-view-work", "Series View Work", "private")
	createWorkTestComics(t, "lib-series-view-work", []workTestComic{
		{ID: "series-view-work-1", Path: "作品/第一话/", Title: "第一话"},
		{ID: "series-view-work-2", Path: "作品/第二话/", Title: "第二话"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"lib-series-view-work"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()

	response := performAuthedRequest(r, http.MethodGet, "/api/comics?seriesView=true", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Comics []service.Work `json:"comics"`
		Total  int            `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 1 || len(payload.Comics) != 1 || len(payload.Comics[0].Units) != 2 ||
		payload.Comics[0].MetadataHostType != "work" ||
		payload.Comics[0].MetadataHostID != payload.Comics[0].ID ||
		!strings.HasPrefix(payload.Comics[0].ID, "work_") {
		t.Fatalf("seriesView did not return unified Work DTO: %s", response.Body.String())
	}
}

func TestWorkCatalogCacheReusesFingerprintAndIndexesByID(t *testing.T) {
	cache := newWorkCatalogCache()
	loads := 0
	loader := func() ([]service.Work, error) {
		loads++
		return []service.Work{{ID: "work-1", Title: "作品"}}, nil
	}
	first, err := cache.get("reader:lib", "fingerprint-1", loader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get("reader:lib", "fingerprint-1", loader)
	if err != nil {
		t.Fatal(err)
	}
	if loads != 1 || first.ByID["work-1"] == nil || second.ByID["work-1"] == nil {
		t.Fatalf("cache did not reuse/index catalog: loads=%d first=%#v second=%#v", loads, first, second)
	}
	if _, err := cache.get("reader:lib", "fingerprint-2", loader); err != nil {
		t.Fatal(err)
	}
	if loads != 2 {
		t.Fatalf("fingerprint change did not invalidate cache: loads=%d", loads)
	}
}

func TestWorkAPIUsesPersistedSeriesMetadataFromDatabase(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	resetWorkCatalogCache()
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "lib-series-real", "Series", "private")
	createWorkTestComics(t, "lib-series-real", []workTestComic{
		{ID: "series-real-1", Path: "真实作品/第一话/", Title: "第一话"},
		{ID: "series-real-2", Path: "真实作品/第二话/", Title: "第二话"},
	})
	if err := service.RebuildComicSeriesForLibrary("lib-series-real"); err != nil {
		t.Fatal(err)
	}
	summaries, err := store.ListSeriesSummaries([]string{"lib-series-real"}, "", "")
	if err != nil || len(summaries) != 1 {
		t.Fatalf("series=%#v err=%v", summaries, err)
	}
	title := "数据库中的刮削标题"
	author := "数据库作者"
	rating := 9.4
	ratingMax := 10.0
	ratingSource := "bangumi"
	if err := store.UpdateSeriesMetadata(summaries[0].ID, store.SeriesMetadataUpdate{
		Title: &title, Author: &author, ExternalRating: &rating,
		ExternalRatingMax: &ratingMax, ExternalRatingSource: &ratingSource,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateComicSeriesToLogicalWorks(); err != nil {
		t.Fatal(err)
	}

	response := performAuthedRequest(r, http.MethodGet, "/api/works", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var list struct {
		Works []service.Work `json:"works"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Works) != 1 {
		t.Fatalf("works=%#v", list.Works)
	}
	work := list.Works[0]
	if work.Title != title || work.Author != author || work.ExternalRating == nil || *work.ExternalRating != rating ||
		work.MetadataHostType != "work" || work.MetadataHostID != work.ID || work.SeriesID != "" {
		t.Fatalf("persisted series metadata was not integrated: %#v", work)
	}
	persisted, err := store.GetLogicalWork(work.ID)
	if err != nil || persisted == nil || persisted.Title != title || persisted.Author != author {
		t.Fatalf("ComicSeries metadata was not migrated to LogicalWork: work=%#v err=%v", persisted, err)
	}
}

type workTestComic struct {
	ID    string
	Path  string
	Title string
}

func createWorkTestLibrary(t *testing.T, id, name, defaultAccess string) {
	t.Helper()
	if err := store.CreateLibrary(&model.Library{
		ID: id, Name: name, Type: "comic", RootPath: "/test/" + id,
		Enabled: true, ScanEnabled: true, DefaultAccess: defaultAccess,
	}); err != nil {
		t.Fatal(err)
	}
}

func createWorkTestComics(t *testing.T, libraryID string, comics []workTestComic) {
	t.Helper()
	rows := make([]struct {
		ID       string
		Filename string
		Title    string
		FileSize int64
	}, len(comics))
	source := make(map[string]string, len(comics))
	libraries := make(map[string]string, len(comics))
	for i, comic := range comics {
		rows[i] = struct {
			ID       string
			Filename string
			Title    string
			FileSize int64
		}{ID: comic.ID, Filename: comic.Path, Title: comic.Title, FileSize: 100}
		source[comic.ID] = "comics"
		libraries[comic.ID] = libraryID
	}
	if err := store.BulkCreateComicsWithSource(rows, source, libraries); err != nil {
		t.Fatal(err)
	}
}

func createWorkTestUserSession(t *testing.T, userID, token string) string {
	t.Helper()
	if err := store.CreateUser(&model.User{
		ID: userID, Username: userID, Password: "not-used", Nickname: userID, Role: "user",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(&model.UserSession{
		ID: token, UserID: userID, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	return token
}
