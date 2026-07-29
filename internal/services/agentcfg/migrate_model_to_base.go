package agentcfg

import (
	"context"
	"encoding/json"
	"errors"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// claudeModelToBaseMigratedKey 是「model 归位公共配置」一次性修复的幂等标记键(AgentCfgKV)。
const claudeModelToBaseMigratedKey = "claude_model_to_base_migrated"

// migrateClaudeModelToBase 把历史上被误塞进 claude 供应商补丁的 model 挪回系统配置基底。
//
// 起因：早期 claudeCredLeafPaths 把 "model" 当凭据路径，claude CLI 自己改写 settings.json
// 的 model(/model 切模型、opus[1m])会被反向同步拆进当前供应商补丁，症状是供应商配置被
// 替换成 {"model":"opus[1m]"}。又因 credPathSet 含「补丁已声明路径」，model 一旦进补丁
// 就再也出不来(棘轮)，故光改逻辑不够，必须做一次数据归位。
//
// 处理：剥离所有 claude 供应商补丁里的 model；若基底尚无 model，用「当前供应商补丁的
// model」(无当前则取首个非空)补进基底，保证 live 投影后模型选择不丢失。
// 幂等：置位 KV 后直接返回；单事务(改补丁 + 写基底 + 置标记)。失败只记日志、不阻断启动。
func (s *Service) migrateClaudeModelToBase(ctx context.Context) error {
	var kv storage.AgentCfgKV
	err := s.db().WithContext(ctx).First(&kv, "key = ?", claudeModelToBaseMigratedKey).Error
	if err == nil && kv.Value == "1" {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []storage.AgentProvider
		if err := tx.Where("app_type = ?", appTypeClaude).
			Order("is_current desc, sort_order, created_at").Find(&rows).Error; err != nil {
			return err
		}

		var rescued any
		for i := range rows {
			plain, decErr := cryptox.DecryptString(rows[i].SettingsEnc)
			if decErr != nil {
				continue
			}
			var patch map[string]any
			if jErr := json.Unmarshal([]byte(plain), &patch); jErr != nil || patch == nil {
				continue
			}
			modelVal, hasModel := patch["model"]
			if !hasModel {
				continue
			}
			if rescued == nil {
				rescued = modelVal
			}
			delete(patch, "model")
			data, mErr := json.Marshal(patch)
			if mErr != nil {
				return mErr
			}
			enc, encErr := cryptox.EncryptString(string(data))
			if encErr != nil {
				return encErr
			}
			if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", rows[i].ID).
				Update("settings_enc", enc).Error; err != nil {
				return err
			}
		}

		if rescued != nil {
			base, err := s.readBaseConfig(tx, appTypeClaude)
			if err != nil {
				return err
			}
			if _, hasModel := base["model"]; !hasModel {
				base["model"] = rescued
				if err := s.writeBaseConfig(tx, appTypeClaude, base); err != nil {
					return err
				}
			}
		}

		return tx.Save(&storage.AgentCfgKV{Key: claudeModelToBaseMigratedKey, Value: "1"}).Error
	})
}
