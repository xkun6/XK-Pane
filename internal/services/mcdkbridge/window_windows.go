//go:build windows

package mcdkbridge

// window_windows.go — MC PE 渲染窗口定位 + capture/click 共享的 Win32 FFI 声明。
// 忠实移植 cc-pane mcdk_bridge/window.rs；GDI/User32 多数符号无 x/sys/windows 现成 wrapper，
// 用 LazyDLL.NewProc 手写（见 P6e_PLAN.md §5）。结构体布局对 SendInput/GetDIBits 敏感，
// 在 win32_windows_test.go 用 unsafe.Sizeof 断言（INPUT=40 / MOUSEINPUT=32 / BITMAPINFOHEADER=40）。

import (
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ── DLL & procs ───────────────────────────────────────────────────────────

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procGetWindow                = user32.NewProc("GetWindow")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetDC                    = user32.NewProc("GetDC")
	procReleaseDC                = user32.NewProc("ReleaseDC")
	procPrintWindow              = user32.NewProc("PrintWindow")
	procSetThreadDpiAwareness    = user32.NewProc("SetThreadDpiAwarenessContext")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procClientToScreen           = user32.NewProc("ClientToScreen")
	procSendInput                = user32.NewProc("SendInput")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

// ── 常量（命门，逐字照搬 cc-pane）────────────────────────────────────────

const (
	gwOwner   = 4
	swRestore = 9

	pwRenderFullContent = 0x0000_0002
	biRGB               = 0
	dibRGBColors        = 0
	srccopy             = 0x00CC_0020

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	mouseeventfMove        = 0x0001
	mouseeventfLeftDown    = 0x0002
	mouseeventfLeftUp      = 0x0004
	mouseeventfAbsolute    = 0x8000
	mouseeventfVirtualDesk = 0x4000
	inputMouse             = 0

	maxDimension = 480
	jpegQuality  = 75
)

// dpiAwarenessPerMonitorV2 = DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2，定义为 (HANDLE)-4。
// ^uintptr(3) == ...FFFC == -4 的无符号表示。
const dpiAwarenessPerMonitorV2 = ^uintptr(3)

// ── 结构体（ABI 敏感）──────────────────────────────────────────────────

type rect struct{ left, top, right, bottom int32 }
type point struct{ x, y int32 }

// bitmapInfoHeader == BITMAPINFOHEADER（40 字节）。
type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

// bitmapInfo == BITMAPINFO（header + 1 个 RGBQUAD 占位；32bpp BI_RGB 不用色表，但需合法指针）。
type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

// mouseInput == MOUSEINPUT（x64 = 32 字节：time 后 4 字节 pad 对齐 8 字节的 dwExtraInfo）。
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// inputStruct == INPUT（x64 = 40 字节：type(4) + pad(4) + MOUSEINPUT(32)）。
type inputStruct struct {
	inputType uint32
	_         uint32 // 对齐 union 到 8 字节边界
	mi        mouseInput
}

// ── DPI 辅助 ──────────────────────────────────────────────────────────

// setThreadDpi 切线程 DPI 上下文，返回原 context（传回即可恢复；旧系统不支持时返回 0，恢复为 no-op）。
func setThreadDpi(ctx uintptr) uintptr {
	prev, _, _ := procSetThreadDpiAwareness.Call(ctx)
	return prev
}

// restoreThreadDpi 恢复原 DPI 上下文（prev==0 时跳过，对齐 cc-pane Drop guard 的 null 判断）。
func restoreThreadDpi(prev uintptr) {
	if prev != 0 {
		procSetThreadDpiAwareness.Call(prev)
	}
}

func lastErr() uint32 { return uint32(windows.GetLastError().(windows.Errno)) }

// ── 窗口定位（EnumWindows 串行化）──────────────────────────────────────

type windowCandidate struct {
	hwnd  uintptr
	title string
	area  int64
}

// enum 状态用包级变量 + 锁串行化（EnumWindows 同步执行，回调在本调用内同步触发）；
// 回调只创建一次（windows.NewCallback 创建的回调不可回收，避免每次 capture 泄漏）。
var (
	enumMu       sync.Mutex
	enumState    *windowFinder
	enumCallback = windows.NewCallback(enumWindowsProc)
)

type windowFinder struct {
	targetPID  uint32
	candidates []windowCandidate
}

func enumWindowsProc(hwnd uintptr, _ uintptr) uintptr {
	f := enumState
	if f == nil {
		return 1
	}
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	// 有 owner（owned popup）→ 跳过。
	if owner, _, _ := procGetWindow.Call(hwnd, gwOwner); owner != 0 {
		return 1
	}
	var ownerPID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&ownerPID)))
	if ownerPID != f.targetPID {
		return 1
	}
	var cr rect
	if r, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr))); r == 0 {
		return 1
	}
	area := int64(cr.right-cr.left) * int64(cr.bottom-cr.top)
	if area <= 0 {
		return 1
	}
	f.candidates = append(f.candidates, windowCandidate{hwnd: hwnd, title: windowTitle(hwnd), area: area})
	return 1
}

func windowTitle(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if int32(n) <= 0 {
		return ""
	}
	buf := make([]uint16, int(n)+1)
	copied, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if int32(copied) <= 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:int(copied)])
}

// findMCWindowForPID 在 pid 的顶层窗口里挑 D3D 渲染窗口：先按标题（含 minecraft / 我的世界），
// 否则取客户区面积最大者。忠实 cc-pane window.rs。
func findMCWindowForPID(pid uint32) (uintptr, bool) {
	enumMu.Lock()
	defer enumMu.Unlock()

	f := &windowFinder{targetPID: pid}
	enumState = f
	procEnumWindows.Call(enumCallback, 0)
	enumState = nil

	cands := f.candidates
	if len(cands) == 0 {
		return 0, false
	}
	for _, c := range cands {
		if strings.Contains(strings.ToLower(c.title), "minecraft") || strings.Contains(c.title, "我的世界") {
			return c.hwnd, true
		}
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.area > best.area {
			best = c
		}
	}
	return best.hwnd, true
}
