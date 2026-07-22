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
//     SettingsEnc；无论是否可读都清掉其 IsCurrent，保证单一 current 不变式）；
//  2. 目标行 IsCurrent=true；
//  3. 事务内最后一步把目标配置写入 live —— 写失败则整个事务回滚（DB 状态不变，
//     live 未写成功也未损坏，因为 filex.WriteAtomic 本身是原子的）。
func (s *Service) switchClaude(ctx context.Context, target storage.AgentProvider) error {
	targetPlain, err := cryptox.DecryptString(target.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密目标 provider 配置失败: %w", err)
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current storage.AgentProvider
		err := tx.Where("app_type = ? AND is_current = ?", appTypeClaude, true).First(&current).Error
		switch {
		case err == nil:
			if err := s.backfillCurrent(tx, current); err != nil {
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
