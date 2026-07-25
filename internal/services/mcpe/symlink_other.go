//go:build !windows

package mcpe

import (
	"fmt"
	"os"
)

// isPrivilegeError 非 Windows 上 os.Symlink 不涉及 SeCreateSymbolicLinkPrivilege，恒 false。
func isPrivilegeError(error) bool { return false }

// isLinkEntry 非 Windows 上没有 junction/reparse point，链接就等于 symlink。
func isLinkEntry(fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}

// sudoCreateSymlinks 仅 Windows 实现（UAC 提权）。
func sudoCreateSymlinks([]symlinkPair) error {
	return fmt.Errorf("sudoCreateSymlinks 仅在 Windows 实现")
}
