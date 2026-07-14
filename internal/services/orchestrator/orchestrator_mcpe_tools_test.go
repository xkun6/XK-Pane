package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"xkpane/internal/services/mcdkbridge"
	"xkpane/internal/storage"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── fakes ───────────────────────────────────────────────────────────────

// sentPacket 记录一次 SendPacketToRun 调用的 (runID, 帧)。
type sentPacket struct {
	runID string
	pkt   mcdkbridge.Packet
}

// fakeBridge 捕获最近发送的帧，可注入 SendPacket 错误 / ClientRunIDs 返回值。
type fakeBridge struct {
	sent       []mcdkbridge.Packet
	sentToRun  []sentPacket
	err        error
	clientRuns []string
}

func (f *fakeBridge) SendPacket(p mcdkbridge.Packet) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, p)
	return nil
}
func (f *fakeBridge) SendPacketToRun(runID string, p mcdkbridge.Packet) error {
	if f.err != nil {
		return f.err
	}
	f.sentToRun = append(f.sentToRun, sentPacket{runID: runID, pkt: p})
	return nil
}
func (f *fakeBridge) Status() mcdkbridge.ClientStatus { return mcdkbridge.ClientStatus{} }
func (f *fakeBridge) ClientRunIDs() []string          { return f.clientRuns }

func (f *fakeBridge) last() mcdkbridge.Packet { return f.sent[len(f.sent)-1] }

// fakeMCPE 返回预置日志，并记录最近一次 RecentLogs 的入参（验证 clamp）；可注入 ActiveRunSummaries。
type fakeMCPE struct {
	logs      []string
	gotRunID  string
	gotLines  int
	summaries []storage.MCRunSummary
}

func (f *fakeMCPE) RecentLogs(runID string, lines int) []string {
	f.gotRunID = runID
	f.gotLines = lines
	return f.logs
}
func (f *fakeMCPE) ActiveRunPIDs() []int                       { return nil }
func (f *fakeMCPE) ActiveRunSummaries() []storage.MCRunSummary { return f.summaries }

// resultText 取 CallToolResult 的首个 TextContent 文本。
func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		t.Fatal("结果为 nil 或无 Content")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] 不是 TextContent，实际 %T", r.Content[0])
	}
	return tc.Text
}

// ── execute_mc_code ───────────────────────────────────────────────────────

func TestToolExecuteMcCode(t *testing.T) {
	ctx := context.Background()

	// 服务端（默认 isClient=false）→ type 3，payload 为裸 UTF-8 代码。
	fb := &fakeBridge{}
	s := New(Deps{Bridge: fb})
	res, _, _ := s.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "print(1)"})
	if got := fb.last(); got.Type != mcdkbridge.PacketExecuteServerCode || string(got.Payload) != "print(1)" {
		t.Errorf("server: type=%d payload=%q，want type=3 payload=print(1)", got.Type, got.Payload)
	}
	if txt := resultText(t, res); !strings.Contains(txt, "isClient=false") || !strings.Contains(txt, "get_mc_pe_logs") {
		t.Errorf("成功文案应含 isClient=false 与 get_mc_pe_logs，实际 %q", txt)
	}

	// 客户端 isClient=true → type 4。
	fb2 := &fakeBridge{}
	s2 := New(Deps{Bridge: fb2})
	s2.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "x", IsClient: true})
	if got := fb2.last(); got.Type != mcdkbridge.PacketExecuteClientCode {
		t.Errorf("client: type=%d，want 4", got.Type)
	}

	// nil bridge → 兜底错误文本（不发帧、不 panic）。
	sNil := New(Deps{})
	res3, _, _ := sNil.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "x"})
	if txt := resultText(t, res3); !strings.Contains(txt, "错误") {
		t.Errorf("nil bridge 应返回错误文本，实际 %q", txt)
	}

	// SendPacket 失败 → 「错误: 发送失败」。
	fbErr := &fakeBridge{err: errors.New("no client")}
	sErr := New(Deps{Bridge: fbErr})
	res4, _, _ := sErr.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "x"})
	if txt := resultText(t, res4); !strings.Contains(txt, "发送失败") {
		t.Errorf("发送失败应反映在文本，实际 %q", txt)
	}
}

// 带 runId → 走 SendPacketToRun 精确定向，不碰 SendPacket。
func TestToolExecuteMcCodeWithRunID(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBridge{}
	s := New(Deps{Bridge: fb})
	s.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "print(1)", RunID: "run-a"})
	if len(fb.sent) != 0 {
		t.Errorf("带 runId 不应走 SendPacket，实际发了 %d 帧", len(fb.sent))
	}
	if len(fb.sentToRun) != 1 {
		t.Fatalf("应走 SendPacketToRun 一次，实际 %d 次", len(fb.sentToRun))
	}
	if got := fb.sentToRun[0]; got.runID != "run-a" || got.pkt.Type != mcdkbridge.PacketExecuteServerCode {
		t.Errorf("SendPacketToRun 入参异常：%+v", got)
	}
}

// 不带 runId 且桥上有多个客户端 → 拒绝并提示传 runId，不发任何帧。
func TestToolExecuteMcCodeMultiClientRequiresRunID(t *testing.T) {
	ctx := context.Background()
	fb := &fakeBridge{clientRuns: []string{"run-a", "run-b"}}
	s := New(Deps{Bridge: fb})
	res, _, _ := s.toolExecuteMcCode(ctx, nil, executeMcCodeParams{Code: "print(1)"})
	if txt := resultText(t, res); !strings.Contains(txt, "请传 runId") {
		t.Errorf("多客户端未传 runId 应提示「请传 runId」，实际 %q", txt)
	}
	if len(fb.sent) != 0 || len(fb.sentToRun) != 0 {
		t.Errorf("多客户端未传 runId 不应发帧，实际 sent=%d sentToRun=%d", len(fb.sent), len(fb.sentToRun))
	}
}

// ── reload_mc ─────────────────────────────────────────────────────────────

func TestToolReloadMc(t *testing.T) {
	ctx := context.Background()

	kinds := map[string]mcdkbridge.PacketType{
		"game":           mcdkbridge.PacketReloadGame,
		"addon_and_game": mcdkbridge.PacketReloadAddonAndGame,
		"all_shaders":    mcdkbridge.PacketReloadAllShaders,
	}
	for kind, wantType := range kinds {
		fb := &fakeBridge{}
		s := New(Deps{Bridge: fb})
		res, _, _ := s.toolReloadMc(ctx, nil, reloadMcParams{Kind: kind})
		if got := fb.last(); got.Type != wantType {
			t.Errorf("kind=%s: type=%d，want %d", kind, got.Type, wantType)
		}
		if txt := resultText(t, res); !strings.Contains(txt, "已发送 reload_mc kind="+kind) {
			t.Errorf("kind=%s 成功文案异常：%q", kind, txt)
		}
	}

	// single_shader 带 fileName → type 7，payload 为文件名。
	fb := &fakeBridge{}
	s := New(Deps{Bridge: fb})
	s.toolReloadMc(ctx, nil, reloadMcParams{Kind: "single_shader", FileName: "blur.fragment"})
	if got := fb.last(); got.Type != mcdkbridge.PacketReloadSingleShader || string(got.Payload) != "blur.fragment" {
		t.Errorf("single_shader: type=%d payload=%q，want type=7 payload=blur.fragment", got.Type, got.Payload)
	}

	// single_shader 缺 fileName → 错误，且不发帧。
	fbMiss := &fakeBridge{}
	sMiss := New(Deps{Bridge: fbMiss})
	res, _, _ := sMiss.toolReloadMc(ctx, nil, reloadMcParams{Kind: "single_shader"})
	if txt := resultText(t, res); !strings.Contains(txt, "fileName 必填") {
		t.Errorf("缺 fileName 应提示必填，实际 %q", txt)
	}
	if len(fbMiss.sent) != 0 {
		t.Error("缺 fileName 不应发帧")
	}

	// 未知 kind → 错误，且不发帧。
	fbUnk := &fakeBridge{}
	sUnk := New(Deps{Bridge: fbUnk})
	res2, _, _ := sUnk.toolReloadMc(ctx, nil, reloadMcParams{Kind: "bogus"})
	if txt := resultText(t, res2); !strings.Contains(txt, "未知 kind=bogus") {
		t.Errorf("未知 kind 应报错，实际 %q", txt)
	}
	if len(fbUnk.sent) != 0 {
		t.Error("未知 kind 不应发帧")
	}

	// nil bridge → 兜底错误。
	sNil := New(Deps{})
	res3, _, _ := sNil.toolReloadMc(ctx, nil, reloadMcParams{Kind: "game"})
	if txt := resultText(t, res3); !strings.Contains(txt, "错误") {
		t.Errorf("nil bridge 应返回错误文本，实际 %q", txt)
	}
}

// ── get_mc_pe_logs ─────────────────────────────────────────────────────────

func TestToolGetMcPeLogs(t *testing.T) {
	ctx := context.Background()

	// 有日志 → 多行 join。
	fm := &fakeMCPE{logs: []string{"a", "b", "c"}}
	s := New(Deps{MCPE: fm})
	res, _, _ := s.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{Lines: 50})
	if txt := resultText(t, res); txt != "a\nb\nc" {
		t.Errorf("日志 join 异常：%q", txt)
	}
	if fm.gotRunID != "" || fm.gotLines != 50 {
		t.Errorf("RecentLogs 入参 runID=%q lines=%d，want \"\"/50", fm.gotRunID, fm.gotLines)
	}

	// lines clamp：0/缺省 → 200；>2000 → 2000。
	clampCases := []struct{ in, want int }{
		{0, 200}, {-5, 200}, {5000, 2000}, {2000, 2000}, {1, 1},
	}
	for _, c := range clampCases {
		fmc := &fakeMCPE{logs: []string{"x"}}
		sc := New(Deps{MCPE: fmc})
		sc.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{Lines: c.in})
		if fmc.gotLines != c.want {
			t.Errorf("lines=%d clamp 后 = %d，want %d", c.in, fmc.gotLines, c.want)
		}
	}

	// 空日志 → 错误文本。
	fmEmpty := &fakeMCPE{logs: nil}
	sEmpty := New(Deps{MCPE: fmEmpty})
	resEmpty, _, _ := sEmpty.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{})
	if txt := resultText(t, resEmpty); !strings.Contains(txt, "错误") {
		t.Errorf("空日志应返回错误文本，实际 %q", txt)
	}

	// nil MCPE → 兜底错误。
	sNil := New(Deps{})
	resNil, _, _ := sNil.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{})
	if txt := resultText(t, resNil); !strings.Contains(txt, "错误") {
		t.Errorf("nil MCPE 应返回错误文本，实际 %q", txt)
	}
}

// 带 runId → 直取该 run 的日志，不走分节逻辑。
func TestToolGetMcPeLogsWithRunID(t *testing.T) {
	ctx := context.Background()
	fm := &fakeMCPE{logs: []string{"only-this-run"}}
	s := New(Deps{MCPE: fm})
	res, _, _ := s.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{RunID: "run-a", Lines: 10})
	if txt := resultText(t, res); txt != "only-this-run" {
		t.Errorf("带 runId 应直取该 run 日志，实际 %q", txt)
	}
	if fm.gotRunID != "run-a" {
		t.Errorf("RecentLogs 应传入 runId=run-a，实际 %q", fm.gotRunID)
	}
}

// 带 runId 但该 run 无日志 → 错误文本带上 runId。
func TestToolGetMcPeLogsRunIDNotFound(t *testing.T) {
	ctx := context.Background()
	fm := &fakeMCPE{logs: nil}
	s := New(Deps{MCPE: fm})
	res, _, _ := s.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{RunID: "run-ghost"})
	txt := resultText(t, res)
	if !strings.Contains(txt, "错误") || !strings.Contains(txt, "run-ghost") {
		t.Errorf("runId 无日志应报错并带上 runId，实际 %q", txt)
	}
}

// 不带 runId 且有 2 个活跃客户端 → 分节返回，每节带各自 runId 头。
func TestToolGetMcPeLogsMultiClientSections(t *testing.T) {
	ctx := context.Background()
	pidA := 111
	fm := &fakeMCPE{
		summaries: []storage.MCRunSummary{
			{RunID: "run-a", PackageID: "pkg-a", PackageName: "包A", PID: &pidA},
			{RunID: "run-b", PackageID: "pkg-b", PackageName: "包B"}, // PID nil → 分节里应显示 pid=?
		},
		logs: []string{"line1", "line2"}, // RecentLogs 桩对任何 runID 都返回同一份，够验证分节结构
	}
	s := New(Deps{MCPE: fm})
	res, _, _ := s.toolGetMcPeLogs(ctx, nil, getMcPeLogsParams{})
	txt := resultText(t, res)
	if got := strings.Count(txt, "runId="); got != 2 {
		t.Fatalf("应有 2 个 runId= 分节头，实际 %d，正文：%q", got, txt)
	}
	if !strings.Contains(txt, "runId=run-a") || !strings.Contains(txt, "runId=run-b") {
		t.Errorf("分节应各带自己的 runId，实际 %q", txt)
	}
	if !strings.Contains(txt, "pid=111") || !strings.Contains(txt, "pid=?") {
		t.Errorf("分节应体现 PID（有则数字、无则 ?），实际 %q", txt)
	}
	if !strings.Contains(txt, "包A") || !strings.Contains(txt, "包B") {
		t.Errorf("分节应带包名，实际 %q", txt)
	}
}
