package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLibraryTypeGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{name: "comic create", method: http.MethodPost, body: `{"type":"comic"}`, wantStatus: http.StatusNoContent},
		{name: "novel create", method: http.MethodPost, body: `{"type":"novel"}`, wantStatus: http.StatusNoContent},
		{name: "mixed is rejected", method: http.MethodPost, body: `{"type":"mixed"}`, wantStatus: http.StatusBadRequest},
		{name: "unknown is rejected", method: http.MethodPut, body: `{"type":"other"}`, wantStatus: http.StatusBadRequest},
		{name: "partial update without type", method: http.MethodPut, body: `{"name":"Renamed"}`, wantStatus: http.StatusNoContent},
		{name: "non write route", method: http.MethodGet, body: "", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(LibraryTypeGuard())
			router.Handle(tt.method, "/libraries", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(tt.method, "/libraries", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.wantStatus, resp.Body.String())
			}
		})
	}
}
