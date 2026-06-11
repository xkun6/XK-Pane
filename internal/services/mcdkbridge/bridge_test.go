package mcdkbridge

import (
	"net"
	"strconv"
	"testing"
	"time"
)

func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("waitFor: condition not met within ~1s")
}

func startBridge(t *testing.T) *Service {
	t.Helper()
	s := New()
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

func dialBridge(t *testing.T, s *Service) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port())))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestStartAssignsPort(t *testing.T) {
	s := startBridge(t)
	if s.Port() <= 0 {
		t.Fatalf("port should be ephemeral non-zero, got %d", s.Port())
	}
}

func TestAcceptMarksClientConnected(t *testing.T) {
	s := startBridge(t)
	dialBridge(t, s)

	waitFor(t, func() bool { return s.Status().Connected })
	st := s.Status()
	if !st.Connected {
		t.Fatal("expected connected")
	}
	if st.RemoteAddr == "" {
		t.Fatal("expected non-empty remote addr")
	}
	if st.ConnectedAtMs <= 0 {
		t.Fatalf("expected connectedAtMs > 0, got %d", st.ConnectedAtMs)
	}
}

func TestSendPacketWritesFrame(t *testing.T) {
	s := startBridge(t)
	client := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })

	if err := s.SendPacket(ReloadGame()); err != nil {
		t.Fatalf("send: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	got, err := ReadFrom(client)
	if err != nil {
		t.Fatalf("client read frame: %v", err)
	}
	if !packetEqual(got, ReloadGame()) {
		t.Fatalf("got %+v want ReloadGame", got)
	}
}

func TestSecondConnectionKicksFirst(t *testing.T) {
	s := startBridge(t)
	first := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })
	firstAddr := s.Status().RemoteAddr

	dialBridge(t, s)
	waitFor(t, func() bool {
		st := s.Status()
		return st.Connected && st.RemoteAddr != firstAddr
	})

	// 第一个 client 被踢后 read 应返回 EOF（server 关了它的 socket）。
	_ = first.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 32)
	n, err := first.Read(buf)
	if n != 0 || err == nil {
		t.Fatalf("first client expected EOF after kick, got n=%d err=%v", n, err)
	}

	st := s.Status()
	if !st.Connected || st.RemoteAddr == firstAddr {
		t.Fatalf("expected active switched to second client, got %+v", st)
	}
}

func TestSendWithoutClientErrors(t *testing.T) {
	s := startBridge(t)
	err := s.SendPacket(ReloadGame())
	if got := codeOf(err); got != CodeBridgeNoClient {
		t.Fatalf("code=%q want %q", got, CodeBridgeNoClient)
	}
}

func TestClientDisconnectClearsState(t *testing.T) {
	s := startBridge(t)
	client := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })

	_ = client.Close()
	waitFor(t, func() bool { return !s.Status().Connected })
}
