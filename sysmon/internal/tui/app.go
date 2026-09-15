package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"sysmon/internal/config"
	"sysmon/internal/fake"
	"sysmon/internal/feed"
)

const (
	tabProcesses = 0
	tabPerf      = 1
	tabDetail    = 2

	redrawEvery  = 250 * time.Millisecond
	fetchTimeout = 30 * time.Second
	histLen      = 120
)

// Key 解析后的一个按键事件。
type Key struct {
	R    rune
	Code string // up / down / left / right / tab / esc
}

// App TUI 应用。
type App struct {
	cfg     *config.Config
	term    *Terminal
	theme   Theme
	fetcher *feed.Fetcher
	engine  *fake.Engine

	mu   sync.Mutex
	snap feed.Snapshot

	tab     int
	sel     int
	sortCol int
	sortAsc bool

	status      string
	statusUntil time.Time

	cpuHist    []float64
	memHist    []float64
	netHist    []float64
	lastHistAt time.Time

	startAt time.Time
	quit    bool
}

// NewApp 构建应用实例。
func NewApp(cfg *config.Config, term *Terminal) *App {
	th := darkTheme
	if cfg.UI.Theme == "light" {
		th = lightTheme
	}
	a := &App{
		cfg:     cfg,
		term:    term,
		theme:   th,
		fetcher: feed.New(cfg),
		engine:  fake.NewEngine(cfg),
		sortCol: 4, // 默认按 CPU（收益率）降序，与任务管理器一致
		sortAsc: false,
		startAt: time.Now(),
	}
	switch cfg.UI.DefaultTab {
	case "performance":
		a.tab = tabPerf
	case "detail":
		if cfg.UI.ShowDetail {
			a.tab = tabDetail
		}
	}
	return a
}

// Run 进入主事件循环，直到用户退出。
func (a *App) Run() error {
	if err := a.term.EnterRaw(); err != nil {
		return err
	}
	defer func() {
		a.term.Restore()
		_ = a.term.WriteString("\x1b[2J\x1b[H")
	}()

	_ = a.term.WriteString("\x1b[2J\x1b[H")

	keys := make(chan Key, 64)
	go readKeys(a.term, keys)

	dataCh := make(chan feed.Snapshot, 2)
	trigger := make(chan struct{}, 1)
	go a.fetchLoop(dataCh, trigger)

	select {
	case trigger <- struct{}{}:
	default:
	}

	ticker := time.NewTicker(redrawEvery)
	defer ticker.Stop()

	for !a.quit {
		select {
		case k := <-keys:
			a.handleKey(k, trigger)
		case s := <-dataCh:
			a.mu.Lock()
			a.snap = s
			a.mu.Unlock()
		case <-ticker.C:
			a.draw()
		}
	}
	return nil
}

// ---- 数据 ----

func (a *App) fetchLoop(out chan<- feed.Snapshot, trigger <-chan struct{}) {
	interval := time.Duration(a.cfg.UI.RefreshMS) * time.Millisecond
	t := time.NewTicker(interval)
	defer t.Stop()

	var last time.Time
	doFetch := func() {
		if time.Since(last) < 700*time.Millisecond {
			return
		}
		last = time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		snap := a.fetcher.Fetch(ctx)
		select {
		case out <- snap:
		default: // 上一次结果还没被消费，丢掉这次的
		}
	}

	for {
		select {
		case <-trigger:
			doFetch()
		case <-t.C:
			doFetch()
		}
	}
}

func (a *App) handleKey(k Key, trigger chan<- struct{}) {
	switch k.Code {
	case "up":
		if a.sel > 0 {
			a.sel--
		}
		return
	case "down":
		a.sel++
		return
	case "left":
		a.tab = (a.tab + a.tabCount() - 1) % a.tabCount()
		a.sel = 0
		return
	case "right", "tab":
		a.tab = (a.tab + 1) % a.tabCount()
		a.sel = 0
		return
	case "esc":
		return
	}

	switch k.R {
	case 'q', 'Q', 3: // 3 = Ctrl+C
		a.quit = true
	case '1':
		a.tab = tabProcesses
		a.sel = 0
	case '2':
		a.tab = tabPerf
		a.sel = 0
	case '3':
		if a.cfg.UI.ShowDetail {
			a.tab = tabDetail
			a.sel = 0
		}
	case 'r', 'R':
		select {
		case trigger <- struct{}{}:
		default:
		}
		a.setStatus("正在刷新…")
	case 't', 'T':
		if a.theme.Name == "dark" {
			a.theme = lightTheme
		} else {
			a.theme = darkTheme
		}
	case 'c', 'C':
		a.toggleSort(4) // CPU = 收益率
	case 'm', 'M':
		a.toggleSort(5) // 内存 = 保证金
	case 'n', 'N':
		a.toggleSort(7) // 网络 = 24h 涨跌幅
	case 'd', 'D':
		a.toggleSort(1) // 趋势 = 盈亏方向，一键把盈利的排在一起
	case 'p', 'P':
		a.toggleSort(2) // PID
	case 's', 'S':
		a.toggleSort(9) // 提交大小 = 最新价
	case 'e', 'E':
		a.toggleSort(10) // 页面错误 = 持仓均价
	}
}

func (a *App) toggleSort(col int) {
	if a.sortCol == col {
		a.sortAsc = !a.sortAsc
	} else {
		a.sortCol = col
		a.sortAsc = col == 0 // 名称默认升序，数值默认降序
	}
}

func (a *App) tabCount() int {
	if a.cfg.UI.ShowDetail {
		return 3
	}
	return 2
}

func (a *App) setStatus(s string) {
	a.status = s
	a.statusUntil = time.Now().Add(2 * time.Second)
}

// ---- 渲染 ----

func (a *App) draw() {
	w, h := a.term.Size()

	a.mu.Lock()
	snap := a.snap
	a.mu.Unlock()

	c := a.BuildFrame(w, h, snap, time.Now())
	_ = a.term.WriteString(c.Render())
}

// BuildFrame 构造一帧画面但不输出，供主循环与快照预览共用。
func (a *App) BuildFrame(w, h int, snap feed.Snapshot, now time.Time) *Canvas {
	procs := a.engine.Project(snap.Rows, now)
	a.sortProcs(procs)

	if a.sel >= len(procs) {
		a.sel = len(procs) - 1
	}
	if a.sel < 0 {
		a.sel = 0
	}

	agg := aggregate(procs, a.cfg.UI.CoreCount)
	a.sampleHistory(agg, now)

	th := a.theme
	c := NewCanvas(w, h)
	c.Clear(th.Text, th.Bg)

	a.drawTitle(c, w)
	if h < 8 {
		return c
	}
	a.drawTabs(c, 1, w)
	c.HLine(0, 2, w, '─', th.Border, th.Bg)

	top := 3
	bottom := h - 2
	if bottom <= top {
		bottom = top + 1
	}

	switch a.tab {
	case tabPerf:
		a.drawPerformance(c, top, bottom, w, procs, agg)
	case tabDetail:
		a.drawDetail(c, top, bottom, w, procs)
	default:
		a.drawProcesses(c, top, bottom, w, procs)
	}

	a.drawStatus(c, h-1, w, snap, agg)
	return c
}

// DumpFrame 渲染指定页签的一帧纯文本快照。
// 用于在非交互环境（CI、重定向输出）下预览界面外观。
func DumpFrame(cfg *config.Config, w, h, tab int) string {
	a := NewApp(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snap := a.fetcher.Fetch(ctx) // 未配置凭证时自动退回演示数据，但仍会套用配置映射
	a.mu.Lock()
	a.snap = snap
	a.mu.Unlock()
	a.tab = tab

	// 连续投影若干次，把历史曲线填满，效果与程序跑了一段时间后一致
	now := time.Now()
	step := 3 * time.Second
	for i := 0; i < 45; i++ {
		a.BuildFrame(w, h, snap, now.Add(time.Duration(i)*step))
	}
	return a.BuildFrame(w, h, snap, now.Add(46*step)).PlainText()
}

func (a *App) drawTitle(c *Canvas, w int) {
	th := a.theme
	c.Fill(0, 0, w, 1, th.HeaderFg, th.HeaderBg)
	c.TextBold(2, 0, a.cfg.UI.Title, th.HeaderFg, th.HeaderBg, true)
	c.TextRight(w-2, 0, "─   □   ✕", th.TextDim, th.HeaderBg)
}

func (a *App) drawTabs(c *Canvas, y, w int) {
	th := a.theme
	names := []string{"进程", "性能"}
	if a.cfg.UI.ShowDetail {
		names = append(names, "详细信息")
	}
	x := 2
	for i, name := range names {
		label := "  " + name + "  "
		fg, bg, bold := th.TextDim, th.Bg, false
		if i == a.tab {
			fg, bg, bold = th.Accent, th.Bg, true
		}
		c.TextBold(x, y, label, fg, bg, bold)
		x += DisplayWidth(label)
		if i == a.tab {
			c.HLine(x-DisplayWidth(label)+2, y+1, DisplayWidth(label)-4, '─', th.Accent, th.Bg)
		}
	}
}

func (a *App) drawStatus(c *Canvas, y, w int, snap feed.Snapshot, agg aggStats) {
	th := a.theme
	if y-1 > 0 {
		c.HLine(0, y-1, w, '─', th.Border, th.Bg)
	}
	c.Fill(0, y, w, 1, th.Text, th.Bg)

	left := fmt.Sprintf(" CPU %s    内存 %s    磁盘 %s    网络 %s    进程 %d",
		fmtPct(agg.cpu), fmtMem(agg.mem), fmtRate(agg.disk), fmtNet(agg.net), agg.count)
	if !snap.Time.IsZero() {
		left += "    更新 " + snap.Time.Format("15:04:05")
	}
	c.Text(1, y, Truncate(left, w-2), th.TextDim, th.Bg)

	right := "↑↓ 选择  ←→ 切换  R 刷新  T 主题  Q 退出"
	if a.status != "" && time.Now().Before(a.statusUntil) {
		right = a.status
	}
	if snap.Err != "" {
		right = snap.Err
	}
	rw := DisplayWidth(right)
	if rw < w-DisplayWidth(left)-4 {
		c.Text(w-2-rw, y, right, th.TextDim, th.Bg)
	}
}

// ---- 进程页 ----

type tableCol struct {
	title string
	width int
	right bool
	key   int
}

func (a *App) drawProcesses(c *Canvas, top, bottom, w int, procs []fake.Proc) {
	th := a.theme

	showDisk := w >= 88
	showNet := w >= 76
	showGPU := w >= 100
	// 行情两列（提交大小/页面错误）放在最后：终端窄的时候优先保住系统指标本身。
	// 想在宽终端下也关掉，把 ui.show_price 设成 false 即可。
	showPrice := a.cfg.UI.ShowPrice && w >= 112

	fixed := 4 + 7 + 8 + 9 + 11 // 趋势 PID 状态 CPU 内存
	ncol := 5                   // 固定列数量（不含名称列）
	if showDisk {
		fixed += 10
		ncol++
	}
	if showNet {
		fixed += 10
		ncol++
	}
	if showGPU {
		fixed += 6
		ncol++
	}
	if showPrice {
		fixed += 10 + 10
		ncol += 2
	}
	// 左留 1 列边距；名称列与固定列各占 1 列尾部间隔（共 ncol+1 个）
	nameW := w - fixed - ncol - 2
	if nameW < 14 {
		nameW = 14
	}

	cols := []tableCol{
		{"名称", nameW, false, 0},
		{"趋势", 4, false, 1},
		{"PID", 7, true, 2},
		{"状态", 8, false, 3},
		{"CPU", 9, true, 4},
		{"内存", 11, true, 5},
	}
	if showDisk {
		cols = append(cols, tableCol{"磁盘", 10, true, 6})
	}
	if showNet {
		cols = append(cols, tableCol{"网络", 10, true, 7})
	}
	if showGPU {
		cols = append(cols, tableCol{"GPU", 6, true, 8})
	}
	if showPrice {
		cols = append(cols, tableCol{"提交大小", 10, true, 9})
		cols = append(cols, tableCol{"页面错误", 10, true, 10})
	}

	// 表头
	c.Fill(0, top, w, 1, th.HeaderFg, th.HeaderBg)
	x := 1
	for _, col := range cols {
		s := col.title
		if a.sortCol == col.key {
			if a.sortAsc {
				s += "▲"
			} else {
				s += "▼"
			}
		}
		c.Text(x, top, FitCell(s, col.width), th.HeaderFg, th.HeaderBg)
		x += col.width + 1
	}

	listTop := top + 1
	listH := bottom - listTop
	if listH <= 0 {
		return
	}

	off := 0
	if len(procs) > listH {
		off = a.sel - listH/2
		if off > len(procs)-listH {
			off = len(procs) - listH
		}
		if off < 0 {
			off = 0
		}
	}

	for i := off; i < len(procs) && i < off+listH; i++ {
		p := procs[i]
		y := listTop + (i - off)
		selected := i == a.sel
		bg := th.Bg
		fg := th.Text
		if selected {
			bg = th.SelBg
			fg = th.SelFg
		}
		c.Fill(0, y, w, 1, fg, bg)

		nameFg := fg
		if p.Decoy && !selected {
			nameFg = th.TextDim
		}
		cpuFg := th.loadColor(p.CPU)
		if selected {
			cpuFg = fg
		}
		// 趋势列承担盈亏方向：红 + 是赚，绿 - 是亏。
		dirFg := fg
		switch {
		case p.Dir > 0:
			dirFg = th.Rise
		case p.Dir < 0:
			dirFg = th.Fall
		}
		if selected {
			dirFg = fg
		}

		cells := []struct {
			text  string
			width int
			right bool
			fgc   Color
		}{
			{p.Name, nameW, false, nameFg},
			{dirMark(p.Dir), 4, false, dirFg},
			{fmt.Sprintf("%d", p.PID), 7, true, fg},
			{p.Status, 8, false, fg},
			{fmtPct(p.CPU), 9, true, cpuFg},
			{fmtMem(p.Mem), 11, true, fg},
		}
		if showDisk {
			cells = append(cells, struct {
				text  string
				width int
				right bool
				fgc   Color
			}{fmtRate(p.Disk), 10, true, fg})
		}
		if showNet {
			cells = append(cells, struct {
				text  string
				width int
				right bool
				fgc   Color
			}{fmtNet(p.Net), 10, true, fg})
		}
		if showGPU {
			cells = append(cells, struct {
				text  string
				width int
				right bool
				fgc   Color
			}{fmtPct0(p.GPU), 6, true, fg})
		}
		if showPrice {
			cells = append(cells, struct {
				text  string
				width int
				right bool
				fgc   Color
			}{FmtCommit(p.Commit), 10, true, fg})
			cells = append(cells, struct {
				text  string
				width int
				right bool
				fgc   Color
			}{FmtFaults(p.Faults), 10, true, fg})
		}

		cx := 1
		for _, cell := range cells {
			s := cell.text
			if cell.right {
				s = PadLeft(Truncate(s, cell.width), cell.width)
			} else {
				s = FitCell(s, cell.width)
			}
			c.Text(cx, y, s, cell.fgc, bg)
			cx += cell.width + 1
		}
	}

	if len(procs) == 0 {
		c.Text(2, listTop+1, "正在加载数据…", th.TextDim, th.Bg)
	}
}

// ---- 性能页 ----

func (a *App) drawPerformance(c *Canvas, top, bottom, w int, procs []fake.Proc, agg aggStats) {
	th := a.theme
	y := top

	// --- CPU ---
	c.TextBold(2, y, "CPU", th.Text, th.Bg, true)
	c.TextRight(w-3, y, fmtPct(agg.cpu), th.Accent, th.Bg)
	y++
	if y >= bottom {
		return
	}
	c.Text(2, y, Truncate(a.cfg.UI.CPUName, w-24), th.TextDim, th.Bg)
	c.TextRight(w-3, y, fmt.Sprintf("%.2f GHz", 1.20+agg.cpu/100*2.40), th.TextDim, th.Bg)
	y++

	chartH := (bottom - y) / 4
	if chartH > 5 {
		chartH = 5
	}
	if chartH < 2 {
		chartH = 2
	}
	if y+chartH+1 < bottom {
		a.drawChart(c, 2, y, w-5, chartH, a.cpuHist, th.BarFill)
	}
	y += chartH + 2
	if y >= bottom {
		return
	}

	// --- 内存 ---
	memPct := 0.0
	if a.cfg.UI.MemTotalGB > 0 {
		memPct = clampF(agg.mem/(a.cfg.UI.MemTotalGB*1024)*100, 0, 100)
	}
	c.TextBold(2, y, "内存", th.Text, th.Bg, true)
	c.TextRight(w-3, y, fmtPct(memPct), th.Accent, th.Bg)
	y++
	if y >= bottom {
		return
	}
	c.Text(2, y, fmt.Sprintf("%s / %.1f GB", fmtMem(agg.mem), a.cfg.UI.MemTotalGB), th.TextDim, th.Bg)
	y++

	if y+chartH+1 < bottom {
		a.drawChart(c, 2, y, w-5, chartH, a.memHist, th.BarFill)
	}
	y += chartH + 2
	if y >= bottom {
		return
	}

	// --- 各进程占用 ---
	c.TextBold(2, y, "占用最高的进程", th.TextDim, th.Bg, false)
	y++
	if y >= bottom {
		return
	}

	barW := w - 42
	if barW < 10 {
		barW = 10
	}
	for i := 0; i < len(procs) && y < bottom; i++ {
		p := procs[i]
		if i >= 8 {
			break
		}
		c.Text(2, y, FitCell(p.Name, 22), th.Text, th.Bg)
		a.drawBar(c, 25, y, barW, p.CPU/100, th)
		c.Text(25+barW+2, y, PadLeft(fmtPct(p.CPU), 8), th.loadColor(p.CPU), th.Bg)
		y++
	}
}

// drawChart 用块字符画一条面积曲线。
func (a *App) drawChart(c *Canvas, x, y, w, h int, hist []float64, fill Color) {
	th := a.theme
	if w < 4 || h < 1 {
		return
	}
	c.Fill(x, y, w, h, th.TextDim, th.Bg)

	// 参考线：1/4、1/2、3/4 处点出淡痕，像真实图表
	for _, frac := range []float64{0.25, 0.5, 0.75} {
		ly := y + h - 1 - int(frac*float64(h))
		if ly < y || ly >= y+h {
			continue
		}
		for i := 2; i < w; i += 6 {
			c.Set(x+i, ly, '·', th.Border, th.Bg)
		}
	}

	if len(hist) == 0 {
		c.Text(x+2, y+h/2, "等待数据…", th.TextDim, th.Bg)
		c.Box(x-1, y-1, w+2, h+2, th.Border, th.Bg)
		return
	}

	start := 0
	if len(hist) > w {
		start = len(hist) - w
	}
	data := hist[start:]
	off := w - len(data)

	for i, v := range data {
		cx := x + off + i
		filled := clampF(v/100*float64(h), 0, float64(h))
		for row := 0; row < h; row++ {
			cy := y + h - 1 - row
			rem := filled - float64(row)
			switch {
			case rem >= 1:
				c.Set(cx, cy, '█', fill, th.Bg)
			case rem > 0:
				c.Set(cx, cy, blockRune(rem), fill, th.Bg)
			}
		}
	}
	c.Box(x-1, y-1, w+2, h+2, th.Border, th.Bg)
}

// drawBar 画一条横向进度条。
func (a *App) drawBar(c *Canvas, x, y, w int, frac float64, th Theme) {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(w))
	for i := 0; i < w; i++ {
		if i < filled {
			c.Set(x+i, y, '█', th.BarFill, th.Bg)
		} else {
			c.Set(x+i, y, '░', th.BarBg, th.Bg)
		}
	}
}

// ---- 详细信息页 ----

func (a *App) drawDetail(c *Canvas, top, bottom, w int, procs []fake.Proc) {
	th := a.theme

	// 窄终端下收缩标的/类型列，保证名称列有位置
	instW, typeW := 24, 16
	if w < 132 {
		instW, typeW = 18, 12
	}
	if w < 112 {
		instW, typeW = 16, 10
	}
	fixed := instW + typeW + 10 + 7 + 11 + 12 + 11 // 标的 类型 方向 杠杆 收益率 浮动盈亏 保证金
	nameW := w - fixed - (7 + 2)
	if nameW < 14 {
		nameW = 14
	}

	cols := []tableCol{
		{"名称", nameW, false, 0},
		{"标的", instW, false, -1},
		{"类型", typeW, false, -1},
		{"方向", 10, false, -1},
		{"杠杆", 7, true, -1},
		{"收益率", 11, true, -1},
		{"浮动盈亏", 12, true, -1},
		{"保证金", 11, true, -1},
	}

	c.Fill(0, top, w, 1, th.HeaderFg, th.HeaderBg)
	x := 1
	for _, col := range cols {
		c.Text(x, top, FitCell(col.title, col.width), th.HeaderFg, th.HeaderBg)
		x += col.width + 1
	}

	listTop := top + 1
	listH := bottom - listTop
	if listH <= 0 {
		return
	}

	// 只列真实数据行，装饰进程在详细信息页没有意义
	real := make([]fake.Proc, 0, len(procs))
	for _, p := range procs {
		if p.Real != nil {
			real = append(real, p)
		}
	}

	if len(real) == 0 {
		c.Text(2, listTop+1, "暂无数据（请先在 config.json 配置 OKX API Key，或开启 okx.mock 查看演示）", th.TextDim, th.Bg)
		return
	}

	off := 0
	if len(real) > listH {
		off = a.sel - listH/2
		if off > len(real)-listH {
			off = len(real) - listH
		}
		if off < 0 {
			off = 0
		}
	}

	for i := off; i < len(real) && i < off+listH; i++ {
		p := real[i]
		r := p.Real
		y := listTop + (i - off)
		selected := i == a.sel
		bg := th.Bg
		fg := th.Text
		if selected {
			bg = th.SelBg
			fg = th.SelFg
		}
		c.Fill(0, y, w, 1, fg, bg)

		uplFg := th.Rise
		if r.Upl < 0 {
			uplFg = th.Fall
		}
		if selected {
			uplFg = fg
		}

		vals := []string{
			p.Name,
			r.InstID,
			r.Note,
			sideLabel(r.Side),
			leverLabel(r.Lever),
			fmtPct(r.UplRatio * 100),
			fmtSigned(r.Upl),
			fmt.Sprintf("%.1f", r.Margin),
		}
		cx := 1
		for ci, col := range cols {
			s := vals[ci]
			if col.right {
				s = PadLeft(Truncate(s, col.width), col.width)
			} else {
				s = FitCell(s, col.width)
			}
			cf := fg
			if col.key == -1 && (ci == 5 || ci == 6) {
				cf = uplFg
			}
			c.Text(cx, y, s, cf, bg)
			cx += col.width + 1
		}
	}
}

// ---- 辅助 ----

type aggStats struct {
	cpu   float64
	mem   float64
	disk  float64
	net   float64
	count int
}

func aggregate(procs []fake.Proc, cores int) aggStats {
	var s aggStats
	for _, p := range procs {
		s.cpu += p.CPU
		s.mem += p.Mem
		s.disk += p.Disk
		s.net += p.Net
	}
	s.count = len(procs)
	if cores > 0 {
		s.cpu = s.cpu / float64(cores)
	}
	if s.cpu > 100 {
		s.cpu = 100
	}
	return s
}

func (a *App) sampleHistory(agg aggStats, now time.Time) {
	interval := time.Duration(a.cfg.UI.RefreshMS) * time.Millisecond
	if interval < time.Second {
		interval = time.Second
	}
	if !a.lastHistAt.IsZero() && now.Sub(a.lastHistAt) < interval {
		return
	}
	a.lastHistAt = now

	memPct := 0.0
	if a.cfg.UI.MemTotalGB > 0 {
		memPct = agg.mem / (a.cfg.UI.MemTotalGB * 1024) * 100
	}
	a.cpuHist = appendCap(a.cpuHist, agg.cpu)
	a.memHist = appendCap(a.memHist, clampF(memPct, 0, 100))
	a.netHist = appendCap(a.netHist, agg.net)
}

func appendCap(s []float64, v float64) []float64 {
	s = append(s, v)
	if len(s) > histLen {
		s = s[len(s)-histLen:]
	}
	return s
}

func (a *App) sortProcs(procs []fake.Proc) {
	cmp := func(x, y fake.Proc) int {
		if x.Pinned != y.Pinned {
			if x.Pinned {
				return -1
			}
			return 1
		}
		var l, r float64
		var ls, rs string
		useStr := false
		switch a.sortCol {
		case 0:
			useStr = true
			ls, rs = strings.ToLower(x.Name), strings.ToLower(y.Name)
		case 1:
			l, r = float64(x.Dir), float64(y.Dir)
		case 2:
			l, r = float64(x.PID), float64(y.PID)
		case 3:
			useStr = true
			ls, rs = x.Status, y.Status
		case 5:
			l, r = x.Mem, y.Mem
		case 6:
			l, r = x.Disk, y.Disk
		case 7:
			l, r = x.Net, y.Net
		case 8:
			l, r = x.GPU, y.GPU
		case 9:
			l, r = x.Commit, y.Commit // 提交大小 = 最新价
		case 10:
			l, r = float64(x.Faults), float64(y.Faults) // 页面错误 = 持仓均价
		default:
			l, r = x.CPU, y.CPU
		}
		if useStr {
			return strings.Compare(ls, rs)
		}
		switch {
		case l < r:
			return -1
		case l > r:
			return 1
		}
		return 0
	}
	sort.SliceStable(procs, func(i, j int) bool {
		c := cmp(procs[i], procs[j])
		if a.sortAsc {
			return c < 0
		}
		return c > 0
	})
}

func readKeys(t *Terminal, out chan<- Key) {
	buf := make([]byte, 128)
	pending := make([]byte, 0, 128)
	for {
		n, err := t.Read(buf)
		if err != nil {
			return
		}
		pending = append(pending, buf[:n]...)
		for len(pending) > 0 {
			k, size := parseKey(pending)
			if size == 0 {
				break // 序列还没收全，等下一批字节
			}
			pending = pending[size:]
			if k.R != 0 || k.Code != "" {
				select {
				case out <- k:
				default:
				}
			}
		}
	}
}

// parseKey 从字节流头部解析一个按键，返回消耗的字节数（0 表示需要更多数据）。
func parseKey(b []byte) (Key, int) {
	if len(b) == 0 {
		return Key{}, 0
	}
	if b[0] == 0x1b {
		if len(b) == 1 {
			return Key{Code: "esc"}, 1
		}
		if b[1] != '[' && b[1] != 'O' {
			return Key{Code: "esc"}, 1
		}
		if len(b) < 3 {
			return Key{}, 0
		}
		switch b[2] {
		case 'A':
			return Key{Code: "up"}, 3
		case 'B':
			return Key{Code: "down"}, 3
		case 'C':
			return Key{Code: "right"}, 3
		case 'D':
			return Key{Code: "left"}, 3
		}
		// 其余 escape 序列整段丢弃（如 PageUp/Home 等）
		for i := 2; i < len(b); i++ {
			if (b[i] >= 'A' && b[i] <= 'Z') || (b[i] >= 'a' && b[i] <= 'z') || b[i] == '~' {
				return Key{}, i + 1
			}
		}
		return Key{}, 0
	}
	if b[0] == '\t' {
		return Key{Code: "tab"}, 1
	}
	if b[0] == '\r' || b[0] == '\n' {
		return Key{Code: "enter"}, 1
	}
	r, size := utf8.DecodeRune(b)
	if r == utf8.RuneError && size <= 1 {
		if len(b) < 4 {
			return Key{}, 0
		}
		return Key{}, 1
	}
	return Key{R: r}, size
}

// ---- 格式化 ----

func fmtPct(v float64) string  { return fmt.Sprintf("%.1f%%", v) }
func fmtPct0(v float64) string { return fmt.Sprintf("%.0f%%", v) }

// dirMark 把盈亏方向渲染成「趋势」列的标记。
//
// 刻意用 ASCII 的 + / - 而不是 ▲▼ 之类箭头：那些字符属于 Unicode 的
// Ambiguous 宽度区，中文终端常按两列渲染，会让整张表的列对不齐。
// 无方向的记录（装饰进程、取数失败）留空。
func dirMark(dir int) string {
	switch {
	case dir > 0:
		return "+"
	case dir < 0:
		return "-"
	}
	return ""
}

func fmtMem(mb float64) string {
	switch {
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", mb/1024)
	case mb >= 1:
		return fmt.Sprintf("%.1f MB", mb)
	default:
		return fmt.Sprintf("%.0f KB", mb*1024)
	}
}

func fmtRate(v float64) string { return fmt.Sprintf("%.1f MB/s", v) }
func fmtNet(v float64) string  { return fmt.Sprintf("%.1f Mbps", v) }

// FmtCommit 渲染「提交大小」列。真实来源是最新价，取不到行情时留 "-"，
// 而不是显示 0 —— 一个 0 大小的进程在任务管理器里同样是不可能的。
//
// 固定用 MB 作单位、刻意不自动升到 GB：这样两列读法统一成「数字 × 10 = 价格」。
// 一旦自动升到 GB，BTC 那行就变成 6.2 GB，反算回价格还得再乘 1024，反而费事。
//
// 导出是为了让 -check 自检表格复用同一套格式，避免两处规则各写一遍后走偏。
func FmtCommit(mb float64) string {
	if mb <= 0 {
		return "-"
	}
	if mb < 1 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmtInt(int(mb+0.5)) + " MB"
}

// FmtFaults 渲染「页面错误」列。真实来源是持仓均价；网格/信号这类
// 不返回均价字段的策略会留 "-"。
func FmtFaults(n int) string {
	if n <= 0 {
		return "-"
	}
	return fmtInt(n)
}

// fmtInt 给整数加千分位，跟任务管理器里那一列的样子对齐。
func fmtInt(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func fmtSigned(v float64) string { return fmt.Sprintf("%+.2f", v) }

func sideLabel(s string) string {
	switch s {
	case "long":
		return "多"
	case "short":
		return "空"
	case "net":
		return "双向"
	case "neutral":
		return "中性"
	}
	return "-"
}

func leverLabel(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0fx", v)
}

func blockRune(frac float64) rune {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	i := int(frac*8 + 0.5)
	if i < 1 {
		i = 1
	}
	if i > 8 {
		i = 8
	}
	return blocks[i-1]
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
