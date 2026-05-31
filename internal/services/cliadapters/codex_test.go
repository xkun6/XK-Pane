package cliadapters

import (
	"slices"
	"strings"
	"testing"
)

// 守门：MCP 覆盖用 codex 的 dotted-key 形式，含空格的 server 名加 JSON 引号、值 JSON 编码。
func TestMcpOverridesUseCodexTomlDottedKeys(t *testing.T) {
	var args []string
	args = pushMcpURLOverride(args, "context 7", "http://127.0.0.1:3100/mcp")
	args = pushMcpBearerEnvOverride(args, "panes", "XKPANE_API_TOKEN")
	args = pushMcpEnabledOverride(args, "panes", false)

	want := []string{
		"-c", `mcp_servers."context 7".url="http://127.0.0.1:3100/mcp"`,
		"-c", `mcp_servers.panes.bearer_token_env_var="XKPANE_API_TOKEN"`,
		"-c", "mcp_servers.panes.enabled=false",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args =\n%v\nwant\n%v", args, want)
	}
}

// 守门：orchestrator 就绪时 pushMcpOverrides 的 panes url 自报 caller_tool=codex（同源继承用）。
func TestPushMcpOverridesSelfReportsCallerTool(t *testing.T) {
	args := pushMcpOverrides(nil, &Context{OrchestratorPort: 5000, OrchestratorToken: "tok"})
	found := false
	for i := 0; i+1 < len(args); i += 2 {
		if strings.Contains(args[i+1], "mcp_servers.panes.url=") && strings.Contains(args[i+1], "caller_tool=codex") {
			found = true
		}
	}
	if !found {
		t.Fatalf("panes url 应含 caller_tool=codex，args = %v", args)
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
	configured := []string{"panes", "fetch", "chrome-devtools-windows", "Desktop Commander"}
	args := pushMcpIsolationOverridesForNames(nil, []string{"panes", "fetch"}, configured)

	want := []string{
		"-c", `mcp_servers."Desktop Commander".enabled=false`,
		"-c", "mcp_servers.chrome-devtools-windows.enabled=false",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args =\n%v\nwant\n%v", args, want)
	}
}

// 守门（回归）：SkipMCP 时 BuildCommand 不得产出任何 panes 覆盖。
// 历史 bug：注入 `-c mcp_servers.panes.enabled=false` 会让 codex 0.134.0 凭空造出无 transport 的表，
// 启动即 "invalid transport in mcp_servers.panes"。orchestrator 未就绪即 SkipMCP，必须保证零注入。
func TestBuildCommandSkipMCPOmitsPanesOverride(t *testing.T) {
	if _, err := resolveSimple("codex"); err != nil {
		t.Skip("codex 不在 PATH，跳过（BuildCommand 需解析可执行文件）")
	}
	res, err := NewCodex().BuildCommand(&Context{SkipMCP: true, ProjectPath: "/proj"})
	if err != nil {
		t.Fatalf("BuildCommand error: %v", err)
	}
	for _, a := range res.Args {
		if strings.Contains(a, "panes") {
			t.Fatalf("SkipMCP 时不应出现 panes 覆盖，args = %v", res.Args)
		}
	}
}
