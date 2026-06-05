package terminal

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// 本文件移植 cc-pane terminal_service.rs 的 SSH 启动命令构造（build_ssh_command +
// append_ssh_session_options）。核心做法：把本地 `ssh` 客户端当作 PTY 命令拉起，
// 复用整条 ptyx→readLoop→batchLoop→xterm 管线（不内嵌 Go SSH 库）。
//
// 远端命令是**裸命令名**（claude/codex --full-auto/exec $SHELL -l），不复用本地 adapter 的
// MCP/arg 构造——远程主机够不到本地 127.0.0.1 orchestrator，故不注入 panes MCP（对齐 cc-pane）。

// sshSessionOptions 是连接保活/超时项（对齐 cc-pane append_ssh_session_options）。
var sshSessionOptions = []string{
	"ConnectTimeout=10",
	"ServerAliveInterval=15",
	"ServerAliveCountMax=2",
	"TCPKeepAlive=yes",
}

// remoteCLICommand 把 XK 的 cliTool id 映射成远端裸命令（对齐 cc-pane build_ssh_command 的 match）。
// XK 仅 claude/reclaude/codex（+ none 裸 shell）；未知工具回退裸 shell（不臆造远端命令）。
func remoteCLICommand(cliTool string) string {
	switch cliTool {
	case "claude":
		return "claude"
	case "reclaude":
		return "reclaude"
	case "codex":
		return "codex --full-auto"
	default: // "", "none", 未知
		return "exec $SHELL -l"
	}
}

// buildSSHCommand 生成本地 ssh 客户端命令把远程会话拉进 PTY（移植 cc-pane build_ssh_command）：
//
//	ssh -tt [keepalive opts] [-p port] [-i identity] [user@]host '<export K=V && ... && cd path && cli>'
//
// providerEnv 仅 cliTool!=none 时远程 export（key 白名单 + value POSIX 单引号转义）。
// remotePath 为 "~"/"~/"/"" 时跳过 cd（SSH 登录默认即 home）。命令名 "ssh" 经 PATH 解析；
// 缺失则报清晰错误（Windows 需 OpenSSH 客户端，Win10 1809+/Win11 默认自带）。
func buildSSHCommand(ssh SSHSpec, cliTool string, providerEnv map[string]string) (string, []string, error) {
	if strings.TrimSpace(ssh.Host) == "" {
		return "", nil, errors.New("terminal: ssh host is empty")
	}
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return "", nil, errors.New("terminal: ssh client not found in PATH (Windows 需安装 OpenSSH 客户端)")
	}
	return sshPath, buildSSHArgs(ssh, cliTool, providerEnv), nil
}

// buildSSHArgs 是纯 argv 构造（无 PATH/IO 依赖，便于单测）。调用方已保证 ssh.Host 非空。
func buildSSHArgs(ssh SSHSpec, cliTool string, providerEnv map[string]string) []string {
	args := []string{"-tt"} // 强制伪终端，避免远端 TUI 降级
	for _, opt := range sshSessionOptions {
		args = append(args, "-o", opt)
	}
	port := ssh.Port
	if port <= 0 {
		port = 22
	}
	if port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	if ssh.IdentityFile != "" {
		args = append(args, "-i", ssh.IdentityFile)
	}
	args = append(args, sshTarget(ssh.User, ssh.Host))

	// 远端命令：export 注入（仅真 CLI）→ cd → 工具，用 && 串联（任一失败即中断，避免在错目录跑工具）。
	var remote []string
	if cliTool != "" && cliTool != "none" {
		for k, v := range providerEnv {
			if isValidEnvKey(k) {
				remote = append(remote, "export "+k+"="+shellEscape(v))
			}
		}
	}
	if rp := ssh.RemotePath; rp != "" && rp != "~" && rp != "~/" {
		remote = append(remote, "cd "+shellEscape(rp))
	}
	remote = append(remote, remoteCLICommand(cliTool))
	args = append(args, strings.Join(remote, " && "))
	return args
}

// sshTarget 拼 user@host 或仅 host。
func sshTarget(user, host string) string {
	if user != "" {
		return user + "@" + host
	}
	return host
}

// shellEscape 用 POSIX 单引号把任意值包成安全的单 shell token：内嵌单引号 ' → '\''。
// 单引号内除单引号外一切（含 $、空格、; | & 等）皆字面，杜绝远端命令注入。
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isValidEnvKey 仅允许 [A-Za-z_][A-Za-z0-9_]*，防把非法 key 拼进远程 export（对齐 cc-pane is_valid_env_key）。
func isValidEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			// ok
		case i > 0 && r >= '0' && r <= '9':
			// ok（数字不可做首字符）
		default:
			return false
		}
	}
	return true
}
