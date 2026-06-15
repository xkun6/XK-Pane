package mcpe

import (
	"testing"

	"xkpane/internal/utils/ptyx"
)

func bufWith(lines ...string) *logBuffer {
	b := newLogBuffer(100)
	for _, l := range lines {
		b.push(l)
	}
	return b
}

func TestRecentLogsEmptyReturnsNil(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{}}
	if got := s.RecentLogs("", 10); got != nil {
		t.Fatalf("无 run 时应返回 nil，得 %v", got)
	}
}

func TestRecentLogsByRunID(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{
		"a": {session: &runSession{}, logs: bufWith("a1", "a2")},
		"b": {session: &runSession{}, logs: bufWith("b1")},
	}}
	if got := s.RecentLogs("a", 10); len(got) != 2 || got[0] != "a1" {
		t.Fatalf("按 runID 取 a 失败：%v", got)
	}
	if got := s.RecentLogs("b", 10); len(got) != 1 || got[0] != "b1" {
		t.Fatalf("按 runID 取 b 失败：%v", got)
	}
}

// 退出驱逐：mc_pe:exited 触发 removeRun 后，get_mc_pe_logs（runID=""）不应再吐旧日志。
func TestRemoveRunEvictsStaleBuffer(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{
		"dead": {session: &runSession{}, logs: bufWith("old-stale")},
	}}
	if got := s.RecentLogs("", 10); len(got) != 1 || got[0] != "old-stale" {
		t.Fatalf("驱逐前应能读到该 run：%v", got)
	}
	s.removeRun("dead") // 模拟 capturingEmit 收到 MCPeExited
	if got := s.RecentLogs("", 10); got != nil {
		t.Fatalf("驱逐后 runID=\"\" 应返回 nil（不再吐冻结旧日志），得 %v", got)
	}
	if _, ok := s.runs["dead"]; ok {
		t.Fatal("removeRun 后 run 仍残留在注册表")
	}
}

// 优先活跃 run：死 run 与活 run 共存时（退出宽限窗 / 多包），runID="" 必须命中活 run，
// 不受 Go map 随机迭代顺序影响。pty!=nil 即视为存活（alive 只判空，不解引用）。
func TestRecentLogsPrefersAliveRun(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{
		"dead": {session: &runSession{}, logs: bufWith("old")},
		"live": {session: &runSession{pty: new(ptyx.PTY)}, logs: bufWith("fresh")},
	}}
	for i := 0; i < 30; i++ {
		got := s.RecentLogs("", 10)
		if len(got) != 1 || got[0] != "fresh" {
			t.Fatalf("第 %d 次：runID=\"\" 应优先取活跃 run(fresh)，得 %v", i, got)
		}
	}
}
