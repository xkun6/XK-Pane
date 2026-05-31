package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// notifyRequest 是 POST /api/notifications/trigger 的请求体（camelCase）。
type notifyRequest struct {
	Kind              string          `json:"kind"`
	Title             string          `json:"title"`
	Body              *string         `json:"body,omitempty"`
	Source            *string         `json:"source,omitempty"`
	Scope             *string         `json:"scope,omitempty"`
	DedupeKey         *string         `json:"dedupeKey,omitempty"`
	OnlyWhenUnfocused *bool           `json:"onlyWhenUnfocused,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

// orchestratorConfig 是 mcp-orchestrator.json 的解析结构（仅取 panes 端点）。
type orchestratorConfig struct {
	McpServers map[string]struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	} `json:"mcpServers"`
}

// runNotify 是 notify 子命令入口（显式触发桌面通知）。失败 stderr + exit 1（对齐 cc-pane）。
func runNotify(args []string) {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	kind := fs.String("kind", "custom", "通知类别")
	title := fs.String("title", "", "通知标题（必填）")
	body := fs.String("body", "", "通知正文")
	source := fs.String("source", "cli", "来源标识")
	scope := fs.String("scope", "", "作用域")
	dedupeKey := fs.String("dedupe-key", "", "去重键")
	onlyWhenUnfocused := fs.Bool("only-when-unfocused", false, "仅窗口未聚焦时通知")
	metadataJSON := fs.String("metadata-json", "", "附加元数据（JSON）")
	if err := fs.Parse(args); err != nil {
		failNotify(fmt.Sprintf("参数解析失败: %v", err))
	}
	if strings.TrimSpace(*title) == "" {
		failNotify("--title 必填")
	}

	triggerURL, token, err := resolveNotifyEndpoint()
	if err != nil {
		failNotify(err.Error())
	}

	req := notifyRequest{
		Kind:              *kind,
		Title:             *title,
		Body:              optStr(*body),
		Source:            optStr(*source),
		Scope:             optStr(*scope),
		DedupeKey:         optStr(*dedupeKey),
		OnlyWhenUnfocused: onlyWhenUnfocused,
	}
	if strings.TrimSpace(*metadataJSON) != "" {
		if !json.Valid([]byte(*metadataJSON)) {
			failNotify("--metadata-json 不是合法 JSON")
		}
		req.Metadata = json.RawMessage(*metadataJSON)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		failNotify(fmt.Sprintf("编码请求失败: %v", err))
	}

	status, snippet, err := postJSON(triggerURL, token, payload)
	if err != nil {
		failNotify(fmt.Sprintf("通知 API 请求失败: %v", err))
	}
	if status < 200 || status >= 300 {
		failNotify(fmt.Sprintf("通知 API 返回 HTTP %d: %s", status, snippet))
	}
	fmt.Println(snippet)
}

func failNotify(msg string) {
	fmt.Fprintf(os.Stderr, "[xkpane-cli-hook] notify failed: %s\n", msg)
	os.Exit(1)
}

// resolveNotifyEndpoint 解析触发端点 URL + token：优先继承的 env（pane 内调用），
// 回退读 mcp-orchestrator.json（手动调用，从软件目录发现）。
func resolveNotifyEndpoint() (string, string, error) {
	if base := resolveBaseURL(); base != "" {
		if token := apiToken(); token != "" {
			return base + "/api/notifications/trigger", token, nil
		}
	}
	return endpointFromConfig()
}

func endpointFromConfig() (string, string, error) {
	path := findOrchestratorConfig()
	if path == "" {
		return "", "", fmt.Errorf("未找到 mcp-orchestrator.json（软件目录），且 env 未注入端点")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("读取 %s 失败: %v", path, err)
	}
	var cfg orchestratorConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", fmt.Errorf("解析 orchestrator 配置失败: %v", err)
	}
	server, ok := cfg.McpServers["panes"]
	if !ok {
		return "", "", fmt.Errorf("mcpServers.panes 缺失")
	}

	token := bearerFromHeaders(server.Headers)
	if token == "" {
		token = tokenFromQuery(server.URL)
	}
	if token == "" {
		return "", "", fmt.Errorf("orchestrator 配置缺少 token")
	}

	u, err := url.Parse(server.URL)
	if err != nil {
		return "", "", fmt.Errorf("orchestrator URL 非法 '%s': %v", server.URL, err)
	}
	u.RawQuery = ""
	u.Path = "/api/notifications/trigger"
	return u.String(), token, nil
}

// findOrchestratorConfig 找 mcp-orchestrator.json：XKPANE_CONFIG_DIR 覆盖 → 可执行文件同目录。
func findOrchestratorConfig() string {
	const name = "mcp-orchestrator.json"
	if dir := strings.TrimSpace(os.Getenv("XKPANE_CONFIG_DIR")); dir != "" {
		if p := filepath.Join(dir, name); fileExists(p) {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), name); fileExists(p) {
			return p
		}
	}
	return ""
}

func bearerFromHeaders(headers map[string]string) string {
	if headers == nil {
		return ""
	}
	v := headers["Authorization"]
	if strings.HasPrefix(v, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	}
	return ""
}

func tokenFromQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("token")
}

func optStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
