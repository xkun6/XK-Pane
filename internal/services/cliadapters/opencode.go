package cliadapters

// OpenCodeAdapter 适配 OpenCode（opencode）。仅把初始 prompt 作为位置参数传入，无额外 flag/env。
type OpenCodeAdapter struct{ base }

// NewOpenCode 构造 OpenCode 适配器。
func NewOpenCode() *OpenCodeAdapter {
	return &OpenCodeAdapter{base{
		info: ToolInfo{
			ID:          "opencode",
			DisplayName: "OpenCode",
			Executable:  "opencode",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          false,
			SupportsMCP:             false,
			SupportsSystemPrompt:    false,
			SupportsWorkspace:       false,
			SupportsProjectHooks:    false,
			CompatibleProviderTypes: []string{"open_ai", "opencode", "anthropic", "config_profile"},
		},
	}}
}

// BuildCommand 拼装 opencode 启动命令：[PROMPT]。
func (a *OpenCodeAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("opencode")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.InitialPrompt != "" {
		args = append(args, ctx.InitialPrompt)
	}

	return CommandResult{Command: path, Args: args, EnvInject: map[string]string{}}, nil
}
