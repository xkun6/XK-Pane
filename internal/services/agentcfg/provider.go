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
	if appType != appTypeClaude && appType != appTypeCodex {
		return fmt.Errorf("agentcfg: 不支持的 appType %q（仅 claude | codex）", appType)
	}
	return nil
}

// validateSettingsJSON 校验是合法 JSON 对象。codex 的形状（{"auth":...,"config":"..."}）
// 也是对象，本单元不做更细的 codex 形状校验（留后续单元）。
func validateSettingsJSON(raw string) error {
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("agentcfg: settingsJson 不是合法的 JSON 对象: %w", err)
	}
	return nil
}

func categoryOrDefault(c string) string {
	if c == "" {
		return "custom"
	}
	return c
}

// ListAgentProviders 按 appType 列出全部 Provider（含解密后的明文配置），按 SortOrder,
// CreatedAt 排序。空 → 非 nil 空切片。
func (s *Service) ListAgentProviders(ctx context.Context, appType string) ([]AgentProviderDTO, error) {
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
	if err := validateSettingsJSON(in.SettingsJSON); err != nil {
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

// UpdateAgentProvider 修改一个 Provider（AppType 不可改）。若该行是 claude 且 IsCurrent，
// 更新后立即把新 SettingsJSON 写入 live（写失败回滚整个更新——live 写放事务内最后一步）。
func (s *Service) UpdateAgentProvider(ctx context.Context, id string, in AgentProviderInput) (*AgentProviderDTO, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("agentcfg: name 不能为空")
	}
	if err := validateSettingsJSON(in.SettingsJSON); err != nil {
		return nil, err
	}
	enc, err := cryptox.EncryptString(in.SettingsJSON)
	if err != nil {
		return nil, err
	}

	var result storage.AgentProvider
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
		if row.IsCurrent && row.AppType == appTypeClaude {
			if err := s.writeClaudeLive(in.SettingsJSON); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	default:
		return fmt.Errorf("agentcfg: appType %q 尚未支持切换", target.AppType)
	}
}

// ImportCurrentAgentConfig 是「从当前 live 导入」的手动入口（对齐 Startup 的自动导入）。
// codex 本单元尚未支持。
func (s *Service) ImportCurrentAgentConfig(ctx context.Context, appType string) (*AgentProviderDTO, error) {
	if err := validateAppType(appType); err != nil {
		return nil, err
	}
	if appType == appTypeCodex {
		return nil, errors.New("agentcfg: codex 尚未支持")
	}
	return s.importClaudeDefault(ctx, true)
}
