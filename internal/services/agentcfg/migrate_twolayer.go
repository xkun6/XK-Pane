package agentcfg

import (
	"context"
	"encoding/json"
	"errors"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// claudeTwoLayerMigratedKey 是「两层模型一次性迁移已完成」的幂等标记键(AgentCfgKV)。
const claudeTwoLayerMigratedKey = "claude_twolayer_migrated"

// migrateClaudeTwoLayer 把老用户「每个 claude/reclaude Provider 各存整份 settings.json」的
// 数据一次性迁移成两层模型:取 IsCurrent 行(无则首行)整份作为系统配置基底,各 Provider
// 的补丁 = deepDiff(整份, 基底)。当前供应商补丁收敛为 {} → 切回它 merged==原整份(零丢失);
// 凭据永不丢(diff 保留所有相异叶子)。
//
// 幂等:置位 claude_twolayer_migrated 后直接返回;单事务(建基底 + 改补丁 + 置标记全在一个
// tx),避免「改了补丁但没置标记 → 重试二次 diff 损坏」。失败只记日志、不阻断启动(见调用方)。
//
// 跳过条件:某 appType 的基底行已存在 → 该 appType 无需迁移(全新用户由 importDefaultTwoLayer
// 首启已建基底,本函数对其为 no-op,且绝不覆盖已建基底)。
// 已知边缘限制:若「首次运行本功能」与「旧 providers 表 legacy 迁移」在同一次启动发生,
// import 会先建基底导致本函数跳过、legacy 迁入的整份行不被拆补丁——该行整份仍能正常切换
// (deepMerge(基底, 整份) 由整份主导),只是未享受两层收益;此场景在本功能已运行多版本后极罕见。
func (s *Service) migrateClaudeTwoLayer(ctx context.Context) error {
	var kv storage.AgentCfgKV
	err := s.db().WithContext(ctx).First(&kv, "key = ?", claudeTwoLayerMigratedKey).Error
	if err == nil && kv.Value == "1" {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, appType := range []string{appTypeClaude, appTypeReclaude} {
			var baseCount int64
			if err := tx.Model(&storage.AgentBaseConfig{}).Where("app_type = ?", appType).Count(&baseCount).Error; err != nil {
				return err
			}
			if baseCount > 0 {
				continue // 基底已存在(首启已建)→ 跳过,绝不覆盖
			}

			var rows []storage.AgentProvider
			if err := tx.Where("app_type = ?", appType).
				Order("is_current desc, sort_order, created_at").Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				continue // 无 Provider → 不建基底,留待后续 import
			}

			baseRow := &rows[0]
			for i := range rows {
				if rows[i].IsCurrent {
					baseRow = &rows[i]
					break
				}
			}
			basePlain, err := cryptox.DecryptString(baseRow.SettingsEnc)
			if err != nil {
				return err
			}
			var base map[string]any
			if err := json.Unmarshal([]byte(basePlain), &base); err != nil {
				base = map[string]any{}
			}
			if err := s.writeBaseConfig(tx, appType, base); err != nil {
				return err
			}

			for i := range rows {
				plain, err := cryptox.DecryptString(rows[i].SettingsEnc)
				if err != nil {
					return err
				}
				var full map[string]any
				if err := json.Unmarshal([]byte(plain), &full); err != nil {
					full = map[string]any{}
				}
				data, err := json.Marshal(deepDiffJSON(full, base))
				if err != nil {
					return err
				}
				enc, err := cryptox.EncryptString(string(data))
				if err != nil {
					return err
				}
				if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", rows[i].ID).
					Update("settings_enc", enc).Error; err != nil {
					return err
				}
			}
		}
		return tx.Save(&storage.AgentCfgKV{Key: claudeTwoLayerMigratedKey, Value: "1"}).Error
	})
}
