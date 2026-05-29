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
//
// 注意 ConPTY 输出语义：它是**异步屏幕刷新**，不是简单管道。进程退出后若立刻
// Close() 会在最后一次刷新前关掉伪控制台、丢掉尾部输出（这正是 Phase 2 终端服务
// 要在进程退出后留 drain 宽限再关 PTY 的原因）。因此本测试**轮询缓冲区等 marker
// 出现**，不依赖 Wait/Close 时序，避免 race。
func TestSpawnEchoProducesOutput(t *testing.T) {
	const marker = "ptyx-spike-OK-7421"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd, args := echoCmd(marker)
	p, err := Spawn(ctx, Spec{Command: cmd, Args: args, Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Kill(); _ = p.Wait(); _ = p.Close() }()

	if p.PID() == 0 {
		t.Fatalf("PID() = 0 after Spawn")
	}

	var (
		mu  sync.Mutex
		buf bytes.Buffer
	)
	go func() {
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

	// 轮询：reader 持续读，等 marker 出现（不提前 Close，规避 ConPTY 尾部刷新 race）。
	const (
		deadline = 12 * time.Second
		step     = 25 * time.Millisecond
	)
	for waited := time.Duration(0); waited < deadline; waited += step {
		mu.Lock()
		has := strings.Contains(buf.String(), marker)
		mu.Unlock()
		if has {
			return // 成功
		}
		time.Sleep(step)
	}

	mu.Lock()
	out := buf.String()
	mu.Unlock()
	t.Fatalf("marker %q not seen within %s; got %d bytes: %q", marker, deadline, len(out), out)
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
