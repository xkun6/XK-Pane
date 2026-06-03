package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/storage"
)

func TestRateLimiterWindow(t *testing.T) {
	base := time.Unix(1000, 0)
	now := base
	rl := newRateLimiter(10*time.Second, 3)
	rl.nowFn = func() time.Time { return now }

	if !rl.allow() || !rl.allow() || !rl.allow() {
		t.Fatal("前 3 个应放行")
	}
	if rl.allow() {
		t.Fatal("第 4 个应限流")
	}
	// 窗口滑出后恢复。
	now = base.Add(11 * time.Second)
	if !rl.allow() {
		t.Fatal("窗口滑出后应放行")
	}
}

// 构造一个已设 token 的 Service（不真正监听网络），用 httptest 直驱 handler。
func newTestService(token string) *Service {
	s := New(Deps{})
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
	return s
}

func TestHandleSessionStartedUnauthorized(t *testing.T) {
	s := newTestService("secret")
	req := httptest.NewRequest(http.MethodPost, "/api/terminal/session-started", bytes.NewBufferString(`{"ptySessionId":"x"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	s.handleSessionStarted(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("code=%d want 401", w.Code)
	}
}

func TestHandleSessionStartedEmitsEvent(t *testing.T) {
	s := newTestService("secret")
	var (
		mu     sync.Mutex
		gotEvt string
		gotPay any
	)
	s.emit = func(name string, payload any) {
		mu.Lock()
		defer mu.Unlock()
		gotEvt = name
		gotPay = payload
	}

	body, _ := json.Marshal(sessionStartedRequest{
		PtySessionID: "pty-1", ResumeSessionID: "rs-1", CliTool: "reclaude", RuntimeKind: "local", Cwd: `C:\proj`,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/terminal/session-started", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	s.handleSessionStarted(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d want 200 body=%s", w.Code, w.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if gotEvt != events.OrchestratorSessionStarted {
		t.Errorf("event=%q want %q", gotEvt, events.OrchestratorSessionStarted)
	}
	pay, ok := gotPay.(events.OrchestratorSessionStartedPayload)
	if !ok {
		t.Fatalf("payload 类型=%T", gotPay)
	}
	if pay.PtySessionID != "pty-1" || pay.CliTool != "reclaude" || pay.Cwd != `C:\proj` {
		t.Errorf("payload=%+v 不符", pay)
	}
}

func TestHandleSessionStartedMissingPty(t *testing.T) {
	s := newTestService("secret")
	req := httptest.NewRequest(http.MethodPost, "/api/terminal/session-started", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	s.handleSessionStarted(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code=%d want 400", w.Code)
	}
}

// fakeWorkspacesFull 直接返回预设工作空间切片（覆盖多 ws / Alias / 多归属去重场景）。
type fakeWorkspacesFull struct{ wss []storage.Workspace }

func (f fakeWorkspacesFull) List(_ context.Context) ([]storage.Workspace, error) {
	return f.wss, nil
}

func TestWorkspaceSiblings(t *testing.T) {
	beta := "Beta"
	blank := "   "
	wsA := storage.Workspace{Name: "A", Projects: []storage.WorkspaceProject{
		{Path: `C:\proj\a`},
		{Path: `C:\proj\b`, Alias: &beta},
	}}
	wsB := storage.Workspace{Name: "B", Projects: []storage.WorkspaceProject{
		{Path: `C:\proj\a`},                // a 也在 B → 验证「自身」始终排除
		{Path: `C:\proj\b`},                // b 也在 B → 验证去重（保留 A 的 Beta 别名）
		{Path: `C:\proj\c`, Alias: &blank}, // 空白 alias 回退目录名；来自 B → 验证多归属合并
	}}
	wsC := storage.Workspace{Name: "C", Projects: []storage.WorkspaceProject{
		{Path: `C:\proj\x`}, // 不含 a → 整个工作空间忽略
	}}
	s := New(Deps{Workspaces: fakeWorkspacesFull{wss: []storage.Workspace{wsA, wsB, wsC}}})

	// 传 / 形式，归一化后命中存的 \ 形式。
	got := s.workspaceSiblings(context.Background(), `C:/proj/a`)
	want := map[string]string{
		"Beta": `C:\proj\b`, // alias 优先 + 去重（A、B 都有 b，只一次）
		"c":    `C:\proj\c`, // 空白 alias 回退目录名 + 多归属合并（来自 B）
	}
	if len(got) != len(want) {
		t.Fatalf("got %d siblings %+v, want %d", len(got), got, len(want))
	}
	for _, sib := range got {
		wp, ok := want[sib.Name]
		if !ok {
			t.Errorf("意外项目 %q (%s)", sib.Name, sib.Path)
			continue
		}
		if sib.Path != wp {
			t.Errorf("项目 %q path=%q want %q", sib.Name, sib.Path, wp)
		}
	}

	if got := s.workspaceSiblings(context.Background(), `C:/nope`); got != nil {
		t.Errorf("不属任何工作空间应 nil，got %+v", got)
	}
	if got := s.workspaceSiblings(context.Background(), ""); got != nil {
		t.Errorf("空 cwd 应 nil，got %+v", got)
	}
}

func TestHandleSessionStartedInjectsSiblings(t *testing.T) {
	beta := "Beta"
	ws := storage.Workspace{Name: "A", Projects: []storage.WorkspaceProject{
		{Path: `C:\proj\a`},
		{Path: `C:\proj\b`, Alias: &beta},
	}}
	s := New(Deps{Workspaces: fakeWorkspacesFull{wss: []storage.Workspace{ws}}})
	s.mu.Lock()
	s.token = "secret"
	s.mu.Unlock()

	body, _ := json.Marshal(sessionStartedRequest{PtySessionID: "pty-1", Cwd: `C:\proj\a`})
	req := httptest.NewRequest(http.MethodPost, "/api/terminal/session-started", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	s.handleSessionStarted(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d want 200 body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK                bool             `json:"ok"`
		WorkspaceProjects []siblingProject `json:"workspaceProjects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if !resp.OK {
		t.Error("ok 应为 true")
	}
	if len(resp.WorkspaceProjects) != 1 || resp.WorkspaceProjects[0].Name != "Beta" || resp.WorkspaceProjects[0].Path != `C:\proj\b` {
		t.Errorf("workspaceProjects=%+v 不符（期望仅 Beta→C:\\proj\\b）", resp.WorkspaceProjects)
	}
}
