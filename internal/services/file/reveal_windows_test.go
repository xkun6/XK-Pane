//go:build windows

package file

import (
	"reflect"
	"testing"
)

// explorer.exe 只认反斜杠：正斜杠必须先转换，文件走 /select 选中、目录直接打开。
func TestExplorerArgs(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		isDir bool
		want  []string
	}{
		{"dir backslash", `J:\a\b`, true, []string{`J:\a\b`}},
		{"dir forwardslash", "J:/a/b", true, []string{`J:\a\b`}},
		{"file backslash", `J:\a\b.txt`, false, []string{`/select,J:\a\b.txt`}},
		{"file forwardslash", "J:/a/b.txt", false, []string{`/select,J:\a\b.txt`}},
	}
	for _, c := range cases {
		if got := explorerArgs(c.path, c.isDir); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: explorerArgs(%q, %v) = %v, want %v", c.name, c.path, c.isDir, got, c.want)
		}
	}
}
