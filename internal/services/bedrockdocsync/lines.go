package bedrockdocsync

import "strings"

// splitLines 复刻 Rust str::lines() 语义（与 bedrock/lines.go 同逻辑，独立一份以免跨包导出私有件）。
// Go 的 strings.Split(s,"\n") 会为结尾换行多产一个尾随空串且不剥 \r，会让行号/总行数偏 1，故封装。
//
// Rust lines() = split_terminator('\n') + 每行剥尾随 \r：
//   - ""       → []
//   - "\n"     → [""]
//   - "a\n"    → ["a"]
//   - "a\nb\n" → ["a","b"]
//   - "a\nb"   → ["a","b"]
//   - "a\n\nb" → ["a","","b"]
//   - "a\r\nb" → ["a","b"]
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "\r")
	}
	return parts
}
