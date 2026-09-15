//go:build windows

package tui

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// 通过 kernel32 直接用控制台 API，不引入任何第三方依赖。
const (
	stdInputHandle  = ^uintptr(9)  // -10
	stdOutputHandle = ^uintptr(10) // -11

	enableProcessedOutput           = 0x0001
	enableVirtualTerminalProcessing = 0x0004

	enableProcessedInput       = 0x0001
	enableLineInput            = 0x0002
	enableEchoInput            = 0x0004
	enableVirtualTerminalInput = 0x0200

	utf8CodePage = 65001
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetStdHandle               = kernel32.NewProc("GetStdHandle")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorInfo       = kernel32.NewProc("SetConsoleCursorInfo")
	procSetConsoleOutputCP         = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP               = kernel32.NewProc("SetConsoleCP")
	procWriteConsoleW              = kernel32.NewProc("WriteConsoleW")
)

type coord struct{ X, Y int16 }

type smallRect struct{ Left, Top, Right, Bottom int16 }

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

type consoleCursorInfo struct {
	Size    uint32
	Visible int32
}

// Terminal 对 Windows 控制台的薄封装：负责尺寸探测、raw 模式与光标隐藏。
type Terminal struct {
	out    syscall.Handle
	in     syscall.Handle
	oldOut uint32
	oldIn  uint32
	oldCur consoleCursorInfo
	raw    bool
}

// OpenTerminal 打开并初始化控制台（开启 VT 转义处理，切换 UTF-8 代码页）。
func OpenTerminal() (*Terminal, error) {
	t := &Terminal{}
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	t.out = syscall.Handle(h)
	h, _, _ = procGetStdHandle.Call(stdInputHandle)
	t.in = syscall.Handle(h)
	if t.out == 0 || t.in == 0 {
		return nil, fmt.Errorf("无法获取控制台句柄（请在真实终端中运行）")
	}

	// 切到 UTF-8，保证中文与块状字符正常显示
	procSetConsoleOutputCP.Call(utf8CodePage)
	procSetConsoleCP.Call(utf8CodePage)

	// 输出模式：开启 ANSI 转义序列处理
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(t.out), uintptr(unsafe.Pointer(&mode))); r != 0 {
		t.oldOut = mode
		procSetConsoleMode.Call(uintptr(t.out), uintptr(mode|enableProcessedOutput|enableVirtualTerminalProcessing))
	}

	// 输入模式先记录原始值，raw 切换在 EnterRaw 里做
	if r, _, _ := procGetConsoleMode.Call(uintptr(t.in), uintptr(unsafe.Pointer(&mode))); r != 0 {
		t.oldIn = mode
	}
	return t, nil
}

// Size 返回控制台可见窗口的列数与行数。
func (t *Terminal) Size() (int, int) {
	var info consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(uintptr(t.out), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 120, 30
	}
	w := int(info.Window.Right-info.Window.Left) + 1
	h := int(info.Window.Bottom-info.Window.Top) + 1
	if w < 20 {
		w = 80
	}
	if h < 8 {
		h = 24
	}
	return w, h
}

// EnterRaw 关闭行缓冲/回显，开启 VT 输入，隐藏光标。
func (t *Terminal) EnterRaw() error {
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(t.in), uintptr(unsafe.Pointer(&mode))); r != 0 {
		t.oldIn = mode
		next := mode &^ (enableEchoInput | enableLineInput | enableProcessedInput)
		next |= enableVirtualTerminalInput
		procSetConsoleMode.Call(uintptr(t.in), uintptr(next))
	}
	t.hideCursor(true)
	t.raw = true
	return nil
}

// Restore 还原控制台状态（退出前务必调用）。
func (t *Terminal) Restore() {
	t.hideCursor(false)
	if t.oldIn != 0 {
		procSetConsoleMode.Call(uintptr(t.in), uintptr(t.oldIn))
	}
	if t.oldOut != 0 {
		procSetConsoleMode.Call(uintptr(t.out), uintptr(t.oldOut))
	}
	t.raw = false
}

func (t *Terminal) hideCursor(hide bool) {
	// Size 用 25（百分比，标准方块光标）；这里只关心可见性。
	var info consoleCursorInfo
	info.Size = 25
	if hide {
		info.Visible = 0
	} else {
		info.Visible = 1
	}
	procSetConsoleCursorInfo.Call(uintptr(t.out), uintptr(unsafe.Pointer(&info)))
}

// Read 从标准输入读取按键字节（raw 模式下无缓冲）。
func (t *Terminal) Read(buf []byte) (int, error) {
	return os.Stdin.Read(buf)
}

// WriteString 向控制台输出一段已经拼好的帧。
//
// 优先走 WriteConsoleW（宽字符）：直接写 UTF-8 字节流在部分旧版 conhost 上
// 会因为代码页换算而出现中文/块字符乱码。输出被重定向到文件时该调用会失败，
// 此时退回普通字节写入。
func (t *Terminal) WriteString(s string) error {
	if u, err := syscall.UTF16FromString(s); err == nil && len(u) > 0 {
		var written uint32
		r, _, _ := procWriteConsoleW.Call(
			uintptr(t.out),
			uintptr(unsafe.Pointer(&u[0])),
			uintptr(len(u)-1), // 不含结尾 NUL
			uintptr(unsafe.Pointer(&written)),
			0,
		)
		if r != 0 {
			return nil
		}
	}
	_, err := os.Stdout.WriteString(s)
	return err
}
