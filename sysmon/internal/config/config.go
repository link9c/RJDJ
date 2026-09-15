package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultFileName 默认配置文件名，与可执行文件同目录或当前目录。
const DefaultFileName = "config.json"

// OKXCfg OKX 接口凭证。只读权限即可（行情/持仓/策略查询）。
type OKXCfg struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Passphrase string `json:"passphrase"`
	Simulated  bool   `json:"simulated"` // true=模拟盘（会带 x-simulated-trading: 1）
	BaseURL    string `json:"base_url"`  // 留空用官方域名，可填代理域名
	Mock       bool   `json:"mock"`      // true=不请求接口，用内置假数据渲染界面（离线调试外观用）
}

// UICfg 界面外观与刷新行为。
type UICfg struct {
	Title      string  `json:"title"`        // 窗口标题（伪装用）
	RefreshMS  int     `json:"refresh_ms"`   // 刷新间隔，毫秒
	Theme      string  `json:"theme"`        // dark | light
	Hostname   string  `json:"hostname"`     // 标题栏右侧显示的主机名
	CPUName    string  `json:"cpu_name"`     // 性能页展示的处理器型号
	CoreCount  int     `json:"core_count"`   // 性能页展示的核心数
	ShowDetail bool    `json:"show_detail"`  // 是否显示"详细信息"页签（该页会暴露真实标的名称）
	ShowPrice  bool    `json:"show_price"`   // 是否显示「提交大小/页面错误」两列（真实来源是最新价与持仓均价）
	DefaultTab string  `json:"default_tab"`  // processes | performance | detail
	DecoyCount int     `json:"decoy_count"`  // 额外混入的装饰性系统进程数量（增强伪装）
	MemTotalGB float64 `json:"mem_total_gb"` // 性能页展示的"物理内存总量"(GB)，纯装饰
}

// ProcessCfg 一条伪装映射：把某个 OKX 标的或策略伪装成一个系统进程。
//
// 例如 name=BTC-cpu, inst_id=BTC-USDT-SWAP, source=position，
// 界面上就会以 "BTC-cpu" 这个名字展示该持仓的监控指标。
type ProcessCfg struct {
	Name   string `json:"name"`    // 进程显示名（伪装名）
	InstID string `json:"inst_id"` // 对应标的，如 BTC-USDT-SWAP；source=auto 时可留空
	Source string `json:"source"`  // position | grid | dca | recurring | signal | algo | auto
	User   string `json:"user"`    // 任务管理器"用户名"列，留空用默认
	// 下面五个系数默认都是 1：真实数值直接带上单位就是该列显示的数字，
	// 不做任何倍率换算（收益率 3.5% → CPU 3.5%，保证金 1240 → 内存 1240 MB）。
	// 只有在想让某列数字整体放大/缩小时才需要改动。
	CpuScale  float64 `json:"cpu_scale"`  // 收益率(%) → CPU% 的缩放系数
	MemScale  float64 `json:"mem_scale"`  // 保证金(USDT) → 内存(MB) 的缩放系数
	DiskScale float64 `json:"disk_scale"` // 浮动盈亏(USDT) → 磁盘(MB/s) 的缩放系数
	NetScale  float64 `json:"net_scale"`  // 24h涨跌幅(%) → 网络(Mbps) 的缩放系数
	GpuScale  float64 `json:"gpu_scale"`  // 杠杆倍数 → GPU% 的缩放系数
	Pinned    bool    `json:"pinned"`     // 钉在顶部（界面排序用）

	// 下面两个系数把行情伪装成任务管理器的两列整数/容量指标。
	// 默认都是 0.1，也就是这两列的数字 × 10 就是真实价格。
	PriceScale float64 `json:"price_scale"` // 最新价 → 「提交大小」(MB) 的缩放系数
	AvgScale   float64 `json:"avg_scale"`   // 持仓均价 → 「页面错误」的缩放系数
}

// Config 顶层配置。
type Config struct {
	OKX       OKXCfg       `json:"okx"`
	UI        UICfg        `json:"ui"`
	Processes []ProcessCfg `json:"processes"`
	path      string       // 实际加载的配置文件路径
}

// Default 返回内置默认配置（未提供配置文件时使用）。
func Default() *Config {
	return &Config{
		OKX: OKXCfg{
			BaseURL: "https://www.okx.com",
		},
		UI: UICfg{
			Title:      "任务管理器",
			RefreshMS:  3000,
			Theme:      "dark",
			Hostname:   "",
			CPUName:    "Intel(R) Core(TM) i7-10700 CPU @ 2.90GHz",
			CoreCount:  8,
			ShowDetail: true,
			ShowPrice:  true,
			DefaultTab: "processes",
			DecoyCount: 8,
			MemTotalGB: 32,
		},
		Processes: nil,
	}
}

// Load 加载配置。path 为空时按顺序查找：
//  1. 当前目录 ./config.json
//  2. 可执行文件同目录 config.json
//  3. 当前目录 ./config.example.json
//
// 找不到任何文件时返回默认配置（并标记 path 为空，由上层提示）。
func Load(path string) (*Config, error) {
	cfg := Default()

	resolved := ""
	if strings.TrimSpace(path) != "" {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("指定的配置文件不存在: %s", path)
		}
		resolved = path
	} else {
		candidates := []string{DefaultFileName, "config.example.json"}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates,
				filepath.Join(dir, DefaultFileName),
				filepath.Join(dir, "config.example.json"))
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				resolved = c
				break
			}
		}
	}

	if resolved != "" {
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("读取配置失败: %w", err)
		}
		// 合并到默认值之上：JSON 中缺失的字段保留默认。
		if err := json.Unmarshal(stripJSONComments(stripBOM(raw)), cfg); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w（请检查 JSON 语法，注意不能有注释和尾逗号）", resolved, err)
		}
		cfg.path = resolved
	}

	cfg.normalize()
	cfg.applyEnv()
	cfg.normalize()
	return cfg, nil
}

// Path 返回实际加载的配置文件路径，未加载到文件时为空。
func (c *Config) Path() string { return c.path }

// Normalize 补齐空字段并纠正非法值。导出版本供外部构造配置时复用
// （例如测试里直接拼一个 Config 而不经过 Load）。
func (c *Config) Normalize() { c.normalize() }

// normalize 补齐空字段并纠正非法值。
func (c *Config) normalize() {
	if c.OKX.BaseURL == "" {
		c.OKX.BaseURL = "https://www.okx.com"
	}
	if c.UI.Title == "" {
		c.UI.Title = "任务管理器"
	}
	if c.UI.RefreshMS < 500 {
		c.UI.RefreshMS = 3000
	}
	if c.UI.Theme != "light" {
		c.UI.Theme = "dark"
	}
	if c.UI.CoreCount <= 0 {
		c.UI.CoreCount = 8
	}
	if c.UI.DecoyCount < 0 {
		c.UI.DecoyCount = 0
	}
	if c.UI.MemTotalGB <= 0 {
		// 默认 32 而不是 16：内存列 1:1 等于保证金(1U→1MB)，要装得下所有持仓的
		// 保证金之和，否则性能页的内存占比会一直贴在 100%，反而不像真机。
		c.UI.MemTotalGB = 32
	}
	if c.UI.CPUName == "" {
		c.UI.CPUName = "Intel(R) Core(TM) i7-10700 CPU @ 2.90GHz"
	}
	if c.UI.Hostname == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			c.UI.Hostname = strings.ToUpper(h)
		} else {
			c.UI.Hostname = "DESKTOP-PC"
		}
	}
	switch c.UI.DefaultTab {
	case "processes", "performance", "detail":
	default:
		c.UI.DefaultTab = "processes"
	}

	// 没有任何进程映射时给一份开箱即用的兜底：自动铺满账户里所有持仓与策略。
	// 必须在补缩放系数之前追加，否则新加的条目拿不到默认系数。
	if len(c.Processes) == 0 {
		c.Processes = []ProcessCfg{{Source: "auto"}}
	}

	// 每条进程映射补齐默认缩放系数与用户名。
	for i := range c.Processes {
		p := &c.Processes[i]
		p.Source = strings.ToLower(strings.TrimSpace(p.Source))
		switch p.Source {
		case "position", "grid", "dca", "recurring", "signal", "algo", "auto":
		default:
			p.Source = "auto"
		}
		// 全部默认 1:1：不加任何倍率，真实数值直接带上单位就是该列的数字。
		// 这样在界面上读到的数字就是原始数据，不用再心算还原。
		if p.CpuScale <= 0 {
			p.CpuScale = 1
		}
		if p.MemScale <= 0 {
			p.MemScale = 1
		}
		if p.DiskScale <= 0 {
			p.DiskScale = 1
		}
		if p.NetScale <= 0 {
			p.NetScale = 1
		}
		if p.GpuScale <= 0 {
			p.GpuScale = 1
		}
		// 默认 0.1：这两列的数字 × 10 就是真实价格。
		// 不取 1 的原因：BTC 现价 63,385 会原样显示成 61.9 GB 的「提交大小」，
		// 在一台只有十几 GB 内存的机器上太扎眼。取 0.1 后落在 6.2 GB，正常范围。
		if p.PriceScale <= 0 {
			p.PriceScale = 0.1
		}
		if p.AvgScale <= 0 {
			p.AvgScale = 0.1
		}
	}

}

// applyEnv 允许用环境变量覆盖敏感字段，避免明文写入配置文件。
func (c *Config) applyEnv() {
	if v := os.Getenv("OKX_API_KEY"); v != "" {
		c.OKX.APIKey = v
	}
	if v := os.Getenv("OKX_API_SECRET"); v != "" {
		c.OKX.APISecret = v
	}
	if v := os.Getenv("OKX_PASSPHRASE"); v != "" {
		c.OKX.Passphrase = v
	}
	if v := os.Getenv("OKX_BASE_URL"); v != "" {
		c.OKX.BaseURL = v
	}
	if v := os.Getenv("OKX_SIMULATED"); v == "1" || strings.EqualFold(v, "true") {
		c.OKX.Simulated = true
	}
}

// stripBOM 去掉 UTF-8 BOM，避免 JSON 解析报 unexpected token。
func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// stripJSONComments 去掉 JSON 中的 // 行注释与 /* */ 块注释，
// 让配置文件可以像下面这样写说明而不必牺牲可读性：
//
//	{
//	  // 你的 OKX 只读 API Key
//	  "api_key": "xxxx"
//	}
//
// 字符串字面量内部的内容不会被误删。
func stripJSONComments(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(b) {
			if b[i+1] == '/' {
				for i < len(b) && b[i] != '\n' {
					i++
				}
				out = append(out, '\n')
				continue
			}
			if b[i+1] == '*' {
				i += 2
				for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// Redacted 返回脱敏后的 key 摘要，用于启动自检输出。
func (o OKXCfg) Redacted() string {
	if o.APIKey == "" {
		return "(未配置)"
	}
	k := o.APIKey
	if len(k) > 8 {
		k = k[:4] + "****" + k[len(k)-4:]
	} else {
		k = "****"
	}
	if o.Simulated {
		return k + " [模拟盘]"
	}
	return k + " [实盘]"
}
