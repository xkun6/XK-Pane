// Package provider 管理 CLI Agent 的接入凭据/配置（providers 表）的 CRUD。
//
// 移植 cc-pane ProviderService 的 MVP 子集。**关键偏离（均为安全/一致性加固，
// 已与 D8 铁律对齐；用户可否决）**：
//  1. 持久化：cc-pane 存明文 providers.json 文件；本实现按 D8 落 SQLite，APIKey
//     经 cryptox 加密（service 层加解密，非 GORM 钩子——读返回脱敏 DTO 无需解密、
//     写为 patch 保留）。
//  2. 脱敏：List/Get 返回 ProviderDTO **不含明文 apiKey**（仅 hasApiKey 标志）——
//     明文密钥**绝不跨 IPC 回前端**。cc-pane 把明文整条回前端（每次 list 都泄漏）。
//  3. Update 为 patch：apiKey 输入为空则**保留**原密钥（cc-pane 是整条覆盖，空 key
//     会抹掉）；且 Update **不动 is_default**（默认标志只由 Add/SetDefault/Remove 管理，
//     避免编辑时误清默认）。
//  4. id 服务端生成 UUID（与 project service 一致；cc-pane 由前端供 id 无去重，易撞）。
//  5. SetDefault 未知 id 报错（cc-pane 静默留零默认）。
//
// GetEnvVars（provider→启动 env 的 to_env_vars 中央映射）与 ToCliProvider 桥接**不在
// 本单元**——它们暴露明文 env、绝不能进前端 bindings，留到「启动接线」单元做成非绑定 helper。
package provider

import (
	"context"
	"errors"
	"fmt"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errNotFound = errors.New("Provider does not exist")

type Service struct {
	holder *storage.Holder
}

func New(holder *storage.Holder) *Service {
	return &Service{holder: holder}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// ProviderDTO 是前端可见的脱敏视图：**不含明文 apiKey**，仅以 hasApiKey 表示是否已设。
type ProviderDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderType string `json:"providerType"`
	HasAPIKey    bool   `json:"hasApiKey"`
	BaseURL      string `json:"baseUrl"`
	Region       string `json:"region"`
	ProjectID    string `json:"projectId"`
	AWSProfile   string `json:"awsProfile"`
	ConfigDir    string `json:"configDir"`
	IsDefault    bool   `json:"isDefault"`
}

// ProviderInput 是前端提交的可写字段。APIKey 为**明文**：Add 时为空=无 key；
// Update 时为空=保留原 key（patch）。IsDefault 仅 Add 时生效（Update 不动它）。
type ProviderInput struct {
	Name         string `json:"name"`
	ProviderType string `json:"providerType"`
	APIKey       string `json:"apiKey"`
	BaseURL      string `json:"baseUrl"`
	Region       string `json:"region"`
	ProjectID    string `json:"projectId"`
	AWSProfile   string `json:"awsProfile"`
	ConfigDir    string `json:"configDir"`
	IsDefault    bool   `json:"isDefault"`
}

func toDTO(p storage.Provider) ProviderDTO {
	return ProviderDTO{
		ID:           p.ID,
		Name:         p.Name,
		ProviderType: p.ProviderType,
		HasAPIKey:    p.APIKey != "", // APIKey 恒为密文/空；非空即已设
		BaseURL:      p.BaseURL,
		Region:       p.Region,
		ProjectID:    p.ProjectID,
		AWSProfile:   p.AWSProfile,
		ConfigDir:    p.ConfigDir,
		IsDefault:    p.IsDefault,
	}
}

func providerTypeOrDefault(t string) string {
	if t == "" {
		return "anthropic"
	}
	return t
}

// validProviderTypes 是 Add/Update 允许写入的 provider 类型白名单——本服务现只管理「非
// claude/codex 的 CLI 类型」，claude/codex 已迁移到 agentcfg（cc-switch 式全局配置 Provider，
// 见 internal/services/agentcfg）。历史类型 anthropic/proxy/bedrock/vertex/open_ai 已一次性
// 迁移过去并从本表删除（agentcfg.migrateLegacyProviders）；config_profile 无法用「整份 live
// 配置文件」的全局配置模型表达，旧行原地保留但永久休眠（List 不再返回，仅 Get/Remove/SetDefault
// 仍可操作，避免僵尸行永远删不掉）。
var validProviderTypes = map[string]bool{
	"gemini":   true,
	"kimi":     true,
	"glm":      true,
	"cursor":   true,
	"opencode": true,
}

// validateProviderType 拒绝白名单外类型，给出明确中文错误引导用户改用正确入口。
func validateProviderType(t string) error {
	if validProviderTypes[t] {
		return nil
	}
	return fmt.Errorf("provider 类型 %q 已不支持新增/修改：Claude/Codex 请使用设置页的全局配置 Provider", t)
}

// List 返回全部 provider（脱敏），按插入序（SQLite 隐式 rowid）——确定性，且与
// cc-pane providers[] 数组（Vec 插入序）一致。不用 created_at：同 tick 多次 Add 的
// 时间戳会撞，字典序非确定（同 storage UTC 修复处的教训）。空→非 nil []。
// 只返回 validProviderTypes 白名单内的行——历史 claude/codex 类型已迁移删除、config_profile
// 永久休眠，均不出现在这里（但 Get/Remove/SetDefault 仍可直接按 id 操作这些行）。
func (s *Service) List(ctx context.Context) ([]ProviderDTO, error) {
	var rows []storage.Provider
	if err := s.db().WithContext(ctx).Order("rowid").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ProviderDTO, 0, len(rows))
	for _, p := range rows {
		if !validProviderTypes[p.ProviderType] {
			continue // 休眠类型（历史 claude/codex 类型已迁移删除；config_profile 永久休眠）对 List 不可见
		}
		out = append(out, toDTO(p))
	}
	return out, nil
}

// Get 按 id 查（脱敏）。不存在返回 (nil, nil)，与 project.Get 一致。
func (s *Service) Get(ctx context.Context, id string) (*ProviderDTO, error) {
	var p storage.Provider
	err := s.db().WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dto := toDTO(p)
	return &dto, nil
}

// Add 新增一个 provider（服务端生成 id），返回脱敏记录。默认标志不变式：
// 表空→强制为默认；输入 isDefault=true→先清其它默认。apiKey 明文经 cryptox 加密落库。
func (s *Service) Add(ctx context.Context, in ProviderInput) (ProviderDTO, error) {
	providerType := providerTypeOrDefault(in.ProviderType)
	if err := validateProviderType(providerType); err != nil {
		return ProviderDTO{}, err
	}
	enc, err := cryptox.EncryptString(in.APIKey)
	if err != nil {
		return ProviderDTO{}, err
	}
	p := storage.Provider{
		ID:           uuid.NewString(),
		Name:         in.Name,
		ProviderType: providerType,
		APIKey:       enc,
		BaseURL:      in.BaseURL,
		Region:       in.Region,
		ProjectID:    in.ProjectID,
		AWSProfile:   in.AWSProfile,
		ConfigDir:    in.ConfigDir,
		IsDefault:    in.IsDefault,
	}
	err = s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Provider{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			p.IsDefault = true // 首个 provider 强制默认
		} else if p.IsDefault {
			if err := tx.Model(&storage.Provider{}).Where("is_default = ?", true).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&p).Error
	})
	if err != nil {
		return ProviderDTO{}, err
	}
	return toDTO(p), nil
}

// Update 修改一个 provider（patch）。apiKey 为空则保留原密钥；不动 is_default。
// 无匹配行报错。
func (s *Service) Update(ctx context.Context, id string, in ProviderInput) error {
	providerType := providerTypeOrDefault(in.ProviderType)
	if err := validateProviderType(providerType); err != nil {
		return err
	}
	updates := map[string]any{
		"name":          in.Name,
		"provider_type": providerType,
		"base_url":      in.BaseURL,
		"region":        in.Region,
		"project_id":    in.ProjectID,
		"aws_profile":   in.AWSProfile,
		"config_dir":    in.ConfigDir,
	}
	if in.APIKey != "" { // 仅在显式给了新 key 时才覆盖（空=保留原密钥）
		enc, err := cryptox.EncryptString(in.APIKey)
		if err != nil {
			return err
		}
		updates["api_key"] = enc
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Provider{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotFound
		}
		return tx.Model(&storage.Provider{}).Where("id = ?", id).Updates(updates).Error
	})
}

// Remove 删除 provider。若删的是默认且仍有剩余→提升最早的剩余为默认。无匹配行报错。
func (s *Service) Remove(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p storage.Provider
		err := tx.First(&p, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if err := tx.Delete(&storage.Provider{}, "id = ?", id).Error; err != nil {
			return err
		}
		if p.IsDefault {
			// 提升最早插入的剩余 provider 为默认（rowid=插入序，确定性；
			// 等价 cc-pane 的 providers[0]。created_at 同 tick 会撞，不可用）。
			var next storage.Provider
			err := tx.Order("rowid").First(&next).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 无剩余，无需提升
			}
			if err != nil {
				return err
			}
			return tx.Model(&storage.Provider{}).Where("id = ?", next.ID).
				Update("is_default", true).Error
		}
		return nil
	})
}

// SetDefault 把某 provider 设为唯一默认。未知 id 报错（偏离 cc-pane 的静默留零默认）。
func (s *Service) SetDefault(ctx context.Context, id string) error {
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Provider{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errNotFound
		}
		if err := tx.Model(&storage.Provider{}).Where("is_default = ?", true).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&storage.Provider{}).Where("id = ?", id).
			Update("is_default", true).Error
	})
}
