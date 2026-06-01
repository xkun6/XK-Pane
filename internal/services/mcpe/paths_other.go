//go:build !windows

package mcpe

// scanLaunchers 在非 Windows 平台无意义（网易版 MC 仅 Windows 有开发启动器）。
func scanLaunchers() []string { return nil }
