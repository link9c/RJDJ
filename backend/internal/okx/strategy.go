package okx

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
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
	AlgoID        string `json:"algoId"`
	AlgoOrdType   string `json:"algoOrdType,omitempty"`   // grid / contract_grid / spot_dca / contract_dca / recurring ...
	AlgoClOrdType string `json:"algoClOrdType,omitempty"` // DCA 返回的类型字段
	InstType      string `json:"instType,omitempty"`      // SPOT / SWAP / FUTURES
	InstID        string `json:"instId,omitempty"`
	Ccy           string `json:"ccy,omitempty"`
	Side          string `json:"side,omitempty"`
	State         string `json:"state,omitempty"` // live / running / effective / paused ...
	Type          string `json:"type,omitempty"`  // 策略大类标注：grid/dca/recurring/signal/algo

	// 展示字段（各来源尽量填充）
	Direction       string `json:"direction,omitempty"` // long / short / neutral
	Lever           string `json:"lever,omitempty"`
	MaxPx           string `json:"maxPx,omitempty"`
	MinPx           string `json:"minPx,omitempty"`
	GridNum         string `json:"gridNum,omitempty"`
	RunType         string `json:"runType,omitempty"`
	QuoteSz         string `json:"quoteSz,omitempty"`         // 网格投入
	InvestAmt       string `json:"investAmt,omitempty"`       // DCA/定投投入或保证金
	BaseSz          string `json:"baseSz,omitempty"`          // DCA 首单
	SafetyOrderSz   string `json:"safetyOrderSz,omitempty"`   // DCA 补仓单
	MaxSafetyOrders string `json:"maxSafetyOrders,omitempty"` // DCA 最大补仓次数
	TotalPnl        string `json:"totalPnl,omitempty"`        // 累计已实现+浮动收益
	PnlRatio        string `json:"pnlRatio,omitempty"`
	ClosePnl        string `json:"closePnl,omitempty"`
	Upl             string `json:"upl,omitempty"`
	TriggerPx       string `json:"triggerPx,omitempty"`
	OrdPx           string `json:"ordPx,omitempty"`
	CreatedAt       string `json:"cTime,omitempty"`
	UpdatedAt       string `json:"uTime,omitempty"`
}

// StrategyResp 保留原始 data（AI 可能需要更多字段）
type StrategyResp struct {
	List []BotStrategy     `json:"list"`
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

// DcaPositionDetail 马丁策略「当前周期持仓」(/dca/position-details)。
// 该接口返回机器人自己的持仓（注意：account/positions 不包含机器人仓位），
// avgPx/sz/tpPx 是止盈测算最权威的数据来源。
type DcaPositionDetail struct {
	AlgoID         string `json:"algoId"`
	AlgoOrdType    string `json:"algoOrdType"`
	InstID         string `json:"instId"`
	CurCycleID     string `json:"curCycleId"` // 文档示例误写为 curCycleld，实测字段为 curCycleId
	StartTime      string `json:"startTime"`
	FillManualOrds string `json:"fillManualOrds"` // 周期手动加仓次数
	FillSafetyOrds string `json:"fillSafetyOrds"` // 周期已加仓次数
	FundingFee     string `json:"fundingFee"`     // 当轮累计资金费
	InitPx         string `json:"initPx"`         // 初始订单成交价
	NotionalUsd    string `json:"notionalUsd"`
	AvgPx          string `json:"avgPx"` // 开仓均价
	Upl            string `json:"upl"`   // 未实现收益
	LiqPx          string `json:"liqPx"`
	Sz             string `json:"sz"`     // 合约持仓量（张）
	BaseSz         string `json:"baseSz"` // 现货持币量
	QuoteSz        string `json:"quoteSz"`
	SlPx           string `json:"slPx"`
	TpPx           string `json:"tpPx"` // 止盈价（机器人实际止盈挂单价）
	Fee            string `json:"fee"`  // 当轮累计手续费
}

// DcaCycle 马丁周期 (/dca/cycle-list)
type DcaCycle struct {
	CycleID      string `json:"cycleId"`
	CurrentCycle bool   `json:"currentCycle"`
	CycleStatus  string `json:"cycleStatus"` // running / stopped
	RealizedPnl  string `json:"realizedPnl"`
	StartTime    string `json:"startTime"`
	EndTime      string `json:"endTime"`
	Fee          string `json:"fee"`
	AvgPx        string `json:"avgPx"`
	TpPx         string `json:"tpPx"`
}

// DcaSubOrder 马丁子订单/成交记录 (/dca/orders)
type DcaSubOrder struct {
	CycleID   string `json:"cycleId"`
	OrdID     string `json:"ordId"`
	AvgFillPx string `json:"avgFillPx"`
	Direction string `json:"direction"`
	Side      string `json:"side"` // buy / sell
	OrdType   string `json:"ordType"`
	Px        string `json:"px"`
	Sz        string `json:"sz"`
	FilledSz  string `json:"filledSz"`
	State     string `json:"state"`
	Fee       string `json:"fee"`
	Rebate    string `json:"rebate"`
	RebateCcy string `json:"rebateCcy"`
	Lever     string `json:"lever"`
	InstID    string `json:"instId"`
	CtVal     string `json:"ctVal"`
	FillTime  string `json:"fillTime"`
	CTime     string `json:"cTime"`
	Utime     string `json:"uTime"`
}

// dcaOrdTypes 返回该策略实际使用的 algoOrdType 候选（优先使用详情自带值）。
func dcaOrdTypes(known string) []string {
	switch known {
	case "contract_dca", "spot_dca":
		return []string{known, otherDcaOrdType(known)}
	default:
		return []string{"contract_dca", "spot_dca"}
	}
}

func otherDcaOrdType(t string) string {
	if t == "contract_dca" {
		return "spot_dca"
	}
	return "contract_dca"
}

// GetDcaPositionDetail 拉取马丁策略当前周期持仓；已停止/无持仓周期返回 ErrStrategyDetailNotFound。
func (c *Client) GetDcaPositionDetail(algoId, knownOrdType string) (*DcaPositionDetail, string, error) {
	var lastErr error
	for _, ot := range dcaOrdTypes(knownOrdType) {
		q := url.Values{}
		q.Set("algoId", algoId)
		q.Set("algoOrdType", ot)
		data, err := c.Get("/api/v5/tradingBot/dca/position-details", q)
		if err != nil {
			lastErr = err
			continue
		}
		if len(data) == 0 {
			lastErr = ErrStrategyDetailNotFound
			continue
		}
		var p DcaPositionDetail
		if err := json.Unmarshal(data[0], &p); err != nil {
			lastErr = err
			continue
		}
		if p.AlgoOrdType == "" {
			p.AlgoOrdType = ot
		}
		return &p, ot, nil
	}
	return nil, "", lastErr
}

// GetDcaCycles 拉取马丁周期列表（最新在前）。
func (c *Client) GetDcaCycles(algoId, knownOrdType string, limit int) ([]DcaCycle, string, error) {
	var lastErr error
	for _, ot := range dcaOrdTypes(knownOrdType) {
		q := url.Values{}
		q.Set("algoId", algoId)
		q.Set("algoOrdType", ot)
		if limit > 0 {
			q.Set("limit", strconv.Itoa(limit))
		}
		data, err := c.Get("/api/v5/tradingBot/dca/cycle-list", q)
		if err != nil {
			lastErr = err
			continue
		}
		cycles := make([]DcaCycle, 0, len(data))
		for _, raw := range data {
			var cy DcaCycle
			if err := json.Unmarshal(raw, &cy); err != nil {
				continue
			}
			cycles = append(cycles, cy)
		}
		return cycles, ot, nil
	}
	return nil, "", lastErr
}

// GetDcaOrders 拉取马丁某周期的子订单（成交记录），cycleId 必填。
func (c *Client) GetDcaOrders(algoId, ordType, cycleId string, limit int) ([]DcaSubOrder, error) {
	q := url.Values{}
	q.Set("algoId", algoId)
	q.Set("algoOrdType", ordType)
	q.Set("cycleId", cycleId)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	data, err := c.Get("/api/v5/tradingBot/dca/orders", q)
	if err != nil {
		return nil, err
	}
	orders := make([]DcaSubOrder, 0, len(data))
	for _, raw := range data {
		var o DcaSubOrder
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// ============= 策略详情（止盈测算用） =============
// 各 bot 详情接口返回结构均为 data[0] 下一个嵌套数组（键名不统一），数组首项即该策略详情：
//   grid      → /api/v5/tradingBot/grid/orders-algo-details       data[0].gridDetail[0]
//   dca       → /api/v5/tradingBot/dca/orders-algo-details        data[0].dcaDetails[0]
//   recurring → /api/v5/tradingBot/recurring/orders-algo-details  data[0].recurringDetails[0]
//   signal    → /api/v5/tradingBot/signal/orders-algo-details     data[0].signalDetails[0]

// DetailCalc 止盈测算结果（quote 币计价，通常为 USDT）
type DetailCalc struct {
	PosCoin       float64 `json:"posCoin"`       // 持仓量（币；合约已按 张数×ctVal 换算）
	PosContracts  float64 `json:"posContracts"`  // 合约持仓量（张），现货为 0
	CostPx        float64 `json:"costPx"`        // 平均持仓成本价
	TpPx          float64 `json:"tpPx"`          // 止盈触发价
	CurrentPx     float64 `json:"currentPx"`     // 最新价
	CostValue     float64 `json:"costValue"`     // 持仓成本 = 均价 × 持仓量
	CurrentValue  float64 `json:"currentValue"`  // 当前市值 = 最新价 × 持仓量
	CurrentFloat  float64 `json:"currentFloat"`  // 按最新价估算的浮动盈亏
	TpReturnValue float64 `json:"tpReturnValue"` // 止盈卖出收入 = 止盈价 × 持仓量
	TpProfit      float64 `json:"tpProfit"`      // 止盈盈利（扣成本，做空方向自动反向）
	TpRatio       float64 `json:"tpRatio"`       // 止盈收益率 = 盈利 / 成本
	Direction     string  `json:"direction"`     // long / short / neutral
	IsContract    bool    `json:"isContract"`
	CtVal         float64 `json:"ctVal"` // 合约面值（币/张）
	Ccy           string  `json:"ccy"`   // 计价币种（quote）
}

// StrategyDetail 策略详情归一化结构
type StrategyDetail struct {
	BotStrategy
	// 持仓与成本
	AvgPx        string `json:"avgPx,omitempty"`
	PosContracts string `json:"posContracts,omitempty"` // 合约持仓（张）
	CtVal        string `json:"ctVal,omitempty"`        // 合约面值（币/张）
	// 止盈止损
	TpTriggerPx     string `json:"tpTriggerPx,omitempty"`
	SlTriggerPx     string `json:"slTriggerPx,omitempty"`
	TpTriggerPxType string `json:"tpTriggerPxType,omitempty"`
	SlTriggerPxType string `json:"slTriggerPxType,omitempty"`
	// DCA(马丁) 特有
	TagNum          string `json:"tagNum,omitempty"`
	InitOrdAmt      string `json:"initOrdAmt,omitempty"`      // 首单金额
	SafetyOrdAmt    string `json:"safetyOrdAmt,omitempty"`    // 补仓单金额
	PxSteps         string `json:"pxSteps,omitempty"`         // 补仓价格步长（比例）
	VolMult         string `json:"volMult,omitempty"`         // 补仓量倍数
	InvestmentAmt   string `json:"investmentAmt,omitempty"`   // 累计投入
	TotalFundingFee string `json:"totalFundingFee,omitempty"` // 累计资金费
	ArbitragePnl    string `json:"arbitragePnl,omitempty"`    // 套利收益
	AllowReinvest   bool   `json:"allowReinvest,omitempty"`
	// DCA 当前周期持仓（position-details）与成交记录
	Position    *DcaPositionDetail `json:"position,omitempty"`
	Cycles      []DcaCycle         `json:"cycles,omitempty"`
	Orders      []DcaSubOrder      `json:"orders,omitempty"`
	PosSource   string             `json:"posSource,omitempty"` // position=当前持仓 cycle=最近周期重建 none=无
	CycleID     string             `json:"cycleId,omitempty"`
	NotionalUsd string             `json:"notionalUsd,omitempty"`
	// 网格特有
	ArbitrageNum   string `json:"arbitrageNum,omitempty"`
	GridProfit     string `json:"gridProfit,omitempty"`
	FloatProfit    string `json:"floatProfit,omitempty"`
	AnnualizedRate string `json:"annualizedRate,omitempty"`
	RealizedPnl    string `json:"realizedPnl,omitempty"`
	StopResult     string `json:"stopResult,omitempty"`
	// 定投特有
	TotalInvestedAmt string `json:"totalInvestedAmt,omitempty"`
	// 实时价与测算
	CurrentPx string      `json:"currentPx,omitempty"`
	Calc      *DetailCalc `json:"calc,omitempty"`
	// 原始详情（字段排查/前端兜底展示用）
	Raw json.RawMessage `json:"raw,omitempty"`
}

// extractNestedFirst 从 wrapper 中按候选键名取嵌套数组的第一项原始 JSON。
// OKX 不同接口键名单复数不一（gridDetail/dcaDetails/signalDetails），做兜底枚举。
func extractNestedFirst(wrapper json.RawMessage, keys ...string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(wrapper, &m); err != nil {
		return nil
	}
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
			return arr[0]
		}
	}
	return nil
}

// GetStrategyDetail 拉取单策略详情并归一化。找不到/已结束返回 ErrStrategyDetailNotFound 供上层走列表兜底。
//
// 实测（2026-09，真实账户）：
//   - DCA(马丁) 没有 orders-algo-details 端点（全路径 404）；
//     用 GET /tradingBot/dca/ongoing-list?algoId=&algoOrdType= 即可返回单策略完整详情，
//     已结束的改查 history-list。止盈价字段为 tpPriceRange，最大补仓字段为 maxSafetyOrds。
//   - grid/signal 的 orders-algo-details 端点必填 algoOrdType，否则 50014。
var ErrStrategyDetailNotFound = errors.New("策略详情为空（可能已结束，详情接口仅保留运行中策略）")

func (c *Client) GetStrategyDetail(kind, algoId string) (*StrategyDetail, error) {
	if algoId == "" {
		return nil, ErrStrategyDetailNotFound
	}
	var wrappers []json.RawMessage
	var err error
	switch kind {
	case "dca":
		wrappers, err = c.fetchByOrdTypes(
			[]string{"/api/v5/tradingBot/dca/ongoing-list", "/api/v5/tradingBot/dca/history-list"},
			algoId,
			"contract_dca", "spot_dca",
		)
	case "grid":
		wrappers, err = c.fetchByOrdTypes(
			[]string{"/api/v5/tradingBot/grid/orders-algo-details"},
			algoId,
			"contract_grid", "grid",
		)
	case "signal":
		wrappers, err = c.fetchByOrdTypes(
			[]string{"/api/v5/tradingBot/signal/orders-algo-details"},
			algoId,
			"contract", "spot",
		)
	case "recurring":
		wrappers, err = c.fetchByOrdTypes(
			[]string{"/api/v5/tradingBot/recurring/orders-algo-details", "/api/v5/tradingBot/recurring/orders-algo-pending"},
			algoId,
			"recurring",
		)
	default:
		return nil, ErrStrategyDetailNotFound
	}
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	for _, w := range wrappers {
		// DCA/grid/recurring/signal 列表项可能直接是对象，也可能包在嵌套数组里，两种都兼容
		var one json.RawMessage
		var obj map[string]any
		if e1 := json.Unmarshal(w, &obj); e1 == nil {
			if _, ok := obj["algoId"]; ok {
				one = w
			}
		}
		if one == nil {
			one = extractNestedFirst(w, "gridDetail", "gridDetails", "dcaDetails", "dcaDetail", "recurringDetails", "signalDetails", "signalDetail")
		}
		if one != nil {
			raw = one
			break
		}
	}
	if raw == nil {
		return nil, ErrStrategyDetailNotFound
	}

	// 中间结构兼容 OKX 字段命名差异（tpPriceRange / maxSafetyOrds）
	var alias struct {
		StrategyDetail
		TpPriceRange  string `json:"tpPriceRange"`
		MaxSafetyOrds string `json:"maxSafetyOrds"`
	}
	if err := json.Unmarshal(raw, &alias); err != nil {
		return nil, err
	}
	d := alias.StrategyDetail
	if d.TpTriggerPx == "" {
		d.TpTriggerPx = alias.TpPriceRange
	}
	if d.MaxSafetyOrders == "" {
		d.MaxSafetyOrders = alias.MaxSafetyOrds
	}
	d.Type = kind
	if d.AlgoID == "" {
		d.AlgoID = algoId
	}
	if d.AlgoOrdType == "" {
		d.AlgoOrdType = map[string]string{"grid": "grid", "dca": "spot_dca", "recurring": "recurring", "signal": "signal"}[kind]
	}
	// OKX DCA ongoing-list 某些时序下不返回 instType，按 algoOrdType 推断
	if d.InstType == "" {
		if strings.Contains(d.AlgoOrdType, "contract") {
			d.InstType = "SWAP"
		} else if d.AlgoOrdType == "spot_dca" || d.AlgoOrdType == "grid" || d.AlgoOrdType == "recurring" {
			d.InstType = "SPOT"
		}
	}
	normalizeDirection(&d)
	d.Raw = raw
	return &d, nil
}

// fetchByOrdTypes 依次以候选 algoOrdType 请求候选 URI，返回首个非空 data 数组。
// instId 相关的类型枚举顺序已按账户中最常见的合约优先排列。
func (c *Client) fetchByOrdTypes(uris []string, algoId string, ordTypes ...string) ([]json.RawMessage, error) {
	var lastErr error
	for _, ot := range ordTypes {
		for _, uri := range uris {
			q := url.Values{}
			q.Set("algoId", algoId)
			q.Set("algoOrdType", ot)
			data, err := c.Get(uri, q)
			if err != nil {
				lastErr = err
				continue
			}
			if len(data) > 0 {
				return data, nil
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrStrategyDetailNotFound
}

// normalizeDirection 统一方向：grid 用 runType(1=long 2=short 3=neutral)，dca 用 direction 字符串。
func normalizeDirection(d *StrategyDetail) {
	dir := strings.ToLower(d.Direction)
	if dir == "" {
		switch d.RunType {
		case "1", "long":
			dir = "long"
		case "2", "short":
			dir = "short"
		case "3", "neutral", "netural":
			dir = "neutral"
		}
	}
	if dir == "netural" { // OKX 历史拼写错误兼容
		dir = "neutral"
	}
	d.Direction = dir
}

// BuildDetailCalc 依据详情 + 最新价 + 合约面值做止盈测算。
// 合约持仓量（张）需乘 ctVal 换算成币；现货 baseSz 即币数。
func BuildDetailCalc(d *StrategyDetail, currentPx, ctVal string) {
	calc := &DetailCalc{
		Direction:  d.Direction,
		IsContract: d.InstType == "SWAP" || d.InstType == "FUTURES",
		Ccy:        quoteCcy(d.InstID),
		CurrentPx:  num(currentPx),
		CostPx:     num(d.AvgPx),
		TpPx:       num(d.TpTriggerPx),
	}
	if calc.IsContract {
		ctv := num(ctVal)
		if ctv <= 0 {
			ctv = 1 // 面值拿不到时退化为 1（按张近似），前端可手改持仓量
		}
		calc.CtVal = ctv
		contracts := num(firstNonEmpty(d.PosContracts, d.BaseSz))
		calc.PosContracts = contracts
		calc.PosCoin = contracts * ctv
	} else {
		calc.PosCoin = num(firstNonEmpty(d.BaseSz, d.QuoteSz))
	}
	calc.CostValue = calc.CostPx * calc.PosCoin
	calc.CurrentValue = calc.CurrentPx * calc.PosCoin
	sign := 1.0
	if calc.Direction == "short" {
		sign = -1
	}
	calc.CurrentFloat = sign * (calc.CurrentPx - calc.CostPx) * calc.PosCoin
	if calc.TpPx > 0 && calc.PosCoin > 0 {
		calc.TpReturnValue = calc.TpPx * calc.PosCoin
		calc.TpProfit = sign * (calc.TpPx - calc.CostPx) * calc.PosCoin
		if calc.CostValue > 0 {
			calc.TpRatio = calc.TpProfit / calc.CostValue
		}
	}
	d.CurrentPx = currentPx
	d.CtVal = ctVal
	d.Calc = calc
}

// quoteCcy 从 instId 提取计价币：BTC-USDT-SWAP → USDT；BTC-USD-240628 → USD；BTC-USDT → USDT。
func quoteCcy(instId string) string {
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
