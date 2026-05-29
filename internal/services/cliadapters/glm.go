package cliadapters

import (
	"path/filepath"

	"xkpane/internal/utils/filex"
)

// GlmAdapter 适配 GLM CLI —— 底层可执行是 crush（非 glm）。不支持 Resume-as-flag 外的 MCP/system-prompt；
// 通过 CRUSH_GLOBAL_CONFIG/DATA 隔离配置，Provider 为 glm 时注入 ZAI_API_KEY/ZAI_BASE_URL。
type GlmAdapter struct{ base }

// NewGlm 构造 GLM 适配器（executable = crush）。
func NewGlm() *GlmAdapter {
	return &GlmAdapter{base{
		info: ToolInfo{
			ID:          "glm",
			DisplayName: "GLM CLI",
			Executable:  "crush",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          false,
			SupportsMCP:             false,
			SupportsSystemPrompt:    false,
			SupportsWorkspace:       true,
			SupportsProjectHooks:    false,
			CompatibleProviderTypes: []string{"glm", "config_profile"},
		},
	}}
}

// glmLaunchCwd 透传给 crush 的 --cwd：与 terminal_service 决策一致，始终 = ProjectPath（铁律）。
func glmLaunchCwd(ctx *Context) string { return ctx.ProjectPath }

// BuildCommand 拼装 crush 启动命令：--cwd <project> --data-dir <data> [--session <resume>] [run <prompt>]。
// 同时准备隔离的 config/data 目录与空 crush.json。
func (a *GlmAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("crush")
	if err != nil {
		return CommandResult{}, err
	}

	adapterRoot := filepath.Join(ctx.DataDir, "cli-adapters", "glm")
	configPath := filepath.Join(adapterRoot, "crush.json")
	dataPath := filepath.Join(adapterRoot, "data")

	if err := filex.EnsureDir(dataPath); err != nil {
		return CommandResult{}, err
	}
	if !isFile(configPath) {
		if err := filex.WriteAtomic(configPath, []byte("{}\n"), 0o644); err != nil {
			return CommandResult{}, err
		}
	}

	envInject := map[string]string{
		"CRUSH_GLOBAL_CONFIG": configPath,
		"CRUSH_GLOBAL_DATA":   dataPath,
	}
	if p := ctx.Provider; p != nil && p.ProviderType == "glm" {
		if p.APIKey != "" {
			envInject["ZAI_API_KEY"] = p.APIKey
		}
		if p.BaseURL != "" {
			envInject["ZAI_BASE_URL"] = p.BaseURL
		}
	}

	args := []string{"--cwd", glmLaunchCwd(ctx), "--data-dir", dataPath}
	if ctx.ResumeID != "" {
		args = append(args, "--session", ctx.ResumeID)
	}
	if ctx.InitialPrompt != "" {
		args = append(args, "run", ctx.InitialPrompt)
	}

	return CommandResult{Command: path, Args: args, EnvInject: envInject}, nil
}
