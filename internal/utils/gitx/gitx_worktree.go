package gitx

import (
	"context"
	"strings"
)

// WorktreeInfo 是一条 git worktree 条目（对齐 cc-pane WorktreeInfo）。
//   - Branch：去掉 refs/heads/ 前缀的分支名；detached / bare 无分支时为 ""。
//   - Commit：HEAD 提交的 7 位短哈希（与 cc-pane 一致）。
//   - IsMain：是否主工作树。
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	IsMain bool   `json:"isMain"`
}

// Worktrees 列出本仓库的所有 worktree（含主工作树）。走 CLI `worktree list --porcelain`
// （go-git 的 worktree 仍是实验性，D4 决定 CLI-only）。
//
// IsMain 判定：`git worktree list` **始终把主工作树列在首位**（git 的文档化行为），
// 故以「首条 == 主」标注——比 cc-pane 的「路径字符串相等」更健壮（后者在路径大小写/
// 分隔符/尾斜杠不一致时会误判）。这是相对 cc-pane 的有意偏离。
func (r *Repo) Worktrees(ctx context.Context) ([]WorktreeInfo, error) {
	stdout, _, err := runGit(ctx, r.path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(stdout), nil
}

// parseWorktreeList 解析 `worktree list --porcelain` 输出：以空行分块，每块含
// `worktree <path>` / `HEAD <sha>` / `branch refs/heads/<name>`（detached/bare 块无
// branch 行）。末块即使无结尾空行也 flush。首条标 IsMain。
func parseWorktreeList(output string) []WorktreeInfo {
	out := make([]WorktreeInfo, 0)
	var path, commit, branch string
	flush := func() {
		if path == "" {
			return
		}
		short := commit
		if len(short) > 7 {
			short = short[:7]
		}
		out = append(out, WorktreeInfo{
			Path:   path,
			Branch: branch,
			Commit: short,
			IsMain: len(out) == 0, // 首条即主工作树（git 保证主树列首位）
		})
		path, commit, branch = "", "", ""
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush() // 遇到新块起始，先 flush 上一块（兼容无空行分隔的边界）
			path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			commit = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			b := strings.TrimPrefix(line, "branch ")
			branch = strings.TrimPrefix(b, "refs/heads/")
		case line == "":
			flush()
		}
	}
	flush()
	return out
}

// WorktreeAdd 在 worktreePath 处新建一个 worktree。newBranch 非空则 `-b <newBranch>`
// （创建并检出该新分支）；为空则 `git worktree add <path>`（git 默认按路径末段名建分支）。
// 注入 GIT_LFS_SKIP_SMUDGE=1（对齐 cc-pane，避免检出触发 LFS smudge）。
//
// 注意（层次）：cc-pane 的「分组目录策略」（{parent}/{repo}-worktrees/{name}）与
// worktree 名称安全校验属上层 UX 决策，留给 gitsvc 服务层；本原语只接收一个**已算好的
// 绝对路径**。调用方须保证 worktreePath 为绝对路径、newBranch 不以 '-' 开头（避免被
// 当作 git 选项）——procx 按参数数组传入无 shell 注入面，但仍需防选项注入。
func (r *Repo) WorktreeAdd(ctx context.Context, worktreePath, newBranch string) error {
	args := []string{"worktree", "add"}
	if newBranch != "" {
		args = append(args, "-b", newBranch)
	}
	// `--` 终止选项解析（实测 git worktree add 支持），护住以 '-' 开头的 worktreePath
	// 不被当作 git 选项——纵深防御，与服务层的路径/名称校验互补。
	args = append(args, "--", worktreePath)
	_, _, err := runGitEnv(ctx, r.path, []string{"GIT_LFS_SKIP_SMUDGE=1"}, args...)
	return err
}

// WorktreeRemove 删除 worktreePath 处的 worktree。恒带 --force（对齐 cc-pane：
// 即使工作树有未提交改动 / 已锁定也强制移除）。
func (r *Repo) WorktreeRemove(ctx context.Context, worktreePath string) error {
	_, _, err := runGit(ctx, r.path, "worktree", "remove", "--force", worktreePath)
	return err
}
