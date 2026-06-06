//go:build windows

package file

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"xkpane/internal/utils/procx"
)

// explorerArgs 构造 explorer.exe 的参数：目录直接打开，文件用 /select 选中。
// explorer.exe 只认反斜杠：正斜杠路径既无法 /select 选中、也不会打开目标目录
// （实测打开默认位置而非目标）。仅对传给 explorer 的实参转分隔符，不动存储值（D7）。
func explorerArgs(path string, isDir bool) []string {
	winPath := filepath.FromSlash(path)
	if isDir {
		return []string{winPath}
	}
	return []string{"/select," + winPath} // 逗号后无空格，定位并选中文件
}

// revealInOS 用 Windows 资源管理器定位 path：目录直接打开，文件用 /select 选中。
func revealInOS(ctx context.Context, path string) error {
	isDir := true // 调用方（file.go RevealInOS）已 Stat 过 path 存在；Stat 失败兜底当目录
	if fi, err := os.Stat(path); err == nil {
		isDir = fi.IsDir()
	}
	// ShowWindow=true：否则 procx 默认的 HideWindow(SW_HIDE) 会把弹出的资源管理器窗口一并隐藏。
	err := procx.Run(ctx, procx.Spec{Name: "explorer.exe", Args: explorerArgs(path, isDir), ShowWindow: true})
	// explorer.exe 即使成功也常返回 exit code 1，忽略 ExitError；启动失败（找不到）仍上报。
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return nil
	}
	return err
}
