package mcdkbridge

import (
	"errors"
	"net"
	"os"
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

// sendHello 模拟游戏内桥模组连上后发的第一帧（payload=runId）。
func sendHello(t *testing.T, conn net.Conn, runID string) {
	t.Helper()
	if _, err := conn.Write(Packet{Type: PacketHello, Payload: []byte(runID)}.Encode()); err != nil {
		t.Fatalf("write hello: %v", err)
	}
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
	got, err := readFrom(client)
	if err != nil {
		t.Fatalf("client read frame: %v", err)
	}
	if !packetEqual(got, ReloadGame()) {
		t.Fatalf("got %+v want ReloadGame", got)
	}
}

// 多开并存：第二个连接不再顶替第一个（旧版单 active client 语义已废）。
// Status() 仍报告最新连入者，兼容单开调用方。
func TestSecondConnectionCoexists(t *testing.T) {
	s := startBridge(t)
	first := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })
	firstAddr := s.Status().RemoteAddr

	dialBridge(t, s)
	waitFor(t, func() bool {
		st := s.Status()
		return st.Connected && st.RemoteAddr != firstAddr
	})

	// 第一个 client 不该被踢：短暂读超时应超时而非 EOF（server 未关它的 socket）。
	_ = first.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 32)
	if _, err := first.Read(buf); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("first client expected read timeout (仍连着), got err=%v", err)
	}

	st := s.Status()
	if !st.Connected || st.RemoteAddr == firstAddr {
		t.Fatalf("expected Status() 报告最新（第二个）client, got %+v", st)
	}
}

// 两 client 各发 hello 后应能各自按 runID 查状态、精确定向发帧；未知 runID 报 CodeBridgeNoClient。
func TestSendPacketToRunTargetsMatchingClient(t *testing.T) {
	s := startBridge(t)
	connA := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })
	sendHello(t, connA, "run-a")
	waitFor(t, func() bool { return s.StatusForRun("run-a").Connected })

	connB := dialBridge(t, s)
	waitFor(t, func() bool { return len(s.ClientRunIDs()) == 2 })
	sendHello(t, connB, "run-b")
	waitFor(t, func() bool { return s.StatusForRun("run-b").Connected })

	if err := s.SendPacketToRun("run-a", ReloadGame()); err != nil {
		t.Fatalf("send to run-a: %v", err)
	}
	_ = connA.SetReadDeadline(time.Now().Add(time.Second))
	got, err := readFrom(connA)
	if err != nil {
		t.Fatalf("connA read: %v", err)
	}
	if !packetEqual(got, ReloadGame()) {
		t.Fatalf("connA got %+v want ReloadGame", got)
	}

	// connB 不该收到发给 run-a 的帧。
	_ = connB.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 8)
	if _, err := connB.Read(buf); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("connB 不该收到 run-a 的帧, got err=%v", err)
	}

	if err := s.SendPacketToRun("run-zzz", ReloadGame()); codeOf(err) != CodeBridgeNoClient {
		t.Fatalf("未知 runID 应报 CodeBridgeNoClient，got %v", err)
	}
}

// ClientRunIDs 应按连接建立顺序（连接 id 升序）返回。
func TestClientRunIDsOrderedByConnection(t *testing.T) {
	s := startBridge(t)
	connA := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })
	sendHello(t, connA, "run-a")
	waitFor(t, func() bool { return s.StatusForRun("run-a").Connected })

	connB := dialBridge(t, s)
	waitFor(t, func() bool { return len(s.ClientRunIDs()) == 2 })
	sendHello(t, connB, "run-b")
	waitFor(t, func() bool { return s.StatusForRun("run-b").Connected })

	ids := s.ClientRunIDs()
	if len(ids) != 2 || ids[0] != "run-a" || ids[1] != "run-b" {
		t.Fatalf("ClientRunIDs=%v，want [run-a run-b]（按连接顺序）", ids)
	}
}

func TestSendWithoutClientErrors(t *testing.T) {
	s := startBridge(t)
	err := s.SendPacket(ReloadGame())
	if got := codeOf(err); got != CodeBridgeNoClient {
		t.Fatalf("code=%q want %q", got, CodeBridgeNoClient)
	}
}

// 多开场景下关掉其一，只清那一个的状态，另一个不受影响。
func TestClientDisconnectClearsState(t *testing.T) {
	s := startBridge(t)
	connA := dialBridge(t, s)
	waitFor(t, func() bool { return s.Status().Connected })
	sendHello(t, connA, "run-a")
	waitFor(t, func() bool { return s.StatusForRun("run-a").Connected })

	connB := dialBridge(t, s)
	waitFor(t, func() bool { return len(s.ClientRunIDs()) == 2 })
	sendHello(t, connB, "run-b")
	waitFor(t, func() bool { return s.StatusForRun("run-b").Connected })

	_ = connA.Close()
	waitFor(t, func() bool { return !s.StatusForRun("run-a").Connected })
	if !s.StatusForRun("run-b").Connected {
		t.Fatal("关闭 run-a 不应影响 run-b 的连接状态")
	}
}
