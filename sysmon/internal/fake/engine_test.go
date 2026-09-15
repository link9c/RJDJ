package fake

import (
	"math"
	"testing"
	"time"

	"sysmon/internal/config"
	"sysmon/internal/feed"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := config.Default()
	cfg.Normalize()
	return NewEngine(cfg)
}

// 盈亏方向必须能穿透到 Proc.Dir。
// 几个监控指标都取了绝对值（负的 CPU 占用会立刻穿帮），
// 方向只能由 Dir 单独承载，界面「趋势」列读的就是它。
func TestDirFollowsSign(t *testing.T) {
	eng := newTestEngine(t)

	rows := []feed.Row{
		{InstID: "BTC-USDT-SWAP", Kind: "position", UplRatio: 0.035, Upl: 412, Margin: 1240},
		{InstID: "ETH-USDT-SWAP", Kind: "position", UplRatio: -0.0127, Upl: -96, Margin: 760},
		{InstID: "SOL-USDT-SWAP", Kind: "position", UplRatio: 0, Upl: 0, Margin: 100},
		{InstID: "XRP-USDT-SWAP", Kind: "position", Err: "取数失败"},
	}

	got := map[string]int{}
	for _, p := range eng.Project(rows, time.Now()) {
		if p.Real != nil {
			got[p.Real.InstID] = p.Dir
		}
	}

	want := map[string]int{
		"BTC-USDT-SWAP": 1,  // 盈利
		"ETH-USDT-SWAP": -1, // 亏损
		"SOL-USDT-SWAP": 0,  // 平
		"XRP-USDT-SWAP": 0,  // 取数失败不给方向
	}
	for inst, w := range want {
		if got[inst] != w {
			t.Errorf("%s 的方向 = %d，期望 %d", inst, got[inst], w)
		}
	}
}

// 装饰进程没有真实数据，趋势列必须留空，否则会出现凭空的涨跌。
func TestDecoysHaveNoDirection(t *testing.T) {
	eng := newTestEngine(t)
	for _, p := range eng.Project(nil, time.Now()) {
		if !p.Decoy {
			continue
		}
		if p.Dir != 0 || p.Real != nil {
			t.Fatalf("装饰进程 %s 不应有方向或真实数据: dir=%d", p.Name, p.Dir)
		}
	}
}

// 收益率取绝对值后映射为 CPU：亏 4% 和赚 4% 的 CPU 占用应当一样。
func TestIndicatorsUseMagnitude(t *testing.T) {
	eng := newTestEngine(t)

	rows := []feed.Row{
		{InstID: "AAA-USDT-SWAP", Kind: "position", UplRatio: 0.04, Upl: 400, Margin: 1000},
		{InstID: "BBB-USDT-SWAP", Kind: "position", UplRatio: -0.04, Upl: -400, Margin: 1000},
	}
	procs := eng.Project(rows, time.Now())

	byInst := map[string]Proc{}
	for _, p := range procs {
		if p.Real != nil {
			byInst[p.Real.InstID] = p
		}
	}
	a, b := byInst["AAA-USDT-SWAP"], byInst["BBB-USDT-SWAP"]
	if math.Abs(a.CPU-b.CPU) > 1.5 {
		t.Errorf("盈亏同幅时 CPU 应接近：%v vs %v", a.CPU, b.CPU)
	}
	if a.Dir == b.Dir {
		t.Errorf("盈亏方向应相反，却都是 %d", a.Dir)
	}
}

// 行情两列是给人读价格的，所以不能加抖动：
// 同一个价格连续投影多次，输出必须完全一致，否则就没法反推出真实价格。
func TestPriceColumnsAreStable(t *testing.T) {
	eng := newTestEngine(t)
	rows := []feed.Row{{
		InstID: "BTC-USDT-SWAP", Kind: "position",
		MarkPx: 63385, AvgPx: 61230.5, UplRatio: 0.035, Margin: 1240,
	}}

	now := time.Now()
	first := eng.Project(rows, now)[0]
	for i := 1; i < 8; i++ {
		p := eng.Project(rows, now.Add(time.Duration(i)*time.Second))[0]
		if p.Commit != first.Commit || p.Faults != first.Faults {
			t.Fatalf("第 %d 次投影价格列发生了变化：提交大小 %v→%v，页面错误 %d→%d",
				i, first.Commit, p.Commit, first.Faults, p.Faults)
		}
	}
}

// 默认系数 0.1：两列数字 × 10 应当还原出价格。
func TestPriceScaleRoundTrip(t *testing.T) {
	eng := newTestEngine(t)
	rows := []feed.Row{{
		InstID: "BTC-USDT-SWAP", Kind: "position",
		MarkPx: 63385, AvgPx: 61230.5, UplRatio: 0.035, Margin: 1240,
	}}

	p := eng.Project(rows, time.Now())[0]

	if got := p.Commit * 10; math.Abs(got-63385) > 1 {
		t.Errorf("提交大小 × 10 = %.0f，期望约 63385", got)
	}
	if got := float64(p.Faults) * 10; math.Abs(got-61230.5) > 10 {
		t.Errorf("页面错误 × 10 = %.0f，期望约 61230", got)
	}
}

// 取不到行情时两列留 0（界面渲染成 "-"），不能编造一个假价格。
func TestPriceColumnsEmptyWithoutQuote(t *testing.T) {
	eng := newTestEngine(t)

	rows := []feed.Row{
		// 网格策略接口不返回均价，只有现价
		{InstID: "BTC-USDT-SWAP", Kind: "grid", MarkPx: 63385, Margin: 3000},
		// 取数失败
		{InstID: "XRP-USDT-SWAP", Kind: "position", Err: "取数失败"},
	}

	byKind := map[string]Proc{}
	for _, p := range eng.Project(rows, time.Now()) {
		if p.Real != nil {
			byKind[p.Real.Kind] = p
		}
	}

	grid := byKind["grid"]
	if grid.Commit <= 0 {
		t.Errorf("有现价时提交大小不应为空，实际 %v", grid.Commit)
	}
	if grid.Faults != 0 {
		t.Errorf("没有均价时页面错误应留 0，实际 %d", grid.Faults)
	}

	broken := byKind["position"]
	if broken.Commit != 0 || broken.Faults != 0 {
		t.Errorf("取数失败的行两列都应为 0，实际 %v / %d", broken.Commit, broken.Faults)
	}
}

// 装饰进程的提交大小/页面错误不能是 0：
// 这两列在真实任务管理器里每个进程都有值，一片 0 会很显眼。
func TestDecoysHavePriceColumns(t *testing.T) {
	eng := newTestEngine(t)

	n := 0
	for _, p := range eng.Project(nil, time.Now()) {
		if !p.Decoy {
			continue
		}
		n++
		if p.Commit <= 0 {
			t.Errorf("装饰进程 %s 的提交大小为 0", p.Name)
		}
		if p.Faults <= 0 {
			t.Errorf("装饰进程 %s 的页面错误为 0", p.Name)
		}
	}
	if n == 0 {
		t.Fatal("默认配置应当混入装饰进程")
	}
}

// price_scale / avg_scale 要能按配置生效。
func TestPriceScaleOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Processes = []config.ProcessCfg{{Source: "auto", PriceScale: 1000, AvgScale: 1000}}
	cfg.Normalize()

	eng := NewEngine(cfg)
	rows := []feed.Row{{InstID: "LAB-USDT-SWAP", Kind: "position", MarkPx: 0.05, AvgPx: 0.05}}

	p := eng.Project(rows, time.Now())[0]
	if math.Abs(p.Commit-50) > 0.5 {
		t.Errorf("price_scale=1000 时 0.05 应约为 50 MB，实际 %v", p.Commit)
	}
	if p.Faults != 50 {
		t.Errorf("avg_scale=1000 时 0.05 应约为 50，实际 %d", p.Faults)
	}
}

// 默认全部 1:1：界面上读到的数字就是真实数值本身，不做任何倍率换算。
// 这条约束是刻意的设计——一旦引入系数，看盘时就得心算还原。
func TestDefaultScalesAreOneToOne(t *testing.T) {
	eng := newTestEngine(t)

	// 收益率 3.5% / 保证金 1240U / 浮盈 412U / 24h 涨跌 1.83% / 杠杆 10 倍
	rows := []feed.Row{{
		InstID: "BTC-USDT-SWAP", Kind: "position", Lever: 10,
		UplRatio: 0.035, Upl: 412, Margin: 1240, Chg24h: 0.0183,
	}}
	p := eng.Project(rows, time.Now())[0]

	if math.Abs(p.CPU-3.5) > 0.5 {
		t.Errorf("CPU 应等于收益率 3.5%%，实际 %.1f%%", p.CPU)
	}
	if math.Abs(p.Mem-1240) > 30 {
		t.Errorf("内存应等于保证金 1240 MB，实际 %.1f MB", p.Mem)
	}
	// 磁盘抖动幅度较大（±25%），按相对误差判断
	if math.Abs(p.Disk-412)/412 > 0.3 {
		t.Errorf("磁盘应约为浮动盈亏 412 MB/s，实际 %.1f MB/s", p.Disk)
	}
	if math.Abs(p.Net-1.83) > 0.6 {
		t.Errorf("网络应约为 24h 涨跌 1.83 Mbps，实际 %.1f Mbps", p.Net)
	}
	if math.Abs(p.GPU-10) > 0.6 {
		t.Errorf("GPU 应等于杠杆 10%%，实际 %.1f%%", p.GPU)
	}
}

// 系数仍然可用：显式配成别的值时应按配置换算。
func TestScaleOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Processes = []config.ProcessCfg{{
		Source: "auto", CpuScale: 2, MemScale: 0.5, DiskScale: 0.5,
		NetScale: 2, GpuScale: 2,
	}}
	cfg.Normalize()

	eng := NewEngine(cfg)
	rows := []feed.Row{{
		InstID: "BTC-USDT-SWAP", Kind: "position", Lever: 10,
		UplRatio: 0.035, Upl: 412, Margin: 1240, Chg24h: 0.0183,
	}}
	p := eng.Project(rows, time.Now())[0]

	if math.Abs(p.CPU-7) > 0.8 {
		t.Errorf("cpu_scale=2 时 3.5%% 应为 7%%，实际 %.1f%%", p.CPU)
	}
	if math.Abs(p.Mem-620) > 20 {
		t.Errorf("mem_scale=0.5 时 1240 应为 620 MB，实际 %.1f MB", p.Mem)
	}
	if math.Abs(p.GPU-20) > 1.2 {
		t.Errorf("gpu_scale=2 时 10 倍杠杆应为 20%%，实际 %.1f%%", p.GPU)
	}
}
