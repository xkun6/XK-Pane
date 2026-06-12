//go:build windows

package screenshotsvc

// clipboard_read_windows.go — 读剪贴板供终端 Ctrl+V「文件/图片 → 路径」粘贴
//（对译 cc-pane terminalClipboard ②流）：
//   CF_HDROP  → 直接返回文件路径列表（Explorer 复制的文件、XK 自己的截图）
//   CF_BITMAP → 位图抓成 RGBA 落盘 screenshots 目录成 PNG，返回该路径（截图工具
//               「复制图像」场景；CF_DIB 由系统自动合成 CF_BITMAP，一并覆盖）
//   都没有    → 返回空（前端 no-op）

import (
	"fmt"
	"image"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procGetClipboardData       = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvail = user32.NewProc("IsClipboardFormatAvailable")
	procDragQueryFileW         = shell32.NewProc("DragQueryFileW")
)

const dragQueryFileCount = 0xFFFF_FFFF // DragQueryFileW 第二参传 -1 取文件总数

func (s *Service) platformClipboardPastePaths() ([]string, error) {
	// 剪贴板会话有 OS 线程亲和性：CloseClipboard 必须由调 OpenClipboard 的同一线程执行，
	// goroutine 迁移（长 syscall 后换 M / 异步抢占）会让 defer 的 Close 跨线程静默失败 →
	// 剪贴板滞留打开态，全系统真实窗口应用的复制粘贴失败（对抗审查真机压测实证）。
	// 写路径天然安全（trigger goroutine 先 LockOSThread 再写），读路径在此显式锁线程。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	paths, bgra, w, h, err := readClipboardForPaste()
	if err != nil || len(paths) > 0 {
		return paths, err
	}
	if bgra == nil {
		return nil, nil
	}
	// 以下都在剪贴板会话之外：BGRA→RGBA 交换（大位图 >10ms）与 PNG 落盘不占系统剪贴板。
	for i := 0; i+3 < len(bgra); i += 4 {
		bgra[i], bgra[i+2] = bgra[i+2], bgra[i]
		bgra[i+3] = 255
	}
	img := &image.RGBA{Pix: bgra, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	path, err := s.saveScreenshot(img)
	if err != nil {
		return nil, err
	}
	// 刻意不 Emit screenshot:saved —— 那是热键截图的事件，ScreenshotBridge 收到会再写一次
	// 活跃终端，与本次前端 term.paste 重复成双份。
	return []string{path}, nil
}

// readClipboardForPaste 在持有剪贴板的窗口内只做内存读取（调用方须已 LockOSThread）：
// 文件列表或原始自顶向下 BGRA 位图二选一，像素转换留到会话之外。
func readClipboardForPaste() ([]string, []byte, int, int, error) {
	if err := openClipboardRetry(); err != nil {
		return nil, nil, 0, 0, err
	}
	defer func() {
		if r, _, _ := procCloseClipboard.Call(); r == 0 {
			logFor().Warn("CloseClipboard 失败（剪贴板可能滞留打开态）", "lastErr", lastErr())
		}
	}()

	if r, _, _ := procIsClipboardFormatAvail.Call(cfHDrop); r != 0 {
		paths, err := readClipboardFileList()
		return paths, nil, 0, 0, err
	}
	if r, _, _ := procIsClipboardFormatAvail.Call(cfBitmap); r != 0 {
		bgra, w, h, err := readClipboardBitmapBGRA()
		return nil, bgra, w, h, err
	}
	return nil, nil, 0, 0, nil
}

// readClipboardFileList 经 DragQueryFileW 读 CF_HDROP 的全部文件路径（HDROP 即 HGLOBAL，
// shell 帮我们解析 DROPFILES，无需 GlobalLock）。
func readClipboardFileList() ([]string, error) {
	hDrop, _, _ := procGetClipboardData.Call(cfHDrop)
	if hDrop == 0 {
		return nil, fmt.Errorf("GetClipboardData(CF_HDROP) failed (GetLastError=%d)", lastErr())
	}
	count, _, _ := procDragQueryFileW.Call(hDrop, dragQueryFileCount, 0, 0)
	paths := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		n, _, _ := procDragQueryFileW.Call(hDrop, i, 0, 0) // 不含 NUL 的字符数
		if n == 0 {
			continue
		}
		buf := make([]uint16, n+1)
		got, _, _ := procDragQueryFileW.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), n+1)
		if got == 0 {
			continue
		}
		paths = append(paths, windows.UTF16ToString(buf))
	}
	return paths, nil
}

// readClipboardBitmapBGRA 把剪贴板位图（HBITMAP 归剪贴板所有，不得 DeleteObject）经
// GetDIBits 抓成原始自顶向下 BGRA + 宽高。两段式：先查询头取宽高，再按 32bpp 取像素
// （同 capture_windows.go）。BGRA→RGBA 转换由调用方在剪贴板会话之外做。
func readClipboardBitmapBGRA() ([]byte, int, int, error) {
	hbm, _, _ := procGetClipboardData.Call(cfBitmap)
	if hbm == 0 {
		return nil, 0, 0, fmt.Errorf("GetClipboardData(CF_BITMAP) failed (GetLastError=%d)", lastErr())
	}
	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, 0, 0, fmt.Errorf("GetDC(NULL) failed (GetLastError=%d)", lastErr())
	}
	defer procReleaseDC.Call(0, hdcScreen)

	// 查询式 GetDIBits：lpvBits = NULL 且 biBitCount = 0 → 只回填 BITMAPINFOHEADER。
	query := bitmapInfo{header: bitmapInfoHeader{biSize: uint32(unsafe.Sizeof(bitmapInfoHeader{}))}}
	if r, _, _ := procGetDIBits.Call(hdcScreen, hbm, 0, 0, 0, uintptr(unsafe.Pointer(&query)), dibRGBColors); r == 0 {
		return nil, 0, 0, fmt.Errorf("GetDIBits(query) failed (GetLastError=%d)", lastErr())
	}
	w, h := query.header.biWidth, query.header.biHeight
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 {
		return nil, 0, 0, fmt.Errorf("剪贴板位图尺寸异常 %dx%d", w, h)
	}

	buffer := make([]byte, int(w)*4*int(h))
	bi := bitmapInfo{header: bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       w,
		biHeight:      -h, // 负高 → 自顶向下
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}}
	scanLines, _, _ := procGetDIBits.Call(
		hdcScreen, hbm, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if scanLines == 0 {
		return nil, 0, 0, fmt.Errorf("GetDIBits returned 0 (GetLastError=%d)", lastErr())
	}
	return buffer, int(w), int(h), nil
}
