// Package agentcfg 实现 cc-switch 式全局配置 Provider：把 Claude Code / Codex 等 CLI
// 的全局配置文件整份存成一个 Provider，切换时原子性直写 live 配置文件，而不是像旧
// providers 表那样只存几个零散字段再拼装启动 env。
//
// 第一单元做了 Claude 部分：三张表骨架 + Claude 的导入/切换/回填全流程。
// 第二单元补齐 Codex 部分（auth.json + config.toml 双文件，见 codex.go）。
// Grok Build 配置模型与 codex 同构（见 grok.go）；panes MCP 不写 ~/.grok，由
// cliadapters 在启动会话时注入项目级 .grok/config.toml（对齐 Claude --mcp-config /
// Codex -c 的会话级注入，而非 agentcfg live 投影）。
package agentcfg

import (
	"context"

	"xkpane/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ServiceStartup 是 Wails 生命周期钩子。应用启动时若本地还没有 Claude / Codex / Grok
// Provider（首次使用本功能 / 升级前的老用户），分别从当前 live 配置导入一份，保证
// 「切换回填」永远有目标行可用。任何错误只记日志、不阻断应用启动——cc-switch 域是
// 新增能力，不应拖垮已有功能；三个 appType 相互独立，一个导入失败不影响其它。
//
// 启动后只拉起 live 文件监听；与磁盘的全量对齐改在打开设置（List）时按需做，
// 既保证需要时读到最新，又不拖慢冷启动。
func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	var claudeCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeClaude).Count(&claudeCount).Error; err != nil {
		s.log.Warn("count claude providers failed", "err", err)
	} else if claudeCount == 0 {
		if _, err := s.importClaudeDefault(ctx, false); err != nil {
			s.log.Warn("import default claude provider failed", "err", err)
		}
	}

	var codexCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeCodex).Count(&codexCount).Error; err != nil {
		s.log.Warn("count codex providers failed", "err", err)
	} else if codexCount == 0 {
		if _, err := s.importCodexDefault(ctx, false); err != nil {
			s.log.Warn("import default codex provider failed", "err", err)
		}
	}

	var grokCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeGrok).Count(&grokCount).Error; err != nil {
		s.log.Warn("count grok providers failed", "err", err)
	} else if grokCount == 0 {
		if _, err := s.importGrokDefault(ctx, false); err != nil {
			s.log.Warn("import default grok provider failed", "err", err)
		}
	}

	var reclaudeCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeReclaude).Count(&reclaudeCount).Error; err != nil {
		s.log.Warn("count reclaude providers failed", "err", err)
	} else if reclaudeCount == 0 {
		if _, err := s.importReclaudeDefault(ctx, false); err != nil {
			s.log.Warn("import default reclaude provider failed", "err", err)
		}
	}

	var mcpCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentMcpServer{}).Count(&mcpCount).Error; err != nil {
		s.log.Warn("count agent mcp servers failed", "err", err)
	} else if mcpCount == 0 {
		if _, err := s.ImportMcpFromLive(ctx); err != nil {
			s.log.Warn("import mcp servers from live failed", "err", err)
		}
	}

	var skillCount int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentSkill{}).Count(&skillCount).Error; err != nil {
		s.log.Warn("count agent skills failed", "err", err)
	} else if skillCount == 0 {
		if _, err := s.ImportSkillsFromApps(ctx); err != nil {
			s.log.Warn("import skills from apps failed", "err", err)
		}
	}

	// 一次性迁移：旧 internal/services/provider 里 claude/codex 相关类型迁移成 agentcfg
	// Provider（见 migrate_legacy.go）。放在以上导入步骤之后，与其它步骤同一容错模式。
	if err := s.migrateLegacyProviders(ctx); err != nil {
		s.log.Warn("migrate legacy providers failed", "err", err)
	}

	// 两层模型一次性迁移:老用户「每 Provider 存整份」→ 拆成系统配置基底 + 凭据补丁(幂等)。
	if err := s.migrateClaudeTwoLayer(ctx); err != nil {
		s.log.Warn("migrate claude two-layer failed", "err", err)
	}

	// 运行期监听全局配置目录；外部改动（含 CLAUDE.md / AGENTS.md）→ 同步 DB + 广播事件。
	// 启动时不做全量 live 同步（打开设置/List 时再按需对齐，见 ListAgentProviders），
	// 避免拖慢冷启动；用 ServiceStartup 的 ctx：应用退出时取消 watcher。
	s.startLiveWatcher(ctx)
	return nil
}
