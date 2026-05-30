package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findLine 在所有 hunk 行里找首个 (changeType, content) 匹配的行。
func findLine(r DiffResult, ct DiffChangeType, content string) *DiffLine {
	for hi := range r.Hunks {
		for li := range r.Hunks[hi].Lines {
			l := &r.Hunks[hi].Lines[li]
			if l.ChangeType == ct && l.Content == content {
				return l
			}
		}
	}
	return nil
}

func TestComputeDiffReplaceWithInline(t *testing.T) {
	r := ComputeDiff("a\nb\nc\nd\ne\n", "a\nb\nX\nd\ne\n")
	if r.IsBinary || r.Truncated {
		t.Fatalf("unexpected flags: %+v", r)
	}
	if len(r.Hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(r.Hunks))
	}
	if r.Stats.Additions != 1 || r.Stats.Deletions != 1 || r.Stats.Changes != 1 {
		t.Fatalf("stats = %+v, want +1/-1/~1", r.Stats)
	}
	del := findLine(r, ChangeDelete, "c")
	ins := findLine(r, ChangeInsert, "X")
	if del == nil || ins == nil {
		t.Fatalf("missing del 'c' or ins 'X'")
	}
	if len(del.InlineChanges) == 0 || len(ins.InlineChanges) == 0 {
		t.Errorf("expected inline changes on replaced line")
	}
	// 上下文行带双侧行号。
	if eq := findLine(r, ChangeEqual, "a"); eq == nil || eq.OldLineNo == nil || eq.NewLineNo == nil {
		t.Errorf("context line 'a' missing line numbers")
	}
}

func TestComputeDiffInsertDelete(t *testing.T) {
	// 末尾加一行。
	add := ComputeDiff("a\nb\n", "a\nb\nc\n")
	if add.Stats.Additions != 1 || add.Stats.Deletions != 0 {
		t.Errorf("add stats = %+v, want +1/-0", add.Stats)
	}
	if l := findLine(add, ChangeInsert, "c"); l == nil || l.NewLineNo == nil || l.OldLineNo != nil {
		t.Errorf("inserted 'c' line-number sides wrong: %+v", l)
	}
	// 删一行。
	del := ComputeDiff("a\nb\nc\n", "a\nc\n")
	if del.Stats.Deletions != 1 || del.Stats.Additions != 0 {
		t.Errorf("del stats = %+v, want -1/+0", del.Stats)
	}
	if l := findLine(del, ChangeDelete, "b"); l == nil || l.OldLineNo == nil || l.NewLineNo != nil {
		t.Errorf("deleted 'b' line-number sides wrong: %+v", l)
	}
}

func TestComputeDiffIdentical(t *testing.T) {
	r := ComputeDiff("a\nb\nc\n", "a\nb\nc\n")
	if len(r.Hunks) != 0 || r.Stats.Additions != 0 || r.Stats.Deletions != 0 {
		t.Fatalf("identical diff non-empty: %+v", r)
	}
}

func TestComputeDiffBinary(t *testing.T) {
	r := ComputeDiff("text\x00with null", "other")
	if !r.IsBinary || len(r.Hunks) != 0 {
		t.Fatalf("binary not detected: %+v", r)
	}
}

func TestComputeDiffTruncated(t *testing.T) {
	big := strings.Repeat("x\n", maxDiffLines+1)
	r := ComputeDiff(big, big+"y\n")
	if !r.Truncated || len(r.Hunks) != 0 {
		t.Fatalf("over-MAX not truncated: truncated=%v hunks=%d", r.Truncated, len(r.Hunks))
	}
}

func TestComputeDiffRuneInlinePositions(t *testing.T) {
	// 多字节字符：inline 位置必须按 rune 计数（你→世 在 rune 下标 1，而非字节下标 1..4）。
	r := ComputeDiff("a你b\n", "a世b\n")
	del := findLine(r, ChangeDelete, "a你b")
	ins := findLine(r, ChangeInsert, "a世b")
	if del == nil || ins == nil {
		t.Fatalf("missing replaced line")
	}
	if len(del.InlineChanges) != 1 || del.InlineChanges[0].Start != 1 || del.InlineChanges[0].End != 2 {
		t.Errorf("old inline = %+v, want [{1,2,delete}] (rune positions)", del.InlineChanges)
	}
	if len(ins.InlineChanges) != 1 || ins.InlineChanges[0].Start != 1 || ins.InlineChanges[0].End != 2 {
		t.Errorf("new inline = %+v, want [{1,2,insert}] (rune positions)", ins.InlineChanges)
	}
}

func TestComputeDiffTwoHunks(t *testing.T) {
	var ob, nb strings.Builder
	for i := 1; i <= 30; i++ {
		ob.WriteString("line")
		ob.WriteString(itoa(i))
		ob.WriteByte('\n')
		if i == 2 {
			nb.WriteString("LINE2\n")
		} else if i == 25 {
			nb.WriteString("LINE25\n")
		} else {
			nb.WriteString("line")
			nb.WriteString(itoa(i))
			nb.WriteByte('\n')
		}
	}
	r := ComputeDiff(ob.String(), nb.String())
	// 改动在第 2、25 行，相距远超 2*contextLines → 两个独立 hunk。
	if len(r.Hunks) != 2 {
		t.Fatalf("hunks = %d, want 2 (distant changes)", len(r.Hunks))
	}
	if r.Stats.Additions != 2 || r.Stats.Deletions != 2 {
		t.Errorf("stats = %+v, want +2/-2", r.Stats)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestStoreDiffMethods(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v1, _ := s.SaveVersion(ctx, "a.txt", []byte("a\nb\n"), false, "main", 0)
	v2, _ := s.SaveVersion(ctx, "a.txt", []byte("a\nB\n"), false, "main", 0)

	// 版本间 diff。
	vd, err := s.GetVersionsDiff(ctx, "a.txt", v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("GetVersionsDiff: %v", err)
	}
	if vd.Stats.Additions != 1 || vd.Stats.Deletions != 1 {
		t.Errorf("versions diff stats = %+v, want +1/-1", vd.Stats)
	}

	// 版本 vs 当前工作树文件。
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\nB\nc\n"), 0o644); err != nil {
		t.Fatalf("write working: %v", err)
	}
	wd, err := s.GetVersionDiff(ctx, "a.txt", v1.ID)
	if err != nil {
		t.Fatalf("GetVersionDiff: %v", err)
	}
	// v1("a\nb\n") vs 工作树("a\nB\nc\n")：b→B 改 + 加 c。
	if wd.Stats.Additions != 2 || wd.Stats.Deletions != 1 {
		t.Errorf("working diff stats = %+v, want +2/-1", wd.Stats)
	}
}
