//go:build windows

package file

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"xkpane/internal/utils/procx"
)

// revealInOS 用 Windows 资源管理器定位 path：目录直接打开，文件用 /select 选中。
func revealInOS(ctx context.Context, path string) error {
	args := []string{path}
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		args = []string{"/select," + path} // 逗号后无空格，定位并选中文件
	}
	err := procx.Run(ctx, procx.Spec{Name: "explorer.exe", Args: args})
	// explorer.exe 即使成功也常返回 exit code 1，忽略 ExitError；启动失败（找不到）仍上报。
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return nil
	}
	return err
}
