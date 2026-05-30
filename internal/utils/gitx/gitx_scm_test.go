package gitx

import (
	"strings"
	"testing"
)

// 守门：porcelain 状态行解析覆盖暂存/未暂存/二者兼有/未跟踪/重命名/冲突。
func TestParsePorcelainStatus(t *testing.T) {
	// 注意：每行 = XY + 空格 + 路径。重命名为 "R  new <- ..."？git 实际输出 "orig -> dest"。
	stdout := strings.Join([]string{
		"M  staged.go",       // 仅暂存修改
		" M unstaged.go",     // 仅工作树修改
		"MM both.go",         // 暂存 + 再改
		"A  added.go",        // 暂存新增
		"D  deleted.go",      // 暂存删除
		" D wt-deleted.go",   // 工作树删除（未暂存）
		"?? untracked.txt",   // 未跟踪
		"R  old.go -> new.go", // 暂存重命名
		"UU conflict.go",     // 冲突
	}, "\n")

	got := parsePorcelainStatus(stdout)
	if len(got) != 9 {
		t.Fatalf("got %d entries, want 9: %+v", len(got), got)
	}

	check := func(i int, path string, staged, unstaged, untracked, conflicted bool) {
		t.Helper()
		fs := got[i]
		if fs.Path != path {
			t.Errorf("[%d] path = %q, want %q", i, fs.Path, path)
		}
		if fs.Staged != staged || fs.Unstaged != unstaged || fs.Untracked != untracked || fs.Conflicted != conflicted {
			t.Errorf("[%d] %q flags = staged:%v unstaged:%v untracked:%v conflicted:%v, want %v/%v/%v/%v",
				i, path, fs.Staged, fs.Unstaged, fs.Untracked, fs.Conflicted, staged, unstaged, untracked, conflicted)
		}
	}

	check(0, "staged.go", true, false, false, false)
	check(1, "unstaged.go", false, true, false, false)
	check(2, "both.go", true, true, false, false)
	check(3, "added.go", true, false, false, false)
	check(4, "deleted.go", true, false, false, false)
	check(5, "wt-deleted.go", false, true, false, false)
	check(6, "untracked.txt", false, true, true, false)
	check(7, "new.go", true, false, false, false)
	check(8, "conflict.go", false, true, false, true)

	// 重命名：来源路径
	if got[7].OrigPath != "old.go" {
		t.Errorf("rename origPath = %q, want old.go", got[7].OrigPath)
	}
}

// 守门：含引号的特殊路径 best-effort 还原。
func TestUnquotePath(t *testing.T) {
	if got := unquotePath("plain.go"); got != "plain.go" {
		t.Errorf("plain = %q", got)
	}
	if got := unquotePath(`"with space.go"`); got != "with space.go" {
		t.Errorf("quoted = %q", got)
	}
}
