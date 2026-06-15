package mcpe

import "testing"

// handleBridgeLog：过白名单的行入缓冲 + emit + 置 useBridge；被白名单丢的噪音不入缓冲不 emit。
func TestHandleBridgeLogBuffersAndLatches(t *testing.T) {
	s := &Service{runs: map[string]*runtimeRun{}}
	rt := &runtimeRun{session: &runSession{}, logs: newLogBuffer(100)}
	s.runs["r1"] = rt

	emitted := 0
	s.SetEmitter(func(name string, payload any) { emitted++ })

	// 带 mod 标签的行（规则 4）应保留。
	s.handleBridgeLog("[ContainerTreasure] hello")
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
	s.handleBridgeLog("random c++ engine noise without tag")
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
	s.handleBridgeLog("[ContainerTreasure] hello") // 不应 panic
}
