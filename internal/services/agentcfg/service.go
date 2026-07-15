package agentcfg

import (
	"context"

	"xkpane/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ServiceStartup 是 Wails 生命周期钩子。应用启动时若本地还没有 Claude / Codex
// Provider（首次使用本功能 / 升级前的老用户），分别从当前 live 配置导入一份，保证
// 「切换回填」永远有目标行可用。任何错误只记日志、不阻断应用启动——cc-switch 域是
// 新增能力，不应拖垮已有功能；两个 appType 相互独立，一个导入失败不影响另一个。
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
	return nil
}
