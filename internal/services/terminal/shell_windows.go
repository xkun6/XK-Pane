//go:build windows

package terminal

import (
	"os"
	"os/exec"
)

// defaultShell 解析 Windows 平台默认 shell：pwsh > powershell > %ComSpec%（缺省 cmd.exe），
// 均不带参数。忠实移植 cc-pane terminal_service/shell.rs::resolve_default_shell 的 #[cfg(windows)] 分支
// （那里用 which 探测；此处用 exec.LookPath 既探测又解析到绝对路径，行为一致且对 ConPTY 更确定）。
func defaultShell() (string, []string) {
	if p, err := exec.LookPath("pwsh"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("powershell"); err == nil {
		return p, nil
	}
	if comspec := os.Getenv("ComSpec"); comspec != "" {
		return comspec, nil
	}
	return "cmd.exe", nil
}
