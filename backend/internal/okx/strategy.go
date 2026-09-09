package okx

import (
	"encoding/json"
	"net/url"
)

// ============= 策略 / Trading Bot 查询（只读） =============
// OKX 把自动化策略归为 tradingBot 下的几类，注意它们列表接口的后缀并不统一：
//   - 网格 Grid、定投 Recurring、信号 Signal：POST 创建 + GET `orders-algo-pending`(运行中) / `orders-algo-history`(历史)
//   - 马丁格尔 DCA(Martingale)：GET `ongoing-list`(运行中) / `history-list`(历史)，且 algoOrdType 必填
//   - 策略委托 Algo(条件单/止盈止损/trailing)：GET /api/v5/trade/orders-algo-pending|history
//
// algoOrdType 枚举（用户 App「策略」里建的类型）：
//   grid=现货网格 contract_grid=合约网格 spot_dca=现货马丁 contract_dca=合约马丁(即 DCA/Martingale)
//   recurring=定投 signal_* =信号 以及条件单 conditional / oco / trailing / move_order_stop 等
//
// instType 枚举：SPOT=现货 SWAP=永续(合约) FUTURES=交割(合约) OPTION=期权

// BotStrategy 统一封装的策略条目（多来源映射到通用结构，便于前端/AI 展示）
type BotStrategy struct {
	AlgoID      string `json:"algoId"`
	AlgoOrdType string `json:"algoOrdType,omitempty"` // grid / contract_grid / spot_dca / contract_dca / recurring ...
	AlgoClOrdType string `json:"algoClOrdType,omitempty"` // DCA 返回的类型字段
	InstType    string `json:"instType,omitempty"`    // SPOT / SWAP / FUTURES
	InstID      string `json:"instId,omitempty"`
	Ccy         string `json:"ccy,omitempty"`
	Side        string `json:"side,omitempty"`
	State       string `json:"state,omitempty"` // live / running / effective / paused ...
	Type        string `json:"type,omitempty"`  // 策略大类标注：grid/dca/recurring/signal/algo

	// 展示字段（各来源尽量填充）
	Direction  string `json:"direction,omitempty"`  // long / short / neutral
	Lever      string `json:"lever,omitempty"`
	MaxPx      string `json:"maxPx,omitempty"`
	MinPx      string `json:"minPx,omitempty"`
	GridNum    string `json:"gridNum,omitempty"`
	RunType    string `json:"runType,omitempty"`
	QuoteSz    string `json:"quoteSz,omitempty"` // 网格投入
	InvestAmt  string `json:"investAmt,omitempty"` // DCA/定投投入或保证金
	BaseSz     string `json:"baseSz,omitempty"` // DCA 首单
	SafetyOrderSz string `json:"safetyOrderSz,omitempty"` // DCA 补仓单
	MaxSafetyOrders string `json:"maxSafetyOrders,omitempty"` // DCA 最大补仓次数
	TotalPnl   string `json:"totalPnl,omitempty"`   // 累计已实现+浮动收益
	PnlRatio   string `json:"pnlRatio,omitempty"`
	ClosePnl   string `json:"closePnl,omitempty"`
	Upl        string `json:"upl,omitempty"`
	TriggerPx  string `json:"triggerPx,omitempty"`
	OrdPx      string `json:"ordPx,omitempty"`
	CreatedAt  string `json:"cTime,omitempty"`
	UpdatedAt  string `json:"uTime,omitempty"`
}

// StrategyResp 保留原始 data（AI 可能需要更多字段）
type StrategyResp struct {
	List []BotStrategy   `json:"list"`
	Raw  []json.RawMessage `json:"raw,omitempty"`
}

// listSuffix pending 与 history 的实际路径结尾
type botListPath struct {
	prefix  string
	pending string
	history string
}

// getBotRaw GET 拉取 bot 列表，返回原始 data；pending/history 用各自真实后缀
func (c *Client) getBotRaw(pl botListPath, history bool, params map[string]string) ([]json.RawMessage, error) {
	seg := pl.pending
	if history {
		seg = pl.history
	}
	uri := pl.prefix + "/" + seg
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	q.Set("limit", "20")
	return c.Get(uri, q)
}

// fetchBotKind 拉取一类 bot 并归一化，失败返回 nil
func (c *Client) fetchBotKind(pl botListPath, history bool, kind, algoOrdType, instType string, params map[string]string) ([]BotStrategy, error) {
	p := map[string]string{}
	if params != nil {
		for k, v := range params {
			p[k] = v
		}
	}
	if algoOrdType != "" {
		p["algoOrdType"] = algoOrdType
	}
	if instType != "" {
		p["instType"] = instType
	}
	data, err := c.getBotRaw(pl, history, p)
	if err != nil {
		return nil, err
	}
	list := make([]BotStrategy, 0, len(data))
	for _, raw := range data {
		var s BotStrategy
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		s.Type = kind
		if s.AlgoOrdType == "" {
			s.AlgoOrdType = algoOrdType
		}
		list = append(list, s)
	}
	return list, nil
}

// GetGridStrategies 网格策略：现货(grid) + 合约(contract_grid)。以 algoOrdType 区分现货/合约。
func (c *Client) GetGridStrategies(history bool, instType string) ([]BotStrategy, error) {
	pl := botListPath{prefix: "/api/v5/tradingBot/grid", pending: "orders-algo-pending", history: "orders-algo-history"}
	var algoTypes []string
	switch instType {
	case "SPOT":
		algoTypes = []string{"grid"}
	case "SWAP", "FUTURES":
		algoTypes = []string{"contract_grid"}
	default:
		algoTypes = []string{"grid", "contract_grid"}
	}
	out := []BotStrategy{}
	for _, at := range algoTypes {
		if v, err := c.fetchBotKind(pl, history, "grid", at, "", nil); err == nil {
			out = append(out, v...)
		}
	}
	return out, nil
}

// GetDcaStrategies 马丁格尔(DCA / Martingale)：现货 spot_dca 或 合约 contract_dca
// OKX 独立路径：GET /api/v5/tradingBot/dca/ongoing-list(运行中) / history-list(历史)
// 注意：此接口仅以 algoOrdType 区分现货/合约，勿再额外传 instType。
func (c *Client) GetDcaStrategies(history bool, instType string) ([]BotStrategy, error) {
	pl := botListPath{prefix: "/api/v5/tradingBot/dca", pending: "ongoing-list", history: "history-list"}
	// 现货与合约的 algoOrdType 不同，按需拉取；未指定 instType 时两批都拉
	var algoTypes []string
	switch instType {
	case "SPOT":
		algoTypes = []string{"spot_dca"}
	case "SWAP", "FUTURES":
		algoTypes = []string{"contract_dca"}
	default:
		algoTypes = []string{"spot_dca", "contract_dca"}
	}
	out := []BotStrategy{}
	for _, at := range algoTypes {
		if v, err := c.fetchBotKind(pl, history, "dca", at, "", nil); err == nil {
			out = append(out, v...)
		}
	}
	return out, nil
}

// GetRecurringStrategies 定投 recurring buy
func (c *Client) GetRecurringStrategies(history bool) ([]BotStrategy, error) {
	pl := botListPath{prefix: "/api/v5/tradingBot/recurring", pending: "orders-algo-pending", history: "orders-algo-history"}
	return c.fetchBotKind(pl, history, "recurring", "recurring", "", nil)
}

// GetSignalStrategies 信号机器人
func (c *Client) GetSignalStrategies(history bool) ([]BotStrategy, error) {
	pl := botListPath{prefix: "/api/v5/tradingBot/signal", pending: "orders-algo-pending", history: "orders-algo-history"}
	return c.fetchBotKind(pl, history, "signal", "", "", nil)
}

// GetAlgoStrategies 策略委托/条件单（algo: conditional / oco / trailing / move_order_stop / iceberg / twap）
// ordType 可选过滤（如 conditional / trailing）；不传则接口返回全部 algo 待触发单
func (c *Client) GetAlgoStrategies(history bool, ordType, instType string) ([]BotStrategy, error) {
	pl := botListPath{prefix: "/api/v5/trade", pending: "orders-algo-pending", history: "orders-algo-history"}
	p := map[string]string{"instType": instType}
	if ordType != "" {
		p["ordType"] = ordType
	}
	data, err := c.getBotRaw(pl, history, p)
	if err != nil {
		return nil, err
	}
	out := make([]BotStrategy, 0, len(data))
	for _, raw := range data {
		var s BotStrategy
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		s.Type = "algo"
		if s.AlgoOrdType == "" {
			s.AlgoOrdType = ordType
		}
		out = append(out, s)
	}
	return out, nil
}

// GetAllStrategies 聚合所有可获取的策略（网格 + 马丁DCA + 定投 + 信号 + algo 条件单）
// 某类失败不影响其它；返回 map: {"grid": [...], "dca": [...], "recurring": [...], "signal": [...], "algo": [...]}
func (c *Client) GetAllStrategies(history bool) map[string][]BotStrategy {
	out := map[string][]BotStrategy{}
	if v, err := c.GetGridStrategies(history, ""); err == nil {
		out["grid"] = v
	}
	if v, err := c.GetDcaStrategies(history, ""); err == nil {
		out["dca"] = v
	}
	if v, err := c.GetRecurringStrategies(history); err == nil {
		out["recurring"] = v
	}
	if v, err := c.GetSignalStrategies(history); err == nil {
		out["signal"] = v
	}
	if v, err := c.GetAlgoStrategies(history, "", ""); err == nil {
		out["algo"] = v
	}
	return out
}

// GetGridPositions 网格策略持仓/收益（同可用于按 algoId 查其他策略收益）
func (c *Client) GetGridPositions(algoId, instId string) ([]json.RawMessage, error) {
	q := url.Values{}
	if algoId != "" {
		q.Set("algoId", algoId)
	}
	if instId != "" {
		q.Set("instId", instId)
	}
	return c.Get("/api/v5/tradingBot/grid/positions", q)
}

// GetDcaOrderDetails DCA 单策略详情
func (c *Client) GetDcaOrderDetails(algoId string) ([]json.RawMessage, error) {
	q := url.Values{}
	if algoId != "" {
		q.Set("algoId", algoId)
	}
	return c.Get("/api/v5/tradingBot/dca/orders-algo-details", q)
}
