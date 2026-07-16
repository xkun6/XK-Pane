package cliupdate

import "testing"

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"claude 版本行", "2.1.195 (Claude Code)", "2.1.195"},
		{"带预发布后缀", "claude v1.0.0-beta.1", "1.0.0-beta.1"},
		{"无版本号", "no version here", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractVersion(c.input); got != c.want {
				t.Errorf("extractVersion(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"a 更大", "2.0.0", "1.9.9", 1},
		{"a 更小", "1.9.9", "2.0.0", -1},
		{"相等", "1.2.3", "1.2.3", 0},
		{"忽略预发布后缀", "1.0.0-beta.1", "1.0.0", 0},
		{"位数不同不能按字符串比", "1.10.0", "1.9.9", 1},
		{"解析失败按 0 段处理", "abc", "0.0.1", -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compareSemver(c.a, c.b); got != c.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestLastLines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"不足 n 行原样返回", "a\nb", 5, "a\nb"},
		{"超过 n 行只保留尾部", "a\nb\nc\nd\ne", 2, "d\ne"},
		{"首尾空行被去掉", "\n\na\nb\n\n", 5, "a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastLines(c.input, c.n); got != c.want {
				t.Errorf("lastLines(%q, %d) = %q, want %q", c.input, c.n, got, c.want)
			}
		})
	}
}
