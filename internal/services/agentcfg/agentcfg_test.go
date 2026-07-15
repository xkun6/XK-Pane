package agentcfg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := New(storage.NewHolder(db))
	svc.home = t.TempDir()    // 隔离真实 ~/.claude，测试专用主目录
	svc.dataDir = t.TempDir() // 隔离真实应用数据目录，agent-skills SSOT 落在这里
	return svc
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustStartup(t *testing.T, svc *Service) {
	t.Helper()
	if err := svc.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("ServiceStartup: %v", err)
	}
}

// ---- 1. 首启导入：预写 live settings.json ----

func TestStartupImportsFromLiveSettings(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_AUTH_TOKEN": "sk-test-token-abc"},
	})

	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if !list[0].IsCurrent {
		t.Error("首次导入应为 IsCurrent")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(list[0].SettingsJSON), &got); err != nil {
		t.Fatalf("unmarshal SettingsJSON: %v", err)
	}
	env, _ := got["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-test-token-abc" {
		t.Fatalf("导入内容与 live 文件不一致: %+v", got)
	}
}

// ---- 2. live 不存在时导入 → "{}" 默认行 ----

func TestStartupImportsDefaultWhenLiveMissing(t *testing.T) {
	svc := newTestService(t)

	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if !list[0].IsCurrent {
		t.Error("默认行应为 IsCurrent（保证永远有回填目标）")
	}
	if strings.TrimSpace(list[0].SettingsJSON) != "{}" {
		t.Fatalf("SettingsJSON = %q, want {}", list[0].SettingsJSON)
	}
}

// ---- 3. Add + Switch：live 切换 + 旧行回填 + 备份 ----

func TestAddAndSwitchWritesLiveAndBackfillsOld(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://original.example.com"},
	})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	oldID := list[0].ID

	newJSON, _ := json.Marshal(map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://new.example.com"},
	})
	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeClaude,
		Name:         "new-provider",
		SettingsJSON: string(newJSON),
	})
	if err != nil {
		t.Fatalf("AddAgentProvider: %v", err)
	}
	if added.IsCurrent {
		t.Fatal("新增的 provider 不应自动成为 current")
	}

	if err := svc.SwitchAgentProvider(context.Background(), added.ID); err != nil {
		t.Fatalf("SwitchAgentProvider: %v", err)
	}

	// live 文件已切到新配置
	liveMap := readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://new.example.com" {
		t.Fatalf("live 未切到新配置: %+v", liveMap)
	}

	// 旧行被回填为切换前的 live 内容，且不再是 current
	after, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
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
	var oldMap map[string]any
	if err := json.Unmarshal([]byte(oldRow.SettingsJSON), &oldMap); err != nil {
		t.Fatalf("unmarshal 回填内容: %v", err)
	}
	if oldEnv, _ := oldMap["env"].(map[string]any); oldEnv["ANTHROPIC_BASE_URL"] != "https://original.example.com" {
		t.Fatalf("旧行未回填切换前 live 内容: %+v", oldMap)
	}

	// 备份文件已生成且等于最初内容
	bakMap := readJSONFile(t, settingsPath+".xkpane-bak")
	if bakEnv, _ := bakMap["env"].(map[string]any); bakEnv["ANTHROPIC_BASE_URL"] != "https://original.example.com" {
		t.Fatalf("备份内容不是最初内容: %+v", bakMap)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

// ---- 4. Switch 到已是 current 的行 → no-op ----

func TestSwitchToCurrentIsNoop(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if err := svc.SwitchAgentProvider(context.Background(), list[0].ID); err != nil {
		t.Fatalf("SwitchAgentProvider(current) 应为 no-op, got err: %v", err)
	}
}

// ---- 5. Delete current → 报错；Delete 非 current → 成功 ----

func TestDeleteCurrentRejected(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if err := svc.DeleteAgentProvider(context.Background(), list[0].ID); err == nil {
		t.Fatal("删除 current 行应报错")
	}
}

func TestDeleteNonCurrentSucceeds(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType: appTypeClaude, Name: "extra", SettingsJSON: "{}",
	})
	if err != nil {
		t.Fatalf("AddAgentProvider: %v", err)
	}
	if err := svc.DeleteAgentProvider(context.Background(), added.ID); err != nil {
		t.Fatalf("DeleteAgentProvider(非-current): %v", err)
	}
	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
}

// ---- 6. Update current 行 → live 同步更新 ----

func TestUpdateCurrentSyncsLive(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{"env": map[string]any{"X": "1"}})
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	current := list[0]

	if _, err := svc.UpdateAgentProvider(context.Background(), current.ID, AgentProviderInput{
		Name:         current.Name,
		SettingsJSON: `{"env":{"X":"2"}}`,
	}); err != nil {
		t.Fatalf("UpdateAgentProvider: %v", err)
	}

	liveMap := readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["X"] != "2" {
		t.Fatalf("live 未同步更新: %+v", liveMap)
	}
}

// ---- 7. 加密验证：settings_enc 列不含明文 ----

func TestSettingsEncNotPlaintext(t *testing.T) {
	svc := newTestService(t)
	const token = "sk-super-secret-token-xyz"
	settingsJSON := `{"env":{"ANTHROPIC_AUTH_TOKEN":"` + token + `"}}`

	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType: appTypeClaude, Name: "secret-holder", SettingsJSON: settingsJSON,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider: %v", err)
	}

	var raw string
	if err := svc.db().Raw("SELECT settings_enc FROM agent_providers WHERE id = ?", added.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if raw == "" {
		t.Fatal("settings_enc 应为非空密文")
	}
	if strings.Contains(raw, token) {
		t.Fatalf("settings_enc 疑似明文，含原始 token: %q", raw)
	}

	dec, err := cryptox.DecryptString(raw)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if !strings.Contains(dec, token) {
		t.Fatalf("解密后不含原文 token: %q", dec)
	}
}
