package lhwatcher

import "testing"

func TestShouldIgnoreDefaults(t *testing.T) {
	// 默认忽略规则（对齐 localhistory.DefaultHistoryConfig）。
	patterns := []string{
		"node_modules/**", ".git/**", "target/**", "dist/**", "build/**",
		"*.log", "*.lock", ".xkpanes/**",
	}
	cases := []struct {
		path string
		want bool
	}{
		// 目录 dir/** 精确前缀
		{"node_modules", true},
		{"node_modules/react/index.js", true},
		{".git", true},
		{".git/HEAD", true},
		{"target/debug/app", true},
		{"dist/bundle.js", true},
		{"build/out.o", true},
		// 不应误匹配相邻同前缀目录（cc-pane 第 541-543 行的精确性保证）
		{"target2/foo", false},
		{"node_modules_backup/x", false},
		// 扩展名
		{"server.log", true},
		{"logs/app.log", true},
		{"pnpm-lock.lock", true},
		{"package-lock.json", false},
		// 普通文件不忽略
		{"src/main.go", false},
		{"README.md", false},
		// .xkpanes 硬过滤（即使 patterns 为空也忽略，见下一个 case）
		{".xkpanes", true},
		{".xkpanes/history/history.db", true},
	}
	for _, c := range cases {
		if got := ShouldIgnore(c.path, patterns); got != c.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestShouldIgnoreHardFiltersXkpanesWithoutPattern(t *testing.T) {
	// 即便忽略规则里没有 .xkpanes/**，也必须硬过滤（防自噬）。
	if !ShouldIgnore(".xkpanes/history/blobs/abc", nil) {
		t.Fatal(".xkpanes must be hard-filtered even without ignore patterns")
	}
	if !ShouldIgnore(".xkpanes", []string{"node_modules/**"}) {
		t.Fatal(".xkpanes (exact) must be hard-filtered")
	}
}

func TestMatchesPatternBackslashInput(t *testing.T) {
	// Windows 反斜杠相对路径应被归一为正斜杠后匹配。
	if !ShouldIgnore(`node_modules\pkg\a.js`, []string{"node_modules/**"}) {
		t.Fatal("backslash path under node_modules should be ignored")
	}
}
