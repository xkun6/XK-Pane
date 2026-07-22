package agentcfg

import (
	"context"
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
