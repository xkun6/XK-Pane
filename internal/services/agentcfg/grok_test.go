package agentcfg

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ---- 1. 导入 + 切换：config.toml/auth.json 均更新、两者 .xkpane-bak 生成、旧行回填切换
// 前 live 内容；且全程 config 原样保留 [mcp_servers] 段（与 codex 行为的关键差异：grok
// 不做 MCP 投影，不剥离）。----

func TestSwitchGrokWritesLiveAndPreservesMcpServers(t *testing.T) {
	svc := newTestService(t)
	configPath := svc.grokConfigPath()
	authPath := svc.grokAuthPath()
	writeTextFile(t, configPath, `model = "grok-4"

[mcp_servers.foo]
command = "foo-server"
`)
	writeJSONFile(t, authPath, map[string]any{"GROK_API_KEY": "sk-original"})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeGrok)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if !list[0].IsCurrent {
		t.Error("首次导入应为 IsCurrent")
	}

	// 首次导入的快照应原样保留 mcp_servers 段（readGrokLive 不剥离，与 codex 不同）。
	initialCs := mustUnmarshalCodexSettings(t, list[0].SettingsJSON)
	if !strings.Contains(initialCs.Config, "[mcp_servers.foo]") {
		t.Fatalf("grok 导入快照不该剥离 mcp_servers 段: %q", initialCs.Config)
	}

	oldID := list[0].ID

	newSettings, _ := json.Marshal(codexSettings{
		Auth:   map[string]any{"GROK_API_KEY": "sk-new"},
		Config: "model = \"grok-4-fast\"\n",
	})
	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeGrok,
		Name:         "new-provider",
		SettingsJSON: string(newSettings),
	})
	if err != nil {
		t.Fatalf("AddAgentProvider: %v", err)
	}

	if err := svc.SwitchAgentProvider(context.Background(), added.ID); err != nil {
		t.Fatalf("SwitchAgentProvider: %v", err)
	}

	// live config.toml 已切到新配置
	liveConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if !strings.Contains(string(liveConfig), `model = "grok-4-fast"`) {
		t.Fatalf("live config 未切到新配置: %s", liveConfig)
	}
	// live auth.json 已切到新配置
	liveAuth := readJSONFile(t, authPath)
	if liveAuth["GROK_API_KEY"] != "sk-new" {
		t.Fatalf("live auth 未切到新配置: %+v", liveAuth)
	}
	// 两者备份均已生成，内容为切换前的 live 内容
	bakConfig, err := os.ReadFile(configPath + ".xkpane-bak")
	if err != nil {
		t.Fatalf("config.toml 备份未生成: %v", err)
	}
	if !strings.Contains(string(bakConfig), "[mcp_servers.foo]") {
		t.Fatalf("config.toml 备份内容不是最初内容: %s", bakConfig)
	}
	bakAuth := readJSONFile(t, authPath+".xkpane-bak")
	if bakAuth["GROK_API_KEY"] != "sk-original" {
		t.Fatalf("auth.json 备份内容不是最初内容: %+v", bakAuth)
	}

	// 旧行被回填为切换前的 live 内容，且不再是 current；mcp_servers 段仍原样保留
	after, err := svc.ListAgentProviders(context.Background(), appTypeGrok)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	var oldRow *AgentProviderDTO
	for i := range after {
		if after[i].ID == oldID {
			oldRow = &after[i]
		}
	}
	if oldRow == nil {
		t.Fatal("旧行丢失")
	}
	if oldRow.IsCurrent {
		t.Fatal("旧行应不再是 current")
	}
	oldCs := mustUnmarshalCodexSettings(t, oldRow.SettingsJSON)
	if !strings.Contains(oldCs.Config, "[mcp_servers.foo]") {
		t.Fatalf("旧行回填后不该剥离 mcp_servers 段: %q", oldCs.Config)
	}
	if oldCs.Auth["GROK_API_KEY"] != "sk-original" {
		t.Fatalf("旧行 auth 未回填切换前 live 内容: %+v", oldCs.Auth)
	}
}

// ---- 2. auth=null 的 provider：切换到它 auth.json 不动；切走后它自己的 auth 仍为 null ----

func TestGrokAuthNullProviderProtectsLiveAuth(t *testing.T) {
	svc := newTestService(t)
	configPath := svc.grokConfigPath()
	authPath := svc.grokAuthPath()
	writeTextFile(t, configPath, "model = \"grok-4\"\n")
	writeJSONFile(t, authPath, map[string]any{"GROK_API_KEY": "sk-keep-me"})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeGrok)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	defaultID := list[0].ID

	nullAuthSettings, _ := json.Marshal(codexSettings{
		Auth:   nil,
		Config: "model = \"grok-4-fast\"\n",
	})
	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeGrok,
		Name:         "no-auth-provider",
		SettingsJSON: string(nullAuthSettings),
	})
	if err != nil {
		t.Fatalf("AddAgentProvider: %v", err)
	}

	// 切到 auth=null 的 provider：auth.json 应保持不动
	if err := svc.SwitchAgentProvider(context.Background(), added.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(到 auth=null): %v", err)
	}
	liveAuth := readJSONFile(t, authPath)
	if liveAuth["GROK_API_KEY"] != "sk-keep-me" {
		t.Fatalf("切到 auth=null 的 provider 不该动 auth.json: %+v", liveAuth)
	}
	liveConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if !strings.Contains(string(liveConfig), `model = "grok-4-fast"`) {
		t.Fatalf("config 未切换: %s", liveConfig)
	}

	// 从它切走：回填后它自己的 auth 仍应为 null
	if err := svc.SwitchAgentProvider(context.Background(), defaultID); err != nil {
		t.Fatalf("SwitchAgentProvider(切走): %v", err)
	}
	after, err := svc.ListAgentProviders(context.Background(), appTypeGrok)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	var noAuthRow *AgentProviderDTO
	for i := range after {
		if after[i].ID == added.ID {
			noAuthRow = &after[i]
		}
	}
	if noAuthRow == nil {
		t.Fatal("行丢失")
	}
	cs := mustUnmarshalCodexSettings(t, noAuthRow.SettingsJSON)
	if cs.Auth != nil {
		t.Fatalf("auth=null 的行回填后 auth 应仍为 null，got %+v", cs.Auth)
	}
}

// ---- 3. 非法 TOML 的 config → AddAgentProvider 被拒（证明 validateProviderSettings 对
// grok 也复用了 validateCodexSettings） ----

func TestAddGrokProviderRejectsInvalidToml(t *testing.T) {
	svc := newTestService(t)
	badSettings, _ := json.Marshal(codexSettings{
		Auth:   nil,
		Config: "key = \"unterminated",
	})
	_, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeGrok,
		Name:         "bad-toml",
		SettingsJSON: string(badSettings),
	})
	if err == nil {
		t.Fatal("非法 TOML 的 config 应被 AddAgentProvider 拒绝")
	}
}

// ---- 4. 全局指令编辑对 grok 明确拒绝，避免误读误写 Claude 的 ~/.claude/CLAUDE.md ----

func TestGrokInstructionFileRejected(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.ReadInstructionFile(appTypeGrok); err == nil {
		t.Fatal("ReadInstructionFile(grok) 应报错（本期不支持）")
	}
	if err := svc.WriteInstructionFile(appTypeGrok, "x"); err == nil {
		t.Fatal("WriteInstructionFile(grok) 应报错（本期不支持）")
	}
}
