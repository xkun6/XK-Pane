package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBearerFromHeaders(t *testing.T) {
	if got := bearerFromHeaders(map[string]string{"Authorization": "Bearer abc123"}); got != "abc123" {
		t.Errorf("bearerFromHeaders=%q want abc123", got)
	}
	if got := bearerFromHeaders(map[string]string{"Authorization": "Basic x"}); got != "" {
		t.Errorf("bearerFromHeaders 非 Bearer 应空, got %q", got)
	}
	if got := bearerFromHeaders(nil); got != "" {
		t.Errorf("bearerFromHeaders(nil) 应空, got %q", got)
	}
}

func TestTokenFromQuery(t *testing.T) {
	if got := tokenFromQuery("http://127.0.0.1:48080/mcp?token=deadbeef"); got != "deadbeef" {
		t.Errorf("tokenFromQuery=%q want deadbeef", got)
	}
	if got := tokenFromQuery("http://127.0.0.1:48080/mcp"); got != "" {
		t.Errorf("tokenFromQuery 无 token 应空, got %q", got)
	}
}

// endpointFromConfig 应从 mcp-orchestrator.json 的 panes 端点派生 trigger URL + token。
func TestEndpointFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"panes": map[string]any{
				"type":    "http",
				"url":     "http://127.0.0.1:51234/mcp?token=tok1234",
				"headers": map[string]string{"Authorization": "Bearer tok1234"},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "mcp-orchestrator.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XKPANE_CONFIG_DIR", dir)
	// 确保不走 env-first 分支。
	t.Setenv("XKPANE_API_BASE_URL", "")
	t.Setenv("XKPANE_API_PORT", "")

	url, token, err := endpointFromConfig()
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:51234/api/notifications/trigger" {
		t.Errorf("trigger url=%q", url)
	}
	if token != "tok1234" {
		t.Errorf("token=%q want tok1234", token)
	}
}

func TestResolveNotifyEndpointEnvFirst(t *testing.T) {
	t.Setenv("XKPANE_API_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("XKPANE_API_TOKEN", "envtok")
	url, token, err := resolveNotifyEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:9999/api/notifications/trigger" || token != "envtok" {
		t.Errorf("env-first 端点=%q token=%q", url, token)
	}
}

func TestOptStr(t *testing.T) {
	if optStr("  ") != nil {
		t.Error("optStr 空白应为 nil")
	}
	if p := optStr(" x "); p == nil || *p != "x" {
		t.Errorf("optStr 应 trim 非空")
	}
}
