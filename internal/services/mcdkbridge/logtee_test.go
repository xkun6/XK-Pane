package mcdkbridge

import (
	"net"
	"testing"
	"time"
)

// readDrain 收到 PacketLogLine 帧应回调 logHandler，payload 原样透传；其余帧类型不回调。
// 未发 hello 帧时归属 runID 为空串。
func TestReadDrainRoutesLogFrame(t *testing.T) {
	s := New()
	type call struct{ runID, line string }
	got := make(chan call, 4)
	s.SetLogHandler(func(runID, line string) { got <- call{runID, line} })

	c1, c2 := net.Pipe()
	done := make(chan struct{})
	go func() { s.readDrain(c1, 1); close(done) }()

	// 先发一帧非日志类型（应被忽略），再发一帧 PacketLogLine（应回调）。
	if _, err := c2.Write(NewPacket(PacketReloadMod, nil).Encode()); err != nil {
		t.Fatalf("write reload frame: %v", err)
	}
	if _, err := c2.Write(NewPacket(PacketLogLine, []byte("[ContainerTreasure] hi")).Encode()); err != nil {
		t.Fatalf("write log frame: %v", err)
	}

	select {
	case c := <-got:
		if c.line != "[ContainerTreasure] hi" {
			t.Fatalf("回调收到 %q，期望 [ContainerTreasure] hi", c.line)
		}
		if c.runID != "" {
			t.Fatalf("未发 hello 帧时 runID 应为空串，得 %q", c.runID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logHandler 未在超时内收到日志帧")
	}

	_ = c2.Close()
	<-done
}

// 先发 hello 帧（runId 归属）再发日志帧：日志回调应带上 hello 帧回传的 runID。
func TestReadDrainRoutesHelloThenLogFrame(t *testing.T) {
	s := New()
	type call struct{ runID, line string }
	got := make(chan call, 4)
	s.SetLogHandler(func(runID, line string) { got <- call{runID, line} })

	c1, c2 := net.Pipe()
	done := make(chan struct{})
	go func() { s.readDrain(c1, 1); close(done) }()

	if _, err := c2.Write(NewPacket(PacketHello, []byte("run-x")).Encode()); err != nil {
		t.Fatalf("write hello frame: %v", err)
	}
	if _, err := c2.Write(NewPacket(PacketLogLine, []byte("hello world")).Encode()); err != nil {
		t.Fatalf("write log frame: %v", err)
	}

	select {
	case c := <-got:
		if c.runID != "run-x" {
			t.Fatalf("hello 后日志帧 runID=%q，期望 run-x", c.runID)
		}
		if c.line != "hello world" {
			t.Fatalf("回调收到 %q，期望 hello world", c.line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logHandler 未在超时内收到日志帧")
	}

	_ = c2.Close()
	<-done
}
