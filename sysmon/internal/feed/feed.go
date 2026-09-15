// Package feed 负责拉取 OKX 数据，并按配置里的伪装映射表组装成统一的「进程行」。
package feed

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"time"

	"sysmon/internal/config"
	"sysmon/internal/okx"
)

// Row 一条统一的展示条目：既可能是真实持仓，也可能是一台策略机器人。
// 这里的字段全部是**真实业务数值**，伪装成监控指标是下一步 fake 包的职责。
type Row struct {
	Name     string  // 伪装后的显示名（由配置或自动名池决定）
	InstID   string  // 真实标的
	Kind     string  // position / grid / dca / recurring / signal / algo
	AlgoID   string  //
	Side     string  // long / short / ""
	State    string  // live / running / paused ...
	Lever    float64 // 杠杆倍数
	UplRatio float64 // 收益率（小数，0.035 = +3.5%）
	Upl      float64 // 未实现/累计盈亏（计价币）
	Margin   float64 // 占用保证金或投入本金
	Notional float64 // 名义价值（USDT）
	AvgPx    float64 // 持仓均价
	MarkPx   float64 // 最新价
	Pos      float64 // 持仓量（张 / 币）
	Chg24h   float64 // 24h 涨跌幅（小数）
	Ccy      string  // 计价币
	Note     string  // 附加说明（策略类型等）
	Err      string  // 该条目取数失败时的原因

	// Cfg 是这条数据命中的配置项，映射引擎据此取缩放系数与用户名。
	// 由 applyConfigToRows 填充，未命中时为 nil。
	Cfg *config.ProcessCfg
}

// Snapshot 一次完整的数据快照。
type Snapshot struct {
	Time     time.Time
	Rows     []Row
	TotalEq  float64
	Warnings []string
	Err      string
}

// Fetcher 数据拉取器。
type Fetcher struct {
	cfg  *config.Config
	cli  *okx.Client
	mock bool
}

// New 创建拉取器。
func New(cfg *config.Config) *Fetcher {
	f := &Fetcher{cfg: cfg, mock: cfg.OKX.Mock}
	if !f.mock {
		f.cli = okx.NewClient(okx.Credentials{
			APIKey:     cfg.OKX.APIKey,
			APISecret:  cfg.OKX.APISecret,
			Passphrase: cfg.OKX.Passphrase,
			BaseURL:    cfg.OKX.BaseURL,
			Simulated:  cfg.OKX.Simulated,
		})
	}
	return f
}

// MissingCredential 判断凭证是否缺失（缺了就只可能走 mock）。
func (f *Fetcher) MissingCredential() bool {
	return strings.TrimSpace(f.cfg.OKX.APIKey) == "" ||
		strings.TrimSpace(f.cfg.OKX.APISecret) == "" ||
		strings.TrimSpace(f.cfg.OKX.Passphrase) == ""
}

// Fetch 拉取一次快照。任何单点失败都不阻断整体，失败原因落到 Warnings 或 Row.Err。
func (f *Fetcher) Fetch(ctx context.Context) Snapshot {
	if f.mock || f.MissingCredential() {
		snap := mockSnapshot()
		if !f.mock {
			snap.Warnings = append(snap.Warnings,
				"未配置 OKX API Key，当前显示为演示数据（请在 config.json 的 okx 段填写）")
		}
		// 即便凭证缺失也走一遍配置映射，让默认/自定义进程名生效
		snap.Rows = f.applyConfigToRows(snap.Rows)
		return snap
	}

	snap := Snapshot{Time: time.Now()}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		pos     []okx.Position
		strats  = map[string][]okx.BotStrategy{}
		tickers = map[string]okx.Ticker{}
		totalEq float64
	)

	collect := func(err error, msg string) {
		if err == nil {
			return
		}
		mu.Lock()
		snap.Warnings = append(snap.Warnings, msg+": "+err.Error())
		mu.Unlock()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := f.cli.GetPositions("")
		if err != nil {
			collect(err, "持仓获取失败")
			return
		}
		mu.Lock()
		pos = v
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		v, errs := f.cli.GetAllStrategies()
		mu.Lock()
		strats = v
		for k, e := range errs {
			snap.Warnings = append(snap.Warnings, fmt.Sprintf("%s 策略获取失败: %v", k, e))
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// 永续/交割与现货各拉一次并合并：只拉 SWAP 的话，现货网格、
		// 现货定投这类以 "BTC-USDT" 为标的的记录会取不到价格。
		merged := make(map[string]okx.Ticker, 1024)
		failed := make([]string, 0, 2)
		for _, instType := range []string{"SWAP", "SPOT"} {
			v, err := f.cli.GetTickers(instType)
			if err != nil {
				failed = append(failed, instType)
				continue
			}
			for k, t := range v {
				merged[k] = t
			}
		}
		mu.Lock()
		tickers = merged
		if len(failed) > 0 {
			snap.Warnings = append(snap.Warnings,
				"行情获取失败: "+strings.Join(failed, "/")+" 类型拉取失败")
		}
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := f.cli.GetBalance()
		if err != nil {
			collect(err, "账户权益获取失败")
			return
		}
		mu.Lock()
		totalEq = okx.Num(v.TotalEq)
		mu.Unlock()
	}()

	wg.Wait()

	if ctx.Err() != nil {
		snap.Err = "请求已取消"
		return snap
	}

	snap.TotalEq = totalEq
	snap.Time = time.Now()

	rows := f.buildRows(pos, strats, tickers)
	snap.Rows = f.applyConfigToRows(rows)
	return snap
}

// buildRows 把原始数据铺成候选行（不带伪装名）。
func (f *Fetcher) buildRows(pos []okx.Position, strats map[string][]okx.BotStrategy, tickers map[string]okx.Ticker) []Row {
	rows := make([]Row, 0, len(pos)+8)
	for _, p := range pos {
		chg := chg24h(tickers, p.InstID)
		rows = append(rows, Row{
			InstID:   p.InstID,
			Kind:     "position",
			Side:     sideOf(p.PosSide, p.Pos),
			State:    "live",
			Lever:    okx.Num(p.Lever),
			UplRatio: okx.Num(p.UplRatio),
			Upl:      okx.Num(p.Upl),
			Margin:   okx.Num(p.Margin),
			Notional: okx.Num(okx.FirstNonEmpty(p.NotionalUsd, "0")),
			AvgPx:    okx.Num(p.AvgPx),
			MarkPx:   okx.Num(okx.FirstNonEmpty(p.MarkPx, tickerLast(tickers, p.InstID))),
			Pos:      math.Abs(okx.Num(p.Pos)),
			Chg24h:   chg,
			Ccy:      quoteOf(p.InstID),
			Note:     "合约持仓",
		})
	}
	for _, kind := range []string{"grid", "dca", "recurring", "signal", "algo"} {
		for _, s := range strats[kind] {
			lever := okx.Num(s.Lever)
			uplRatio := okx.Num(okx.FirstNonEmpty(s.PnlRatio, "0"))
			upl := okx.Num(okx.FirstNonEmpty(s.Upl, s.TotalPnl, s.FloatPnl, s.FloatProfit, "0"))
			margin := okx.Num(okx.FirstNonEmpty(s.InvestAmt, s.FrozenBal, s.Margin, s.QuoteSz, "0"))
			rows = append(rows, Row{
				InstID:   s.InstID,
				Kind:     kind,
				AlgoID:   s.AlgoID,
				Side:     sideOf(s.Direction, s.RunType),
				State:    strings.ToLower(s.State),
				Lever:    lever,
				UplRatio: uplRatio,
				Upl:      upl,
				Margin:   margin,
				Notional: okx.Num(okx.FirstNonEmpty(s.NotionalUsd, s.QuoteSz, "0")),
				AvgPx:    okx.Num(s.AvgPx),
				MarkPx:   okx.Num(tickerLast(tickers, s.InstID)),
				Chg24h:   chg24h(tickers, s.InstID),
				Ccy:      quoteOf(s.InstID),
				// 条件单没有 algoOrdType，子类型落在 ordType 上
				Note: labelOf(kind, okx.FirstNonEmpty(s.AlgoOrdType, s.OrdType)),
			})
		}
	}
	return rows
}

// ApplyConfigToRows 按配置的进程映射表筛选/排序/命名。
// 导出以便界面预览等场景复用同一套映射结果。
func ApplyConfigToRows(cfg *config.Config, all []Row) []Row {
	return (&Fetcher{cfg: cfg}).applyConfigToRows(all)
}

// applyConfigToRows 按配置的进程映射表筛选/排序/命名。
func (f *Fetcher) applyConfigToRows(all []Row) []Row {
	out := make([]Row, 0, len(all))
	used := map[string]bool{}

	key := func(r Row) string { return r.Kind + "|" + r.InstID + "|" + r.AlgoID + "|" + r.Side }

	for _, pc := range f.cfg.Processes {
		matched := matchOne(pc, all)
		if len(matched) == 0 {
			placeholder := Row{
				InstID: pc.InstID,
				Kind:   pc.Source,
				State:  "paused",
				Note:   "无匹配数据",
				Err:    "配置项未匹配到数据",
			}
			placeholder.Name = nameFor(pc, 0, 0)
			out = append(out, placeholder)
			continue
		}
		for i, r := range matched {
			k := key(r)
			if used[k] {
				continue
			}
			used[k] = true
			r.Name = nameFor(pc, i, len(matched))
			out = append(out, r)
		}
	}
	return out
}

// matchOne 找出一条配置项对应的原始行集合。
func matchOne(pc config.ProcessCfg, all []Row) []Row {
	src := pc.Source
	inst := strings.ToUpper(strings.TrimSpace(pc.InstID))
	out := []Row{}
	for _, r := range all {
		if inst != "" && strings.ToUpper(r.InstID) != inst {
			continue
		}
		switch src {
		case "auto":
			out = append(out, r)
		case "position":
			if r.Kind == "position" {
				out = append(out, r)
			}
		default:
			if r.Kind == src {
				out = append(out, r)
			}
		}
	}
	return out
}

// nameFor 决定该行的伪装名：
//   - 配置写了 name → 用它；一行展开成多条时追加序号
//   - 配置没写 name 且指定了 instId → 用自动名池中该标的稳定对应的名字
//   - 都没写（自动铺满模式）→ 按序号从名池取，保证每次启动名字一致
func nameFor(pc config.ProcessCfg, idx, total int) string {
	base := strings.TrimSpace(pc.Name)
	if base != "" {
		if total > 1 {
			return fmt.Sprintf("%s (%d)", base, idx+1)
		}
		return base
	}
	return autoName(pc.InstID, idx)
}

// namePool 用于自动命名的进程名池。刻意选用「应用/服务级」名字，
// 与 fake 包里装饰用的「系统级」名字（System / Registry / lsass.exe 等）错开，
// 避免列表里出现重复进程名。
var namePool = []string{
	"chrome.exe", "msedge.exe", "Code.exe", "node.exe", "python.exe",
	"powershell.exe", "WeChat.exe", "DingTalk.exe", "Teams.exe", "OneDrive.exe",
	"Everything.exe", "Snipaste.exe", "Bandizip.exe", "Notepad.exe", "conhost.exe",
	"dllhost.exe", "taskhostw.exe", "sihost.exe", "RuntimeBroker.exe", "SearchApp.exe",
	"StartMenuExperienceHost.exe", "TextInputHost.exe", "ctfmon.exe",
	"SecurityHealthService.exe", "explorer.exe",
}

// autoName 由标的字符串稳定映射到名池中的一项。
func autoName(instID string, idx int) string {
	seed := instID
	if seed == "" {
		return namePool[idx%len(namePool)]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return namePool[int(h.Sum32())%len(namePool)]
}

// ---- 小工具 ----

func sideOf(posSide, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(posSide))
	switch s {
	case "long", "short", "net":
		return s
	}
	switch fallback {
	case "1", "long":
		return "long"
	case "2", "short":
		return "short"
	case "3", "neutral", "netural":
		return "neutral"
	}
	// 多头持仓的 pos 通常为正，做空为负
	v := okx.Num(fallback)
	if v < 0 {
		return "short"
	}
	if v > 0 {
		return "long"
	}
	return ""
}

func quoteOf(instID string) string {
	parts := strings.Split(instID, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "USDT"
}

func tickerLast(t map[string]okx.Ticker, instID string) string {
	if v, ok := t[instID]; ok {
		return v.Last
	}
	return ""
}

func chg24h(t map[string]okx.Ticker, instID string) float64 {
	v, ok := t[instID]
	if !ok {
		return 0
	}
	open := okx.Num(v.Open24h)
	if open <= 0 {
		return 0
	}
	return (okx.Num(v.Last) - open) / open
}

// labelOf 生成「详细信息」页里那个中文的子类型说明。
// 参数是策略子类型：多数类别是 algoOrdType，条件单是 ordType。
func labelOf(kind, sub string) string {
	switch kind {
	case "grid":
		if sub == "contract_grid" {
			return "合约网格"
		}
		return "现货网格"
	case "dca":
		if sub == "contract_dca" {
			return "合约马丁"
		}
		return "现货马丁"
	case "recurring":
		return "定投"
	case "signal":
		return "信号"
	case "algo":
		switch sub {
		case "oco":
			return "双向止盈止损"
		case "trigger":
			return "单向触发"
		case "move_order_stop":
			return "移动止盈止损"
		}
		return "条件单"
	}
	return kind
}
