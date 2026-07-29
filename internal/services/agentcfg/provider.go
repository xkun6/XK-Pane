package agentcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	appTypeClaude = "claude"
	appTypeCodex  = "codex"
	appTypeGrok   = "grok"
)

var errNotFound = errors.New("agentcfg: provider does not exist")

// AgentProviderDTO 是前端可见视图：SettingsJSON 是解密后的**明文**完整配置 JSON。
// 仅供桌面本地 IPC 使用（Wails binding 走进程内调用，无网络暴露）；绝不可挂到
// internal/webapi 任何路由——那条路径面向浏览器/局域网，明文密钥不能过去。
type AgentProviderDTO struct {
	ID           string    `json:"id"`
	AppType      string    `json:"appType"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	IsCurrent    bool      `json:"isCurrent"`
	SortOrder    int       `json:"sortOrder"`
	Notes        string    `json:"notes"`
	WebsiteURL   string    `json:"websiteUrl"`
	SettingsJSON string    `json:"settingsJson"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AgentProviderInput 是前端提交的可写字段。AppType 仅 Add 时生效（Update 不允许改）。
type AgentProviderInput struct {
	AppType      string `json:"appType"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Notes        string `json:"notes"`
	WebsiteURL   string `json:"websiteUrl"`
	SettingsJSON string `json:"settingsJson"`
}

func (s *Service) toDTO(row storage.AgentProvider) (AgentProviderDTO, error) {
	plain, err := cryptox.DecryptString(row.SettingsEnc)
	if err != nil {
		return AgentProviderDTO{}, fmt.Errorf("agentcfg: 解密 provider %s 失败: %w", row.ID, err)
	}
	return AgentProviderDTO{
		ID:           row.ID,
		AppType:      row.AppType,
		Name:         row.Name,
		Category:     row.Category,
		IsCurrent:    row.IsCurrent,
		SortOrder:    row.SortOrder,
		Notes:        row.Notes,
		WebsiteURL:   row.WebsiteURL,
		SettingsJSON: plain,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func validateAppType(appType string) error {
	if appType != appTypeClaude && appType != appTypeCodex && appType != appTypeGrok {
		return fmt.Errorf("agentcfg: 不支持的 appType %q（仅 claude | codex | grok）", appType)
	}
	return nil
}

// validateSettingsJSON 校验是合法 JSON 对象（claude 用：settings.json 形状随意，只要求
// 是对象）。
func validateSettingsJSON(raw string) error {
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("agentcfg: settingsJson 不是合法的 JSON 对象: %w", err)
	}
	return nil
}

// validateProviderSettings 按 appType 分派校验规则：codex 与 grok 都走
// validateCodexSettings（额外校验 auth/config 形状与 config 的 TOML 合法性；grok 与 codex
// 配置同形故直接复用），其余（claude）走 validateSettingsJSON（只要求合法 JSON 对象）。
func validateProviderSettings(appType, raw string) error {
	if appType == appTypeCodex || appType == appTypeGrok {
		return validateCodexSettings(raw)
	}
	return validateSettingsJSON(raw)
}

func categoryOrDefault(c string) string {
	if c == "" {
		return "custom"
	}
	return c
}

// ListAgentProviders 按 appType 列出全部 Provider（含解密后的明文配置），按 SortOrder,
// CreatedAt 排序。空 → 非 nil 空切片。
//
// 列表前会把「当前 Provider」与磁盘 live 全局配置对齐（打开设置页 / 刷新时调用），
// 这样用户在软件外改过 settings.json 等也能在需要时读到最新，而不必在冷启动时同步。
func (s *Service) ListAgentProviders(ctx context.Context, appType string) ([]AgentProviderDTO, error) {
	if _, err := s.syncCurrentFromLive(ctx, appType); err != nil {
		s.log.Warn("sync current from live before list failed", "appType", appType, "err", err)
		// 同步失败不阻断列表：仍返回 DB 内已有快照
	}
	var rows []storage.AgentProvider
	if err := s.db().WithContext(ctx).Where("app_type = ?", appType).
		Order("sort_order, created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AgentProviderDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := s.toDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

// AddAgentProvider 新增一个 Provider（服务端生成 id），不改动任何其它行的 IsCurrent。
func (s *Service) AddAgentProvider(ctx context.Context, in AgentProviderInput) (*AgentProviderDTO, error) {
	if err := validateAppType(in.AppType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("agentcfg: name 不能为空")
	}
	if err := validateProviderSettings(in.AppType, in.SettingsJSON); err != nil {
		return nil, err
	}
	enc, err := cryptox.EncryptString(in.SettingsJSON)
	if err != nil {
		return nil, err
	}
	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     in.AppType,
		Name:        in.Name,
		SettingsEnc: enc,
		Category:    categoryOrDefault(in.Category),
		Notes:       in.Notes,
		WebsiteURL:  in.WebsiteURL,
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

// UpdateAgentProvider 修改一个 Provider（AppType 不可改）。若该行 IsCurrent，更新后立即
// 把新 SettingsJSON 写入对应 CLI 的 live 配置（写失败回滚整个更新——live 写放事务内最后
// 一步）。
func (s *Service) UpdateAgentProvider(ctx context.Context, id string, in AgentProviderInput) (*AgentProviderDTO, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("agentcfg: name 不能为空")
	}
	// AppType 不可改，但校验 SettingsJSON 的规则依 appType 而定 → 先查一次现有行拿
	// AppType（Update 不允许改 AppType，这里只读不写，不存在并发风险）。
	var existing storage.AgentProvider
	if err := s.db().WithContext(ctx).First(&existing, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errNotFound
		}
		return nil, err
	}
	if err := validateProviderSettings(existing.AppType, in.SettingsJSON); err != nil {
		return nil, err
	}
	enc, err := cryptox.EncryptString(in.SettingsJSON)
	if err != nil {
		return nil, err
	}

	var result storage.AgentProvider
	var wroteClaudeLive bool
	err = s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row storage.AgentProvider
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNotFound
			}
			return err
		}
		updates := map[string]any{
			"name":         in.Name,
			"settings_enc": enc,
			"category":     categoryOrDefault(in.Category),
			"notes":        in.Notes,
			"website_url":  in.WebsiteURL,
		}
		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		result = row
		if row.IsCurrent {
			switch row.AppType {
			case appTypeClaude:
				merged, mErr := s.mergedLive(tx, appTypeClaude, in.SettingsJSON)
				if mErr != nil {
					return mErr
				}
				if err := s.writeClaudeLive(merged); err != nil {
					return err
				}
				wroteClaudeLive = true
			case appTypeCodex:
				if err := s.writeCodexLive(ctx, tx, in.SettingsJSON); err != nil {
					return err
				}
			case appTypeGrok:
				if err := s.writeGrokLive(in.SettingsJSON); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if wroteClaudeLive {
		// setClaudeSettingsOwner 必须在事务外调用：SQLite 写锁下事务内另开连接写会超时。
		if err := s.setClaudeSettingsOwner(ctx, appTypeClaude); err != nil {
			return nil, err
		}
	}
	dto, err := s.toDTO(result)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// DeleteAgentProvider 删除一个 Provider。IsCurrent 行拒绝删除。
func (s *Service) DeleteAgentProvider(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row storage.AgentProvider
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNotFound
			}
			return err
		}
		if row.IsCurrent {
			return errors.New("agentcfg: 当前使用中的 Provider 不能删除，请先切换到其它 Provider")
		}
		return tx.Delete(&storage.AgentProvider{}, "id = ?", id).Error
	})
}

// SwitchAgentProvider 把某 Provider 切为当前，并原子性写入对应 CLI 的 live 配置文件。
// 已是 current → no-op。
func (s *Service) SwitchAgentProvider(ctx context.Context, id string) error {
	var target storage.AgentProvider
	if err := s.db().WithContext(ctx).First(&target, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errNotFound
		}
		return err
	}
	if target.IsCurrent {
		return nil
	}
	switch target.AppType {
	case appTypeClaude:
		return s.switchClaude(ctx, target)
	case appTypeCodex:
		return s.switchCodex(ctx, target)
	case appTypeGrok:
		return s.switchGrok(ctx, target)
	default:
		return fmt.Errorf("agentcfg: appType %q 尚未支持切换", target.AppType)
	}
}

// ImportCurrentAgentConfig 是「从当前 live 导入」的手动入口（对齐 Startup 的自动导入）。
func (s *Service) ImportCurrentAgentConfig(ctx context.Context, appType string) (*AgentProviderDTO, error) {
	if err := validateAppType(appType); err != nil {
		return nil, err
	}
	switch appType {
	case appTypeCodex:
		return s.importCodexDefault(ctx, true)
	case appTypeGrok:
		return s.importGrokDefault(ctx, true)
	default:
		return s.importClaudeDefault(ctx, true)
	}
}
