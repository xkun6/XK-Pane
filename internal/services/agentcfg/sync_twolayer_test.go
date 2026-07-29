package agentcfg

import (
	"context"
	"encoding/json"
	"testing"
)

// 本文件覆盖 claude 反向同步（syncClaudeCurrentFromLive）按叶子路径把 live 改动拆回
// 「系统配置基底」与「当前供应商补丁」的行为。

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

// TestSyncClaudeCurrentFromLiveKeepsPatchIntact 改凭据键：live 里 env.ANTHROPIC_BASE_URL
// 变化落在当前补丁已声明的路径上，按「供应商补丁只由设置面板改写」的约定被忽略——
// changed=false、当前补丁仍是原值、基底不含该路径，外部改动不会污染供应商配置。
func TestSyncClaudeCurrentFromLiveKeepsPatchIntact(t *testing.T) {
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
	if changed {
		t.Fatal("凭据键变化应被忽略,changed 应为 false")
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
		t.Fatalf("当前补丁不应被外部改动污染,应仍是 https://a: %+v", got)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	hooks, _ := base["hooks"].(map[string]any)
	if hooks["k"] != "v1" {
		t.Fatalf("基底 hooks.k 不应被污染: %+v", base)
	}
	if _, ok := base["env"]; ok {
		t.Fatalf("基底不应含凭据路径 env: %+v", base)
	}
}

// TestSyncClaudeCurrentFromLiveRoutesModelToBase 回归测试：claude CLI 自己改写 live 的
// model(如 /model 切模型)必须沉淀进系统配置基底,而不是被反向同步塞进当前供应商补丁
// (历史 bug:model 曾被当凭据,导致供应商配置被替换成 {"model":"opus[1m]"})。
func TestSyncClaudeCurrentFromLiveRoutesModelToBase(t *testing.T) {
	svc := syncTwoLayerFixture(t)
	ctx := context.Background()

	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{
		"hooks": map[string]any{"k": "v1"},
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://a"},
		"model": "opus[1m]",
	})

	changed, err := svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if !changed {
		t.Fatal("新增 model 应 changed=true")
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if base["model"] != "opus[1m]" {
		t.Fatalf("model 应沉淀进系统配置基底: %+v", base)
	}

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(list[0].SettingsJSON), &patch); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, dup := patch["model"]; dup {
		t.Fatalf("当前供应商补丁不应含 model: %+v", patch)
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
