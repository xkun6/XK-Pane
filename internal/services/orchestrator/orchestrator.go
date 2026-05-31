// Package orchestrator 是进程内的本地 HTTP + MCP 服务器（对齐 cc-pane src-tauri OrchestratorService）。
// 让在 pane 内运行的 Claude/Codex 经 MCP 反向调用宿主能力（开新会话编排别的 Agent、读写其它 PTY…）。
//
// P6a（本单元）只做地基：
//   - net/http server 绑 127.0.0.1:随机端口（decision：本轮不做 WSL，收窄到 127.0.0.1）
//   - 32-hex 随机 Bearer token；/mcp 走官方 modelcontextprotocol/go-sdk 的 Streamable HTTP transport
//   - Bearer 鉴权中间件，复刻 cc-pane 两个 CLI 兼容补丁：?token= query 后备（Claude Issue #7290）、
//     Accept: text/event-stream 注入（Codex 缺此头会被 406）
//   - 一个 trivial `health` MCP 工具（验证 tools/list + tools/call 闭环）
//   - 启动后把 {url, token} 写到 app 数据目录的 mcp-orchestrator.json（供被启动 CLI / 未来 cli-hook 发现）
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

	"xkpane/internal/storage"
	"xkpane/internal/utils/filex"
	"xkpane/internal/utils/logx"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// orchestratorVersion 报给 MCP 客户端的 server 版本（与应用版本解耦，独立演进）。
const orchestratorVersion = "0.1.0"

// configFileName 落在 app 数据目录，记录本地 MCP 端点（被启动 CLI / cli-hook 据此回连）。
const configFileName = "mcp-orchestrator.json"

// Service 持有 HTTP+MCP server 的运行态（端口/token/server 句柄）。生命周期由 app 装配处管：
// Start 后台起、Stop 优雅关。非 Wails service（无前端调用面，纯后端）——前端只读 UI 留后续单元。
type Service struct {
	mu    sync.RWMutex
	port  int
	token string
	srv   *http.Server
}

// New 构造一个未启动的 orchestrator 服务。
func New() *Service { return &Service{} }

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

	mcpHandler := authMiddleware(token, buildMCPHandler())
	mux := http.NewServeMux()
	// 无鉴权探活（对照 cc-pane GET /api/health）。
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
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

	if err := writeConfig(port, token); err != nil {
		logx.For("orchestrator").Warn("write mcp-orchestrator.json failed", "err", err)
	}
	logx.For("orchestrator").Info("orchestrator started",
		"port", port, "url", fmt.Sprintf("http://127.0.0.1:%d/mcp", port))
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
// P6a 仅注册 health 工具；P6c 在此追加 launch_task / list_sessions 等编排工具。
func buildMCPHandler() http.Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "xkpane-orchestrator", Version: orchestratorVersion}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "health",
		Description: "返回 XK-Pane orchestrator 在线状态（连通自检）",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "XK-Pane orchestrator online"}},
		}, nil, nil
	})
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

// writeConfig 把本地 MCP 端点写到 app 数据目录的 mcp-orchestrator.json（filex 原子写，配置写铁律）。
func writeConfig(port int, token string) error {
	dir, err := storage.UserDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"ccpanes": map[string]any{
				"type":    "http",
				"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", port, token),
				"headers": map[string]string{"Authorization": "Bearer " + token},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return filex.WriteAtomic(filepath.Join(dir, configFileName), data, 0o644)
}
