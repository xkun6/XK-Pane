// Package gitsvc 是 gitx 只读/工具 Git 门面的 Wails service 包装（Phase 4，接前端）。
//
// 设计：gitx 是**按路径操作的无状态原语**，故本 service 不持 *storage.Holder，
// 仅做「打开仓库 → 调原语 → 收口 DTO」的薄包装，让 Git 后端经 bindings 可达前端、
// 且本身可单测。每次调用按 path 现开仓库（gitx.Open 廉价，DetectDotGit）。
//
// 错误语义：GetStatus 对「非仓库」返回 {IsRepo:false}（对齐 cc-pane 的 null-status
// 友好降级，前端据此隐藏 Git 面板）；其余明细/变更方法对非仓库**透传 ErrNotARepo**
// （UI 仅在 IsRepo 时调用它们）。所有人类可见错误文案归前端 i18n，后端只回英文/原始错误。
//
// 层次边界（与 gitx 文档一致）：worktree 分组目录策略、名称安全校验、pull/push/stash 前
// 的 Local History 自动打标签钩子——都属**上层 UX/历史子系统**，留待后续单元在本 service
// 外层或更高层补；本 service 只做原语的 1:1 暴露。
package gitsvc

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/gitx"
)

// Service 是无状态的 Git 门面。
type Service struct{}

// New 创建 gitsvc。无依赖（gitx 按路径操作，不碰 SQLite Holder）。
func New() *Service {
	return &Service{}
}

// StatusDTO 是仓库的概要状态（gitx 无此组合类型，故 service 本地定义）。
//   - IsRepo：path 是否一个 git 仓库；为 false 时 Branch/IsDirty 无意义。
//   - Branch：当前分支（detached → "HEAD"，空仓 → ""，对齐 cc-pane）。
//   - IsDirty：工作树是否有未提交改动（含未跟踪）。
type StatusDTO struct {
	IsRepo  bool   `json:"isRepo"`
	Branch  string `json:"branch"`
	IsDirty bool   `json:"isDirty"`
}

// IsRepo 廉价判定 path 是否一个 git 仓库（不 shell git）。
func (s *Service) IsRepo(path string) bool {
	return gitx.IsRepo(path)
}

// GetStatus 返回仓库概要状态。非仓库 → {IsRepo:false}（不报错，友好降级）；
// 其它打开错误 / 状态查询错误透传。
func (s *Service) GetStatus(ctx context.Context, path string) (StatusDTO, error) {
	repo, err := gitx.Open(path)
	if errors.Is(err, gitx.ErrNotARepo) {
		return StatusDTO{IsRepo: false}, nil
	}
	if err != nil {
		return StatusDTO{}, err
	}
	branch, err := repo.CurrentBranch()
	if err != nil {
		return StatusDTO{}, err
	}
	dirty, err := repo.IsDirty(ctx)
	if err != nil {
		return StatusDTO{}, err
	}
	return StatusDTO{IsRepo: true, Branch: branch, IsDirty: dirty}, nil
}

// Branches 返回本地分支列表（标注当前分支）。非仓库透传 ErrNotARepo。
func (s *Service) Branches(_ context.Context, path string) ([]gitx.BranchInfo, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return nil, err
	}
	return repo.Branches()
}

// Log 返回从 HEAD 起最近 limit 条提交（最新在前）；limit<=0 取默认 50，空仓回 nil。
// 非仓库透传 ErrNotARepo。
func (s *Service) Log(_ context.Context, path string, limit int) ([]gitx.CommitInfo, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return nil, err
	}
	return repo.Log(limit)
}

// Worktrees 列出本仓库所有 worktree（含主工作树，首条为主）。非仓库透传 ErrNotARepo。
func (s *Service) Worktrees(ctx context.Context, path string) ([]gitx.WorktreeInfo, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return nil, err
	}
	return repo.Worktrees(ctx)
}

// WorktreeAdd 在 worktreePath 处新建 worktree；newBranch 非空则创建并检出该新分支。
// 注意：worktreePath 须为调用方算好的绝对路径（分组目录策略属上层 UX，见包注释）。
func (s *Service) WorktreeAdd(ctx context.Context, repoPath, worktreePath, newBranch string) error {
	repo, err := gitx.Open(repoPath)
	if err != nil {
		return err
	}
	return repo.WorktreeAdd(ctx, worktreePath, newBranch)
}

// WorktreeRemove 删除 worktreePath 处的 worktree（恒带 --force，对齐 cc-pane）。
func (s *Service) WorktreeRemove(ctx context.Context, repoPath, worktreePath string) error {
	repo, err := gitx.Open(repoPath)
	if err != nil {
		return err
	}
	return repo.WorktreeRemove(ctx, worktreePath)
}

// WorktreeAddUnder 在 parentDir 下以 name 为子目录名新建 worktree（cc-pane 分组目录策略，
// gitx 文档明确把此「上层 UX 决策」留给 service 层）。本方法在此做服务层该做的两件事：
//   1. name 安全校验（非空、无路径分隔符 / '..' / 前导 '-'）—— 护住「子目录名」语义不被
//      穿越或当作 git 选项；
//   2. 用 filepath.Join（OS 感知）算出绝对 worktreePath = parentDir/name，再交原语创建。
//
// newBranch 非空则创建并检出该新分支（典型用法：name 同时作目录名与分支名，由 View 传同值）。
// 返回创建出的 worktree 绝对路径，便于前端提示 / 后续操作。目标路径已存在时 git 原语报错透传。
func (s *Service) WorktreeAddUnder(ctx context.Context, repoPath, parentDir, name, newBranch string) (string, error) {
	if err := validateWorktreeName(name); err != nil {
		return "", err
	}
	if strings.TrimSpace(parentDir) == "" {
		return "", errors.New("gitsvc: worktree parent directory required")
	}
	repo, err := gitx.Open(repoPath)
	if err != nil {
		return "", err
	}
	wtPath := filepath.Join(parentDir, strings.TrimSpace(name))
	if err := repo.WorktreeAdd(ctx, wtPath, newBranch); err != nil {
		return "", err
	}
	return wtPath, nil
}

// validateWorktreeName 校验「子目录名 / 分支名」安全：非空、不含路径分隔符或 '..'、不以 '-'
// 开头（避免被当作 git 选项）。允许的字符集刻意保守（基本名段）——复杂分支名（含 '/'）超出
// 本基础 UI 范畴，留待更专门的 worktree 单元。
func validateWorktreeName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return errors.New("gitsvc: worktree name cannot be empty")
	}
	if strings.ContainsAny(n, `/\`) || strings.Contains(n, "..") {
		return errors.New("gitsvc: worktree name must not contain path separators or '..'")
	}
	if strings.HasPrefix(n, "-") {
		return errors.New("gitsvc: worktree name must not start with '-'")
	}
	return nil
}

// Stash 暂存当前工作区改动，返回 git 提示信息。非仓库透传 ErrNotARepo。
func (s *Service) Stash(ctx context.Context, path string) (string, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return "", err
	}
	return repo.Stash(ctx)
}

// StashPop 弹出最近一次 stash，返回 git 提示信息。无 stash 条目时透传错误。
func (s *Service) StashPop(ctx context.Context, path string) (string, error) {
	repo, err := gitx.Open(path)
	if err != nil {
		return "", err
	}
	return repo.StashPop(ctx)
}

// Clone 克隆 url 到 dst（dst 为目标目录绝对路径，git 自建之）；shallow → --depth 1。
// dst 已存在则报错。鉴权走系统凭据助手 / SSH（GIT_TERMINAL_PROMPT=0 禁交互弹窗）。
func (s *Service) Clone(ctx context.Context, url, dst string, shallow bool) error {
	return gitx.Clone(ctx, url, dst, gitx.CloneOptions{Shallow: shallow})
}

// Fetch 拉取所有远端引用（git fetch --all）。非仓库透传 ErrNotARepo。
func (s *Service) Fetch(ctx context.Context, path string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	return repo.Fetch(ctx)
}

// Pull 合并远端改动（git pull）。非仓库透传 ErrNotARepo。
func (s *Service) Pull(ctx context.Context, path string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	return repo.Pull(ctx)
}

// Push 推送当前分支（git push）。无上游分支 / 非仓库透传错误。
func (s *Service) Push(ctx context.Context, path string) error {
	repo, err := gitx.Open(path)
	if err != nil {
		return err
	}
	return repo.Push(ctx)
}
