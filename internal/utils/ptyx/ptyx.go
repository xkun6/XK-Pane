// Package ptyx 是跨平台伪终端（PTY）门面，封装 github.com/aymanbagabas/go-pty。
//
// 为什么不用 procx：procx 跑的是「非交互、行缓冲」的子进程（git/构建脚本等），
// 走 stdout/stderr 管道 + 逐行回调。但 Claude Code / codex 这类**交互式 TUI**
// 需要真·伪终端（原始字节流，含 ANSI / 光标控制序列），行缓冲会破坏渲染。
// 因此终端会话走 ptyx（PTY 原始字节流 → 前端 xterm.js），git CLI 等走 procx。
//
// 跨平台：
//   - Windows：go-pty 内部用 ConPTY（CreatePseudoConsole）。本包在 Start 后把进程
//     绑到 JobObject（KILL_ON_JOB_CLOSE），父进程崩溃连带 kill，杜绝孤儿。
//     为防绑定前 fork 逃逸，用 CREATE_SUSPENDED 起进程，绑完 Job 再 ResumeThread。
//   - Unix：go-pty 在 cmd_unix.go 强制 Setsid=true，子进程自身即新进程组组长
//     （pgid==pid）。**不能再叠 procx 的 Setpgid**（与 Setsid 互斥）；kill 走
//     syscall.Kill(-pid, SIGKILL) 干掉整组。
//
// 关闭顺序（铁律，否则丢尾部输出）：Kill（如需）→ Wait()（收子进程，非零退出是
// *exec.ExitError）→ Close()（关 pty）。前端首帧 fit 后应立刻 Resize 一次，规避
// ConPTY 0 尺寸竞态。
package ptyx

import (
	"context"
	"errors"
	"fmt"
	"sync"

	pty "github.com/aymanbagabas/go-pty"
)

// 默认终端尺寸；调用方应在拿到前端实际尺寸后尽快 Resize。
const (
	defaultCols = 80
	defaultRows = 24
)

// Spec 描述要在 PTY 中启动的命令。
type Spec struct {
	Command string   // 程序路径或 PATH 可解析名
	Args    []string // 参数（不含 argv[0]）
	Cwd     string   // 工作目录；空 = 继承父进程
	Env     []string // 形如 "K=V"；nil = 继承父进程；非 nil 完全替换
	Cols    uint16   // 初始列数；0 取默认
	Rows    uint16   // 初始行数；0 取默认
}

// PTY 是一次启动后的伪终端会话句柄。
//
// 它本身实现 io.Reader / io.Writer：Read 读 PTY 原始字节（喂给前端 xterm.js），
// Write 写用户输入。并发安全由底层 go-pty 的 RWMutex 保证；Wait 可重入。
type PTY struct {
	pt  pty.Pty
	cmd *pty.Cmd

	platform platformState

	waitMu  sync.Mutex
	waited  bool
	exitErr error
}

// Spawn 在新 PTY 中启动 spec 描述的命令。
//
// 返回的 *PTY 即使调用方提前 return 也必须最终 Kill+Wait+Close（否则泄漏 PTY / 子进程）。
func Spawn(ctx context.Context, spec Spec) (*PTY, error) {
	if spec.Command == "" {
		return nil, errors.New("ptyx: empty Command")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cols := spec.Cols
	if cols == 0 {
		cols = defaultCols
	}
	rows := spec.Rows
	if rows == 0 {
		rows = defaultRows
	}

	pt, err := pty.New()
	if err != nil {
		return nil, fmt.Errorf("ptyx: new pty: %w", err)
	}
	// 先定尺寸再 Start：规避 ConPTY 在 0 尺寸下的异常。go-pty 签名是 Resize(width, height)。
	if err := pt.Resize(int(cols), int(rows)); err != nil {
		_ = pt.Close()
		return nil, fmt.Errorf("ptyx: initial resize: %w", err)
	}

	cmd := pt.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Cwd
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	applyJobAttrs(cmd) // Windows: CREATE_SUSPENDED；Unix: no-op（go-pty 自管 Setsid）

	p := &PTY{pt: pt, cmd: cmd}

	if err := cmd.Start(); err != nil {
		_ = pt.Close()
		return nil, fmt.Errorf("ptyx: start %s: %w", spec.Command, err)
	}

	// 绑进程组 / Job；失败不致命（进程已起，与 procx 策略一致），但会留下孤儿风险。
	if err := bindJob(p); err != nil {
		_ = err
	}
	return p, nil
}

// Read 读 PTY 原始输出字节。
func (p *PTY) Read(b []byte) (int, error) { return p.pt.Read(b) }

// Write 把用户输入写入 PTY。
func (p *PTY) Write(b []byte) (int, error) { return p.pt.Write(b) }

// Resize 调整 PTY 尺寸（cols=宽, rows=高）。0 尺寸被忽略以避免 ConPTY 异常。
func (p *PTY) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return p.pt.Resize(int(cols), int(rows))
}

// PID 返回子进程 ID；未启动返回 0。
func (p *PTY) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Wait 阻塞等待子进程退出。可重入：第二次起立即返回首次结果。
// 非零退出返回 *exec.ExitError（errors.As 可取）。
func (p *PTY) Wait() error {
	p.waitMu.Lock()
	if p.waited {
		err := p.exitErr
		p.waitMu.Unlock()
		return err
	}
	p.waited = true
	p.waitMu.Unlock()

	err := p.cmd.Wait()
	p.exitErr = err
	releaseGroup(p)
	return err
}

// Kill 硬终止整个进程组 / Job（子孙一起死）。之后应 Wait() 收尾再 Close()。
func (p *PTY) Kill() error { return killGroup(p) }

// Close 关闭伪终端。务必在 Wait() 之后调用，否则可能丢尾部输出。
func (p *PTY) Close() error { return p.pt.Close() }
