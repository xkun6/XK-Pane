//go:build windows

package procx

import (
	"os/exec"
	"testing"
)

// ShowWindow 决定是否对子进程设 HideWindow(SW_HIDE)：
//   - false（默认）：控制台子进程隐藏窗口，不闪窗（铁律）
//   - true：GUI 启动器（如 explorer.exe）必须不隐藏，否则弹出的窗口被一并隐藏
func TestApplyPlatformAttrsShowWindow(t *testing.T) {
	hidden := exec.Command("git")
	applyPlatformAttrs(hidden, false)
	if !hidden.SysProcAttr.HideWindow {
		t.Error("ShowWindow=false 应设 HideWindow（控制台子进程不闪窗）")
	}

	shown := exec.Command("explorer.exe")
	applyPlatformAttrs(shown, true)
	if shown.SysProcAttr.HideWindow {
		t.Error("ShowWindow=true 不应设 HideWindow（否则 explorer 弹窗被隐藏）")
	}
}
