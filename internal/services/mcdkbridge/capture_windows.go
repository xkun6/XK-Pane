//go:build windows

package mcdkbridge

// capture_windows.go — 截图：定位窗口 → PrintWindow / 窗口 DC BitBlt 降级 → GetDIBits BGRA
// → BGRA→RGBA → 最长边 480 缩放 → JPEG q75。忠实移植 cc-pane capture.rs。
//
// 返回原始 JPEG 字节（非 base64）：orchestrator 工具包成 mcp.ImageContent{Data: bytes}，
// go-sdk 的 json.Marshal 对 []byte 自动 base64（见 content.go imageAudioWire）。
//
// 显式偏离：缩放滤波 Lanczos3 → golang.org/x/image/draw.CatmullRom（Go 无原生 Lanczos3，
// 画质略降，docs/archive/P6e_PLAN.md §2 已拍板）。

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"unsafe"

	xdraw "golang.org/x/image/draw"
)

// CaptureWindowByPID 截取 pid 进程的 MC 渲染窗口，返回 JPEG 字节。
func CaptureWindowByPID(pid uint32) ([]byte, error) {
	hwnd, ok := findMCWindowForPID(pid)
	if !ok {
		return nil, coded(CodeCaptureNoWindow, fmt.Sprintf("no Minecraft-like top-level window found for pid %d", pid))
	}

	// 临时切线程 DPI 上下文，确保 GetWindowRect 返回物理像素。
	prev := setThreadDpi(dpiAwarenessPerMonitorV2)
	defer restoreThreadDpi(prev)

	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("window for pid %d is minimized", pid))
	}

	var wr rect
	if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); r == 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("GetWindowRect failed (GetLastError=%d)", lastErr()))
	}
	width := wr.right - wr.left
	height := wr.bottom - wr.top
	if width <= 0 || height <= 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("invalid window size %dx%d", width, height))
	}

	bgra, err := captureRawBGRA(hwnd, width, height)
	if err != nil {
		return nil, err
	}
	return encodeJPEG(bgra, width, height)
}

// captureRawBGRA 抓窗口像素为 32-bit BGRA（自顶向下）。defer LIFO 对齐 cc-pane RAII drop 序：
// 还原 SelectObject → DeleteObject(bmp) → DeleteDC → ReleaseDC(screen)。
func captureRawBGRA(hwnd uintptr, width, height int32) ([]byte, error) {
	// 桌面 DC：兼容位图基于它，PrintWindow 才能正确处理 D3D11/DXGI 合成窗口（窗口 DC 兼容位图会全白）。
	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("GetDC(NULL) failed (GetLastError=%d)", lastErr()))
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcCapture, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcCapture == 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("CreateCompatibleDC failed (GetLastError=%d)", lastErr()))
	}
	defer procDeleteDC.Call(hdcCapture)

	bmp, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(width), uintptr(height))
	if bmp == 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("CreateCompatibleBitmap failed (GetLastError=%d)", lastErr()))
	}
	defer procDeleteObject.Call(bmp)

	oldObj, _, _ := procSelectObject.Call(hdcCapture, bmp)
	defer func() {
		if oldObj != 0 {
			procSelectObject.Call(hdcCapture, oldObj)
		}
	}()

	// 先试 PrintWindow（PW_RENDERFULLCONTENT，被遮挡也能截）。
	printOK, _, _ := procPrintWindow.Call(hwnd, hdcCapture, pwRenderFullContent)
	if printOK == 0 {
		// 降级：窗口自己的 DC 直接 BitBlt（源 (0,0)，不受遮挡影响）。
		hdcWindow, _, _ := procGetDC.Call(hwnd)
		if hdcWindow == 0 {
			return nil, coded(CodeCaptureFailed, fmt.Sprintf("PrintWindow returned FALSE and GetDC(hwnd) failed (GetLastError=%d)", lastErr()))
		}
		defer procReleaseDC.Call(hwnd, hdcWindow)
		if r, _, _ := procBitBlt.Call(hdcCapture, 0, 0, uintptr(width), uintptr(height), hdcWindow, 0, 0, srccopy); r == 0 {
			return nil, coded(CodeCaptureFailed, fmt.Sprintf("PrintWindow returned FALSE and window-DC BitBlt failed (GetLastError=%d)", lastErr()))
		}
	}

	// GetDIBits 取 BGRA：biHeight 取负 → 自顶向下。
	buffer := make([]byte, int(width)*4*int(height))
	bi := bitmapInfo{header: bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       width,
		biHeight:      -height,
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	scanLines, _, _ := procGetDIBits.Call(
		hdcCapture, bmp, 0, uintptr(height),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if scanLines == 0 {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("GetDIBits returned 0 (GetLastError=%d)", lastErr()))
	}
	return buffer, nil
}

// encodeJPEG 把 BGRA buffer（自顶向下）转 RGBA、按需缩放、编码 JPEG q75。
func encodeJPEG(bgra []byte, width, height int32) ([]byte, error) {
	// BGRA → RGBA：原地交换 B 和 R 通道。
	for i := 0; i+3 < len(bgra); i += 4 {
		bgra[i], bgra[i+2] = bgra[i+2], bgra[i]
	}

	img := &image.RGBA{Pix: bgra, Stride: int(width) * 4, Rect: image.Rect(0, 0, int(width), int(height))}

	var src image.Image = img
	if nw, nh, need := scaledDimensions(int(width), int(height), maxDimension); need {
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
		src = dst
	}

	// jpeg.Encode 对 RGBA 自动丢 alpha（编码为 RGB/YCbCr）。
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, coded(CodeCaptureFailed, fmt.Sprintf("JPEG encode failed: %v", err))
	}
	return out.Bytes(), nil
}
