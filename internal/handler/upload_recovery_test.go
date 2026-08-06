package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/middleware"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

func TestUploadRecoversMatchingUnindexedFile(t *testing.T) {
	r := setupTestRouter(t)
	cookie := registerAndLogin(t, r)

	root := t.TempDir()
	library := &model.Library{
		ID:            "upload-recovery-library",
		Name:          "Upload Recovery Library",
		Type:          "novel",
		RootPath:      root,
		Enabled:       true,
		DefaultAccess: "private",
		ScanEnabled:   true,
	}
	if err := store.CreateLibrary(library); err != nil {
		t.Fatalf("CreateLibrary failed: %v", err)
	}

	const filename = "Robinson Crusoe.epub"
	existingPath := filepath.Join(root, filename)
	writeMinimalEpub(t, existingPath)
	existingBytes, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing epub: %v", err)
	}

	var requestBody bytes.Buffer
	form := multipart.NewWriter(&requestBody)
	if err := form.WriteField("libraryId", library.ID); err != nil {
		t.Fatalf("write libraryId: %v", err)
	}
	part, err := form.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("create upload part: %v", err)
	}
	if _, err := part.Write(existingBytes); err != nil {
		t.Fatalf("write upload part: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close multipart form: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/api/upload", &requestBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookie, Value: cookie})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("upload recovery: %d %s", response.Code, response.Body.String())
	}

	var payload struct {
		SuccessCount int `json:"successCount"`
		Results      []struct {
			Success   bool `json:"success"`
			Recovered bool `json:"recovered"`
		} `json:"results"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if payload.SuccessCount != 1 || len(payload.Results) != 1 || !payload.Results[0].Success || !payload.Results[0].Recovered {
		t.Fatalf("unexpected recovery response: %s", response.Body.String())
	}

	// The upload dialog performs this selected-library scan after every success.
	// Verify the recovered on-disk file becomes a normal database row again.
	scan := performAuthedRequest(r, http.MethodPost, "/api/admin/libraries/"+library.ID+"/scan", nil, cookie)
	if scan.Code != http.StatusOK {
		t.Fatalf("scan recovered library: %d %s", scan.Code, scan.Body.String())
	}
	indexed, err := store.ComicExistsAtLibraryPath(library.ID, filename)
	if err != nil {
		t.Fatalf("ComicExistsAtLibraryPath failed: %v", err)
	}
	if !indexed {
		t.Fatal("recovered physical file was not indexed by the follow-up scan")
	}
}
