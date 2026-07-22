package agentcfg

import (
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
