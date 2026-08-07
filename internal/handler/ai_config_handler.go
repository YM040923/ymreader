package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
)

type AIHandler struct{}

func NewAIHandler() *AIHandler { return &AIHandler{} }

// GET /api/ai/status
func (h *AIHandler) Status(c *gin.Context) {
	status := service.GetAIStatus()
	c.JSON(200, status)
}

// GET /api/ai/settings
func (h *AIHandler) GetSettings(c *gin.Context) {
	cfg := service.LoadAIConfig()
	// Mask API key
	maskedKey := ""
	if cfg.CloudAPIKey != "" {
		if len(cfg.CloudAPIKey) > 8 {
			maskedKey = cfg.CloudAPIKey[:4] + "****" + cfg.CloudAPIKey[len(cfg.CloudAPIKey)-4:]
		} else {
			maskedKey = "****"
		}
	}

	c.JSON(200, gin.H{
		"enableCloudAI":   cfg.EnableCloudAI,
		"cloudProvider":   cfg.CloudProvider,
		"cloudApiKey":     maskedKey,
		"cloudApiUrl":     cfg.CloudAPIURL,
		"cloudModel":      cfg.CloudModel,
		"maxTokens":       cfg.MaxTokens,
		"maxRetries":      cfg.MaxRetries,
		"enableLocalAI":   cfg.EnableLocalAI,
		"localEngine":     cfg.LocalEngine,
		"localBinaryPath": cfg.LocalBinaryPath,
		"localModelPath":  cfg.LocalModelPath,
		"localHost":       cfg.LocalHost,
		"localPort":       cfg.LocalPort,
		"contextSize":     cfg.ContextSize,
		"threads":         cfg.Threads,
		"gpuLayers":       cfg.GPULayers,
		"providerPresets": service.ProviderPresets,
	})
}

// PUT /api/ai/settings
func (h *AIHandler) UpdateSettings(c *gin.Context) {
	var body service.AIConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	// Protect existing API key if masked
	if strings.Contains(body.CloudAPIKey, "****") {
		existing := service.LoadAIConfig()
		body.CloudAPIKey = existing.CloudAPIKey
	}

	if err := service.SaveAIConfig(body); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save AI config"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// GET /api/ai/models?provider=...&apiUrl=...&apiKey=...
func (h *AIHandler) Models(c *gin.Context) {
	cfg, err := aiConfigFromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 配置格式无效"})
		return
	}
	if c.Request.Method == http.MethodGet && c.Query("provider") != "" {
		cfg.CloudProvider = c.Query("provider")
	}
	if c.Request.Method == http.MethodGet && c.Query("apiUrl") != "" {
		cfg.CloudAPIURL = c.Query("apiUrl")
	}
	if c.Request.Method == http.MethodGet && c.Query("apiKey") != "" {
		cfg.CloudAPIKey = c.Query("apiKey")
	}
	cfg = mergeMaskedAIKey(cfg)

	preset, hasPreset := service.ProviderPresets[cfg.CloudProvider]
	if cfg.CloudAPIURL == "" && hasPreset {
		cfg.CloudAPIURL = preset.APIURL
	}

	switch cfg.CloudProvider {
	case "anthropic", "google":
		models := make([]service.AIModelInfo, 0, len(preset.Models))
		for _, id := range preset.Models {
			models = append(models, service.AIModelInfo{ID: id, Name: id})
		}
		c.JSON(http.StatusOK, gin.H{
			"models":   models,
			"provider": cfg.CloudProvider,
			"source":   "preset",
		})
		return
	}

	if cfg.CloudAPIURL != "" && cfg.CloudAPIKey != "" {
		models, listErr := service.ListOpenAICompatibleModels(cfg)
		if listErr != nil {
			writeAIError(c, listErr)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"models":   models,
			"provider": cfg.CloudProvider,
			"source":   "provider",
		})
		return
	}

	models := make([]service.AIModelInfo, 0, len(preset.Models))
	for _, id := range preset.Models {
		models = append(models, service.AIModelInfo{ID: id, Name: id})
	}
	c.JSON(http.StatusOK, gin.H{
		"models":   models,
		"provider": cfg.CloudProvider,
		"source":   "preset",
	})
}

// GET /api/ai/usage — 获取 AI 使用量统计
func (h *AIHandler) GetUsageStats(c *gin.Context) {
	stats := service.GetAIUsageStats()
	c.JSON(200, stats)
}

// DELETE /api/ai/usage — 重置 AI 使用量统计
func (h *AIHandler) ResetUsageStats(c *gin.Context) {
	service.ResetAIUsageStats()
	c.JSON(200, gin.H{"success": true})
}

// POST /api/ai/test — 测试 AI 连接
func (h *AIHandler) TestConnection(c *gin.Context) {
	cfg, err := aiConfigFromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI 配置格式无效"})
		return
	}
	cfg = mergeMaskedAIKey(cfg)
	if !cfg.EnableCloudAI || cfg.CloudAPIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "云端 AI 尚未启用或 API Key 为空",
			"errorType": service.AIErrorInvalidConfig,
		})
		return
	}
	// 云端连接测试必须绕过本地模型，否则本地模型成功会掩盖错误的云端 URL、Key 或模型。
	cfg.EnableLocalAI = false

	result, err := service.TranslateMetadataFields(cfg, map[string]string{
		"title":       "The Apothecary Diaries",
		"description": "A palace mystery story.",
	}, "zh-CN")
	if err != nil {
		writeAIError(c, err)
		return
	}
	if strings.TrimSpace(result["title"]) == "" {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":     "AI 返回了空的结构化翻译",
			"errorType": service.AIErrorInvalidResponse,
		})
		return
	}

	protocol := ""
	if cfg.CloudProvider != "anthropic" && cfg.CloudProvider != "google" {
		if resolved, resolveErr := service.ResolveAIProtocol(cfg.CloudAPIURL); resolveErr == nil {
			protocol = string(resolved)
		}
	}
	c.JSON(200, gin.H{
		"success":     true,
		"reply":       result["title"],
		"translation": result,
		"protocol":    protocol,
		"model":       cfg.CloudModel,
	})
}

func aiConfigFromRequest(c *gin.Context) (service.AIConfig, error) {
	cfg := service.LoadAIConfig()
	if c.Request == nil || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return cfg, nil
	}
	var submitted service.AIConfig
	if err := c.ShouldBindJSON(&submitted); err != nil {
		if errors.Is(err, io.EOF) {
			return cfg, nil
		}
		return service.AIConfig{}, err
	}
	return submitted, nil
}

func mergeMaskedAIKey(cfg service.AIConfig) service.AIConfig {
	if cfg.CloudAPIKey == "" || strings.Contains(cfg.CloudAPIKey, "****") {
		cfg.CloudAPIKey = service.LoadAIConfig().CloudAPIKey
	}
	return cfg
}

func writeAIError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	body := gin.H{
		"success": false,
		"error":   err.Error(),
	}
	var providerErr *service.AIProviderError
	if errors.As(err, &providerErr) {
		body["errorType"] = providerErr.Kind
		if providerErr.StatusCode > 0 {
			body["statusCode"] = providerErr.StatusCode
		}
		if providerErr.RequestID != "" {
			body["requestId"] = providerErr.RequestID
		}
		switch providerErr.Kind {
		case service.AIErrorInvalidConfig, service.AIErrorInvalidRequest, service.AIErrorModelUnavailable:
			status = http.StatusBadRequest
		case service.AIErrorAuthentication:
			status = http.StatusUnauthorized
		case service.AIErrorRateLimited:
			status = http.StatusTooManyRequests
		case service.AIErrorTimeout:
			status = http.StatusGatewayTimeout
		case service.AIErrorEndpointNotFound:
			status = http.StatusBadGateway
		default:
			if providerErr.StatusCode >= 500 && providerErr.StatusCode <= 599 {
				status = providerErr.StatusCode
			}
		}
	}
	c.JSON(status, body)
}
