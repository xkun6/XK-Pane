package lhwatcher

import (
	"path/filepath"
	"strings"
)

// xkpanesDir 是本地历史自身的存储目录名（每项目 <proj>/.xkpanes/）。watcher 必须硬过滤它，
// 否则保存版本写 .xkpanes/ 会再触发文件事件，形成「自噬」死循环。
const xkpanesDir = ".xkpanes"

// ShouldIgnore 判断项目内相对路径 relPath 是否应被忽略（不记历史 / 不监控）。
// = 硬过滤 .xkpanes/（防自噬）+ 逐条匹配 patterns（来自 HistoryConfig.IgnorePatterns）。
// relPath 用项目内相对路径（正斜杠或反斜杠均可，内部归一为正斜杠）。
func ShouldIgnore(relPath string, patterns []string) bool {
	p := filepath.ToSlash(relPath)
	if p == xkpanesDir || strings.HasPrefix(p, xkpanesDir+"/") {
		return true
	}
	for _, pat := range patterns {
		if matchesPattern(p, pat) {
			return true
		}
	}
	return false
}

// matchesPattern 忠实移植 cc-pane history_service.rs matches_pattern（529-548）。支持三种形式：
//   - "dir/**"：精确目录前缀 —— path==dir 或 path 以 dir+"/" 开头（"target2/x" 不会误匹配 "target/**"）。
//   - "*.ext"：扩展名后缀匹配。
//   - 其它：精确相等，或作为目录前缀（path 以 pattern+"/" 开头）。
// path 已是正斜杠形式；pattern 也归一为正斜杠。
func matchesPattern(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	switch {
	case strings.HasSuffix(pattern, "/**"):
		dir := strings.TrimSuffix(pattern, "/**")
		return path == dir || strings.HasPrefix(path, dir+"/")
	case strings.HasPrefix(pattern, "*."):
		ext := strings.TrimPrefix(pattern, "*") // ".log"
		return strings.HasSuffix(path, ext)
	default:
		return path == pattern || strings.HasPrefix(path, pattern+"/")
	}
}
