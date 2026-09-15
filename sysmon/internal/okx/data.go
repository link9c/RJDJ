package okx

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// ============= 持仓 /api/v5/account/positions =============

// Position 合约持仓（OKX 原生字段）。
type Position struct {
	InstType    string `json:"instType"`
	MgnMode     string `json:"mgnMode"`
	PosSide     string `json:"posSide"`
	Pos         string `json:"pos"`
	AvailPos    string `json:"availPos"`
	AvgPx       string `json:"avgPx"`
	Upl         string `json:"upl"`
	UplRatio    string `json:"uplRatio"`
	InstID      string `json:"instId"`
	Lever       string `json:"lever"`
	Margin      string `json:"margin"`
	LiqPx       string `json:"liqPx"`
	MarkPx      string `json:"markPx"`
	NotionalUsd string `json:"notionalUsd"`
	MgnRatio    string `json:"mgnRatio"`
	UTime       string `json:"uTime"`
}

// GetPositions 拉取持仓（instType 留空表示全部）。
func (c *Client) GetPositions(instType string) ([]Position, error) {
	q := url.Values{}
	if instType != "" {
		q.Set("instType", instType)
	}
	data, err := c.Get("/api/v5/account/positions", q)
	if err != nil {
		return nil, err
	}
	out := make([]Position, 0, len(data))
	for _, raw := range data {
		var p Position
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// AccountBalance 账户资产总览（只取总权益）。
type AccountBalance struct {
	TotalEq string `json:"totalEq"`
	UTime   string `json:"uTime"`
}

// GetBalance 拉取账户总权益。
func (c *Client) GetBalance() (*AccountBalance, error) {
	data, err := c.Get("/api/v5/account/balance", nil)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &AccountBalance{}, nil
	}
	var b AccountBalance
	if err := json.Unmarshal(data[0], &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// ============= 策略 / 量化机器人（tradingBot） =============
//
// OKX 把自动化策略放在 /api/v5/tradingBot 下，各类的列表后缀并不统一：
//   grid      网格      GET orders-algo-pending | orders-algo-history （以 algoOrdType 区分 grid/contract_grid）
//   dca       马丁(DCA) GET ongoing-list | history-list              （algoOrdType: spot_dca / contract_dca）
//   recurring 定投      GET orders-algo-pending | orders-algo-history
//   signal    信号      GET orders-algo-pending | orders-algo-history
//   algo      条件单    GET /api/v5/trade/orders-algo-pending | orders-algo-history

// BotStrategy 归一化后的策略条目，多来源合并成统一结构。
type BotStrategy struct {
	Kind        string `json:"-"`
	AlgoID      string `json:"algoId"`
	AlgoOrdType string `json:"algoOrdType"`
	OrdType     string `json:"ordType"` // 条件单用：conditional / oco / trigger / move_order_stop
	InstType    string `json:"instType"`
	InstID      string `json:"instId"`
	State       string `json:"state"`
	Direction   string `json:"direction"`
	Lever       string `json:"lever"`
	RunType     string `json:"runType"` // 网格方向 1多 2空 3中性
	AvgPx       string `json:"avgPx"`
	TotalPnl    string `json:"totalPnl"`
	PnlRatio    string `json:"pnlRatio"`
	FloatProfit string `json:"floatProfit"`
	FloatPnl    string `json:"floatPnl"` // 信号策略的浮动盈亏
	Upl         string `json:"upl"`
	InvestAmt   string `json:"investAmt"`
	FrozenBal   string `json:"frozenBal"` // 信号策略的占用保证金
	QuoteSz     string `json:"quoteSz"`
	BaseSz      string `json:"baseSz"`
	NotionalUsd string `json:"notionalUsd"`
	Margin      string `json:"margin"`
	GridNum     string `json:"gridNum"`
	MinPx       string `json:"minPx"`
	MaxPx       string `json:"maxPx"`
}

type botPath struct {
	prefix  string
	pending string
	history string
	param   string   // 区分子类型的查询参数名（algoOrdType 或 ordType）
	values  []string // 需要逐个轮询的取值
}

// botPaths 各类策略的端点。
//
// param/values 描述"怎么区分同一端点下的子类型"，两者是必须的：
// OKX 对 grid / recurring / signal 强制要求 algoOrdType，条件单端点
// `/api/v5/trade/orders-algo-pending` 则要求 ordType。漏传会直接返回
// 50014 `Parameter algoOrdType can not be empty` 或 51000 `Parameter ordType error`。
var botPaths = map[string]botPath{
	"grid": {prefix: "/api/v5/tradingBot/grid", pending: "orders-algo-pending",
		history: "orders-algo-history", param: "algoOrdType",
		values: []string{"grid", "contract_grid"}},
	"dca": {prefix: "/api/v5/tradingBot/dca", pending: "ongoing-list",
		history: "history-list", param: "algoOrdType",
		values: []string{"spot_dca", "contract_dca"}},
	"recurring": {prefix: "/api/v5/tradingBot/recurring", pending: "orders-algo-pending",
		history: "orders-algo-history", param: "algoOrdType",
		values: []string{"recurring"}},
	// 信号策略的 algoOrdType 取值是 contract（合约信号），不是 "signal"——
	// 传错会得到 51000 Parameter algoOrdType error。
	"signal": {prefix: "/api/v5/tradingBot/signal", pending: "orders-algo-pending",
		history: "orders-algo-history", param: "algoOrdType",
		values: []string{"contract"}},
	"algo": {prefix: "/api/v5/trade", pending: "orders-algo-pending",
		history: "orders-algo-history", param: "ordType",
		values: []string{"conditional", "oco", "trigger", "move_order_stop"}},
}

// GetStrategies 拉取指定类别（grid/dca/recurring/signal/algo）的**运行中**策略。
// 同一类别可能对应多个 algoOrdType 端点，按需轮询并去重合并。
func (c *Client) GetStrategies(kind string) ([]BotStrategy, error) {
	bp, ok := botPaths[kind]
	if !ok {
		return nil, nil
	}
	out := []BotStrategy{}
	seen := map[string]bool{}
	var firstErr error

	for _, at := range bp.values {
		q := url.Values{}
		q.Set(bp.param, at)
		q.Set("limit", "50")
		data, err := c.Get(bp.prefix+"/"+bp.pending, q)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, raw := range data {
			var s BotStrategy
			if err := json.Unmarshal(raw, &s); err != nil {
				continue
			}
			s.Kind = kind
			// 接口未必回传子类型，补上本次轮询用的取值（条件单补到 ordType）。
			switch bp.param {
			case "ordType":
				if s.OrdType == "" {
					s.OrdType = at
				}
			default:
				if s.AlgoOrdType == "" {
					s.AlgoOrdType = at
				}
			}
			if s.AlgoID == "" || seen[s.AlgoID] {
				continue
			}
			seen[s.AlgoID] = true
			out = append(out, s)
		}
	}
	// 全部端点都失败才视为错误；部分成功即返回已拿到的数据。
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// GetAllStrategies 聚合全部类别；某一类失败不影响其余类别。
// 返回的 map 键为类别，值可能为空切片。
func (c *Client) GetAllStrategies() (map[string][]BotStrategy, map[string]error) {
	kinds := []string{"grid", "dca", "recurring", "signal", "algo"}
	out := make(map[string][]BotStrategy, len(kinds))
	errs := make(map[string]error)
	for _, k := range kinds {
		v, err := c.GetStrategies(k)
		if err != nil {
			errs[k] = err
			continue
		}
		out[k] = v
	}
	return out, errs
}

// ============= 行情 /api/v5/market/tickers =============

// Ticker 行情快照。
type Ticker struct {
	InstID  string `json:"instId"`
	Last    string `json:"last"`
	Open24h string `json:"open24h"`
	Vol24h  string `json:"vol24h"`
	High24h string `json:"high24h"`
	Low24h  string `json:"low24h"`
}

// GetTickers 一次性拉取某 instType 的全部行情，返回 instId → Ticker。
func (c *Client) GetTickers(instType string) (map[string]Ticker, error) {
	q := url.Values{}
	q.Set("instType", instType)
	data, err := c.PublicGet("/api/v5/market/tickers", q)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Ticker, len(data))
	for _, raw := range data {
		var t Ticker
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		out[t.InstID] = t
	}
	return out, nil
}

// Num 把 OKX 的字符串数值安全转成 float64，空串/非法值返回 0。
func Num(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// FirstNonEmpty 返回第一个非空字符串。
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
