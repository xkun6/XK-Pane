//go:build !windows

package file

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"xkpane/internal/utils/procx"
)

// revealInOS 非 Windows 实现：macOS 用 `open`（文件 -R 选中、目录直接打开），
// 其余（Linux 等）用 `xdg-open` 打开所在目录（无跨发行版统一的「选中文件」语义）。
func revealInOS(ctx context.Context, path string) error {
	if runtime.GOOS == "darwin" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return procx.Run(ctx, procx.Spec{Name: "open", Args: []string{"-R", path}})
		}
		return procx.Run(ctx, procx.Spec{Name: "open", Args: []string{path}})
	}
	dir := path
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		dir = filepath.Dir(path)
	}
	return procx.Run(ctx, procx.Spec{Name: "xdg-open", Args: []string{dir}})
}
