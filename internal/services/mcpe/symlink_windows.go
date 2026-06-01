//go:build windows

package mcpe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"xkpane/internal/utils/procx"

	"github.com/google/uuid"
)

// errorPrivilegeNotHeld = Windows ERROR_PRIVILEGE_NOT_HELD（创建 symlink 缺权限）。
const errorPrivilegeNotHeld syscall.Errno = 1314

// isPrivilegeError 判断 os.Symlink 的错误是否是“缺 SeCreateSymbolicLinkPrivilege”。
func isPrivilegeError(err error) bool {
	var le *os.LinkError
	if errors.As(err, &le) {
		var errno syscall.Errno
		if errors.As(le.Err, &errno) && errno == errorPrivilegeNotHeld {
			return true
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "privilege")
}

// sudoCreateSymlinks 合并多对 (target, link) 生成临时 .bat，用 PowerShell Start-Process
// -Verb RunAs 弹一次 UAC 全部完成。
//
// 安全：所有路径经 validateCommandLinePath 校验（高权限上下文，路径注入等价本地提权）。
// 子进程走 procx（铁律；procx 已设 HideWindow，等效 cc-pane 的 CREATE_NO_WINDOW，不闪窗）。
func sudoCreateSymlinks(pairs []symlinkPair) error {
	if len(pairs) == 0 {
		return nil
	}
	content, err := buildSymlinkBat(pairs)
	if err != nil {
		return err
	}
	// 用 uuid 避免并发碰撞。
	batPath := filepath.Join(os.TempDir(), "xkpane_mcpe_symlink_"+uuid.NewString()+".bat")
	if err := os.WriteFile(batPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("创建临时 bat %s: %w", batPath, err)
	}
	defer os.Remove(batPath)

	// PowerShell 单引号字符串里 ' 需转义；含单引号直接拒绝（防注入）。
	if strings.Contains(batPath, "'") {
		return fmt.Errorf("临时 bat 路径含单引号")
	}
	psCmd := fmt.Sprintf("Start-Process -FilePath '%s' -Verb RunAs -Wait -WindowStyle Hidden", batPath)

	var stderrLines []string
	runErr := procx.Run(context.Background(), procx.Spec{
		Name:          "powershell",
		Args:          []string{"-NoProfile", "-Command", psCmd},
		CaptureOutput: true,
		OnStderr:      func(line string) { stderrLines = append(stderrLines, line) },
	})
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return fmt.Errorf("UAC 提权 symlink 失败 (exit %d): %s", ee.ExitCode(), strings.Join(stderrLines, "\n"))
		}
		return fmt.Errorf("PowerShell 启动失败: %w", runErr)
	}
	return nil
}
