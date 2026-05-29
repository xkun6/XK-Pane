//go:build !windows

package ptyx

import (
	"syscall"

	pty "github.com/aymanbagabas/go-pty"
)

// platformState：Unix 上保存进程组 ID（pgid）。
type platformState struct {
	pgid int
}

// applyJobAttrs：Unix 上**留空**。go-pty 在 cmd_unix.go 的 start() 强制
// Setsid=true + Setctty=true，子进程自身即新 session/进程组组长（pgid==pid）；
// 内核不允许 Setsid 与 Setpgid 共存，故绝不能再叠 procx 那样的 Setpgid。
func applyJobAttrs(_ *pty.Cmd) {}

// bindJob：go-pty Setsid 后 pgid==pid（子进程是组长），直接记录 pid 作 pgid。
func bindJob(p *PTY) error {
	if p.cmd.Process == nil {
		return nil
	}
	p.platform.pgid = p.cmd.Process.Pid
	return nil
}

// killGroup：给进程组发 SIGKILL，子孙一起死。
func killGroup(p *PTY) error {
	if p.platform.pgid != 0 {
		return syscall.Kill(-p.platform.pgid, syscall.SIGKILL)
	}
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// releaseGroup：Unix 无句柄可关，留空。
func releaseGroup(_ *PTY) {}
