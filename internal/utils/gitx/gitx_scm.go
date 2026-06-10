package gitx

import (
	"context"
	"strconv"
	"strings"
)

// SCM 原语（VSCode 源代码管理面板）：结构化 per-file 状态 + 工作区↔暂存区 diff +
// stage/unstage/discard/commit。go-git 做不了这些（无 staging area / 工作区↔暂存区
// 文件 diff / commit hooks，见 D4），全走 git CLI（runGit allowlist 已加 diff/add/
// restore/commit）。
//
// 解析选型：`git status --porcelain`（v1，换行分隔）而非 `--porcelain=v2 -z`（NUL 分隔）——
// runGit 走 bufio.Scanner 行扫描，换行分隔天然适配；NUL 流会被整行吞成一条。代价：含特殊
// 字符的路径会被 git C 风格引号包裹（unquotePath best-effort 还原），常见 ASCII 路径不受影响。

// FileStatus 是工作区一个变更文件（解析自 `git status --porcelain` 的一行）。
// Index/Worktree 为 porcelain 的 XY 两位状态码（" " 表示该侧无变更）。
// 一个文件可同时 Staged 且 Unstaged（如 "MM"：暂存了一次修改后又改了），对齐 VSCode。
type FileStatus struct {
	Path       string `json:"path"`               // 仓库相对路径（git 输出原样，正斜杠）
	OrigPath   string `json:"origPath,omitempty"` // 重命名/复制来源（仅 R/C）
	Index      string `json:"index"`              // 暂存区相对 HEAD：M/A/D/R/C/" "
	Worktree   string `json:"worktree"`           // 工作树相对暂存区：M/D/" "/"?"
	Staged     bool   `json:"staged"`
	Unstaged   bool   `json:"unstaged"`
	Untracked  bool   `json:"untracked"`
	Conflicted bool   `json:"conflicted"`
}

// conflictCodes 是 porcelain 未合并（冲突）的 XY 组合。
var conflictCodes = map[string]bool{
	"DD": true, "AU": true, "UD": true, "UA": true, "DU": true, "AA": true, "UU": true,
}

// StatusFiles 解析 `git status --porcelain --untracked-files=all` 为结构化变更列表。
func (r *Repo) StatusFiles(ctx context.Context) ([]FileStatus, error) {
	stdout, _, err := runGit(ctx, r.path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parsePorcelainStatus(stdout), nil
}

// parsePorcelainStatus 是 StatusFiles 的纯解析逻辑（与 git 调用解耦，便于单测）。
func parsePorcelainStatus(stdout string) []FileStatus {
	out := make([]FileStatus, 0)
	for _, line := range strings.Split(stdout, "\n") {
		if len(line) < 4 {
			continue // 空行 / 不完整（XY + 空格 + 至少 1 字符路径）
		}
		xy := line[:2]
		x, y := string(line[0]), string(line[1])
		rest := line[3:] // 跳过 XY 之后的分隔空格

		fs := FileStatus{}
		switch {
		case xy == "??":
			fs.Path = unquotePath(rest)
			fs.Index, fs.Worktree = " ", "?"
			fs.Untracked, fs.Unstaged = true, true
		case conflictCodes[xy]:
			fs.Path = unquotePath(rest)
			fs.Index, fs.Worktree = x, y
			fs.Conflicted, fs.Unstaged = true, true
		default:
			path, orig := rest, ""
			if x == "R" || x == "C" {
				if i := strings.Index(rest, " -> "); i >= 0 {
					orig = unquotePath(rest[:i])
					path = rest[i+4:]
				}
			}
			fs.Path = unquotePath(path)
			fs.OrigPath = orig
			fs.Index, fs.Worktree = x, y
			fs.Staged = x != " "
			fs.Unstaged = y != " "
		}
		out = append(out, fs)
	}
	return out
}

// unquotePath 还原 git 在路径含特殊字符时的 C 风格引号（best-effort；常见 ASCII 路径不被引用）。
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if uq, err := strconv.Unquote(p); err == nil {
			return uq
		}
	}
	return p
}

// DiffFileRaw 返回单文件的 unified diff 文本：
//   - staged=true  → `git diff --cached`（暂存区相对 HEAD）
//   - staged=false → `git diff`（工作树相对暂存区）
//
// --no-color/--no-ext-diff 保证可解析、不触发用户外部 difftool。未跟踪文件无 diff（git
// 对其返回空），由上层据 status 决定是否调用。
func (r *Repo) DiffFileRaw(ctx context.Context, file string, staged bool) (string, error) {
	args := []string{"diff", "--no-color", "--no-ext-diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", file)
	stdout, _, err := runGit(ctx, r.path, args...)
	return stdout, err
}

// Stage 把指定文件加入暂存区（git add -- ...）。`--` 防路径被当作选项。空列表 no-op。
func (r *Repo) Stage(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}
	_, _, err := runGit(ctx, r.path, append([]string{"add", "--"}, files...)...)
	return err
}

// Unstage 撤销暂存（git restore --staged -- ...）。需仓库已有 ≥1 次提交；全新 unborn HEAD
// 仓库的撤销暂存（git rm --cached）属边角，P0 暂不支持。
func (r *Repo) Unstage(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}
	_, _, err := runGit(ctx, r.path, append([]string{"restore", "--staged", "--"}, files...)...)
	return err
}

// RestoreWorktree 丢弃工作树改动（git restore -- ...）；仅对已跟踪文件有效（未跟踪文件
// 由上层 os.Remove 处理）。
func (r *Repo) RestoreWorktree(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}
	_, _, err := runGit(ctx, r.path, append([]string{"restore", "--"}, files...)...)
	return err
}

// Commit 提交暂存区内容（git commit -m）。无暂存内容 / 空消息时 git 报错透传。
func (r *Repo) Commit(ctx context.Context, message string) (string, error) {
	stdout, _, err := runGit(ctx, r.path, "commit", "-m", message)
	return stdout, err
}

// CommitPaths 仅提交指定路径的当前工作树内容（git commit -m <msg> -- <paths...>），
// 不携带暂存区里的其它内容——供自动化提交（如 worktreesvc 补 .gitignore）使用，
// 绝不把用户已暂存的无关改动一并带走。空列表 no-op。
func (r *Repo) CommitPaths(ctx context.Context, message string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	args := append([]string{"commit", "-m", message, "--"}, paths...)
	stdout, _, err := runGit(ctx, r.path, args...)
	return stdout, err
}
