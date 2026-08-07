package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestAdminLibraryCountsExposeWorksUnitsAndFilesSeparately(t *testing.T) {
	r := setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	cookie := registerAndLogin(t, r)
	createWorkTestLibrary(t, "library-counts-work", "Library counts", "private")
	createWorkTestComics(t, "library-counts-work", []workTestComic{
		{ID: "library-counts-1", Path: "作品/第001话.cbz", Title: "第001话"},
		{ID: "library-counts-2", Path: "作品/第002话.cbz", Title: "第002话"},
	})
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{"library-counts-work"}, FilterLibraryIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	works := service.BuildWorksFromComicList(result.Comics, service.WorkBuildOptions{})
	if err := service.PersistAndApplyLogicalWorks(works); err != nil {
		t.Fatal(err)
	}

	response := performAuthedRequest(r, http.MethodGet, "/api/admin/libraries", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Libraries []struct {
			ID         string `json:"id"`
			ComicCount int    `json:"comicCount"`
			WorkCount  int    `json:"workCount"`
			UnitCount  int    `json:"unitCount"`
			FileCount  int    `json:"fileCount"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, library := range payload.Libraries {
		if library.ID != "library-counts-work" {
			continue
		}
		if library.WorkCount != 1 || library.UnitCount != 2 || library.FileCount != 2 || library.ComicCount != 1 {
			t.Fatalf("unexpected library counts: %#v", library)
		}
		return
	}
	t.Fatal("test library missing from response")
}
