package agentcfg

import (
	"context"
	"encoding/json"
	"testing"
)

// 本文件覆盖「系统配置基底 + 供应商凭据补丁」两层深合并模型接入 Update/Switch/ApplyForLaunch
// 三条 claude 写 live 路径后的集成行为：live 应始终是 deepMerge(基底, 补丁) 的结果。

func TestUpdateCurrentMergesWithBaseConfig(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	current := list[0]

	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{
		"hooks": map[string]any{"k": "v"},
	}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}

	if _, err := svc.UpdateAgentProvider(context.Background(), current.ID, AgentProviderInput{
		Name:         current.Name,
		SettingsJSON: `{"env":{"ANTHROPIC_AUTH_TOKEN":"tkn"}}`,
	}); err != nil {
		t.Fatalf("UpdateAgentProvider: %v", err)
	}

	liveMap := readJSONFile(t, svc.claudeSettingsPath())
	hooks, _ := liveMap["hooks"].(map[string]any)
	if hooks["k"] != "v" {
		t.Fatalf("live 应保留基底 hooks.k: %+v", liveMap)
	}
	env, _ := liveMap["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "tkn" {
		t.Fatalf("live 应含补丁 env.ANTHROPIC_AUTH_TOKEN: %+v", liveMap)
	}
}

func TestSwitchAgentProviderKeepsSharedBaseConfig(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	_ = list[0] // A：mustStartup 默认导入的 current 行（live 不存在，patch 为 "{}"）

	b, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeClaude,
		Name:         "B",
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://b"}}`,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider(B): %v", err)
	}

	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{
		"hooks": map[string]any{"k": "v"},
	}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}

	if err := svc.SwitchAgentProvider(context.Background(), b.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(B): %v", err)
	}

	liveMap := readJSONFile(t, svc.claudeSettingsPath())
	hooks, _ := liveMap["hooks"].(map[string]any)
	if hooks["k"] != "v" {
		t.Fatalf("切换后 live 应仍含公共基底 hooks.k: %+v", liveMap)
	}
	env, _ := liveMap["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://b" {
		t.Fatalf("切换后 live 应含 B 的凭据补丁: %+v", liveMap)
	}
}

func TestApplyForLaunchMergesWithBaseConfig(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	current := list[0]

	if _, err := svc.UpdateAgentProvider(context.Background(), current.ID, AgentProviderInput{
		Name:         current.Name,
		SettingsJSON: `{"env":{"ANTHROPIC_AUTH_TOKEN":"t"}}`,
	}); err != nil {
		t.Fatalf("UpdateAgentProvider: %v", err)
	}

	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{
		"model": "opus",
	}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}

	if err := svc.ApplyForLaunch(context.Background(), appTypeClaude); err != nil {
		t.Fatalf("ApplyForLaunch: %v", err)
	}

	liveMap := readJSONFile(t, svc.claudeSettingsPath())
	if liveMap["model"] != "opus" {
		t.Fatalf("live 应含基底 model: %+v", liveMap)
	}
	env, _ := liveMap["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "t" {
		t.Fatalf("live 应含补丁 env.ANTHROPIC_AUTH_TOKEN: %+v", liveMap)
	}
}

// TestManualImportComputesDiffAgainstBaseConfig 手动重新导入(已有同类 provider)时不应碰
// 基底,新行的补丁应只含「live 相对现有基底」的差异,而非把基底内容也重复存一份。
func TestManualImportComputesDiffAgainstBaseConfig(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"model": "opus",
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "tkn-old"},
	})
	mustStartup(t, svc) // 首启:基底 = live 整体,默认供应商补丁为空
	ctx := context.Background()

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}

	// 模拟用户手动改了 live(在基底之上新增一个 base_url),随后手动触发重新导入。
	newLive := deepMergeJSON(base, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://x"},
	})
	writeJSONFile(t, settingsPath, newLive)

	dto, err := svc.ImportCurrentAgentConfig(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ImportCurrentAgentConfig: %v", err)
	}
	if dto.IsCurrent {
		t.Fatal("手动导入的新行不应自动成为 current")
	}

	var patch map[string]any
	if err := json.Unmarshal([]byte(dto.SettingsJSON), &patch); err != nil {
		t.Fatalf("unmarshal 补丁: %v", err)
	}
	patchEnv, _ := patch["env"].(map[string]any)
	if patchEnv["ANTHROPIC_BASE_URL"] != "https://x" {
		t.Fatalf("手动导入补丁应含相对基底新增的 ANTHROPIC_BASE_URL: %+v", patch)
	}
	if _, dup := patch["model"]; dup {
		t.Fatalf("手动导入补丁不应重复基底已有的 model: %+v", patch)
	}
	if _, dup := patchEnv["ANTHROPIC_AUTH_TOKEN"]; dup {
		t.Fatalf("手动导入补丁不应重复基底已有的 ANTHROPIC_AUTH_TOKEN: %+v", patch)
	}

	// 基底本身不应被手动导入改动。
	baseAfter, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig(after): %v", err)
	}
	if !jsonEqual(base, baseAfter) {
		t.Fatalf("手动导入不应改动系统配置基底:\n before = %+v\n after  = %+v", base, baseAfter)
	}
}
