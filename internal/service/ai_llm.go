package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================
// Cloud LLM Unified Caller (增强版)
// ============================================================

// LLMCallOptions 调用选项
type LLMCallOptions struct {
	// 场景标识，用于统计（如 translate, summary, tag, chat）
	Scenario string
	// 覆盖 config 的 MaxTokens（0 表示使用 config 的值）
	MaxTokens int
	// 覆盖 config 的 Temperature（nil 表示使用默认 0.3）
	Temperature *float64
	// 图片列表（多模态）
	Images []ImageContent
	// 要求兼容接口返回 JSON 对象。
	JSONMode bool
	// 关闭提供商的深度思考模式，适用于翻译等低复杂度任务。
	DisableThinking bool
}

// CallCloudLLM 调用 LLM，支持重试和 token 统计。
// 优先使用本地模型（如果启用且运行中），否则使用云端模型。
// opts 可传 nil 使用默认选项。
func CallCloudLLM(cfg AIConfig, systemPrompt, userPrompt string, opts *LLMCallOptions) (string, error) {
	// 优先尝试本地模型
	if cfg.EnableLocalAI && LocalAI.IsRunning() {
		result, err := callLocalLLM(cfg, systemPrompt, userPrompt, opts)
		if err == nil {
			return result, nil
		}
		log.Printf("[AI] 本地模型调用失败，回退到云端: %v", err)
	}

	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		return "", fmt.Errorf("cloud AI not configured")
	}

	if opts == nil {
		opts = &LLMCallOptions{}
	}

	maxRetries := normalizeAIRetryCount(cfg.MaxRetries)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := aiRetryDelay(lastErr, attempt)
			log.Printf("[AI] Retry %d/%d after %v (error: %v)", attempt, maxRetries, backoff, lastErr)
			if backoff > 0 {
				time.Sleep(backoff)
			}
		}

		start := time.Now()
		result, usage, err := callCloudLLMOnce(cfg, systemPrompt, userPrompt, opts)
		duration := time.Since(start).Milliseconds()

		// 记录使用量
		record := AIUsageRecord{
			Timestamp:       time.Now(),
			Provider:        cfg.CloudProvider,
			Model:           cfg.CloudModel,
			Protocol:        usage.Protocol,
			PromptTokens:    usage.PromptTokens,
			OutputTokens:    usage.OutputTokens,
			ReasoningTokens: usage.ReasoningTokens,
			TotalTokens:     usage.TotalTokens,
			Scenario:        opts.Scenario,
			Success:         err == nil,
			DurationMs:      duration,
			ErrorType:       aiErrorKind(err),
		}
		recordUsage(record)

		if err == nil {
			return result, nil
		}

		lastErr = err
		if !shouldRetryAIError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("AI 请求在 %d 次尝试后仍失败: %w", maxRetries+1, lastErr)
}

// tokenUsage 从 API 响应中提取的 token 使用量
type tokenUsage struct {
	PromptTokens    int
	OutputTokens    int
	ReasoningTokens int
	TotalTokens     int
	Protocol        string
}

// callLocalLLM 调用本地模型（通过 llama.cpp 的 OpenAI Compatible API）
func callLocalLLM(cfg AIConfig, systemPrompt, userPrompt string, opts *LLMCallOptions) (string, error) {
	if opts == nil {
		opts = &LLMCallOptions{}
	}

	apiURL := LocalAI.GetAPIURL()
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
	result, usage, err := callOpenAICompatible(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, opts.Images)
	duration := time.Since(start).Milliseconds()

	// 记录使用量
	record := AIUsageRecord{
		Timestamp:       time.Now(),
		Provider:        "local",
		Model:           filepath.Base(cfg.LocalModelPath),
		Protocol:        usage.Protocol,
		PromptTokens:    usage.PromptTokens,
		OutputTokens:    usage.OutputTokens,
		ReasoningTokens: usage.ReasoningTokens,
		TotalTokens:     usage.TotalTokens,
		Scenario:        opts.Scenario,
		Success:         err == nil,
		DurationMs:      duration,
		ErrorType:       aiErrorKind(err),
	}
	recordUsage(record)

	return result, err
}

// callCloudLLMOnce 单次调用（不重试）
func callCloudLLMOnce(cfg AIConfig, systemPrompt, userPrompt string, opts *LLMCallOptions) (string, tokenUsage, error) {
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

	switch provider {
	case "anthropic":
		return callAnthropic(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, opts.Images)
	case "google":
		return callGemini(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, opts.Images)
	default:
		return callOpenAICompatibleWithOptions(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temp, opts.Images, opts)
	}
}

// ============================================================
// OpenAI Compatible Provider (含多模态)
// ============================================================

func callOpenAICompatible(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent) (string, tokenUsage, error) {
	return callOpenAICompatibleWithOptions(cfg, apiURL, systemPrompt, userPrompt, maxTokens, temperature, images, nil)
}

func callOpenAICompatibleWithOptions(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent, opts *LLMCallOptions) (string, tokenUsage, error) {
	endpoint, err := resolveOpenAIEndpoint(apiURL)
	if err != nil {
		return "", tokenUsage{}, err
	}
	switch endpoint.Protocol {
	case AIProtocolResponses:
		return callOpenAIResponses(cfg, endpoint, systemPrompt, userPrompt, maxTokens, temperature, images, opts)
	default:
		return callOpenAIChatCompletions(cfg, endpoint, systemPrompt, userPrompt, maxTokens, temperature, images, opts)
	}
}

func callOpenAIChatCompletions(cfg AIConfig, endpoint ResolvedAIEndpoint, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent, opts *LLMCallOptions) (string, tokenUsage, error) {
	// 构建 messages
	messages := []interface{}{
		map[string]string{"role": "system", "content": systemPrompt},
	}

	// 用户消息：如果有图片，使用多模态格式
	if len(images) > 0 {
		contentParts := []interface{}{
			map[string]string{"type": "text", "text": userPrompt},
		}
		for _, img := range images {
			imageURL := ""
			if img.Base64 != "" {
				mimeType := img.MimeType
				if mimeType == "" {
					mimeType = "image/jpeg"
				}
				imageURL = fmt.Sprintf("data:%s;base64,%s", mimeType, img.Base64)
			} else if img.URL != "" {
				imageURL = img.URL
			}
			if imageURL != "" {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": imageURL,
					},
				})
			}
		}
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": contentParts,
		})
	} else {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": userPrompt,
		})
	}

	requestBody := map[string]interface{}{
		"model":       cfg.CloudModel,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
	}
	if opts != nil && opts.JSONMode && supportsOpenAIJSONMode(cfg.CloudProvider) {
		requestBody["response_format"] = map[string]string{"type": "json_object"}
	}
	if opts != nil && opts.DisableThinking && cfg.CloudProvider == "zhipu" {
		requestBody["thinking"] = map[string]string{"type": "disabled"}
	}
	body, _ := json.Marshal(requestBody)

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", endpoint.URL, strings.NewReader(string(body)))
	if err != nil {
		return "", tokenUsage{}, &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CloudAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", tokenUsage{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", tokenUsage{}, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "读取 AI 响应失败: " + err.Error(),
			Retryable:  false,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", tokenUsage{}, parseAIProviderError(resp, respBody)
	}

	var data struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens           int `json:"prompt_tokens"`
			CompletionTokens       int `json:"completion_tokens"`
			TotalTokens            int `json:"total_tokens"`
			CompletionTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return "", tokenUsage{}, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("无法解析 AI 响应: %v；响应片段: %s", err, preview),
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}
	if len(data.Choices) == 0 {
		return "", tokenUsage{}, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "AI 返回了空结果",
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}

	usage := tokenUsage{
		PromptTokens:    data.Usage.PromptTokens,
		OutputTokens:    data.Usage.CompletionTokens,
		ReasoningTokens: data.Usage.CompletionTokenDetails.ReasoningTokens,
		TotalTokens:     data.Usage.TotalTokens,
		Protocol:        string(AIProtocolChatCompletions),
	}
	if data.Choices[0].FinishReason == "length" {
		return "", usage, &AIProviderError{
			Kind:       AIErrorTruncated,
			StatusCode: resp.StatusCode,
			Message:    "AI 输出达到 token 上限，响应被截断",
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}
	return data.Choices[0].Message.Content, usage, nil
}

func callOpenAIResponses(cfg AIConfig, endpoint ResolvedAIEndpoint, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent, opts *LLMCallOptions) (string, tokenUsage, error) {
	requestBody := map[string]interface{}{
		"model":             cfg.CloudModel,
		"instructions":      systemPrompt,
		"input":             buildResponsesInput(userPrompt, images),
		"max_output_tokens": maxTokens,
	}
	if temperature >= 0 {
		requestBody["temperature"] = temperature
	}
	if opts != nil && opts.JSONMode && supportsOpenAIJSONMode(cfg.CloudProvider) {
		requestBody["text"] = map[string]interface{}{
			"format": map[string]string{"type": "json_object"},
		}
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", tokenUsage{}, &AIProviderError{
			Kind:      AIErrorInvalidRequest,
			Message:   "无法构造 AI 请求: " + err.Error(),
			Retryable: false,
		}
	}

	req, err := http.NewRequest("POST", endpoint.URL, strings.NewReader(string(body)))
	if err != nil {
		return "", tokenUsage{}, &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CloudAPIKey)

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", tokenUsage{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", tokenUsage{}, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "读取 AI 响应失败: " + err.Error(),
			Retryable:  false,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", tokenUsage{}, parseAIProviderError(resp, respBody)
	}

	var data struct {
		Status            string `json:"status"`
		OutputText        string `json:"output_text"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens        int `json:"input_tokens"`
			OutputTokens       int `json:"output_tokens"`
			TotalTokens        int `json:"total_tokens"`
			OutputTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return "", tokenUsage{}, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("无法解析 Responses API 响应: %v；响应片段: %s", err, preview),
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}

	usage := tokenUsage{
		PromptTokens:    data.Usage.InputTokens,
		OutputTokens:    data.Usage.OutputTokens,
		ReasoningTokens: data.Usage.OutputTokenDetails.ReasoningTokens,
		TotalTokens:     data.Usage.TotalTokens,
		Protocol:        string(AIProtocolResponses),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.OutputTokens
	}

	switch strings.ToLower(strings.TrimSpace(data.Status)) {
	case "failed", "cancelled":
		message := "Responses API 返回失败状态"
		if data.Error != nil && data.Error.Message != "" {
			message = data.Error.Message
		}
		return "", usage, &AIProviderError{
			Kind:       AIErrorProvider,
			StatusCode: resp.StatusCode,
			Message:    message,
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	case "incomplete":
		message := "AI 响应未完成"
		kind := AIErrorInvalidResponse
		if strings.Contains(strings.ToLower(data.IncompleteDetails.Reason), "max") {
			message = "AI 输出达到 token 上限，响应被截断"
			kind = AIErrorTruncated
		}
		return "", usage, &AIProviderError{
			Kind:       kind,
			StatusCode: resp.StatusCode,
			Message:    message,
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}

	text := strings.TrimSpace(data.OutputText)
	if text == "" {
		var parts []string
		for _, output := range data.Output {
			for _, content := range output.Content {
				if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
					parts = append(parts, content.Text)
				}
			}
		}
		text = strings.TrimSpace(strings.Join(parts, ""))
	}
	if text == "" {
		return "", usage, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "Responses API 返回了空结果",
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}
	return text, usage, nil
}

func buildResponsesInput(userPrompt string, images []ImageContent) interface{} {
	if len(images) == 0 {
		return userPrompt
	}

	content := []interface{}{
		map[string]string{"type": "input_text", "text": userPrompt},
	}
	for _, img := range images {
		imageURL := ""
		if img.Base64 != "" {
			mimeType := img.MimeType
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			imageURL = fmt.Sprintf("data:%s;base64,%s", mimeType, img.Base64)
		} else if img.URL != "" {
			imageURL = img.URL
		}
		if imageURL != "" {
			content = append(content, map[string]string{
				"type":      "input_image",
				"image_url": imageURL,
			})
		}
	}
	return []map[string]interface{}{{
		"role":    "user",
		"content": content,
	}}
}

func supportsOpenAIJSONMode(provider string) bool {
	switch provider {
	case "openai", "zhipu", "deepseek":
		return true
	default:
		return false
	}
}

// ============================================================
// Anthropic Provider (含多模态)
// ============================================================

func callAnthropic(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent) (string, tokenUsage, error) {
	reqURL := apiURL + "/v1/messages"

	// 构建 content
	var content []interface{}
	if len(images) > 0 {
		for _, img := range images {
			if img.Base64 != "" {
				mimeType := img.MimeType
				if mimeType == "" {
					mimeType = "image/jpeg"
				}
				content = append(content, map[string]interface{}{
					"type": "image",
					"source": map[string]string{
						"type":       "base64",
						"media_type": mimeType,
						"data":       img.Base64,
					},
				})
			}
		}
	}
	content = append(content, map[string]interface{}{
		"type": "text",
		"text": userPrompt,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"model":       cfg.CloudModel,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"system":      systemPrompt,
		"messages":    []map[string]interface{}{{"role": "user", "content": content}},
	})

	client := &http.Client{Timeout: 120 * time.Second}
	req, _ := http.NewRequest("POST", reqURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.CloudAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return "", tokenUsage{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		errMsg := string(respBody)
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		return "", tokenUsage{}, fmt.Errorf("Anthropic API error %d: %s", resp.StatusCode, errMsg)
	}

	var data struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", tokenUsage{}, err
	}

	usage := tokenUsage{
		PromptTokens: data.Usage.InputTokens,
		OutputTokens: data.Usage.OutputTokens,
		TotalTokens:  data.Usage.InputTokens + data.Usage.OutputTokens,
		Protocol:     "anthropic",
	}

	for _, c := range data.Content {
		if c.Type == "text" {
			return c.Text, usage, nil
		}
	}
	return "", usage, fmt.Errorf("no text in Anthropic response")
}

// ============================================================
// Google Gemini Provider (含多模态)
// ============================================================

func callGemini(cfg AIConfig, apiURL, systemPrompt, userPrompt string, maxTokens int, temperature float64, images []ImageContent) (string, tokenUsage, error) {
	model := cfg.CloudModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	reqURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", apiURL, model, cfg.CloudAPIKey)

	// 构建 parts
	parts := []interface{}{
		map[string]string{"text": systemPrompt + "\n\n" + userPrompt},
	}
	for _, img := range images {
		if img.Base64 != "" {
			mimeType := img.MimeType
			if mimeType == "" {
				mimeType = "image/jpeg"
			}
			parts = append(parts, map[string]interface{}{
				"inline_data": map[string]string{
					"mime_type": mimeType,
					"data":      img.Base64,
				},
			})
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": parts},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     temperature,
			"maxOutputTokens": maxTokens,
		},
	})

	client := &http.Client{Timeout: 120 * time.Second}
	req, _ := http.NewRequest("POST", reqURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", tokenUsage{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		errMsg := string(respBody)
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		return "", tokenUsage{}, fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, errMsg)
	}

	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", tokenUsage{}, err
	}

	usage := tokenUsage{
		PromptTokens: data.UsageMetadata.PromptTokenCount,
		OutputTokens: data.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  data.UsageMetadata.TotalTokenCount,
		Protocol:     "gemini",
	}

	if len(data.Candidates) > 0 && len(data.Candidates[0].Content.Parts) > 0 {
		return data.Candidates[0].Content.Parts[0].Text, usage, nil
	}
	return "", usage, fmt.Errorf("no response from Gemini")
}
