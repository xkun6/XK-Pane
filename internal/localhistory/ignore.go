package localhistory

import (
	"path/filepath"
	"strings"
)

// PathMatchesIgnore 判断项目内相对路径 relPath 是否匹配任一忽略规则 patterns（HistoryConfig.IgnorePatterns）。
// 这是忽略匹配的**单一实现**：watcher 保存前过滤（lhwatcher.ShouldIgnore 在此之上加 .xkpanes 自噬硬过滤）
// 与 Cleanup 清理「已匹配忽略规则的历史版本」共用同一套语义，避免两份副本漂移。
//
// 不含 .xkpanes 自噬硬过滤（那是 watcher 侧的关注点；Cleanup 也不该把 .xkpanes 当业务文件，但默认
// 忽略规则已含 .xkpanes/** 故无碍）。
func PathMatchesIgnore(relPath string, patterns []string) bool {
	p := filepath.ToSlash(relPath)
	for _, pat := range patterns {
		if matchIgnorePattern(p, pat) {
			return true
		}
	}
	return false
}

// matchIgnorePattern 基于 cc-pane matches_pattern，但**有意改进** "dir/**" 的匹配深度
// （cc-pane 仅根前缀，对嵌套依赖目录会漏匹配——Go 根 + frontend/node_modules 即踩中）。
//   - "dir/**" 且 dir 为单段目录名（node_modules/.git/dist/build/target/.xkpanes 等）：匹配**任意
//     深度**的该路径段（path 的任一段 == dir），对齐 gitignore「无前导斜杠 = 任意层级」；同时
//     "target2/x" 不会误匹配 "target/**"（段不相等）。
//   - "dir/**" 且 dir 含 "/"（多段前缀）：根相对前缀。
//   - "*.ext"：扩展名后缀。
//   - 其它：精确相等或目录前缀。
func matchIgnorePattern(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	switch {
	case strings.HasSuffix(pattern, "/**"):
		dir := strings.TrimSuffix(pattern, "/**")
		if dir != "" && !strings.Contains(dir, "/") {
			for _, seg := range strings.Split(path, "/") {
				if seg == dir {
					return true
				}
			}
			return false
		}
		return path == dir || strings.HasPrefix(path, dir+"/")
	case strings.HasPrefix(pattern, "*."):
		ext := strings.TrimPrefix(pattern, "*") // ".log"
		return strings.HasSuffix(path, ext)
	default:
		return path == pattern || strings.HasPrefix(path, pattern+"/")
	}
}
