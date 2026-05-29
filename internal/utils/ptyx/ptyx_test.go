package ptyx

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// echoCmd 跨平台地构造一个「打印 marker 后退出」的命令。
func echoCmd(marker string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/c", "echo " + marker}
	}
	return "/bin/sh", []string{"-c", "echo " + marker}
}

// TestSpawnEchoProducesOutput 是 ptyx 的 ConPTY/PTY 冒烟测试（R1 spike 守门）：
// 在真 PTY 里跑 echo，断言能从 PTY 原始字节流读到 marker。
// 同时验证 spawn → 读流 → Wait → Close 的生命周期不卡死。
func TestSpawnEchoProducesOutput(t *testing.T) {
	const marker = "ptyx-spike-OK-7421"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd, args := echoCmd(marker)
	p, err := Spawn(ctx, Spec{Command: cmd, Args: args, Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if p.PID() == 0 {
		t.Fatalf("PID() = 0 after Spawn")
	}

	var (
		mu  sync.Mutex
		buf bytes.Buffer
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		tmp := make([]byte, 4096)
		for {
			n, rerr := p.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf.Write(tmp[:n])
				mu.Unlock()
			}
			if rerr != nil {
				return // EOF / closed
			}
		}
	}()

	if werr := p.Wait(); werr != nil {
		// echo 正常退出码 0；非 0 仅告警（不同 shell 噪声），不直接失败
		t.Logf("Wait returned: %v", werr)
	}
	_ = p.Close() // 关 PTY，让 reader 收到 EOF

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reader goroutine did not finish after Close (PTY read blocked)")
	}

	mu.Lock()
	out := buf.String()
	mu.Unlock()
	if !strings.Contains(out, marker) {
		t.Fatalf("expected PTY output to contain %q, got %d bytes: %q", marker, len(out), out)
	}
}

// TestResizeNoError 验证 Resize 在会话存活期间可调用且 0 尺寸被安全忽略。
func TestResizeNoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 跑一个会停留片刻的命令，确保 Resize 时进程还活着。
	var cmd string
	var args []string
	if runtime.GOOS == "windows" {
		cmd, args = "cmd.exe", []string{"/c", "ping -n 2 127.0.0.1 >NUL"}
	} else {
		cmd, args = "/bin/sh", []string{"-c", "sleep 1"}
	}
	p, err := Spawn(ctx, Spec{Command: cmd, Args: args})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	// 持续读掉输出，避免 PTY 缓冲塞满阻塞子进程。
	go func() { _, _ = io.Copy(io.Discard, p) }()

	if err := p.Resize(0, 0); err != nil {
		t.Fatalf("Resize(0,0) should be ignored, got: %v", err)
	}
	if err := p.Resize(100, 40); err != nil {
		t.Fatalf("Resize(100,40): %v", err)
	}
	_ = p.Wait()
	_ = p.Close()
}
