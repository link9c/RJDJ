package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"RJDJ/backend/internal/ai"
	"RJDJ/backend/internal/config"
	"RJDJ/backend/internal/middleware"
	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/okx"
)

// AIHandler AI 对话分析
type AIHandler struct {
	db       *gorm.DB
	cfg      *config.Config
	settings *SettingsHandler
	data     *OkxDataHandler
}

func NewAIHandler(db *gorm.DB, cfg *config.Config, settingsH *SettingsHandler, dataH *OkxDataHandler) *AIHandler {
	return &AIHandler{db: db, cfg: cfg, settings: settingsH, data: dataH}
}

// buildClient 构造 LLM 客户端：优先取网页入库的 AI 配置，缺省回退 .env 环境变量
func (h *AIHandler) buildClient() (*ai.Client, error) {
	provider, baseURL, model, apiKey := h.settings.LoadAIConfig()
	// 回退 .env
	if apiKey == "" {
		provider, baseURL, model, apiKey = h.cfg.LLMProvider, h.cfg.LLMBaseURL, h.cfg.LLMModel, h.cfg.LLMAPIKey
	}
	return buildAIClient(provider, baseURL, model, apiKey)
}

// normalizeLLMEndpoint 兼容 base_url 与完整 endpoint 两种写法
func normalizeLLMEndpoint(u string) string {
	u = strings.TrimRight(u, "/")
	if strings.Contains(u, "/chat/completions") {
		return u
	}
	if strings.HasSuffix(u, "/v1") {
		return u + "/chat/completions"
	}
	if strings.Contains(u, "openai.com") || strings.Contains(u, "deepseek.com") ||
		strings.Contains(u, "moonshot") || strings.Contains(u, "dashscope") ||
		strings.Contains(u, "volces.com") || strings.Contains(u, "api.anthropic") {
		return u + "/v1/chat/completions"
	}
	// 其他假定用户给了完整 base（如 /v1/chat/completions 变体）
	if strings.Contains(u, "chat") {
		return u
	}
	return u + "/chat/completions"
}

type chatReq struct {
	Message   string `json:"message" binding:"required"`
	ConfigID  uint   `json:"config_id"`
	HistoryID uint   `json:"history_id"` // 预留：分组会话
}

// Chat 处理一次对话（含多轮工具调用）
func (h *AIHandler) Chat(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	llm, err := h.buildClient()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 关联的配置ID（用户消息里可能带，若无则用解析默认）
	configID := req.ConfigID

	// 构造历史：最近 20 条
	history := h.loadHistory(uid, configID, 20)
	msgs := make([]ai.Message, 0, len(history)+1)
	for _, m := range history {
		msgs = append(msgs, ai.Message{Role: m.Role, Content: ai.StrPtr(m.Content)})
	}
	msgs = append(msgs, ai.Message{Role: "user", Content: ai.StrPtr(req.Message)})

	// 持久化用户消息
	h.db.Create(&model.ChatMessage{UserID: uid, ConfigID: configID, Role: "user", Content: req.Message})

	// 工具执行器：基于请求选中的配置拉 OKX
	executor := func(name, args string) (string, error) {
		return h.runTool(c, uid, configID, name, args)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 150*time.Second)
	defer cancel()
	systemMsg := "你是专业的加密货币量化交易分析助手。用户已授权你访问其 OKX 账户。当需要账户资产、持仓或行情数据时，请先调用对应工具获取真实数据，再基于数据做分析回答。请用简体中文、口语化专业地回答。注意所有金额均为OKX返回数值。"
	finalMsgs := []ai.Message{{Role: "system", Content: ai.StrPtr(systemMsg)}}
	finalMsgs = append(finalMsgs, msgs...)

	answer, allMsgs, err := llm.Chat(ctx, finalMsgs, executor)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	// 持久化 AI 回复
	h.db.Create(&model.ChatMessage{UserID: uid, ConfigID: configID, Role: "assistant", Content: answer})

	_ = allMsgs
	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

// runTool 执行某个 OKX 工具调用，返回给模型的文本
func (h *AIHandler) runTool(c *gin.Context, uid uint, configID uint, name, args string) (string, error) {
	// 若有指定 config_id 直接用它，否则解析默认
	cred, err := h.data.resolveCredentialWithID(uid, configID)
	if err != nil {
		return "", err
	}
	client := h.data.buildOKXClient(cred)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(args), &parsed)

	switch name {
	case "get_account_overview":
		bal, err := client.GetAccountBalance()
		if err != nil {
			return "", err
		}
		pos, err := client.GetPositions("")
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(gin.H{"balance": bal, "positions": pos})
		return string(out), nil
	case "get_positions":
		instType, _ := parsed["inst_type"].(string)
		pos, err := client.GetPositions(instType)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(gin.H{"list": pos})
		return string(out), nil
	case "get_ticker":
		instID, _ := parsed["inst_id"].(string)
		t, err := client.GetTicker(instID)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(t)
		return string(out), nil
	case "get_candles":
		instID, _ := parsed["inst_id"].(string)
		bar, _ := parsed["bar"].(string)
		limit := 100
		if v, ok := parsed["limit"].(float64); ok {
			limit = int(v)
		}
		kl, err := client.GetCandles(instID, bar, limit)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(gin.H{"list": kl})
		return string(out), nil
	case "get_strategies":
		history := false
		if v, ok := parsed["history"].(float64); ok && v == 1 {
			history = true
		}
		res := client.GetAllStrategies(history)
		out, _ := json.Marshal(gin.H{"list": res, "history": history})
		return string(out), nil
	}
	return "", errors.New("未知工具: " + name)
}

// resolveCredentialWithID 优先指定 ID，其次默认
func (h *OkxDataHandler) resolveCredentialWithID(uid uint, configID uint) (*okx.Credentials, error) {
	if configID > 0 {
		return h.loadCredential(uid, int(configID))
	}
	cfgs := h.listConfigs(uid)
	if len(cfgs) == 0 {
		return nil, errNoConfig
	}
	chosen := cfgs[0]
	for _, cg := range cfgs {
		if cg.IsDefault {
			chosen = cg
			break
		}
	}
	return h.loadCredential(uid, int(chosen.ID))
}

// loadHistory 加载历史消息
func (h *AIHandler) loadHistory(uid uint, configID uint, limit int) []model.ChatMessage {
	var msgs []model.ChatMessage
	q := h.db.Where("user_id = ?", uid).Order("id desc")
	if configID > 0 {
		q = q.Where("config_id = ?", configID)
	}
	q.Limit(limit).Find(&msgs)
	// 反转为正序
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs
}

// History 返回对话历史（用于前端回显）
func (h *AIHandler) History(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	var configID uint
	if v := c.Query("config_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			configID = uint(n)
		}
	}
	limit := 50
	msgs := h.loadHistory(uid, configID, limit)
	views := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		views = append(views, gin.H{
			"id": m.ID, "role": m.Role, "content": m.Content,
			"config_id": m.ConfigID, "created_at": m.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"list": views})
}
