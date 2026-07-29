package agentcfg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListSyncsCurrentFromLiveIgnoresCredEdit 验证：打开设置（List）触发的反向同步里，
// 若 live 被软件外改了「凭据形状」的路径（如 env.ANTHROPIC_BASE_URL），按「供应商配置
// 只能由设置面板改，外部编辑绝不能进补丁」的约定被忽略——不写补丁，也不用新值覆盖/清空
// 基底里该路径下原有的值，且不新建行。
func TestListSyncsCurrentFromLiveIgnoresCredEdit(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://old.example.com"},
	})
	mustStartup(t, svc)

	// 模拟用户在软件外（或关闭期间）改了 live 的凭据形状路径——此时 DB 仍是旧值
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://new.example.com"},
	})

	// 打开设置 = List：应忽略这次外部编辑，不新建行
	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if strings.TrimSpace(list[0].SettingsJSON) != "{}" {
		t.Fatalf("外部对凭据路径的编辑不应写入供应商补丁: %q", list[0].SettingsJSON)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	env, _ := base["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://old.example.com" {
		t.Fatalf("基底里的凭据形状路径不应被外部编辑覆盖: %+v", base)
	}
}

// TestSyncCurrentFromLiveNoopWhenEqual 内容一致时 changed=false。
func TestSyncCurrentFromLiveNoopWhenEqual(t *testing.T) {
	svc := newTestService(t)
	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{"k": "v"})
	mustStartup(t, svc)

	changed, err := svc.syncCurrentFromLive(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if changed {
		t.Fatal("内容一致时应 changed=false")
	}
}

// TestSyncCurrentFromLiveSkipsMissingLive live 缺失时不抹掉 current。
func TestSyncCurrentFromLiveSkipsMissingLive(t *testing.T) {
	svc := newTestService(t)
	writeJSONFile(t, svc.claudeSettingsPath(), map[string]any{"keep": true})
	mustStartup(t, svc)

	if err := os.Remove(svc.claudeSettingsPath()); err != nil {
		t.Fatalf("remove live: %v", err)
	}

	changed, err := svc.syncCurrentFromLive(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive: %v", err)
	}
	if changed {
		t.Fatal("live 缺失不应改 current")
	}
	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// U5 两层模型:首启内容落在系统配置基底,claude Provider 补丁仍是空对象,校验点改为
	// 基底而非补丁(补丁本就不含 keep)。
	if strings.TrimSpace(list[0].SettingsJSON) != "{}" {
		t.Fatalf("live 缺失不应改动 claude Provider 补丁: %q", list[0].SettingsJSON)
	}
	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if base["keep"] != true {
		t.Fatalf("current 被错误清空: %+v", base)
	}
}

// TestClassifyLivePath 覆盖关键路径识别。
func TestClassifyLivePath(t *testing.T) {
	svc := newTestService(t)
	cases := []struct {
		path    string
		kind    string
		appType string
		ok      bool
	}{
		{filepath.Join(svc.home, ".claude", "settings.json"), "provider", appTypeClaude, true},
		{filepath.Join(svc.home, ".claude", "CLAUDE.md"), "instruction", appTypeClaude, true},
		{filepath.Join(svc.home, ".codex", "config.toml"), "provider", appTypeCodex, true},
		{filepath.Join(svc.home, ".codex", "auth.json"), "provider", appTypeCodex, true},
		{filepath.Join(svc.home, ".codex", "AGENTS.md"), "instruction", appTypeCodex, true},
		{filepath.Join(svc.home, ".grok", "config.toml"), "provider", appTypeGrok, true},
		{filepath.Join(svc.home, ".claude", "other.txt"), "", "", false},
	}
	for _, tc := range cases {
		kind, appType, ok := svc.classifyLivePath(tc.path)
		if ok != tc.ok || kind != tc.kind || appType != tc.appType {
			t.Errorf("path=%s got (%s,%s,%v) want (%s,%s,%v)",
				tc.path, kind, appType, ok, tc.kind, tc.appType, tc.ok)
		}
	}
}

// TestHandleLiveChangeInstructionEmits 指令文件变更只广播不写 Provider。
func TestHandleLiveChangeInstructionEmits(t *testing.T) {
	svc := newTestService(t)
	mustStartup(t, svc)

	var got []string
	svc.SetEmitter(func(name string, payload any) {
		got = append(got, name)
	})
	svc.handleLiveChange(context.Background(), "instruction", appTypeClaude)
	if len(got) != 1 {
		t.Fatalf("emit count = %d, want 1", len(got))
	}
}
