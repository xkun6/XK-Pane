package cliadapters

import (
	"encoding/json"
	"path/filepath"

	"xkpane/internal/utils/filex"
)

const defaultKimiBaseURL = "https://api.moonshot.cn/v1"

// KimiAdapter 适配 Kimi CLI（kimi）。Provider 为 kimi 时写 per-session 配置文件并以 --config-file 传入；
// 支持多目录 --add-dir（铁律）；通过 KIMI_SHARE_DIR 隔离共享目录。
type KimiAdapter struct{ base }

// NewKimi 构造 Kimi 适配器。
func NewKimi() *KimiAdapter {
	return &KimiAdapter{base{
		info: ToolInfo{
			ID:          "kimi",
			DisplayName: "Kimi CLI",
			Executable:  "kimi",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          false,
			SupportsMCP:             false,
			SupportsSystemPrompt:    false,
			SupportsWorkspace:       true,
			SupportsProjectHooks:    false,
			CompatibleProviderTypes: []string{"kimi", "config_profile"},
		},
	}}
}

// writeSessionConfig 在 Provider 为 kimi 且有 api_key 时写 per-session 配置，返回路径；否则返回 ""。
func (a *KimiAdapter) writeSessionConfig(ctx *Context) (string, error) {
	p := ctx.Provider
	if p == nil || p.ProviderType != "kimi" || p.APIKey == "" {
		return "", nil
	}

	configDir := filepath.Join(ctx.DataDir, "cli-adapters", "kimi", "configs")
	if err := filex.EnsureDir(configDir); err != nil {
		return "", err
	}

	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = defaultKimiBaseURL
	}
	config := map[string]any{
		"providers": map[string]any{
			"ccpanes": map[string]any{
				"type":     "kimi",
				"api_key":  p.APIKey,
				"base_url": baseURL,
			},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, ctx.SessionID+".json")
	// 0o600（仅属主）：该文件含 Provider api_key，无法加密（kimi CLI 需明文读），
	// 故以最严文件权限加固（较 cc-pane 的 0o644 更安全的有意取舍）。
	if err := filex.WriteAtomic(configPath, data, 0o600); err != nil {
		return "", err
	}
	return configPath, nil
}

// BuildCommand 拼装 kimi 启动命令：[--config-file <path>] [--add-dir <workspace>] [PROMPT]；
// 注入 KIMI_SHARE_DIR。
func (a *KimiAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("kimi")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	configPath, err := a.writeSessionConfig(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	if configPath != "" {
		args = append(args, "--config-file", configPath)
	}
	args = append(args, buildAddDirArgs(ctx)...)
	if ctx.InitialPrompt != "" {
		args = append(args, ctx.InitialPrompt)
	}

	shareDir := filepath.Join(ctx.DataDir, "cli-adapters", "kimi", "share")
	if err := filex.EnsureDir(shareDir); err != nil {
		return CommandResult{}, err
	}

	return CommandResult{
		Command:   path,
		Args:      args,
		EnvInject: map[string]string{"KIMI_SHARE_DIR": shareDir},
	}, nil
}
