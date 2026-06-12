//go:build windows

package screenshotsvc

// ABI 敏感结构体的布局断言（同 mcdkbridge/win32_windows_test.go 范式）：
// 结构体经 unsafe.Pointer 直接传给 Win32，字段错位即静默数据损坏。

import (
	"testing"
	"unsafe"
)

func TestWin32StructSizes(t *testing.T) {
	if got := unsafe.Sizeof(msgStruct{}); got != 48 {
		t.Errorf("MSG 应为 48 字节，实际 %d", got)
	}
	if got := unsafe.Sizeof(wndClassExW{}); got != 80 {
		t.Errorf("WNDCLASSEXW 应为 80 字节，实际 %d", got)
	}
	if got := unsafe.Sizeof(paintStruct{}); got != 72 {
		t.Errorf("PAINTSTRUCT 应为 72 字节，实际 %d", got)
	}
	if got := unsafe.Sizeof(monitorInfo{}); got != 40 {
		t.Errorf("MONITORINFO 应为 40 字节，实际 %d", got)
	}
	if got := unsafe.Sizeof(bitmapInfoHeader{}); got != 40 {
		t.Errorf("BITMAPINFOHEADER 应为 40 字节，实际 %d", got)
	}
}

func TestBlendFunctionArg(t *testing.T) {
	// BLENDFUNCTION{AC_SRC_OVER, 0, 102, 0} 小端打包：alpha 在 byte2。
	if got := blendFunctionArg(0, 0, 102, 0); got != 0x00660000 {
		t.Errorf("blendFunctionArg = %#x，期望 0x00660000", got)
	}
}
