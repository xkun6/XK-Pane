//go:build windows

package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/procx"
)

// trashPath 用 PowerShell 调 Microsoft.VisualBasic.FileIO.FileSystem 把 path 送进回收站
// （SendToRecycleBin = 可恢复，非永久删除）。目录/文件分别用 DeleteDirectory/DeleteFile。
// 路径里的单引号转义为 ''（PowerShell 单引号字符串转义），防注入。经 procx 拉子进程（铁律禁 exec.Cmd 直用）。
func trashPath(ctx context.Context, path string) error {
	win := filepath.FromSlash(path)
	method := "DeleteFile"
	if fi, err := os.Stat(win); err == nil && fi.IsDir() {
		method = "DeleteDirectory"
	}
	esc := strings.ReplaceAll(win, "'", "''")
	script := fmt.Sprintf(
		"Add-Type -AssemblyName Microsoft.VisualBasic; "+
			"[Microsoft.VisualBasic.FileIO.FileSystem]::%s('%s','OnlyErrorDialogs','SendToRecycleBin')",
		method, esc,
	)

	var stderr strings.Builder
	err := procx.Run(ctx, procx.Spec{
		Name:          "powershell",
		Args:          []string{"-NoProfile", "-NonInteractive", "-Command", script},
		CaptureOutput: true,
		OnStderr:      func(line string) { stderr.WriteString(line); stderr.WriteByte('\n') },
	})
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("trash: %s", msg)
		}
		return err
	}
	return nil
}
