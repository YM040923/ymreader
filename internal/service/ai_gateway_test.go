package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestResolveOpenAIEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		raw          string
		wantURL      string
		wantProtocol AIProtocol
	}{
		{
			name:         "base v1 url",
			raw:          "https://example.test/v1",
			wantURL:      "https://example.test/v1/chat/completions",
			wantProtocol: AIProtocolChatCompletions,
		},
		{
			name:         "base v1 url with trailing slash and whitespace",
			raw:          "  https://example.test/v1/  ",
			wantURL:      "https://example.test/v1/chat/completions",
			wantProtocol: AIProtocolChatCompletions,
		},
		{
			name:         "complete chat completions endpoint",
			raw:          "https://example.test/v1/chat/completions",
			wantURL:      "https://example.test/v1/chat/completions",
			wantProtocol: AIProtocolChatCompletions,
		},
		{
			name:         "complete responses endpoint",
			raw:          "https://example.test/v1/responses",
			wantURL:      "https://example.test/v1/responses",
			wantProtocol: AIProtocolResponses,
		},
		{
			name:         "complete endpoint keeps query",
			raw:          "https://example.test/openai/responses?api-version=2026-01-01",
			wantURL:      "https://example.test/openai/responses?api-version=2026-01-01",
			wantProtocol: AIProtocolResponses,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveOpenAIEndpoint(tt.raw)
			if err != nil {
				t.Fatalf("resolveOpenAIEndpoint(%q) returned error: %v", tt.raw, err)
			}
			if got.URL != tt.wantURL {
				t.Fatalf("URL = %q, want %q", got.URL, tt.wantURL)
			}
			if got.Protocol != tt.wantProtocol {
				t.Fatalf("Protocol = %q, want %q", got.Protocol, tt.wantProtocol)
			}
		})
	}
}

func TestResolveOpenAIEndpointRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "example.test/v1", "ftp://example.test/v1", "://bad"} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			_, err := resolveOpenAIEndpoint(raw)
			if err == nil {
				t.Fatalf("resolveOpenAIEndpoint(%q) unexpectedly succeeded", raw)
			}
			var providerErr *AIProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error type = %T, want *AIProviderError", err)
			}
			if providerErr.Kind != AIErrorInvalidConfig {
				t.Fatalf("Kind = %q, want %q", providerErr.Kind, AIErrorInvalidConfig)
			}
			if providerErr.Retryable {
				t.Fatal("invalid configuration must not be retryable")
			}
		})
	}
}

func TestParseAIProviderErrorClassifiesProviderFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		status        int
		headers       map[string]string
		body          string
		wantKind      string
		wantMessage   string
		wantRequestID string
		wantRetry     bool
	}{
		{
			name:          "model unavailable",
			status:        http.StatusBadRequest,
			body:          `{"error":{"code":"400004","message":"The model or service ID hy3-preview does not exist.","message_zh":"请求中的模型或服务 ID hy3-preview 不存在","request_id":"body-request"}}`,
			wantKind:      AIErrorModelUnavailable,
			wantMessage:   "请求中的模型或服务 ID hy3-preview 不存在",
			wantRequestID: "body-request",
			wantRetry:     false,
		},
		{
			name:        "authentication",
			status:      http.StatusUnauthorized,
			body:        `{"error":{"message":"invalid api key"}}`,
			wantKind:    AIErrorAuthentication,
			wantMessage: "invalid api key",
			wantRetry:   false,
		},
		{
			name:        "endpoint not found",
			status:      http.StatusNotFound,
			body:        `404 page not found`,
			wantKind:    AIErrorEndpointNotFound,
			wantMessage: "404 page not found",
			wantRetry:   false,
		},
		{
			name:        "rate limited",
			status:      http.StatusTooManyRequests,
			body:        `{"error":{"message":"slow down"}}`,
			wantKind:    AIErrorRateLimited,
			wantMessage: "slow down",
			wantRetry:   true,
		},
		{
			name:   "provider failure with header request id",
			status: http.StatusServiceUnavailable,
			headers: map[string]string{
				"X-Request-ID": "header-request",
			},
			body:          `{"error":{"message":"temporarily unavailable"}}`,
			wantKind:      AIErrorProvider,
			wantMessage:   "temporarily unavailable",
			wantRequestID: "header-request",
			wantRetry:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: tt.status,
				Header:     make(http.Header),
			}
			for key, value := range tt.headers {
				resp.Header.Set(key, value)
			}

			got := parseAIProviderError(resp, []byte(tt.body))
			if got.Kind != tt.wantKind {
				t.Fatalf("Kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if !strings.Contains(got.Message, tt.wantMessage) {
				t.Fatalf("Message = %q, want it to contain %q", got.Message, tt.wantMessage)
			}
			if got.RequestID != tt.wantRequestID {
				t.Fatalf("RequestID = %q, want %q", got.RequestID, tt.wantRequestID)
			}
			if got.Retryable != tt.wantRetry {
				t.Fatalf("Retryable = %v, want %v", got.Retryable, tt.wantRetry)
			}
			if got.StatusCode != tt.status {
				t.Fatalf("StatusCode = %d, want %d", got.StatusCode, tt.status)
			}
		})
	}
}

func TestShouldRetryAIError(t *testing.T) {
	t.Parallel()

	if shouldRetryAIError(&AIProviderError{Kind: AIErrorInvalidRequest, Retryable: false}) {
		t.Fatal("deterministic 400 error must not be retried")
	}
	if !shouldRetryAIError(&AIProviderError{Kind: AIErrorRateLimited, Retryable: true}) {
		t.Fatal("429 error must be retried")
	}
	if shouldRetryAIError(errors.New("plain deterministic error")) {
		t.Fatal("unknown plain error must not be retried")
	}
}

func TestCallOpenAICompatibleResponsesEndpoint(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("request path = %q, want /v1/responses", r.URL.Path)
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []map[string]any{{
				"type": "message",
				"content": []map[string]any{{
					"type": "output_text",
					"text": `{"title":"药屋少女的呢喃"}`,
				}},
			}},
			"usage": map[string]int{
				"input_tokens":  20,
				"output_tokens": 8,
				"total_tokens":  28,
			},
		})
	}))
	defer server.Close()

	got, usage, err := callOpenAICompatibleWithOptions(
		AIConfig{
			CloudProvider: "compatible",
			CloudAPIKey:   "test-key",
			CloudModel:    "test-model",
		},
		server.URL+"/v1/responses",
		"system instructions",
		"translate this",
		256,
		0.1,
		nil,
		&LLMCallOptions{JSONMode: true, DisableThinking: true},
	)
	if err != nil {
		t.Fatalf("callOpenAICompatibleWithOptions returned error: %v", err)
	}
	if got != `{"title":"药屋少女的呢喃"}` {
		t.Fatalf("result = %q", got)
	}
	if usage.PromptTokens != 20 || usage.OutputTokens != 8 || usage.TotalTokens != 28 {
		t.Fatalf("usage = %#v", usage)
	}
	if usage.Protocol != string(AIProtocolResponses) {
		t.Fatalf("protocol = %q, want %q", usage.Protocol, AIProtocolResponses)
	}
	if requestBody["model"] != "test-model" {
		t.Fatalf("model = %#v", requestBody["model"])
	}
	if requestBody["instructions"] != "system instructions" {
		t.Fatalf("instructions = %#v", requestBody["instructions"])
	}
	if requestBody["input"] != "translate this" {
		t.Fatalf("input = %#v", requestBody["input"])
	}
	if gotMax := int(requestBody["max_output_tokens"].(float64)); gotMax != 256 {
		t.Fatalf("max_output_tokens = %d, want 256", gotMax)
	}
}

func TestCallOpenAICompatibleResponsesUsesTopLevelOutputText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"status":"completed",
			"output_text":"直接文本",
			"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}
		}`)
	}))
	defer server.Close()

	got, usage, err := callOpenAICompatible(
		AIConfig{CloudAPIKey: "test-key", CloudModel: "test-model"},
		server.URL+"/responses",
		"system",
		"user",
		64,
		0,
		nil,
	)
	if err != nil {
		t.Fatalf("callOpenAICompatible returned error: %v", err)
	}
	if got != "直接文本" {
		t.Fatalf("result = %q, want 直接文本", got)
	}
	if usage.TotalTokens != 5 {
		t.Fatalf("total tokens = %d, want 5", usage.TotalTokens)
	}
}

func TestCallOpenAICompatiblePreservesCompleteChatEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/chat/completions" {
			t.Fatalf("request path = %q, want /custom/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"choices":[{"finish_reason":"stop","message":{"content":"ok"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`)
	}))
	defer server.Close()

	got, _, err := callOpenAICompatible(
		AIConfig{CloudAPIKey: "test-key", CloudModel: "test-model"},
		server.URL+"/custom/chat/completions",
		"system",
		"user",
		64,
		0,
		nil,
	)
	if err != nil {
		t.Fatalf("callOpenAICompatible returned error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("result = %q, want ok", got)
	}
}

func TestCallOpenAICompatibleReturnsTypedProviderError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{
			"error":{
				"message_zh":"请求中的模型或服务 ID test-model 不存在",
				"request_id":"request-123"
			}
		}`)
	}))
	defer server.Close()

	_, _, err := callOpenAICompatible(
		AIConfig{CloudAPIKey: "test-key", CloudModel: "test-model"},
		server.URL,
		"system",
		"user",
		64,
		0,
		nil,
	)
	var providerErr *AIProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %T %v, want *AIProviderError", err, err)
	}
	if providerErr.Kind != AIErrorModelUnavailable {
		t.Fatalf("Kind = %q, want %q", providerErr.Kind, AIErrorModelUnavailable)
	}
	if providerErr.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", providerErr.RequestID)
	}
}

func TestCallCloudLLMDoesNotRetryDeterministicClientError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid parameter"}}`)
	}))
	defer server.Close()

	_, err := CallCloudLLM(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL,
		CloudModel:    "test-model",
		MaxRetries:    1,
	}, "system", "user", &LLMCallOptions{Scenario: "test"})
	if err == nil {
		t.Fatal("CallCloudLLM unexpectedly succeeded")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1 for deterministic 400 error", got)
	}
}

func TestCallCloudLLMRetriesTransientProviderError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"message":"try again"}}`)
			return
		}
		_, _ = io.WriteString(w, `{
			"choices":[{"finish_reason":"stop","message":{"content":"ok"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`)
	}))
	defer server.Close()

	got, err := CallCloudLLM(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL,
		CloudModel:    "test-model",
		MaxRetries:    1,
	}, "system", "user", &LLMCallOptions{Scenario: "test"})
	if err != nil {
		t.Fatalf("CallCloudLLM returned error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("result = %q, want ok", got)
	}
	if requestCount := calls.Load(); requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
}

func TestNormalizeAIRetryCountCapsLegacyValues(t *testing.T) {
	t.Parallel()

	tests := map[int]int{
		-1: 0,
		0:  0,
		1:  1,
		2:  2,
		5:  2,
	}
	for raw, want := range tests {
		if got := normalizeAIRetryCount(raw); got != want {
			t.Fatalf("normalizeAIRetryCount(%d) = %d, want %d", raw, got, want)
		}
	}
}

func TestResolveOpenAIModelsEndpoint(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"https://example.test/v1":                         "https://example.test/v1/models",
		"https://example.test/v1/":                        "https://example.test/v1/models",
		"https://example.test/v1/responses":               "https://example.test/v1/models",
		"https://example.test/v1/chat/completions":        "https://example.test/v1/models",
		"https://example.test/openai/responses?version=1": "https://example.test/openai/models?version=1",
	}
	for raw, want := range tests {
		got, err := resolveOpenAIModelsEndpoint(raw)
		if err != nil {
			t.Fatalf("resolveOpenAIModelsEndpoint(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("resolveOpenAIModelsEndpoint(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestListOpenAICompatibleModelsPreservesStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("request path = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"object":"list",
			"data":[
				{"id":"hy3","name":"Hy3","status":"online"},
				{"id":"hy3-preview","name":"Hy3 preview","status":"pre-offline"}
			]
		}`)
	}))
	defer server.Close()

	models, err := ListOpenAICompatibleModels(AIConfig{
		CloudAPIKey: "test-key",
		CloudAPIURL: server.URL + "/v1/responses",
	})
	if err != nil {
		t.Fatalf("ListOpenAICompatibleModels returned error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models count = %d, want 2", len(models))
	}
	if models[0].ID != "hy3" || models[0].Status != "online" {
		t.Fatalf("first model = %#v", models[0])
	}
	if models[1].ID != "hy3-preview" || models[1].Status != "pre-offline" {
		t.Fatalf("second model = %#v", models[1])
	}
}
