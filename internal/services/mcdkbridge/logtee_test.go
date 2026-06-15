package mcdkbridge

import (
	"net"
	"testing"
	"time"
)

// readDrain 收到 PacketLogLine 帧应回调 logHandler，payload 原样透传；其余帧类型不回调。
func TestReadDrainRoutesLogFrame(t *testing.T) {
	s := New()
	got := make(chan string, 4)
	s.SetLogHandler(func(line string) { got <- line })

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
	case line := <-got:
		if line != "[ContainerTreasure] hi" {
			t.Fatalf("回调收到 %q，期望 [ContainerTreasure] hi", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logHandler 未在超时内收到日志帧")
	}

	_ = c2.Close()
	<-done
}
