package agentcfg

// Grok Build（xAI CLI）的配置模型与 codex 完全同构：主配置 <home>/.grok/config.toml
// （TOML）+ 登录态 <home>/.grok/auth.json（JSON）。因此本文件直接复用 codex.go 定义的
// codexSettings / parseCodexSettings（形状 {"auth":<对象|null>,"config":"<TOML 全文>"}），
// 不重新定义一套同构类型。
//
// 与 codex 的唯一实质差异：grok 不做 MCP 投影——codex 的 writeCodexLive 会
// stripMcpServers 剥离 config 里的 [mcp_servers] 段再拼 agent_mcp_servers 表投影，本文件
// 一概不做，config 原文整份读写（用户 config.toml 里的 [mcp_servers] 段原样保留、原样
// 快照进 Provider）。因此本文件没有 db/tx 参数，也没有 stripMcpServers /
// renderCodexMcpToml 那套逻辑。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"xkpane/internal/storage"
	"xkpane/internal/utils/cryptox"
	"xkpane/internal/utils/filex"

	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
	"gorm.io/gorm"
)

// writeGrokLive 把 grok Provider 的 settingsJSON 拆成 auth.json + config.toml 写入 live
// 位置。结构与 writeCodexLive 一致，唯独没有 mcp_servers 剥离/投影这一步：
//
//  1. 解析 settings，取 auth / config；
//  2. config 非空时整体 toml 解析校验一遍——校验失败则不触碰任何文件；
//  3. 一次性备份：config.toml 总会写，无条件备份一次；auth.json 只在本次确实要写它
//     （auth != nil）时才备份——「auth=null 表示不管理 auth.json」这条语义要贯彻到底，
//     不该为一个根本不打算碰的文件生成备份；
//  4. auth 先写、config 后写：若 auth 非 null，先读旧 auth.json 字节留作回滚，
//     WriteAtomic 写新 auth.json；再写 config.toml；config 写失败且刚才写过 auth →
//     尽力把旧 auth 字节写回（旧文件本不存在则删除新写的 auth.json），并返回错误，
//     避免登录态和配置文件停在不一致的组合上。auth 为 null → 完全不碰 auth.json。
func (s *Service) writeGrokLive(settingsJSON string) error {
	cs, err := parseCodexSettings(settingsJSON)
	if err != nil {
		return err
	}

	configPath := s.grokConfigPath()
	authPath := s.grokAuthPath()

	if err := backupLiveOnce(configPath); err != nil {
		return fmt.Errorf("agentcfg: 备份 live config.toml 失败: %w", err)
	}
	if cs.Auth != nil {
		if err := backupLiveOnce(authPath); err != nil {
			return fmt.Errorf("agentcfg: 备份 live auth.json 失败: %w", err)
		}
	}

	if strings.TrimSpace(cs.Config) != "" {
		var m map[string]any
		if err := toml.Unmarshal([]byte(cs.Config), &m); err != nil {
			return fmt.Errorf("agentcfg: config 不是合法 TOML: %w", err)
		}
	}

	var authWritten, prevAuthExisted bool
	var prevAuthBytes []byte
	if cs.Auth != nil {
		if data, readErr := os.ReadFile(authPath); readErr == nil {
			prevAuthBytes, prevAuthExisted = data, true
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("agentcfg: 读取旧 live auth.json 失败: %w", readErr)
		}
		authData, marshalErr := json.MarshalIndent(cs.Auth, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if err := filex.WriteAtomic(authPath, authData, 0o600); err != nil {
			return fmt.Errorf("agentcfg: 写入 live auth.json 失败: %w", err)
		}
		authWritten = true
	}

	if err := filex.WriteAtomic(configPath, []byte(cs.Config), 0o600); err != nil {
		if authWritten {
			if prevAuthExisted {
				_ = filex.WriteAtomic(authPath, prevAuthBytes, 0o600)
			} else {
				_ = os.Remove(authPath)
			}
		}
		return fmt.Errorf("agentcfg: 写入 live config.toml 失败: %w", err)
	}
	return nil
}

// readGrokLive 读 config.toml + auth.json，组装成规范化的 grok Provider SettingsJSON，
// 供回填 / 导入共用。与 readCodexLive 的关键差异：不剥离 [mcp_servers] 段——grok 不做
// MCP 投影，config.toml 原文整份就是唯一事实来源，用户手写的 [mcp_servers.xxx] 段原样
// 保留、原样快照进 Provider。config.toml 不存在视为 ""；auth.json 不存在或不是合法 JSON
// 对象 → auth=null（首次导入时保守起见，不擅自把形状不明的旧文件内容当 auth 捕获）。
func (s *Service) readGrokLive() (string, error) {
	config := ""
	data, err := filex.ReadLimit(s.grokConfigPath(), 8<<20)
	switch {
	case err == nil:
		config = string(data)
	case errors.Is(err, os.ErrNotExist):
		// config.toml 不存在，config 保持 ""
	default:
		return "", fmt.Errorf("agentcfg: 读取 %s 失败: %w", s.grokConfigPath(), err)
	}

	var auth map[string]any
	if authData, err := filex.ReadLimit(s.grokAuthPath(), 8<<20); err == nil {
		var m map[string]any
		if json.Unmarshal(authData, &m) == nil {
			auth = m
		}
	}

	out, err := json.Marshal(codexSettings{Auth: auth, Config: config})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// switchGrok 是 SwitchAgentProvider 的 grok 分支，事务结构与 switchCodex 对称（少一步
// MCP 投影）：
//  1. 事务内回填当前 IsCurrent 行（backfillCurrentGrok）；
//  2. 目标行 IsCurrent=true；
//  3. 事务内最后一步把目标配置写入 live —— 写失败则整个事务回滚（DB 状态不变，live 未
//     写成功也未损坏，filex.WriteAtomic 本身是原子的）。
func (s *Service) switchGrok(ctx context.Context, target storage.AgentProvider) error {
	targetPlain, err := cryptox.DecryptString(target.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密目标 provider 配置失败: %w", err)
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current storage.AgentProvider
		err := tx.Where("app_type = ? AND is_current = ?", appTypeGrok, true).First(&current).Error
		switch {
		case err == nil:
			if err := s.backfillCurrentGrok(tx, current); err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 无当前行（异常场景，正常应有 importGrokDefault 兜底），跳过回填
		default:
			return err
		}

		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", target.ID).
			Update("is_current", true).Error; err != nil {
			return err
		}

		return s.writeGrokLive(targetPlain)
	})
}

// backfillCurrentGrok 把 live 当前内容规范化后加密写回旧 current 行，并清掉它的
// IsCurrent 标志。关键点与 backfillCurrentCodex 一致：旧行原 auth 为 null 时只回填
// config、强制保持 auth=null——即便切换前 live auth.json 确实存在内容，也不擅自替它捕获，
// 因为该行明确声明「不管理 auth」，回填不能悄悄改变这条语义。live 不可读时跳过内容回填但
// 仍清 IsCurrent，保证单一 current 不变式。
func (s *Service) backfillCurrentGrok(tx *gorm.DB, current storage.AgentProvider) error {
	updates := map[string]any{"is_current": false}

	currentPlain, err := cryptox.DecryptString(current.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密旧 current provider 配置失败: %w", err)
	}
	currentSettings, err := parseCodexSettings(currentPlain)
	if err != nil {
		return err
	}

	if liveJSON, err := s.readGrokLive(); err == nil {
		if liveSettings, parseErr := parseCodexSettings(liveJSON); parseErr == nil {
			normalized := liveSettings
			if currentSettings.Auth == nil {
				normalized = codexSettings{Auth: nil, Config: liveSettings.Config}
			}
			data, marshalErr := json.Marshal(normalized)
			if marshalErr != nil {
				return marshalErr
			}
			enc, encErr := cryptox.EncryptString(string(data))
			if encErr != nil {
				return encErr
			}
			updates["settings_enc"] = enc
		}
	}
	return tx.Model(&storage.AgentProvider{}).Where("id = ?", current.ID).Updates(updates).Error
}

// importGrokDefault 是 Startup 与 ImportCurrentAgentConfig 共用的导入逻辑，镜像
// importCodexDefault。live 可读 → 用其规范化内容；config.toml/auth.json 都不存在（或
// 读取失败）→ 用 {"auth":null,"config":""}。均命名「默认配置」、Category=imported、
// IsCurrent=true —— 除非 manual=true 且已存在同 appType 行（手动「从当前 live 导入」不
// 抢当前），此时命名「导入配置 <时间>」且 IsCurrent=false。
func (s *Service) importGrokDefault(ctx context.Context, manual bool) (*AgentProviderDTO, error) {
	settingsJSON := `{"auth":null,"config":""}`
	if liveJSON, err := s.readGrokLive(); err == nil {
		settingsJSON = liveJSON
	}

	enc, err := cryptox.EncryptString(settingsJSON)
	if err != nil {
		return nil, err
	}

	var existing int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeGrok).Count(&existing).Error; err != nil {
		return nil, err
	}

	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appTypeGrok,
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
