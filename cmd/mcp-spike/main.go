// Command mcp-spike 是 P6a 的去风险探针：用同一套 HTTP 外壳分别跑两个 Go MCP SDK
// （官方 modelcontextprotocol/go-sdk 与社区 mark3labs/mcp-go），验证 Streamable HTTP
// MCP server 能起、能被 MCP 客户端/真实 Claude·Codex CLI 连上。
//
// 它不是 XK 的正式代码（不进 internal），SDK 选定后由正式的 internal/services/orchestrator 取代。
// 运行：go run ./cmd/mcp-spike -sdk official  （或 -sdk mark3labs）
//
// 复刻 cc-pane orchestrator_service.rs 的两个 CLI 兼容补丁：
//   - Bearer 鉴权同时接受 Authorization header 与 ?token= query（Claude Code Issue #7290）
//   - 注入 Accept: application/json, text/event-stream（Codex 缺此头会被 MCP server 406）
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	m3mcp "github.com/mark3labs/mcp-go/mcp"
	m3server "github.com/mark3labs/mcp-go/server"
)

func main() {
	sdk := flag.String("sdk", "official", "official | mark3labs")
	host := flag.String("host", "127.0.0.1", "bind host (127.0.0.1 本机；0.0.0.0 含 WSL)")
	flag.Parse()

	token := genToken()

	var mcpHandler http.Handler
	switch *sdk {
	case "official":
		mcpHandler = officialHandler()
	case "mark3labs":
		mcpHandler = mark3labsHandler()
	default:
		log.Fatalf("unknown -sdk %q (official|mark3labs)", *sdk)
	}

	mux := http.NewServeMux()
	// 无鉴权探活（对照 cc-pane GET /api/health）。
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "sdk": *sdk})
	})
	mux.Handle("/mcp", authMiddleware(token, mcpHandler))
	mux.Handle("/mcp/", authMiddleware(token, mcpHandler))

	ln, err := net.Listen("tcp", net.JoinHostPort(*host, "0"))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", port, token)

	cfgPath := writeMCPConfig(port, token)
	fmt.Printf("[mcp-spike] sdk=%s listening on %s:%d\n", *sdk, *host, port)
	fmt.Printf("[mcp-spike] token=%s\n", token)
	fmt.Printf("[mcp-spike] mcp url=%s\n", url)
	fmt.Printf("[mcp-spike] health=http://127.0.0.1:%d/api/health\n", port)
	fmt.Printf("[mcp-spike] mcp-config written: %s\n", cfgPath)
	fmt.Printf("[mcp-spike] 真实 CLI 连通测试：claude --mcp-config %q  然后 /mcp 里看是否有 ping 工具\n", cfgPath)

	srv := &http.Server{Handler: mux}
	if err := srv.Serve(ln); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// authMiddleware 复刻 cc-pane mcp_auth_middleware + inject_mcp_accept_headers。
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
		// Codex 兼容：缺 text/event-stream 会被 MCP server 406。
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

// —— 官方 modelcontextprotocol/go-sdk ——

type pingArgs struct{}

func officialHandler() http.Handler {
	s := officialmcp.NewServer(&officialmcp.Implementation{Name: "xkpane-orchestrator-spike", Version: "0.0.1"}, nil)
	officialmcp.AddTool(s, &officialmcp.Tool{Name: "ping", Description: "health ping; returns pong"},
		func(_ context.Context, _ *officialmcp.CallToolRequest, _ pingArgs) (*officialmcp.CallToolResult, any, error) {
			return &officialmcp.CallToolResult{
				Content: []officialmcp.Content{&officialmcp.TextContent{Text: "pong (official go-sdk v1.6.1)"}},
			}, nil, nil
		})
	return officialmcp.NewStreamableHTTPHandler(func(*http.Request) *officialmcp.Server { return s }, nil)
}

// —— 社区 mark3labs/mcp-go ——

func mark3labsHandler() http.Handler {
	s := m3server.NewMCPServer("xkpane-orchestrator-spike", "0.0.1")
	s.AddTool(m3mcp.NewTool("ping", m3mcp.WithDescription("health ping; returns pong")),
		func(context.Context, m3mcp.CallToolRequest) (*m3mcp.CallToolResult, error) {
			return m3mcp.NewToolResultText("pong (mark3labs/mcp-go v0.54.1)"), nil
		})
	return m3server.NewStreamableHTTPServer(s)
}

func genToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

func writeMCPConfig(port int, token string) string {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"ccpanes": map[string]any{
				"type":    "http",
				"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", port, token),
				"headers": map[string]string{"Authorization": "Bearer " + token},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	path := filepath.Join(os.TempDir(), "xkpane-mcp-spike.json")
	_ = os.WriteFile(path, data, 0o644)
	return path
}
