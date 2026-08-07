package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AIProtocol string

const (
	AIProtocolChatCompletions AIProtocol = "chat_completions"
	AIProtocolResponses       AIProtocol = "responses"
)

const (
	AIErrorInvalidConfig     = "invalid_config"
	AIErrorInvalidRequest    = "invalid_request"
	AIErrorAuthentication    = "authentication"
	AIErrorEndpointNotFound  = "endpoint_not_found"
	AIErrorModelUnavailable  = "model_unavailable"
	AIErrorRateLimited       = "rate_limited"
	AIErrorTimeout           = "timeout"
	AIErrorProvider          = "provider_error"
	AIErrorInvalidResponse   = "invalid_response"
	AIErrorTruncated         = "truncated"
	AIErrorUnsupportedStream = "unsupported_stream_protocol"
)

type ResolvedAIEndpoint struct {
	URL      string
	Protocol AIProtocol
}

type AIModelInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type AIProviderError struct {
	Kind       string
	StatusCode int
	Message    string
	RequestID  string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *AIProviderError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "AI provider request failed"
	}
	if e.StatusCode > 0 {
		message = fmt.Sprintf("%s (HTTP %d)", message, e.StatusCode)
	}
	if e.RequestID != "" {
		message = fmt.Sprintf("%s [request_id: %s]", message, e.RequestID)
	}
	return message
}

func resolveOpenAIEndpoint(raw string) (ResolvedAIEndpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResolvedAIEndpoint{}, &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   "AI 接口地址不能为空",
			Retryable: false,
		}
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ResolvedAIEndpoint{}, &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   "AI 接口地址无效，请填写完整的 http:// 或 https:// 地址",
			Retryable: false,
		}
	}

	path := strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lowerPath, "/responses"):
		parsed.Path = path
		return ResolvedAIEndpoint{
			URL:      parsed.String(),
			Protocol: AIProtocolResponses,
		}, nil
	case strings.HasSuffix(lowerPath, "/chat/completions"):
		parsed.Path = path
		return ResolvedAIEndpoint{
			URL:      parsed.String(),
			Protocol: AIProtocolChatCompletions,
		}, nil
	default:
		if path == "" {
			path = "/chat/completions"
		} else {
			path += "/chat/completions"
		}
		parsed.Path = path
		return ResolvedAIEndpoint{
			URL:      parsed.String(),
			Protocol: AIProtocolChatCompletions,
		}, nil
	}
}

func ResolveAIProtocol(raw string) (AIProtocol, error) {
	endpoint, err := resolveOpenAIEndpoint(raw)
	if err != nil {
		return "", err
	}
	return endpoint.Protocol, nil
}

func resolveOpenAIModelsEndpoint(raw string) (string, error) {
	endpoint, err := resolveOpenAIEndpoint(raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(endpoint.URL)
	if err != nil {
		return "", &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   "无法解析模型接口地址",
			Retryable: false,
		}
	}
	path := strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lowerPath, "/chat/completions"):
		path = path[:len(path)-len("/chat/completions")]
	case strings.HasSuffix(lowerPath, "/responses"):
		path = path[:len(path)-len("/responses")]
	}
	parsed.Path = strings.TrimRight(path, "/") + "/models"
	return parsed.String(), nil
}

func ListOpenAICompatibleModels(cfg AIConfig) ([]AIModelInfo, error) {
	modelsURL, err := resolveOpenAIModelsEndpoint(cfg.CloudAPIURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, &AIProviderError{
			Kind:      AIErrorInvalidConfig,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	req.Header.Set("Accept", "application/json")
	if cfg.CloudAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.CloudAPIKey)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "读取模型列表失败: " + err.Error(),
			Retryable:  false,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAIProviderError(resp, body)
	}

	var payload struct {
		Data   []json.RawMessage `json:"data"`
		Models []json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &AIProviderError{
			Kind:       AIErrorInvalidResponse,
			StatusCode: resp.StatusCode,
			Message:    "无法解析模型列表: " + err.Error(),
			RequestID:  providerRequestID(resp, providerErrorEnvelope{}),
			Retryable:  false,
		}
	}
	rawModels := payload.Data
	if len(rawModels) == 0 {
		rawModels = payload.Models
	}
	models := make([]AIModelInfo, 0, len(rawModels))
	for _, rawModel := range rawModels {
		var model AIModelInfo
		if err := json.Unmarshal(rawModel, &model); err == nil && strings.TrimSpace(model.ID) != "" {
			model.ID = strings.TrimSpace(model.ID)
			model.Name = strings.TrimSpace(model.Name)
			model.Status = strings.TrimSpace(model.Status)
			models = append(models, model)
			continue
		}
		var id string
		if err := json.Unmarshal(rawModel, &id); err == nil && strings.TrimSpace(id) != "" {
			models = append(models, AIModelInfo{ID: strings.TrimSpace(id)})
		}
	}
	return models, nil
}

type providerErrorEnvelope struct {
	Error struct {
		Type      string `json:"type"`
		Code      string `json:"code"`
		Message   string `json:"message"`
		MessageZH string `json:"message_zh"`
		RequestID string `json:"request_id"`
	} `json:"error"`
	Message   string `json:"message"`
	MessageZH string `json:"message_zh"`
	RequestID string `json:"request_id"`
}

func parseAIProviderError(resp *http.Response, body []byte) *AIProviderError {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	var envelope providerErrorEnvelope
	_ = json.Unmarshal(body, &envelope)

	message := firstNonEmptyAI(
		envelope.Error.MessageZH,
		envelope.MessageZH,
		envelope.Error.Message,
		envelope.Message,
		strings.TrimSpace(string(body)),
		http.StatusText(statusCode),
	)
	requestID := providerRequestID(resp, envelope)

	kind := AIErrorProvider
	retryable := false
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		kind = AIErrorInvalidRequest
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = AIErrorAuthentication
	case http.StatusNotFound:
		kind = AIErrorEndpointNotFound
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooEarly:
		kind = AIErrorProvider
		retryable = true
	case http.StatusTooManyRequests:
		kind = AIErrorRateLimited
		retryable = true
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		kind = AIErrorProvider
		retryable = true
	}

	if looksLikeModelUnavailable(message, envelope.Error.Code) {
		kind = AIErrorModelUnavailable
		retryable = false
	}

	return &AIProviderError{
		Kind:       kind,
		StatusCode: statusCode,
		Message:    message,
		RequestID:  requestID,
		Retryable:  retryable,
		RetryAfter: parseRetryAfter(resp),
	}
}

func providerRequestID(resp *http.Response, envelope providerErrorEnvelope) string {
	if resp != nil {
		for _, header := range []string{"X-Request-ID", "Request-ID", "X-Request-Id", "X-Amzn-RequestId"} {
			if value := strings.TrimSpace(resp.Header.Get(header)); value != "" {
				return value
			}
		}
	}
	return firstNonEmptyAI(envelope.Error.RequestID, envelope.RequestID)
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		wait := time.Until(retryAt)
		if wait > 0 {
			return wait
		}
	}
	return 0
}

func looksLikeModelUnavailable(message, code string) bool {
	lower := strings.ToLower(message + " " + code)
	return strings.Contains(lower, "model or service id") ||
		strings.Contains(lower, "model does not exist") ||
		strings.Contains(lower, "model not found") ||
		strings.Contains(message, "模型或服务 ID") ||
		strings.Contains(message, "模型不存在") ||
		strings.Contains(message, "服务 ID 不存在")
}

func shouldRetryAIError(err error) bool {
	if err == nil {
		return false
	}

	var providerErr *AIProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Retryable
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

func aiErrorKind(err error) string {
	if err == nil {
		return ""
	}
	var providerErr *AIProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Kind
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return AIErrorTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return AIErrorTimeout
	}
	return AIErrorProvider
}

func normalizeAIRetryCount(retries int) int {
	switch {
	case retries <= 0:
		return 0
	case retries > 2:
		return 2
	default:
		return retries
	}
}

func aiRetryDelay(err error, retryNumber int) time.Duration {
	var providerErr *AIProviderError
	if errors.As(err, &providerErr) && providerErr.RetryAfter > 0 {
		if providerErr.RetryAfter > 10*time.Second {
			return 10 * time.Second
		}
		return providerErr.RetryAfter
	}
	if retryNumber <= 0 {
		retryNumber = 1
	}
	delay := 250 * time.Millisecond * time.Duration(1<<(retryNumber-1))
	if delay > 2*time.Second {
		return 2 * time.Second
	}
	return delay
}

func firstNonEmptyAI(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
