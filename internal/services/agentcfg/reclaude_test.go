package agentcfg

import (
	"context"
	"strings"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

// ---- 1. reclaude 切换只翻 IsCurrent 指针，不写 live、不碰 owner ----

func TestReclaudeSwitchOnlyFlipsPointerNoLiveWrite(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://claude-live.example.com"},
	})
	mustStartup(t, svc) // 从 live 各自导入一份 claude/reclaude 默认配置

	added, err := svc.AddAgentProvider(context.Background(), AgentProviderInput{
		AppType:      appTypeReclaude,
		Name:         "reclaude-new",
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://reclaude-new.example.com"}}`,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider(reclaude): %v", err)
	}
	if added.IsCurrent {
		t.Fatal("新增的 reclaude provider 不应自动成为 current")
	}

	if err := svc.SwitchAgentProvider(context.Background(), added.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(reclaude): %v", err)
	}

	// live 文件必须保持原样（reclaude 切换不写 live）
	liveMap := readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://claude-live.example.com" {
		t.Fatalf("reclaude 切换不应改动共享 live 文件: %+v", liveMap)
	}

	list, err := svc.ListAgentProviders(context.Background(), appTypeReclaude)
	if err != nil {
		t.Fatalf("ListAgentProviders(reclaude): %v", err)
	}
	var newRow, foundOther bool
	for _, row := range list {
		if row.ID == added.ID {
			newRow = row.IsCurrent
		} else if row.IsCurrent {
			foundOther = true
		}
	}
	if !newRow {
		t.Fatal("新行应变为 IsCurrent")
	}
	if foundOther {
		t.Fatal("旧 reclaude 行应不再是 IsCurrent（单一 current 不变式）")
	}

	// owner 标记不应被 reclaude 切换动到（缺省仍是 claude）
	if got := svc.getClaudeSettingsOwner(context.Background()); got != appTypeClaude {
		t.Fatalf("owner = %q, want %q（reclaude 切换不应碰 owner）", got, appTypeClaude)
	}
}

// ---- 2. ApplyForLaunch 双向：claude/reclaude 各自的当前配置正确落地到共享文件 + owner 正确 ----

func TestApplyForLaunchSwitchesSharedFileBothDirections(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	mustStartup(t, svc) // 无 live 文件 → claude/reclaude 各自得到 "{}" 默认行

	ctx := context.Background()

	claudeAdded, err := svc.AddAgentProvider(ctx, AgentProviderInput{
		AppType:      appTypeClaude,
		Name:         "claude-c",
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://claude-c.example.com"}}`,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider(claude): %v", err)
	}
	if err := svc.SwitchAgentProvider(ctx, claudeAdded.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(claude): %v", err)
	}

	reclaudeAdded, err := svc.AddAgentProvider(ctx, AgentProviderInput{
		AppType:      appTypeReclaude,
		Name:         "reclaude-d",
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://reclaude-d.example.com"}}`,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider(reclaude): %v", err)
	}
	if err := svc.SwitchAgentProvider(ctx, reclaudeAdded.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(reclaude): %v", err)
	}

	// 此刻共享文件仍是 claude 的内容（switchClaude 写过；switchReclaude 不写）
	liveMap := readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://claude-c.example.com" {
		t.Fatalf("切换 reclaude 前 live 应仍是 claude 内容: %+v", liveMap)
	}

	// 启动 reclaude 前应用 → 共享文件切成 reclaude 当前配置，owner=reclaude
	if err := svc.ApplyForLaunch(ctx, appTypeReclaude); err != nil {
		t.Fatalf("ApplyForLaunch(reclaude): %v", err)
	}
	liveMap = readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://reclaude-d.example.com" {
		t.Fatalf("ApplyForLaunch(reclaude) 未把 live 切成 reclaude 当前配置: %+v", liveMap)
	}
	if got := svc.getClaudeSettingsOwner(ctx); got != appTypeReclaude {
		t.Fatalf("owner = %q, want %q", got, appTypeReclaude)
	}

	// 启动 claude 前应用 → 切回 claude 当前配置，owner=claude
	if err := svc.ApplyForLaunch(ctx, appTypeClaude); err != nil {
		t.Fatalf("ApplyForLaunch(claude): %v", err)
	}
	liveMap = readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://claude-c.example.com" {
		t.Fatalf("ApplyForLaunch(claude) 未把 live 切回 claude 当前配置: %+v", liveMap)
	}
	if got := svc.getClaudeSettingsOwner(ctx); got != appTypeClaude {
		t.Fatalf("owner = %q, want %q", got, appTypeClaude)
	}
}

// ---- 3. ApplyForLaunch 在无当前 Provider 时 no-op（不动文件、不报错）----

func TestApplyForLaunchNoopWhenNoCurrentProvider(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{"env": map[string]any{"X": "untouched"}})
	// 刻意不调用 mustStartup：DB 里没有任何 claude/reclaude Provider 行。

	if err := svc.ApplyForLaunch(context.Background(), appTypeReclaude); err != nil {
		t.Fatalf("ApplyForLaunch(reclaude) 无当前行应 no-op, got err: %v", err)
	}

	liveMap := readJSONFile(t, settingsPath)
	if env, _ := liveMap["env"].(map[string]any); env["X"] != "untouched" {
		t.Fatalf("无当前 Provider 时 ApplyForLaunch 不应改动 live 文件: %+v", liveMap)
	}
}

// ---- 4. owner 串味防护：owner=reclaude 期间 syncCurrentFromLive(claude) 应跳过；owner 改回
//         claude 后应恢复正常捕获 ----

func TestSyncCurrentFromLiveSkipsClaudeWhenOwnerIsReclaude(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://original.example.com"},
	})
	mustStartup(t, svc)
	ctx := context.Background()

	if err := svc.setClaudeSettingsOwner(ctx, appTypeReclaude); err != nil {
		t.Fatalf("setClaudeSettingsOwner(reclaude): %v", err)
	}

	// 模拟 reclaude 占用期间共享文件被改写（reclaude 运行时自改，或用户误改）
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://reclaude-runtime.example.com"},
	})

	changed, err := svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive(claude) owner=reclaude 期间: %v", err)
	}
	if changed {
		t.Fatal("owner=reclaude 期间不应回灌 live 内容进 claude Provider")
	}

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders(claude): %v", err)
	}
	// U5 两层模型:首启内容落在系统配置基底,claude Provider 补丁仍是空对象。
	if strings.TrimSpace(list[0].SettingsJSON) != "{}" {
		t.Fatalf("owner=reclaude 期间不应改动 claude Provider 补丁: %q", list[0].SettingsJSON)
	}
	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if env, _ := base["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://original.example.com" {
		t.Fatalf("claude 系统配置基底被串味污染: %+v", base)
	}

	// owner 改回 claude 后应恢复正常捕获
	if err := svc.setClaudeSettingsOwner(ctx, appTypeClaude); err != nil {
		t.Fatalf("setClaudeSettingsOwner(claude): %v", err)
	}
	changed, err = svc.syncCurrentFromLive(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("syncCurrentFromLive(claude) owner=claude 后: %v", err)
	}
	if !changed {
		t.Fatal("owner 改回 claude 后应正常捕获 live 变化")
	}
}

// ---- 5. switchClaude 在 owner=reclaude 时跳过旧行内容回填（只清 IsCurrent，避免把 reclaude
//         占用期的 live 内容串进 claude 旧 Provider）----

func TestSwitchClaudeSkipsBackfillWhenOwnerIsReclaude(t *testing.T) {
	svc := newTestService(t)
	settingsPath := svc.claudeSettingsPath()
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://claude-original.example.com"},
	})
	mustStartup(t, svc)
	ctx := context.Background()

	list, err := svc.ListAgentProviders(ctx, appTypeClaude)
	if err != nil {
		t.Fatalf("ListAgentProviders(claude): %v", err)
	}
	oldID := list[0].ID

	claudeNew, err := svc.AddAgentProvider(ctx, AgentProviderInput{
		AppType:      appTypeClaude,
		Name:         "claude-new",
		SettingsJSON: `{"env":{"ANTHROPIC_BASE_URL":"https://claude-new.example.com"}}`,
	})
	if err != nil {
		t.Fatalf("AddAgentProvider(claude): %v", err)
	}

	// 把文件归属标记为 reclaude，并模拟共享文件此刻是 reclaude 占用期内容
	if err := svc.setClaudeSettingsOwner(ctx, appTypeReclaude); err != nil {
		t.Fatalf("setClaudeSettingsOwner(reclaude): %v", err)
	}
	writeJSONFile(t, settingsPath, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "https://reclaude-runtime.example.com"},
	})

	if err := svc.SwitchAgentProvider(ctx, claudeNew.ID); err != nil {
		t.Fatalf("SwitchAgentProvider(claude): %v", err)
	}

	// 旧行内容不应被 reclaude 占用期的 live 内容回填，但 IsCurrent 必须已清除
	var oldRow storage.AgentProvider
	if err := svc.db().WithContext(ctx).First(&oldRow, "id = ?", oldID).Error; err != nil {
		t.Fatalf("查旧行: %v", err)
	}
	if oldRow.IsCurrent {
		t.Fatal("旧行应不再是 current（单一 current 不变式）")
	}
	plain, err := cryptox.DecryptString(oldRow.SettingsEnc)
	if err != nil {
		t.Fatalf("解密旧行: %v", err)
	}
	// U5 两层模型:owner!=claude 时 switchClaude 跳过回填,旧行补丁应仍是首启时的空对象。
	if strings.TrimSpace(plain) != "{}" {
		t.Fatalf("旧行补丁不应被 reclaude 占用期的 live 内容回填: %s", plain)
	}
	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if env, _ := base["env"].(map[string]any); env["ANTHROPIC_BASE_URL"] != "https://claude-original.example.com" {
		t.Fatalf("claude 系统配置基底被 reclaude 占用期的 live 内容串味污染: %+v", base)
	}

	// switchClaude 结束后 owner 应被重新标记为 claude
	if got := svc.getClaudeSettingsOwner(ctx); got != appTypeClaude {
		t.Fatalf("owner = %q, want %q", got, appTypeClaude)
	}
}
