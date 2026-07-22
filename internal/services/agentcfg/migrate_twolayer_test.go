package agentcfg

import (
	"context"
	"encoding/json"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
)

// insertFullProvider 直接向 agent_providers 插入一行「整份」配置(老数据形态：SettingsEnc
// 是完整 settings.json，非补丁)，绕开两层模型的 import/Add 逻辑，模拟迁移前的历史数据。
func insertFullProvider(t *testing.T, svc *Service, p storage.AgentProvider, full map[string]any) {
	t.Helper()
	data, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	enc, err := cryptox.EncryptString(string(data))
	if err != nil {
		t.Fatalf("cryptox.EncryptString: %v", err)
	}
	p.SettingsEnc = enc
	if p.Category == "" {
		p.Category = "custom"
	}
	if err := svc.db().Create(&p).Error; err != nil {
		t.Fatalf("insert full provider: %v", err)
	}
}

// decryptProviderJSON 解密 Provider 行的 SettingsEnc 并解析为 map，供断言补丁内容。
func decryptProviderJSON(t *testing.T, enc string) map[string]any {
	t.Helper()
	plain, err := cryptox.DecryptString(enc)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(plain), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// TestMigrateClaudeTwoLayer_LegacyUsers 老用户迁移：取 IsCurrent 行整份作基底，各行补丁 =
// deepDiff(整份, 基底)——当前行补丁收敛为空（切回零丢失），其它行只保留相对基底的差异。
func TestMigrateClaudeTwoLayer_LegacyUsers(t *testing.T) {
	svc := newTestService(t)

	fullA := map[string]any{
		"hooks": map[string]any{"k": "v"},
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "ta"},
	}
	fullB := map[string]any{
		"hooks": map[string]any{"k": "v"},
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "tb"},
	}
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, fullA)
	insertFullProvider(t, svc, storage.AgentProvider{ID: "B", AppType: appTypeClaude, Name: "B", IsCurrent: false}, fullB)

	ctx := context.Background()
	if err := svc.migrateClaudeTwoLayer(ctx); err != nil {
		t.Fatalf("migrateClaudeTwoLayer: %v", err)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if !jsonEqual(base, fullA) {
		t.Fatalf("基底应取自 IsCurrent 行 A 的整份内容:\n got  = %+v\n want = %+v", base, fullA)
	}

	var rowA, rowB storage.AgentProvider
	if err := svc.db().First(&rowA, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A: %v", err)
	}
	if err := svc.db().First(&rowB, "id = ?", "B").Error; err != nil {
		t.Fatalf("查 B: %v", err)
	}

	patchA := decryptProviderJSON(t, rowA.SettingsEnc)
	if len(patchA) != 0 {
		t.Fatalf("当前供应商 A 的补丁应收敛为空(切回它零丢失): %+v", patchA)
	}

	patchB := decryptProviderJSON(t, rowB.SettingsEnc)
	wantPatchB := map[string]any{"env": map[string]any{"ANTHROPIC_AUTH_TOKEN": "tb"}}
	if !jsonEqual(patchB, wantPatchB) {
		t.Fatalf("B 的补丁应只含相对基底的差异(公共 hooks 不重复进补丁):\n got  = %+v\n want = %+v", patchB, wantPatchB)
	}

	merged := deepMergeJSON(base, patchB)
	if !jsonEqual(merged, fullB) {
		t.Fatalf("deepMergeJSON(基底, B补丁) 应语义等价于 B 原整份:\n merged = %+v\n want   = %+v", merged, fullB)
	}
}

// TestMigrateClaudeTwoLayer_Idempotent 幂等：迁移标记置位后二次调用直接返回，不应改动
// 已迁移生成的基底与各行补丁密文。
func TestMigrateClaudeTwoLayer_Idempotent(t *testing.T) {
	svc := newTestService(t)

	fullA := map[string]any{
		"hooks": map[string]any{"k": "v"},
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "ta"},
	}
	fullB := map[string]any{
		"hooks": map[string]any{"k": "v"},
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "tb"},
	}
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, fullA)
	insertFullProvider(t, svc, storage.AgentProvider{ID: "B", AppType: appTypeClaude, Name: "B", IsCurrent: false}, fullB)

	ctx := context.Background()
	if err := svc.migrateClaudeTwoLayer(ctx); err != nil {
		t.Fatalf("首次 migrateClaudeTwoLayer: %v", err)
	}

	baseBefore, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig(before): %v", err)
	}
	var rowABefore, rowBBefore storage.AgentProvider
	if err := svc.db().First(&rowABefore, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A(before): %v", err)
	}
	if err := svc.db().First(&rowBBefore, "id = ?", "B").Error; err != nil {
		t.Fatalf("查 B(before): %v", err)
	}

	if err := svc.migrateClaudeTwoLayer(ctx); err != nil {
		t.Fatalf("二次 migrateClaudeTwoLayer: %v", err)
	}

	baseAfter, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig(after): %v", err)
	}
	if !jsonEqual(baseBefore, baseAfter) {
		t.Fatalf("二次迁移后基底应与首次迁移后完全一致:\n before = %+v\n after  = %+v", baseBefore, baseAfter)
	}

	var rowAAfter, rowBAfter storage.AgentProvider
	if err := svc.db().First(&rowAAfter, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A(after): %v", err)
	}
	if err := svc.db().First(&rowBAfter, "id = ?", "B").Error; err != nil {
		t.Fatalf("查 B(after): %v", err)
	}
	if rowABefore.SettingsEnc != rowAAfter.SettingsEnc {
		t.Fatal("二次迁移后 A 的补丁密文应与首次迁移后完全一致(标记已置位应直接返回)")
	}
	if rowBBefore.SettingsEnc != rowBAfter.SettingsEnc {
		t.Fatal("二次迁移后 B 的补丁密文应与首次迁移后完全一致(标记已置位应直接返回)")
	}
}

// TestMigrateClaudeTwoLayer_SkipsWhenBaseExists 基底已存在（首启已建）时该 appType 应跳过
// 迁移，绝不用某一行的整份内容覆盖已有基底。
func TestMigrateClaudeTwoLayer_SkipsWhenBaseExists(t *testing.T) {
	svc := newTestService(t)

	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{"model": "opus"}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, map[string]any{
		"model": "sonnet",
		"env":   map[string]any{"ANTHROPIC_AUTH_TOKEN": "ta"},
	})

	ctx := context.Background()
	if err := svc.migrateClaudeTwoLayer(ctx); err != nil {
		t.Fatalf("migrateClaudeTwoLayer: %v", err)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	want := map[string]any{"model": "opus"}
	if !jsonEqual(base, want) {
		t.Fatalf("基底已存在时不应被 Provider 整份覆盖:\n got  = %+v\n want = %+v", base, want)
	}
}
