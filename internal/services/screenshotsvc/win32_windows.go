//go:build windows

package screenshotsvc

// win32_windows.go — 截图链路共享的 Win32 FFI 声明（热键 / 显示器 / GDI / 遮罩窗口）。
// 沿用 mcdkbridge/window_windows.go 的 LazyDLL.NewProc 手写范式；ABI 敏感结构体在
// win32_windows_test.go 用 unsafe.Sizeof 断言。

import (
	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	// 热键 + 消息循环
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procPeekMessageW      = user32.NewProc("PeekMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")

	// 显示器 / 光标
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procMonitorFromPoint = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW  = user32.NewProc("GetMonitorInfoW")

	// 窗口（选区遮罩）
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procSetCursor           = user32.NewProc("SetCursor")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procFillRect            = user32.NewProc("FillRect")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procSetThreadDpi        = user32.NewProc("SetThreadDpiAwarenessContext")

	// GDI
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procGdiAlphaBlend          = gdi32.NewProc("GdiAlphaBlend")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procRectangle              = gdi32.NewProc("Rectangle")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procTextOutW               = gdi32.NewProc("TextOutW")
	procGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	procCreateFontW            = gdi32.NewProc("CreateFontW")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// ── 常量 ──────────────────────────────────────────────────────────────

const (
	// 消息
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmQuit        = 0x0012
	wmEraseBkgnd  = 0x0014
	wmSetCursor   = 0x0020
	wmPaint       = 0x000F
	wmKeyDown     = 0x0100
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmHotkey      = 0x0312
	wmApp         = 0x8000
	// 自定义线程消息：热键改键（lParam/wParam 不用，参数走 hotkeyLoop.pending*）
	wmAppReregister = wmApp + 1

	pmNoRemove = 0
	pmRemove   = 1

	vkEscape = 0x1B

	// 窗口样式
	wsPopup        = 0x8000_0000
	wsVisible      = 0x1000_0000
	wsExTopmost    = 0x0000_0008
	wsExToolWindow = 0x0000_0080
	csHRedraw      = 0x0002
	csVRedraw      = 0x0001
	swShow         = 5
	idcCross       = 32515

	// GDI
	biRGB             = 0
	dibRGBColors      = 0
	srccopy           = 0x00CC_0020
	captureBlt        = 0x4000_0000
	psSolid           = 0
	nullBrush         = 5
	bkTransparent     = 1
	fwNormal          = 400
	defaultCharset    = 1
	outDefaultPrecis  = 0
	clipDefaultPrecis = 0
	cleartypeQuality  = 5
	ffSwiss           = 0x20
	variablePitch     = 2
	acSrcOver         = 0

	monitorDefaultToNearest = 2

	hotkeyID = 1 // RegisterHotKey 的应用内热键 id（仅此一个热键）
)

// dpiAwarenessPerMonitorV2 = DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (HANDLE)-4。
const dpiAwarenessPerMonitorV2 = ^uintptr(3)

// ── 结构体（ABI 敏感）──────────────────────────────────────────────────

type point struct{ x, y int32 }
type rect struct{ left, top, right, bottom int32 }
type size struct{ cx, cy int32 }

// msgStruct == MSG（x64 = 48 字节：hwnd(8) message(4)+pad(4) wParam(8) lParam(8) time(4) pt(8)+pad(4)）。
type msgStruct struct {
	hwnd    uintptr
	message uint32
	_       uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	_       uint32
}

// wndClassExW == WNDCLASSEXW（x64 = 80 字节）。
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// paintStruct == PAINTSTRUCT（x64 = 72 字节）。
type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

// monitorInfo == MONITORINFO（40 字节）。
type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

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

// bitmapInfo == BITMAPINFO（header + 1 个 RGBQUAD 占位）。
type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

// blendFunctionArg 把 BLENDFUNCTION{BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat}
// 按值打包成单个 DWORD 参数（x64 ABI：4 字节结构体按值入寄存器，小端序）。
func blendFunctionArg(blendOp, blendFlags, srcAlpha, alphaFormat byte) uintptr {
	return uintptr(uint32(blendOp) | uint32(blendFlags)<<8 | uint32(srcAlpha)<<16 | uint32(alphaFormat)<<24)
}

// setThreadDpi 切线程 DPI 上下文，返回原 context（0 = 旧系统不支持，恢复为 no-op）。
func setThreadDpi(ctx uintptr) uintptr {
	prev, _, _ := procSetThreadDpi.Call(ctx)
	return prev
}

func restoreThreadDpi(prev uintptr) {
	if prev != 0 {
		procSetThreadDpi.Call(prev)
	}
}

func lastErr() uint32 {
	if e, ok := windows.GetLastError().(windows.Errno); ok {
		return uint32(e)
	}
	return 0
}
