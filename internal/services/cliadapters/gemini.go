package cliadapters

// GeminiAdapter 适配 Gemini CLI（gemini）。仅把初始 prompt 作为位置参数传入，无额外 flag/env。
type GeminiAdapter struct{ base }

// NewGemini 构造 Gemini 适配器。
func NewGemini() *GeminiAdapter {
	return &GeminiAdapter{base{
		info: ToolInfo{
			ID:          "gemini",
			DisplayName: "Gemini CLI",
			Executable:  "gemini",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          false,
			SupportsMCP:             false,
			SupportsSystemPrompt:    false,
			SupportsWorkspace:       false,
			SupportsProjectHooks:    false,
			CompatibleProviderTypes: []string{"gemini", "config_profile"},
		},
	}}
}

// BuildCommand 拼装 gemini 启动命令：[PROMPT]。
func (a *GeminiAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("gemini")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.InitialPrompt != "" {
		args = append(args, ctx.InitialPrompt)
	}

	return CommandResult{Command: path, Args: args, EnvInject: map[string]string{}}, nil
}
