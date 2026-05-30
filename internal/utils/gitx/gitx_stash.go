package gitx

import (
	"context"
	"strings"
)

// Stash 暂存当前工作区改动（`git stash`，对齐 cc-pane git_stash）。返回 git 的提示信息
// （如「Saved working directory ...」或「No local changes to save」——后者退出码 0，
// 不视为错误）。go-git 无 stash，故 CLI-only。
//
// 注意（层次）：cc-pane 在 stash 前会先给 Local History 打标签（auto_label_before_git）。
// 本仓 Local History 子系统尚未落地，该钩子留待 gitsvc 服务层在其外层补上（届时由 service
// 在调用 Stash/StashPop 前触发 history 标签），本原语不耦合历史子系统。
func (r *Repo) Stash(ctx context.Context) (string, error) {
	stdout, _, err := runGit(ctx, r.path, "stash")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// StashPop 弹出最近一次 stash（`git stash pop`，对齐 cc-pane git_stash_pop）。
// 无 stash 条目时 git 退出非零 → 返回错误（含 git 的「No stash entries found.」）。
func (r *Repo) StashPop(ctx context.Context) (string, error) {
	stdout, _, err := runGit(ctx, r.path, "stash", "pop")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
