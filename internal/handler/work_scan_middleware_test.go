package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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
