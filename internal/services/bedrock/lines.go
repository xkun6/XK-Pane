package bedrock

import "strings"

// splitLines 复刻 Rust str::lines() 语义，供 guide read（按行截断/计数）与 review（P6f-5 行扫描）
// 共用。Go 的 strings.Split(s,"\n") 会为结尾换行多产一个尾随空串且不剥 \r，会让行号/总行数偏 1，
// 故必须封装。
//
// Rust lines() = split_terminator('\n') + 每行剥尾随 \r：
//   - ""        → []        （空串产空迭代器）
//   - "\n"      → [""]
//   - "a\n"     → ["a"]
//   - "a\nb\n"  → ["a","b"]
//   - "a\nb"    → ["a","b"]
//   - "a\n\nb"  → ["a","","b"]
//   - "a\r\nb"  → ["a","b"] （\r\n 的 \r 被剥）
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	// split_terminator：结尾换行不产尾随空行 → 丢弃末尾那一个空串。
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "\r")
	}
	return parts
}
