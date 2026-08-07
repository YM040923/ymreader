package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestTranslateMetadataFieldsRetriesMalformedJSONOnce(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		content := `{"title":"中文标题"`
		if call == 2 {
			content = `{"title":"中文标题","description":"中文简介"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
			}},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 20,
				"total_tokens":      40,
			},
		})
	}))
	defer server.Close()

	result, err := TranslateMetadataFields(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "zhipu",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL,
		CloudModel:    "glm-5.2",
		MaxRetries:    0,
	}, map[string]string{
		"title":       "English title",
		"description": "English description",
	}, "zh-CN")
	if err != nil {
		t.Fatalf("TranslateMetadataFields returned error: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("translation calls = %d, want 2", calls.Load())
	}
	if result["title"] != "中文标题" || result["description"] != "中文简介" {
		t.Fatalf("unexpected translation result: %#v", result)
	}
}

func TestTranslateMetadataFieldsUsesCheapStructuredZhipuRequest(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": `{"title":"中文标题"}`,
				},
			}},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 10,
				"total_tokens":      30,
			},
		})
	}))
	defer server.Close()

	_, err := TranslateMetadataFields(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "zhipu",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL,
		CloudModel:    "glm-5.2",
		MaxRetries:    0,
	}, map[string]string{"title": "English title"}, "zh-CN")
	if err != nil {
		t.Fatalf("TranslateMetadataFields returned error: %v", err)
	}

	responseFormat, _ := requestBody["response_format"].(map[string]any)
	if responseFormat["type"] != "json_object" {
		t.Fatalf("response_format = %#v, want json_object", requestBody["response_format"])
	}
	thinking, _ := requestBody["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Fatalf("thinking = %#v, want disabled", requestBody["thinking"])
	}
	if got := int(requestBody["max_tokens"].(float64)); got > 512 {
		t.Fatalf("max_tokens = %d, want <= 512 for a short metadata translation", got)
	}
}

func TestOpenAICompatibleRejectsLengthTruncatedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"finish_reason": "length",
				"message": map[string]any{
					"role":    "assistant",
					"content": `{"title":"truncated`,
				},
			}},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 1000,
				"total_tokens":      1020,
			},
		})
	}))
	defer server.Close()

	_, _, err := callOpenAICompatible(
		AIConfig{CloudAPIKey: "test-key", CloudModel: "glm-5.2"},
		server.URL,
		"system",
		"user",
		1000,
		0,
		nil,
	)
	if err == nil {
		t.Fatal("callOpenAICompatible accepted a length-truncated response")
	}
}

func TestTranslateMetadataFieldsDoesNotRetryDeterministicProviderError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message_zh": "请求中的模型或服务 ID test-model 不存在",
			},
		})
	}))
	defer server.Close()

	_, err := TranslateMetadataFields(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL,
		CloudModel:    "test-model",
		MaxRetries:    2,
	}, map[string]string{"title": "English title"}, "zh-CN")
	if err == nil {
		t.Fatal("TranslateMetadataFields unexpectedly succeeded")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1 for deterministic provider error", got)
	}
}
