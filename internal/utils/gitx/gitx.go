// Package gitx 是只读 Git 信息门面（Phase 4 unit 1）。
//
// 方案 D4：go-git/v5 为主（纯 Go，无 CGO；v5.19.1 已随 Wails 间接引入），
// 个别 go-git 弱项 / 性能热点走 git CLI 回退（经 internal/utils/procx 同步执行，
// 见 cli.go）。**不上 go-git v6-alpha。**
//
// cc-pane 的 git 是 100% CLI（shell 出 git rev-parse/status/...），且范围很小
// （branch / dirty-bit / clone / worktree / pull·push·stash 工具条），**无** diff/log/
// commit/stage（那些由用户在内置终端里手动做）。本 unit 仅做**只读核心**：
//   - Open / IsRepo：打开与是否仓库判定
//   - CurrentBranch：当前分支（detached → "HEAD"，空仓 → ""，对齐 cc-pane）
//   - IsDirty：工作树是否有改动（单 bit，对齐 cc-pane 的 bool-only status；走 CLI
//     `status --porcelain` —— 既忠实 cc-pane，又避开 go-git Worktree.Status() 的
//     大仓性能热点 issue #181/#844）
//   - Branches / Log：本地分支列表 / 最近提交（go-git 原生，常用增强）
//
// CLI-only 变更操作（go-git 弱项/无对应，对齐 cc-pane 100% CLI；见 gitx_worktree.go /
// gitx_stash.go）：
//   - Worktrees / WorktreeAdd / WorktreeRemove：worktree 增删查
//   - Stash / StashPop：暂存 / 弹出
//
// 后续 unit：结构化 per-file status + stage/commit/diff（git 面板）、network（fetch/
// push/pull/clone）。
package gitx

import (
	"context"
	"errors"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// ErrNotARepo 表示路径不是一个 git 仓库（调用方可据此映射为 cc-pane 的 null 语义）。
var ErrNotARepo = errors.New("gitx: not a git repository")

// Repo 是一个已打开仓库的句柄。
type Repo struct {
	path string
	gg   *git.Repository
}

// BranchInfo 是本地分支条目（json camelCase，供后续 service 包装）。
type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
}

// CommitInfo 是一条提交（When 为原始 time.Time，相对时间留前端格式化，后端 i18n-free）。
type CommitInfo struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"shortHash"`
	Subject   string    `json:"subject"`
	Author    string    `json:"author"`
	When      time.Time `json:"when"`
}

// Open 打开 path 处的仓库。非仓库返回 ErrNotARepo。DetectDotGit 处理 worktree/
// submodule 的 `.git` 文件与从子目录打开。
func Open(path string) (*Repo, error) {
	gg, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, ErrNotARepo
	}
	if err != nil {
		return nil, err
	}
	return &Repo{path: path, gg: gg}, nil
}

// IsRepo 廉价判定 path 是否一个 git 仓库（含 worktree 的 .git 文件），不 shell git。
func IsRepo(path string) bool {
	_, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}

// CurrentBranch 返回当前分支名。detached HEAD → "HEAD"（对齐 cc-pane）；
// 空仓（未生分支的 unborn HEAD）→ ""。
func (r *Repo) CurrentBranch() (string, error) {
	ref, err := r.gg.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", nil // 空仓
	}
	if err != nil {
		return "", err
	}
	if ref.Name().IsBranch() {
		return ref.Name().Short(), nil
	}
	return "HEAD", nil // detached
}

// IsDirty 判定工作树是否有未提交改动（含未跟踪）。走 CLI `status --porcelain`
// （忠实 cc-pane + 避开 go-git Status() 大仓性能热点）。
func (r *Repo) IsDirty(ctx context.Context) (bool, error) {
	stdout, _, err := runGit(ctx, r.path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) != "", nil
}

// Branches 返回本地分支列表（refs/heads/*），标注当前分支。
func (r *Repo) Branches() ([]BranchInfo, error) {
	var headName plumbing.ReferenceName
	if head, err := r.gg.Head(); err == nil {
		headName = head.Name()
	}
	iter, err := r.gg.Branches()
	if err != nil {
		return nil, err
	}
	out := make([]BranchInfo, 0)
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		out = append(out, BranchInfo{
			Name:      ref.Name().Short(),
			IsCurrent: ref.Name() == headName,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Log 返回从 HEAD 起的最近 limit 条提交（最新在前）。limit<=0 取默认 50。
// 空仓返回 (nil, nil)。
func (r *Repo) Log(limit int) ([]CommitInfo, error) {
	if _, err := r.gg.Head(); errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, nil // 空仓，无提交
	}
	if limit <= 0 {
		limit = 50
	}
	iter, err := r.gg.Log(&git.LogOptions{})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	out := make([]CommitInfo, 0, limit)
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return storer.ErrStop
		}
		hash := c.Hash.String()
		out = append(out, CommitInfo{
			Hash:      hash,
			ShortHash: hash[:7],
			Subject:   firstLine(c.Message),
			Author:    c.Author.Name,
			When:      c.Author.When,
		})
		count++
		return nil
	})
	if err != nil && !errors.Is(err, storer.ErrStop) {
		return nil, err
	}
	return out, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
