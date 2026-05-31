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

// matchesPattern 基于 cc-pane history_service.rs matches_pattern（529-548），但**有意改进** "dir/**"
// 的匹配深度（cc-pane 仅根前缀，对嵌套依赖目录会漏匹配——本仓 Go 根 + frontend/node_modules 即踩中）。
// 支持三种形式：
//   - "dir/**" 且 dir 为单段目录名（node_modules / .git / dist / build / target / .xkpanes 等）：
//     匹配**任意深度**的该路径段（即 path 的任一路径段 == dir）。对齐 gitignore「无前导斜杠 = 任意层级」
//     语义，使 "frontend/node_modules/..." 也被忽略；同时 "target2/x" 不会误匹配 "target/**"（段不相等）。
//   - "dir/**" 且 dir 含 "/"（多段前缀，如 "src/gen/**"）：保持根相对前缀语义（path==dir 或 path 以 dir+"/" 开头）。
//   - "*.ext"：扩展名后缀匹配。
//   - 其它：精确相等，或作为目录前缀（path 以 pattern+"/" 开头）。
// path 已是正斜杠形式；pattern 也归一为正斜杠。
func matchesPattern(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	switch {
	case strings.HasSuffix(pattern, "/**"):
		dir := strings.TrimSuffix(pattern, "/**")
		if dir != "" && !strings.Contains(dir, "/") {
			// 单段目录名：匹配任意深度的该目录组件。
			for _, seg := range strings.Split(path, "/") {
				if seg == dir {
					return true
				}
			}
			return false
		}
		// 多段前缀：根相对。
		return path == dir || strings.HasPrefix(path, dir+"/")
	case strings.HasPrefix(pattern, "*."):
		ext := strings.TrimPrefix(pattern, "*") // ".log"
		return strings.HasSuffix(path, ext)
	default:
		return path == pattern || strings.HasPrefix(path, pattern+"/")
	}
}
