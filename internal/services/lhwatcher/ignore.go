package lhwatcher

import (
	"path/filepath"
	"strings"

	"xkpane/internal/localhistory"
)

// xkpanesDir 是本地历史自身的存储目录名（每项目 <proj>/.xkpanes/）。watcher 必须硬过滤它，
// 否则保存版本写 .xkpanes/ 会再触发文件事件，形成「自噬」死循环。
const xkpanesDir = ".xkpanes"

// ShouldIgnore 判断项目内相对路径 relPath 是否应被忽略（不记历史 / 不监控）。
// = 硬过滤 .xkpanes/（防自噬）+ 复用域层 localhistory.PathMatchesIgnore（与 Cleanup 清理共用同一
// 套匹配语义，单一实现，避免漂移）。relPath 用项目内相对路径（正/反斜杠均可，内部归一为正斜杠）。
func ShouldIgnore(relPath string, patterns []string) bool {
	p := filepath.ToSlash(relPath)
	if p == xkpanesDir || strings.HasPrefix(p, xkpanesDir+"/") {
		return true
	}
	return localhistory.PathMatchesIgnore(p, patterns)
}
