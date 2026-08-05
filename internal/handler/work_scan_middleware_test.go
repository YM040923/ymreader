package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestRebuildWorksAfterScanRebuildsSuccessfulLibraryScan(t *testing.T) {
	setupTestRouter(t)
	db := store.DB()
	if _, err := db.Exec(`INSERT INTO "Library" ("id", "name", "type", "rootPath") VALUES ('work-hook-lib', 'Work Hook Lib', 'comic', '/tmp/work-hook')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO "Comic" ("id", "filename", "title", "type", "libraryId", "relativePath", "pageCount", "fileSize") VALUES
			('work-hook-2', '大王饶命/第002话.cbz', '第002话', 'comic', 'work-hook-lib', '大王饶命/第002话.cbz', 22, 2200),
			('work-hook-1', '大王饶命/第001话.cbz', '第001话', 'comic', 'work-hook-lib', '大王饶命/第001话.cbz', 11, 1100)
	`); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.POST("/scan/:id", rebuildWorksAfterScan(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scan/work-hook-lib", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	works, err := store.ListWorks([]string{"work-hook-lib"}, "user-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 || works[0].ItemCount != 2 {
		t.Fatalf("works after middleware = %#v", works)
	}
}

func TestLibraryScanRouteRebuildsWorksProjection(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)
	root := t.TempDir()
	for _, name := range []string{"Da Wang Rao Ming Ch.001.cbz", "Da Wang Rao Ming Ch.002.cbz"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("comic"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	w := performAuthedRequest(r, http.MethodPost, "/api/admin/libraries", map[string]interface{}{
		"name": "Route Work Hook", "type": "comic", "rootPaths": []string{root},
	}, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create library: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Library model.Library `json:"library"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	w = performAuthedRequest(r, http.MethodPost, "/api/admin/libraries/"+created.Library.ID+"/scan", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("scan library: %d %s", w.Code, w.Body.String())
	}

	works, err := store.ListWorks([]string{created.Library.ID}, "admin", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("work count = %d, works = %#v", len(works), works)
	}
	if works[0].Title != "Da Wang Rao Ming" || works[0].ItemCount != 2 {
		t.Fatalf("work projection = %#v", works[0])
	}
}
