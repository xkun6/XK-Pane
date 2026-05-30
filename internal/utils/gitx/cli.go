package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"xkpane/internal/utils/procx"
)

// allowedSubcmds 是 runGit 允许的 git 子命令白名单。仍走此单一入口（不放开任意
// 子命令）；引入新操作时显式追加。当前含：只读核心（status/rev-parse/branch/log/
// for-each-ref）+ CLI-only 变更操作（worktree/stash —— go-git 弱项/无对应，对齐
// cc-pane 100% CLI）。
var allowedSubcmds = map[string]bool{
	"status":       true,
	"rev-parse":    true,
	"branch":       true,
	"log":          true,
	"for-each-ref": true,
	"worktree":     true,
	"stash":        true,
	"clone":        true,
	"fetch":        true,
	"pull":         true,
	"push":         true,
}

// runGit 经 procx 同步执行 git 子命令（go-git 无对应/弱项时的 CLI 回退）。
// 仅允许 allowlist 内子命令；参数为本包内部固定值（非用户拼接），无注入面。
// 注入 GIT_TERMINAL_PROMPT=0 屏蔽交互式鉴权弹窗（GUI 子进程铁律）。
// 非零退出 → 返回含 stderr（空则 stdout）的错误；spawn 失败（git 不在 PATH）→ 透传。
//
// 并发安全：procx.Run 在返回前 wait 了 stdout/stderr 扫描 goroutine，故 Run 返回后
// 读 builder 无竞态；stdout/stderr 各由单一 goroutine 写不同 builder。
func runGit(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	return runGitEnv(ctx, dir, nil, args...)
}

// runGitEnv 同 runGit，但在标准 env 之上追加 extraEnv（如 worktree add 的
// GIT_LFS_SKIP_SMUDGE=1，避免在新工作树检出时触发 LFS smudge）。
func runGitEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (stdout, stderr string, err error) {
	if len(args) == 0 || !allowedSubcmds[args[0]] {
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		return "", "", errors.New("gitx: git subcommand not allowed: " + sub)
	}

	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	env = append(env, extraEnv...)

	var sb, eb strings.Builder
	spec := procx.Spec{
		Name:          "git",
		Args:          args,
		Dir:           dir,
		Env:           env,
		CaptureOutput: true,
		OnStdout: func(line string) {
			sb.WriteString(line)
			sb.WriteByte('\n')
		},
		OnStderr: func(line string) {
			eb.WriteString(line)
			eb.WriteByte('\n')
		},
	}

	runErr := procx.Run(ctx, spec)
	stdout, stderr = sb.String(), eb.String()
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			msg := strings.TrimSpace(stderr)
			if msg == "" {
				msg = strings.TrimSpace(stdout)
			}
			return stdout, stderr, errors.New("gitx: git " + args[0] + " failed: " + msg)
		}
		return stdout, stderr, runErr // spawn 失败（如 git 不在 PATH）
	}
	return stdout, stderr, nil
}
