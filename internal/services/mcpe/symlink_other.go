//go:build !windows

package mcpe

import "fmt"

// isPrivilegeError 非 Windows 上 os.Symlink 不涉及 SeCreateSymbolicLinkPrivilege，恒 false。
func isPrivilegeError(error) bool { return false }

// sudoCreateSymlinks 仅 Windows 实现（UAC 提权）。
func sudoCreateSymlinks([]symlinkPair) error {
	return fmt.Errorf("sudoCreateSymlinks 仅在 Windows 实现")
}
