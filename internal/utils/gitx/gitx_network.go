package gitx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// CloneOptions 控制 Clone 行为。
//
// 当前仅 Shallow（对齐 cc-pane GitCloneRequest.shallow）。延后（已记 findings J11）：
//   - 显式 username/password：cc-pane 把凭据注入 URL（https://user:pass@host）。argv 里
//     带明文凭据会泄漏到进程列表 / 日志 / reflog，是已知反模式；本仓优先走系统 git 凭据助手
//     （HTTPS）/ SSH key（无需 argv 凭据）。显式凭据待 clone UI 单元用 askpass / credential
//     -helper 设计，不走 argv 注入。
//   - 细粒度进度事件：git 进度用 \r 覆盖同一行，而 procx.scan 按 \n 切分（无法增量流出
//     phase/percent）；待 clone UI 单元引入 \r-aware reader + Wails 事件。
type CloneOptions struct {
	Shallow bool // true → --depth 1
}

// Clone 克隆 url 到 dst（dst 为目标仓库目录的绝对路径，git 自建之）。dst 已存在则报错
// （对齐 cc-pane「Target directory already exists」）。鉴权走系统 git 凭据助手 / SSH key；
// GIT_TERMINAL_PROMPT=0 禁交互弹窗（GUI 子进程铁律）——私库无凭据助手时直接失败而非挂起。
func Clone(ctx context.Context, url, dst string, opts CloneOptions) error {
	if url == "" || dst == "" {
		return errors.New("gitx: clone requires url and dst")
	}
	if _, err := os.Stat(dst); err == nil {
		return errors.New("gitx: clone destination already exists: " + dst)
	}
	args := []string{"clone"}
	if opts.Shallow {
		args = append(args, "--depth", "1")
	}
	// `--` 终止选项，护住以 '-' 开头的 url/dst 不被当作 git 选项。
	args = append(args, "--", url, dst)
	// cwd 取 dst 父目录（须存在）；clone 用绝对 dst，cwd 仅需是有效目录。
	_, _, err := runGit(ctx, filepath.Dir(dst), args...)
	return err
}

// Fetch 拉取所有远端引用（`git fetch --all`，对齐 cc-pane git_fetch）。只更新远端跟踪
// 引用、不动工作树，无需历史标签。
func (r *Repo) Fetch(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "fetch", "--all")
	return err
}

// Pull 合并远端改动（`git pull`，对齐 cc-pane git_pull）。
//
// 注意（层次）：cc-pane 在 pull 前打 Local History 标签（auto_label_before_git）；该钩子
// 属历史子系统，留 gitsvc 服务层在外层补（本仓 Local History 尚未落地）。本原语不耦合历史。
func (r *Repo) Pull(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "pull")
	return err
}

// Push 推送当前分支（`git push`，对齐 cc-pane git_push）。无上游分支时 git 退出非零 →
// 返回错误。同 Pull：cc-pane 的 push 前 Local History 标签钩子留 gitsvc 服务层。
func (r *Repo) Push(ctx context.Context) error {
	_, _, err := runGit(ctx, r.path, "push")
	return err
}
