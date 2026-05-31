// Package terminal 是终端核心服务：在 PTY 中以选定 CLI 工具（或裸 shell）启动交互式
// 会话，把 PTY 原始字节流批量推送给前端 xterm.js，并管理会话生命周期（resize/kill/list）。
//
// 这是 cc-pane `cc-panes-core/src/services/terminal_service.rs` 的**最小核心移植**
// （Phase 2 MVP）。刻意延后到后续 Phase 的部分（均以 TODO 标注）：
//   - 输出/回放/dead 缓冲区（会话恢复、attach-existing）→ Phase 3
//   - SSH / WSL 运行时、launch profile、spec prompt 注入、Provider 选择 → Phase 3/4
//   - MCP 注入、orchestrator HTTP 回调、launch_cwd 上报接收端 → Phase 5
//   - ConPTY 光标伪影 sanitize、GBK 回退、chunked writer、状态推断(idle/waiting_input) → 后续
//
// I/O 通道铁律（findings J2）：终端必须走 PTY 原始字节流（含 ANSI/光标控制序列），
// 不能用 procx 的行回调——行缓冲会破坏 TUI 渲染。因此本服务包 internal/utils/ptyx。
//
// 关闭顺序铁律（ptyx 测试暴露的 ConPTY 异步刷新教训）：子进程退出后必须留 drain 宽限，
// 等 reader 把缓冲区里的尾部输出读尽再 Close PTY，否则丢尾部输出。见 waitLoop。
package terminal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"xkpane/internal/events"
	"xkpane/internal/services/cliadapters"
	"xkpane/internal/utils/logx"
	"xkpane/internal/utils/ptyx"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// readBufSize 是 PTY reader 单次读取缓冲区。64KB 偏大，减少高吞吐时的 read 次数。
	readBufSize = 64 * 1024
	// batchSizeThreshold / batchTimeout：累积 ≥16KB 或 ≥16ms 刷出一次 terminal:output，
	// 降低 IPC 事件频率（防 webview 主线程被高频事件压垮），同时保证低吞吐下不滞留。
	batchSizeThreshold = 16 * 1024
	batchTimeout       = 16 * time.Millisecond
	// batchChanCap 给 reader→batcher 段一个确定的内存上限（背压）。
	batchChanCap = 32
	// drainGraceTimeout 是子进程退出后等待 reader 读尽尾部输出的安全上限；
	// 正常情况下 reader 会在 PTY EOF 时自然退出，远早于此超时。超时则强制 Close 兜底，
	// 避免孤儿孙进程占住 PTY 导致 reader 永不返回、会话永久驻留。
	drainGraceTimeout = 2 * time.Second
	// deadRetention 是会话退出后输出缓冲在 dead-buffers 里的保留时长（移植 cc-pane 5 分钟）。
	deadRetention = 5 * time.Minute
)

// ProviderResolver 据 providerId 解析启动所需：Tier-A 启动 env + Tier-B adapter
// Provider 上下文（含解密明文 key）。由 app 装配（闭包调 provider.ResolveLaunchEnv +
// holder）；nil = 无 provider 支持（裸单测）。**绝不暴露给前端**——明文经此进 PTY env。
type ProviderResolver func(ctx context.Context, providerID string) (env map[string]string, cli *cliadapters.Provider, err error)

// Service 是 Wails 终端服务，前端通过生成的 bindings 调用。
type Service struct {
	registry *cliadapters.Registry
	logger   *slog.Logger

	// resolveProvider 把 providerId 解析成启动 env + adapter Provider 上下文；
	// nil 时跳过 provider 注入（与 ProviderID=="" 等效）。由 SetProviderResolver 注入。
	resolveProvider ProviderResolver

	// sessionStartHook 在会话成功创建后回调（参数 = 该会话的 ProjectPath = launchProject 目标）。
	// 供 lhwatcher 在「启动项目」时补启自动捕获（决策 D4-4）；nil = 无（裸单测 / app 未接线）。
	sessionStartHook func(projectPath string)

	// emit 把事件推给前端；nil 时丢弃（unit test 或 app 未就绪）。
	// 用字段而非直接持 *application.App，便于测试注入捕获回调。
	emitMu sync.RWMutex
	emit   func(name string, payload any)

	// orchestrator MCP 端点（P6 MCP 注入用）：app 装配阶段经 SetOrchestrator 注入。
	// 0/"" = orchestrator 未就绪（CLI 不注入 panes MCP，fail-open 裸跑）。P6b 起 resolveLaunch 据此填
	// cliadapters.Context 并翻转 SkipMCP；P6a 仅存储（地基阶段还不注入）。
	orchMu    sync.RWMutex
	orchPort  int
	orchToken string

	mu       sync.Mutex
	sessions map[string]*session

	// deadBuffers 保存已退出会话的输出缓冲，供 get_session_output 在退出后短时间（deadRetention）回看。
	// 懒清理：SessionOutput 读 dead 时顺带剔除过期条目（移植 cc-pane dead_buffers）。
	deadMu      sync.Mutex
	deadBuffers map[string]*deadBuffer
}

// deadBuffer 是已退出会话的输出缓冲快照 + 退出时刻（用于 deadRetention 过期判定）。
type deadBuffer struct {
	buf       *outputBuffer
	createdAt time.Time
}

// SetProviderResolver 注入 provider 解析器（app 装配阶段调，镜像 AttachApp 模式）。
func (s *Service) SetProviderResolver(r ProviderResolver) {
	s.resolveProvider = r
}

// SetSessionStartHook 注入「会话启动 → 补启 Local History watcher」回调（app 装配阶段调）。
func (s *Service) SetSessionStartHook(h func(projectPath string)) {
	s.sessionStartHook = h
}

// SetOrchestrator 注入本地 orchestrator 的 MCP 端点 port/token（app 装配阶段调，orchestrator 启动后）。
// P6b 起 resolveLaunch 据此填 cliadapters.Context（OrchestratorPort/Token）并翻转 SkipMCP。
func (s *Service) SetOrchestrator(port int, token string) {
	s.orchMu.Lock()
	s.orchPort = port
	s.orchToken = token
	s.orchMu.Unlock()
}

// orchestratorEndpoint 返回已注入的 orchestrator MCP 端点（port/token）；(0,"") = 未就绪。
// port/token 在 app 装配阶段一次性 set，之后不变。
func (s *Service) orchestratorEndpoint() (int, string) {
	s.orchMu.RLock()
	defer s.orchMu.RUnlock()
	return s.orchPort, s.orchToken
}

// session 是一次终端会话的内部状态。
type session struct {
	id      string
	pty     *ptyx.PTY
	pid     int
	cwd     string
	cliTool string

	// cancelled / cancelCh：Kill 时设置标志并关闭 channel，让 reader 的阻塞发送能确定性打断。
	cancelled atomic.Bool
	cancelCh  chan struct{}
	killOnce  sync.Once

	batchCh     chan string
	readerDone  chan struct{}
	batcherDone chan struct{}

	firstOutput atomic.Bool
	// outBuf 累积本会话纯文本输出（ANSI 已剥离）供 get_session_output；lastOutputMs 记最近输出时刻（unix ms）。
	outBuf       *outputBuffer
	lastOutputMs atomic.Int64
}

// New 构造终端服务。registry 提供 CLI 工具命令拼装（Phase 1 cliadapters）。
// app 引用通过 AttachApp 在 application.New 之后回填（与 subprocess service 一致）。
func New(registry *cliadapters.Registry) *Service {
	return &Service{
		registry:    registry,
		logger:      logx.For("terminal"),
		sessions:    make(map[string]*session),
		deadBuffers: make(map[string]*deadBuffer),
	}
}

// AttachApp 回填 app 引用，让会话能 Emit 事件给前端。不调用则前端收不到任何输出。
// 包一层适配：Wails 的 Event.Emit 是 func(name string, data ...any) bool，
// 这里收敛成内部 emit 字段的 func(name string, payload any) 签名（忽略返回值）。
func (s *Service) AttachApp(app *application.App) {
	s.setEmit(func(name string, payload any) {
		app.Event.Emit(name, payload)
	})
}

func (s *Service) setEmit(fn func(name string, payload any)) {
	s.emitMu.Lock()
	s.emit = fn
	s.emitMu.Unlock()
}

func (s *Service) emitEvent(name string, payload any) {
	s.emitMu.RLock()
	fn := s.emit
	s.emitMu.RUnlock()
	if fn == nil {
		return
	}
	fn(name, payload)
}

// CreateSpec 是前端发来的会话启动请求（Phase 2 最小集）。
type CreateSpec struct {
	// ProjectPath = PTY 的 cwd，永远等于用户操作目标（路径字段铁律 D7）。必填。
	ProjectPath string `json:"projectPath"`
	// WorkspacePath 仅多目录 CLI 拼 --add-dir，且仅当 ≠ProjectPath（铁律 D7）。"" = 无。
	WorkspacePath string `json:"workspacePath"`
	// CliTool 适配器 id（claude/codex/...）；"" 或 "none" = 裸 shell。
	CliTool string `json:"cliTool"`
	// ProviderID 选定的接入 Provider id；"" = 不注入 provider env。后端经注入的
	// ProviderResolver 解析为 Tier-A 启动 env + Tier-B adapter Provider 上下文。
	ProviderID string `json:"providerId"`
	// Cols/Rows 初始终端尺寸；0 取 ptyx 默认。前端拿到真实尺寸后应立即 Resize。
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
	// InitialPrompt 初始用户 prompt（作为 CLI 位置参数）；"" = 无。
	InitialPrompt string `json:"initialPrompt"`
	// ResumeID 恢复已有会话的 id；"" = 新会话。明文透传给 adapter（claude --resume /
	// codex resume / reclaude --resume）。CLI resume 必需明文，不加密（拍板点 5）。
	ResumeID string `json:"resumeId"`
}

// SessionInfo 是 List 返回的会话快照。
type SessionInfo struct {
	ID      string `json:"id"`
	PID     int    `json:"pid"`
	Cwd     string `json:"cwd"`
	CliTool string `json:"cliTool"`
}

// CreateSession 在 ProjectPath 下用 CliTool（或裸 shell）启动 PTY 会话，返回 sessionId。
// 启动后输出经 terminal:output 事件流式推送，退出经 terminal:exit 通知。
func (s *Service) CreateSession(ctx context.Context, spec CreateSpec) (string, error) {
	if strings.TrimSpace(spec.ProjectPath) == "" {
		return "", errors.New("terminal: empty projectPath")
	}

	id := newSessionID()

	command, args, providerEnv, envInject, envRemove, err := s.resolveLaunch(ctx, id, spec)
	if err != nil {
		return "", err
	}

	orchPort, orchToken := s.orchestratorEndpoint()
	env := buildEnv(id, spec.CliTool, orchPort, orchToken, providerEnv, envInject, envRemove)

	pt, err := ptyx.Spawn(context.Background(), ptyx.Spec{
		Command: command,
		Args:    args,
		Cwd:     spec.ProjectPath,
		Env:     env,
		Cols:    spec.Cols,
		Rows:    spec.Rows,
	})
	if err != nil {
		s.logger.Error("spawn failed", "session", id, "command", command, "err", err)
		return "", fmt.Errorf("terminal: spawn %s: %w", command, err)
	}

	sess := &session{
		id:          id,
		pty:         pt,
		pid:         pt.PID(),
		cwd:         spec.ProjectPath,
		cliTool:     spec.CliTool,
		cancelCh:    make(chan struct{}),
		batchCh:     make(chan string, batchChanCap),
		readerDone:  make(chan struct{}),
		batcherDone: make(chan struct{}),
		outBuf:      newOutputBuffer(liveOutputMaxLines, liveOutputMaxBytes),
	}

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	s.logger.Info("session created", "session", id, "pid", sess.pid, "command", command, "cwd", spec.ProjectPath)

	// 通知 watcher 在该项目补启自动捕获（launchProject 时机，决策 D4-4）。异步避免阻塞会话创建。
	if hook := s.sessionStartHook; hook != nil {
		go hook(spec.ProjectPath)
	}

	go s.batchLoop(sess)
	go s.readLoop(sess)
	go s.waitLoop(sess)

	return id, nil
}

// WriteInput 把用户输入写入 PTY。
func (s *Service) WriteInput(_ context.Context, sessionID string, data string) error {
	sess, ok := s.get(sessionID)
	if !ok {
		return fmt.Errorf("terminal: unknown session %q", sessionID)
	}
	if _, err := sess.pty.Write([]byte(data)); err != nil {
		return fmt.Errorf("terminal: write to %s: %w", sessionID, err)
	}
	return nil
}

// Resize 调整 PTY 尺寸（cols=宽, rows=高）。
func (s *Service) Resize(_ context.Context, sessionID string, cols, rows uint16) error {
	sess, ok := s.get(sessionID)
	if !ok {
		return fmt.Errorf("terminal: unknown session %q", sessionID)
	}
	return sess.pty.Resize(cols, rows)
}

// Kill 硬终止会话（子进程组一起死）。幂等：未知/已退出会话返回 nil。
func (s *Service) Kill(_ context.Context, sessionID string) error {
	sess, ok := s.get(sessionID)
	if !ok {
		return nil
	}
	s.killSession(sess)
	return nil
}

// List 返回当前活跃会话快照。
func (s *Service) List(_ context.Context) ([]SessionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, SessionInfo{ID: sess.id, PID: sess.pid, Cwd: sess.cwd, CliTool: sess.cliTool})
	}
	return out, nil
}

// SessionOutput 是 get_session_output 的返回：会话最近 N 行纯文本（ANSI 已剥离）。
type SessionOutput struct {
	SessionID string   `json:"sessionId"`
	Lines     []string `json:"lines"`
}

// SessionSnapshot 是 list_sessions / get_session_status 的单会话视图（活跃会话）。
type SessionSnapshot struct {
	ID           string `json:"id"`
	PID          int    `json:"pid"`
	Cwd          string `json:"cwd"`
	CliTool      string `json:"cliTool"`
	Status       string `json:"status"`       // 活跃会话恒 "active"（XK 暂不推断 idle/waiting）
	LastOutputAt int64  `json:"lastOutputAt"` // 最近输出时刻 unix ms；无输出则为创建后的 0
}

// SessionSnapshots 返回当前活跃会话快照（含 status + 最近输出时刻），供 orchestrator
// list_sessions / get_session_status。已退出会话不在此列（其输出仍可经 SessionOutput 读 dead-buffers）。
func (s *Service) SessionSnapshots() []SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionSnapshot, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, SessionSnapshot{
			ID:           sess.id,
			PID:          sess.pid,
			Cwd:          sess.cwd,
			CliTool:      sess.cliTool,
			Status:       "active",
			LastOutputAt: sess.lastOutputMs.Load(),
		})
	}
	return out
}

// SessionOutput 读会话最近 lines 行纯文本（lines<=0 = 全部）。先查活跃会话，未命中则查
// dead-buffers（退出后 deadRetention 内可读，懒清理过期条目）。均未命中返回 error。
func (s *Service) SessionOutput(sessionID string, lines int) (SessionOutput, error) {
	if sess, ok := s.get(sessionID); ok {
		return SessionOutput{SessionID: sessionID, Lines: sess.outBuf.getRecent(lines)}, nil
	}
	s.deadMu.Lock()
	defer s.deadMu.Unlock()
	now := time.Now()
	for id, d := range s.deadBuffers {
		if now.Sub(d.createdAt) >= deadRetention {
			delete(s.deadBuffers, id)
		}
	}
	if d, ok := s.deadBuffers[sessionID]; ok {
		return SessionOutput{SessionID: sessionID, Lines: d.buf.getRecent(lines)}, nil
	}
	return SessionOutput{}, fmt.Errorf("terminal: session %q not found or expired", sessionID)
}

// retireBuffer 把已退出会话的输出缓冲缩到 dead 上限后存入 dead-buffers（带退出时刻）。
func (s *Service) retireBuffer(sess *session) {
	sess.outBuf.shrink(deadOutputMaxLines, deadOutputMaxBytes)
	s.deadMu.Lock()
	s.deadBuffers[sess.id] = &deadBuffer{buf: sess.outBuf, createdAt: time.Now()}
	s.deadMu.Unlock()
}

func (s *Service) get(id string) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *Service) remove(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// killSession 设置取消标志、关闭 cancelCh 打断 reader 阻塞、硬杀进程组。幂等（killOnce）。
// 真正的资源回收（Close PTY / 移除会话 / emit exit）由 waitLoop 在 Wait 返回后统一处理。
func (s *Service) killSession(sess *session) {
	sess.killOnce.Do(func() {
		sess.cancelled.Store(true)
		close(sess.cancelCh)
		if err := sess.pty.Kill(); err != nil {
			s.logger.Warn("kill failed", "session", sess.id, "err", err)
		}
	})
}

// resolveLaunch 决定要在 PTY 里启动的命令：裸 shell 或经 cliadapters 拼装的 CLI。
// cwd 永远是 spec.ProjectPath（铁律 D7），由调用方传给 ptyx，不在此返回。
//
// Provider 注入两路（cc-pane terminal_service 双层）：① Tier-A 启动 env（providerEnv，
// 由注入的 resolveProvider 产出，在 buildEnv 里于 adapter EnvInject 之前合并）；
// ② Tier-B adapter 上下文 cctx.Provider（glm/cursor 自注入同名 env、kimi 据此写 per-session
// 配置）。ProviderID=="" 或无 resolver 时两路皆空。裸 shell 也应用 Tier-A env（无 adapter）。
func (s *Service) resolveLaunch(ctx context.Context, id string, spec CreateSpec) (command string, args []string, providerEnv map[string]string, envInject map[string]string, envRemove []string, err error) {
	var cli *cliadapters.Provider
	if spec.ProviderID != "" && s.resolveProvider != nil {
		penv, pc, perr := s.resolveProvider(ctx, spec.ProviderID)
		if perr != nil {
			return "", nil, nil, nil, nil, fmt.Errorf("terminal: resolve provider %s: %w", spec.ProviderID, perr)
		}
		providerEnv = penv
		cli = pc
	}

	toolID := spec.CliTool
	if toolID == "" || toolID == "none" {
		c, a := defaultShell()
		return c, a, providerEnv, nil, nil, nil
	}

	adapter, ok := s.registry.Get(toolID)
	if !ok {
		return "", nil, nil, nil, nil, fmt.Errorf("terminal: unknown cli tool %q", toolID)
	}

	// orchestrator 就绪（port/token 非空）才注入 panes MCP（翻转 SkipMCP）。未就绪 fail-open 裸跑，
	// 避免 adapter 生成依赖未落地端口的配置。claude/reclaude 写 <proj>/.claude/mcp-<sid>.json；
	// codex 经 -c mcp_servers.panes.url + bearer_token_env_var=XKPANE_API_TOKEN（env 由 buildEnv 注入）。
	orchPort, orchToken := s.orchestratorEndpoint()
	cctx := &cliadapters.Context{
		SessionID:         id,
		ProjectPath:       spec.ProjectPath,
		WorkspacePath:     spec.WorkspacePath,
		InitialPrompt:     spec.InitialPrompt,
		ResumeID:          spec.ResumeID, // 透传给 adapter → claude/codex/reclaude resume
		Provider:          cli,           // Tier-B：glm/cursor/kimi 据此自注入
		OrchestratorPort:  orchPort,
		OrchestratorToken: orchToken,
		SkipMCP:           orchPort == 0 || orchToken == "",
	}
	res, err := adapter.BuildCommand(cctx)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("terminal: build command for %s: %w", toolID, err)
	}
	return res.Command, res.Args, providerEnv, res.EnvInject, res.EnvRemove, nil
}

// readLoop 循环读 PTY 原始字节 → UTF-8 安全处理 → 投递给 batcher。
// 退出条件：cancelled / PTY EOF / read error。退出时关闭 batchCh（触发 batcher 收尾）
// 与 readerDone（通知 waitLoop reader 已读尽）。
func (s *Service) readLoop(sess *session) {
	defer close(sess.readerDone)
	defer close(sess.batchCh)

	buf := make([]byte, readBufSize)
	var carry []byte

	for {
		if sess.cancelled.Load() {
			return
		}
		n, err := sess.pty.Read(buf)
		if n > 0 {
			var data string
			data, carry = utf8SafeProcess(buf[:n], carry)
			if data != "" {
				sess.outBuf.push(data)              // 累积纯文本供 get_session_output
				sess.lastOutputMs.Store(nowMilli()) // 最近输出时刻（list_sessions/status 用）
				if sess.firstOutput.CompareAndSwap(false, true) {
					s.emitEvent(events.TerminalStatus, events.TerminalStatusPayload{
						SessionID:    sess.id,
						Status:       "active",
						LastOutputAt: nowMilli(),
						PID:          sess.pid,
					})
				}
				select {
				case sess.batchCh <- data:
				case <-sess.cancelCh:
					return
				}
			}
		}
		if err != nil {
			// EOF / closed pty / read error 都终止读取。EOF 是正常的进程退出路径。
			return
		}
	}
}

// batchLoop 累积 reader 投递的数据，按 ≥16KB 或 ≥16ms 刷出一次 terminal:output。
// batchCh 关闭后刷出残留并退出，关闭 batcherDone 通知 waitLoop 输出已全部 emit。
func (s *Service) batchLoop(sess *session) {
	defer close(sess.batcherDone)

	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		s.emitEvent(events.TerminalOutput, events.TerminalOutputPayload{
			SessionID: sess.id,
			Data:      b.String(),
		})
		b.Reset()
	}

	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-sess.batchCh:
			if !ok {
				flush()
				return
			}
			b.WriteString(data)
			if b.Len() >= batchSizeThreshold {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// waitLoop 等子进程退出，留 drain 宽限让 reader 读尽尾部输出再 Close PTY，
// 最后 emit terminal:exit / terminal:status(exited) 并移除会话。
//
// 顺序铁律：先等 reader 自然读尽（PTY EOF）→ 强制 Close 兜底 → 等 reader+batcher 收尾
// → emit exit。保证前端先收到全部输出，再收到退出事件，且不丢尾部输出。
func (s *Service) waitLoop(sess *session) {
	err := sess.pty.Wait()
	exitCode := exitCodeOf(err)

	// drain 宽限：进程已退出，但 ConPTY/PTY 缓冲里可能还有尾部输出未被 reader 读尽。
	// 正常情况 reader 很快读到 EOF 自行退出（readerDone 关闭）；超时则强制 Close 兜底。
	select {
	case <-sess.readerDone:
	case <-time.After(drainGraceTimeout):
		s.logger.Warn("drain grace timeout, forcing close", "session", sess.id)
	}

	// Close PTY：若 reader 仍阻塞在 Read 上，Close 会让其返回错误从而退出。
	if cerr := sess.pty.Close(); cerr != nil {
		s.logger.Warn("pty close error", "session", sess.id, "err", cerr)
	}

	// 确保 reader 与 batcher 都已收尾（所有 terminal:output 已 emit），再发退出事件。
	<-sess.readerDone
	<-sess.batcherDone

	s.remove(sess.id)
	s.retireBuffer(sess) // 输出缓冲转 dead-buffers，供退出后 deadRetention 内回看

	s.logger.Info("session exited", "session", sess.id, "exitCode", exitCode)
	s.emitEvent(events.TerminalExit, events.TerminalExitPayload{
		SessionID: sess.id,
		ExitCode:  exitCode,
	})
	s.emitEvent(events.TerminalStatus, events.TerminalStatusPayload{
		SessionID:    sess.id,
		Status:       "exited",
		LastOutputAt: nowMilli(),
		PID:          sess.pid,
	})
}

// exitCodeOf 从 ptyx.Wait 的返回值取退出码：nil=0，*exec.ExitError=进程码，其他=-1。
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func nowMilli() int64 { return time.Now().UnixMilli() }

// buildEnv 从父进程环境派生子进程完整环境（ptyx 要求 Env 非 nil 时完全替换）。
// 注入 TERM/COLORTERM（Release .app 从 Finder 启动不继承终端环境）与 XKPANE_* 标识，
// 合并 provider 的 Tier-A env，再叠加 adapter 的 envInject，最后移除 adapter 声明的 envRemove。
//
// orchestrator 端点 env（XKPANE_API_PORT/TOKEN/BASE_URL）在此对所有本地 CLI 会话统一注入
// （注入位置对齐 cc-pane terminal_service line 1192-1204，不在 adapter 里）：codex 经
// bearer_token_env_var=XKPANE_API_TOKEN 读 token，cli-hook（P6d）经 BASE_URL 回连。
// 注：env 名 CC_PANES_*→XKPANE_*、MCP server 名 ccpanes→panes 是有意偏离 cc-pane（换 XK 品牌、
// 与 cc-pane 共存不撞名）。
//
// 合并顺序（与 cc-pane terminal_service 一致）：base → providerEnv(Tier-A) →
// adapter inject(Tier-B) → remove。故 adapter 自注入覆盖 provider 同名键（如 glm ZAI_*
// 两层都设、值相同，无害）。
func buildEnv(sessionID, toolID string, orchPort int, orchToken string, providerEnv, inject map[string]string, remove []string) []string {
	m := make(map[string]string)
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	if _, ok := m["TERM"]; !ok {
		m["TERM"] = "xterm-256color"
	}
	if _, ok := m["COLORTERM"]; !ok {
		m["COLORTERM"] = "truecolor"
	}
	// XKPANE_PTY_SESSION_ID 是 launch_cwd 上报回路的关联键（cli-hook 据此上报 $PWD）。
	// TODO(phase5): 接收端 = orchestrator HTTP 回调，尚未移植；此处先注入以备后续启用。
	m["XKPANE_PTY_SESSION_ID"] = sessionID
	if toolID != "" && toolID != "none" {
		m["XKPANE_CLI_TOOL"] = toolID
	}
	// orchestrator MCP 端点（仅就绪时注入，本地全工具统一）。codex bearer / cli-hook 回连依赖此组。
	if orchPort != 0 && orchToken != "" {
		m["XKPANE_API_PORT"] = strconv.Itoa(orchPort)
		m["XKPANE_API_TOKEN"] = orchToken
		m["XKPANE_API_BASE_URL"] = fmt.Sprintf("http://127.0.0.1:%d", orchPort)
	}
	for k, v := range providerEnv { // Tier-A provider env（adapter inject 之前）
		m[k] = v
	}
	for k, v := range inject { // Tier-B adapter inject（覆盖 Tier-A 冲突键）
		m[k] = v
	}
	for _, k := range remove {
		delete(m, k)
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// utf8SafeProcess 处理跨 read() 的 UTF-8 多字节字符截断：
//   - 全部有效 → 返回字符串，carry 清空。
//   - 末尾是不完整的多字节序列（incomplete）→ 保留到 carry，emit 有效前缀。
//   - 中段出现非法字节（genuine invalid）→ 整体 lossy 转换（插入 U+FFFD），不 carry。
//
// 对应 Rust std::str::from_utf8 的 Ok / error_len().is_none() / error_len().is_some() 三分支。
// TODO: cc-pane 在严格 UTF-8 失败时回退 GBK（Windows PowerShell 5.1 中文系统）；MVP 暂用 lossy。
func utf8SafeProcess(input, carry []byte) (string, []byte) {
	combined := make([]byte, 0, len(carry)+len(input))
	combined = append(combined, carry...)
	combined = append(combined, input...)
	if len(combined) == 0 {
		return "", nil
	}
	if utf8.Valid(combined) {
		return string(combined), nil
	}

	// 找到最长有效前缀，并判断剩余尾部是「不完整」还是「非法」。
	validUpTo := 0
	i := 0
	incompleteTail := false
	for i < len(combined) {
		if combined[i] < utf8.RuneSelf {
			i++
			validUpTo = i
			continue
		}
		r, size := utf8.DecodeRune(combined[i:])
		if r == utf8.RuneError && size == 1 {
			incompleteTail = isIncompleteTrailing(combined[i:])
			break
		}
		i += size
		validUpTo = i
	}

	if incompleteTail {
		tail := combined[validUpTo:]
		newCarry := make([]byte, len(tail))
		copy(newCarry, tail)
		if validUpTo == 0 {
			return "", newCarry
		}
		return string(combined[:validUpTo]), newCarry
	}
	// 中段非法字节：lossy 转换（Go string() 对非法字节生成 U+FFFD），不保留 carry。
	return string(combined), nil
}

// isIncompleteTrailing 判断 b（以一个多字节起始字节开头）是否是某个合法多字节序列的
// 不完整前缀（字节数不够但已出现的续字节都合法）。区分「截断待补」与「真非法」。
func isIncompleteTrailing(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	var need int
	switch b0 := b[0]; {
	case b0&0xE0 == 0xC0:
		need = 2
	case b0&0xF0 == 0xE0:
		need = 3
	case b0&0xF8 == 0xF0:
		need = 4
	default:
		return false // 非合法起始字节 → 真非法
	}
	if len(b) >= need {
		return false // 字节数足够却解码失败 → 真非法
	}
	for _, c := range b[1:] {
		if c&0xC0 != 0x80 {
			return false // 续字节非法
		}
	}
	return true
}
