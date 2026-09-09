package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"RJDJ/backend/internal/middleware"
	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/okx"
	"RJDJ/backend/internal/util"
)

// OkxDataHandler 拉取 OKX 数据（账户/行情）供看板与 AI 使用
type OkxDataHandler struct {
	*OkxConfigHandler // 复用配置管理与凭证加载能力
}

func NewOkxDataHandler(cfgH *OkxConfigHandler) *OkxDataHandler {
	return &OkxDataHandler{OkxConfigHandler: cfgH}
}

var errNoConfig = errors.New("尚未配置 OKX API Key，请先在「OKX 配置」中添加")

// listConfigsOf 返回某用户全部配置（按默认优先）
func (h *OkxConfigHandler) listConfigs(uid uint) []model.OkxConfig {
	var cfgs []model.OkxConfig
	h.db.Where("user_id = ?", uid).Order("is_default desc, id asc").Find(&cfgs)
	return cfgs
}

// resolveCredential 解析并解密一个 OKX 凭证：config_id 优先，其次默认配置
func (h *OkxDataHandler) resolveCredential(c *gin.Context) (*okx.Credentials, error) {
	uid := middleware.CurrentUser(c)
	param := c.Query("config_id")
	var targetID int
	if param != "" {
		targetID, _ = strconv.Atoi(param)
	}

	if targetID > 0 {
		cred, err := h.loadCredential(uid, targetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("配置不存在")
			}
			return nil, err
		}
		return cred, nil
	}

	// 取默认配置或唯一配置
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
	cred, err := h.loadCredential(uid, int(chosen.ID))
	if err != nil {
		return nil, err
	}
	return cred, nil
}

// resolveConfig 解析并解密一个 OKX 配置，返回配置对象与解密后的凭证。
// config_id 优先，其次默认配置。与 resolveCredential 逻辑一致，额外返回 cfg 供缓存归属。
func (h *OkxDataHandler) resolveConfig(c *gin.Context) (*model.OkxConfig, *okx.Credentials, error) {
	uid := middleware.CurrentUser(c)
	param := c.Query("config_id")
	var targetID int
	if param != "" {
		targetID, _ = strconv.Atoi(param)
	}
	if targetID > 0 {
		var cg model.OkxConfig
		if err := h.db.Where("id = ? AND user_id = ?", targetID, uid).First(&cg).Error; err != nil {
			return nil, nil, errors.New("配置不存在")
		}
		cred, err := h.decryptCred(&cg)
		if err != nil {
			return nil, nil, err
		}
		return &cg, cred, nil
	}
	cfgs := h.listConfigs(uid)
	if len(cfgs) == 0 {
		return nil, nil, errNoConfig
	}
	chosen := cfgs[0]
	for _, cg := range cfgs {
		if cg.IsDefault {
			chosen = cg
			break
		}
	}
	cred, err := h.decryptCred(&chosen)
	if err != nil {
		return nil, nil, err
	}
	return &chosen, cred, nil
}

func (h *OkxDataHandler) decryptCred(cg *model.OkxConfig) (*okx.Credentials, error) {
	key, err := util.DecryptSecret(h.cfg.EncryptKey, cg.ApiKey)
	if err != nil {
		return nil, err
	}
	secret, err := util.DecryptSecret(h.cfg.EncryptKey, cg.ApiSecret)
	if err != nil {
		return nil, err
	}
	phrase, err := util.DecryptSecret(h.cfg.EncryptKey, cg.Passphrase)
	if err != nil {
		return nil, err
	}
	return &okx.Credentials{
		ApiKey:     key,
		ApiSecret:  secret,
		Passphrase: phrase,
		BaseURL:    normalizeBaseURL(cg.BaseURL),
	}, nil
}

// Overview 账户总览：余额 + 持仓
func (h *OkxDataHandler) Overview(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	bal, err := client.GetAccountBalance()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "拉取余额失败: " + err.Error()})
		return
	}
	positions, err := client.GetPositions("")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "拉取持仓失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"balance":    bal,
		"positions":  positions,
		"updated_at": time.Now().Unix(),
	})
}

// Positions 仅持仓
func (h *OkxDataHandler) Positions(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	positions, err := client.GetPositions(c.Query("inst_type"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "拉取持仓失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": positions})
}

// Ticker 单个行情
func (h *OkxDataHandler) Ticker(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	instId := c.Query("inst_id")
	if instId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inst_id 必填"})
		return
	}
	client := h.buildOKXClient(cred)
	t, err := client.GetTicker(instId)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// Tickers 全部行情
func (h *OkxDataHandler) Tickers(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	ts, err := client.GetTickers(c.Query("inst_type"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": ts})
}

// Candles K线
func (h *OkxDataHandler) Candles(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	instId := c.Query("inst_id")
	if instId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inst_id 必填"})
		return
	}
	client := h.buildOKXClient(cred)
	limit := atoiDefault(c.Query("limit"), 100)
	if limit > 300 {
		limit = 300
	}
	klines, err := client.GetCandles(instId, c.Query("bar"), limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": klines})
}

// PnlDaily 合约历史盈亏：每日已实现盈亏 + 各标的币盈亏（账单流水聚合，同步版，前端已改走异步任务）
// 参数: days=回溯天数(默认90,上限180)  asset=可选标的币筛选(如 BTC/ETH)，填了则 Daily/ByAsset 只含该币
func (h *OkxDataHandler) PnlDaily(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	days := atoiDefault(c.Query("days"), 90)
	if days > 180 {
		days = 180
	}
	bills, err := client.GetContractBills(days, 120)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "拉取账单流水失败: " + err.Error()})
		return
	}
	asset := strings.ToUpper(strings.TrimSpace(c.Query("asset")))
	var summary okx.PnlSummary
	if asset != "" {
		summary = okx.AggregatePnl(okx.FilterBillsByAsset(bills, asset))
	} else {
		summary = okx.AggregatePnl(bills)
	}
	c.JSON(http.StatusOK, gin.H{
		"summary": summary,
		"days":    days,
		"asset":   asset,
	})
}

// Strategies 我的策略：聚合网格/马丁DCA/定投/信号/条件单（运行中或历史）
func (h *OkxDataHandler) Strategies(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	history := c.Query("history") == "1"
	kind := c.Query("kind") // 可选：grid/dca/recurring/signal/algo/all

	var data map[string][]okx.BotStrategy
	switch kind {
	case "grid":
		data = map[string][]okx.BotStrategy{"grid": fetchSafe(func() ([]okx.BotStrategy, error) {
			return client.GetGridStrategies(history, c.Query("inst_type"))
		})}
	case "dca":
		data = map[string][]okx.BotStrategy{"dca": fetchSafe(func() ([]okx.BotStrategy, error) {
			return client.GetDcaStrategies(history, c.Query("inst_type"))
		})}
	case "recurring":
		data = map[string][]okx.BotStrategy{"recurring": fetchSafe(func() ([]okx.BotStrategy, error) {
			return client.GetRecurringStrategies(history)
		})}
	case "signal":
		data = map[string][]okx.BotStrategy{"signal": fetchSafe(func() ([]okx.BotStrategy, error) {
			return client.GetSignalStrategies(history)
		})}
	case "algo":
		data = map[string][]okx.BotStrategy{"algo": fetchSafe(func() ([]okx.BotStrategy, error) {
			return client.GetAlgoStrategies(history, "", c.Query("inst_type"))
		})}
	default: // all / 空
		data = client.GetAllStrategies(history)
	}
	// 汇总计数
	var total int
	for _, l := range data {
		total += len(l)
	}
	c.JSON(http.StatusOK, gin.H{"list": data, "total": total, "history": history})
}

// fetchSafe 忽略单个来源错误，失败返回空切片
func fetchSafe(fn func() ([]okx.BotStrategy, error)) []okx.BotStrategy {
	v, err := fn()
	if err != nil {
		return []okx.BotStrategy{}
	}
	return v
}

// GridPositions 网格策略持仓/收益
func (h *OkxDataHandler) GridPositions(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client := h.buildOKXClient(cred)
	res, err := client.GetGridPositions(c.Query("algo_id"), c.Query("inst_id"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": res})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
