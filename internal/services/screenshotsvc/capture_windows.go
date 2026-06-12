//go:build windows

package screenshotsvc

// capture_windows.go — 截取光标所在显示器全屏到内存（对齐 cc-pane capture_to_memory）：
// GetCursorPos → MonitorFromPoint → 屏幕 DC BitBlt（SRCCOPY|CAPTUREBLT 含分层窗口）→
// GetDIBits BGRA → RGBA image。调用方线程须已切 Per-Monitor V2 DPI（坐标全物理像素）。

import (
	"fmt"
	"image"
	"unsafe"
)

// monitorRect 是显示器在虚拟屏幕中的物理像素矩形。
type monitorRect struct {
	x, y, w, h int32
}

// captureCursorMonitor 截取鼠标当前所在显示器，返回 RGBA 图像与该显示器矩形。
func captureCursorMonitor() (*image.RGBA, monitorRect, error) {
	var zero monitorRect

	var pt point
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return nil, zero, fmt.Errorf("GetCursorPos failed (GetLastError=%d)", lastErr())
	}
	// POINT（8 字节）按值传参：x 低 32 位、y 高 32 位（x64 小端）。
	ptArg := uintptr(uint64(uint32(pt.x)) | uint64(uint32(pt.y))<<32)
	hmon, _, _ := procMonitorFromPoint.Call(ptArg, monitorDefaultToNearest)
	if hmon == 0 {
		return nil, zero, fmt.Errorf("MonitorFromPoint failed")
	}
	mi := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return nil, zero, fmt.Errorf("GetMonitorInfoW failed (GetLastError=%d)", lastErr())
	}
	mon := monitorRect{
		x: mi.rcMonitor.left,
		y: mi.rcMonitor.top,
		w: mi.rcMonitor.right - mi.rcMonitor.left,
		h: mi.rcMonitor.bottom - mi.rcMonitor.top,
	}
	if mon.w <= 0 || mon.h <= 0 {
		return nil, zero, fmt.Errorf("invalid monitor size %dx%d", mon.w, mon.h)
	}

	bgra, err := captureScreenRegionBGRA(mon)
	if err != nil {
		return nil, zero, err
	}
	// BGRA → RGBA：原地交换 B/R 通道（同 mcdkbridge encodeJPEG）。
	for i := 0; i+3 < len(bgra); i += 4 {
		bgra[i], bgra[i+2] = bgra[i+2], bgra[i]
		bgra[i+3] = 255
	}
	img := &image.RGBA{Pix: bgra, Stride: int(mon.w) * 4, Rect: image.Rect(0, 0, int(mon.w), int(mon.h))}
	return img, mon, nil
}

// captureScreenRegionBGRA 从屏幕 DC BitBlt 指定区域，返回自顶向下 BGRA。
// defer LIFO 还原序：SelectObject → DeleteObject(bmp) → DeleteDC → ReleaseDC(screen)。
func captureScreenRegionBGRA(mon monitorRect) ([]byte, error) {
	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC(NULL) failed (GetLastError=%d)", lastErr())
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed (GetLastError=%d)", lastErr())
	}
	defer procDeleteDC.Call(hdcMem)

	bmp, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(mon.w), uintptr(mon.h))
	if bmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed (GetLastError=%d)", lastErr())
	}
	defer procDeleteObject.Call(bmp)

	oldObj, _, _ := procSelectObject.Call(hdcMem, bmp)
	defer func() {
		if oldObj != 0 {
			procSelectObject.Call(hdcMem, oldObj)
		}
	}()

	if r, _, _ := procBitBlt.Call(
		hdcMem, 0, 0, uintptr(mon.w), uintptr(mon.h),
		hdcScreen, uintptr(int(mon.x)), uintptr(int(mon.y)), // 负坐标（主屏左侧显示器）按补码传参
		srccopy|captureBlt,
	); r == 0 {
		return nil, fmt.Errorf("BitBlt screen capture failed (GetLastError=%d)", lastErr())
	}

	buffer := make([]byte, int(mon.w)*4*int(mon.h))
	bi := bitmapInfo{header: bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       mon.w,
		biHeight:      -mon.h, // 负高 → 自顶向下
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	scanLines, _, _ := procGetDIBits.Call(
		hdcMem, bmp, 0, uintptr(mon.h),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if scanLines == 0 {
		return nil, fmt.Errorf("GetDIBits returned 0 (GetLastError=%d)", lastErr())
	}
	return buffer, nil
}
