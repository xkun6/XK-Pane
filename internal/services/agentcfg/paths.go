// Package agentcfg 实现 cc-switch 式全局配置 Provider：把 Claude Code / Codex 等 CLI
// 的全局配置文件整份存成一个 Provider，切换时原子性直写 live 配置文件，而不是像旧
// providers 表那样只存几个零散字段再拼装启动 env。
//
// 本单元（第一单元）只做 Claude 部分：三张表骨架 + Claude 的导入/切换/回填全流程。
// Codex 部分（auth.json + config.toml 双文件）留后续单元，本包当前不预置其路径方法。
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
	holder *storage.Holder
	home   string // 用户主目录；测试可注入 t.TempDir() 隔离真实 ~/.claude
	log    *slog.Logger
}

// New 创建 Service。home 取 os.UserHomeDir()；取不到则留空并记日志——后续 live 读写
// 会因路径落在 "/.claude/settings.json" 而明显出错，好过悄悄写到意外位置。
func New(holder *storage.Holder) *Service {
	log := logx.For("agentcfg")
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn("resolve user home dir failed", "err", err)
	}
	return &Service{holder: holder, home: home, log: log}
}

func (s *Service) db() *gorm.DB {
	return s.holder.Current().GORM
}

// claudeSettingsPath 是 Claude Code 全局配置文件路径：<home>/.claude/settings.json。
func (s *Service) claudeSettingsPath() string {
	return filepath.Join(s.home, ".claude", "settings.json")
}
