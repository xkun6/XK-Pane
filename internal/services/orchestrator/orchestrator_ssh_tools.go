package orchestrator

// orchestrator_ssh_tools.go：结合 ccpane SSH 的两个编排工具 —— ssh_exec / ssh_upload。
//
// 与「向用户交互式 SSH 会话写键」（write_to_session / submit_to_session）互补、本质不同：
// 那条是把按键发进用户在 SSH 页打开的 `ssh -tt` PTY 会话（拿不到 exit code、会回显污染、抢 PTY）；
// 这俩则【另起】一条非交互 ssh/scp 子进程通道——拿干净的 stdout/stderr + 真实退出码、可二进制传文件，
// 既不打扰也不依赖用户那条已打开的会话。machineId 可从 list_sessions 里 runtimeKind=='ssh' 的会话取得。
//
// 兜底实现在 sshmachine.Executor（procx 起进程 + ResolveForSSH 解密 + host/user 防注入）。

import (
	"context"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Service) registerSSHTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ssh_exec",
		Description: "对已登记的 SSH 机器（machine_id）经独立非交互通道执行一条远端命令，返回分离的 stdout/stderr 和真实退出码 exitCode。与 write_to_session 不同：这里不复用用户打开的交互式 SSH 会话，而是另起一条干净的 ssh 通道，适合需要可靠结果/退出码的自动化操作。要指定目录请在 command 里自行写 'cd /path && ...'。仅支持 key/agent 认证的机器；password 认证机器会返回提示。machine_id 可从 list_sessions 中 runtimeKind=='ssh' 的会话取得。",
	}, s.toolSSHExec)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ssh_upload",
		Description: "把本机一个文件经非交互 scp 上传到已登记 SSH 机器（machine_id）的远端路径 remote_path。二进制安全，返回 exitCode（0=成功）与 scp 的 stderr。仅支持 key/agent 认证的机器。",
	}, s.toolSSHUpload)
}

// ── 参数结构 ─────────────────────────────────────────────────────────────

type sshExecParams struct {
	MachineID   string `json:"machine_id" jsonschema:"目标 SSH 机器 id（ssh_machines.id；可从 list_sessions 的 SSH 会话取 machineId）"`
	Command     string `json:"command" jsonschema:"要在远端登录 shell 执行的命令（原样执行；要切目录请自行 cd && ）"`
	TimeoutSecs int    `json:"timeout_secs,omitempty" jsonschema:"超时秒数（默认 120，封顶 600）"`
}

type sshUploadParams struct {
	MachineID   string `json:"machine_id" jsonschema:"目标 SSH 机器 id"`
	LocalPath   string `json:"local_path" jsonschema:"本机待上传文件的绝对路径"`
	RemotePath  string `json:"remote_path" jsonschema:"远端目标文件路径"`
	TimeoutSecs int    `json:"timeout_secs,omitempty" jsonschema:"超时秒数（默认 300，封顶 600）"`
}

// ── 工具实现 ─────────────────────────────────────────────────────────────

func (s *Service) toolSSHExec(ctx context.Context, _ *mcp.CallToolRequest, in sshExecParams) (*mcp.CallToolResult, any, error) {
	if s.deps.SSHExec == nil {
		return textResult("错误: ssh_exec 不可用（SSHExec 未接线）"), nil, nil
	}
	res, err := s.deps.SSHExec.Exec(ctx, in.MachineID, in.Command, in.TimeoutSecs)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(map[string]any{
		"machineId": in.MachineID,
		"exitCode":  res.ExitCode,
		"stdout":    res.Stdout,
		"stderr":    res.Stderr,
	}), nil, nil
}

func (s *Service) toolSSHUpload(ctx context.Context, _ *mcp.CallToolRequest, in sshUploadParams) (*mcp.CallToolResult, any, error) {
	if s.deps.SSHExec == nil {
		return textResult("错误: ssh_upload 不可用（SSHExec 未接线）"), nil, nil
	}
	res, err := s.deps.SSHExec.Upload(ctx, in.MachineID, in.LocalPath, in.RemotePath, in.TimeoutSecs)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(map[string]any{
		"machineId": in.MachineID,
		"exitCode":  res.ExitCode,
		"stderr":    res.Stderr,
		"success":   res.ExitCode == 0,
	}), nil, nil
}
