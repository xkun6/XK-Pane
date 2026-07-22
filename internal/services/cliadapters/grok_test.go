package cliadapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// 守门：orchestrator 就绪时把 panes 写入【项目】.grok/config.toml，绝不碰 home/.grok。
func TestEnsureGrokProjectPanesMCPWritesProjectConfigOnly(t *testing.T) {
	proj := t.TempDir()
	// 预置用户其它 project MCP，注入后必须保留
	pre := filepath.Join(proj, ".grok")
	if err := os.MkdirAll(pre, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pre, "config.toml"), []byte(`
[mcp_servers.manual]
url = "http://example.com/mcp"
enabled = true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// home 侧放一个哨兵文件：注入逻辑不得改它
	home := t.TempDir()
	t.Setenv("USERPROFILE", home) // Windows
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".grok"), 0o755); err != nil {
		t.Fatal(err)
	}
	homeCfg := filepath.Join(home, ".grok", "config.toml")
	if err := os.WriteFile(homeCfg, []byte("model = \"keep-me\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ensureGrokProjectPanesMCP(&Context{
		ProjectPath:       proj,
		OrchestratorPort:  17890,
		OrchestratorToken: "tok-abc",
	})
	if err != nil {
		t.Fatalf("ensureGrokProjectPanesMCP: %v", err)
	}

	// 项目侧：panes + manual 都在
	data, err := os.ReadFile(filepath.Join(pre, "config.toml"))
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}
	var root map[string]any
	if err := toml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse project config: %v", err)
	}
	servers, _ := root["mcp_servers"].(map[string]any)
	if servers == nil {
		t.Fatal("mcp_servers missing")
	}
	if _, ok := servers["manual"]; !ok {
		t.Fatal("user project MCP manual 应保留")
	}
	panes, ok := servers["panes"].(map[string]any)
	if !ok {
		t.Fatalf("panes missing: %+v", servers)
	}
	url, _ := panes["url"].(string)
	if !strings.Contains(url, "127.0.0.1:17890") || !strings.Contains(url, "token=tok-abc") || !strings.Contains(url, "caller_tool=grok") {
		t.Fatalf("panes url 不正确: %q", url)
	}
	headers, _ := panes["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer tok-abc" {
		t.Fatalf("Authorization header = %v", headers["Authorization"])
	}

	// home 侧哨兵未动
	homeData, err := os.ReadFile(homeCfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(homeData) != "model = \"keep-me\"\n" {
		t.Fatalf("~/.grok/config.toml 被改动: %q", homeData)
	}
}

// 守门：SkipMCP / orchestrator 未就绪时不写项目配置。
func TestEnsureGrokProjectPanesMCPNoopWhenOrchestratorMissing(t *testing.T) {
	proj := t.TempDir()
	if err := ensureGrokProjectPanesMCP(&Context{ProjectPath: proj}); err != nil {
		t.Fatalf("nil orch should no-op: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".grok", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("不应创建 project config，stat err = %v", err)
	}
}

// 守门：cleanup 只摘 panes，保留其它 project MCP；仅 panes 时删文件。
func TestCleanupGrokProjectPanesMCP(t *testing.T) {
	proj := t.TempDir()
	dir := filepath.Join(proj, ".grok")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[mcp_servers.panes]
url = "http://127.0.0.1:1/mcp"
enabled = true

[mcp_servers.keep]
url = "http://keep.example/mcp"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cleanupGrokProjectPanesMCP(proj)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config should remain (keep still present): %v", err)
	}
	if strings.Contains(string(data), "panes") {
		t.Fatalf("panes 应被摘掉: %s", data)
	}
	if !strings.Contains(string(data), "keep") {
		t.Fatalf("keep 应保留: %s", data)
	}

	// 只剩 panes 时整文件删除
	if err := os.WriteFile(path, []byte(`
[mcp_servers.panes]
url = "http://x"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanupGrokProjectPanesMCP(proj)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("仅 panes 时文件应删除, stat err = %v", err)
	}
}

// 守门：SupportsMCP=true，且 SkipMCP 时 BuildCommand 不写项目配置。
func TestGrokBuildCommandSkipMCPDoesNotWriteProjectConfig(t *testing.T) {
	if _, err := resolveGrok(); err != nil {
		t.Skip("grok 不在 PATH / ~/.grok/bin，跳过")
	}
	a := NewGrok()
	if !a.Capabilities().SupportsMCP {
		t.Fatal("grok 应 SupportsMCP=true")
	}
	proj := t.TempDir()
	_, err := a.BuildCommand(&Context{
		ProjectPath:       proj,
		SkipMCP:           true,
		OrchestratorPort:  1,
		OrchestratorToken: "t",
	})
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".grok", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("SkipMCP 时不应写 project config, stat err = %v", err)
	}
}
