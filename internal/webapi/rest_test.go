package webapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xkpane/internal/services/orchestrator"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/terminal"
)

// ── fakes ─────────────────────────────────────────────────────────────────

type fakeLauncher struct {
	got orchestrator.LaunchParams
	res orchestrator.LaunchResult
	err error
}

func (f *fakeLauncher) LaunchTask(_ context.Context, p orchestrator.LaunchParams) (orchestrator.LaunchResult, error) {
	f.got = p
	return f.res, f.err
}

type fakeSessions struct{ snaps []terminal.SessionSnapshot }

func (f fakeSessions) SessionSnapshots() []terminal.SessionSnapshot { return f.snaps }

type fakeScanner struct {
	gotPath  string
	gotLimit int
	rows     []sessionscan.ResumeSession
}

func (f *fakeScanner) ListProjectSessions(_ context.Context, projectPath string, limit int) ([]sessionscan.ResumeSession, error) {
	f.gotPath = projectPath
	f.gotLimit = limit
	return f.rows, nil
}

type fakeReplayer struct {
	gotID string
	rep   terminal.RawReplay
	ok    bool
}

func (f *fakeReplayer) SessionReplay(sessionID string) (terminal.RawReplay, bool) {
	f.gotID = sessionID
	return f.rep, f.ok
}

type fakeWriter struct {
	gotID   string
	gotData string
	err     error
}

func (f *fakeWriter) WriteInput(_ context.Context, sessionID, data string) error {
	f.gotID = sessionID
	f.gotData = data
	return f.err
}

// authedClient 返回一个携带有效 Session Cookie 的请求执行器（禁自动重定向）。
func authedReq(t *testing.T, method, url string, body []byte, g *Gateway) *http.Request {
	t.Helper()
	var r *http.Request
	var err error
	if body != nil {
		r, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		r, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.AddCookie(&http.Cookie{Name: cookieName, Value: g.sessionCookieValue()})
	return r
}

// ── REST 端点：鉴权 + 行为 ──────────────────────────────────────────────────

func TestLaunchRequiresPairing(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Launcher: &fakeLauncher{}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 无 cookie → 401（fail-closed）。
	resp, err := http.Post(srv.URL+"/api/launch", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired launch = %d, want 401", resp.StatusCode)
	}
}

func TestLaunchInvokesLauncher(t *testing.T) {
	fl := &fakeLauncher{res: orchestrator.LaunchResult{TaskID: "t1", SessionID: "s1"}}
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Launcher: fl})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	body := []byte(`{"projectPath":"/p","prompt":"hi","cliTool":"codex","title":"T"}`)
	req := authedReq(t, http.MethodPost, srv.URL+"/api/launch", body, g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("launch = %d, want 200", resp.StatusCode)
	}
	var out orchestrator.LaunchResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SessionID != "s1" || out.TaskID != "t1" {
		t.Fatalf("result = %+v, want t1/s1", out)
	}
	if fl.got.ProjectPath != "/p" || fl.got.Prompt != "hi" || fl.got.CliTool != "codex" || fl.got.Title != "T" {
		t.Fatalf("launcher got = %+v", fl.got)
	}
}

func TestLaunchValidationErrorIs400(t *testing.T) {
	fl := &fakeLauncher{err: context.DeadlineExceeded} // 任意 error → 400 + {error}
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Launcher: fl})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req := authedReq(t, http.MethodPost, srv.URL+"/api/launch", []byte(`{"projectPath":"/p"}`), g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("launch err = %d, want 400", resp.StatusCode)
	}
	var out map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["error"] == "" {
		t.Fatal("expected {error} body")
	}
}

func TestLaunchUnavailableWhenNoLauncher(t *testing.T) {
	g := newTestGateway("tok") // 未 AttachServices
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req := authedReq(t, http.MethodPost, srv.URL+"/api/launch", []byte(`{}`), g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("no launcher = %d, want 503", resp.StatusCode)
	}
}

func TestSessionsListGated(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Sessions: fakeSessions{snaps: []terminal.SessionSnapshot{
		{ID: "s1", Status: "active", CliTool: "claude"},
	}}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 无 cookie → 401。
	resp, err := http.Get(srv.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired sessions = %d, want 401", resp.StatusCode)
	}

	// 带 cookie → 200 + 列表。
	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions", nil, g)
	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sessions = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Sessions []terminal.SessionSnapshot `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Sessions) != 1 || out.Sessions[0].ID != "s1" {
		t.Fatalf("sessions = %+v", out.Sessions)
	}
}

func TestSessionsScanRequiresProjectPath(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Scanner: &fakeScanner{}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 缺 projectPath → 400。
	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions/scan", nil, g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("scan without projectPath = %d, want 400", resp.StatusCode)
	}
}

func TestSessionsScanReturnsSessions(t *testing.T) {
	fs := &fakeScanner{rows: []sessionscan.ResumeSession{
		{ID: "abc", Source: "claude", ProjectPath: "/p", Description: "hi"},
	}}
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Scanner: fs})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 无 cookie → 401（fail-closed）。
	resp, err := http.Get(srv.URL + "/api/sessions/scan?projectPath=/p")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired scan = %d, want 401", resp.StatusCode)
	}

	// 带 cookie + projectPath + limit → 200 + 列表，且扫描器收到归一参数。
	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions/scan?projectPath=%2Fp&limit=5", nil, g)
	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scan = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Sessions []sessionscan.ResumeSession `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Sessions) != 1 || out.Sessions[0].ID != "abc" {
		t.Fatalf("scan sessions = %+v", out.Sessions)
	}
	if fs.gotPath != "/p" || fs.gotLimit != 5 {
		t.Fatalf("scanner got path=%q limit=%d, want /p,5", fs.gotPath, fs.gotLimit)
	}
}

func TestSessionReplayRequiresSessionId(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Replayer: &fakeReplayer{}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions/replay", nil, g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay without sessionId = %d, want 400", resp.StatusCode)
	}
}

func TestSessionReplayReturnsSnapshot(t *testing.T) {
	fr := &fakeReplayer{rep: terminal.RawReplay{Data: "\x1b[31mhi\x1b[0m", LastSeq: 7}, ok: true}
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Replayer: fr})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	// 无 cookie → 401（fail-closed）。
	resp, err := http.Get(srv.URL + "/api/sessions/replay?sessionId=s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired replay = %d, want 401", resp.StatusCode)
	}

	// 带 cookie → 200 + 快照，且 sessionId 透传、由 handler 回填。
	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions/replay?sessionId=s1", nil, g)
	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replay = %d, want 200", resp.StatusCode)
	}
	var out terminal.RawReplay
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SessionID != "s1" || out.Data != "\x1b[31mhi\x1b[0m" || out.LastSeq != 7 {
		t.Fatalf("replay out = %+v", out)
	}
	if fr.gotID != "s1" {
		t.Fatalf("replayer got id = %q, want s1", fr.gotID)
	}
}

func TestSessionReplayUnknownSessionReturnsEmpty(t *testing.T) {
	fr := &fakeReplayer{ok: false} // 未知会话
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Replayer: fr})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req := authedReq(t, http.MethodGet, srv.URL+"/api/sessions/replay?sessionId=ghost", nil, g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unknown replay = %d, want 200 (graceful empty)", resp.StatusCode)
	}
	var out terminal.RawReplay
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SessionID != "ghost" || out.Data != "" || out.LastSeq != 0 {
		t.Fatalf("unknown replay out = %+v, want {ghost,\"\",0}", out)
	}
}

func TestSessionInputRequiresPairing(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Writer: &fakeWriter{}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/sessions/input", "application/json",
		bytes.NewReader([]byte(`{"sessionId":"s1","data":"x"}`)))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unpaired input = %d, want 401", resp.StatusCode)
	}
}

func TestSessionInputWritesToSession(t *testing.T) {
	fw := &fakeWriter{}
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Writer: fw})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	body := []byte(`{"sessionId":"s1","data":"ls\r"}`)
	req := authedReq(t, http.MethodPost, srv.URL+"/api/sessions/input", body, g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("input = %d, want 204", resp.StatusCode)
	}
	if fw.gotID != "s1" || fw.gotData != "ls\r" {
		t.Fatalf("writer got id=%q data=%q", fw.gotID, fw.gotData)
	}
}

func TestSessionInputMissingSessionId(t *testing.T) {
	g := newTestGateway("tok")
	g.AttachServices(ServiceDeps{Writer: &fakeWriter{}})
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	req := authedReq(t, http.MethodPost, srv.URL+"/api/sessions/input", []byte(`{"data":"x"}`), g)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("input without sessionId = %d, want 400", resp.StatusCode)
	}
}
