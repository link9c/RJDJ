package okx

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ============= 账户账单流水（历史已实现盈亏数据源） =============
// 接口：/api/v5/account/bills-archive —— 近 3 个月，翻页参数 after/before，limit≤100
// 注意：OKX 已把「近3个月账单」端点由 bills-history 改名为 bills-archive，
//       旧路径 /api/v5/account/bills-history 已废弃并返回 404 Not Found。
//       近 7 天用 /api/v5/account/bills，但 bills-archive 已包含 7 天内数据，故统一用它。
// 账单字段语义：
//   - pnl：已实现盈亏（衍生品平仓/强平/交割时产生；合约资金费通常也计入 pnl 或体现于 balChg）
//   - fee：交易手续费（负数=平台扣费）
//   - balChg：该笔事件对本 ccy 账户余额的变动（含划转等，统计盈亏时应排除划转）
//   - ccy：结算/计价币种（如 USDT）
//   - instType：SPOT / SWAP / FUTURES / OPTION / MARGIN（合约 = SWAP / FUTURES）
//   - type / subType：账单类型（1=买入 2=卖出 …，及衍生品平仓等类别）
//   - ts：事件时间戳（毫秒）

// Bill 账单流水（映射 OKX 返回）
type Bill struct {
	BillID    string `json:"billId"`
	Type      string `json:"type"`
	SubType   string `json:"subType"`
	Ts        string `json:"ts"`
	InstType  string `json:"instType"`
	InstID    string `json:"instId"`
	Ccy       string `json:"ccy"`
	Pnl       string `json:"pnl"`
	Fee       string `json:"fee"`
	BalChg    string `json:"balChg"`
	Bal       string `json:"bal"`
	Notes     string `json:"notes"`
	From      string `json:"from"`
	To        string `json:"to"`
	MgnMode   string `json:"mgnMode"`
}

// num parse float helper
func num(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// BillPage 一页账单 + 是否还有更早数据（本页是否等于 limit，是则可能还有）
func (c *Client) GetBillsPage(instType, after string, limit int) ([]Bill, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	q := url.Values{}
	if instType != "" {
		q.Set("instType", instType)
	}
	if after != "" {
		q.Set("after", after) // 取比该 billId 更早的记录
	}
	q.Set("limit", strconv.Itoa(limit))
	data, err := c.Get("/api/v5/account/bills-archive", q)
	if err != nil {
		return nil, "", err
	}
	bills := make([]Bill, 0, len(data))
	for _, raw := range data {
		var b Bill
		if err := json.Unmarshal(raw, &b); err != nil {
			continue
		}
		bills = append(bills, b)
	}
	// 计算下一页 after：本页最后一条 billId
	nextAfter := ""
	if len(bills) == limit {
		nextAfter = bills[len(bills)-1].BillID
	}
	return bills, nextAfter, nil
}

// BillPageEvent 翻页过程中的一页回调信息（供进度上报 / 提前停 / 增量命中判断）
type BillPageEvent struct {
	InstType   string // 当前类型 SWAP / FUTURES
	PageInType int    // 该类型内第几页（0 基）
	TotalPages int    // 已累计翻页数（跨类型）
	Bills      []Bill // 本页账单
}

// FetchOpts 合约账单翻页控制
type FetchOpts struct {
	Days          int           // 回溯天数
	MaxPagesPerType int         // 每类型最大翻页保护
	PageInterval  time.Duration // 相邻请求的最小间隔（限速，缓解 50011）
}

// FetchContractBillsWithProgress 带限速 + 50011 退避地翻页拉取合约(SWAP+FUTURES)账单。
// 逐页调用 onPage：返回 stop=true 则立即停止（可用于「已命中缓存，更早的不需要」）。
// 每页之间等待 PageInterval；遇到限流(50011)指数退避重试（最多 retryMax 次）。
func (c *Client) FetchContractBillsWithProgress(opts FetchOpts, onPage func(ev BillPageEvent) (stop bool, err error)) error {
	if opts.Days <= 0 {
		opts.Days = 90
	}
	if opts.MaxPagesPerType <= 0 {
		opts.MaxPagesPerType = 60
	}
	if opts.PageInterval <= 0 {
		opts.PageInterval = 300 * time.Millisecond
	}
	cutoff := time.Now().AddDate(0, 0, -opts.Days).UnixMilli()
	const retryMax = 6
	totalPages := 0
	// 返回本页是否有比 cutoff 更早的数据（是则需继续更早翻）
	pageBeyond := func(bills []Bill) bool {
		for _, b := range bills {
			if int64(num(b.Ts)) < cutoff {
				return true
			}
		}
		return false
	}
	for _, instType := range []string{"SWAP", "FUTURES"} {
		after := ""
		for page := 0; page < opts.MaxPagesPerType; page++ {
			var bills []Bill
			var nextAfter string
			var err error
			// 拉取 + 50011 退避重试
			for attempt := 0; attempt <= retryMax; attempt++ {
				bills, nextAfter, err = c.GetBillsPage(instType, after, 100)
				if err == nil {
					break
				}
				if !IsRateLimit(err) {
					return err
				}
				backoff := opts.PageInterval * time.Duration(1<<uint(attempt))
				if backoff < time.Second {
					backoff = time.Second
				}
				time.Sleep(backoff)
			}
			if err != nil {
				return err
			}
			totalPages++
			stop, err2 := onPage(BillPageEvent{
				InstType:   instType,
				PageInType: page,
				TotalPages: totalPages,
				Bills:      bills,
			})
			if err2 != nil {
				return err2
			}
			// 翻过 cutoff 即停（本类型结束 & 整体结束，因为从新到旧）
			if pageBeyond(bills) {
				return nil
			}
			if stop {
				return nil
			}
			if nextAfter == "" {
				break // 该类型到底
			}
			after = nextAfter
			// 限速：下一请求前等待
			time.Sleep(opts.PageInterval)
		}
	}
	return nil
}

// GetContractBills 同步拉取合约账单（默认限速；兼容旧调用）。days 回溯、maxPages 每类型上限。
func (c *Client) GetContractBills(days int, maxPages int) ([]Bill, error) {
	var all []Bill
	err := c.FetchContractBillsWithProgress(FetchOpts{
		Days:            days,
		MaxPagesPerType: maxPages,
		PageInterval:    300 * time.Millisecond,
	}, func(ev BillPageEvent) (bool, error) {
		all = append(all, ev.Bills...)
		return false, nil
	})
	return all, err
}

// ============= 盈亏聚合 =============

// PnlRecord 一条已实现盈亏摘要（按时/币种聚合单元）
type PnlPoint struct {
	Date string `json:"date"` // 2006-01-02（本地日）
	Pnl  float64 `json:"pnl"`  // 已实现盈亏（含资金费口径下的 pnl）
	Fee  float64 `json:"fee"`  // 手续费（负）
	Net  float64 `json:"net"`  // 净 = pnl + fee
}

// CcyPnl 各结算币种盈亏
type CcyPnl struct {
	Ccy string  `json:"ccy"`
	Pnl float64 `json:"pnl"`
	Fee float64 `json:"fee"`
	Net float64 `json:"net"`
	Count int    `json:"count"`
}

// AssetPnl 各交易标的币(如 BTC/ETH)的盈亏。数值均为 quote 币(如 USDT)计价，可直接求和。
type AssetPnl struct {
	Asset string  `json:"asset"`
	Pnl   float64 `json:"pnl"`
	Fee   float64 `json:"fee"`
	Net   float64 `json:"net"`
	Count int     `json:"count"`
}

// PnlSummary 整体聚合结果
type PnlSummary struct {
	Daily   []PnlPoint `json:"daily"`   // 按天（升序）
	ByCcy   []CcyPnl   `json:"byCcy"`   // 按结算币种，net 降序（通常只有 USDT）
	ByAsset []AssetPnl `json:"byAsset"` // 按交易标的币(如 BTC/ETH)，net 降序
	TotalPnl float64   `json:"totalPnl"`
	TotalFee float64   `json:"totalFee"`
	TotalNet float64   `json:"totalNet"`
	Days    int        `json:"days"`
	Records int        `json:"records"` // 参与聚合的账单条数
}

// FilterBillsByCcy 只保留指定结算币种(ccy)的账单，用于「按结算币种看每日/总盈亏」。
// 大小写不敏感（如 "USDT" 或 "usdt"）。空 ccy 返回原切片。
func FilterBillsByCcy(bills []Bill, ccy string) []Bill {
	if ccy == "" {
		return bills
	}
	ccy = strings.ToUpper(ccy)
	out := make([]Bill, 0, len(bills))
	for _, b := range bills {
		if strings.ToUpper(b.Ccy) == ccy {
			out = append(out, b)
		}
	}
	return out
}

// baseAsset 从 instId 提取标的币：如 "BTC-USDT-SWAP" → "BTC"。无法识别返回空。
func baseAsset(instId string) string {
	instId = strings.ToUpper(strings.TrimSpace(instId))
	i := strings.Index(instId, "-")
	if i <= 0 {
		return ""
	}
	return instId[:i]
}

// FilterBillsByAsset 只保留指定标的币(asset，如 BTC/ETH)的合约账单。
// 空 asset 返回原切片。
func FilterBillsByAsset(bills []Bill, asset string) []Bill {
	if asset == "" {
		return bills
	}
	asset = strings.ToUpper(strings.TrimSpace(asset))
	out := make([]Bill, 0, len(bills))
	for _, b := range bills {
		if baseAsset(b.InstID) == asset {
			out = append(out, b)
		}
	}
	return out
}

// AggregatePnl 从合约账单聚合出每日盈亏、各结算币种盈亏与各标的币盈亏。
func AggregatePnl(bills []Bill) PnlSummary {
	dayMap := map[string]*PnlPoint{}
	ccyMap := map[string]*CcyPnl{}
	assetMap := map[string]*AssetPnl{}
	for _, b := range bills {
		p := num(b.Pnl)
		f := num(b.Fee)
		// 仅统计有 pnl 或 fee 的记录（排除纯划转 balChg 引起的 pnl=0 & fee=0）
		if p == 0 && f == 0 {
			continue
		}
		ts := num(b.Ts)
		day := time.UnixMilli(int64(ts)).Format("2006-01-02")

		d, ok := dayMap[day]
		if !ok {
			d = &PnlPoint{Date: day}
			dayMap[day] = d
		}
		d.Pnl += p
		d.Fee += f

		ccy := b.Ccy
		if ccy == "" {
			ccy = "?"
		}
		cc, ok := ccyMap[ccy]
		if !ok {
			cc = &CcyPnl{Ccy: ccy}
			ccyMap[ccy] = cc
		}
		cc.Pnl += p
		cc.Fee += f
		cc.Count++

		// 按交易标的币聚合（无 instId 的记录不参与 byAsset）
		if asset := baseAsset(b.InstID); asset != "" {
			ap, ok := assetMap[asset]
			if !ok {
				ap = &AssetPnl{Asset: asset}
				assetMap[asset] = ap
			}
			ap.Pnl += p
			ap.Fee += f
			ap.Count++
		}
	}
	// 组装 daily 升序
	summary := PnlSummary{Days: 0, Records: len(bills)}
	for _, d := range dayMap {
		d.Net = d.Pnl + d.Fee
		summary.Daily = append(summary.Daily, *d)
	}
	// 按日期升序
	sortPnlPoints(summary.Daily)
	for _, c := range ccyMap {
		c.Net = c.Pnl + c.Fee
		summary.ByCcy = append(summary.ByCcy, *c)
	}
	sortCcyByNetDesc(summary.ByCcy)
	for _, a := range assetMap {
		a.Net = a.Pnl + a.Fee
		summary.ByAsset = append(summary.ByAsset, *a)
	}
	sortAssetByNetDesc(summary.ByAsset)
	for _, d := range summary.Daily {
		summary.TotalPnl += d.Pnl
		summary.TotalFee += d.Fee
	}
	summary.TotalNet = summary.TotalPnl + summary.TotalFee
	if len(summary.Daily) > 0 {
		summary.Days = len(summary.Daily)
	}
	if summary.Daily == nil {
		summary.Daily = []PnlPoint{}
	}
	if summary.ByCcy == nil {
		summary.ByCcy = []CcyPnl{}
	}
	if summary.ByAsset == nil {
		summary.ByAsset = []AssetPnl{}
	}
	return summary
}

func sortPnlPoints(list []PnlPoint) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].Date < list[j-1].Date; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func sortCcyByNetDesc(list []CcyPnl) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].Net > list[j-1].Net; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func sortAssetByNetDesc(list []AssetPnl) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].Net > list[j-1].Net; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
