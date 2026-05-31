package cliadapters

import (
	"runtime"
	"time"
)

// ClaudeAdapter 适配 Claude Code CLI（claude）。多目录、Resume、MCP、system-prompt、项目 hooks 全支持。
type ClaudeAdapter struct{ base }

// NewClaude 构造 Claude Code 适配器。
func NewClaude() *ClaudeAdapter {
	return &ClaudeAdapter{base{
		info: ToolInfo{
			ID:          "claude",
			DisplayName: "Claude Code",
			Executable:  "claude",
			VersionArgs: []string{"--version"},
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

// Detect 覆盖默认实现，走 claude 专用路径解析（PATH + Windows npm/scoop/winget 回退）。
func (a *ClaudeAdapter) Detect() ToolInfo {
	out := a.info
	if path, err := resolveClaudeLike("claude"); err == nil {
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

// BuildCommand 拼装 claude 启动命令。参数顺序：--resume → --add-dir → --mcp-config →
// --append-system-prompt → `-- <prompt>`（位置参数必须在所有 flag 之后，`--` 防误解析）。
func (a *ClaudeAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveClaudeLike("claude")
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

	// XKPANE_* 会话/orchestrator env 由 terminal.buildEnv 统一注入（对齐 cc-pane terminal_service）。
	return CommandResult{
		Command:   command,
		Args:      args,
		EnvRemove: []string{"CLAUDECODE"},
	}, nil
}
