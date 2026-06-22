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
		// 中文路径：每个非字母数字字符（含每个汉字）各换一个 `-`，对照磁盘真相
		// ~/.claude/projects/ 下 Claude CLI 实际生成的目录名。
		`J:\Program\PE\sanjiaozhou\红石洲行动`: "J--Program-PE-sanjiaozhou------",
		`J:\Program\PE\sanjiaozhou\武器`:    "J--Program-PE-sanjiaozhou---",
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

func TestNewestAfter(t *testing.T) {
	sessions := []ResumeSession{
		{ID: "old", ModifiedAt: 100},
		{ID: "mid", ModifiedAt: 200},
		{ID: "new", ModifiedAt: 300},
	}
	// 取窗口内最新一条。
	if got := newestAfter(sessions, 150); got != "new" {
		t.Errorf("newestAfter(>=150) = %q, want new", got)
	}
	// afterUnix 卡在最新之后 → 全部被过滤，返回空。
	if got := newestAfter(sessions, 400); got != "" {
		t.Errorf("newestAfter(>=400) = %q, want empty", got)
	}
	// 边界：等于 afterUnix 也算命中。
	if got := newestAfter(sessions, 300); got != "new" {
		t.Errorf("newestAfter(>=300) = %q, want new", got)
	}
	if got := newestAfter(nil, 0); got != "" {
		t.Errorf("newestAfter(nil) = %q, want empty", got)
	}
}

func TestCandidatePaths(t *testing.T) {
	// 去重 + 剔除空串。
	if got := candidatePaths("/a", "/a"); len(got) != 1 || got[0] != "/a" {
		t.Errorf("candidatePaths(/a,/a) = %v, want [/a]", got)
	}
	if got := candidatePaths("/a", ""); len(got) != 1 || got[0] != "/a" {
		t.Errorf("candidatePaths(/a,'') = %v, want [/a]", got)
	}
	if got := candidatePaths("/a", "/b"); len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Errorf("candidatePaths(/a,/b) = %v, want [/a /b]", got)
	}
	if got := candidatePaths("", ""); len(got) != 0 {
		t.Errorf("candidatePaths('','') = %v, want []", got)
	}
}

// 非 claude/codex 引擎或裸 shell：DetectResumeSession 不扫盘、直接空。
func TestDetectResumeSessionUnsupportedTool(t *testing.T) {
	s := New()
	if got := s.DetectResumeSession("/p", "", "none", "", 0); got != "" {
		t.Errorf("DetectResumeSession(none) = %q, want empty", got)
	}
	if got := s.DetectResumeSession("/p", "", "gemini", "pin", 0); got != "" {
		t.Errorf("DetectResumeSession(gemini) = %q, want empty", got)
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
