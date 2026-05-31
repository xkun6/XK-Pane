package orchestrator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"xkpane/internal/events"
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
