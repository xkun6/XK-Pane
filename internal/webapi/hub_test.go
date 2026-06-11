package webapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xkpane/internal/events"

	"github.com/coder/websocket"
)

func TestHubBroadcastEnvelope(t *testing.T) {
	h := newHub()
	c := newWSClient()
	h.add(c)

	h.broadcast(events.TerminalOutput, events.TerminalOutputPayload{SessionID: "s1", Data: "hello"})

	select {
	case msg := <-c.send:
		var env struct {
			Event string                       `json:"event"`
			Data  events.TerminalOutputPayload `json:"data"`
		}
		if err := json.Unmarshal(msg, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Event != events.TerminalOutput {
			t.Fatalf("event = %q, want %q", env.Event, events.TerminalOutput)
		}
		if env.Data.SessionID != "s1" || env.Data.Data != "hello" {
			t.Fatalf("data = %+v", env.Data)
		}
	default:
		t.Fatal("expected a message in send queue")
	}
}

func TestHubEvictsSlowClient(t *testing.T) {
	h := newHub()
	c := newWSClient()
	h.add(c)

	// 填满 send 队列（不消费），再多投一条 → 该慢端被 kill（closed 关闭），broadcast 不阻塞。
	for i := 0; i < clientSendBuffer; i++ {
		c.send <- []byte("x")
	}
	done := make(chan struct{})
	go func() {
		h.broadcast(events.TerminalOutput, events.TerminalOutputPayload{SessionID: "s"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on slow client (backpressure leak)")
	}
	select {
	case <-c.closed:
		// 已剔除，符合预期。
	default:
		t.Fatal("slow client was not evicted")
	}
}

func TestRouteEventWhitelist(t *testing.T) {
	g := newTestGateway("tok")
	c := newWSClient()
	g.hub.add(c)

	// 白名单内：转发。
	g.RouteEvent(events.OrchestratorLaunch, events.OrchestratorLaunchPayload{SessionID: "s1"})
	select {
	case <-c.send:
	default:
		t.Fatal("whitelisted event was not forwarded")
	}

	// 白名单外：丢弃，不进队列。
	g.RouteEvent("demo:unlisted", "12:00")
	select {
	case <-c.send:
		t.Fatal("non-whitelisted event leaked to ws")
	default:
	}
}

// TestWSEndToEnd 走完整 upgrade：dial（带 cookie）→ RouteEvent → 浏览器收到 envelope。
func TestWSEndToEnd(t *testing.T) {
	g := newTestGateway("tok")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {cookieName + "=" + g.sessionCookieValue()}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// 等服务端 goroutine 把该连接登记进 hub（dial 返回与 add 之间有微小竞态）。
	if !waitFor(func() bool { return g.hub.clientCount() == 1 }, 2*time.Second) {
		t.Fatal("client not registered in hub")
	}

	g.RouteEvent(events.TerminalStatus, events.TerminalStatusPayload{SessionID: "s9", Status: "active"})

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env struct {
		Event string                       `json:"event"`
		Data  events.TerminalStatusPayload `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Event != events.TerminalStatus || env.Data.SessionID != "s9" {
		t.Fatalf("env = %+v", env)
	}
}

func TestWSRequiresPairing(t *testing.T) {
	g := newTestGateway("tok")
	srv := httptest.NewServer(g.buildMux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	// 不带 cookie → requirePaired 401 → upgrade 失败。
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("ws dial without cookie should fail")
	}
}

// waitFor 轮询 cond 直到为 true 或超时（避免裸 sleep 的竞态/慢）。
func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
