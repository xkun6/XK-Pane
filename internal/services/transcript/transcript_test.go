package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

// 真实 Claude JSONL 形态的最小样本（对照 ~/.claude/projects/<cwd>/<id>.jsonl）。
const sample = `{"type":"permission-mode","permissionMode":"bypassPermissions","sessionId":"x"}
{"type":"user","message":{"role":"user","content":"帮我分析这段代码"},"uuid":"u1","timestamp":"2026-06-01T00:00:00Z"}
{"type":"ai-title","aiTitle":"分析代码","sessionId":"x"}
{"type":"assistant","message":{"model":"claude-opus-4-8","role":"assistant","content":[{"type":"thinking","thinking":"先看结构","signature":"sig"}]},"uuid":"a1","timestamp":"2026-06-01T00:00:01Z"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"我来分析。"}]},"uuid":"a2"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Glob","input":{"pattern":"**/*.go"}}]},"uuid":"a3"}
{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_1","type":"tool_result","content":"main.go\napp.go"}]},"uuid":"u2"}
{"type":"user","message":{"role":"user","content":"[Request interrupted by user]"},"uuid":"u3"}`

func writeSample(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	return p
}

func TestParseFull(t *testing.T) {
	res := Parse(writeSample(t, sample), 0)

	if res.Total != 8 {
		t.Fatalf("Total = %d, want 8 (行数)", res.Total)
	}
	if res.Title != "分析代码" {
		t.Fatalf("Title = %q, want 分析代码", res.Title)
	}

	// 期望事件序列：user / thinking / assistant / tool_use / tool_result
	// （permission-mode、ai-title、[Request interrupted] 均被过滤）
	wantKinds := []string{KindUser, KindThinking, KindAssistant, KindToolUse, KindToolResult}
	if len(res.Events) != len(wantKinds) {
		t.Fatalf("len(Events) = %d, want %d: %+v", len(res.Events), len(wantKinds), res.Events)
	}
	for i, k := range wantKinds {
		if res.Events[i].Kind != k {
			t.Fatalf("Events[%d].Kind = %q, want %q", i, res.Events[i].Kind, k)
		}
	}

	tu := res.Events[3]
	if tu.ToolName != "Glob" || tu.ToolUseID != "toolu_1" {
		t.Fatalf("tool_use mismatch: %+v", tu)
	}
	if tu.ToolInput == "" {
		t.Fatalf("tool_use input empty")
	}
	tr := res.Events[4]
	if tr.ToolUseID != "toolu_1" || tr.Text == "" {
		t.Fatalf("tool_result mismatch: %+v", tr)
	}
}

func TestParseIncremental(t *testing.T) {
	full := Parse(writeSample(t, sample), 0)
	// 从游标 6 起（已消费前 6 行：跳过到 a3 的下一行），只应拿到 tool_result（u2 在行索引 6）。
	inc := Parse(writeSample(t, sample), 6)
	if inc.Total != full.Total {
		t.Fatalf("incremental Total = %d, want %d", inc.Total, full.Total)
	}
	// 行 6 = tool_result（u2），行 7 = [Request interrupted]（被过滤）→ 仅 1 个事件
	if len(inc.Events) != 1 || inc.Events[0].Kind != KindToolResult {
		t.Fatalf("incremental events = %+v, want single tool_result", inc.Events)
	}
}

func TestParseMissingFile(t *testing.T) {
	res := Parse(filepath.Join(t.TempDir(), "nope.jsonl"), 0)
	if res.Events == nil {
		t.Fatalf("Events must be non-nil empty slice (JSON [])")
	}
	if len(res.Events) != 0 || res.Total != 0 {
		t.Fatalf("missing file should yield empty result, got %+v", res)
	}
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mred\x1b[0m plain"
	if got := stripANSI(in); got != "red plain" {
		t.Fatalf("stripANSI = %q, want %q", got, "red plain")
	}
}
