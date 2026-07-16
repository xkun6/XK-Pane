package cliupdate

import (
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[A-Za-z0-9.]+)?`)

// extractVersion 从命令输出中提取首个语义化版本号；无匹配返回 ""。
func extractVersion(s string) string {
	return versionPattern.FindString(s)
}

// compareSemver 按 major.minor.patch 逐段数字比较；忽略预发布后缀；a>b→1，a<b→-1，相等→0。
// 任一侧解析失败按 0 段处理（宽容比较，探测场景不苛求）。
func compareSemver(a, b string) int {
	pa := parseSemverCore(a)
	pb := parseSemverCore(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// parseSemverCore 提取 major.minor.patch 三段数字（忽略预发布/build 后缀，即第一个
// '-' 或 '+' 之后的部分）；无法解析成数字的段按 0 处理。
func parseSemverCore(v string) [3]int {
	core := v
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	var out [3]int
	parts := strings.SplitN(core, ".", 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		out[i] = n
	}
	return out
}

// lastLines 取文本尾部最多 n 行（去掉首尾空白行）。
func lastLines(s string, n int) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
