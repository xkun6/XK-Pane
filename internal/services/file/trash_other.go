//go:build !windows

package file

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"xkpane/internal/utils/procx"
)

var errTrashUnavailable = errors.New("no trash utility available (install gio / trash-cli)")

// trashPath 非 Windows 实现：macOS 让 Finder delete（进废纸篓，可恢复）；
// Linux 依次尝试 gio trash / trash-put / trash（freedesktop 回收站工具，按可用性回退）。
func trashPath(ctx context.Context, path string) error {
	if runtime.GOOS == "darwin" {
		esc := strings.ReplaceAll(path, `"`, `\"`)
		script := fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, esc)
		return procx.Run(ctx, procx.Spec{Name: "osascript", Args: []string{"-e", script}})
	}
	candidates := [][]string{
		{"gio", "trash", path},
		{"trash-put", path},
		{"trash", path},
	}
	for _, c := range candidates {
		if err := procx.Run(ctx, procx.Spec{Name: c[0], Args: c[1:]}); err == nil {
			return nil
		}
	}
	return errTrashUnavailable
}
