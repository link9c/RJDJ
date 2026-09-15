package config

import (
	"strings"
	"testing"
)

// 配置里的 // 与 /* */ 注释必须被正确剥离，同时不能误伤字符串内的斜杠。
func TestStripJSONComments(t *testing.T) {
	in := []byte(`{
		// 行注释
		"a": "https://www.okx.com/v5", /* 块注释 */
		"b": 1
	}`)
	out := string(stripJSONComments(in))
	if !strings.Contains(out, "https://www.okx.com/v5") {
		t.Fatalf("字符串内的 // 被误删: %s", out)
	}
	if strings.Contains(out, "行注释") || strings.Contains(out, "块注释") {
		t.Fatalf("注释未被移除: %s", out)
	}
}

// 空配置应当被补上可用的默认值。
func TestNormalizeDefaults(t *testing.T) {
	c := &Config{}
	c.normalize()
	if c.UI.Title == "" || c.UI.RefreshMS < 500 || c.UI.MemTotalGB <= 0 {
		t.Fatalf("UI 默认值缺失: %+v", c.UI)
	}
	if c.OKX.BaseURL == "" {
		t.Fatal("BaseURL 未补默认值")
	}
	if len(c.Processes) == 0 {
		t.Fatal("未生成默认真人映射")
	}
	for _, p := range c.Processes {
		if p.CpuScale <= 0 || p.MemScale <= 0 || p.DiskScale <= 0 || p.NetScale <= 0 || p.GpuScale <= 0 {
			t.Fatalf("缩放系数未补默认值: %+v", p)
		}
		if p.PriceScale <= 0 || p.AvgScale <= 0 {
			t.Fatalf("行情缩放系数未补默认值: %+v", p)
		}
	}
}

// 五个监控指标系数默认必须都是 1（1:1），不做任何倍率换算：
// 界面上显示的数字就是真实数值本身，避免看盘时还要心算还原。
func TestIndicatorScalesDefaultToOne(t *testing.T) {
	c := &Config{}
	c.normalize()
	for _, p := range c.Processes {
		if p.CpuScale != 1 || p.MemScale != 1 || p.DiskScale != 1 ||
			p.NetScale != 1 || p.GpuScale != 1 {
			t.Fatalf("监控指标缩放系数默认值应为 1，实际 cpu=%v mem=%v disk=%v net=%v gpu=%v",
				p.CpuScale, p.MemScale, p.DiskScale, p.NetScale, p.GpuScale)
		}
	}
}

// 行情两列默认 0.1：用 1 的话 BTC 现价会原样显示成 61.9 GB 的「提交大小」，
// 在一台只有十几 GB 内存的机器上太扎眼。
func TestPriceScaleDefaults(t *testing.T) {
	c := &Config{}
	c.normalize()
	for _, p := range c.Processes {
		if p.PriceScale != 0.1 || p.AvgScale != 0.1 {
			t.Fatalf("行情缩放系数默认值应为 0.1，实际 %v / %v", p.PriceScale, p.AvgScale)
		}
	}
}

// 凭证脱敏不应泄露完整 key。
func TestRedacted(t *testing.T) {
	c := OKXCfg{APIKey: "abcdefgh12345678", APISecret: "x", Passphrase: "y"}
	got := c.Redacted()
	if strings.Contains(got, "12345678") || !strings.Contains(got, "****") {
		t.Fatalf("脱敏不正确: %s", got)
	}
	if c2 := (OKXCfg{}); c2.Redacted() != "(未配置)" {
		t.Fatalf("空凭证提示不正确: %s", c2.Redacted())
	}
}
