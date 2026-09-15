package handler

import (
	"errors"
	"math"
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

// StrategyDetail 单个策略详情 + 止盈测算
// 参数: algo_id(必填) kind(grid/dca/recurring/signal/algo) history=0|1 inst_type
func (h *OkxDataHandler) StrategyDetail(c *gin.Context) {
	cred, err := h.resolveCredential(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	algoID := c.Query("algo_id")
	if algoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "algo_id 必填"})
		return
	}
	kind := c.Query("kind")
	history := c.Query("history") == "1"
	client := h.buildOKXClient(cred)

	// 1) 先试详情接口（主要对运行中策略有效）
	detail, derr := client.GetStrategyDetail(kind, algoID)
	fallback := derr != nil
	if fallback {
		// 2) 兜底：从运行中/历史列表按 algoId 捞条目（详情接口对已结束策略常返回空）
		detail = h.findStrategyFromLists(client, kind, algoID, history)
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该策略（可能已被删除，或该策略类型暂不支持详情查询）"})
		return
	}
	if detail.InstType == "" {
		detail.InstType = strings.ToUpper(c.Query("inst_type"))
	}
	if detail.InstID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "策略详情缺少 instId，无法拉取行情与测算"})
		return
	}

	// 3) 持仓量与均价。
	// 马丁(DCA)：机器人仓位不在 account/positions 里，必须用 tradingBot/dca/position-details
	// （含开仓均价/持仓量/止盈价/强平价/费用），已停止则用最近周期 + 成交记录重建。
	currentPx := ""
	isContract := detail.InstType == "SWAP" || detail.InstType == "FUTURES"
	dcaHandled := false
	if kind == "dca" {
		dcaHandled = h.enrichDcaTradingData(client, detail)
	}
	if !dcaHandled {
		if isContract {
			if positions, err := client.GetPositions(detail.InstType); err == nil {
				for _, p := range positions {
					if p.InstId != detail.InstID {
						continue
					}
					// 开平仓模式下按方向匹配；净仓模式 posSide=net
					if detail.Direction == "long" && strings.EqualFold(p.PosSide, "short") {
						continue
					}
					if detail.Direction == "short" && strings.EqualFold(p.PosSide, "long") {
						continue
					}
					posSz, _ := strconv.ParseFloat(p.Pos, 64)
					if math.Abs(posSz) == 0 {
						continue
					}
					detail.PosContracts = strconv.FormatFloat(math.Abs(posSz), 'f', -1, 64)
					if detail.AvgPx == "" {
						detail.AvgPx = p.AvgPx
					}
					if currentPx == "" && p.MarkPx != "" {
						currentPx = p.MarkPx
					}
					break
				}
			}
		} else if base := baseCcy(detail.InstID); base != "" {
			// 现货：从资金账户余额取持币量（无持仓均价，前端支持手填成本价）
			if bal, err := client.GetAccountBalance(); err == nil {
				for _, coin := range bal.Details {
					if strings.EqualFold(coin.Ccy, base) {
						if eq, err := strconv.ParseFloat(coin.Eq, 64); err == nil && eq > 0 {
							detail.BaseSz = coin.Eq
						}
						break
					}
				}
			}
		}
	}

	// 4) 最新价（持仓里没拿到标记价时）
	if currentPx == "" {
		if t, err := client.GetTicker(detail.InstID); err == nil && t != nil {
			currentPx = t.Last
		}
	}

	// 5) 合约面值：详情自带优先，缺失再查产品配置
	ctVal := detail.CtVal
	if isContract && ctVal == "" {
		if in, err := client.GetInstrument(detail.InstID); err == nil && in != nil {
			ctVal = in.CtVal
		}
	}

	// 6) 止盈测算
	okx.BuildDetailCalc(detail, currentPx, ctVal)

	c.JSON(http.StatusOK, gin.H{
		"detail":     detail,
		"fallback":   fallback, // true=详情接口无数据，结果来自列表
		"current_px": currentPx,
		"ct_val":     ctVal,
	})
}

// enrichDcaTradingData 用马丁专属接口（position-details / cycle-list / orders）
// 填充当前持仓的开仓均价、持仓量、止盈价及成交记录。
// 机器人仓位不体现在 account/positions 中，因此该函数处理后无需再走账户持仓兜底；
// 返回 true 表示这是 DCA 策略（含查不到数据的已停止策略）。
func (h *OkxDataHandler) enrichDcaTradingData(client *okx.Client, d *okx.StrategyDetail) bool {
	isContract := d.InstType == "SWAP" || d.InstType == "FUTURES" ||
		strings.Contains(d.AlgoOrdType, "contract")

	// 1) 当前周期持仓
	pos, ordType, perr := client.GetDcaPositionDetail(d.AlgoID, d.AlgoOrdType)
	cycles, cycOt, _ := client.GetDcaCycles(d.AlgoID, firstNonEmpty(ordType, d.AlgoOrdType), 50)
	d.Cycles = cycles

	if perr == nil && pos != nil {
		d.Position = pos
		d.PosSource = "position"
		d.CycleID = pos.CurCycleID
		d.AvgPx = pos.AvgPx
		d.TpTriggerPx = pos.TpPx
		if pos.SlPx != "" {
			d.SlTriggerPx = pos.SlPx
		}
		if isContract {
			d.PosContracts = pos.Sz
		} else if pos.BaseSz != "" {
			d.BaseSz = pos.BaseSz
		}
		d.NotionalUsd = pos.NotionalUsd
		if pos.Upl != "" {
			d.Upl = pos.Upl
		}
		ot := firstNonEmpty(ordType, cycOt, d.AlgoOrdType)
		if pos.CurCycleID != "" {
			if orders, err := client.GetDcaOrders(d.AlgoID, ot, pos.CurCycleID, 100); err == nil {
				d.Orders = orders
				if d.CtVal == "" {
					d.CtVal = firstOrderCtVal(orders)
				}
				if d.TpTriggerPx == "" {
					d.TpTriggerPx = tpPxFromOrders(orders)
				}
			}
		}
		return true
	}

	// 2) 无当前持仓（已停止或等待下一轮开仓）：取最近一个有均价的周期，用成交记录重建
	d.PosSource = "none"
	for _, cy := range cycles {
		if cy.AvgPx == "" {
			continue
		}
		ot := firstNonEmpty(cycOt, ordType, d.AlgoOrdType)
		orders, err := client.GetDcaOrders(d.AlgoID, ot, cy.CycleID, 100)
		if err != nil {
			continue
		}
		d.PosSource = "cycle"
		d.CycleID = cy.CycleID
		d.AvgPx = cy.AvgPx
		d.TpTriggerPx = firstNonEmpty(cy.TpPx, tpPxFromOrders(orders))
		if cy.RealizedPnl != "" {
			d.RealizedPnl = cy.RealizedPnl
		}
		d.Orders = orders
		if d.CtVal == "" {
			d.CtVal = firstOrderCtVal(orders)
		}
		// 该周期开仓单（初始+加仓+手动加仓）累计成交量即为周期持仓量
		var openQty float64
		for _, o := range orders {
			if !isDcaOpeningOrder(o.OrdType) || o.State != "filled" {
				continue
			}
			openQty += parseFloat(o.FilledSz)
		}
		if openQty > 0 {
			if isContract {
				d.PosContracts = strconv.FormatFloat(openQty, 'f', -1, 64)
			} else {
				d.BaseSz = strconv.FormatFloat(openQty, 'f', -1, 64)
			}
		}
		break
	}
	return true
}

// isDcaOpeningOrder 是否为开仓类子订单（止盈/止损/平仓单不计入持仓）
func isDcaOpeningOrder(t string) bool {
	switch t {
	case "init_order", "safety_order", "manual_add_order":
		return true
	}
	return false
}

// firstOrderCtVal 从成交记录里取合约面值
func firstOrderCtVal(orders []okx.DcaSubOrder) string {
	for _, o := range orders {
		if o.CtVal != "" && o.CtVal != "0" {
			return o.CtVal
		}
	}
	return ""
}

// tpPxFromOrders 从成交记录里推断止盈价：优先未成交止盈单挂价，其次已成交止盈单均价
func tpPxFromOrders(orders []okx.DcaSubOrder) string {
	for _, o := range orders {
		if o.OrdType == "tp_order" && o.State == "live" && o.Px != "" {
			return o.Px
		}
	}
	for _, o := range orders {
		if (o.OrdType == "tp_order" || o.OrdType == "close_position") && o.State == "filled" && o.AvgFillPx != "" {
			return o.AvgFillPx
		}
	}
	return ""
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// findStrategyFromLists 从策略列表（先运行中后历史）中按 algoId 找条目并包装为详情。
func (h *OkxDataHandler) findStrategyFromLists(client *okx.Client, kind, algoID string, historyFirst bool) *okx.StrategyDetail {
	tryOrders := []bool{historyFirst, !historyFirst}
	for _, hist := range tryOrders {
		var groups map[string][]okx.BotStrategy
		if kind == "" || kind == "all" || kind == "algo" {
			groups = client.GetAllStrategies(hist)
		} else {
			groups = map[string][]okx.BotStrategy{}
			switch kind {
			case "grid":
				groups["grid"], _ = client.GetGridStrategies(hist, "")
			case "dca":
				groups["dca"], _ = client.GetDcaStrategies(hist, "")
			case "recurring":
				groups["recurring"], _ = client.GetRecurringStrategies(hist)
			case "signal":
				groups["signal"], _ = client.GetSignalStrategies(hist)
			}
		}
		for _, list := range groups {
			for i := range list {
				if list[i].AlgoID == algoID {
					s := list[i]
					return &okx.StrategyDetail{BotStrategy: s}
				}
			}
		}
	}
	return nil
}

// baseCcy 从 instId 提取标的币：BTC-USDT-SWAP → BTC。
func baseCcy(instId string) string {
	parts := strings.Split(instId, "-")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
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
