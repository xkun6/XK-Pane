package mcpe

// launcher.go — 启动 Minecraft.Windows.exe（经 ptyx ConPTY）+ 逐行过滤转发。忠实移植 launcher.rs。
//
// 为什么用 PTY 而非普通管道：普通管道下 MC PE 内嵌的 Python 2.7 拿不到真 console handle，
// 会把 [Python]/[ContainerTreasure] 等 mod 层 stdout 丢弃。ConPTY 让它看到真 console，
// 全部 stdout（含 mod print）正常流到 PTY master。PTY 下 stdout/stderr 合并为单一通道。

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/utils/logx"
	"xkpane/internal/utils/ptyx"
)

const (
	mcPtyCols    = 200 // 与 cc-pane 一致（影响 ConPTY 换行/列宽）
	mcPtyRows    = 50
	mcDrainGrace = time.Second // Wait 后给 reader 收尾的宽限再 Close（ConPTY 需 Close 才让 reader EOF）
)

// ConPTY 会让 MC PE 输出 VT100 转义（颜色/光标/CSI 清屏），解析前先剥掉。覆盖 CSI / OSC / Fe 转义。
var ansiRE = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*\x07|[@-_])`)

// emitFunc 是事件发射器（app.Event.Emit / mirrorEmit 注入）。
type emitFunc func(name string, payload any)

// runSession 一次运行的句柄。
type runSession struct {
	runID     string
	packageID string
	pid       *int
	mu        sync.Mutex
	pty       *ptyx.PTY // wait goroutine 完成或 kill 后置 nil
}

// kill 通知进程退出；Wait + Close + emit exited 由 wait goroutine 负责。
func (s *runSession) kill() {
	s.mu.Lock()
	p := s.pty
	s.mu.Unlock()
	if p != nil {
		if err := p.Kill(); err != nil {
			logx.For("mcpe").Warn("kill MC PE 进程失败", "err", err)
		}
	}
}

// spawnRun 起 MC PE 进程（ConPTY），按行过滤 stdout → emit mc_pe:output；退出 → emit mc_pe:exited。
func spawnRun(runID, packageID, exe string, args []string, mcEnv map[string]string, emit emitFunc) (*runSession, error) {
	pt, err := ptyx.Spawn(context.Background(), ptyx.Spec{
		Command: exe,
		Args:    args,
		Cwd:     filepath.Dir(exe), // 工作目录 = exe 所在版本目录
		Env:     mergeEnv(os.Environ(), mcEnv),
		Cols:    mcPtyCols,
		Rows:    mcPtyRows,
	})
	if err != nil {
		return nil, fmt.Errorf("启动 %s 失败: %w", exe, err)
	}
	pid := pt.PID()
	var pidPtr *int
	if pid > 0 {
		pidPtr = &pid
	}
	s := &runSession{runID: runID, packageID: packageID, pid: pidPtr, pty: pt}

	readerDone := make(chan struct{})
	go readLoop(pt, runID, packageID, emit, readerDone)

	go func() {
		code := exitCode(pt.Wait())
		logx.For("mcpe").Info("MC PE 进程退出", "code", code, "runId", runID)
		s.mu.Lock()
		s.pty = nil
		s.mu.Unlock()
		// 铁律：Wait → (宽限让 reader 收尾) → Close（ConPTY 需 Close 才让 reader EOF），否则丢尾部输出。
		select {
		case <-readerDone:
		case <-time.After(mcDrainGrace):
		}
		_ = pt.Close()
		<-readerDone
		emit(events.MCPeExited, events.MCExitedPayload{RunID: runID, PackageID: packageID, Code: code})
	}()
	return s, nil
}

// readLoop 同步读 PTY master，逐行剥 ANSI + lossy 解码 + 白名单过滤后 emit。
func readLoop(pt *ptyx.PTY, runID, packageID string, emit emitFunc, done chan struct{}) {
	defer close(done)
	br := bufio.NewReaderSize(pt, 4096)
	for {
		lineBytes, err := br.ReadBytes('\n')
		if len(lineBytes) > 0 {
			trimmed := bytes.TrimRight(lineBytes, "\r\n")
			cleaned := ansiRE.ReplaceAll(trimmed, nil)
			// MC 在 Windows 输出混合编码（GBK+UTF-8），lossy 解码避免非法字节丢整行
			// （忠实 cc-pane from_utf8_lossy；若中文乱码再上 GBK 解码，见 docs/archive/P6e_PLAN.md 待澄清）。
			line := strings.ToValidUTF8(string(cleaned), "�")
			if parsed, ok := parseStdoutLine(line); ok {
				emit(events.MCPeOutput, events.MCOutputPayload{
					RunID:     runID,
					PackageID: packageID,
					Line:      parsed.Display,
					Severity:  string(parsed.Severity),
					Timestamp: strPtr(parsed.Timestamp),
					Category:  strPtr(parsed.Category),
					Level:     strPtr(parsed.Level),
				})
			}
		}
		if err != nil {
			return // EOF / pty 已关
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// mergeEnv 把 extra 叠加到 base（K=V 列表），extra 覆盖同名键。对应 cc-pane 在继承环境上 cmd.env() 追加。
func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(extra))
	used := make(map[string]bool, len(extra))
	for _, kv := range base {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			if v, ok := extra[kv[:eq]]; ok {
				out = append(out, kv[:eq]+"="+v)
				used[kv[:eq]] = true
				continue
			}
		}
		out = append(out, kv)
	}
	for k, v := range extra {
		if !used[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
