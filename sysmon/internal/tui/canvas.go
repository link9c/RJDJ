package tui

import (
	"strings"
	"unicode/utf8"
)

// Color 使用 256 色索引；ColorDefault 表示不显式设置（沿用终端默认）。
type Color int16

// ColorDefault 表示交给终端默认色。
const ColorDefault Color = -1

// Cell 画布上的一个字符单元。
type Cell struct {
	R    rune
	Fg   Color
	Bg   Color
	Bold bool
}

// Canvas 一个按显示列/行寻址的字符画布。
// 宽字符（中文、全角符号）占两列，第二列用 R==0 占位。
type Canvas struct {
	W, H  int
	cells []Cell
}

// NewCanvas 创建画布。
func NewCanvas(w, h int) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	c := &Canvas{W: w, H: h, cells: make([]Cell, w*h)}
	c.Clear(ColorDefault, ColorDefault)
	return c
}

// Clear 用空格铺满画布。
func (c *Canvas) Clear(fg, bg Color) {
	for i := range c.cells {
		c.cells[i] = Cell{R: ' ', Fg: fg, Bg: bg}
	}
}

// Fill 用指定背景色填充一块矩形区域。
func (c *Canvas) Fill(x, y, w, h int, fg, bg Color) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			c.Set(i, j, ' ', fg, bg)
		}
	}
}

// Set 写入单个字符。
func (c *Canvas) Set(x, y int, r rune, fg, bg Color) {
	c.SetBold(x, y, r, fg, bg, false)
}

// SetBold 写入单个字符并指定是否加粗。
func (c *Canvas) SetBold(x, y int, r rune, fg, bg Color, bold bool) {
	if y < 0 || y >= c.H || x < 0 || x >= c.W {
		return
	}
	// 若目标位置原本是宽字符的右半格，先把那个宽字符抹成空格，避免残留
	if x > 0 {
		if idx := y*c.W + x - 1; c.cells[idx].R != 0 && runeWidth(c.cells[idx].R) == 2 {
			c.cells[idx].R = ' '
		}
	}
	w := runeWidth(r)
	c.cells[y*c.W+x] = Cell{R: r, Fg: fg, Bg: bg, Bold: bold}
	if w == 2 && x+1 < c.W {
		c.cells[y*c.W+x+1] = Cell{R: 0, Fg: fg, Bg: bg, Bold: bold}
	}
}

// Text 写入一个字符串（自动处理宽字符与截断）。
func (c *Canvas) Text(x, y int, s string, fg, bg Color) {
	c.TextBold(x, y, s, fg, bg, false)
}

// TextBold 写入字符串并指定是否加粗。
func (c *Canvas) TextBold(x, y int, s string, fg, bg Color, bold bool) {
	cx := x
	for _, r := range s {
		if cx >= c.W {
			break
		}
		w := runeWidth(r)
		if cx+w > c.W {
			break
		}
		c.SetBold(cx, y, r, fg, bg, bold)
		cx += w
	}
}

// TextRight 在给定右边界处右对齐写入字符串。
func (c *Canvas) TextRight(xRight, y int, s string, fg, bg Color) {
	w := DisplayWidth(s)
	c.Text(xRight-w, y, s, fg, bg)
}

// TextCenter 在 [x, x+w) 区间内居中写入。
func (c *Canvas) TextCenter(x, y, w int, s string, fg, bg Color) {
	sw := DisplayWidth(s)
	c.Text(x+(w-sw)/2, y, s, fg, bg)
}

// HLine 画一条水平线。
func (c *Canvas) HLine(x, y, w int, r rune, fg, bg Color) {
	for i := 0; i < w; i++ {
		c.Set(x+i, y, r, fg, bg)
	}
}

// VLine 画一条垂直线。
func (c *Canvas) VLine(x, y, h int, r rune, fg, bg Color) {
	for j := 0; j < h; j++ {
		c.Set(x, y+j, r, fg, bg)
	}
}

// Box 画一个矩形边框。
func (c *Canvas) Box(x, y, w, h int, fg, bg Color) {
	if w < 2 || h < 2 {
		return
	}
	c.Set(x, y, '┌', fg, bg)
	c.Set(x+w-1, y, '┐', fg, bg)
	c.Set(x, y+h-1, '└', fg, bg)
	c.Set(x+w-1, y+h-1, '┘', fg, bg)
	c.HLine(x+1, y, w-2, '─', fg, bg)
	c.HLine(x+1, y+h-1, w-2, '─', fg, bg)
	c.VLine(x, y+1, h-2, '│', fg, bg)
	c.VLine(x+w-1, y+1, h-2, '│', fg, bg)
}

// Render 把画布序列化成 ANSI 帧。整帧一次性输出，避免闪烁。
func (c *Canvas) Render() string {
	var sb strings.Builder
	sb.Grow(c.W*c.H*4 + 128)
	sb.WriteString("\x1b[H")

	for y := 0; y < c.H; y++ {
		curFg, curBg := Color(-2), Color(-2)
		curBold := false
		for x := 0; x < c.W; x++ {
			cell := c.cells[y*c.W+x]
			if cell.R == 0 {
				continue // 被前一个宽字符覆盖
			}
			if cell.Fg != curFg || cell.Bg != curBg || cell.Bold != curBold {
				writeSGR(&sb, cell.Fg, cell.Bg, cell.Bold)
				curFg, curBg, curBold = cell.Fg, cell.Bg, cell.Bold
			}
			sb.WriteRune(cell.R)
		}
		sb.WriteString("\x1b[0m\x1b[K")
		if y < c.H-1 {
			sb.WriteString("\r\n")
		}
	}
	sb.WriteString("\x1b[J")
	return sb.String()
}

// PlainText 输出不带任何 ANSI 转义的纯文本，用于快照预览与自动化测试。
func (c *Canvas) PlainText() string {
	var sb strings.Builder
	sb.Grow(c.W*c.H + c.H)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.cells[y*c.W+x]
			if cell.R == 0 {
				continue // 被宽字符覆盖的占位格
			}
			sb.WriteRune(cell.R)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func writeSGR(sb *strings.Builder, fg, bg Color, bold bool) {
	sb.WriteString("\x1b[0")
	if bold {
		sb.WriteString(";1")
	}
	if fg != ColorDefault {
		sb.WriteString(";38;5;")
		sb.WriteString(itoa(int(fg)))
	}
	if bg != ColorDefault {
		sb.WriteString(";48;5;")
		sb.WriteString(itoa(int(bg)))
	}
	sb.WriteByte('m')
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---- 显示宽度工具 ----

// runeWidth 估算字符在终端中占用的列数（中文/全角算 2 列）。
func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20:
		return 0
	case r < 0x7F:
		return 1
	}
	switch {
	case r >= 0x1100 && r <= 0x115F, // 谚文字母
		r >= 0x2E80 && r <= 0x303E, // CJK 部首 / 标点
		r >= 0x3041 && r <= 0x33FF, // 假名、注音、CJK 兼容
		r >= 0x3400 && r <= 0x4DBF, // CJK 扩展 A
		r >= 0x4E00 && r <= 0x9FFF, // CJK 基本区
		r >= 0xA000 && r <= 0xA4CF, // 彝文
		r >= 0xAC00 && r <= 0xD7A3, // 韩文音节
		r >= 0xF900 && r <= 0xFAFF, // CJK 兼容表意
		r >= 0xFE30 && r <= 0xFE6F, // CJK 兼容形式
		r >= 0xFF00 && r <= 0xFF60, // 全角 ASCII
		r >= 0xFFE0 && r <= 0xFFE6, // 全角符号
		r >= 0x20000 && r <= 0x2FFFD,
		r >= 0x30000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

// DisplayWidth 返回字符串在终端中占用的列数。
func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// Truncate 按显示宽度截断字符串，超出部分用省略号代替。
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if DisplayWidth(s) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	var sb strings.Builder
	cur := 0
	for _, r := range s {
		rw := runeWidth(r)
		if cur+rw > w-1 {
			break
		}
		sb.WriteRune(r)
		cur += rw
	}
	sb.WriteString("…")
	return sb.String()
}

// PadRight 右侧补空格到指定显示宽度。
func PadRight(s string, w int) string {
	d := w - DisplayWidth(s)
	if d <= 0 {
		return Truncate(s, w)
	}
	return s + strings.Repeat(" ", d)
}

// PadLeft 左侧补空格到指定显示宽度。
func PadLeft(s string, w int) string {
	d := w - DisplayWidth(s)
	if d <= 0 {
		return Truncate(s, w)
	}
	return strings.Repeat(" ", d) + s
}

// FitCell 把字符串裁成恰好 w 列（先截断再补齐）。
func FitCell(s string, w int) string {
	s = Truncate(s, w)
	return PadRight(s, w)
}

var _ = utf8.RuneLen
