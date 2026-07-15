package agentcfg

import (
	"context"

	"xkpane/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ServiceStartup 是 Wails 生命周期钩子。应用启动时若本地还没有 Claude Provider
// （首次使用本功能 / 升级前的老用户），从当前 live settings.json 导入一份，保证
// 「切换回填」永远有目标行可用。任何错误只记日志、不阻断应用启动——cc-switch 域是
// 新增能力，不应拖垮已有功能。
func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	var count int64
	if err := s.db().WithContext(ctx).Model(&storage.AgentProvider{}).
		Where("app_type = ?", appTypeClaude).Count(&count).Error; err != nil {
		s.log.Warn("count claude providers failed", "err", err)
		return nil
	}
	if count > 0 {
		return nil
	}
	if _, err := s.importClaudeDefault(ctx, false); err != nil {
		s.log.Warn("import default claude provider failed", "err", err)
	}
	return nil
}
