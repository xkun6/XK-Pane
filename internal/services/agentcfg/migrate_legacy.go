package agentcfg

import (
	"context"
	"encoding/json"
	"fmt"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// migrate_legacy.go 是【一次性】把旧 internal/services/provider 里 claude/codex 相关类型
// （anthropic/proxy/bedrock/vertex/open_ai）迁移成 agentcfg 全局配置 Provider 的收尾脚本。
//
// 背景：providers 表本是「启动时拼装几个零散 env 变量」的轻量模型（cc-pane MVP 遗留）；claude/
// codex 已改用 agentcfg 的「整份 live 配置文件」模型（cc-switch 式，直写 ~/.claude/settings.json
// 或 ~/.codex/config.toml+auth.json）。旧表里如果还留着这 5 种 claude/codex 相关类型的行（升级前
// 的老用户 / 尚未清理的历史数据），本函数负责把它们逐条换算成 agentcfg 的 AgentProvider 行
// （IsCurrent=false，仅登记，不切换、不碰 live 文件），随后删除旧行——迁移完成后旧表里就不会
// 再有这些类型，函数天然幂等（下次启动查不到匹配行直接返回）。
//
// config_profile 不在迁移范围：它依赖「目录级配置」（CLAUDE_CONFIG_DIR 指向另一份完整的
// ~/.claude 目录），agentcfg 的模型是「一份 settings.json 内容」，两者不同构，没有可靠的自动
// 换算方式——该类型的旧行原地保留、永久休眠（provider.Service.List 已把它过滤掉，不再出现在
// 任何 UI），不在本函数处理范围。
//
// 调用点：agentcfg.Service.ServiceStartup，紧跟在 claude/codex/mcp/skills 四个既有导入步骤之后，
// 沿用同一套「失败只记日志、不阻断启动」的容错模式。
var legacyProviderTypes = []string{"anthropic", "proxy", "bedrock", "vertex", "open_ai"}

// migrateLegacyProviders 查旧 providers 表中 legacyProviderTypes 命中的行，逐条迁移。单行失败
// 只记日志跳过（不影响其它行）；查询本身失败才把错误向上抛给调用方（ServiceStartup 统一记一条
// warn，与其它导入步骤对齐）。
func (s *Service) migrateLegacyProviders(ctx context.Context) error {
	var rows []storage.Provider
	if err := s.db().WithContext(ctx).Where("provider_type IN ?", legacyProviderTypes).
		Order("rowid").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil // 幂等：迁移完成后本就查不到匹配行
	}
	for _, row := range rows {
		if err := s.migrateOneLegacyProvider(ctx, row); err != nil {
			s.log.Warn("migrate legacy provider failed，跳过该行", "id", row.ID, "name", row.Name, "type", row.ProviderType, "err", err)
		}
	}
	return nil
}

// migrateOneLegacyProvider 迁移单行：解密旧 APIKey → 按类型换算成 agentcfg 的 SettingsJSON →
// 同一事务内建新 AgentProvider 行 + 删旧 Provider 行。
func (s *Service) migrateOneLegacyProvider(ctx context.Context, row storage.Provider) error {
	apiKey := ""
	if row.APIKey != "" {
		dec, err := cryptox.DecryptString(row.APIKey)
		if err != nil {
			return fmt.Errorf("解密旧 apiKey 失败: %w", err)
		}
		apiKey = dec
	}

	appType, settingsJSON, err := legacyProviderSettings(row, apiKey)
	if err != nil {
		return err
	}

	enc, err := cryptox.EncryptString(settingsJSON)
	if err != nil {
		return fmt.Errorf("加密迁移后配置失败: %w", err)
	}

	newRow := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appType,
		Name:        row.Name,
		SettingsEnc: enc,
		Category:    "imported",
		IsCurrent:   false,
		Notes:       fmt.Sprintf("由旧接入 Provider（类型 %s）自动迁移", row.ProviderType),
	}

	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newRow).Error; err != nil {
			return fmt.Errorf("写入新 AgentProvider 失败: %w", err)
		}
		if err := tx.Delete(&storage.Provider{}, "id = ?", row.ID).Error; err != nil {
			return fmt.Errorf("删除旧 provider 行失败: %w", err)
		}
		return nil
	})
}

// legacyProviderSettings 按旧 ProviderType 把一行换算成 agentcfg 的 (appType, settingsJSON)：
//   - anthropic / proxy → claude，settings = {"env":{...}}，key 非空写 ANTHROPIC_API_KEY，
//     BaseURL 非空写 ANTHROPIC_BASE_URL。
//   - bedrock → claude，env 固定 CLAUDE_CODE_USE_BEDROCK=1，Region/AWSProfile 非空分别写
//     AWS_REGION/AWS_PROFILE。
//   - vertex → claude，env 固定 CLAUDE_CODE_USE_VERTEX=1，Region/ProjectID 非空分别写
//     CLOUD_ML_REGION/ANTHROPIC_VERTEX_PROJECT_ID。
//   - open_ai → codex，settings = {"auth":...,"config":...}：key 非空则 auth=
//     {"OPENAI_API_KEY":key}，否则 auth=null；BaseURL 非空则 config 是一段声明
//     model_providers.xkpane-migrated 的 TOML，否则 config=""。
func legacyProviderSettings(row storage.Provider, apiKey string) (appType string, settingsJSON string, err error) {
	switch row.ProviderType {
	case "anthropic", "proxy":
		env := map[string]string{}
		if apiKey != "" {
			env["ANTHROPIC_API_KEY"] = apiKey
		}
		if row.BaseURL != "" {
			env["ANTHROPIC_BASE_URL"] = row.BaseURL
		}
		data, err := json.Marshal(map[string]any{"env": env})
		if err != nil {
			return "", "", err
		}
		return appTypeClaude, string(data), nil

	case "bedrock":
		env := map[string]string{"CLAUDE_CODE_USE_BEDROCK": "1"}
		if row.Region != "" {
			env["AWS_REGION"] = row.Region
		}
		if row.AWSProfile != "" {
			env["AWS_PROFILE"] = row.AWSProfile
		}
		data, err := json.Marshal(map[string]any{"env": env})
		if err != nil {
			return "", "", err
		}
		return appTypeClaude, string(data), nil

	case "vertex":
		env := map[string]string{"CLAUDE_CODE_USE_VERTEX": "1"}
		if row.Region != "" {
			env["CLOUD_ML_REGION"] = row.Region
		}
		if row.ProjectID != "" {
			env["ANTHROPIC_VERTEX_PROJECT_ID"] = row.ProjectID
		}
		data, err := json.Marshal(map[string]any{"env": env})
		if err != nil {
			return "", "", err
		}
		return appTypeClaude, string(data), nil

	case "open_ai":
		var auth map[string]any
		if apiKey != "" {
			auth = map[string]any{"OPENAI_API_KEY": apiKey}
		}
		cfg := ""
		if row.BaseURL != "" {
			cfg = fmt.Sprintf(
				"model_provider = \"xkpane-migrated\"\n\n[model_providers.xkpane-migrated]\nname = \"xkpane-migrated\"\nbase_url = %q\nwire_api = \"responses\"\n",
				row.BaseURL,
			)
		}
		data, err := json.Marshal(codexSettings{Auth: auth, Config: cfg})
		if err != nil {
			return "", "", err
		}
		return appTypeCodex, string(data), nil

	default:
		return "", "", fmt.Errorf("未知的旧 provider 类型 %q", row.ProviderType)
	}
}
