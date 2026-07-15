package agentcfg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// writeTextFile 把 content 原样写入 path（config.toml 是自由文本，不能用 writeJSONFile）。
func writeTextFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustUnmarshalCodexSettings(t *testing.T, raw string) codexSettings {
	t.Helper()
	var cs codexSettings
	if err := json.Unmarshal([]byte(raw), &cs); err != nil {
		t.Fatalf("unmarshal codexSettings: %v (raw=%q)", err, raw)
	}
	return cs
}

// ---- 1. 导入：config 剥离 mcp_servers 段、auth 捕获成功 ----

func TestImportCodexStripsMcpAndCapturesAuth(t *testing.T) {
	svc := newTestService(t)
	writeTextFile(t, svc.codexConfigPath(), `model = "gpt-5"

[mcp_servers.foo]
command = "foo-server"
args = ["--flag"]
`)
	writeJSONFile(t, svc.codexAuthPath(), map[string]any{"OPENAI_API_KEY": "sk-live"})

	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeCodex)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if !list[0].IsCurrent {
		t.Error("首次导入应为 IsCurrent")
	}

	cs := mustUnmarshalCodexSettings(t, list[0].SettingsJSON)
	if strings.Contains(cs.Config, "mcp_servers") {
		t.Fatalf("config 未剥离 mcp_servers 段: %q", cs.Config)
	}
	if !strings.Contains(cs.Config, `model = "gpt-5"`) {
		t.Fatalf("config 误删了 mcp_servers 之外的内容: %q", cs.Config)
	}
	if cs.Auth["OPENAI_API_KEY"] != "sk-live" {
		t.Fatalf("auth 未正确捕获: %+v", cs.Auth)
	}
}

// ---- 2. 切换：config.toml/auth.json 均更新、两者 .xkpane-bak 生成、旧行回填切换前 live 内容 ----

func TestSwitchCodexWritesLiveAndBackfillsOld(t *testing.T) {
	svc := newTestService(t)
	configPath := svc.codexConfigPath()
	authPath := svc.codexAuthPath()
	writeTextFile(t, configPath, "model = \"gpt-5\"\n")
	writeJSONFile(t, authPath, map[string]any{"OPENAI_API_KEY": "sk-original"})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeCodex)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	oldID := list[0].ID

	newSettings, _ := json.Marshal(codexSettings{
		Auth:   map[string]any{"OPENAI_API_KEY": "sk-new"},
		Config: "model = \"gpt-4\"\n",
	})
	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeCodex,
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
	if !strings.Contains(string(liveConfig), `model = "gpt-4"`) {
		t.Fatalf("live config 未切到新配置: %s", liveConfig)
	}
	// live auth.json 已切到新配置
	liveAuth := readJSONFile(t, authPath)
	if liveAuth["OPENAI_API_KEY"] != "sk-new" {
		t.Fatalf("live auth 未切到新配置: %+v", liveAuth)
	}
	// 两者备份均已生成，内容为切换前的 live 内容
	bakConfig, err := os.ReadFile(configPath + ".xkpane-bak")
	if err != nil {
		t.Fatalf("config.toml 备份未生成: %v", err)
	}
	if !strings.Contains(string(bakConfig), `model = "gpt-5"`) {
		t.Fatalf("config.toml 备份内容不是最初内容: %s", bakConfig)
	}
	bakAuth := readJSONFile(t, authPath+".xkpane-bak")
	if bakAuth["OPENAI_API_KEY"] != "sk-original" {
		t.Fatalf("auth.json 备份内容不是最初内容: %+v", bakAuth)
	}

	// 旧行被回填为切换前的 live 内容，且不再是 current
	after, err := svc.ListAgentProviders(context.Background(), appTypeCodex)
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
	if !strings.Contains(oldCs.Config, `model = "gpt-5"`) {
		t.Fatalf("旧行 config 未回填切换前 live 内容: %q", oldCs.Config)
	}
	if oldCs.Auth["OPENAI_API_KEY"] != "sk-original" {
		t.Fatalf("旧行 auth 未回填切换前 live 内容: %+v", oldCs.Auth)
	}
}

// ---- 3. auth=null 的 provider：切换到它 auth.json 不动；切走后它自己的 auth 仍为 null ----

func TestCodexAuthNullProviderProtectsLiveAuth(t *testing.T) {
	svc := newTestService(t)
	configPath := svc.codexConfigPath()
	authPath := svc.codexAuthPath()
	writeTextFile(t, configPath, "model = \"gpt-5\"\n")
	writeJSONFile(t, authPath, map[string]any{"OPENAI_API_KEY": "sk-keep-me"})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeCodex)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	defaultID := list[0].ID

	nullAuthSettings, _ := json.Marshal(codexSettings{
		Auth:   nil,
		Config: "model = \"gpt-4\"\n",
	})
	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeCodex,
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
	if liveAuth["OPENAI_API_KEY"] != "sk-keep-me" {
		t.Fatalf("切到 auth=null 的 provider 不该动 auth.json: %+v", liveAuth)
	}
	liveConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if !strings.Contains(string(liveConfig), `model = "gpt-4"`) {
		t.Fatalf("config 未切换: %s", liveConfig)
	}

	// 从它切走：回填后它自己的 auth 仍应为 null
	if err := svc.SwitchAgentProvider(context.Background(), defaultID); err != nil {
		t.Fatalf("SwitchAgentProvider(切走): %v", err)
	}
	after, err := svc.ListAgentProviders(context.Background(), appTypeCodex)
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

// ---- 4. stripMcpServers 单测：多段 / 带注释 / 带前导空白 / 嵌套子表 / 数组表均剥干净，余文可解析 ----

func TestStripMcpServersRemovesAllForms(t *testing.T) {
	input := `model = "gpt-5"

[mcp_servers]
foo = "bar"

  [mcp_servers.alpha]  # 前导空白 + 行尾注释
command = "alpha-cmd"

[mcp_servers.alpha.env]
KEY = "value"

[[mcp_servers.legacy]]
command = "legacy-cmd"

[other_section]
enabled = true
`
	out, err := stripMcpServers(input)
	if err != nil {
		t.Fatalf("stripMcpServers: %v", err)
	}
	if strings.Contains(out, "mcp_servers") {
		t.Fatalf("仍残留 mcp_servers 段: %q", out)
	}
	if !strings.Contains(out, `model = "gpt-5"`) {
		t.Fatalf("误删了 mcp_servers 之外的内容: %q", out)
	}
	if !strings.Contains(out, "[other_section]") || !strings.Contains(out, "enabled = true") {
		t.Fatalf("误删了 mcp_servers 之后的表: %q", out)
	}
	var m map[string]any
	if err := toml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("剥离结果不可解析: %v", err)
	}
}

// ---- 5. 非法 TOML 的 config → AddAgentProvider 被拒 ----

func TestAddCodexProviderRejectsInvalidToml(t *testing.T) {
	svc := newTestService(t)
	badSettings, _ := json.Marshal(codexSettings{
		Auth:   nil,
		Config: "key = \"unterminated",
	})
	_, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeCodex,
		Name:         "bad-toml",
		SettingsJSON: string(badSettings),
	})
	if err == nil {
		t.Fatal("非法 TOML 的 config 应被 AddAgentProvider 拒绝")
	}
}

// ---- 6. config 写失败回滚 auth 的路径 ----
//
// writeCodexLive 里"config 写失败且刚才写过 auth → 回滚 auth"这条分支依赖让
// filex.WriteAtomic 在写 config.toml 时精确失败（例如把 .codex 目录本身占位成一个文件
// 制造 rename 失败），构造出的失败点与真实故障模式（磁盘满 / 权限骤变）关联度低，
// 稳定性差，价值不高，故不写自动化测试。该分支的逻辑见 codex.go 的 writeCodexLive
// 文档注释与实现：auth 先写时记录旧字节（存在则记内容、不存在则记"不存在"），config 写
// 失败时按记录状态原样写回或删除新写的 auth.json。
