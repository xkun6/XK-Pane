// Package agentcfg 实现 cc-switch 式全局配置 Provider：把 Claude Code / Codex 等 CLI
// 的全局配置文件整份存成一个 Provider，切换时原子性直写 live 配置文件，而不是像旧
// providers 表那样只存几个零散字段再拼装启动 env。
//
// 第一单元做了 Claude 部分：三张表骨架 + Claude 的导入/切换/回填全流程。
// 第二单元补齐 Codex 部分（auth.json + config.toml 双文件，见 codex.go）。
package agentcfg

import (
	"log/slog"
	"os"
	"path/filepath"

	"xkpane/internal/storage"
	"xkpane/internal/utils/logx"

	"gorm.io/gorm"
)

// Service 是 cc-switch 式全局配置 Provider 的 Wails 门面：既做 agent_providers 表的
// CRUD，又直接读写 CLI 的全局 live 配置文件。
type Service struct {
	holder  *storage.Holder
	home    string // 用户主目录；测试可注入 t.TempDir() 隔离真实 ~/.claude
	dataDir string // 应用数据目录根（与 storage.json 同一目录，不受用户自定义数据库路径
	// 影响）；agent-skills SSOT 建在其下（见 skills.go）；测试可注入 t.TempDir()
	log *slog.Logger
}

// New 创建 Service。home 取 os.UserHomeDir()；dataDir 取 storage.UserDataDir()（与
// storage.json 同一个应用配置目录）。两者任一解析失败都只记日志、留空——后续读写会因
// 路径明显异常而报错，好过悄悄写到意外位置。
func New(holder *storage.Holder) *Service {
	log := logx.For("agentcfg")
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn("resolve user home dir failed", "err", err)
	}
	dataDir, err := storage.UserDataDir()
	if err != nil {
		log.Warn("resolve user data dir failed", "err", err)
	}
	return &Service{holder: holder, home: home, dataDir: dataDir, log: log}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// claudeSettingsPath 是 Claude Code 全局配置文件路径：<home>/.claude/settings.json。
func (s *Service) claudeSettingsPath() string {
	return filepath.Join(s.home, ".claude", "settings.json")
}

// codexConfigPath 是 Codex CLI 全局配置文件路径：<home>/.codex/config.toml。
func (s *Service) codexConfigPath() string {
	return filepath.Join(s.home, ".codex", "config.toml")
}

// codexAuthPath 是 Codex CLI 全局登录态文件路径：<home>/.codex/auth.json。
func (s *Service) codexAuthPath() string {
	return filepath.Join(s.home, ".codex", "auth.json")
}

// grokConfigPath 是 Grok Build CLI 全局配置文件路径：<home>/.grok/config.toml。
func (s *Service) grokConfigPath() string {
	return filepath.Join(s.home, ".grok", "config.toml")
}

// grokAuthPath 是 Grok Build CLI 全局登录态文件路径：<home>/.grok/auth.json。
func (s *Service) grokAuthPath() string {
	return filepath.Join(s.home, ".grok", "auth.json")
}

// claudeMcpPath 是 claude CLI 自身维护的主状态文件路径：<home>/.claude.json（注意不是
// <home>/.claude/settings.json —— 那是 Provider 域管理的全局配置文件）。mcpServers 只是
// 这份大文件顶层的一个键，文件可能很大且含大量与本包无关的字段（projects/history 等），
// 投影逻辑必须只碰 mcpServers，绝不整体覆盖，见 mcp.go 的 projectMcpClaude。
func (s *Service) claudeMcpPath() string {
	return filepath.Join(s.home, ".claude.json")
}
