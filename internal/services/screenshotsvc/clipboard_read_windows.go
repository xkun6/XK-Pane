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
	paths, img, err := readClipboardForPaste()
	if err != nil || len(paths) > 0 {
		return paths, err
	}
	if img == nil {
		return nil, nil
	}
	// 位图离开剪贴板锁之后再落盘（PNG 编码可能数十毫秒，不能拖着剪贴板不放）。
	path, err := s.saveScreenshot(img)
	if err != nil {
		return nil, err
	}
	// 刻意不 Emit screenshot:saved —— 那是热键截图的事件，ScreenshotBridge 收到会再写一次
	// 活跃终端，与本次前端 term.paste 重复成双份。
	return []string{path}, nil
}

// readClipboardForPaste 在持有剪贴板的窗口内只做内存读取：文件列表或位图二选一。
func readClipboardForPaste() ([]string, *image.RGBA, error) {
	if err := openClipboardRetry(); err != nil {
		return nil, nil, err
	}
	defer procCloseClipboard.Call()

	if r, _, _ := procIsClipboardFormatAvail.Call(cfHDrop); r != 0 {
		paths, err := readClipboardFileList()
		return paths, nil, err
	}
	if r, _, _ := procIsClipboardFormatAvail.Call(cfBitmap); r != 0 {
		img, err := readClipboardBitmap()
		return nil, img, err
	}
	return nil, nil, nil
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

// readClipboardBitmap 把剪贴板位图（HBITMAP 归剪贴板所有，不得 DeleteObject）经 GetDIBits
// 抓成 RGBA。两段式：先查询头取宽高，再按 32bpp 自顶向下取像素（同 capture_windows.go）。
func readClipboardBitmap() (*image.RGBA, error) {
	hbm, _, _ := procGetClipboardData.Call(cfBitmap)
	if hbm == 0 {
		return nil, fmt.Errorf("GetClipboardData(CF_BITMAP) failed (GetLastError=%d)", lastErr())
	}
	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC(NULL) failed (GetLastError=%d)", lastErr())
	}
	defer procReleaseDC.Call(0, hdcScreen)

	// 查询式 GetDIBits：lpvBits = NULL 且 biBitCount = 0 → 只回填 BITMAPINFOHEADER。
	query := bitmapInfo{header: bitmapInfoHeader{biSize: uint32(unsafe.Sizeof(bitmapInfoHeader{}))}}
	if r, _, _ := procGetDIBits.Call(hdcScreen, hbm, 0, 0, 0, uintptr(unsafe.Pointer(&query)), dibRGBColors); r == 0 {
		return nil, fmt.Errorf("GetDIBits(query) failed (GetLastError=%d)", lastErr())
	}
	w, h := query.header.biWidth, query.header.biHeight
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("剪贴板位图尺寸异常 %dx%d", w, h)
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
		return nil, fmt.Errorf("GetDIBits returned 0 (GetLastError=%d)", lastErr())
	}
	// BGRA → RGBA（同 capture_windows.go：原地换 B/R，alpha 置不透明）。
	for i := 0; i+3 < len(buffer); i += 4 {
		buffer[i], buffer[i+2] = buffer[i+2], buffer[i]
		buffer[i+3] = 255
	}
	return &image.RGBA{Pix: buffer, Stride: int(w) * 4, Rect: image.Rect(0, 0, int(w), int(h))}, nil
}
