package agentcfg

import (
	"context"
	"encoding/json"
	"testing"
)

// 本文件覆盖 claude 反向同步（syncClaudeCurrentFromLive）按叶子路径把 live 改动拆回
// 「系统配置基底」与「当前供应商补丁」的行为。owner!=claude 时跳过同步的用例已在
// reclaude_test.go 的 TestSyncCurrentFromLiveSkipsClaudeWhenOwnerIsReclaude 覆盖，此处不重复。

// syncTwoLayerFixture 搭建反向同步测试共用的初态：claude 默认 current provider，
// 基底 hooks.k=v1，当前补丁 env.ANTHROPIC_BASE_URL=https://a（经 UpdateAgentProvider
// 写入，顺带把 live 落成 deepMerge(基底,补丁) 的结果）。
func syncTwoLayerFixture(t *testing.T) *Service {
	t.Helper()
	svc := newTestService(t)
	mustStartup(t, svc)
	ctx := context.Background()

	if err := svc.setClaudeSettingsOwner(ctx, appTypeClaude); err != nil {
		t.Fatalf("setClaudeSettingsOwner: %v", err)
	}
	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{
		"hooks": map[string]any{"k": "v1"},
	}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	current := list[0]
	if _, err := svc.UpdateAgentProvider(ctx, current.ID, AgentProviderInput{
		Name:         current.Name,
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://a"}}`,
	}); err != nil {
		t.Fatalf("UpdateAgentProvider: %v", err)
	}
	return svc
}

// TestSyncClaudeCurrentFromLiveRoutesBaseKeyToBase 改基底键：live 里 hooks.k 变化应回灌基底，
// 当前补丁的凭据键（env.ANTHROPIC_BASE_URL）不受影响。
func TestSyncClaudeCurrentFromLiveRoutesBaseKeyToBase(t *testing.T) {
	svc := syncTwoLayerFixture(t)
	ctx := context.Background()

	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{
		"hooks": map[string]any{"k": "v2"},
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://a"},
	})

	changed, err := svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if !changed {
		t.Fatal("基底键变化应 changed=true")
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	hooks, _ := base["hooks"].(map[string]any)
	if hooks["k"] != "v2" {
		t.Fatalf("基底 hooks.k 应回灌为 v2: %+v", base)
	}

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(list[0].SettingsJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env, _ := got["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://a" {
		t.Fatalf("当前补丁的凭据键不应被污染: %+v", got)
	}
}

// TestSyncClaudeCurrentFromLiveRoutesCredKeyToPatch 改凭据键：live 里 env.ANTHROPIC_BASE_URL
// 变化应回灌当前补丁，基底 hooks.k 不受影响。
func TestSyncClaudeCurrentFromLiveRoutesCredKeyToPatch(t *testing.T) {
	svc := syncTwoLayerFixture(t)
	ctx := context.Background()

	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{
		"hooks": map[string]any{"k": "v1"},
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://b"},
	})

	changed, err := svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if !changed {
		t.Fatal("凭据键变化应 changed=true")
	}

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(list[0].SettingsJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env, _ := got["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://b" {
		t.Fatalf("当前补丁应回灌为 https://b: %+v", got)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	hooks, _ := base["hooks"].(map[string]any)
	if hooks["k"] != "v1" {
		t.Fatalf("基底 hooks.k 不应被污染: %+v", base)
	}
}

// TestSyncClaudeCurrentFromLiveNoopWhenEqual live 与 deepMerge(基底,补丁) 完全一致时无变化。
func TestSyncClaudeCurrentFromLiveNoopWhenEqual(t *testing.T) {
	svc := syncTwoLayerFixture(t)
	ctx := context.Background()

	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{
		"hooks": map[string]any{"k": "v1"},
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://a"},
	})

	changed, err := svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if changed {
		t.Fatal("live 与 deepMerge(基底,补丁) 一致时应 changed=false")
	}
}
