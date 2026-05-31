//go:build !windows

package main

import "os/exec"

// noWindowCmd 在非 Windows 平台等价于普通 exec.Command（无窗口概念）。
func noWindowCmd(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
