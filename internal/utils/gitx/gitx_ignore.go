package gitx

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// IsPathIgnored 判定 relPath（仓库相对路径，如 ".claude"）当前是否会被 git 忽略——
// 综合 .gitignore / .git/info/exclude / 全局 excludesfile 所有来源（这正是不直接读
// .gitignore 文本的原因：用户可能在别处已忽略，重复追加是噪声）。
// 底层 `git check-ignore -q`：退出码 0=忽略、1=不忽略、其余=出错。
// 路径不需要真实存在（check-ignore 按规则评估 pathspec，与磁盘无关）。
func (r *Repo) IsPathIgnored(ctx context.Context, relPath string) (bool, error) {
	code, _, stderr, err := runGitCode(ctx, r.path, "check-ignore", "-q", "--", relPath)
	if err != nil {
		return false, err
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = "exit code " + strconv.Itoa(code)
		}
		return false, errors.New("gitx: git check-ignore failed: " + msg)
	}
}

// TopLevel 返回工作树根目录的绝对路径（git rev-parse --show-toplevel）。
// 从仓库子目录打开时也返回根（.gitignore / worktree 路径 hash 都以根为锚）。
// 在 worktree 内调用返回该 worktree 自己的根（git 文档化行为）。
func (r *Repo) TopLevel(ctx context.Context) (string, error) {
	stdout, _, err := runGit(ctx, r.path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// HasTrackedFiles 判定 relPath（目录或文件）下是否存在被 git 跟踪的文件
// （git ls-files 有输出即有）。供 worktreesvc 守门：目录内容已入库的仓库
// 不自动追加 ignore（gitignore 不影响已跟踪文件，但会让后续新增文件被静默漏收）。
func (r *Repo) HasTrackedFiles(ctx context.Context, relPath string) (bool, error) {
	stdout, _, err := runGit(ctx, r.path, "ls-files", "--", relPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) != "", nil
}

// PathHasChanges 判定指定路径相对 HEAD 是否有未提交改动（工作树或暂存区，含未跟踪；
// git status --porcelain -- <path> 非空即有）。
func (r *Repo) PathHasChanges(ctx context.Context, relPath string) (bool, error) {
	stdout, _, err := runGit(ctx, r.path, "status", "--porcelain", "--", relPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) != "", nil
}

// BranchNamesCLI 经 CLI（for-each-ref refs/heads）列本地分支名。与 go-git 版 Branches()
// 的差别：在 linked worktree 内打开仓库时 go-git（未开 EnableDotGitCommonDir）看不到
// 共享 refs 而返回空；CLI 始终读 commondir，结果正确——分支占用判定必须用本方法。
func (r *Repo) BranchNamesCLI(ctx context.Context) ([]string, error) {
	stdout, _, err := runGit(ctx, r.path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}
