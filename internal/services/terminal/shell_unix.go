//go:build !windows

package terminal

import "os"

// defaultShell 解析 Unix 平台默认 shell：$SHELL，缺省 /bin/sh，不带参数。
// 忠实移植 cc-pane terminal_service/shell.rs::resolve_default_shell 的 #[cfg(unix)] 分支。
func defaultShell() (string, []string) {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh, nil
	}
	return "/bin/sh", nil
}
