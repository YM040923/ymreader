package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// komgaTestLibrary seeds one comic library with two volumes of one Work so the
// Series -> Book mapping has something real to resolve.
func komgaTestLibrary(t *testing.T) (libraryID string, comicIDs []string) {
	t.Helper()
	libraryDir := t.TempDir()
	library := &model.Library{
		ID: "komga-lib", Name: "Komga 测试库", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}
	comics := []struct {
		id   string
		file string
	}{
		{"komga-comic-1", "久保同学/久保同学 - Vol. 001.cbz"},
		{"komga-comic-2", "久保同学/久保同学 - Vol. 002.cbz"},
	}
	for _, comic := range comics {
		archivePath := filepath.Join(libraryDir, filepath.FromSlash(comic.file))
		if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(archivePath, createImageCBZ(t), 0o600); err != nil {
			t.Fatalf("write cbz failed: %v", err)
		}
		t.Cleanup(func() { service.InvalidateContentFileCaches(comic.id, archivePath) })
		createOPDSHandlerComic(t, comic.id, comic.file, "comic", library.ID)
		if _, err := store.DB().Exec(`UPDATE "Comic" SET "pageCount" = 1 WHERE "id" = ?`, comic.id); err != nil {
			t.Fatalf("set pageCount failed: %v", err)
		}
		comicIDs = append(comicIDs, comic.id)
	}
	return library.ID, comicIDs
}

func komgaRequest(t *testing.T, router *gin.Engine, method, path, user, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.SetBasicAuth(user, secret)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestKomgaAuthBasicAndAPIKey(t *testing.T) {
	router := setupTestRouter(t)
	user, token := createOPDSTestUserAndKey(t, "komga-auth-user", "komga-auth")

	if response := komgaRequest(t, router, http.MethodGet, "/api/v1/libraries", user.Username, token); response.Code != http.StatusOK {
		t.Fatalf("Basic API key login returned %d: %s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	request.Header.Set("X-API-Key", token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("X-API-Key login returned %d: %s", response.Code, response.Body.String())
	}

	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request returned %d, want 401", anonymous.Code)
	}
	if challenge := anonymous.Header().Get("WWW-Authenticate"); !strings.Contains(challenge, "Basic") {
		t.Fatalf("missing Basic challenge: %q", challenge)
	}
}

func TestKomgaSeriesUsesSpringPageWrapper(t *testing.T) {
	router := setupTestRouter(t)
	komgaTestLibrary(t)
	user, token := createOPDSTestUserAndKey(t, "komga-series-user", "komga-series")
	if _, err := store.DB().Exec(`
		INSERT INTO "UserLibraryAccess" ("userId", "libraryId", "canView", "canDownload", "canManage")
		VALUES (?, 'komga-lib', 1, 1, 0)
	`, user.ID); err != nil {
		t.Fatalf("grant library access failed: %v", err)
	}

	response := komgaRequest(t, router, http.MethodGet, "/api/v1/series?page=0&size=10&search=", user.Username, token)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/series returned %d: %s", response.Code, response.Body.String())
	}
	var page struct {
		Content       []map[string]any `json:"content"`
		TotalElements int              `json:"totalElements"`
		TotalPages    int              `json:"totalPages"`
		Number        int              `json:"number"`
		Size          int              `json:"size"`
		Last          bool             `json:"last"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode series page failed: %v", err)
	}
	if page.TotalElements != 1 || len(page.Content) != 1 {
		t.Fatalf("expected one series, got total=%d content=%d", page.TotalElements, len(page.Content))
	}
	series := page.Content[0]
	if series["name"] != "久保同学" {
		t.Fatalf("series name = %v, want 久保同学", series["name"])
	}
	if series["libraryId"] != "komga-lib" {
		t.Fatalf("series libraryId = %v", series["libraryId"])
	}
	metadata, ok := series["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("series metadata missing: %v", series)
	}
	if metadata["title"] != "久保同学" {
		t.Fatalf("metadata.title = %v", metadata["title"])
	}
	if _, ok := series["booksMetadata"]; !ok {
		t.Fatalf("series booksMetadata missing")
	}

	seriesID, _ := series["id"].(string)
	booksResponse := komgaRequest(t, router, http.MethodGet, "/api/v1/series/"+seriesID+"/books?unpaged=true", user.Username, token)
	if booksResponse.Code != http.StatusOK {
		t.Fatalf("GET series books returned %d: %s", booksResponse.Code, booksResponse.Body.String())
	}
	var books struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(booksResponse.Body.Bytes(), &books); err != nil {
		t.Fatalf("decode books failed: %v", err)
	}
	if len(books.Content) != 2 {
		t.Fatalf("expected two books, got %d", len(books.Content))
	}
	first := books.Content[0]
	for _, key := range []string{"id", "seriesId", "seriesTitle", "name", "number", "sizeBytes", "size", "media", "metadata"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("book missing %q: %v", key, first)
		}
	}
	media, _ := first["media"].(map[string]any)
	if media["status"] != "READY" {
		t.Fatalf("book media.status = %v, want READY", media["status"])
	}
	if media["pagesCount"].(float64) != 1 {
		t.Fatalf("book media.pagesCount = %v, want 1", media["pagesCount"])
	}
}

func TestKomgaPagesAreOneBased(t *testing.T) {
	router := setupTestRouter(t)
	komgaTestLibrary(t)
	user, token := createOPDSTestUserAndKey(t, "komga-pages-user", "komga-pages")
	if _, err := store.DB().Exec(`
		INSERT INTO "UserLibraryAccess" ("userId", "libraryId", "canView", "canDownload", "canManage")
		VALUES (?, 'komga-lib', 1, 1, 0)
	`, user.ID); err != nil {
		t.Fatalf("grant library access failed: %v", err)
	}

	series := komgaRequest(t, router, http.MethodGet, "/api/v1/series?page=0&size=10", user.Username, token)
	var seriesPage struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(series.Body.Bytes(), &seriesPage); err != nil || len(seriesPage.Content) == 0 {
		t.Fatalf("decode series failed: %v", err)
	}
	seriesID, _ := seriesPage.Content[0]["id"].(string)
	bookPage := komgaRequest(t, router, http.MethodGet, "/api/v1/series/"+seriesID+"/books?unpaged=true", user.Username, token)
	var booksPayload struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(bookPage.Body.Bytes(), &booksPayload); err != nil || len(booksPayload.Content) == 0 {
		t.Fatalf("decode books failed: %v", err)
	}
	bookID, _ := booksPayload.Content[0]["id"].(string)

	pages := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+bookID+"/pages", user.Username, token)
	if pages.Code != http.StatusOK {
		t.Fatalf("GET pages returned %d: %s", pages.Code, pages.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(pages.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode pages failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one page, got %d", len(list))
	}
	if list[0]["number"].(float64) != 1 {
		t.Fatalf("first page number = %v, want 1 (Komga is 1-based)", list[0]["number"])
	}
	if list[0]["mediaType"] != "image/png" {
		t.Fatalf("page mediaType = %v, want image/png", list[0]["mediaType"])
	}

	image := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+bookID+"/pages/1", user.Username, token)
	if image.Code != http.StatusOK {
		t.Fatalf("GET page image returned %d: %s", image.Code, image.Body.String())
	}
	if !strings.HasPrefix(image.Header().Get("Content-Type"), "image/") {
		t.Fatalf("page image Content-Type = %q", image.Header().Get("Content-Type"))
	}
	outOfRange := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+bookID+"/pages/2", user.Username, token)
	if outOfRange.Code != http.StatusNotFound {
		t.Fatalf("out-of-range page returned %d, want 404", outOfRange.Code)
	}
}

func TestKomgaReadProgressRoundTrip(t *testing.T) {
	router := setupTestRouter(t)
	_, comicIDs := komgaTestLibrary(t)
	user, token := createOPDSTestUserAndKey(t, "komga-progress-user", "komga-progress")
	if _, err := store.DB().Exec(`
		INSERT INTO "UserLibraryAccess" ("userId", "libraryId", "canView", "canDownload", "canManage")
		VALUES (?, 'komga-lib', 1, 1, 0)
	`, user.ID); err != nil {
		t.Fatalf("grant library access failed: %v", err)
	}

	_ = comicIDs
	series := komgaRequest(t, router, http.MethodGet, "/api/v1/series?page=0&size=10", user.Username, token)
	var seriesPage struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(series.Body.Bytes(), &seriesPage); err != nil || len(seriesPage.Content) == 0 {
		t.Fatalf("decode series failed: %v", err)
	}
	seriesID, _ := seriesPage.Content[0]["id"].(string)
	booksPayload := komgaRequest(t, router, http.MethodGet, "/api/v1/series/"+seriesID+"/books?unpaged=true", user.Username, token)
	var booksPage struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(booksPayload.Body.Bytes(), &booksPage); err != nil || len(booksPage.Content) == 0 {
		t.Fatalf("decode books failed: %v", err)
	}
	comicID, _ := booksPage.Content[0]["id"].(string)
	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+comicID+"/read-progress", strings.NewReader(`{"page":1}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.SetBasicAuth(user.Username, token)
	patchResponse := httptest.NewRecorder()
	router.ServeHTTP(patchResponse, patch)
	if patchResponse.Code != http.StatusNoContent {
		t.Fatalf("PATCH read-progress returned %d: %s", patchResponse.Code, patchResponse.Body.String())
	}

	get := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+comicID+"/read-progress", user.Username, token)
	if get.Code != http.StatusOK {
		t.Fatalf("GET read-progress returned %d: %s", get.Code, get.Body.String())
	}
	var progress map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &progress); err != nil {
		t.Fatalf("decode progress failed: %v", err)
	}
	if progress["page"].(float64) != 1 {
		t.Fatalf("progress page = %v, want 1", progress["page"])
	}
	if progress["completed"].(bool) != true {
		t.Fatalf("single-page book should be completed, got %v", progress["completed"])
	}

	cleared := komgaRequest(t, router, http.MethodDelete, "/api/v1/books/"+comicID+"/read-progress", user.Username, token)
	if cleared.Code != http.StatusNoContent {
		t.Fatalf("DELETE read-progress returned %d", cleared.Code)
	}
}

func TestKomgaMultiChapterArchiveExposesDistinctBooks(t *testing.T) {
	router := setupTestRouter(t)
	libraryDir := t.TempDir()
	library := &model.Library{
		ID: "komga-packed-lib", Name: "打包漫画", Type: "comic",
		RootPath: libraryDir, Enabled: true, DefaultAccess: "private", ScanEnabled: true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	// One archive holding three chapter folders, like a Chinese webtoon release.
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range []string{
		"第0001话/001.png", "第0001话/002.png",
		"第0002话/001.png", "第0002话/002.png", "第0002话/003.png",
		"第0003话/001.png",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry failed: %v", err)
		}
		if _, err := entry.Write(komgaPNGBytes(t)); err != nil {
			t.Fatalf("write entry failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}

	const filename = "大王饶命.cbz"
	archivePath := filepath.Join(libraryDir, filename)
	if err := os.WriteFile(archivePath, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write archive failed: %v", err)
	}
	const comicID = "komga-packed-comic"
	t.Cleanup(func() { service.InvalidateContentFileCaches(comicID, archivePath) })
	createOPDSHandlerComic(t, comicID, filename, "comic", library.ID)
	if _, err := store.DB().Exec(`UPDATE "Comic" SET "pageCount" = 6 WHERE "id" = ?`, comicID); err != nil {
		t.Fatalf("set pageCount failed: %v", err)
	}

	user, token := createOPDSTestUserAndKey(t, "komga-packed-user", "komga-packed")
	if _, err := store.DB().Exec(`
		INSERT INTO "UserLibraryAccess" ("userId", "libraryId", "canView", "canDownload", "canManage")
		VALUES (?, 'komga-packed-lib', 1, 1, 0)
	`, user.ID); err != nil {
		t.Fatalf("grant library access failed: %v", err)
	}

	series := komgaRequest(t, router, http.MethodGet, "/api/v1/series?page=0&size=10", user.Username, token)
	var seriesPage struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(series.Body.Bytes(), &seriesPage); err != nil || len(seriesPage.Content) != 1 {
		t.Fatalf("series decode failed: %v %s", err, series.Body.String())
	}
	seriesID, _ := seriesPage.Content[0]["id"].(string)

	books := komgaRequest(t, router, http.MethodGet, "/api/v1/series/"+seriesID+"/books?unpaged=true", user.Username, token)
	var booksPage struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(books.Body.Bytes(), &booksPage); err != nil {
		t.Fatalf("books decode failed: %v", err)
	}
	if len(booksPage.Content) != 3 {
		t.Fatalf("expected 3 chapters as books, got %d: %s", len(booksPage.Content), books.Body.String())
	}
	seen := make(map[string]struct{})
	for _, book := range booksPage.Content {
		id, _ := book["id"].(string)
		if id == "" {
			t.Fatalf("book missing id: %v", book)
		}
		if id == comicID {
			t.Fatalf("chapter book reused the physical comic id: %v", book)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate book id %q", id)
		}
		seen[id] = struct{}{}
	}

	// Page lists must be scoped to the chapter, not the whole archive.
	firstID, _ := booksPage.Content[0]["id"].(string)
	firstPages := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+firstID+"/pages", user.Username, token)
	var firstList []map[string]any
	if err := json.Unmarshal(firstPages.Body.Bytes(), &firstList); err != nil {
		t.Fatalf("decode chapter pages failed: %v", err)
	}
	if len(firstList) != 2 {
		t.Fatalf("chapter 1 should expose 2 pages, got %d", len(firstList))
	}

	secondID, _ := booksPage.Content[1]["id"].(string)
	secondPages := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+secondID+"/pages", user.Username, token)
	var secondList []map[string]any
	if err := json.Unmarshal(secondPages.Body.Bytes(), &secondList); err != nil {
		t.Fatalf("decode chapter pages failed: %v", err)
	}
	if len(secondList) != 3 {
		t.Fatalf("chapter 2 should expose 3 pages, got %d", len(secondList))
	}

	// Chapter 2 page 1 must resolve to archive page 3, not page 1.
	image := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+secondID+"/pages/1", user.Username, token)
	if image.Code != http.StatusOK {
		t.Fatalf("chapter 2 first page returned %d: %s", image.Code, image.Body.String())
	}
	outOfRange := komgaRequest(t, router, http.MethodGet, "/api/v1/books/"+secondID+"/pages/4", user.Username, token)
	if outOfRange.Code != http.StatusNotFound {
		t.Fatalf("chapter overflow page returned %d, want 404", outOfRange.Code)
	}
}

func TestKomgaTaxonomyEndpoints(t *testing.T) {
	router := setupTestRouter(t)
	komgaTestLibrary(t)
	user, token := createOPDSTestUserAndKey(t, "komga-taxonomy-user", "komga-taxonomy")
	if _, err := store.DB().Exec(`
		INSERT INTO "UserLibraryAccess" ("userId", "libraryId", "canView", "canDownload", "canManage")
		VALUES (?, 'komga-lib', 1, 1, 0)
	`, user.ID); err != nil {
		t.Fatalf("grant library access failed: %v", err)
	}
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "genre" = '恋爱,校园', "author" = '雪森宁宁' WHERE "id" = 'komga-comic-1'
	`); err != nil {
		t.Fatalf("seed metadata failed: %v", err)
	}
	if _, err := store.DB().Exec(`
		UPDATE "Comic" SET "genre" = '恋爱', "author" = '雪森宁宁' WHERE "id" = 'komga-comic-2'
	`); err != nil {
		t.Fatalf("seed metadata failed: %v", err)
	}

	genres := komgaRequest(t, router, http.MethodGet, "/api/v1/genres", user.Username, token)
	if genres.Code != http.StatusOK {
		t.Fatalf("GET genres returned %d: %s", genres.Code, genres.Body.String())
	}
	var genreList []string
	if err := json.Unmarshal(genres.Body.Bytes(), &genreList); err != nil {
		t.Fatalf("decode genres failed: %v", err)
	}
	joined := strings.Join(genreList, ",")
	if !strings.Contains(joined, "恋爱") || !strings.Contains(joined, "校园") {
		t.Fatalf("genres = %v, want 恋爱 and 校园", genreList)
	}

	authors := komgaRequest(t, router, http.MethodGet, "/api/v1/authors", user.Username, token)
	if authors.Code != http.StatusOK {
		t.Fatalf("GET authors returned %d: %s", authors.Code, authors.Body.String())
	}
	var authorList []map[string]string
	if err := json.Unmarshal(authors.Body.Bytes(), &authorList); err != nil {
		t.Fatalf("decode authors failed: %v", err)
	}
	if len(authorList) == 0 || authorList[0]["name"] != "雪森宁宁" || authorList[0]["role"] != "writer" {
		t.Fatalf("authors = %v, want 雪森宁宁/writer", authorList)
	}

	collections := komgaRequest(t, router, http.MethodGet, "/api/v1/collections?unpaged=true", user.Username, token)
	if collections.Code != http.StatusOK {
		t.Fatalf("GET collections returned %d", collections.Code)
	}
}

// komgaPNGBytes is a tiny valid PNG used as a page inside test archives.
func komgaPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	page := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			page.Set(x, y, color.NRGBA{R: 200, G: 60, B: 60, A: 255})
		}
	}
	if err := png.Encode(&buffer, page); err != nil {
		t.Fatalf("encode png failed: %v", err)
	}
	return buffer.Bytes()
}
