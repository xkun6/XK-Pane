//go:build windows

package screenshotsvc

// clipboard_windows.go — 把已保存的截图写入剪贴板，一次会话同时放两种格式（多格式共存，
// 粘贴目标各取所需）：
//   CF_HDROP —— 文件对象（聊天工具得图、资源管理器得文件，同 Explorer「复制文件」语义）
//   CF_DIB   —— 位图（Claude Code 在 Windows 的 Alt+V 粘图走 PowerShell
//               `Get-Clipboard -Format Image`，只认位图、不认文件列表）
// Wails 的 ClipboardManager 仅支持文本，故自接 Win32。

import (
	"encoding/binary"
	"fmt"
	"image"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfBitmap     = 2
	cfDIB        = 8
	cfHDrop      = 15
	gmemMoveable = 0x0002
)

// dropFiles == DROPFILES（20 字节），后跟双 NUL 结尾的宽字符路径列表。
type dropFiles struct {
	pFiles uint32
	pt     point
	fNC    int32
	fWide  int32
}

// buildDropFilesPayload 拼 CF_HDROP 负载：DROPFILES 头 + 宽字符路径 + 列表收尾 NUL。
func buildDropFilesPayload(path string) ([]byte, error) {
	wide, err := windows.UTF16FromString(path) // 含结尾 NUL
	if err != nil {
		return nil, fmt.Errorf("路径转宽字符失败: %w", err)
	}
	headerSize := int(unsafe.Sizeof(dropFiles{}))
	buf := make([]byte, headerSize+(len(wide)+1)*2)            // +1 = 文件列表的第二个 NUL 收尾
	binary.LittleEndian.PutUint32(buf[0:], uint32(headerSize)) // pFiles；pt/fNC 保持零值
	binary.LittleEndian.PutUint32(buf[16:], 1)                 // fWide = TRUE
	for i, u := range wide {
		binary.LittleEndian.PutUint16(buf[headerSize+i*2:], u)
	}
	return buf, nil
}

// buildDIBPayload 把 RGBA 图像拼成 CF_DIB 负载：BITMAPINFOHEADER（32bpp BI_RGB）+
// 自底向上 BGRA 像素行（剪贴板 DIB 的规范形态）。兼容 SubImage（非零 Rect.Min、外部 Stride）。
func buildDIBPayload(img *image.RGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	headerSize := int(unsafe.Sizeof(bitmapInfoHeader{}))
	stride := w * 4
	buf := make([]byte, headerSize+stride*h)

	binary.LittleEndian.PutUint32(buf[0:], uint32(headerSize)) // biSize
	binary.LittleEndian.PutUint32(buf[4:], uint32(int32(w)))   // biWidth
	binary.LittleEndian.PutUint32(buf[8:], uint32(int32(h)))   // biHeight（正高 = 自底向上）
	binary.LittleEndian.PutUint16(buf[12:], 1)                 // biPlanes
	binary.LittleEndian.PutUint16(buf[14:], 32)                // biBitCount
	binary.LittleEndian.PutUint32(buf[16:], biRGB)             // biCompression
	binary.LittleEndian.PutUint32(buf[20:], uint32(stride*h))  // biSizeImage；其余字段保持 0

	for y := 0; y < h; y++ {
		src := img.PixOffset(b.Min.X, b.Min.Y+y)
		dst := headerSize + (h-1-y)*stride // 自底向上：源第 y 行落到倒数第 y+1 行
		for x := 0; x < w; x++ {
			si, di := src+x*4, dst+x*4
			buf[di+0] = img.Pix[si+2] // B
			buf[di+1] = img.Pix[si+1] // G
			buf[di+2] = img.Pix[si+0] // R
			buf[di+3] = 0xFF          // 截图恒不透明
		}
	}
	return buf
}

// allocGlobalPayload 把 Go 缓冲整块拷入新分配的可移动全局内存。先在 Go 侧拼好完整负载再
// RtlMoveMemory 进锁定内存——避免把 GlobalLock 返回的 uintptr 转回 unsafe.Pointer（go vet 禁止）。
func allocGlobalPayload(data []byte) (uintptr, error) {
	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if hMem == 0 {
		return 0, fmt.Errorf("GlobalAlloc failed (GetLastError=%d)", lastErr())
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0, fmt.Errorf("GlobalLock failed (GetLastError=%d)", lastErr())
	}
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	procGlobalUnlock.Call(hMem)
	return hMem, nil
}

// openClipboardRetry 打开剪贴板；可能被其它进程短暂持有，带退避重试。
func openClipboardRetry() error {
	for i := 0; i < 5; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("OpenClipboard failed (GetLastError=%d)", lastErr())
}

// setClipboardPayload 把负载放上剪贴板指定格式。成功后内存归系统所有，不得 GlobalFree。
func setClipboardPayload(format uintptr, data []byte) error {
	hMem, err := allocGlobalPayload(data)
	if err != nil {
		return err
	}
	if r, _, _ := procSetClipboardData.Call(format, hMem); r == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData(%d) failed (GetLastError=%d)", format, lastErr())
	}
	return nil
}

// copyScreenshotToClipboard 以「文件对象 + 位图」双格式写剪贴板（路径须为已存在文件的绝对路径）。
func copyScreenshotToClipboard(path string, img *image.RGBA) error {
	hdrop, err := buildDropFilesPayload(path)
	if err != nil {
		return err
	}
	dib := buildDIBPayload(img)

	if err := openClipboardRetry(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	if err := setClipboardPayload(cfHDrop, hdrop); err != nil {
		return err
	}
	if err := setClipboardPayload(cfDIB, dib); err != nil {
		return fmt.Errorf("位图格式写入失败（文件格式已就位）: %w", err)
	}
	return nil
}
