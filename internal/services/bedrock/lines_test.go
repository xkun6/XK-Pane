package bedrock

import (
	"reflect"
	"testing"
)

// splitLines 必须复刻 Rust str::lines() 语义（split_terminator + 去尾 \r）。
func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"\n", []string{""}},
		{"a\n", []string{"a"}},
		{"a\nb\n", []string{"a", "b"}},
		{"a\nb", []string{"a", "b"}},
		{"a\n\nb", []string{"a", "", "b"}},
		{"a\n\n", []string{"a", ""}},
		{"a\r\nb", []string{"a", "b"}},
		{"a\r\n", []string{"a"}},
		{"a\rb", []string{"a\rb"}}, // 孤立 \r（非 \r\n）不分行也不剥
	}
	for _, c := range cases {
		got := splitLines(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitLines(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}
