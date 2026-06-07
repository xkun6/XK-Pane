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

func TestParseAskUserQuestion(t *testing.T) {
	body := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_q","name":"AskUserQuestion","input":{"questions":[{"question":"选哪个方案？","header":"方案","multiSelect":false,"options":[{"label":"A 方案","description":"简单"},{"label":"B 方案","description":"灵活"}]},{"question":"附带哪些？","header":"附带","multiSelect":true,"options":[{"label":"测试"},{"label":"文档"}]}]}}]},"uuid":"aq"}`
	res := Parse(writeSample(t, body), 0)
	if len(res.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1: %+v", len(res.Events), res.Events)
	}
	ev := res.Events[0]
	if ev.Kind != KindToolUse || ev.ToolName != askToolName {
		t.Fatalf("kind/name mismatch: %+v", ev)
	}
	if len(ev.Questions) != 2 {
		t.Fatalf("Questions = %d, want 2: %+v", len(ev.Questions), ev.Questions)
	}
	q0 := ev.Questions[0]
	if q0.Header != "方案" || q0.Question != "选哪个方案？" || q0.MultiSelect {
		t.Fatalf("q0 fields mismatch: %+v", q0)
	}
	if len(q0.Options) != 2 || q0.Options[0].Label != "A 方案" || q0.Options[1].Description != "灵活" {
		t.Fatalf("q0 options mismatch: %+v", q0.Options)
	}
	q1 := ev.Questions[1]
	if !q1.MultiSelect || len(q1.Options) != 2 || q1.Options[0].Label != "测试" {
		t.Fatalf("q1 (multiSelect) mismatch: %+v", q1)
	}
}

// 非 AskUserQuestion 工具不应被结构化（Questions 为空，仍走裸 ToolInput）。
func TestParseNonAskToolHasNoQuestions(t *testing.T) {
	res := Parse(writeSample(t, sample), 0)
	tu := res.Events[3] // Glob tool_use
	if tu.ToolName != "Glob" {
		t.Fatalf("expected Glob tool_use at index 3, got %+v", tu)
	}
	if len(tu.Questions) != 0 {
		t.Fatalf("non-Ask tool should have no Questions, got %+v", tu.Questions)
	}
}

// Edit 工具结果：行顶层 toolUseResult.structuredPatch → Detail（差异行 + 增删计数 + filePath）。
func TestParseEditToolResultDetail(t *testing.T) {
	body := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_e","type":"tool_result","content":"ok"}]},"toolUseResult":{"filePath":"/repo/a.go","structuredPatch":[{"oldStart":1,"oldLines":3,"newStart":1,"newLines":3,"lines":[" ctx"," before","-old line","+new line","+extra line"]}]},"uuid":"ue"}`
	res := Parse(writeSample(t, body), 0)
	if len(res.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1: %+v", len(res.Events), res.Events)
	}
	d := res.Events[0].Detail
	if d == nil || d.Kind != "edit" {
		t.Fatalf("Detail mismatch: %+v", d)
	}
	if d.FilePath != "/repo/a.go" {
		t.Fatalf("FilePath = %q", d.FilePath)
	}
	if d.Added != 2 || d.Removed != 1 {
		t.Fatalf("Added/Removed = %d/%d, want 2/1", d.Added, d.Removed)
	}
	if len(d.Diff) != 5 || d.Diff[2] != "-old line" || d.Truncated {
		t.Fatalf("Diff mismatch: %+v truncated=%v", d.Diff, d.Truncated)
	}
}

// Bash 工具结果：行顶层 toolUseResult.{stdout,stderr} → Detail（输出分离，stderr 供 danger 渲染）。
func TestParseBashToolResultDetail(t *testing.T) {
	body := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_b","type":"tool_result","content":"merged"}]},"toolUseResult":{"stdout":"hello\nworld","stderr":"warn: x","interrupted":false,"isImage":false},"uuid":"ub"}`
	res := Parse(writeSample(t, body), 0)
	if len(res.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(res.Events))
	}
	d := res.Events[0].Detail
	if d == nil || d.Kind != "bash" {
		t.Fatalf("Detail mismatch: %+v", d)
	}
	if d.Stdout != "hello\nworld" || d.Stderr != "warn: x" {
		t.Fatalf("stdout/stderr mismatch: %q / %q", d.Stdout, d.Stderr)
	}
}

// 一行含多个 tool_result 时不能误挂顶层 toolUseResult（无法确定归属）→ Detail 为 nil，回退 Text。
func TestParseMultiToolResultNoDetail(t *testing.T) {
	body := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"t1","type":"tool_result","content":"a"},{"tool_use_id":"t2","type":"tool_result","content":"b"}]},"toolUseResult":{"stdout":"x","stderr":""},"uuid":"um"}`
	res := Parse(writeSample(t, body), 0)
	if len(res.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(res.Events))
	}
	for i, ev := range res.Events {
		if ev.Detail != nil {
			t.Fatalf("Events[%d].Detail should be nil for multi tool_result line, got %+v", i, ev.Detail)
		}
	}
}

// 非 Edit/Bash 的 toolUseResult（无 structuredPatch / stdout）→ Detail 为 nil，前端回退 Text。
func TestParseOtherToolResultNoDetail(t *testing.T) {
	body := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"tr","type":"tool_result","content":"file body"}]},"toolUseResult":{"type":"text","file":{"filePath":"/x","numLines":10}},"uuid":"ur"}`
	res := Parse(writeSample(t, body), 0)
	if len(res.Events) != 1 || res.Events[0].Detail != nil {
		t.Fatalf("expected single tool_result with nil Detail, got %+v", res.Events)
	}
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mred\x1b[0m plain"
	if got := stripANSI(in); got != "red plain" {
		t.Fatalf("stripANSI = %q, want %q", got, "red plain")
	}
}
