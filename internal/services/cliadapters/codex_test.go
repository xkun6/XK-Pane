package cliadapters

import (
	"slices"
	"testing"
)

// 守门：MCP 覆盖用 codex 的 dotted-key 形式，含空格的 server 名加 JSON 引号、值 JSON 编码。
func TestMcpOverridesUseCodexTomlDottedKeys(t *testing.T) {
	var args []string
	args = pushMcpURLOverride(args, "context 7", "http://127.0.0.1:3100/mcp")
	args = pushMcpBearerEnvOverride(args, "ccpanes", "CC_PANES_API_TOKEN")
	args = pushMcpEnabledOverride(args, "ccpanes", false)

	want := []string{
		"-c", `mcp_servers."context 7".url="http://127.0.0.1:3100/mcp"`,
		"-c", `mcp_servers.ccpanes.bearer_token_env_var="CC_PANES_API_TOKEN"`,
		"-c", "mcp_servers.ccpanes.enabled=false",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args =\n%v\nwant\n%v", args, want)
	}
}

// 守门：append_system_prompt 走 developer_instructions 的 -c 覆盖（值 JSON 编码）。
func TestDeveloperInstructionsOverrideUsesCodexCliConfig(t *testing.T) {
	args := pushDeveloperInstructionsOverride(nil, "CC-Panes launch profile skill")
	want := []string{"-c", `developer_instructions="CC-Panes launch profile skill"`}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

// 守门：MCP 隔离把 allowlist 之外的已配置 server 显式 disabled，按名字排序输出。
func TestMcpIsolationDisablesConfiguredServersOutsideAllowlist(t *testing.T) {
	configured := []string{"ccpanes", "fetch", "chrome-devtools-windows", "Desktop Commander"}
	args := pushMcpIsolationOverridesForNames(nil, []string{"ccpanes", "fetch"}, configured)

	want := []string{
		"-c", `mcp_servers."Desktop Commander".enabled=false`,
		"-c", "mcp_servers.chrome-devtools-windows.enabled=false",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args =\n%v\nwant\n%v", args, want)
	}
}
