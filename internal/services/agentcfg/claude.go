package agentcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/filex"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// readClaudeLive 读并解析 Claude 全局配置文件。文件不存在返回可用 errors.Is(err,
// os.ErrNotExist) 区分的错误；JSON 解析失败返回错误。
func (s *Service) readClaudeLive() (map[string]any, []byte, error) {
	path := s.claudeSettingsPath()
	data, err := filex.ReadLimit(path, 8<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("agentcfg: 读取 %s 失败: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf("agentcfg: 解析 %s 失败: %w", path, err)
	}
	return m, data, nil
}

// writeClaudeLive 把 settingsJSON（明文完整配置）规范化后原子写入 Claude 全局配置
// 文件，写前做一次性备份（<path>.xkpane-bak，仅在其不存在时生成一次）。
func (s *Service) writeClaudeLive(settingsJSON string) error {
	var m map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &m); err != nil {
		return fmt.Errorf("agentcfg: settingsJson 不是合法 JSON 对象: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := s.claudeSettingsPath()
	s.noteOwnWrite()
	if err := backupLiveOnce(path); err != nil {
		return fmt.Errorf("agentcfg: 备份 live 配置失败: %w", err)
	}
	if err := filex.WriteAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("agentcfg: 写入 live 配置失败: %w", err)
	}
	return nil
}

// backupLiveOnce 若 live 文件存在且备份文件尚不存在，把原文件字节原样备份一份。
// 已有备份 / live 尚不存在 → no-op。供 claude.go / codex.go 共用（第二单元泛化自
// 原先仅 claude 专属的 backupClaudeLiveOnce）。
func backupLiveOnce(path string) error {
	bakPath := path + ".xkpane-bak"
	if _, err := os.Stat(bakPath); err == nil {
		return nil // 已备份过
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	orig, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // live 尚不存在，无需备份
		}
		return err
	}
	return filex.WriteAtomic(bakPath, orig, 0o600)
}

// switchClaude 是 SwitchAgentProvider 的 claude 分支：
//  1. 事务内回填当前 IsCurrent 行（若存在且 live 可读合法，把 live 规范化内容加密写回其
//     SettingsEnc；无论是否可读都清掉其 IsCurrent，保证单一 current 不变式）——但仅当
//     settings.json 归属 claude 时才回填内容，否则 live 此刻是 reclaude 占用期的内容，
//     回填会把它串进 claude 的旧 Provider（只清 IsCurrent，不碰内容）；
//  2. 目标行 IsCurrent=true；
//  3. 事务内最后一步把目标配置写入 live —— 写失败则整个事务回滚（DB 状态不变，
//     live 未写成功也未损坏，因为 filex.WriteAtomic 本身是原子的）；
//  4. 事务成功后把 owner 标记为 claude（必须在事务外：SQLite 写锁下事务内另开连接写会超时）。
func (s *Service) switchClaude(ctx context.Context, target storage.AgentProvider) error {
	targetPlain, err := cryptox.DecryptString(target.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密目标 provider 配置失败: %w", err)
	}
	ownerIsClaude := s.getClaudeSettingsOwner(ctx) == appTypeClaude
	err = s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current storage.AgentProvider
		err := tx.Where("app_type = ? AND is_current = ?", appTypeClaude, true).First(&current).Error
		switch {
		case err == nil:
			if ownerIsClaude {
				if err := s.backfillCurrent(tx, current); err != nil {
					return err
				}
			} else if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", current.ID).
				Update("is_current", false).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 无当前行（异常场景，正常应有 importClaudeDefault 兜底），跳过回填
		default:
			return err
		}

		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", target.ID).
			Update("is_current", true).Error; err != nil {
			return err
		}

		return s.writeClaudeLive(targetPlain)
	})
	if err != nil {
		return err
	}
	return s.setClaudeSettingsOwner(ctx, appTypeClaude)
}

// switchReclaude 是 SwitchAgentProvider 的 reclaude 分支：reclaude 是 DB 权威，切换只翻转
// IsCurrent 指针，不写 live、不回填、不碰 owner（配置在启动 reclaude 会话时才落地，见
// ApplyForLaunch）。
func (s *Service) switchReclaude(ctx context.Context, target storage.AgentProvider) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&storage.AgentProvider{}).
			Where("app_type = ? AND is_current = ?", appTypeReclaude, true).
			Update("is_current", false).Error; err != nil {
			return err
		}
		return tx.Model(&storage.AgentProvider{}).Where("id = ?", target.ID).
			Update("is_current", true).Error
	})
}

// backfillCurrent 把 live 文件当前内容（规范化后）加密写回旧 current 行，并清掉它的
// IsCurrent 标志。live 不可读 / 不是合法 JSON 时跳过内容回填（避免用垃圾覆盖旧配置），
// 但仍清掉 IsCurrent —— 否则切换后会出现两行同时 IsCurrent=true。
func (s *Service) backfillCurrent(tx *gorm.DB, current storage.AgentProvider) error {
	updates := map[string]any{"is_current": false}
	if liveMap, _, err := s.readClaudeLive(); err == nil {
		normalized, marshalErr := json.Marshal(liveMap)
		if marshalErr != nil {
			return marshalErr
		}
		enc, encErr := cryptox.EncryptString(string(normalized))
		if encErr != nil {
			return encErr
		}
		updates["settings_enc"] = enc
	}
	return tx.Model(&storage.AgentProvider{}).Where("id = ?", current.ID).Updates(updates).Error
}

// importClaudeDefault 是 Startup 与 ImportCurrentAgentConfig 共用的导入逻辑。
// live 可读且合法 → 用其内容；不可读/不存在 → 用 "{}"。均命名「默认配置」、
// Category=imported、IsCurrent=true —— 除非 manual=true 且已存在同 appType 行
// （手动「从当前 live 导入」不抢当前），此时命名「导入配置 <时间>」且 IsCurrent=false。
func (s *Service) importClaudeDefault(ctx context.Context, manual bool) (*AgentProviderDTO, error) {
	settingsJSON := "{}"
	if liveMap, _, err := s.readClaudeLive(); err == nil {
		normalized, marshalErr := json.Marshal(liveMap)
		if marshalErr != nil {
			return nil, marshalErr
		}
		settingsJSON = string(normalized)
	}

	enc, err := cryptox.EncryptString(settingsJSON)
	if err != nil {
		return nil, err
	}

	var existing int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeClaude).Count(&existing).Error; err != nil {
		return nil, err
	}

	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appTypeClaude,
		SettingsEnc: enc,
		Category:    "imported",
	}
	if manual && existing > 0 {
		row.Name = fmt.Sprintf("导入配置 %s", time.Now().Format("2006-01-02 15:04:05"))
		row.IsCurrent = false
	} else {
		row.Name = "默认配置"
		row.IsCurrent = true
	}

	if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	dto, err := s.toDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// importReclaudeDefault 是 reclaude 版的 importClaudeDefault：reclaude 没有独立 live 文件，
// 默认配置拷自 claude 的 live（readClaudeLive），读不到则用 "{}"。命名/Category/IsCurrent
// 逻辑与 importClaudeDefault 完全一致（manual&&existing>0 时命名「导入配置 <时间>」且
// IsCurrent=false，否则「默认配置」且 IsCurrent=true）。
func (s *Service) importReclaudeDefault(ctx context.Context, manual bool) (*AgentProviderDTO, error) {
	settingsJSON := "{}"
	if liveMap, _, err := s.readClaudeLive(); err == nil {
		normalized, marshalErr := json.Marshal(liveMap)
		if marshalErr != nil {
			return nil, marshalErr
		}
		settingsJSON = string(normalized)
	}

	enc, err := cryptox.EncryptString(settingsJSON)
	if err != nil {
		return nil, err
	}

	var existing int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeReclaude).Count(&existing).Error; err != nil {
		return nil, err
	}

	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appTypeReclaude,
		SettingsEnc: enc,
		Category:    "imported",
	}
	if manual && existing > 0 {
		row.Name = fmt.Sprintf("导入配置 %s", time.Now().Format("2006-01-02 15:04:05"))
		row.IsCurrent = false
	} else {
		row.Name = "默认配置"
		row.IsCurrent = true
	}

	if err := s.db().WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	dto, err := s.toDTO(row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// claudeSettingsOwnerKey 是 AgentCfgKV 里记录「~/.claude/settings.json 当前归属谁」的键，
// 值 ∈ {claude, reclaude}。
const claudeSettingsOwnerKey = "claude_settings_owner"

// getClaudeSettingsOwner 返回 ~/.claude/settings.json 当前归属的 appType（claude|reclaude）。
// 键缺失/空 → 缺省 claude（今日行为：文件默认视为 claude 的）。
func (s *Service) getClaudeSettingsOwner(ctx context.Context) string {
	var kv storage.AgentCfgKV
	if err := s.db().WithContext(ctx).First(&kv, "key = ?", claudeSettingsOwnerKey).Error; err != nil || kv.Value == "" {
		return appTypeClaude
	}
	return kv.Value
}

// setClaudeSettingsOwner 把 owner 标记写为 appType（claude|reclaude）。
func (s *Service) setClaudeSettingsOwner(ctx context.Context, appType string) error {
	return s.db().WithContext(ctx).Save(&storage.AgentCfgKV{Key: claudeSettingsOwnerKey, Value: appType}).Error
}

// ApplyForLaunch 在 terminal 启动 claude/reclaude 会话前调用：把该 appType 的当前 Provider
// 配置写入共享的 ~/.claude/settings.json，并把文件归属标记为该 appType。无当前 Provider →
// no-op（回退今日行为，不动文件）。applyMu 串行化，避免两次启动交叉写。
func (s *Service) ApplyForLaunch(ctx context.Context, appType string) error {
	if appType != appTypeClaude && appType != appTypeReclaude {
		return nil
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	var cur storage.AgentProvider
	err := s.db().WithContext(ctx).Where("app_type = ? AND is_current = ?", appType, true).First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	plain, err := cryptox.DecryptString(cur.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密 %s 当前配置失败: %w", appType, err)
	}
	if err := s.writeClaudeLive(plain); err != nil {
		return err
	}
	return s.setClaudeSettingsOwner(ctx, appType)
}
