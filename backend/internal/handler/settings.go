package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"RJDJ/backend/internal/ai"
	"RJDJ/backend/internal/model"
)

// SettingsHandler 系统设置（如 AI 大模型配置），支持网页编辑入库
type SettingsHandler struct {
	db *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

const (
	KeyAIProvider = "ai.provider"
	KeyAIBaseURL  = "ai.base_url"
	KeyAIModel    = "ai.model"
	KeyAIAPIKey   = "ai.api_key"
)

// AISettingReq AI 配置请求体。api_key 留空表示保留原值（不明文回显）
type AISettingReq struct {
	Provider string `json:"provider"` // openai / deepseek / 其它，标识用
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

func (h *SettingsHandler) get(key string) string {
	var s model.AppSetting
	if err := h.db.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func (h *SettingsHandler) set(key, value string) {
	if value == "" {
		h.db.Where("key = ?", key).Delete(&model.AppSetting{})
		return
	}
	var s model.AppSetting
	err := h.db.Where("key = ?", key).First(&s).Error
	if err != nil {
		h.db.Create(&model.AppSetting{Key: key, Value: value})
	} else {
		s.Value = value
		h.db.Save(&s)
	}
}

// GetAI 读取 AI 配置（含掩码后的 key）
func (h *SettingsHandler) GetAI(c *gin.Context) {
	apiKey := h.get(KeyAIAPIKey)
	c.JSON(http.StatusOK, gin.H{
		"provider": h.get(KeyAIProvider),
		"base_url": h.get(KeyAIBaseURL),
		"model":    h.get(KeyAIModel),
		"api_key":  maskSecret(apiKey),
		"has_key":  apiKey != "",
	})
}

// UpdateAI 保存 AI 配置
func (h *SettingsHandler) UpdateAI(c *gin.Context) {
	var req AISettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	h.set(KeyAIProvider, req.Provider)
	h.set(KeyAIBaseURL, req.BaseURL)
	h.set(KeyAIModel, req.Model)
	// api_key 留空则保留原值
	if req.APIKey != "" {
		h.set(KeyAIAPIKey, req.APIKey)
	}
	// 测试连通性
	testMsg := ""
	if err := h.testAIConnection(c, req); err != nil {
		testMsg = err.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "已保存",
		"test_error": testMsg,
		"ai": gin.H{
			"provider": req.Provider,
			"base_url": req.BaseURL,
			"model":    req.Model,
			"has_key":  req.APIKey != "" || h.get(KeyAIAPIKey) != "",
		},
	})
}

// testAIConnection 用当前配置尝试一次最小 chat 请求
func (h *SettingsHandler) testAIConnection(c *gin.Context, req AISettingReq) error {
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = h.get(KeyAIAPIKey)
	}
	if apiKey == "" {
		return nil
	}
	client, err := buildAIClient(req.Provider, req.BaseURL, req.Model, apiKey)
	if err != nil {
		return err
	}
	_, _, err = client.Ping(c.Request.Context())
	return err
}

// LoadAIConfig 供 AI handler 复用：从 DB 读，缺省回退 env
func (h *SettingsHandler) LoadAIConfig() (provider, baseURL, model, apiKey string) {
	return h.get(KeyAIProvider), h.get(KeyAIBaseURL), h.get(KeyAIModel), h.get(KeyAIAPIKey)
}

// buildAIClient 由配置构造 LLM 客户端（供测试与对话复用）
func buildAIClient(provider, baseURL, model, apiKey string) (*ai.Client, error) {
	if apiKey == "" {
		return nil, errors.New("尚未配置大模型 API Key，请先在「AI 配置」中填写")
	}
	base := baseURL
	if base == "" {
		base = "https://api.openai.com/v1/chat/completions"
	}
	base = normalizeLLMEndpoint(base)
	if model == "" {
		model = "gpt-4o-mini"
	}
	client := ai.NewClient(ai.Provider{
		BaseURL: base,
		APIKey:  apiKey,
		Model:   model,
	})
	client.RegisterTools(ai.BuildAccountTools()...)
	return client, nil
}

// maskSecret 仅展示首尾两字符
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
