package cliadapters

import (
	"runtime"
	"time"
)

// ReclaudeAdapter 适配 Reclaude CLI（reclaude）。与 Claude Code 完全共享配置/hooks/MCP/全局命令目录
// 及 npm 包（@anthropic-ai/claude-code），仅可执行名换成 reclaude；启动参数与 ClaudeAdapter 一致。
type ReclaudeAdapter struct{ base }

// NewReclaude 构造 Reclaude 适配器。
func NewReclaude() *ReclaudeAdapter {
	return &ReclaudeAdapter{base{
		info: ToolInfo{
			ID:          "reclaude",
			DisplayName: "Reclaude",
			Executable:  "reclaude",
			VersionArgs: []string{"version"}, // 注意：reclaude 用 `version` 子命令，非 --version
		},
		caps: Capabilities{
			SupportsProvider:     true,
			SupportsResume:       true,
			SupportsMCP:          true,
			SupportsSystemPrompt: true,
			SupportsWorkspace:    true,
			SupportsProjectHooks: true,
			CompatibleProviderTypes: []string{
				"anthropic", "bedrock", "vertex", "proxy", "config_profile",
			},
		},
	}}
}

// Detect 覆盖默认实现，走 reclaude 专用路径解析（PATH + Windows npm/scoop/winget 回退）。
func (a *ReclaudeAdapter) Detect() ToolInfo {
	out := a.info
	if path, err := resolveClaudeLike("reclaude"); err == nil {
		out.Installed = true
		out.Path = path
		out.Version = probeVersion(path, a.info.VersionArgs, 5*time.Second)
	} else {
		out.Installed = false
	}
	c := a.caps
	out.Capabilities = &c
	return out
}

// BuildCommand 与 ClaudeAdapter 同构（参数顺序与 env 一致），仅可执行名为 reclaude。
func (a *ReclaudeAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveClaudeLike("reclaude")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.ResumeID != "" {
		args = append(args, "--resume", ctx.ResumeID)
	}
	args = append(args, buildAddDirArgs(ctx)...)
	if !ctx.SkipMCP {
		if cfg := generateClaudeMCPConfig(ctx); cfg != "" {
			args = append(args, "--mcp-config", cfg)
		}
	}
	if ctx.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", ctx.AppendSystemPrompt)
	}
	if ctx.InitialPrompt != "" {
		args = append(args, "--", ctx.InitialPrompt)
	}

	command := path
	if runtime.GOOS == "windows" {
		command, args = windowsNpmShimInvocation(path, args)
	}

	// CC_PANES_* 会话/orchestrator env 由 terminal.buildEnv 统一注入（对齐 cc-pane terminal_service）。
	return CommandResult{
		Command:   command,
		Args:      args,
		EnvRemove: []string{"CLAUDECODE"},
	}, nil
}
