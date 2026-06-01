//go:build !windows

package mcpe

import "fmt"

// moveToTrash 仅 Windows 实现（回收站是 Windows 概念；网易版 MC 也仅 Windows）。
func moveToTrash(path string) error {
	return fmt.Errorf("moveToTrash 仅在 Windows 实现：%s", path)
}
