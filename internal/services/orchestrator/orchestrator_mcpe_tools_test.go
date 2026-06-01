package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"xkpane/internal/services/mcdkbridge"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── fakes ───────────────────────────────────────────────────────────────

// fakeBridge 捕获最近发送的帧，可注入 SendPacket 错误。
type fakeBridge struct {
	sent []mcdkbridge.Packet
	err  error
}

func (f *fakeBridge) SendPacket(p mcdkbridge.Packet) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, p)
	return nil
}
func (f *fakeBridge) Status() mcdkbridge.ClientStatus { return mcdkbridge.ClientStatus{} }

func (f *fakeBridge) last() mcdkbridge.Packet { return f.sent[len(f.sent)-1] }

// fakeMCPE 返回预置日志，并记录最近一次 RecentLogs 的入参（验证 clamp）。
type fakeMCPE struct {
	logs     []string
	gotRunID string
	gotLines int
}

func (f *fakeMCPE) RecentLogs(runID string, lines int) []string {
	f.gotRunID = runID
	f.gotLines = lines
	return f.logs
}
func (f *fakeMCPE) ActiveRunPIDs() []int { return nil }

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
