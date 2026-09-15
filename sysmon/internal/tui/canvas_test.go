package tui

import (
	"strings"
	"testing"

	"sysmon/internal/config"
)

// 复现：靠近右边缘的文本只画出了第一个字符。
func TestTextNearRightEdge(t *testing.T) {
	const W = 140
	c := NewCanvas(W, 1)
	c.Clear(ColorDefault, ColorDefault)

	x := 1
	x += 70 + 1 // 名称
	x += 7 + 1  // PID
	x += 8 + 1  // 状态
	x += 9 + 1  // CPU
	x += 11 + 1 // 内存
	x += 10 + 1 // 磁盘
	x += 10 + 1 // 网络

	t.Logf("GPU 列起始 x=%d，画布宽 W=%d", x, W)
	c.Text(x, 0, "GPU   ", ColorDefault, ColorDefault)

	line := strings.TrimRight(c.PlainText(), "\n")
	t.Logf("渲染结果 = %q", line)
	if !strings.Contains(line, "GPU") {
		t.Fatalf("GPU 未被完整绘制，实际内容: %q", line)
	}
}

// 中文与宽字符混排时，右侧内容不应被吃掉。
func TestWideRunesThenText(t *testing.T) {
	c := NewCanvas(40, 1)
	c.Text(1, 0, "网络", ColorDefault, ColorDefault)
	c.Text(10, 0, "GPU", ColorDefault, ColorDefault)
	line := c.PlainText()
	t.Logf("结果 = %q", line)
	if !strings.Contains(line, "GPU") {
		t.Fatalf("GPU 丢失: %q", line)
	}
}

// 行情两列的格式化：提交大小固定用 MB，页面错误要带千分位，无数据一律 "-"。
func TestPriceColumnFormat(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"无数据留横杠", FmtCommit(0), "-"},
		{"小于 1MB 保留一位小数", FmtCommit(0.3), "0.3 MB"},
		{"整数带千分位", FmtCommit(6338.5), "6,339 MB"},
		{"万位以上", FmtCommit(63385), "63,385 MB"},
		{"页面错误无数据", FmtFaults(0), "-"},
		{"页面错误千分位", FmtFaults(6123), "6,123"},
		{"页面错误百万位", FmtFaults(1234567), "1,234,567"},
		{"三位数不加逗号", FmtFaults(612), "612"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: 得到 %q，期望 %q", c.name, c.got, c.want)
		}
	}
}

// 两列必须装得下 10 列宽：格式变了宽度不跟着变就会把后面的列挤出去。
func TestPriceColumnFitsWidth(t *testing.T) {
	for _, s := range []string{FmtCommit(6338.5), FmtCommit(63385), FmtCommit(0.3), FmtFaults(1234567)} {
		if got := DisplayWidth(s); got > 10 {
			t.Errorf("%q 宽 %d，超过列的 10 列宽", s, got)
		}
	}
}

// 窄终端下每一列都必须落在画布内，不允许被挤到边界之外。
func TestProcessTableFitsWidth(t *testing.T) {
	cfg := config.Default()
	// 112 是行情两列（提交大小/页面错误）的显示门槛，把前后几个宽度都覆盖上。
	for _, w := range []int{60, 76, 88, 100, 108, 111, 112, 113, 118, 160} {
		for _, tab := range []int{0, 1, 2} {
			frame := DumpFrame(cfg, w, 30, tab)
			for i, line := range strings.Split(strings.TrimRight(frame, "\n"), "\n") {
				if got := DisplayWidth(line); got > w {
					t.Fatalf("w=%d tab=%d 第 %d 行超宽: %d > %d\n%s", w, tab, i+1, got, w, line)
				}
			}
		}
	}
}
