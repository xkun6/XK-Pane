package sshmachine

// exec.go 提供经【独立非交互通道】对 ssh_machines 登记机器的远端命令执行 / 文件上传。
//
// 为什么不复用用户在 SSH 页打开的交互式 PTY 会话：那条会话是 `ssh -tt` 强制伪终端，
//   - 无 exit code（进程常驻不退出）；
//   - stdout/stderr/远端回显/ANSI/提示符混在一条流里，拿不到干净结果；
//   - 注入命令会回显污染用户屏幕、和用户/正在跑的 TUI 抢同一 PTY。
// 故这里【另起一条】非交互 ssh/scp 子进程（BatchMode），换来分离的 stdout/stderr + 真实退出码。
//
// 铁律对账：子进程一律走 procx（绑 JobObject/pgid）；明文密码经 ResolveForSSH 在进程内解密、
// 绝不跨 IPC；host/user 经 validateSSHField + hasDangerousChars 防注入；远端命令名经 ssh PATH 解析。
// **Executor 是普通结构体、不注册成 Wails service**——方法不进前端 bindings，仅供 orchestrator
// 的 ssh_exec / ssh_upload MCP 工具调用。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/procx"
)

// Executor 持有 holder，按 machineID 解密连接信息后另起非交互 ssh/scp 通道。
type Executor struct {
	holder *storage.Holder
}

// NewExecutor 构造 Executor（app 装配阶段调，传入与 sshmachine.Service 同一个 holder）。
func NewExecutor(holder *storage.Holder) *Executor {
	return &Executor{holder: holder}
}

// ExecResult 是 Exec 的返回：远端命令的分离输出 + 真实退出码。
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"` // 远端命令退出码；ssh 自身连接失败通常为 255
}

// UploadResult 是 Upload 的返回（scp 成功 exitCode=0）。
type UploadResult struct {
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

const (
	defaultExecTimeout   = 120 * time.Second
	defaultUploadTimeout = 300 * time.Second
	maxOpTimeout         = 600 * time.Second
)

// sshOpExecOptions 是非交互操作通道的公共 ssh/scp 选项：
//   - BatchMode=yes：禁交互密码提示——password 机器会【快速失败】而非挂起（ssh 从 /dev/tty 读密码不读 stdin）；
//   - ConnectTimeout=10：连不上快速返回；
//   - StrictHostKeyChecking=accept-new：首连自动接受主机指纹、后续仍校验（不像 =no 全跳过，留 MITM 防护）；
//   - ServerAlive*：长任务/大文件传输防被中间设备掐断。
var sshOpExecOptions = []string{
	"BatchMode=yes",
	"ConnectTimeout=10",
	"StrictHostKeyChecking=accept-new",
	"ServerAliveInterval=15",
	"ServerAliveCountMax=3",
}

// Exec 对 machineID 指定的机器经非交互 ssh 通道执行 command，返回分离的 stdout/stderr + 退出码。
// command 原样交远端登录 shell 执行（Agent 自负其内容；要指定目录请自行在 command 里 `cd /path && ...`）。
// 仅 key/agent 认证可用；password 机器返回清晰错误（非交互喂密码是独立攻坚项）。
// timeoutSecs<=0 取默认 120s，封顶 600s。
func (e *Executor) Exec(ctx context.Context, machineID, command string, timeoutSecs int) (ExecResult, error) {
	if strings.TrimSpace(command) == "" {
		return ExecResult{}, errors.New("command is empty")
	}
	m, err := e.resolve(ctx, machineID)
	if err != nil {
		return ExecResult{}, err
	}
	args, err := buildRemoteExecArgs(m)
	if err != nil {
		return ExecResult{}, err
	}
	args = append(args, command) // `--` 之后：host 已是首元素，command 作为远端待执行命令

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return ExecResult{}, errors.New("ssh not found in PATH")
	}

	var stdout, stderr strings.Builder
	timeout := opTimeout(timeoutSecs, defaultExecTimeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	runErr := procx.Run(runCtx, procx.Spec{
		Name:          sshPath,
		Args:          args,
		CaptureOutput: true,
		OnStdout:      func(line string) { stdout.WriteString(line + "\n") },
		OnStderr:      func(line string) { stderr.WriteString(line + "\n") },
	})
	if runCtx.Err() == context.DeadlineExceeded {
		return ExecResult{}, fmt.Errorf("ssh 执行超时（%ds）", int(timeout/time.Second))
	}
	code, err := exitCodeFrom(runErr)
	if err != nil {
		return ExecResult{}, err
	}
	return ExecResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}, nil
}

// Upload 经非交互 scp 把本机 localPath 上传到 machineID 的 remotePath（远端文件路径）。
// 仅 key/agent；password 机器同 Exec 拦截。localPath 须为本机已存在文件。timeoutSecs<=0 取默认 300s，封顶 600s。
func (e *Executor) Upload(ctx context.Context, machineID, localPath, remotePath string, timeoutSecs int) (UploadResult, error) {
	if strings.TrimSpace(localPath) == "" || strings.TrimSpace(remotePath) == "" {
		return UploadResult{}, errors.New("localPath / remotePath is empty")
	}
	if hasDangerousChars(remotePath) {
		return UploadResult{}, errors.New("remotePath contains invalid characters")
	}
	fi, err := os.Stat(localPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("本地文件不可读: %v", err)
	}
	if fi.IsDir() {
		return UploadResult{}, errors.New("localPath 是目录，暂只支持上传单个文件")
	}
	m, err := e.resolve(ctx, machineID)
	if err != nil {
		return UploadResult{}, err
	}
	args, err := buildUploadArgs(m, localPath, remotePath)
	if err != nil {
		return UploadResult{}, err
	}
	scpPath, err := exec.LookPath("scp")
	if err != nil {
		return UploadResult{}, errors.New("scp not found in PATH")
	}

	var stderr strings.Builder
	timeout := opTimeout(timeoutSecs, defaultUploadTimeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	runErr := procx.Run(runCtx, procx.Spec{
		Name:          scpPath,
		Args:          args,
		CaptureOutput: true,
		OnStderr:      func(line string) { stderr.WriteString(line + "\n") },
	})
	if runCtx.Err() == context.DeadlineExceeded {
		return UploadResult{}, fmt.Errorf("scp 上传超时（%ds）", int(timeout/time.Second))
	}
	code, err := exitCodeFrom(runErr)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{Stderr: stderr.String(), ExitCode: code}, nil
}

// resolve 按 machineID 解密加载机器（明文密码就地解密，仅进程内用、不外泄）。空/未知 id 报错。
func (e *Executor) resolve(ctx context.Context, machineID string) (*storage.SshMachine, error) {
	if strings.TrimSpace(machineID) == "" {
		return nil, errors.New("machineId is empty")
	}
	m, err := ResolveForSSH(ctx, e.holder.Current().GORM, machineID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("SSH machine %q not found", machineID)
	}
	return m, nil
}

// authArgs 做认证方式校验 + host/user 注入防御，返回 [-i identity]（key 认证且有私钥时）。
// password 机器在此拦截：非交互通道喂不了密码，给清晰错误引导改用 key/agent 或交互式会话。
func authArgs(m *storage.SshMachine) ([]string, error) {
	if m.AuthMethod == "password" {
		return nil, errors.New("密码认证的机器暂不支持经 MCP 非交互执行/上传（ssh/scp 从 /dev/tty 读密码、不读 stdin）。请把该机器改用 key/agent 认证，或在 SSH 页用交互式会话手动操作。")
	}
	if err := validateSSHField(m.Host, "host"); err != nil {
		return nil, err
	}
	if hasDangerousChars(m.Host) {
		return nil, errors.New("host contains invalid characters")
	}
	if m.Username != "" {
		if err := validateSSHField(m.Username, "user"); err != nil {
			return nil, err
		}
		if hasDangerousChars(m.Username) {
			return nil, errors.New("user contains invalid characters")
		}
	}
	var args []string
	if m.AuthMethod == "key" && m.IdentityFile != "" {
		args = append(args, "-i", m.IdentityFile)
	}
	return args, nil
}

// buildRemoteExecArgs 拼 `ssh [opts] [-p port] [-i id] -- user@host`（command 由调用方追加）。
func buildRemoteExecArgs(m *storage.SshMachine) ([]string, error) {
	auth, err := authArgs(m)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, 16)
	for _, opt := range sshOpExecOptions {
		args = append(args, "-o", opt)
	}
	if m.Port != 22 && m.Port > 0 {
		args = append(args, "-p", strconv.Itoa(m.Port))
	}
	args = append(args, auth...)
	args = append(args, "--", sshTarget(m.Username, m.Host))
	return args, nil
}

// buildUploadArgs 拼 `scp [opts] [-P port] [-i id] -- localPath user@host:remotePath`（scp 端口是大写 -P）。
func buildUploadArgs(m *storage.SshMachine, localPath, remotePath string) ([]string, error) {
	auth, err := authArgs(m)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, 16)
	for _, opt := range sshOpExecOptions {
		args = append(args, "-o", opt)
	}
	if m.Port != 22 && m.Port > 0 {
		args = append(args, "-P", strconv.Itoa(m.Port))
	}
	args = append(args, auth...)
	args = append(args, "--", localPath, sshTarget(m.Username, m.Host)+":"+remotePath)
	return args, nil
}

// exitCodeFrom 从 procx.Run 的返回取退出码：nil→0；*exec.ExitError→进程码；其它（启动失败）→ error。
func exitCodeFrom(runErr error) (int, error) {
	if runErr == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return ee.ExitCode(), nil
	}
	return -1, runErr
}

// opTimeout 归一化超时：<=0 取 fallback，封顶 maxOpTimeout。
func opTimeout(secs int, fallback time.Duration) time.Duration {
	if secs <= 0 {
		return fallback
	}
	d := time.Duration(secs) * time.Second
	if d > maxOpTimeout {
		return maxOpTimeout
	}
	return d
}
