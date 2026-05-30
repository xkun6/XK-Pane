package sessionscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeClaudeProjectPath(t *testing.T) {
	cases := map[string]string{
		`I:\vms-workspace`:         "I--vms-workspace",
		`D:\04_workspace_rust\vms`: "D--04-workspace-rust-vms",
		`/home/user/my_project`:    "-home-user-my-project",
	}
	for in, want := range cases {
		if got := encodeClaudeProjectPath(in); got != want {
			t.Errorf("encode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeComparePath(t *testing.T) {
	cases := map[string]string{
		`C:\Foo\Bar\`:   "c:/foo/bar",
		`/tmp/Project/`: "/tmp/project",
		`a/b`:           "a/b",
	}
	for in, want := range cases {
		if got := normalizeComparePath(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncateDesc(t *testing.T) {
	short := "hello"
	if got := truncateDesc(short); got != short {
		t.Errorf("truncate(short) = %q, want %q", got, short)
	}
	long := make([]rune, 100)
	for i := range long {
		long[i] = 'a'
	}
	got := truncateDesc(string(long))
	if len([]rune(got)) != 83 { // 80 + "..."
		t.Errorf("truncate(long) rune len = %d, want 83", len([]rune(got)))
	}
}

func writeFile(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	data := ""
	for _, l := range lines {
		data += l + "\n"
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractClaudeDescription_StringContent(t *testing.T) {
	p := writeFile(t,
		`{"type":"summary"}`,
		`{"type":"user","data":{"x":1},"message":{"content":"应跳过含 data 的 progress"}}`,
		`{"type":"user","message":{"content":"fix the resume passthrough chain"}}`,
	)
	if got := extractClaudeDescription(p); got != "fix the resume passthrough chain" {
		t.Errorf("got %q", got)
	}
}

func TestExtractClaudeDescription_ArrayContent(t *testing.T) {
	p := writeFile(t,
		`{"type":"user","message":{"content":[{"type":"tool_result","text":"含 tool_use_id 应跳"},{"type":"text","text":"actual user prompt here"}]}}`,
	)
	if got := extractClaudeDescription(p); got != "actual user prompt here" {
		t.Errorf("got %q", got)
	}
}

func TestParseCodexMetaAndDescription(t *testing.T) {
	p := writeFile(t,
		`{"type":"session_meta","payload":{"id":"session-42","cwd":"/tmp/project"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /tmp/project"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the failing codex resume flow"}]}}`,
	)
	id, cwd, ok := parseCodexMeta(p)
	if !ok || id != "session-42" || cwd != "/tmp/project" {
		t.Fatalf("meta = %q,%q,%v", id, cwd, ok)
	}
	if got := extractCodexDescription(p); got != "fix the failing codex resume flow" {
		t.Errorf("desc = %q", got)
	}
}
