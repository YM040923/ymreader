package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
)

func TestAIConnectionUsesSubmittedUnsavedConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DATA_DIR", t.TempDir())

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("provider path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{
					"content": `{"title":"药屋少女的呢喃","description":"宫廷推理故事"}`,
				},
			}},
			"usage": map[string]int{
				"prompt_tokens":     12,
				"completion_tokens": 8,
				"total_tokens":      20,
			},
		})
	}))
	defer provider.Close()

	body, _ := json.Marshal(service.AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "submitted-key",
		CloudAPIURL:   provider.URL + "/v1",
		CloudModel:    "submitted-model",
		MaxRetries:    0,
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ai/test", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewAIHandler().TestConnection(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["reply"] != "药屋少女的呢喃" {
		t.Fatalf("reply = %#v", response["reply"])
	}
	if response["protocol"] != string(service.AIProtocolChatCompletions) {
		t.Fatalf("protocol = %#v", response["protocol"])
	}
	if response["model"] != "submitted-model" {
		t.Fatalf("model = %#v", response["model"])
	}
}

func TestAIConnectionReturnsStructuredModelError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DATA_DIR", t.TempDir())

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message_zh": "请求中的模型或服务 ID old-model 不存在",
				"request_id": "provider-request",
			},
		})
	}))
	defer provider.Close()

	body, _ := json.Marshal(service.AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "submitted-key",
		CloudAPIURL:   provider.URL + "/responses",
		CloudModel:    "old-model",
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ai/test", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewAIHandler().TestConnection(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["errorType"] != service.AIErrorModelUnavailable {
		t.Fatalf("errorType = %#v", response["errorType"])
	}
	if response["requestId"] != "provider-request" {
		t.Fatalf("requestId = %#v", response["requestId"])
	}
}

func TestAIModelsUsesSubmittedConfigurationAndReturnsObjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DATA_DIR", t.TempDir())

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("provider path = %q, want /v1/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"hy3","name":"Hy3","status":"online"},
				{"id":"hy3-preview","name":"Hy3 preview","status":"pre-offline"}
			]
		}`))
	}))
	defer provider.Close()

	body, _ := json.Marshal(service.AIConfig{
		CloudProvider: "compatible",
		CloudAPIKey:   "submitted-key",
		CloudAPIURL:   provider.URL + "/v1/responses",
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ai/models", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewAIHandler().Models(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Models []service.AIModelInfo `json:"models"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Models) != 2 {
		t.Fatalf("models count = %d", len(response.Models))
	}
	if response.Models[1].Status != "pre-offline" {
		t.Fatalf("second model = %#v", response.Models[1])
	}
}

func TestAIGetSettingsIncludesLocalConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DATA_DIR", t.TempDir())

	if err := service.SaveAIConfig(service.AIConfig{
		EnableCloudAI:   true,
		CloudProvider:   "compatible",
		CloudAPIKey:     "secret-key",
		CloudAPIURL:     "https://example.test/v1",
		CloudModel:      "test-model",
		EnableLocalAI:   true,
		LocalEngine:     "llama.cpp",
		LocalBinaryPath: "/opt/llama-server",
		LocalModelPath:  "/models/test.gguf",
		LocalHost:       "127.0.0.1",
		LocalPort:       11435,
		ContextSize:     16384,
		Threads:         8,
		GPULayers:       "20",
	}); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/ai/settings", nil)
	NewAIHandler().GetSettings(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["localBinaryPath"] != "/opt/llama-server" {
		t.Fatalf("localBinaryPath = %#v", response["localBinaryPath"])
	}
	if response["localModelPath"] != "/models/test.gguf" {
		t.Fatalf("localModelPath = %#v", response["localModelPath"])
	}
	if response["contextSize"] != float64(16384) {
		t.Fatalf("contextSize = %#v", response["contextSize"])
	}
}
