package agentcfg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestListSyncsCurrentFromLive 验证：打开设置（List）时会把 live 改动灌回 current，
// 启动本身不再做全量同步。
func TestListSyncsCurrentFromLive(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://old.example.com"},
	})
	mustStartup(t, svc)

	// 模拟用户在软件外（或关闭期间）改了 live——此时 DB 仍是旧值
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://new.example.com"},
	})

	// 打开设置 = List：应同步 current，且不新建行
	list, err := svc.ListAgentProviders(context.Background(), appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(list[0].SettingsJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env, _ := got["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://new.example.com" {
		t.Fatalf("List 未把 live 同步进 current: %+v", got)
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
	var got map[string]any
	_ = json.Unmarshal([]byte(list[0].SettingsJSON), &got)
	if got["keep"] != true {
		t.Fatalf("current 被错误清空: %+v", got)
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
