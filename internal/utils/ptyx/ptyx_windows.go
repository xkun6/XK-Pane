//go:build windows

package ptyx

import (
	"syscall"
	"unsafe"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
)

// platformState：Windows 上保存本会话独占的 JobObject 句柄。
type platformState struct {
	job windows.Handle
}

// applyJobAttrs：用 CREATE_SUSPENDED 起进程，便于 Start 后先绑 Job 再 ResumeThread，
// 杜绝「绑定前就 fork 出子进程逃逸 Job」。
//
// 刻意**不加** CREATE_NEW_PROCESS_GROUP：交互式 PTY 的 Ctrl+C 以字节（0x03）经
// PTY 输入流传递，由子进程行规程处理；新进程组标志会改变控制台信号语义、干扰 TUI。
func applyJobAttrs(cmd *pty.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
}

// bindJob：建 Job → 设 KILL_ON_JOB_CLOSE → 把子进程加入 → ResumeThread。
// 父进程（本应用）退出关闭所有 handle 时，系统自动 kill Job 内全部进程。
func bindJob(p *PTY) error {
	if p.cmd.Process == nil {
		return nil
	}
	pid := uint32(p.cmd.Process.Pid)

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		_ = resumeMainThread(pid)
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		_ = resumeMainThread(pid)
		return err
	}

	hProc, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		pid,
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		_ = resumeMainThread(pid)
		return err
	}
	defer windows.CloseHandle(hProc)

	if err := windows.AssignProcessToJobObject(job, hProc); err != nil {
		_ = windows.CloseHandle(job)
		_ = resumeMainThread(pid)
		return err
	}

	p.platform.job = job
	return resumeMainThread(pid)
}

// resumeMainThread：通过 ToolHelp 找到该 PID 的（主）线程并 ResumeThread。
// CREATE_SUSPENDED 起的新进程只有一个挂起的主线程，恢复它即开始执行。
func resumeMainThread(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)

	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	if err := windows.Thread32First(snap, &te); err != nil {
		return err
	}
	for {
		if te.OwnerProcessID == pid {
			h, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, te.ThreadID)
			if err == nil {
				_, _ = windows.ResumeThread(h)
				_ = windows.CloseHandle(h)
				return nil // 主线程一般是第一个；恢复一次即可
			}
		}
		if err := windows.Thread32Next(snap, &te); err != nil {
			return nil // 没找到也不致命
		}
	}
}

// killGroup：硬终止整个 Job —— 子孙进程一起死。
func killGroup(p *PTY) error {
	if p.platform.job != 0 {
		return windows.TerminateJobObject(p.platform.job, 1)
	}
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// releaseGroup：进程已退出，关闭 Job 句柄。
func releaseGroup(p *PTY) {
	if p.platform.job != 0 {
		_ = windows.CloseHandle(p.platform.job)
		p.platform.job = 0
	}
}
