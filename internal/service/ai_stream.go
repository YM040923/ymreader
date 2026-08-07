package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// SSE Streaming Support (0-4)
// ============================================================

// StreamChunk SSE 流式返回的单个数据块
type StreamChunk struct {
	Content string `json:"content"` // 增量文本
	Done    bool   `json:"done"`    // 是否结束
	Error   string `json:"error,omitempty"`
}

// StreamCallback 流式回调函数，返回 false 可中止流
type StreamCallback func(chunk StreamChunk) bool

// CallCloudLLMStream 流式调用云端 LLM（SSE），通过回调逐块返回内容。
// 注意：流式模式不支持重试，也不支持多模态（可后续扩展）。
func CallCloudLLMStream(cfg AIConfig, systemPrompt, userPrompt string, opts *LLMCallOptions, callback StreamCallback) error {
	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		return fmt.Errorf("cloud AI not configured")
	}
	if opts == nil {
		opts = &LLMCallOptions{}
	}

	provider := cfg.CloudProvider
	apiURL := cfg.CloudAPIURL
	if apiURL == "" {
		if p, ok := ProviderPresets[provider]; ok {
			apiURL = p.APIURL
		}
	}

	maxTokens := cfg.MaxTokens
	if opts.MaxTokens > 0 {
		maxTokens = opts.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	temp := 0.3
	if opts.Temperature != nil {
		temp = *opts.Temperature
	}

	start := time.Now()
	var err error

	switch provider {
	case "anthropic":
		err = streamAnthropic(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, callback)
	case "google":
		err = streamGemini(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, callback)
	default:
		err = streamOpenAICompatible(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, callback)
	}

	// 记录使用量（流式模式 token 数量设为 0，因为不一定能拿到）
	duration := time.Since(start).Milliseconds()
	record := AIUsageRecord{
		Timestamp:  time.Now(),
		Provider:   cfg.CloudProvider,
		Model:      cfg.CloudModel,
		Scenario:   opts.Scenario,
		Success:    err == nil,
		DurationMs: duration,
	}
	recordUsage(record)

	return err
}

// streamOpenAICompatible OpenAI 兼容的 SSE 流式调用
func streamOpenAICompatible(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, callback StreamCallback) error {
	endpoint, err := resolveOpenAIEndpoint(apiURL)
	if err != nil {
		return err
	}
	if endpoint.Protocol == AIProtocolResponses {
		return streamOpenAIResponses(cfg, endpoint, systemPrompt, userPrompt, maxTokens, temperature, callback)
	}
	return streamOpenAIChatCompletions(cfg, endpoint, systemPrompt, userPrompt, maxTokens, temperature, callback)
}

func streamOpenAIChatCompletions(cfg AIConfig, endpoint ResolvedAIEndpoint, systemPrompt, userPrompt string, maxTokens int, temperature float64, callback StreamCallback) error {
	body, _ := json.Marshal(map[string]interface{}{
		"model": cfg.CloudModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      true,
	})

	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("POST", endpoint.URL, strings.NewReader(string(body)))
	if err != nil {
		return &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CloudAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return parseAIProviderError(resp, respBody)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			callback(StreamChunk{Done: true})
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if !callback(StreamChunk{Content: chunk.Choices[0].Delta.Content}) {
				return nil // 客户端中止
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return &AIProviderError{
		Kind:       AIErrorInvalidResponse,
		StatusCode: resp.StatusCode,
		Message:    "AI 流在完成事件前中断",
		RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
		Retryable:  true,
	}
}

func streamOpenAIResponses(cfg AIConfig, endpoint ResolvedAIEndpoint, systemPrompt, userPrompt string, maxTokens int, temperature float64, callback StreamCallback) error {
	requestBody := map[string]interface{}{
		"model":             cfg.CloudModel,
		"instructions":      systemPrompt,
		"input":             userPrompt,
		"max_output_tokens": maxTokens,
		"temperature":       temperature,
		"stream":            true,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return &AIProviderError{
			Kind:      AIErrorInvalidRequest,
			Message:   "无法构造 Responses 流请求: " + err.Error(),
			Retryable: false,
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint.URL, strings.NewReader(string(body)))
	if err != nil {
		return &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+cfg.CloudAPIKey)

	resp, err := (&http.Client{Timeout: 300 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return parseAIProviderError(resp, respBody)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataText := strings.TrimPrefix(line, "data: ")
		if dataText == "[DONE]" {
			callback(StreamChunk{Done: true})
			return nil
		}

		var event struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Response struct {
				Status            string `json:"status"`
				IncompleteDetails struct {
					Reason string `json:"reason"`
				} `json:"incomplete_details"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(dataText), &event); err != nil {
			return &AIProviderError{
				Kind:       AIErrorInvalidResponse,
				StatusCode: resp.StatusCode,
				Message:    "无法解析 Responses 流事件: " + err.Error(),
				RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
				Retryable:  false,
			}
		}

		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" && !callback(StreamChunk{Content: event.Delta}) {
				return nil
			}
		case "response.completed":
			callback(StreamChunk{Done: true})
			return nil
		case "response.failed", "error":
			message := "Responses API 流式请求失败"
			if event.Response.Error != nil && event.Response.Error.Message != "" {
				message = event.Response.Error.Message
			} else if event.Error != nil && event.Error.Message != "" {
				message = event.Error.Message
			}
			return &AIProviderError{
				Kind:       AIErrorProvider,
				StatusCode: resp.StatusCode,
				Message:    message,
				RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
				Retryable:  false,
			}
		case "response.incomplete":
			kind := AIErrorInvalidResponse
			message := "Responses API 流式响应未完成"
			if strings.Contains(strings.ToLower(event.Response.IncompleteDetails.Reason), "max") {
				kind = AIErrorTruncated
				message = "AI 输出达到 token 上限，流式响应被截断"
			}
			return &AIProviderError{
				Kind:       kind,
				StatusCode: resp.StatusCode,
				Message:    message,
				RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
				Retryable:  false,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return &AIProviderError{
		Kind:       AIErrorInvalidResponse,
		StatusCode: resp.StatusCode,
		Message:    "Responses API 流在完成事件前中断",
		RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
		Retryable:  true,
	}
}

// streamAnthropic Anthropic 的 SSE 流式调用
func streamAnthropic(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, callback StreamCallback) error {
	reqURL := apiURL + "/v1/messages"

	body, _ := json.Marshal(map[string]interface{}{
		"model":       cfg.CloudModel,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"system":      systemPrompt,
		"messages":    []map[string]interface{}{{"role": "user", "content": userPrompt}},
		"stream":      true,
	})

	client := &http.Client{Timeout: 300 * time.Second}
	req, _ := http.NewRequest("POST", reqURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.CloudAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := string(respBody)
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		return fmt.Errorf("Anthropic stream API error %d: %s", resp.StatusCode, errMsg)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Text != "" {
				if !callback(StreamChunk{Content: event.Delta.Text}) {
					return nil
				}
			}
		case "message_stop":
			callback(StreamChunk{Done: true})
			return nil
		}
	}

	callback(StreamChunk{Done: true})
	return scanner.Err()
}

// streamGemini Google Gemini 的 SSE 流式调用
func streamGemini(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, callback StreamCallback) error {
	model := cfg.CloudModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	reqURL := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", apiURL, model, cfg.CloudAPIKey)

	body, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": systemPrompt + "\n\n" + userPrompt}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     temperature,
			"maxOutputTokens": maxTokens,
		},
	})

	client := &http.Client{Timeout: 300 * time.Second}
	req, _ := http.NewRequest("POST", reqURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := string(respBody)
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		return fmt.Errorf("Gemini stream API error %d: %s", resp.StatusCode, errMsg)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			text := chunk.Candidates[0].Content.Parts[0].Text
			if text != "" {
				if !callback(StreamChunk{Content: text}) {
					return nil
				}
			}
		}
	}

	callback(StreamChunk{Done: true})
	return scanner.Err()
}
