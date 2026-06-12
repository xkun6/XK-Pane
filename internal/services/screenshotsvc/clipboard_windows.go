//go:build windows

package screenshotsvc

// clipboard_windows.go — 把已保存的截图以「文件对象」(CF_HDROP) 写入剪贴板：粘贴目标拿到的
// 是图片文件本身（聊天工具得图、资源管理器得文件，同 Explorer「复制文件」语义），而非路径
// 字符串。Wails 的 ClipboardManager 仅支持文本，故自接 Win32。

import (
	"encoding/binary"
	"fmt"
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

// copyFileToClipboard 以 CF_HDROP 文件对象写剪贴板（路径须为已存在文件的绝对路径）。
func copyFileToClipboard(path string) error {
	wide, err := windows.UTF16FromString(path) // 含结尾 NUL
	if err != nil {
		return fmt.Errorf("路径转宽字符失败: %w", err)
	}
	headerSize := int(unsafe.Sizeof(dropFiles{}))
	total := headerSize + (len(wide)+1)*2 // +1 = 文件列表的第二个 NUL 收尾

	// 先在 Go 侧拼好完整负载（DROPFILES 头 + 宽字符路径 + 双 NUL），再整块 RtlMoveMemory
	// 进锁定内存——避免把 GlobalLock 返回的 uintptr 转回 unsafe.Pointer（go vet 禁止）。
	buf := make([]byte, total)
	binary.LittleEndian.PutUint32(buf[0:], uint32(headerSize)) // pFiles；pt/fNC 保持零值
	binary.LittleEndian.PutUint32(buf[16:], 1)                 // fWide = TRUE
	for i, u := range wide {
		binary.LittleEndian.PutUint16(buf[headerSize+i*2:], u)
	}

	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(total))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed (GetLastError=%d)", lastErr())
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("GlobalLock failed (GetLastError=%d)", lastErr())
	}
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&buf[0])), uintptr(total))
	procGlobalUnlock.Call(hMem)

	// 剪贴板可能被其它进程短暂持有，带退避重试。
	opened := false
	for i := 0; i < 5; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !opened {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("OpenClipboard failed (GetLastError=%d)", lastErr())
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	if r, _, _ := procSetClipboardData.Call(cfHDrop, hMem); r == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData failed (GetLastError=%d)", lastErr())
	}
	// SetClipboardData 成功后内存归系统所有，不得 GlobalFree。
	return nil
}
