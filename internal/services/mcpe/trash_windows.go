//go:build windows

package mcpe

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// moveToTrash 把路径移到 Windows 回收站（SHFileOperationW + FOF_ALLOWUNDO）。
// 对齐 cc-pane 的 trash::delete（move_world_to_trash）。
//
// Go 无内置回收站 API，手写 shell32 FFI。pFrom 需双 NUL 结尾的 UTF-16 路径列表。
var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

// SHFILEOPSTRUCTW（64 位自然对齐；Go 自动补 padding 与 MSVC 一致）。
type shFileOpStructW struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete          uint32 = 0x0003
	fofSilent         uint16 = 0x0004
	fofNoConfirmation uint16 = 0x0010
	fofAllowUndo      uint16 = 0x0040 // 关键：进回收站而非永久删除
	fofNoErrorUI      uint16 = 0x0400
)

func moveToTrash(path string) error {
	abs, err := filepath.Abs(path) // SHFileOperation 要绝对路径
	if err != nil {
		return fmt.Errorf("解析绝对路径 %s: %w", path, err)
	}
	u16, err := windows.UTF16FromString(abs) // 含单个尾 NUL；路径含 NUL 会报错
	if err != nil {
		return fmt.Errorf("路径编码失败 %s: %w", abs, err)
	}
	// pFrom 需双 NUL 结尾：UTF16FromString 已带 1 个，再补 1 个（buf 多一格，零值即 NUL）。
	buf := make([]uint16, len(u16)+1)
	copy(buf, u16)

	op := shFileOpStructW{
		wFunc:  foDelete,
		pFrom:  &buf[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}
	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperation 移到回收站失败 (code=%d): %s", ret, path)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("回收站删除被中止: %s", path)
	}
	return nil
}
