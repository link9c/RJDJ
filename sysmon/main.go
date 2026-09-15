// sysmon —— 把 OKX 持仓与策略机器人伪装成系统监控指标的终端看板。
//
// 界面外观对标 Windows 任务管理器（进程 / 性能 / 详细信息三个页签），
// 每个真实的合约持仓或策略机器人会被映射成一条"进程"记录：
//
//	收益率   → CPU 占用      （1:1，单位 %）
//	保证金   → 内存占用      （1:1，单位 MB）
//	浮动盈亏 → 磁盘吞吐      （1:1，单位 MB/s）
//	24h 涨跌 → 网络流量      （1:1，单位 Mbps）
//	杠杆倍数 → GPU 占用      （1:1，单位 %）
//	最新价   → 提交大小      （÷10）
//	持仓均价 → 页面错误      （÷10）
//
// 前五列不做倍率换算，界面上读到的数字就是真实数值本身。
//
// 用法：
//
//	sysmon                    使用当前目录 / 程序目录下的 config.json
//	sysmon -c my.json         指定配置文件
//	sysmon -check             只做配置与接口连通性自检，不进界面
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"sysmon/internal/config"
	"sysmon/internal/fake"
	"sysmon/internal/feed"
	"sysmon/internal/tui"
)

const version = "1.0.0"

func main() {
	var (
		cfgPath string
		check   bool
		showVer bool
	)
	flag.StringVar(&cfgPath, "c", "", "配置文件路径（默认按 ./config.json → 程序目录/config.json 查找）")
	flag.BoolVar(&check, "check", false, "只做配置与接口连通性自检，不启动界面")
	flag.BoolVar(&showVer, "v", false, "显示版本号")
	dump := flag.String("dump", "", "把界面渲染成纯文本快照输出（格式 宽x高，如 120x30），用于预览外观")
	flag.Parse()

	if showVer {
		fmt.Printf("sysmon %s\n", version)
		return
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fail(err)
	}

	if *dump != "" {
		runDump(cfg, *dump)
		return
	}

	if check {
		runCheck(cfg)
		return
	}

	term, err := tui.OpenTerminal()
	if err != nil {
		fail(err)
	}

	app := tui.NewApp(cfg, term)
	if err := app.Run(); err != nil {
		fail(err)
	}
}

// runCheck 打印配置概览并试拉一次数据，便于先确认凭证与映射是否正确。
func runCheck(cfg *config.Config) {
	fmt.Printf("sysmon %s  配置自检\n", version)
	fmt.Println("─────────────────────────────────────────────")
	if cfg.Path() != "" {
		fmt.Printf("配置文件      : %s\n", cfg.Path())
	} else {
		fmt.Println("配置文件      : (未找到，使用内置默认值)")
	}
	fmt.Printf("OKX API Key   : %s\n", cfg.OKX.Redacted())
	fmt.Printf("OKX 域名      : %s\n", cfg.OKX.BaseURL)
	fmt.Printf("刷新间隔      : %d ms\n", cfg.UI.RefreshMS)
	fmt.Printf("窗口标题      : %s\n", cfg.UI.Title)
	fmt.Printf("伪装进程条目  : %d 条配置 / 额外装饰 %d 条\n", len(cfg.Processes), cfg.UI.DecoyCount)

	f := feed.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("正在拉取 OKX 数据…")
	snap := f.Fetch(ctx)
	if snap.Err != "" {
		fmt.Printf("拉取失败      : %s\n", snap.Err)
	}
	for _, wmsg := range snap.Warnings {
		fmt.Printf("警告          : %s\n", wmsg)
	}
	fmt.Printf("账户总权益    : %.2f USD\n", snap.TotalEq)
	fmt.Printf("取得条目      : %d 条\n", len(snap.Rows))

	eng := fake.NewEngine(cfg)
	procs := eng.Project(snap.Rows, time.Now())

	fmt.Printf("行情映射      : 现价 → 提交大小、持仓均价 → 页面错误；%s\n",
		"两列的默认系数都是 0.1，即该列数字 × 10 就是价格")

	fmt.Println("─────────────────────────────────────────────")
	fmt.Printf("%-20s %-18s %6s %7s %10s %9s %12s %12s %10s %10s\n",
		"伪装名", "对应标的", "趋势", "CPU%", "内存", "收益率", "现价", "持仓均价", "提交大小", "页面错误")
	for _, p := range procs {
		inst := "(装饰进程)"
		if p.Real != nil {
			inst = p.Real.InstID
		}
		fmt.Printf("%-20s %-18s %6s %6.1f%% %9.1f MB %8.2f%% %12s %12s %10s %10s\n",
			trunc(p.Name, 18), trunc(inst, 16), dirLabel(p.Dir), p.CPU, p.Mem, uplPct(p),
			numOrDash(p.Real, func(r *feed.Row) float64 { return r.MarkPx }),
			numOrDash(p.Real, func(r *feed.Row) float64 { return r.AvgPx }),
			tui.FmtCommit(p.Commit), tui.FmtFaults(p.Faults))
	}

	if f.MissingCredential() && !cfg.OKX.Mock {
		fmt.Println("─────────────────────────────────────────────")
		fmt.Println("提示：上面是演示数据。请在配置文件的 okx 段填入 api_key / api_secret / passphrase。")
	}
}

// runDump 把各页签渲染成纯文本打印出来，便于预览外观或做回归对比。
func runDump(cfg *config.Config, size string) {
	w, h := 120, 30
	if _, err := fmt.Sscanf(strings.TrimSpace(strings.ToLower(size)), "%dx%d", &w, &h); err != nil || w < 40 || h < 10 {
		fmt.Fprintln(os.Stderr, "尺寸格式应为 宽x高，例如 -dump 120x30")
		os.Exit(1)
	}
	names := []string{"进程", "性能", "详细信息"}
	for i, name := range names {
		if i == 2 && !cfg.UI.ShowDetail {
			continue
		}
		fmt.Printf("════════ %s 页（%dx%d）════════\n", name, w, h)
		fmt.Print(tui.DumpFrame(cfg, w, h, i))
		fmt.Println()
	}
}

// dirLabel 把盈亏方向写成自检表格里的文字，与界面「趋势」列同义。
func dirLabel(dir int) string {
	switch {
	case dir > 0:
		return "+ 盈"
	case dir < 0:
		return "- 亏"
	}
	return "—"
}

func uplPct(p fake.Proc) float64 {
	if p.Real == nil {
		return 0
	}
	return p.Real.UplRatio * 100
}

// numOrDash 取真实行上的一个数值字段；装饰进程或取不到值时显示 "-"，
// 避免把 0 当成一个"真的价格是 0"来误读。
func numOrDash(r *feed.Row, pick func(*feed.Row) float64) string {
	if r == nil {
		return "-"
	}
	if v := pick(r); v > 0 {
		return fmt.Sprintf("%.2f", v)
	}
	return "-"
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "\n启动失败:", err)
	os.Exit(1)
}
