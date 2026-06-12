//go:build windows

package screenshotsvc

// overlay_windows.go — Win32 原生全屏选区窗口（逐条对译 cc-pane screenshot_overlay.rs）。
// GDI 绘制：截图背景 + 选区外 40% 黑遮罩（AlphaBlend）+ 2px 蓝框 + "宽 x 高"尺寸标签，
// 双缓冲防闪烁。交互：左键按下拖拽选区、松开即确认、ESC 取消。完全不涉及 WebView。
//
// 调用方约定：在已 LockOSThread 且已切 Per-Monitor V2 DPI 的线程上阻塞调用；
// overlayActive CAS 防重入，状态经包级指针共享给 WndProc（单线程消息循环内访问，无并发）。

import (
	"fmt"
	"image"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

type overlayState struct {
	bgDC    uintptr
	bgBmp   uintptr
	maskDC  uintptr
	maskBmp uintptr
	winW    int32
	winH    int32

	dragging       bool
	startX, startY int32
	endX, endY     int32
	hasSelection   bool
	confirmed      bool
}

var (
	overlayActive atomic.Bool
	// WndProc 共享状态：仅在遮罩消息循环线程内读写（overlayActive 串行化整个流程）。
	overlayCur *overlayState
	// 回调只创建一次（windows.NewCallback 不可回收，避免每次截图泄漏，同 mcdkbridge enumCallback）。
	overlayWndProc = windows.NewCallback(overlayWndProcGo)
)

// showSelectionOverlay 显示全屏选区窗口，阻塞直到用户选完或取消。
// 返回相对截图图像的物理像素矩形；取消 / 选区过小（≤2px）返回 ok=false。
func showSelectionOverlay(img *image.RGBA, mon monitorRect) (sel image.Rectangle, ok bool) {
	if !overlayActive.CompareAndSwap(false, true) {
		return image.Rectangle{}, false
	}
	defer overlayActive.Store(false)
	return runOverlay(img, mon)
}

func runOverlay(img *image.RGBA, mon monitorRect) (image.Rectangle, bool) {
	imgW := int32(img.Bounds().Dx())
	imgH := int32(img.Bounds().Dy())
	winW, winH := mon.w, mon.h

	hinstance, _, _ := procGetModuleHandleW.Call(0)
	className := windows.StringToUTF16Ptr("XKPaneScreenshotOverlay")

	cursor, _, _ := procLoadCursorW.Call(0, idcCross)
	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   overlayWndProc,
		hInstance:     hinstance,
		hCursor:       cursor,
		lpszClassName: className,
	}
	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		logFor().Error("选区窗口类注册失败", "lastErr", lastErr())
		return image.Rectangle{}, false
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hinstance)

	hdcScreen, _, _ := procGetDC.Call(0)

	// 背景位图：截图 RGBA → BGRA DIBSection（窗口尺寸 == 图像尺寸，物理像素一致，直接逐像素拷）。
	bgDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	bgBmp, bgBits := createDIBSection(hdcScreen, winW, winH)
	if bgBmp == 0 || bgBits == nil {
		procDeleteDC.Call(bgDC)
		procReleaseDC.Call(0, hdcScreen)
		logFor().Error("选区背景位图创建失败", "lastErr", lastErr())
		return image.Rectangle{}, false
	}
	procSelectObject.Call(bgDC, bgBmp)
	copyRGBAToBGRA(bgBits, img, winW, winH, imgW, imgH)

	// 遮罩位图：全黑 DIBSection（CreateDIBSection 初始化全零），AlphaBlend 40% 用。
	maskDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	maskBmp, _ := createDIBSection(hdcScreen, winW, winH)
	procSelectObject.Call(maskDC, maskBmp)

	procReleaseDC.Call(0, hdcScreen)

	st := &overlayState{
		bgDC: bgDC, bgBmp: bgBmp, maskDC: maskDC, maskBmp: maskBmp,
		winW: winW, winH: winH,
	}
	overlayCur = st
	defer func() {
		overlayCur = nil
		procDeleteDC.Call(st.maskDC)
		procDeleteObject.Call(st.maskBmp)
		procDeleteDC.Call(st.bgDC)
		procDeleteObject.Call(st.bgBmp)
	}()

	hwnd, _, _ := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("XK-Pane Screenshot Overlay"))),
		wsPopup|wsVisible,
		uintptr(int(mon.x)), uintptr(int(mon.y)), uintptr(winW), uintptr(winH),
		0, 0, hinstance, 0,
	)
	if hwnd == 0 {
		logFor().Error("选区窗口创建失败", "lastErr", lastErr())
		return image.Rectangle{}, false
	}

	procShowWindow.Call(hwnd, swShow)
	procSetForegroundWindow.Call(hwnd)

	var m msgStruct
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	procDestroyWindow.Call(hwnd)
	// 清残留 WM_QUIT：DestroyWindow 触发 wmDestroy 分支会再次 PostQuitMessage；本线程
	// UnlockOSThread 后归还调度池，残留的 WM_QUIT 会污染未来跑在该线程上的消息循环。
	for {
		if r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, wmQuit, wmQuit, pmRemove); r == 0 {
			break
		}
	}

	if !st.confirmed || !st.hasSelection {
		return image.Rectangle{}, false
	}
	x1, y1, x2, y2 := normalizeRect(st.startX, st.startY, st.endX, st.endY)
	if x2-x1 <= 2 || y2-y1 <= 2 {
		return image.Rectangle{}, false
	}
	// 窗口坐标 → 图像坐标（窗口与图像同为该显示器物理像素，比例恒为 1；保留映射兜底）。
	scaleX := float64(imgW) / float64(winW)
	scaleY := float64(imgH) / float64(winH)
	r := image.Rect(
		int(float64(x1)*scaleX), int(float64(y1)*scaleY),
		int(float64(x2)*scaleX), int(float64(y2)*scaleY),
	).Intersect(img.Bounds())
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return image.Rectangle{}, false
	}
	return r, true
}

// createDIBSection 创建 32bpp 自顶向下 DIBSection，返回句柄与像素切片（BGRA）。
func createDIBSection(hdc uintptr, w, h int32) (uintptr, []byte) {
	bi := bitmapInfo{header: bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       w,
		biHeight:      -h,
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bi)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return 0, nil
	}
	return bmp, unsafe.Slice((*byte)(bits), int(w)*4*int(h))
}

// copyRGBAToBGRA 把截图像素填进背景 DIBSection（尺寸不一致时按 min 边界拷，不缩放——
// 实际窗口/图像同源同尺寸，此分支仅为防御）。
func copyRGBAToBGRA(dst []byte, img *image.RGBA, dstW, dstH, srcW, srcH int32) {
	w := min(dstW, srcW)
	h := min(dstH, srcH)
	for y := int32(0); y < h; y++ {
		srcRow := img.Pix[int(y)*img.Stride:]
		dstRow := dst[int(y)*int(dstW)*4:]
		for x := int32(0); x < w; x++ {
			si := int(x) * 4
			dstRow[si] = srcRow[si+2]   // B
			dstRow[si+1] = srcRow[si+1] // G
			dstRow[si+2] = srcRow[si]   // R
			dstRow[si+3] = 255          // A
		}
	}
}

func normalizeRect(x1, y1, x2, y2 int32) (int32, int32, int32, int32) {
	return min(x1, x2), min(y1, y2), max(x1, x2), max(y1, y2)
}

// overlayWndProcGo 窗口过程（运行于遮罩消息循环线程）。
func overlayWndProcGo(hwnd, msg, wparam, lparam uintptr) uintptr {
	st := overlayCur
	switch msg {
	case wmPaint:
		if st != nil {
			overlayPaint(hwnd, st)
		}
		return 0
	case wmLButtonDown:
		if st != nil {
			x, y := lparamXY(lparam)
			st.dragging = true
			st.startX, st.startY = x, y
			st.endX, st.endY = x, y
			st.hasSelection = false
			procSetCapture.Call(hwnd)
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case wmMouseMove:
		if st != nil && st.dragging {
			x, y := lparamXY(lparam)
			st.endX, st.endY = x, y
			st.hasSelection = true
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case wmLButtonUp:
		if st != nil && st.dragging {
			x, y := lparamXY(lparam)
			st.endX, st.endY = x, y
			st.dragging = false
			st.hasSelection = true
			st.confirmed = true
			procReleaseCapture.Call()
			procPostQuitMessage.Call(0)
		}
		return 0
	case wmKeyDown:
		if wparam == vkEscape {
			if st != nil {
				st.confirmed = false
			}
			procPostQuitMessage.Call(0)
		}
		return 0
	case wmClose, wmDestroy:
		// 窗口被外部关闭/销毁（如 Alt+F4）也必须退出消息循环，否则 GetMessageW 永久阻塞：
		// capturing 卡 true（热键失效）、protect(false) 不恢复（主窗口被永久排除捕获）、线程泄漏。
		// 视为取消（confirmed 保持 false）。wmClose 直接退出，不让 DefWindowProc 先销毁窗口。
		procPostQuitMessage.Call(0)
		return 0
	case wmSetCursor:
		cursor, _, _ := procLoadCursorW.Call(0, idcCross)
		procSetCursor.Call(cursor)
		return 1
	case wmEraseBkgnd:
		return 1 // 阻止背景擦除（防闪烁）
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

func lparamXY(lparam uintptr) (int32, int32) {
	return int32(int16(lparam & 0xFFFF)), int32(int16((lparam >> 16) & 0xFFFF))
}

// overlayPaint 双缓冲绘制：背景截图 + 半透明遮罩 + 选区边框 + 尺寸标签。
func overlayPaint(hwnd uintptr, st *overlayState) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	w, h := st.winW, st.winH

	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	memBmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	oldBmp, _, _ := procSelectObject.Call(memDC, memBmp)
	defer func() {
		procSelectObject.Call(memDC, oldBmp)
		procDeleteObject.Call(memBmp)
		procDeleteDC.Call(memDC)
	}()

	// 1. 背景截图
	procBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), st.bgDC, 0, 0, srccopy)

	if st.hasSelection {
		x1, y1, x2, y2 := normalizeRect(st.startX, st.startY, st.endX, st.endY)

		// 2. 选区外四块半透明遮罩
		drawMaskRegion(memDC, st.maskDC, 0, 0, w, y1)   // 上
		drawMaskRegion(memDC, st.maskDC, 0, y1, x1, y2) // 左
		drawMaskRegion(memDC, st.maskDC, x2, y1, w, y2) // 右
		drawMaskRegion(memDC, st.maskDC, 0, y2, w, h)   // 下

		// 3. 选区边框（2px #4fc3f7，COLORREF 为 BGR）
		pen, _, _ := procCreatePen.Call(psSolid, 2, 0x00f7c34f)
		oldPen, _, _ := procSelectObject.Call(memDC, pen)
		brush, _, _ := procGetStockObject.Call(nullBrush)
		oldBrush, _, _ := procSelectObject.Call(memDC, brush)
		procRectangle.Call(memDC, uintptr(int(x1)), uintptr(int(y1)), uintptr(int(x2)), uintptr(int(y2)))
		procSelectObject.Call(memDC, oldPen)
		procSelectObject.Call(memDC, oldBrush)
		procDeleteObject.Call(pen)

		// 4. 尺寸标签
		drawSizeLabel(memDC, x1, y1, x2-x1, y2-y1)
	} else {
		drawMaskRegion(memDC, st.maskDC, 0, 0, w, h) // 无选区：全屏遮罩
	}

	// 一次性拷贝到屏幕
	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, srccopy)
}

// drawMaskRegion 用缓存的全黑 mask_dc AlphaBlend 出 40% 黑遮罩（102 = 255*0.4）。
func drawMaskRegion(hdc, maskDC uintptr, x1, y1, x2, y2 int32) {
	if x2 <= x1 || y2 <= y1 {
		return
	}
	rw, rh := x2-x1, y2-y1
	procGdiAlphaBlend.Call(
		hdc, uintptr(int(x1)), uintptr(int(y1)), uintptr(rw), uintptr(rh),
		maskDC, 0, 0, uintptr(rw), uintptr(rh),
		blendFunctionArg(acSrcOver, 0, 102, 0),
	)
}

// drawSizeLabel 在选区上方绘制 "宽 x 高" 标签（深底白字）。
func drawSizeLabel(hdc uintptr, x, y, w, h int32) {
	text := fmt.Sprintf("%d x %d", w, h)
	wide := windows.StringToUTF16(text) // 含 NUL 终止符
	chars := len(wide) - 1

	labelY := y - 24
	if labelY < 0 {
		labelY = y + 4
	}

	font, _, _ := procCreateFontW.Call(
		16, 0, 0, 0, fwNormal, 0, 0, 0,
		defaultCharset, outDefaultPrecis, clipDefaultPrecis, cleartypeQuality,
		ffSwiss|variablePitch,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Segoe UI"))),
	)
	oldFont, _, _ := procSelectObject.Call(hdc, font)
	defer func() {
		procSelectObject.Call(hdc, oldFont)
		procDeleteObject.Call(font)
	}()

	var sz size
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&wide[0])), uintptr(chars), uintptr(unsafe.Pointer(&sz)))

	const padX, padY = 8, 3
	bgRect := rect{left: x, top: labelY, right: x + sz.cx + padX*2, bottom: labelY + sz.cy + padY*2}
	bgBrush, _, _ := procCreateSolidBrush.Call(0x00333333)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bgRect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	procSetBkMode.Call(hdc, bkTransparent)
	procSetTextColor.Call(hdc, 0x00FFFFFF)
	procTextOutW.Call(hdc, uintptr(int(x+padX)), uintptr(int(labelY+padY)), uintptr(unsafe.Pointer(&wide[0])), uintptr(chars))
}
