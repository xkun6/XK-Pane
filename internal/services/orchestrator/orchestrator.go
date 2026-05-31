// Package orchestrator 是进程内的本地 HTTP + MCP 服务器（对齐 cc-pane src-tauri OrchestratorService）。
// 让在 pane 内运行的 Claude/Codex 经 MCP 反向调用宿主能力（开新会话编排别的 Agent、读写其它 PTY…）。
//
// P6a（本单元）只做地基：
//   - net/http server 绑 127.0.0.1:随机端口（decision：本轮不做 WSL，收窄到 127.0.0.1）
//   - 32-hex 随机 Bearer token；/mcp 走官方 modelcontextprotocol/go-sdk 的 Streamable HTTP transport
//   - Bearer 鉴权中间件，复刻 cc-pane 两个 CLI 兼容补丁：?token= query 后备（Claude Issue #7290）、
//     Accept: text/event-stream 注入（Codex 缺此头会被 406）
//   - 一个 trivial `health` MCP 工具（验证 tools/list + tools/call 闭环）
//   - 启动后把 {url, token} 写到可执行文件所在目录（"软件目录"）的 mcp-orchestrator.json，供被启动 CLI /
//     未来 cli-hook 发现（刻意不放 %AppData%：用户要求配置与软件/项目同处。skills / 每会话注入走被启动项目的 .claude/）
//
// 真正的编排工具（launch_task / list_sessions / write_to_session …）在 P6c 接 terminal/project/workspace。
// MCP server 库选型经 P6a spike 实测真实 Claude 连通后定为官方 go-sdk v1.6.1。
package orchestrator

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xkpane/internal/services/notification"
	"xkpane/internal/services/terminal"
	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"
	"xkpane/internal/utils/logx"

	"github.com/wailsapp/wails/v3/pkg/application"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Terminal 是编排工具依赖的终端能力子集（*terminal.Service 满足）。
type Terminal interface {
	CreateSession(ctx context.Context, spec terminal.CreateSpec) (string, error)
	WriteInput(ctx context.Context, sessionID, data string) error
	Kill(ctx context.Context, sessionID string) error
	SessionSnapshots() []terminal.SessionSnapshot
	SessionOutput(sessionID string, lines int) (terminal.SessionOutput, error)
}

// Projects / Workspaces 是项目 / 工作空间列举能力（*project.Service / *workspace.Service 满足）。
type Projects interface {
	List(ctx context.Context) ([]storage.Project, error)
}
type Workspaces interface {
	List(ctx context.Context) ([]storage.Workspace, error)
}

// Notifier 是桌面通知触发能力（*notification.Service 满足）。nil = 不支持 /api/notifications/trigger。
type Notifier interface {
	Trigger(req notification.Request) (notification.Result, error)
}

// Deps 是 orchestrator 编排工具的下游依赖（app 装配阶段注入既有 service）。
type Deps struct {
	Terminal   Terminal
	Projects   Projects
	Workspaces Workspaces
	Notifier   Notifier // P6d-4：cli-hook notify → /api/notifications/trigger
}

// orchestratorVersion 报给 MCP 客户端的 server 版本（与应用版本解耦，独立演进）。
const orchestratorVersion = "0.1.0"

// configFileName 落在 app 数据目录，记录本地 MCP 端点（被启动 CLI / cli-hook 据此回连）。
const configFileName = "mcp-orchestrator.json"

// callerToolHeader 承载「发起方 CLI 工具」（claude/reclaude/codex）。被启动 CLI 经 panes URL 的
// ?caller_tool= 自报，authMiddleware 桥接进请求头，launch_task 据此同源继承缺省 cli_tool。
const callerToolHeader = "X-XKPane-Caller-Tool"

// Service 持有 HTTP+MCP server 的运行态（端口/token/server 句柄）。生命周期由 app 装配处管：
// Start 后台起、Stop 优雅关。非 Wails service（无前端调用面，纯后端）——前端只读 UI 留后续单元。
type Service struct {
	mu    sync.RWMutex
	port  int
	token string
	srv   *http.Server

	deps Deps

	// rl 限 REST handler 的请求频率（10s≤20，对齐 cc-pane）。
	rl *rateLimiter

	// emit 把事件推给前端（orchestrator:launch）；nil 时丢弃（未 AttachApp / 单测）。
	emitMu sync.RWMutex
	emit   func(name string, payload any)
}

// New 构造一个未启动的 orchestrator 服务。deps 为编排工具的下游依赖（terminal/project/workspace）。
func New(deps Deps) *Service {
	return &Service{deps: deps, rl: newRateLimiter(restRateWindowSecs*time.Second, restRateMaxReq)}
}

// AttachApp 回填 app 引用，让编排工具能 Emit orchestrator:launch 给前端（app 装配阶段调，镜像 terminal）。
func (s *Service) AttachApp(app *application.App) {
	s.emitMu.Lock()
	s.emit = func(name string, payload any) { app.Event.Emit(name, payload) }
	s.emitMu.Unlock()
}

func (s *Service) emitEvent(name string, payload any) {
	s.emitMu.RLock()
	fn := s.emit
	s.emitMu.RUnlock()
	if fn != nil {
		fn(name, payload)
	}
}

// Port 返回监听端口（未启动为 0）。
func (s *Service) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// Token 返回本次进程的 Bearer token（未启动为 ""）。
func (s *Service) Token() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// Start 绑定 127.0.0.1:随机端口，起 HTTP+MCP server（内部 goroutine Serve，非阻塞），
// 并把端点写进 mcp-orchestrator.json。返回 error 由调用方决定 fail-open（裸跑无 MCP）。
func (s *Service) Start() error {
	token := genToken()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("orchestrator listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mcpHandler := authMiddleware(token, s.buildMCPHandler())
	mux := http.NewServeMux()
	// 无鉴权探活（对照 cc-pane GET /api/health）。
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	// REST 业务回调（cli-hook 回连）。各 handler 内联 verify_token + rate_limit（对齐 cc-pane，
	// REST 不走 MCP 中间件）。P6d-3：session-started 回报（log + emit）。
	mux.HandleFunc("POST /api/terminal/session-started", s.handleSessionStarted)
	mux.HandleFunc("POST /api/notifications/trigger", s.handleTriggerNotification)
	// 同一个鉴权后的 MCP handler 实例同时挂精确路径与子树（容忍尾斜杠）。
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	srv := &http.Server{Handler: mux}
	s.mu.Lock()
	s.port = port
	s.token = token
	s.srv = srv
	s.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logx.For("orchestrator").Error("serve stopped", "err", err)
		}
	}()

	cfgPath, err := writeConfig(port, token)
	if err != nil {
		logx.For("orchestrator").Warn("write mcp-orchestrator.json failed", "err", err)
	}
	logx.For("orchestrator").Info("orchestrator started",
		"port", port,
		"url", fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
		"config", cfgPath)
	return nil
}

// Stop 优雅关闭 HTTP server（3s 上限）。未启动时 no-op。
func (s *Service) Stop() error {
	s.mu.RLock()
	srv := s.srv
	s.mu.RUnlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// buildMCPHandler 构造 MCP server（官方 go-sdk）并包成 Streamable HTTP handler。
// health（连通自检）+ P6c 编排工具（registerTools，依赖 s.deps 兜底）。
func (s *Service) buildMCPHandler() http.Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "xkpane-orchestrator", Version: orchestratorVersion}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "health",
		Description: "返回 XK-Pane orchestrator 在线状态（连通自检）",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return textResult("XK-Pane orchestrator online"), nil, nil
	})
	s.registerTools(srv)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// authMiddleware 复刻 cc-pane mcp_auth_middleware + inject_mcp_accept_headers：
// Bearer header 或 ?token= query 后备校验（OPTIONS 跳过）；补 Accept: text/event-stream 防 406。
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			got := bearer(r.Header.Get("Authorization"))
			if got == "" {
				got = r.URL.Query().Get("token") // Claude Code Issue #7290 后备
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			r.Header.Set("Accept", "application/json, text/event-stream")
		}
		// 桥接「发起方 CLI」：?caller_tool= query → 请求头，供 tool handler 经 RequestExtra.Header 读取
		// （go-sdk tool handler 只见 header，不见 URL）。launch_task 据此同源继承缺省 cli_tool。
		if ct := r.URL.Query().Get("caller_tool"); ct != "" {
			r.Header.Set(callerToolHeader, ct)
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(h string) string {
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

func genToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败属系统级异常；退化为时间种子不安全，宁可 panic 由上层 fail-open 兜底。
		panic(fmt.Sprintf("orchestrator: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// configDir 返回 mcp-orchestrator.json 的落盘目录 = 可执行文件所在目录（"软件目录"）。
// 刻意不走 storage.UserDataDir（%AppData%）：用户要求 MCP 配置与软件/项目同处、便于发现与连接
// （dev 下即 bin\，已被 .gitignore 忽略；prod 即安装目录）。skills / 每会话注入则落到被启动项目的
// .claude/（P6b/P6d）。
func configDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// writeConfig 把本地 MCP 端点写到 configDir()/mcp-orchestrator.json（filex 原子写，配置写铁律），
// 返回写入的完整路径供日志暴露（方便用户找到并把 CLI 连上去）。
func writeConfig(port int, token string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, configFileName)
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"panes": map[string]any{
				"type":    "http",
				"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", port, token),
				"headers": map[string]string{"Authorization": "Bearer " + token},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := filex.WriteAtomic(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
