// Package fake 是伪装映射引擎：把 OKX 的真实数值换算成一套看起来完全正常的
// 系统监控指标（CPU% / 内存 / 磁盘 / 网络 / GPU），并叠加平滑抖动，
// 使界面上呈现的数字像真实机器负载一样持续微动。
//
// 默认换算关系（可在 config.json 每条映射里用 *_scale 调整）：
//
//	CPU%     ← |收益率%|            × cpu_scale   （默认 1，收益率 3.5% → CPU 3.5%）
//	内存     ← 保证金(USDT)         × mem_scale   （默认 1，保证金 1240 → 1240 MB）
//	磁盘     ← |浮动盈亏(USDT)|     × disk_scale  （默认 1，浮盈 412 → 412 MB/s）
//	网络     ← |24h涨跌幅%|         × net_scale   （默认 1，涨跌 1.83% → 1.83 Mbps）
//	GPU%     ← 杠杆倍数             × gpu_scale   （默认 1，10 倍杠杆 → 10%）
//	提交大小 ← 最新价 × price_scale               （默认 0.1，该列数字 × 10 就是现价）
//	页面错误 ← 持仓均价 × avg_scale               （默认 0.1，同上，用于和现价对照）
//	电源     ← 由 CPU% 分级推导
//
// 前五项都是 1:1，也就是监控指标的数字就是真实数值本身（单位在表头），
// 不需要做任何心算。系数仅作为"想整体放大/缩小某列"时的调节手段保留。
//
// 「提交大小」与「页面错误」是任务管理器详细信息页里的真实列名，取值区间本来就很宽
// （几 MB 到几十 GB / 几百到几百万），所以行情数字放进去不需要额外伪装。
// 这两列刻意不加抖动：抖动会让数字偏离真实价格，而这两列存在的意义就是能读出来。
//
// 上面这些指标都取了绝对值（负的 CPU 占用不合常理），所以盈亏方向由
// Proc.Dir 单独承载，界面用「趋势」列的红 + 与绿 - 表示。
package fake

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"

	"sysmon/internal/config"
	"sysmon/internal/feed"
)

// Proc 伪装后的「进程」条目。
type Proc struct {
	Name     string
	PID      int
	User     string
	Status   string
	CPU      float64 // %
	Mem      float64 // MB
	Disk     float64 // MB/s
	Net      float64 // Mbps
	GPU      float64 // %
	Commit   float64 // MB，伪装成「提交大小」；真实来源是最新价
	Faults   int     // 伪装成「页面错误」；真实来源是持仓均价
	Power    string
	Dir      int     // 方向：+1 盈利/上涨，-1 亏损/下跌，0 中性（装饰进程或无数据）
	CPUTrend float64 // 与上次刷新相比的变化量，用于显示升降箭头
	Hist     []float64
	Real     *feed.Row // nil 表示这是纯装饰进程，没有真实数据
	Decoy    bool
	Pinned   bool
}

// Engine 有状态映射引擎，跨刷新保存历史序列与上一次的读数。
//
// Project 会被高频调用（界面每次重绘都要重新算抖动），但历史采样与趋势对比
// 只在到达数据刷新间隔时才落一次盘，避免序列被重绘频率污染。
type Engine struct {
	cfg         *config.Config
	prev        map[string]float64
	hist        map[string][]float64
	recordEvery time.Duration
	lastRecord  time.Time
}

// NewEngine 创建引擎。
func NewEngine(cfg *config.Config) *Engine {
	every := time.Duration(cfg.UI.RefreshMS) * time.Millisecond
	if every < 500*time.Millisecond {
		every = 3 * time.Second
	}
	return &Engine{
		cfg:         cfg,
		prev:        map[string]float64{},
		hist:        map[string][]float64{},
		recordEvery: every,
	}
}

// Project 把一批真实行投影成伪装进程列表。now 用于计算抖动相位，
// 可以比数据刷新更频繁地调用（界面每次重绘都会调用），从而让数字持续微动。
func (e *Engine) Project(rows []feed.Row, now time.Time) []Proc {
	record := e.lastRecord.IsZero() || now.Sub(e.lastRecord) >= e.recordEvery
	if record {
		e.lastRecord = now
	}

	out := make([]Proc, 0, len(rows)+e.cfg.UI.DecoyCount)
	taken := make(map[string]bool, len(rows)+e.cfg.UI.DecoyCount)
	for i := range rows {
		r := rows[i]
		p := e.projectRow(&r, now, record)
		taken[p.Name] = true
		out = append(out, p)
	}

	// 混入纯装饰的系统进程，让列表看起来更像一台真实运行的 Windows。
	// 名字与已有条目冲突时跳过，避免出现两个同名进程。
	added := 0
	for i := 0; added < e.cfg.UI.DecoyCount && i < len(decoyNames)*2; i++ {
		p := e.decoy(i, now, record)
		if taken[p.Name] {
			continue
		}
		taken[p.Name] = true
		added++
		out = append(out, p)
	}
	return out
}

// projectRow 单条真实数据 → 伪装进程。
func (e *Engine) projectRow(r *feed.Row, now time.Time, record bool) Proc {
	pc := r.Cfg
	if pc == nil {
		pc = e.findCfg(r)
	}
	// 映射系数全部 1:1：收益率 6.8% → CPU 6.8%；保证金 1240U → 内存 1240MB；
	// 浮盈 412U → 磁盘 412MB/s；24h 涨跌 1.8% → 网络 1.8Mbps；10 倍杠杆 → GPU 10%。
	// 界面上读到的数字就是原始数据，不需要换算。
	cpuScale, memScale, diskScale, netScale, gpuScale := 1.0, 1.0, 1.0, 1.0, 1.0
	priceScale, avgScale := 1.0, 1.0
	user := ""
	if pc != nil {
		// 用 orDef 而不是直接赋值：配置未经 normalize 时系数会是 0，
		// 那会让整列指标静默归零。
		cpuScale = orDef(pc.CpuScale, cpuScale)
		memScale = orDef(pc.MemScale, memScale)
		diskScale = orDef(pc.DiskScale, diskScale)
		netScale = orDef(pc.NetScale, netScale)
		gpuScale = orDef(pc.GpuScale, gpuScale)
		priceScale = orDef(pc.PriceScale, priceScale)
		avgScale = orDef(pc.AvgScale, avgScale)
		user = pc.User
	}
	if user == "" {
		user = e.cfg.UI.Hostname
	}

	key := r.Kind + "|" + r.InstID + "|" + r.AlgoID

	// ---- 基准值换算 ----
	// 上限只是防止极端值离谱，不是缩放：CPU / GPU 是百分比，天然封顶 100；
	// 磁盘与网络封顶 999.9（真机上 SSD 能跑出几百 MB/s、千兆网卡也能到几百 Mbps），
	// 卡太紧会让两个不同盈亏的持仓显示成同一个数。
	baseCPU := clamp(math.Abs(r.UplRatio)*100*cpuScale, 0.0, 99.9)
	baseMem := clamp(math.Abs(r.Margin)*memScale, 0.8, 65536)
	baseDisk := clamp(math.Abs(r.Upl)*diskScale, 0.0, 999.9)
	baseNet := clamp(math.Abs(r.Chg24h)*100*netScale, 0.0, 999.9)
	baseGPU := clamp(r.Lever*gpuScale, 0.0, 99.0)

	// 行情伪装：最新价 → 提交大小(MB)，持仓均价 → 页面错误。
	// 取不到行情（现货标的没拉到 ticker、或策略本身不返回均价）时留 0，
	// 界面渲染成 "-"，不编造一个可能被误读的假价格。
	commit, faults := 0.0, 0
	if r.MarkPx > 0 {
		commit = clamp(r.MarkPx*priceScale, 0.1, 8*1024*1024)
	}
	if r.AvgPx > 0 {
		// 下限定在 1：有没有数据的区别靠 0 来体现，有数据就不该显示成空。
		// 价格极小的标的（比如 XRP）请把 avg_scale 调大，否则会被这个下限吃掉。
		faults = int(math.Max(1, math.Round(r.AvgPx*avgScale)))
	}

	// 无数据的占位行直接给一排静默值
	if r.Err != "" {
		baseCPU, baseDisk, baseNet, baseGPU = 0, 0, 0, 0
		baseMem = 2.4
		commit, faults = 0, 0
	}

	// ---- 叠加抖动 ----
	cpu := e.jitter(key+":cpu", baseCPU, 0.06, now)
	mem := e.jitter(key+":mem", baseMem, 0.015, now)
	disk := e.jitter(key+":disk", baseDisk, 0.25, now)
	net := e.jitter(key+":net", baseNet, 0.30, now)
	gpu := e.jitter(key+":gpu", baseGPU, 0.04, now)

	// 内存不会大起大落，抖动后不允许超过基准太多
	if mem > baseMem*1.02 {
		mem = baseMem * 1.02
	}

	status := statusOf(r)

	p := Proc{
		Name:   r.Name,
		PID:    stablePID(key),
		User:   user,
		Status: status,
		CPU:    round1(clamp(cpu, 0, 99.9)),
		Mem:    round2(clamp(mem, 0.1, 65536)),
		Disk:   round1(clamp(disk, 0, 999.9)),
		Net:    round1(clamp(net, 0, 999.9)),
		GPU:    round1(clamp(gpu, 0, 99.9)),
		Commit: round2(commit),
		Faults: faults,
		Dir:    dirOf(r),
		Real:   r,
	}
	p.Power = powerOf(p.CPU)
	p.Pinned = pc != nil && pc.Pinned

	// 趋势对比读旧值；历史采样只在采样点落盘，避免被高频重绘污染
	if prev, ok := e.prev[key]; ok {
		p.CPUTrend = round1(p.CPU - prev)
	}
	if record {
		e.prev[key] = p.CPU
		p.Hist = e.pushHist(key, p.CPU)
	} else {
		p.Hist = e.histCopy(key)
	}

	return p
}

// findCfg 找到该行对应的配置项（用于取缩放系数与用户名）。
// 按 instId + source 精确匹配，匹配不到则退回第一条 auto 配置。
func (e *Engine) findCfg(r *feed.Row) *config.ProcessCfg {
	var bySource, fallback *config.ProcessCfg
	for i := range e.cfg.Processes {
		pc := &e.cfg.Processes[i]
		if pc.Source == "auto" {
			if fallback == nil {
				fallback = pc
			}
			continue
		}
		// 指定了标的：需要标的与类型同时吻合
		if pc.InstID != "" {
			if equalFold(pc.InstID, r.InstID) && pc.Source == r.Kind {
				return pc
			}
			continue
		}
		// 没指定标的：按类型宽松匹配
		if pc.Source == r.Kind && bySource == nil {
			bySource = pc
		}
	}
	if bySource != nil {
		return bySource
	}
	return fallback
}

// ---- 抖动 ----

// jitter 在基准值上叠加一个平滑的时间噪声。
// 用多段正弦叠加而非随机数，保证数字是连续漂移的，而不是每秒乱跳——
// 这一点对"看起来像真实负载"很关键。
func (e *Engine) jitter(key string, base, amp float64, now time.Time) float64 {
	if base <= 0 {
		return 0
	}
	seed := hash32(key)
	phase := float64(seed%4096) / 4096.0 * 2 * math.Pi
	x := float64(now.UnixMilli()) / 1000.0

	v := math.Sin(x*0.37+phase)*0.5 +
		math.Sin(x*0.13+phase*1.7)*0.3 +
		math.Sin(x*0.89+phase*2.3)*0.2

	d := base * amp
	if d < 0.12 {
		d = 0.12 // 小数值也要有可见的波动
	}
	return base + v*d
}

func (e *Engine) pushHist(key string, cpu float64) []float64 {
	const maxLen = 90
	h := append(e.hist[key], cpu)
	if len(h) > maxLen {
		h = h[len(h)-maxLen:]
	}
	e.hist[key] = h
	out := make([]float64, len(h))
	copy(out, h)
	return out
}

// histCopy 返回历史序列副本，不做写入（用于非采样点的重绘）。
func (e *Engine) histCopy(key string) []float64 {
	h := e.hist[key]
	out := make([]float64, len(h))
	copy(out, h)
	return out
}

// ---- 装饰进程 ----

var decoyNames = []string{
	"System", "Registry", "smss.exe", "csrss.exe", "wininit.exe",
	"services.exe", "lsass.exe", "fontdrvhost.exe", "dwm.exe", "WmiPrvSE.exe",
	"svchost.exe", "winlogon.exe", "MsMpEng.exe", "audiodg.exe", "spoolsv.exe",
	"searchprotocolhost.exe",
}

var decoyUsers = []string{
	"SYSTEM", "SYSTEM", "SYSTEM", "SYSTEM", "SYSTEM",
	"SYSTEM", "SYSTEM", "UMFD-0", "Window Manager", "NETWORK SERVICE",
	"NETWORK SERVICE", "SYSTEM", "SYSTEM", "LOCAL SERVICE", "SYSTEM",
	"SYSTEM",
}

// decoyPIDs 与 decoyNames 一一对应，取真实 Windows 上这些系统进程常见的 PID。
var decoyPIDs = []int{
	4, 88, 108, 428, 612, 748, 892, 1024, 1360, 1704,
	2116, 2380, 2612, 3076, 3508, 4120,
}

// decoy 生成一个低负载的系统进程，纯装饰。
func (e *Engine) decoy(i int, now time.Time, record bool) Proc {
	name := decoyNames[i%len(decoyNames)]
	user := decoyUsers[i%len(decoyUsers)]
	key := "decoy:" + name

	baseCPU := 0.1 + float64(i%3)*0.4
	baseMem := 1.2 + float64(i)*6.4

	cpu := e.jitter(key+":cpu", baseCPU, 0.8, now)
	mem := e.jitter(key+":mem", baseMem, 0.02, now)

	p := Proc{
		Name:   name,
		PID:    decoyPIDs[i%len(decoyPIDs)],
		User:   user,
		Status: "运行中",
		CPU:    round1(clamp(cpu, 0, 12)),
		Mem:    round2(clamp(mem, 0.1, 65536)),
		// 提交大小与页面错误是每台机器上「每个进程都有」的两列，
		// 装饰进程如果留空会立刻显出不对劲，所以给一组稳定的合理值。
		Commit: round2(1.4 + float64((i*37)%220)/4.0),
		Faults: 180 + (i*7919)%42000,
		Disk:   0,
		Net:    0,
		GPU:    0,
		Decoy:  true,
	}
	p.Power = powerOf(p.CPU)
	if record {
		e.prev[key] = p.CPU
		p.Hist = e.pushHist(key, p.CPU)
	} else {
		p.Hist = e.histCopy(key)
	}
	return p
}

// ---- 工具 ----

// dirOf 判断这条记录当前是盈还是亏。
//
// CPU / 内存 / 磁盘 / 网络这几个指标都取了绝对值——负的 CPU 占用、负的内存
// 都会立刻穿帮，所以盈亏方向只能由 Proc.Dir 单独承载，界面用「趋势」列表示：
// 红 + 是盈利，绿 - 是亏损。
//
// 先看收益率，收益率为 0（比如刚开仓）时退回浮动盈亏；取数失败的行不显示方向。
func dirOf(r *feed.Row) int {
	if r.Err != "" {
		return 0
	}
	for _, v := range [2]float64{r.UplRatio, r.Upl} {
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
	}
	return 0
}

func statusOf(r *feed.Row) string {
	if r.Err != "" {
		return "已挂起"
	}
	if r.State == "" {
		return "运行中"
	}
	switch r.State {
	case "live", "running", "effective", "started":
		return "运行中"
	case "paused", "stopped", "stopping":
		return "已暂停"
	}
	return "运行中"
}

func powerOf(cpu float64) string {
	switch {
	case cpu < 0.5:
		return "非常低"
	case cpu < 3:
		return "低"
	case cpu < 10:
		return "中"
	case cpu < 25:
		return "高"
	default:
		return "非常高"
	}
}

func stablePID(key string) int {
	return 1000 + int(hash32(key)%55000)
}

func hash32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// orDef 取正数，非正时退回默认值。
func orDef(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	return math.Min(math.Max(v, lo), hi)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Describe 返回该进程当前映射关系的文字说明（详细信息页用）。
func Describe(p Proc) string {
	if p.Real == nil {
		return "系统组件"
	}
	r := p.Real
	return fmt.Sprintf("%s · %s", r.InstID, r.Note)
}
