package terminal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/cliadapters"
)

// --- utf8SafeProcess：跨 read() 的 UTF-8 截断处理三分支 ---

func TestUTF8SafeProcessAllValid(t *testing.T) {
	out, carry := utf8SafeProcess([]byte("hello 世界"), nil)
	if out != "hello 世界" {
		t.Fatalf("out = %q, want %q", out, "hello 世界")
	}
	if len(carry) != 0 {
		t.Fatalf("carry = %v, want empty", carry)
	}
}

// 末尾是不完整多字节序列 → emit 有效前缀，截断字节进 carry；下一次拼回还原完整字符。
func TestUTF8SafeProcessIncompleteTailCarried(t *testing.T) {
	full := []byte("ab世") // '世' = E4 B8 96（3 字节）
	split := len(full) - 1 // 砍掉最后一字节，制造不完整尾部

	out1, carry := utf8SafeProcess(full[:split], nil)
	if out1 != "ab" {
		t.Fatalf("first out = %q, want %q", out1, "ab")
	}
	if len(carry) != 2 { // E4 B8 两字节待补
		t.Fatalf("carry len = %d, want 2", len(carry))
	}

	out2, carry2 := utf8SafeProcess(full[split:], carry)
	if out2 != "世" {
		t.Fatalf("second out = %q, want %q", out2, "世")
	}
	if len(carry2) != 0 {
		t.Fatalf("carry2 = %v, want empty", carry2)
	}
}

// 起始即不完整多字节（validUpTo==0）→ 不 emit，整段进 carry。
func TestUTF8SafeProcessLeadingIncomplete(t *testing.T) {
	out, carry := utf8SafeProcess([]byte{0xE4, 0xB8}, nil)
	if out != "" {
		t.Fatalf("out = %q, want empty", out)
	}
	if len(carry) != 2 {
		t.Fatalf("carry len = %d, want 2", len(carry))
	}
}

// 中段真·非法字节 → lossy 转换（U+FFFD），不 carry。
func TestUTF8SafeProcessMidStreamInvalid(t *testing.T) {
	// 0xFF 不是任何合法 UTF-8 起始字节，且其后还有合法字节 → genuine invalid，非截断。
	out, carry := utf8SafeProcess([]byte{'a', 0xFF, 'b'}, nil)
	if len(carry) != 0 {
		t.Fatalf("carry = %v, want empty (lossy, no carry)", carry)
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("out = %q, want to contain 'a' and 'b'", out)
	}
	if !strings.ContainsRune(out, '�') {
		t.Fatalf("out = %q, want U+FFFD replacement", out)
	}
}

// --- isIncompleteTrailing ---

func TestIsIncompleteTrailing(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"two-byte lead missing continuation", []byte{0xC3}, true},        // 2 字节序列只给了起始
		{"three-byte lead one of two continuation", []byte{0xE4, 0xB8}, true},
		{"len>=need (full 3-byte seq)", []byte{0xE4, 0xB8, 0x96}, false},  // 字节数已够 → 非「截断待补」
		{"invalid lead byte", []byte{0xFF}, false},                        // 0xFF 非合法起始
		{"bad continuation", []byte{0xE4, 0x20}, false},                   // 续字节非 10xxxxxx
		{"empty", nil, false},
	}
	for _, c := range cases {
		// 注意：完整且合法的 E4 B8 96 在 utf8SafeProcess 里早被 utf8.Valid 短路，
		// 不会进 isIncompleteTrailing；这里单测函数本身：字节数 >= need 即返回 false。
		if got := isIncompleteTrailing(c.in); got != c.want {
			t.Errorf("%s: isIncompleteTrailing(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// --- exitCodeOf ---

func TestExitCodeOf(t *testing.T) {
	if got := exitCodeOf(nil); got != 0 {
		t.Errorf("nil err → %d, want 0", got)
	}
	if got := exitCodeOf(errors.New("boom")); got != -1 {
		t.Errorf("generic err → %d, want -1", got)
	}
	// 真实 *exec.ExitError：跑一个必然以 3 退出的命令拿到它。
	var ee *exec.ExitError
	err := failingCmd(3).Run()
	if !errors.As(err, &ee) {
		t.Skipf("could not obtain *exec.ExitError on this platform: %v", err)
	}
	if got := exitCodeOf(err); got != 3 {
		t.Errorf("ExitError → %d, want 3", got)
	}
}

// failingCmd 构造一个跨平台、以指定码退出的命令（用于拿到真实 *exec.ExitError）。
func failingCmd(code int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "exit "+strconv.Itoa(code))
	}
	return exec.Command("sh", "-c", "exit "+strconv.Itoa(code))
}

// --- buildEnv ---

func TestBuildEnvInjectsTermAndSessionID(t *testing.T) {
	env := buildEnv("sess-123", "claude", map[string]string{"FOO": "bar"}, nil)
	m := envToMap(env)
	if m["CC_PANES_PTY_SESSION_ID"] != "sess-123" {
		t.Errorf("session id = %q, want sess-123", m["CC_PANES_PTY_SESSION_ID"])
	}
	if m["CC_PANES_CLI_TOOL"] != "claude" {
		t.Errorf("cli tool = %q, want claude", m["CC_PANES_CLI_TOOL"])
	}
	if m["TERM"] == "" {
		t.Errorf("TERM not injected")
	}
	if m["COLORTERM"] == "" {
		t.Errorf("COLORTERM not injected")
	}
	if m["FOO"] != "bar" {
		t.Errorf("inject FOO = %q, want bar", m["FOO"])
	}
}

func TestBuildEnvRemoveAndOverride(t *testing.T) {
	t.Setenv("XKPANE_TEST_REMOVE_ME", "present")
	env := buildEnv("s", "none", map[string]string{"TERM": "override-term"}, []string{"XKPANE_TEST_REMOVE_ME"})
	m := envToMap(env)
	if _, ok := m["XKPANE_TEST_REMOVE_ME"]; ok {
		t.Errorf("envRemove did not remove the variable")
	}
	if m["TERM"] != "override-term" {
		t.Errorf("inject should override TERM default; got %q", m["TERM"])
	}
	// toolID == "none" → 不注入 CC_PANES_CLI_TOOL
	if _, ok := m["CC_PANES_CLI_TOOL"]; ok {
		t.Errorf("CC_PANES_CLI_TOOL should be absent for 'none' tool")
	}
}

// --- resolveLaunch ---

func TestResolveLaunchBareShell(t *testing.T) {
	svc := New(cliadapters.Default())
	for _, tool := range []string{"", "none"} {
		cmd, _, inject, remove, err := svc.resolveLaunch("id", CreateSpec{ProjectPath: ".", CliTool: tool})
		if err != nil {
			t.Fatalf("tool=%q: %v", tool, err)
		}
		if strings.TrimSpace(cmd) == "" {
			t.Errorf("tool=%q: empty shell command", tool)
		}
		if inject != nil || remove != nil {
			t.Errorf("tool=%q: bare shell should not inject/remove env", tool)
		}
	}
}

func TestResolveLaunchUnknownTool(t *testing.T) {
	svc := New(cliadapters.Default())
	if _, _, _, _, err := svc.resolveLaunch("id", CreateSpec{ProjectPath: ".", CliTool: "no-such-tool"}); err == nil {
		t.Fatalf("expected error for unknown cli tool")
	}
}

func TestCreateSessionEmptyProjectPath(t *testing.T) {
	svc := New(cliadapters.Default())
	if _, err := svc.CreateSession(context.Background(), CreateSpec{ProjectPath: "   "}); err == nil {
		t.Fatalf("expected error for empty projectPath")
	}
}

// --- 集成：完整闭环（spawn → 输出流 → kill → exit 事件 → 清理）---
//
// 仿 ptyx_test 的轮询风格：reader 持续刷缓冲，等 marker / exit 出现，
// 不依赖事件时序，规避 ConPTY 异步刷新 race。
func TestCreateSessionRoundTrip(t *testing.T) {
	svc := New(cliadapters.Default())

	var (
		mu       sync.Mutex
		outBuf   strings.Builder
		exited   bool
		gotPID   int
		statuses []string
	)
	svc.setEmit(func(name string, payload any) {
		mu.Lock()
		defer mu.Unlock()
		switch p := payload.(type) {
		case events.TerminalOutputPayload:
			outBuf.WriteString(p.Data)
		case events.TerminalExitPayload:
			exited = true
		case events.TerminalStatusPayload:
			statuses = append(statuses, p.Status)
			if p.PID != 0 {
				gotPID = p.PID
			}
		}
	})

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	id, err := svc.CreateSession(context.Background(), CreateSpec{
		ProjectPath: cwd,
		CliTool:     "", // 裸 shell
		Cols:        120,
		Rows:        30,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// 写入一条会回显/执行的命令；marker 出现即证明输出隧道连通。
	const marker = "xkpane-term-RT-91237"
	if err := svc.WriteInput(context.Background(), id, "echo "+marker+"\r"); err != nil {
		t.Fatalf("WriteInput: %v", err)
	}

	if !poll(8*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(outBuf.String(), marker)
	}) {
		mu.Lock()
		got := outBuf.String()
		mu.Unlock()
		t.Fatalf("marker %q not seen; got %d bytes: %q", marker, len(got), got)
	}

	// 硬杀 → 应触发 waitLoop：drain 尾部输出 → Close PTY → emit exit → 从注册表移除。
	if err := svc.Kill(context.Background(), id); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	if !poll(8*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return exited
	}) {
		t.Fatalf("terminal:exit not emitted after Kill")
	}

	// 会话应已从注册表回收。
	if !poll(2*time.Second, func() bool {
		list, _ := svc.List(context.Background())
		return len(list) == 0
	}) {
		list, _ := svc.List(context.Background())
		t.Fatalf("session not removed after exit; List = %+v", list)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPID == 0 {
		t.Errorf("never observed a non-zero PID in status events")
	}
	if len(statuses) == 0 {
		t.Errorf("no status events emitted")
	}
}

// --- helpers ---

func poll(deadline time.Duration, cond func() bool) bool {
	const step = 25 * time.Millisecond
	for waited := time.Duration(0); waited < deadline; waited += step {
		if cond() {
			return true
		}
		time.Sleep(step)
	}
	return cond()
}

func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}
