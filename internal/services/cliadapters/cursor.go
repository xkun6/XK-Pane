package cliadapters

// CursorAdapter 适配 Cursor CLI（可执行 cursor-agent）。支持 Resume；Provider 为 cursor 时注入 CURSOR_API_KEY。
type CursorAdapter struct{ base }

// NewCursor 构造 Cursor 适配器。
func NewCursor() *CursorAdapter {
	return &CursorAdapter{base{
		info: ToolInfo{
			ID:          "cursor",
			DisplayName: "Cursor CLI",
			Executable:  "cursor-agent",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          true,
			SupportsMCP:             false,
			SupportsSystemPrompt:    false,
			SupportsWorkspace:       false,
			SupportsProjectHooks:    false,
			CompatibleProviderTypes: []string{"cursor"},
		},
	}}
}

// BuildCommand 拼装 cursor-agent 启动命令：[--resume <id>] [PROMPT]。
func (a *CursorAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("cursor-agent")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.ResumeID != "" {
		args = append(args, "--resume", ctx.ResumeID)
	}
	if ctx.InitialPrompt != "" {
		args = append(args, ctx.InitialPrompt)
	}

	envInject := map[string]string{}
	if p := ctx.Provider; p != nil && p.ProviderType == "cursor" && p.APIKey != "" {
		envInject["CURSOR_API_KEY"] = p.APIKey
	}

	return CommandResult{Command: path, Args: args, EnvInject: envInject}, nil
}
