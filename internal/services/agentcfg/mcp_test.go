package agentcfg

import (
	"context"
	"os"
	"strings"
	"testing"
)

// ---- 1. Add + 投影：无关顶层键与手工 server 原样保留，新键按 Enabled 出现两侧 ----

func TestAddMcpServerProjectsToBothSides(t *testing.T) {
	svc := newTestService(t)
	claudePath := svc.claudeMcpPath()
	writeJSONFile(t, claudePath, map[string]any{
		"projects": map[string]any{"/some/path": map[string]any{"foo": "bar"}},
		"mcpServers": map[string]any{
			"manual": map[string]any{"command": "manual-cmd"},
		},
	})
	writeTextFile(t, svc.codexConfigPath(), "model = \"gpt-5\"\n")
	mustStartup(t, svc)

	ctx := context.Background()
	if _, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "alpha", ServerJSON: `{"command":"alpha-cmd"}`, EnabledClaude: true,
	}); err != nil {
		t.Fatalf("AddMcpServer(alpha): %v", err)
	}
	if _, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "beta", ServerJSON: `{"command":"beta-cmd"}`, EnabledCodex: true,
	}); err != nil {
		t.Fatalf("AddMcpServer(beta): %v", err)
	}

	// Claude 侧：无关顶层键 + 手工 server 原样保留，alpha 出现，beta 不出现
	liveMap := readJSONFile(t, claudePath)
	if _, ok := liveMap["projects"]; !ok {
		t.Fatal("无关顶层键 projects 丢失")
	}
	mcpServers, _ := liveMap["mcpServers"].(map[string]any)
	if mcpServers == nil {
		t.Fatal("mcpServers 丢失")
	}
	if _, ok := mcpServers["manual"]; !ok {
		t.Fatal("手工 mcpServer 丢失")
	}
	if _, ok := mcpServers["alpha"]; !ok {
		t.Fatal("alpha 未投影到 claude.json")
	}
	if _, ok := mcpServers["beta"]; ok {
		t.Fatal("beta 不该出现在 claude.json（EnabledClaude=false）")
	}

	// Codex 侧：beta 出现在 [mcp_servers.beta]，provider 正文保留，alpha 不出现
	configData, err := os.ReadFile(svc.codexConfigPath())
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	config := string(configData)
	if !strings.Contains(config, `model = "gpt-5"`) {
		t.Fatalf("config.toml provider 正文丢失: %s", config)
	}
	if !strings.Contains(config, "[mcp_servers.beta]") {
		t.Fatalf("beta 未投影到 config.toml: %s", config)
	}
	if strings.Contains(config, "[mcp_servers.alpha]") {
		t.Fatalf("alpha 不该出现在 config.toml（EnabledCodex=false）: %s", config)
	}
}

// ---- 2. 禁用/删除：Update 关掉 EnabledClaude → 键消失但 SSOT 行还在；Delete → 两边都消失 ----

func TestUpdateDisableAndDeleteRemovesFromLive(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)
	ctx := context.Background()

	added, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "gamma", ServerJSON: `{"command":"gamma-cmd"}`, EnabledClaude: true,
	})
	if err != nil {
		t.Fatalf("AddMcpServer: %v", err)
	}

	claudeServers := func() map[string]any {
		m, _ := readJSONFile(t, svc.claudeMcpPath())["mcpServers"].(map[string]any)
		return m
	}
	if _, ok := claudeServers()["gamma"]; !ok {
		t.Fatal("gamma 添加后应出现在 claude.json")
	}

	if _, err := svc.UpdateMcpServer(ctx, added.ID, AgentMcpServerInput{
		Name: "gamma", ServerJSON: `{"command":"gamma-cmd"}`, EnabledClaude: false,
	}); err != nil {
		t.Fatalf("UpdateMcpServer(禁用): %v", err)
	}
	if _, ok := claudeServers()["gamma"]; ok {
		t.Fatal("关闭 EnabledClaude 后 gamma 应从 claude.json 消失")
	}

	list, err := svc.ListMcpServers(ctx)
	if err != nil {
		t.Fatalf("ListMcpServers: %v", err)
	}
	found := false
	for _, row := range list {
		if row.ID == added.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("禁用后 SSOT 行应仍存在")
	}

	if err := svc.DeleteMcpServer(ctx, added.ID); err != nil {
		t.Fatalf("DeleteMcpServer: %v", err)
	}
	list, err = svc.ListMcpServers(ctx)
	if err != nil {
		t.Fatalf("ListMcpServers: %v", err)
	}
	for _, row := range list {
		if row.ID == added.ID {
			t.Fatal("Delete 后 SSOT 行应消失")
		}
	}
}

// ---- 3. 改名：旧键从两侧 live 消失、新键出现 ----

func TestUpdateRenameMovesLiveKey(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)
	ctx := context.Background()

	added, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "old-name", ServerJSON: `{"command":"x"}`, EnabledClaude: true, EnabledCodex: true,
	})
	if err != nil {
		t.Fatalf("AddMcpServer: %v", err)
	}

	if _, err := svc.UpdateMcpServer(ctx, added.ID, AgentMcpServerInput{
		Name: "new-name", ServerJSON: `{"command":"x"}`, EnabledClaude: true, EnabledCodex: true,
	}); err != nil {
		t.Fatalf("UpdateMcpServer(改名): %v", err)
	}

	mcpServers, _ := readJSONFile(t, svc.claudeMcpPath())["mcpServers"].(map[string]any)
	if _, ok := mcpServers["old-name"]; ok {
		t.Fatal("旧键应从 claude.json 消失")
	}
	if _, ok := mcpServers["new-name"]; !ok {
		t.Fatal("新键应出现在 claude.json")
	}

	configData, err := os.ReadFile(svc.codexConfigPath())
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	config := string(configData)
	if strings.Contains(config, "[mcp_servers.old-name]") {
		t.Fatalf("旧键应从 config.toml 消失: %s", config)
	}
	if !strings.Contains(config, "[mcp_servers.new-name]") {
		t.Fatalf("新键应出现在 config.toml: %s", config)
	}
}

// ---- 4. ImportMcpFromLive：合并同名/异名、Enabled 侧正确、重复导入幂等 ----

func TestImportMcpFromLiveMergesBothSidesAndIsIdempotent(t *testing.T) {
	svc := newTestService(t)
	writeJSONFile(t, svc.claudeMcpPath(), map[string]any{
		"mcpServers": map[string]any{
			"shared":      map[string]any{"command": "shared-cmd"},
			"claude-only": map[string]any{"command": "claude-cmd"},
		},
	})
	writeTextFile(t, svc.codexConfigPath(), `model = "gpt-5"

[mcp_servers.shared]
command = "shared-cmd"

[mcp_servers.codex-only]
command = "codex-cmd"
`)

	ctx := context.Background()
	n, err := svc.ImportMcpFromLive(ctx)
	if err != nil {
		t.Fatalf("ImportMcpFromLive: %v", err)
	}
	if n != 3 {
		t.Fatalf("首次导入应新增 3 行（shared/claude-only/codex-only），got %d", n)
	}

	list, err := svc.ListMcpServers(ctx)
	if err != nil {
		t.Fatalf("ListMcpServers: %v", err)
	}
	byName := map[string]AgentMcpServerDTO{}
	for _, row := range list {
		byName[row.Name] = row
	}
	if shared, ok := byName["shared"]; !ok || !shared.EnabledClaude || !shared.EnabledCodex {
		t.Fatalf("shared 应两侧都启用: %+v (ok=%v)", byName["shared"], ok)
	}
	if co, ok := byName["claude-only"]; !ok || !co.EnabledClaude || co.EnabledCodex {
		t.Fatalf("claude-only 应只启用 claude 侧: %+v (ok=%v)", co, ok)
	}
	if xo, ok := byName["codex-only"]; !ok || xo.EnabledClaude || !xo.EnabledCodex {
		t.Fatalf("codex-only 应只启用 codex 侧: %+v (ok=%v)", xo, ok)
	}

	n2, err := svc.ImportMcpFromLive(ctx)
	if err != nil {
		t.Fatalf("ImportMcpFromLive(第二次): %v", err)
	}
	if n2 != 0 {
		t.Fatalf("重复导入应为 0，got %d", n2)
	}
}

// ---- 附：非法 name 被拒 ----

func TestAddMcpServerRejectsInvalidName(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)
	ctx := context.Background()

	if _, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "bad name!", ServerJSON: `{}`,
	}); err == nil {
		t.Fatal("非法 name 应被拒")
	}
	if _, err := svc.AddMcpServer(ctx, AgentMcpServerInput{
		Name: "", ServerJSON: `{}`,
	}); err == nil {
		t.Fatal("空 name 应被拒")
	}
}
