// Package localhistorysvc 是 internal/localhistory 子系统的 Wails service 包装（Phase 4，接前端）。
//
// 与 gitsvc 一样按 path 操作，但 Local History 是**每项目独立 db**（决策门 D10：每个项目
// <proj>/.xkpanes/history/，不走 app 主库 Holder）。localhistory 的包注释明确把「服务层按项目
// 缓存 Store」留给本层 —— 故本 service 维护一张 projectPath → *Store 的缓存（互斥保护），
// 首次访问某项目时 Open（建库 + AutoMigrate），后续命中缓存复用同一连接池。
//
// 暴露面（历史面板所需的读 + 快照 + 回滚 + 标签）：
//   - SnapshotFile：读工作树文件 → 存一个版本（手动快照；未来 file-watcher 走同一 SaveVersion）
//   - GetRecentChanges / ListVersions / ListDeletedFiles / ListLabels：浏览
//   - GetVersionContent：取某版本文本内容
//   - GetVersionDiff（版本 vs 工作树）/ GetVersionsDiff（版本间）：diff
//   - RestoreVersion（单文件）/ RestoreToLabel（按标签整组）：回滚（内部自动打 Before-Restore 标签）
//   - DeleteLabel：删标签
//
// 层次边界（留后续）：file-watcher 自动捕获、gitsvc Pull/Push/Stash 前的 auto-label 钩子、
// 清理 / 压缩等维护操作 —— 不在面板交互面内，留更专门的单元。
package localhistorysvc

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"xkpane/internal/localhistory"
)

// Service 维护每项目的 Store 缓存。
type Service struct {
	mu     sync.Mutex
	stores map[string]*localhistory.Store
}

// New 创建 localhistorysvc。无 Holder 依赖（每项目独立 db）。
func New() *Service {
	return &Service{stores: make(map[string]*localhistory.Store)}
}

// store get-or-open：返回 projectPath 的缓存 Store，未缓存则 Open 并缓存。
func (s *Service) store(projectPath string) (*localhistory.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.stores[projectPath]; ok {
		return st, nil
	}
	st, err := localhistory.Open(projectPath)
	if err != nil {
		return nil, err
	}
	s.stores[projectPath] = st
	return st, nil
}

// closeAll 关闭并清空所有缓存 Store（应用退出 / 测试清理用；未导出，不绑定前端）。
func (s *Service) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, st := range s.stores {
		_ = st.Close()
		delete(s.stores, k)
	}
}

// SnapshotFile 读取工作树文件（filePath 为项目内相对路径）并存一个版本（手动快照，无节流）。
// 内容未变会因去重返回 nil（前端据此判断「无变化」）。读文件失败透传。
func (s *Service) SnapshotFile(ctx context.Context, projectPath, filePath, branch string) (*localhistory.FileVersion, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(projectPath, filepath.FromSlash(filePath))
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	return st.SaveVersion(ctx, filePath, content, false, branch, 0)
}

// GetRecentChanges 返回最近版本变更（跨文件，最新在前）；limit<=0 取默认 50。
func (s *Service) GetRecentChanges(ctx context.Context, projectPath string, limit int) ([]localhistory.RecentChange, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	return st.GetRecentChanges(ctx, limit)
}

// ListVersions 列出某文件的所有版本（id 升序，即时序）。
func (s *Service) ListVersions(ctx context.Context, projectPath, filePath string) ([]localhistory.FileVersion, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	return st.ListVersions(ctx, filePath)
}

// ListDeletedFiles 列出「最新版本为删除标记」的文件（已删但有历史可恢复）。
func (s *Service) ListDeletedFiles(ctx context.Context, projectPath string) ([]localhistory.FileVersion, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	return st.ListDeletedFiles(ctx)
}

// GetVersionContent 取某版本的文本内容（gzip 兼容读）。版本不存在透传错误。
func (s *Service) GetVersionContent(ctx context.Context, projectPath, filePath, versionID string) (string, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return "", err
	}
	content, err := st.GetVersionContent(ctx, filePath, versionID)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// GetVersionDiff 返回某版本与当前工作树文件的 diff（工作树文件不存在视为空 = 删除全部）。
func (s *Service) GetVersionDiff(ctx context.Context, projectPath, filePath, versionID string) (localhistory.DiffResult, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	return st.GetVersionDiff(ctx, filePath, versionID)
}

// GetVersionsDiff 返回两个版本之间的 diff。
func (s *Service) GetVersionsDiff(ctx context.Context, projectPath, filePath, oldVersionID, newVersionID string) (localhistory.DiffResult, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return localhistory.DiffResult{}, err
	}
	return st.GetVersionsDiff(ctx, filePath, oldVersionID, newVersionID)
}

// ListLabels 列出所有标签（timestamp 倒序），含各自文件快照。
func (s *Service) ListLabels(ctx context.Context, projectPath string) ([]localhistory.HistoryLabel, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	return st.ListLabels(ctx)
}

// RestoreVersion 把某文件恢复到指定版本（写回工作树）；恢复前自动打 Before-Restore 标签。
// branch 由前端传入（来自 gitsvc.GetStatus().branch，""=未知 / 非 git 仓）。
func (s *Service) RestoreVersion(ctx context.Context, projectPath, filePath, versionID, branch string) error {
	st, err := s.store(projectPath)
	if err != nil {
		return err
	}
	return st.RestoreVersion(ctx, filePath, versionID, branch)
}

// RestoreToLabel 把某标签捕获的所有文件快照恢复到工作树，返回成功恢复的相对路径列表。
func (s *Service) RestoreToLabel(ctx context.Context, projectPath, labelID, branch string) ([]string, error) {
	st, err := s.store(projectPath)
	if err != nil {
		return nil, err
	}
	return st.RestoreToLabel(ctx, labelID, branch)
}

// AutoLabel 为 projectPath 打一个 auto 标签（快照当前所有已跟踪文件的最新版本），返回标签 id。
// 供 gitsvc 在 git stash/pull/push 前注入调用（连接 Git 域与 Local History，对齐 cc-pane
// auto_label_before_git）。**守门**：若该项目尚无历史存储（localhistory.Exists==false），直接
// 跳过返回 ("", nil) —— 不为一次纯 git 操作凭空创建 .xkpanes/（仅对已在用 Local History 的项目生效）。
func (s *Service) AutoLabel(ctx context.Context, projectPath, name, source, branch string) (string, error) {
	if !localhistory.Exists(projectPath) {
		return "", nil
	}
	st, err := s.store(projectPath)
	if err != nil {
		return "", err
	}
	return st.CreateAutoLabel(ctx, name, source, branch)
}

// DeleteLabel 删除标签及其快照。
func (s *Service) DeleteLabel(ctx context.Context, projectPath, labelID string) error {
	st, err := s.store(projectPath)
	if err != nil {
		return err
	}
	return st.DeleteLabel(ctx, labelID)
}
