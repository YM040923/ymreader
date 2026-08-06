package handler

import (
	"net/http"
	"testing"
)

func TestComicGroupRoutesAreRemovedButUserGroupRoutesRemain(t *testing.T) {
	router := setupTestRouter(t)
	adminCookie := registerAndLogin(t, router)

	for _, path := range []string{
		"/api/groups",
		"/api/groups/comic-map",
		"/api/groups/1",
		"/api/groups/auto-group-by-dir",
		"/api/groups/auto-detect",
		"/api/groups/batch-scrape",
	} {
		t.Run(path, func(t *testing.T) {
			response := performAuthedRequest(router, http.MethodGet, path, nil, adminCookie)
			if response.Code != http.StatusNotFound {
				t.Fatalf("%s returned %d, want 404", path, response.Code)
			}
		})
	}

	response := performAuthedRequest(router, http.MethodGet, "/api/admin/user-groups", nil, adminCookie)
	if response.Code == http.StatusNotFound {
		t.Fatal("user-group permission API was removed with ComicGroup")
	}
}
