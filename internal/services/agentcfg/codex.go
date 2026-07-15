package agentcfg

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

// codexSettings 是 codex Provider SettingsJSON 反序列化后的结构：
//
//	{ "auth": <auth.json 的 JSON 对象 | null>, "config": "<config.toml 全文字符串>" }
//
// Auth 为 nil 表示该 Provider 不管理 auth.json —— 切换到它时完全不碰 auth.json，
// 保护当前的 ChatGPT 登录态。Config 是要写入 config.toml 的正文，不含 [mcp_servers]，
// 那部分由 agent_mcp_servers 表投影追加（见 renderCodexMcpToml）。
type codexSettings struct {
	Auth   map[string]any `json:"auth"`
	Config string         `json:"config"`
}

// parseCodexSettings 把 settingsJSON 解析成 codexSettings。依赖 json.Unmarshal 的严格
// 类型检查完成大半校验：顶层不是对象 / auth 存在但不是对象或 null / config 存在但不是
// 字符串，均会在这里报错，不必手写逐字段判断。
func parseCodexSettings(settingsJSON string) (codexSettings, error) {
	var cs codexSettings
	if err := json.Unmarshal([]byte(settingsJSON), &cs); err != nil {
		return codexSettings{}, fmt.Errorf("agentcfg: settingsJson 不是合法的 codex 配置对象: %w", err)
	}
	return cs, nil
}

// validateCodexSettings 校验 codex 专属 SettingsJSON 形状：
// {"auth":<对象|null>,"config":"<TOML 文本>"}。auth 缺省视为 null；存在时必须是对象或
// null。config 缺省视为空字符串；存在时必须是字符串，且非空时必须能被 toml.Unmarshal
// 解析——防止把非法 TOML 存进 Provider，等到切换时才炸。
func validateCodexSettings(settingsJSON string) error {
	cs, err := parseCodexSettings(settingsJSON)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cs.Config) != "" {
		var m map[string]any
		if err := toml.Unmarshal([]byte(cs.Config), &m); err != nil {
			return fmt.Errorf("agentcfg: config 不是合法 TOML: %w", err)
		}
	}
	return nil
}

// parseTableHeader 判断一行（已 TrimSpace）是否是 TOML 表头（[table] 或 [[table]]），
// 是则返回表名（不含方括号 / 前后空白）。为避免把「以 [ 开头的普通值行」（例如多行数组
// 字面量里以 [ 起始的续行）误判成表头，要求闭合括号之后只能跟空白或注释——真正的表头行
// 语法上不允许别的内容。
func parseTableHeader(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	isArray := strings.HasPrefix(trimmed, "[[")
	rest := trimmed[1:]
	closer := "]"
	if isArray {
		rest = trimmed[2:]
		closer = "]]"
	}
	idx := strings.Index(rest, closer)
	if idx < 0 {
		return "", false
	}
	name := strings.TrimSpace(rest[:idx])
	tail := strings.TrimSpace(rest[idx+len(closer):])
	if name == "" {
		return "", false
	}
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", false
	}
	return name, true
}

// stripMcpServers 按行剥离 tomlText 中顶层的 [mcp_servers] 与 [mcp_servers.<name>]
// 表段（含 [[mcp_servers...]] 数组表形式一并剥离以防御）——从匹配的表头行删到下一个
// 非 mcp_servers 前缀的表头行（或文件尾）。剥离结果必须仍能被 toml.Unmarshal 解析，
// 否则报错（防止行级启发式剥出语法不完整的残片）。
//
// 之所以不用「Unmarshal 整体再 Marshal 回去」的结构化方案（projecthooks/codex.go 那种
// round-trip），是因为 config 字段本质是用户维护的自由文本：round-trip 会丢注释/格式，
// 而这里只需要精确摘掉 XK-Pane 接管的 mcp_servers 表，其余原文必须逐字节保留。
func stripMcpServers(tomlText string) (string, error) {
	lines := strings.Split(tomlText, "\n")
	out := make([]string, 0, len(lines))
	inMcp := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if name, ok := parseTableHeader(trimmed); ok {
			if name == "mcp_servers" || strings.HasPrefix(name, "mcp_servers.") {
				inMcp = true
				continue // 跳过表头行本身，不计入输出
			}
			inMcp = false // 遇到别的表头，mcp_servers 段结束
		}
		if inMcp {
			continue
		}
		out = append(out, line)
	}
	result := strings.Join(out, "\n")
	if strings.TrimSpace(result) != "" {
		var m map[string]any
		if err := toml.Unmarshal([]byte(result), &m); err != nil {
			return "", fmt.Errorf("agentcfg: 剥离 mcp_servers 后的 TOML 不可解析: %w", err)
		}
	}
	return result, nil
}

// renderCodexMcpToml 把 agent_mcp_servers 表中 EnabledCodex=true 的行投影成
// [mcp_servers.<name>] TOML 文本，供 writeCodexLive 拼进最终 config.toml。无启用行时
// 返回 ""。本单元该表恒为空（管理 UI 留后续单元）→ 恒返回 ""，但逻辑写全，下一单元
// MCP 管理落地后直接复用。
//
// db 由调用方传入（而非固定用 s.db()）：writeCodexLive 总是在既有事务内被调用，本包
// storage.Holder 的连接池 MaxOpenConns=1（单连接 + WAL 保证写串行），若在事务内又用
// s.db() 发起新查询，会因抢同一个连接而自我死锁——必须复用调用方传入的 tx。
//
// 解密或反序列化失败的单行只记日志跳过，不炸整体投影，避免一条脏数据拖垮所有已启用的
// server。
func (s *Service) renderCodexMcpToml(ctx context.Context, db *gorm.DB) (string, error) {
	var rows []storage.AgentMcpServer
	if err := db.WithContext(ctx).Where("enabled_codex = ?", true).
		Order("name").Find(&rows).Error; err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	servers := map[string]any{}
	for _, row := range rows {
		plain, err := cryptox.DecryptString(row.ServerEnc)
		if err != nil {
			s.log.Warn("解密 mcp server 失败，跳过投影", "name", row.Name, "err", err)
			continue
		}
		var def map[string]any
		if err := json.Unmarshal([]byte(plain), &def); err != nil {
			s.log.Warn("解析 mcp server 定义失败，跳过投影", "name", row.Name, "err", err)
			continue
		}
		servers[row.Name] = def
	}
	if len(servers) == 0 {
		return "", nil
	}
	out, err := toml.Marshal(map[string]any{"mcp_servers": servers})
	if err != nil {
		return "", fmt.Errorf("agentcfg: 序列化 mcp_servers 投影失败: %w", err)
	}
	return string(out), nil
}

// writeCodexLive 把 codex Provider 的 settingsJSON 拆成 auth.json + config.toml 写入
// live 位置。db 见 renderCodexMcpToml 的注释——必须传调用方事务里的 tx。
//
//  1. 解析 settings，取 auth / config；
//  2. 拼最终 config 文本：stripMcpServers(config) 剥掉旧投影段，再接上
//     agent_mcp_servers 表的当前投影（两段之间空一行；投影段为空则不加），整体再
//     toml 解析校验一遍——校验失败则不触碰任何文件；
//  3. 一次性备份：config.toml 总会写，无条件备份一次；auth.json 只在本次确实要写
//     它（auth != nil）时才备份——「auth=null 表示不管理 auth.json」这条语义要贯彻到
//     底，不该为一个根本不打算碰的文件生成备份；
//  4. auth 先写、config 后写：若 auth 非 null，先读旧 auth.json 字节留作回滚，
//     WriteAtomic 写新 auth.json；再写 config.toml；config 写失败且刚才写过 auth →
//     尽力把旧 auth 字节写回（旧文件本不存在则删除新写的 auth.json），并返回错误，
//     避免登录态和配置文件停在不一致的组合上。auth 为 null → 完全不碰 auth.json。
func (s *Service) writeCodexLive(ctx context.Context, db *gorm.DB, settingsJSON string) error {
	cs, err := parseCodexSettings(settingsJSON)
	if err != nil {
		return err
	}

	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()

	if err := backupLiveOnce(configPath); err != nil {
		return fmt.Errorf("agentcfg: 备份 live config.toml 失败: %w", err)
	}
	if cs.Auth != nil {
		if err := backupLiveOnce(authPath); err != nil {
			return fmt.Errorf("agentcfg: 备份 live auth.json 失败: %w", err)
		}
	}

	mcpToml, err := s.renderCodexMcpToml(ctx, db)
	if err != nil {
		return err
	}
	stripped, err := stripMcpServers(cs.Config)
	if err != nil {
		return fmt.Errorf("agentcfg: 剥离 config 中的 mcp_servers 失败: %w", err)
	}
	finalConfig := stripped
	if mcpToml != "" {
		if strings.TrimSpace(stripped) != "" {
			finalConfig = strings.TrimRight(stripped, "\n") + "\n\n" + mcpToml
		} else {
			finalConfig = mcpToml
		}
	}
	if strings.TrimSpace(finalConfig) != "" {
		var m map[string]any
		if err := toml.Unmarshal([]byte(finalConfig), &m); err != nil {
			return fmt.Errorf("agentcfg: 拼接后的 config.toml 不是合法 TOML: %w", err)
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

	if err := filex.WriteAtomic(configPath, []byte(finalConfig), 0o600); err != nil {
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

// readCodexLive 读 config.toml + auth.json，组装成规范化的 codex Provider
// SettingsJSON，供回填 / 导入共用。config.toml 不存在视为 ""；存在则先
// stripMcpServers 剥掉 [mcp_servers] 段（agent_mcp_servers 表是该投影的单一事实来源，
// 不重复存进 Provider 快照）。auth.json 不存在或不是合法 JSON 对象 → auth=null（首次
// 导入时保守起见，不擅自把形状不明的旧文件内容当 auth 捕获）。
func (s *Service) readCodexLive() (string, error) {
	config := ""
	data, err := filex.ReadLimit(s.codexConfigPath(), 8<<20)
	switch {
	case err == nil:
		stripped, stripErr := stripMcpServers(string(data))
		if stripErr != nil {
			return "", fmt.Errorf("agentcfg: 剥离 live config.toml 中的 mcp_servers 失败: %w", stripErr)
		}
		config = stripped
	case errors.Is(err, os.ErrNotExist):
		// config.toml 不存在，config 保持 ""
	default:
		return "", fmt.Errorf("agentcfg: 读取 %s 失败: %w", s.codexConfigPath(), err)
	}

	var auth map[string]any
	if authData, err := filex.ReadLimit(s.codexAuthPath(), 8<<20); err == nil {
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

// switchCodex 是 SwitchAgentProvider 的 codex 分支，事务结构与 switchClaude 对称：
//  1. 事务内回填当前 IsCurrent 行（backfillCurrentCodex）；
//  2. 目标行 IsCurrent=true；
//  3. 事务内最后一步把目标配置写入 live（含 mcp_servers 投影）—— 写失败则整个事务
//     回滚（DB 状态不变，live 未写成功也未损坏，filex.WriteAtomic 本身是原子的）。
func (s *Service) switchCodex(ctx context.Context, target storage.AgentProvider) error {
	targetPlain, err := cryptox.DecryptString(target.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密目标 provider 配置失败: %w", err)
	}
	return s.db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current storage.AgentProvider
		err := tx.Where("app_type = ? AND is_current = ?", appTypeCodex, true).First(&current).Error
		switch {
		case err == nil:
			if err := s.backfillCurrentCodex(tx, current); err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// 无当前行（异常场景，正常应有 importCodexDefault 兜底），跳过回填
		default:
			return err
		}

		if err := tx.Model(&storage.AgentProvider{}).Where("id = ?", target.ID).
			Update("is_current", true).Error; err != nil {
			return err
		}

		return s.writeCodexLive(ctx, tx, targetPlain)
	})
}

// backfillCurrentCodex 把 live 当前内容规范化后加密写回旧 current 行，并清掉它的
// IsCurrent 标志。关键点：旧行原 auth 为 null 时只回填 config、强制保持 auth=null——
// 即便切换前 live auth.json 确实存在内容，也不擅自替它捕获，因为该行明确声明「不管理
// auth」，回填不能悄悄改变这条语义。live 不可读时跳过内容回填但仍清 IsCurrent，保证
// 单一 current 不变式。
func (s *Service) backfillCurrentCodex(tx *gorm.DB, current storage.AgentProvider) error {
	updates := map[string]any{"is_current": false}

	currentPlain, err := cryptox.DecryptString(current.SettingsEnc)
	if err != nil {
		return fmt.Errorf("agentcfg: 解密旧 current provider 配置失败: %w", err)
	}
	currentSettings, err := parseCodexSettings(currentPlain)
	if err != nil {
		return err
	}

	if liveJSON, err := s.readCodexLive(); err == nil {
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

// importCodexDefault 是 Startup 与 ImportCurrentAgentConfig 共用的导入逻辑，镜像
// importClaudeDefault。live 可读 → 用其规范化内容；config.toml/auth.json 都不存在（或
// 读取失败）→ 用 {"auth":null,"config":""}。均命名「默认配置」、Category=imported、
// IsCurrent=true —— 除非 manual=true 且已存在同 appType 行（手动「从当前 live 导入」不
// 抢当前），此时命名「导入配置 <时间>」且 IsCurrent=false。
func (s *Service) importCodexDefault(ctx context.Context, manual bool) (*AgentProviderDTO, error) {
	settingsJSON := `{"auth":null,"config":""}`
	if liveJSON, err := s.readCodexLive(); err == nil {
		settingsJSON = liveJSON
	}

	enc, err := cryptox.EncryptString(settingsJSON)
	if err != nil {
		return nil, err
	}

	var existing int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeCodex).Count(&existing).Error; err != nil {
		return nil, err
	}

	row := storage.AgentProvider{
		ID:          uuid.NewString(),
		AppType:     appTypeCodex,
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
