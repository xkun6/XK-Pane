package mcpe

import (
	"testing"

	"xkpane/internal/utils/ptyx"
)

// handleBridgeLog：过白名单的行入缓冲 + emit + 置 useBridge；被白名单丢的噪音不入缓冲不 emit。
// 单 run 时空 runID（老模组未发 hello）按兜底归属处理。
func TestHandleBridgeLogBuffersAndLatches(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{}}
	rt := &runtimeRun{session: &runSession{}, logs: newLogBuffer(100)}
	s.runs["r1"] = rt

	emitted := 0
	s.SetEmitter(func(name string, payload any) { emitted++ })

	// 带 mod 标签的行（规则 4）应保留。
	s.handleBridgeLog("", "[ContainerTreasure] hello")
	if !rt.useBridge.Load() {
		t.Fatal("收到桥日志后 useBridge 应为 true")
	}
	if got := rt.logs.tail(10); len(got) != 1 {
		t.Fatalf("过白名单的行应入缓冲，期望 1 行，得 %v", got)
	}
	if emitted != 1 {
		t.Fatalf("过白名单的行应 emit 一次，得 %d", emitted)
	}

	// 无时间戳头、无 mod 标签的 C++ 引擎噪音应被 parseStdoutLine 丢弃。
	s.handleBridgeLog("", "random c++ engine noise without tag")
	if got := rt.logs.tail(10); len(got) != 1 {
		t.Fatalf("噪音不应入缓冲，缓冲应仍为 1 行，得 %v", got)
	}
	if emitted != 1 {
		t.Fatalf("噪音不应 emit，emit 次数应仍为 1，得 %d", emitted)
	}
}

// 无活跃 run 时 handleBridgeLog 应安全丢弃，不 panic。
func TestHandleBridgeLogNoRun(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{}}
	s.SetEmitter(func(name string, payload any) {})
	s.handleBridgeLog("", "[ContainerTreasure] hello") // 不应 panic
}

// 多 run 并存时 handleBridgeLog 按 runID 精确归属：带 runID 只进对应缓冲/latch，
// 不误伤另一个 run；空 runID（老模组未发 hello）在多 run 场景宁可丢弃也不乱猜。
func TestHandleBridgeLogRoutesByRunID(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{}}
	r1 := &runtimeRun{session: &runSession{pty: new(ptyx.PTY)}, logs: newLogBuffer(100)}
	r2 := &runtimeRun{session: &runSession{pty: new(ptyx.PTY)}, logs: newLogBuffer(100)}
	s.runs["r1"] = r1
	s.runs["r2"] = r2
	s.SetEmitter(func(name string, payload any) {})

	// 带 runID=r2 → 只进 r2 缓冲、只 latch r2。
	s.handleBridgeLog("r2", "[ContainerTreasure] hello r2")
	if r2.logs.tail(10) == nil || len(r2.logs.tail(10)) != 1 {
		t.Fatalf("r2 应收到 1 行，得 %v", r2.logs.tail(10))
	}
	if !r2.useBridge.Load() {
		t.Fatal("r2 应被 latch useBridge=true")
	}
	if got := r1.logs.tail(10); len(got) != 0 {
		t.Fatalf("r1 不应受影响，得 %v", got)
	}
	if r1.useBridge.Load() {
		t.Fatal("r1 不应被误 latch")
	}

	// 空 runID + 多 run 并存 → 丢弃，两边缓冲都不进。
	s.handleBridgeLog("", "[ContainerTreasure] no hello, multi-run")
	if got := r1.logs.tail(10); len(got) != 0 {
		t.Fatalf("多 run 时空 runID 应被丢弃，r1 不应入缓冲，得 %v", got)
	}
	if got := r2.logs.tail(10); len(got) != 1 {
		t.Fatalf("多 run 时空 runID 应被丢弃，r2 缓冲应仍为 1 行，得 %v", got)
	}
}
