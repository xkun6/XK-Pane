package gitsvc

import (
	"strings"
	"testing"

	"xkpane/internal/localhistory"
)

// 守门：解析典型 unified patch（1 改 1 删 1 增）→ hunk/行类型/行号/统计正确。
func TestParseUnifiedDiffBasic(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/foo.go b/foo.go",
		"index 1111111..2222222 100644",
		"--- a/foo.go",
		"+++ b/foo.go",
		"@@ -1,4 +1,4 @@",
		" package main",
		" ",
		"-const x = 1",
		"+const x = 2",
		" var y = 3",
		"",
	}, "\n")

	d := parseUnifiedDiff(patch)
	if d.IsBinary || d.Truncated {
		t.Fatalf("unexpected flags: %+v", d)
	}
	if len(d.Hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(d.Hunks))
	}
	h := d.Hunks[0]
	if h.OldStart != 1 || h.NewStart != 1 {
		t.Errorf("hunk start = old %d new %d, want 1/1", h.OldStart, h.NewStart)
	}
	// 4 行老侧（2 context + 1 delete + 1 context）/ 4 行新侧（2 context + 1 insert + 1 context）
	if h.OldCount != 4 || h.NewCount != 4 {
		t.Errorf("hunk count = old %d new %d, want 4/4", h.OldCount, h.NewCount)
	}
	if d.Stats.Additions != 1 || d.Stats.Deletions != 1 {
		t.Errorf("stats = +%d -%d, want +1 -1", d.Stats.Additions, d.Stats.Deletions)
	}
	if len(h.Lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(h.Lines))
	}
	// 第 3 行（idx 2）应为 delete "const x = 1"，老行号=3，无新行号。
	del := h.Lines[2]
	if del.ChangeType != localhistory.ChangeDelete || del.Content != "const x = 1" {
		t.Errorf("line[2] = %+v, want delete 'const x = 1'", del)
	}
	if del.OldLineNo == nil || *del.OldLineNo != 3 || del.NewLineNo != nil {
		t.Errorf("delete line numbers wrong: %+v", del)
	}
	ins := h.Lines[3]
	if ins.ChangeType != localhistory.ChangeInsert || ins.NewLineNo == nil || *ins.NewLineNo != 3 || ins.OldLineNo != nil {
		t.Errorf("insert line wrong: %+v", ins)
	}
}

// 守门：二进制 diff → IsBinary，无 hunk。
func TestParseUnifiedDiffBinary(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/logo.png b/logo.png",
		"index 1111111..2222222 100644",
		"Binary files a/logo.png and b/logo.png differ",
	}, "\n")
	d := parseUnifiedDiff(patch)
	if !d.IsBinary {
		t.Fatalf("want IsBinary, got %+v", d)
	}
	if len(d.Hunks) != 0 {
		t.Errorf("binary should have no hunks, got %d", len(d.Hunks))
	}
}

// 守门：空 patch（无变更）→ 空 hunks，非二进制。
func TestParseUnifiedDiffEmpty(t *testing.T) {
	d := parseUnifiedDiff("")
	if d.IsBinary || len(d.Hunks) != 0 {
		t.Errorf("empty patch unexpected: %+v", d)
	}
}

// 守门：hunk 头解析（含/不含 count）。
func TestParseHunkHeader(t *testing.T) {
	os1, oc1, ns1, nc1, ok1 := parseHunkHeader("@@ -12,7 +14,8 @@ func main() {")
	if !ok1 || os1 != 12 || oc1 != 7 || ns1 != 14 || nc1 != 8 {
		t.Errorf("header1 = %d,%d %d,%d ok=%v", os1, oc1, ns1, nc1, ok1)
	}
	// 单行 hunk：count 省略 = 1
	os2, oc2, ns2, nc2, ok2 := parseHunkHeader("@@ -5 +5 @@")
	if !ok2 || os2 != 5 || oc2 != 1 || ns2 != 5 || nc2 != 1 {
		t.Errorf("header2 = %d,%d %d,%d ok=%v", os2, oc2, ns2, nc2, ok2)
	}
}
