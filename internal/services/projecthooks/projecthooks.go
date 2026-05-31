// Package projecthooks 把 XK 的 cli-hook 二进制注册进【被启动项目】的 CLI hook 配置，
// 让在 pane 内运行的 Claude/Codex 在生命周期事件 spawn xkpane-cli-hook（移植 cc-pane
// cc-cli-adapters 的 sync_project_hooks + ProjectCliHooksService）。
//
// 落点（D6d-5 拍板）：
//   - claude / reclaude → <proj>/.claude/settings.local.json 的 hooks 块
//   - codex            → <proj>/.codex/hooks.json + <proj>/.codex/config.toml（[features] codex_hooks=true）
//
// 偏好（D6d-2 拍板）：缺省全启用，本期不做 toggle UI / 偏好文件——启动时自动注册全部支持的 hook。
//
// 触发时机：terminal.CreateSession 启动 CLI 前调 Sync（failure 仅 warn，绝不阻断启动）。
package projecthooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// hookBinaryBaseName 是 cli-hook 二进制名（不含扩展名）。也是判定「本工具托管的 hook 条目」的标记串。
const hookBinaryBaseName = "xkpane-cli-hook"

// hookDef 描述一条要注册的 hook（事件 / 匹配器 / 超时 / 子命令）。
type hookDef struct {
	subcommand string // cli-hook 子命令
	event      string // CLI hook 事件名（SessionStart / PostToolUse …）
	matcher    string // 匹配器（claude/codex 各自语义）
	timeout    int    // 秒
}

// claudeHookDefs 是 claude/reclaude 注册的 hook（im-forward 已按 D6d-3 砍掉）。
var claudeHookDefs = []hookDef{
	{subcommand: "session-start", event: "SessionStart", matcher: "startup", timeout: 10},
	{subcommand: "plan-archive", event: "PostToolUse", matcher: "", timeout: 5},
}

// codexHookDefs 是 codex 注册的 hook：仅 session-start（plan-archive 在 codex 不支持）。
var codexHookDefs = []hookDef{
	{subcommand: "session-start", event: "SessionStart", matcher: "startup|resume", timeout: 10},
}

// Sync 据 cliTool 把 cli-hook 注册进被启动项目的 hook 配置。
// cliTool ∈ {claude, reclaude, codex}；其余（none / 未知）直接 no-op。
// 找不到 hook 二进制时返回 error（调用方仅 warn）。
func Sync(projectPath, cliTool string) error {
	if projectPath == "" {
		return nil
	}
	switch cliTool {
	case "claude", "reclaude":
		bin, err := resolveHookBinary()
		if err != nil {
			return err
		}
		return syncClaudeHooks(projectPath, bin)
	case "codex":
		bin, err := resolveHookBinary()
		if err != nil {
			return err
		}
		return syncCodexHooks(projectPath, bin)
	default:
		return nil
	}
}

// resolveHookBinary 定位 xkpane-cli-hook 可执行文件：
// env XKPANE_CLI_HOOK_BINARY 覆盖 → 主程序同目录 → 同目录 binaries/ → 兜底 PATH 同名。
// 对齐 cc-pane ProjectCliHooksService::get_hook_binary_path（去掉 cargo target 分支，XK 走 Taskfile）。
func resolveHookBinary() (string, error) {
	name := hookBinaryBaseName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	if explicit := strings.TrimSpace(os.Getenv("XKPANE_CLI_HOOK_BINARY")); explicit != "" {
		if fileExists(explicit) {
			return explicit, nil
		}
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if c := filepath.Join(dir, name); fileExists(c) {
			return c, nil
		}
		if c := filepath.Join(dir, "binaries", name); fileExists(c) {
			return c, nil
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("%s 未找到（请先 task build:hook 编出二进制到 bin/）", name)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
