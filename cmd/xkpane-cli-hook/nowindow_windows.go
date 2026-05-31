//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// noWindowCmd 构造一个在 Windows 下不弹控制台窗口的命令（CREATE_NO_WINDOW = 0x08000000）。
// 对齐 cc-pane session_start.rs 的 no_window_command。
func noWindowCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd
}
