package agentcfg

import (
	"context"
	"testing"

	"xkpane/internal/storage"
)

// 本文件覆盖 migrateClaudeModelToBase：把历史上被误当凭据、塞进 claude 供应商补丁的 model
// 挪回系统配置基底的一次性修复迁移。

// TestMigrateClaudeModelToBase_MovesModelFromPatchToBase 补丁含 model → 迁移后补丁不含
// model、基底 model 等于原补丁值，补丁其它内容（如凭据）不受影响。
func TestMigrateClaudeModelToBase_MovesModelFromPatchToBase(t *testing.T) {
	svc := newTestService(t)
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, map[string]any{
		"model": "opus[1m]",
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://a"},
	})

	ctx := context.Background()
	if err := svc.migrateClaudeModelToBase(ctx); err != nil {
		t.Fatalf("migrateClaudeModelToBase: %v", err)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if base["model"] != "opus[1m]" {
		t.Fatalf("基底 model 应等于原补丁值: %+v", base)
	}

	var row storage.AgentProvider
	if err := svc.db().First(&row, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A: %v", err)
	}
	patch := decryptProviderJSON(t, row.SettingsEnc)
	if _, dup := patch["model"]; dup {
		t.Fatalf("补丁不应再含 model: %+v", patch)
	}
	env, _ := patch["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://a" {
		t.Fatalf("补丁其它内容(凭据)不应受影响: %+v", patch)
	}
}

// TestMigrateClaudeModelToBase_Idempotent 幂等：连续调用两次，第二次不应改动任何数据。
func TestMigrateClaudeModelToBase_Idempotent(t *testing.T) {
	svc := newTestService(t)
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, map[string]any{
		"model": "opus[1m]",
		"env":   map[string]any{"ANTHROPIC_BASE_URL": "https://a"},
	})

	ctx := context.Background()
	if err := svc.migrateClaudeModelToBase(ctx); err != nil {
		t.Fatalf("首次 migrateClaudeModelToBase: %v", err)
	}

	baseBefore, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig(before): %v", err)
	}
	var rowBefore storage.AgentProvider
	if err := svc.db().First(&rowBefore, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A(before): %v", err)
	}

	if err := svc.migrateClaudeModelToBase(ctx); err != nil {
		t.Fatalf("二次 migrateClaudeModelToBase: %v", err)
	}

	baseAfter, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig(after): %v", err)
	}
	if !jsonEqual(baseBefore, baseAfter) {
		t.Fatalf("二次迁移后基底应与首次迁移后完全一致:\n before = %+v\n after  = %+v", baseBefore, baseAfter)
	}

	var rowAfter storage.AgentProvider
	if err := svc.db().First(&rowAfter, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A(after): %v", err)
	}
	if rowBefore.SettingsEnc != rowAfter.SettingsEnc {
		t.Fatal("二次迁移后 A 的补丁密文应与首次迁移后完全一致(标记已置位应直接返回)")
	}
}

// TestMigrateClaudeModelToBase_DoesNotOverwriteExistingBaseModel 基底已有 model 时，
// 迁移不覆盖基底原值（即便某供应商补丁里塞了不同的 model）。
func TestMigrateClaudeModelToBase_DoesNotOverwriteExistingBaseModel(t *testing.T) {
	svc := newTestService(t)
	if err := svc.writeBaseConfig(svc.db(), appTypeClaude, map[string]any{
		"model": "sonnet",
	}); err != nil {
		t.Fatalf("writeBaseConfig: %v", err)
	}
	insertFullProvider(t, svc, storage.AgentProvider{ID: "A", AppType: appTypeClaude, Name: "A", IsCurrent: true}, map[string]any{
		"model": "opus[1m]",
	})

	ctx := context.Background()
	if err := svc.migrateClaudeModelToBase(ctx); err != nil {
		t.Fatalf("migrateClaudeModelToBase: %v", err)
	}

	base, err := svc.readBaseConfig(svc.db(), appTypeClaude)
	if err != nil {
		t.Fatalf("readBaseConfig: %v", err)
	}
	if base["model"] != "sonnet" {
		t.Fatalf("基底已有 model 时不应被覆盖: %+v", base)
	}

	var row storage.AgentProvider
	if err := svc.db().First(&row, "id = ?", "A").Error; err != nil {
		t.Fatalf("查 A: %v", err)
	}
	patch := decryptProviderJSON(t, row.SettingsEnc)
	if _, dup := patch["model"]; dup {
		t.Fatalf("补丁仍应被剥离 model,即便未被用于回填基底: %+v", patch)
	}
}
