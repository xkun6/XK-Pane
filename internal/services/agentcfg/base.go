package agentcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"gorm.io/gorm"
)

// readBaseConfig 读指定 appType 的系统配置基底并解密为 map。无行 → 返回空 map(非错误:
// 基底缺失时 deepMergeJSON({}, 补丁)==补丁,保证迁移前行为等价)。db 由调用方传入,便于复用
// s.db() 或事务 tx(本包 holder 连接池 MaxOpenConns=1,事务内另开连接会自我死锁)。
func (s *Service) readBaseConfig(db *gorm.DB, appType string) (map[string]any, error) {
	var row storage.AgentBaseConfig
	err := db.First(&row, "app_type = ?", appType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	plain, err := cryptox.DecryptString(row.SettingsEnc)
	if err != nil {
		return nil, fmt.Errorf("agentcfg: 解密 %s 基底配置失败: %w", appType, err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(plain), &m); err != nil {
		return nil, fmt.Errorf("agentcfg: 解析 %s 基底配置失败: %w", appType, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// writeBaseConfig 把基底 map 加密后 upsert 到 agent_base_configs(按 appType 主键)。
// 用「先查再 Create/Update」而非 Save,避免 Save 把已存在行的 CreatedAt 覆盖成零值。
func (s *Service) writeBaseConfig(db *gorm.DB, appType string, m map[string]any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	enc, err := cryptox.EncryptString(string(data))
	if err != nil {
		return err
	}
	var existing storage.AgentBaseConfig
	err = db.First(&existing, "app_type = ?", appType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&storage.AgentBaseConfig{AppType: appType, SettingsEnc: enc}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&storage.AgentBaseConfig{}).Where("app_type = ?", appType).Update("settings_enc", enc).Error
}

// GetBaseConfig 返回指定 appType 的系统配置基底明文 JSON(仅 claude);无基底行返回 "{}"。
// 仅桌面本地 IPC(Wails binding 进程内调用),绝不可挂到 internal/webapi 任何路由。
func (s *Service) GetBaseConfig(ctx context.Context, appType string) (string, error) {
	if appType != appTypeClaude {
		return "", fmt.Errorf("agentcfg: GetBaseConfig 仅支持 claude,got %q", appType)
	}
	base, err := s.readBaseConfig(s.db().WithContext(ctx), appTypeClaude)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(base)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SetBaseConfig 更新指定 appType 的系统配置基底。校验为合法 JSON 对象后加密 upsert;
// 仅 claude：存在 current Provider 时事务内重投影 live(deepMerge(新基底, 当前补丁))、
// 事务外 setClaudeSettingsOwner(claude)。语义与 UpdateAgentProvider 的 claude 分支一致。
func (s *Service) SetBaseConfig(ctx context.Context, appType, settingsJSON string) error {
	if appType != appTypeClaude {
		return fmt.Errorf("agentcfg: SetBaseConfig 仅支持 claude,got %q", appType)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &m); err != nil {
		return fmt.Errorf("agentcfg: 系统配置不是合法 JSON 对象: %w", err)
	}
	var wroteLive bool
	err := s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.writeBaseConfig(tx, appTypeClaude, m); err != nil {
			return err
		}
		var cur storage.AgentProvider
		qerr := tx.Where("app_type = ? AND is_current = ?", appTypeClaude, true).First(&cur).Error
		if errors.Is(qerr, gorm.ErrRecordNotFound) {
			return nil // 无 current → 不写 live
		}
		if qerr != nil {
			return qerr
		}
		patch, err := cryptox.DecryptString(cur.SettingsEnc)
		if err != nil {
			return err
		}
		merged, err := s.mergedLive(tx, appTypeClaude, patch)
		if err != nil {
			return err
		}
		if err := s.writeClaudeLive(merged); err != nil {
			return err
		}
		wroteLive = true
		return nil
	})
	if err != nil {
		return err
	}
	if wroteLive {
		return s.setClaudeSettingsOwner(ctx, appTypeClaude)
	}
	return nil
}
