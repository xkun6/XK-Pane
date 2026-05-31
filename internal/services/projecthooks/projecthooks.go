// Package projecthooks 把 XK 的 cli-hook 二进制注册进【被启动项目】的 CLI hook 配置，
// 让在 pane 内运行的 Claude/Codex 在生命周期事件 spawn xkpane-cli-hook（移植 cc-pane
// cc-cli-adapters 的 sync_project_hooks + ProjectCliHooksService）。
//
// 落点（D6d-5 拍板，据手测 + codex 源码核对调整）：
//   - claude / reclaude → <proj>/.claude/settings.local.json 的 hooks 块
//   - codex            → <proj>/.codex/config.toml 内联 [[hooks.SessionStart]]（[features] hooks=true）
//                         codex command_windows 用 PowerShell 调用运算符形式（见 codex.go / powershellHookCommand）
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

// hookDef 描述一条要注册的 hook（事件 / 匹配器 / 超时 / 子命令 / 是否异步）。
type hookDef struct {
	subcommand string // cli-hook 子命令
	event      string // CLI hook 事件名（SessionStart / PostToolUse …）
	matcher    string // 匹配器（claude/codex 各自语义）
	timeout    int    // 秒
	async      bool   // true=后台跑（只要副作用，stdout 被忽略）；false=同步（stdout 注入上下文）
}

// claudeHookDefs 是 claude/reclaude 注册的 hook（im-forward 已按 D6d-3 砍掉）。
//
// ⚠️ session-start 必须 async=false：SessionStart hook 的 stdout 只有在【同步】执行时才会被
// CLI 注入进模型上下文；async=true 会让 CLI 后台跑并丢弃 stdout（仅保留 POST 副作用），
// 导致 <xkpane-context> 注入失效（实测发现 + claude-code-guide 权威确认）。
// 这是【有意偏离 cc-pane】——cc-pane 给所有 hook 都设 async:true，其 session-start stdout 注入
// 实际不生效（只靠副作用）。plan-archive 是纯副作用（PostToolUse，无 stdout 注入需求），保留 async
// 以免阻塞 Write 工具完成。
var claudeHookDefs = []hookDef{
	{subcommand: "session-start", event: "SessionStart", matcher: "startup", timeout: 10, async: false},
	{subcommand: "plan-archive", event: "PostToolUse", matcher: "", timeout: 5, async: true},
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

// hookCommand 拼 hook 命令字符串：`"<二进制绝对路径>" <子命令>`。
// 刻意【不用 fmt %q】——%q 会把反斜杠转义成 \\，使命令值里出现双反斜杠路径。这里裸路径加双引号
// （Windows 路径不含 "，安全）；落到 JSON 由 json.Marshal 自行转义、落到 TOML 由 go-toml 用 literal
// 串原样保留，运行时还原为单反斜杠合法路径。
//
// 此形式用于：claude（settings.local.json，实测可执行）+ codex 非 Windows 的 command（codex 在
// 非 Windows 经 `sh -lc "<cmd>"` 执行，sh 能执行带引号的路径）。codex 的 Windows 覆盖见
// powershellHookCommand。
func hookCommand(hookBinary, subcommand string) string {
	return `"` + hookBinary + `" ` + subcommand
}

// powershellHookCommand 拼 PowerShell 可执行的命令串：`& "<二进制绝对路径>" <子命令>`。
// 用于 codex 的 command_windows 字段。
//
// 根因（已由 codex 源码 + 本机实测坐实）：codex 在 Windows 上【默认用 PowerShell】执行 hook
// （core/src/shell.rs default_user_shell → PowerShell；session/mod.rs build_hooks_for_config 经
// derive_exec_args 派生 `pwsh -NoProfile -Command "<cmd>"`）。PowerShell 里以引号开头的命令串会
// 被当作【字符串字面量】而非命令调用，后跟参数即触发解析错误（Unexpected token）→ 进程退出码 1
// （即用户实测的 "hook exited with code 1"）。要让 PowerShell 执行带引号的可执行路径，必须前置
// 【调用运算符 &】。这也解释了为何同一二进制 reclaude（claude 不走 PowerShell）跑通而 codex exit 1。
func powershellHookCommand(hookBinary, subcommand string) string {
	return "& " + hookCommand(hookBinary, subcommand)
}
