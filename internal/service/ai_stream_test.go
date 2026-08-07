package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudLLMStreamPreservesCompleteChatEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var text strings.Builder
	var doneCount int
	err := CallCloudLLMStream(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL + "/v1/chat/completions",
		CloudModel:    "test-model",
	}, "system", "user", nil, func(chunk StreamChunk) bool {
		text.WriteString(chunk.Content)
		if chunk.Done {
			doneCount++
		}
		return true
	})
	if err != nil {
		t.Fatalf("CallCloudLLMStream returned error: %v", err)
	}
	if text.String() != "你好" {
		t.Fatalf("streamed text = %q, want 你好", text.String())
	}
	if doneCount != 1 {
		t.Fatalf("done callback count = %d, want 1", doneCount)
	}
}

func TestCloudLLMStreamSupportsResponsesEvents(t *testing.T) {
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
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: response.output_text.delta\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"你\"}\n\n")
		_, _ = fmt.Fprint(w, "event: response.output_text.delta\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"好\"}\n\n")
		_, _ = fmt.Fprint(w, "event: response.completed\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}))
	defer server.Close()

	var text strings.Builder
	var doneCount int
	err := CallCloudLLMStream(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL + "/v1/responses",
		CloudModel:    "test-model",
	}, "system", "user", &LLMCallOptions{MaxTokens: 128}, func(chunk StreamChunk) bool {
		text.WriteString(chunk.Content)
		if chunk.Done {
			doneCount++
		}
		return true
	})
	if err != nil {
		t.Fatalf("CallCloudLLMStream returned error: %v", err)
	}
	if text.String() != "你好" {
		t.Fatalf("streamed text = %q, want 你好", text.String())
	}
	if doneCount != 1 {
		t.Fatalf("done callback count = %d, want 1", doneCount)
	}
	if requestBody["stream"] != true {
		t.Fatalf("stream = %#v, want true", requestBody["stream"])
	}
	if _, ok := requestBody["messages"]; ok {
		t.Fatalf("Responses request unexpectedly contains messages: %#v", requestBody)
	}
}

func TestCloudLLMStreamReturnsTypedResponsesFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: response.failed\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"model failed\"}}}\n\n")
	}))
	defer server.Close()

	err := CallCloudLLMStream(AIConfig{
		EnableCloudAI: true,
		CloudProvider: "compatible",
		CloudAPIKey:   "test-key",
		CloudAPIURL:   server.URL + "/responses",
		CloudModel:    "test-model",
	}, "system", "user", nil, func(chunk StreamChunk) bool { return true })

	var providerErr *AIProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %T %v, want *AIProviderError", err, err)
	}
	if providerErr.Kind != AIErrorProvider {
		t.Fatalf("Kind = %q, want %q", providerErr.Kind, AIErrorProvider)
	}
	if !strings.Contains(providerErr.Message, "model failed") {
		t.Fatalf("Message = %q, want model failed", providerErr.Message)
	}
}
