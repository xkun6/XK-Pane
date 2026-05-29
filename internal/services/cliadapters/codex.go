package cliadapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CodexAdapter 适配 Codex CLI（codex）。MCP 经 per-launch `-c` dotted-key 覆盖注入（不写用户全局
// config.toml）；system-prompt 走 developer_instructions 覆盖；Resume 用 `codex resume <id>`。
type CodexAdapter struct{ base }

// NewCodex 构造 Codex 适配器。
func NewCodex() *CodexAdapter {
	return &CodexAdapter{base{
		info: ToolInfo{
			ID:          "codex",
			DisplayName: "Codex CLI",
			Executable:  "codex",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsProvider:        true,
			SupportsResume:          true,
			SupportsMCP:             true,
			SupportsSystemPrompt:    true,
			SupportsWorkspace:       true,
			SupportsProjectHooks:    true,
			CompatibleProviderTypes: []string{"open_ai", "config_profile"},
		},
	}}
}

// BuildCommand 拼装 codex 启动命令。顺序：MCP -c 覆盖 → developer_instructions →
// MCP 隔离覆盖 → `resume <id>` → [PROMPT]（位置参数在所有 -c 之后）。
func (a *CodexAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveSimple("codex")
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.SkipMCP {
		// skip_mcp 时显式禁用 ccpanes，避免继承用户全局 config.toml 里的同名项。
		args = pushMcpEnabledOverride(args, "ccpanes", false)
	} else {
		args = pushMcpOverrides(args, ctx)
	}
	if ctx.AppendSystemPrompt != "" {
		args = pushDeveloperInstructionsOverride(args, ctx.AppendSystemPrompt)
	}
	args = pushMcpIsolationOverrides(args, ctx)
	if ctx.ResumeID != "" {
		args = append(args, "resume", ctx.ResumeID)
	}
	if ctx.InitialPrompt != "" {
		args = append(args, ctx.InitialPrompt)
	}

	return CommandResult{
		Command:   path,
		Args:      args,
		EnvInject: map[string]string{},
	}, nil
}

// ── MCP -c 覆盖辅助（纯函数，守门测试覆盖）──────────────────────────────

// pushMcpOverrides 注入 ccpanes 端点 + 共享 MCP（仅在 orchestrator port/token 就绪时，Phase 5）。
func pushMcpOverrides(args []string, ctx *Context) []string {
	if ctx.OrchestratorPort != 0 && ctx.OrchestratorToken != "" {
		url := fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", ctx.OrchestratorPort, ctx.OrchestratorToken)
		args = pushMcpURLOverride(args, "ccpanes", url)
		args = pushMcpBearerEnvOverride(args, "ccpanes", "CC_PANES_API_TOKEN")
		args = pushMcpEnabledOverride(args, "ccpanes", true)
	}
	for name, url := range ctx.SharedMCPURLs {
		args = pushMcpURLOverride(args, name, url)
	}
	return args
}

func pushMcpURLOverride(args []string, name, url string) []string {
	return append(args, "-c", fmt.Sprintf("mcp_servers.%s.url=%s",
		tomlKeySegmentForCLI(name), tomlStringForCLI(url)))
}

func pushMcpBearerEnvOverride(args []string, name, envVar string) []string {
	return append(args, "-c", fmt.Sprintf("mcp_servers.%s.bearer_token_env_var=%s",
		tomlKeySegmentForCLI(name), tomlStringForCLI(envVar)))
}

func pushMcpEnabledOverride(args []string, name string, enabled bool) []string {
	return append(args, "-c", fmt.Sprintf("mcp_servers.%s.enabled=%t",
		tomlKeySegmentForCLI(name), enabled))
}

func pushDeveloperInstructionsOverride(args []string, prompt string) []string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return args
	}
	return append(args, "-c", "developer_instructions="+tomlStringForCLI(prompt))
}

// pushMcpIsolationOverrides 在 DisableUnlistedMCPServers 时，把不在 allowlist 的已配置 MCP 显式禁用。
// Phase 1 默认关闭，恒不动 args。
func pushMcpIsolationOverrides(args []string, ctx *Context) []string {
	if !ctx.DisableUnlistedMCPServers {
		return args
	}
	// TODO(phase5): 读 ~/.codex/config.toml 取已配置 server 名（需引入 TOML 解析）。
	return pushMcpIsolationOverridesForNames(args, ctx.AllowedMCPServerIDs, configuredMCPServerNames())
}

// configuredMCPServerNames 是 Phase 5 hook 点：解析 ~/.codex/config.toml 的 [mcp_servers]。
// Phase 1 返回空（DisableUnlistedMCPServers 默认关，此函数不被实际调用）。
func configuredMCPServerNames() []string { return nil }

// pushMcpIsolationOverridesForNames 对 (configured ∪ allowed ∪ {ccpanes}) 中不在 allowed 的名字
// 逐个 push enabled=false。按名字排序输出（对应 cc-pane 的 BTreeSet 有序遍历）。
func pushMcpIsolationOverridesForNames(args []string, allowed []string, configured []string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}
	set := make(map[string]bool)
	for _, c := range configured {
		set[c] = true
	}
	for _, a := range allowed {
		set[a] = true
	}
	set["ccpanes"] = true

	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names) // byte-wise，匹配 Rust BTreeSet 顺序
	for _, n := range names {
		if !allowedSet[n] {
			args = pushMcpEnabledOverride(args, n, false)
		}
	}
	return args
}

// ── Codex TOML CLI 值编码 ────────────────────────────────────────────

// isSimpleTomlKeySegment：非空且仅含 ASCII 字母数字 / '_' / '-'。
func isSimpleTomlKeySegment(seg string) bool {
	if seg == "" {
		return false
	}
	for _, ch := range seg {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '_', ch == '-':
		default:
			return false
		}
	}
	return true
}

// tomlKeySegmentForCLI：简单段原样，否则 JSON 引号包裹（codex 接受 JSON 风格的带引号 key 段）。
func tomlKeySegmentForCLI(seg string) string {
	if isSimpleTomlKeySegment(seg) {
		return seg
	}
	return tomlStringForCLI(seg)
}

// tomlStringForCLI：把字符串编码为 JSON 字符串字面量（与 cc-pane serde_json::to_string 一致，
// 不做 HTML 转义，以匹配 `"<url>"` 这类含特殊字符的值）。
func tomlStringForCLI(value string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return `""`
	}
	return strings.TrimRight(b.String(), "\n")
}
