//go:build windows

package mcdkbridge

import (
	"errors"
	"testing"
	"unsafe"
)

// Win32 struct 布局对 SendInput/GetDIBits 敏感，size 错了静默失败。锁死 ABI（x64）。
func TestWin32StructSizes(t *testing.T) {
	if got := unsafe.Sizeof(mouseInput{}); got != 32 {
		t.Errorf("sizeof(MOUSEINPUT) = %d, want 32", got)
	}
	if got := unsafe.Sizeof(inputStruct{}); got != 40 {
		t.Errorf("sizeof(INPUT) = %d, want 40", got)
	}
	if got := unsafe.Sizeof(bitmapInfoHeader{}); got != 40 {
		t.Errorf("sizeof(BITMAPINFOHEADER) = %d, want 40", got)
	}
}

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (HANDLE)-4。
func TestDpiContextValue(t *testing.T) {
	neg := int64(-4)     // 经变量绕开常量转换检查
	want := uintptr(neg) // 十进制 -4 → uintptr（独立于定义的 ^uintptr(3)）
	if dpiAwarenessPerMonitorV2 != want {
		t.Errorf("dpiAwarenessPerMonitorV2 = %#x, want %#x (-4)", dpiAwarenessPerMonitorV2, want)
	}
}

// 无效 pid 不会匹配到任何窗口：capture → NO_WINDOW，click → NO_WINDOW（不真截图/点击，安全）。
func TestCaptureClickNoWindow(t *testing.T) {
	const bogusPID = 999_999

	_, err := CaptureWindowByPID(bogusPID)
	if !hasCode(err, CodeCaptureNoWindow) {
		t.Errorf("CaptureWindowByPID(bogus) err = %v, want code %s", err, CodeCaptureNoWindow)
	}

	if err := ClickWindowByPID(bogusPID, 0.5, 0.5); !hasCode(err, CodeClickNoWindow) {
		t.Errorf("ClickWindowByPID(bogus, 0.5, 0.5) err = %v, want code %s", err, CodeClickNoWindow)
	}
}

// 越界坐标在找窗口之前就被拒（不触任何 Win32）。
func TestClickOutOfBounds(t *testing.T) {
	for _, c := range []struct{ x, y float32 }{{1.5, 0.5}, {0.5, -0.1}, {-1, 0}, {0, 2}} {
		if err := ClickWindowByPID(0, c.x, c.y); !hasCode(err, CodeClickOutOfBounds) {
			t.Errorf("ClickWindowByPID(0,%v,%v) err = %v, want code %s", c.x, c.y, err, CodeClickOutOfBounds)
		}
	}
}

func hasCode(err error, code string) bool {
	var ce *CodedError
	return errors.As(err, &ce) && ce.Code == code
}
