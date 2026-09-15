//go:build !windows

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Terminal 类 Unix 平台实现：借助 stty 完成 raw 模式与尺寸探测。
// 主要面向 Windows，这里保证跨平台可编译可用。
type Terminal struct {
	raw bool
}

// OpenTerminal 初始化终端。
func OpenTerminal() (*Terminal, error) { return &Terminal{}, nil }

// Size 通过 stty size 获取终端尺寸，取不到时退化为环境变量/默认值。
func (t *Terminal) Size() (int, int) {
	dm := exec.Command("stty", "size")
	dm.Stdin = os.Stdin
	if out, err := dm.Output(); err == nil {
		if f := strings.Fields(string(out)); len(f) == 2 {
			h, _ := strconv.Atoi(f[0])
			w, _ := strconv.Atoi(f[1])
			if w > 0 && h > 0 {
				return w, h
			}
		}
	}
	w, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	h, _ := strconv.Atoi(os.Getenv("LINES"))
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 30
	}
	return w, h
}

// EnterRaw 打开 raw 模式并隐藏光标。
func (t *Terminal) EnterRaw() error {
	cmd := exec.Command("stty", "raw", "-echo")
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("无法切换到 raw 模式: %w", err)
	}
	t.raw = true
	_, err := os.Stdout.WriteString("\x1b[?25l")
	return err
}

// Restore 还原终端设置。
func (t *Terminal) Restore() {
	_, _ = os.Stdout.WriteString("\x1b[?25h\x1b[0m")
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
	t.raw = false
}

// Read 读取按键字节。
func (t *Terminal) Read(buf []byte) (int, error) { return os.Stdin.Read(buf) }

// WriteString 输出一帧。
func (t *Terminal) WriteString(s string) error {
	_, err := os.Stdout.WriteString(s)
	return err
}
