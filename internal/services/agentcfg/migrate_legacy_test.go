package agentcfg

import (
	"context"
	"encoding/json"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	toml "github.com/pelletier/go-toml/v2"
)

// insertLegacyProvider 直接向旧 providers 表插入一行（APIKey 走 cryptox 加密，模拟历史数据），
// 绕开 provider.Service.Add 的类型白名单——白名单收缩正是本次迁移要清理的对象。
func insertLegacyProvider(t *testing.T, svc *Service, p storage.Provider) {
	t.Helper()
	enc, err := cryptox.EncryptString(p.APIKey)
	if err != nil {
		t.Fatalf("cryptox.EncryptString: %v", err)
	}
	p.APIKey = enc
	if err := svc.db().Create(&p).Error; err != nil {
		t.Fatalf("insert legacy provider: %v", err)
	}
}

func TestMigrateLegacyProviders(t *testing.T) {
	svc := newTestService(t)

	insertLegacyProvider(t, svc, storage.Provider{
		ID: "legacy-anthropic", Name: "老 Claude 直连", ProviderType: "anthropic",
		APIKey: "sk-ant-legacy", BaseURL: "https://api.anthropic.example.com",
	})
	insertLegacyProvider(t, svc, storage.Provider{
		ID: "legacy-bedrock", Name: "老 Bedrock", ProviderType: "bedrock",
		Region: "us-east-1", AWSProfile: "prod",
	})
	insertLegacyProvider(t, svc, storage.Provider{
		ID: "legacy-openai", Name: "老 Codex 直连", ProviderType: "open_ai",
		APIKey: "sk-codex-legacy", BaseURL: "https://api.openai.example.com/v1",
	})
	insertLegacyProvider(t, svc, storage.Provider{
		ID: "legacy-config-profile", Name: "老配置目录", ProviderType: "config_profile",
		ConfigDir: "/home/user/.claude-work",
	})
	insertLegacyProvider(t, svc, storage.Provider{
		ID: "legacy-glm", Name: "智谱", ProviderType: "glm",
		APIKey: "zk-legacy", BaseURL: "https://open.bigmodel.cn/api/paas/v4",
	})

	ctx := context.Background()
	if err := svc.migrateLegacyProviders(ctx); err != nil {
		t.Fatalf("migrateLegacyProviders: %v", err)
	}

	// agent_providers 多 3 行：2 claude（anthropic + bedrock）+ 1 codex（open_ai）。
	var claudeRows, codexRows []storage.AgentProvider
	if err := svc.db().Where("app_type = ?", appTypeClaude).Find(&claudeRows).Error; err != nil {
		t.Fatalf("query claude rows: %v", err)
	}
	if err := svc.db().Where("app_type = ?", appTypeCodex).Find(&codexRows).Error; err != nil {
		t.Fatalf("query codex rows: %v", err)
	}
	if len(claudeRows) != 2 {
		t.Fatalf("claude agent_providers 行数 = %d, want 2", len(claudeRows))
	}
	if len(codexRows) != 1 {
		t.Fatalf("codex agent_providers 行数 = %d, want 1", len(codexRows))
	}

	// IsCurrent 全 false：迁移只登记，不切换、不碰 live 文件。
	for _, row := range claudeRows {
		if row.IsCurrent {
			t.Errorf("迁移的 claude 行不应 IsCurrent: %+v", row)
		}
	}
	for _, row := range codexRows {
		if row.IsCurrent {
			t.Errorf("迁移的 codex 行不应 IsCurrent: %+v", row)
		}
	}

	// 逐行校验 settings 明文内容（anthropic → ANTHROPIC_*；bedrock → CLAUDE_CODE_USE_BEDROCK 一族）。
	var anthropicFound, bedrockFound bool
	for _, row := range claudeRows {
		plain, err := cryptox.DecryptString(row.SettingsEnc)
		if err != nil {
			t.Fatalf("decrypt claude settings: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(plain), &m); err != nil {
			t.Fatalf("unmarshal claude settings: %v", err)
		}
		env, _ := m["env"].(map[string]any)
		switch {
		case env["ANTHROPIC_API_KEY"] != nil:
			anthropicFound = true
			if env["ANTHROPIC_API_KEY"] != "sk-ant-legacy" {
				t.Errorf("ANTHROPIC_API_KEY = %v, want sk-ant-legacy", env["ANTHROPIC_API_KEY"])
			}
			if env["ANTHROPIC_BASE_URL"] != "https://api.anthropic.example.com" {
				t.Errorf("ANTHROPIC_BASE_URL = %v", env["ANTHROPIC_BASE_URL"])
			}
		case env["CLAUDE_CODE_USE_BEDROCK"] != nil:
			bedrockFound = true
			if env["CLAUDE_CODE_USE_BEDROCK"] != "1" {
				t.Errorf("CLAUDE_CODE_USE_BEDROCK = %v, want 1", env["CLAUDE_CODE_USE_BEDROCK"])
			}
			if env["AWS_REGION"] != "us-east-1" || env["AWS_PROFILE"] != "prod" {
				t.Errorf("bedrock env 错误: %+v", env)
			}
		default:
			t.Fatalf("未识别的 claude 迁移行 env: %+v", env)
		}
	}
	if !anthropicFound || !bedrockFound {
		t.Fatalf("anthropic/bedrock 迁移行缺失: anthropicFound=%v bedrockFound=%v", anthropicFound, bedrockFound)
	}

	// codex 行：auth.OPENAI_API_KEY 正确 + config 是可解析的 TOML，且声明了 xkpane-migrated provider。
	codexPlain, err := cryptox.DecryptString(codexRows[0].SettingsEnc)
	if err != nil {
		t.Fatalf("decrypt codex settings: %v", err)
	}
	cs, err := parseCodexSettings(codexPlain)
	if err != nil {
		t.Fatalf("parseCodexSettings: %v", err)
	}
	if cs.Auth["OPENAI_API_KEY"] != "sk-codex-legacy" {
		t.Errorf("codex auth OPENAI_API_KEY = %v, want sk-codex-legacy", cs.Auth["OPENAI_API_KEY"])
	}
	var tomlCheck map[string]any
	if err := toml.Unmarshal([]byte(cs.Config), &tomlCheck); err != nil {
		t.Fatalf("codex config 不是合法 TOML: %v", err)
	}
	if tomlCheck["model_provider"] != "xkpane-migrated" {
		t.Errorf("model_provider = %v, want xkpane-migrated", tomlCheck["model_provider"])
	}

	// providers 表只剩 config_profile 与 glm（未迁移、未删除）。
	var remaining []storage.Provider
	if err := svc.db().Order("rowid").Find(&remaining).Error; err != nil {
		t.Fatalf("query remaining providers: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("剩余 providers 行数 = %d, want 2: %+v", len(remaining), remaining)
	}
	gotTypes := map[string]bool{}
	for _, r := range remaining {
		gotTypes[r.ProviderType] = true
	}
	if !gotTypes["config_profile"] || !gotTypes["glm"] {
		t.Fatalf("剩余 provider 类型 = %+v, want config_profile+glm", gotTypes)
	}

	// 二次运行幂等：查不到匹配行，无新增。
	if err := svc.migrateLegacyProviders(ctx); err != nil {
		t.Fatalf("second migrateLegacyProviders: %v", err)
	}
	var claudeRows2, codexRows2 []storage.AgentProvider
	if err := svc.db().Where("app_type = ?", appTypeClaude).Find(&claudeRows2).Error; err != nil {
		t.Fatalf("query claude rows (2nd): %v", err)
	}
	if err := svc.db().Where("app_type = ?", appTypeCodex).Find(&codexRows2).Error; err != nil {
		t.Fatalf("query codex rows (2nd): %v", err)
	}
	if len(claudeRows2) != 2 || len(codexRows2) != 1 {
		t.Fatalf("二次迁移后行数变化：claude=%d codex=%d", len(claudeRows2), len(codexRows2))
	}
}
