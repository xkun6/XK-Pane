package cliadapters

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"xkpane/internal/utils/filex"
	"xkpane/internal/utils/logx"

	toml "github.com/pelletier/go-toml/v2"
)

// GrokAdapter 适配 Grok Build CLI（grok，xAI 终端编码 agent，Rust 二进制）。全局配置走
// ~/.grok/config.toml（agentcfg 域直写切换），故不接会话级 Provider env 注入；
// Resume 经 `grok --resume <id>`（sessionscan 扫 ~/.grok/sessions）。
//
// panes MCP：与 Claude（--mcp-config 写项目 .claude/mcp-<sid>.json）/ Codex（-c 覆盖）
// 同为【启动时会话级注入】，绝不写 ~/.grok。Grok 无 CLI MCP 覆盖 flag，故写入
// <ProjectPath>/.grok/config.toml 的 [mcp_servers.panes]（项目级优先级高于用户全局，
// 且只碰 panes 键，保留用户其它 project MCP）。会话退出时摘掉 panes 键。
type GrokAdapter struct{ base }

// NewGrok 构造 Grok Build 适配器。
func NewGrok() *GrokAdapter {
	return &GrokAdapter{base{
		info: ToolInfo{
			ID:          "grok",
			DisplayName: "Grok Build",
			Executable:  "grok",
			VersionArgs: []string{"--version"},
		},
		caps: Capabilities{
			SupportsResume: true, // `grok --resume <id>`；会话由 sessionscan 扫 ~/.grok/sessions
			SupportsMCP:    true, // 启动时注入项目级 .grok/config.toml 的 panes
		},
	}}
}

// Detect 覆盖默认实现，走 grok 专用路径解析（PATH + ~/.grok/bin 回退，官方原生安装器默认落点）。
func (a *GrokAdapter) Detect() ToolInfo {
	out := a.info
	if path, err := resolveGrok(); err == nil {
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

// BuildCommand 拼装 grok 启动命令。参数顺序：--resume <id> → [PROMPT]（位置参数在最后）。
// 非 SkipMCP 时把 panes 写入项目 .grok/config.toml（fail-open：写失败仍启动）。
func (a *GrokAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveGrok()
	if err != nil {
		return CommandResult{}, err
	}

	if !ctx.SkipMCP {
		if err := ensureGrokProjectPanesMCP(ctx); err != nil {
			logx.For("cliadapters").Warn("grok panes mcp: inject failed, fail-open", "err", err)
		}
	}

	var args []string
	if ctx.ResumeID != "" {
		args = append(args, "--resume", ctx.ResumeID)
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

// resolveGrok 解析 grok 可执行路径：先 PATH（resolveSimple），失败再回退官方原生安装器的默认
// 安装目录 ~/.grok/bin/grok(.exe)（该安装器会尝试写用户 PATH，但存在未生效场景，如安装后未重启
// 终端），存在则返回该绝对路径，否则返回原 PATH 未找到错误。
func resolveGrok() (string, error) {
	if path, err := resolveSimple("grok"); err == nil {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("grok CLI not found in PATH")
	}
	name := "grok"
	if runtime.GOOS == "windows" {
		name = "grok.exe"
	}
	if candidate := filepath.Join(home, ".grok", "bin", name); isFile(candidate) {
		return candidate, nil
	}
	return "", fmt.Errorf("grok CLI not found in PATH")
}

// ── panes MCP：项目级 .grok/config.toml（不写 ~/.grok）────────────────────────

// grokProjectConfigPath 是项目级 Grok 配置：<project>/.grok/config.toml。
// Grok 文档：cwd/.grok/config.toml 优先级高于 ~/.grok/config.toml，且 [mcp_servers]
// 同名时项目版整体替换用户版——正好用于会话级 panes 覆盖。
func grokProjectConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".grok", "config.toml")
}

// ensureGrokProjectPanesMCP 在 orchestrator 就绪时，把 panes 端点写入
// <ProjectPath>/.grok/config.toml 的 [mcp_servers.panes]。只增改 panes 键，其它
// mcp_servers / 用户段落原样保留。绝不触碰 ~/.grok。
func ensureGrokProjectPanesMCP(ctx *Context) error {
	if ctx.OrchestratorPort == 0 || ctx.OrchestratorToken == "" {
		return nil
	}
	if strings.TrimSpace(ctx.ProjectPath) == "" {
		return fmt.Errorf("empty project path")
	}

	dir := filepath.Join(ctx.ProjectPath, ".grok")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure .grok dir: %w", err)
	}
	path := grokProjectConfigPath(ctx.ProjectPath)

	root := map[string]any{}
	if data, err := filex.ReadLimit(path, 8<<20); err == nil {
		if err := toml.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse project .grok/config.toml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read project .grok/config.toml: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}

	mcpServers, _ := root["mcp_servers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}
	// caller_tool=grok 自报发起方，供 orchestrator launch_task 同源继承。
	url := fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s&caller_tool=grok",
		ctx.OrchestratorPort, ctx.OrchestratorToken)
	mcpServers["panes"] = map[string]any{
		"url":     url,
		"enabled": true,
		"headers": map[string]any{
			"Authorization": "Bearer " + ctx.OrchestratorToken,
		},
	}
	root["mcp_servers"] = mcpServers

	out, err := toml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal project .grok/config.toml: %w", err)
	}
	if err := filex.WriteAtomic(path, out, 0o644); err != nil {
		return fmt.Errorf("write project .grok/config.toml: %w", err)
	}
	return nil
}

// cleanupGrokProjectPanesMCP 从 <project>/.grok/config.toml 摘掉 [mcp_servers.panes]。
// 若摘完后 mcp_servers 为空且文件无其它有意义内容，删除整个文件；文件不存在 no-op。
// 同项目多 grok 会话并发时，后退出者仍会摘 panes——同一 XK-Pane 进程内 token 相同，
// 仍活着的会话不受影响（已在启动时连上）；进程外裸跑 grok 时 panes 失效是预期 fail-open。
func cleanupGrokProjectPanesMCP(projectPath string) {
	if strings.TrimSpace(projectPath) == "" {
		return
	}
	path := grokProjectConfigPath(projectPath)
	data, err := filex.ReadLimit(path, 8<<20)
	if err != nil {
		return // 不存在或不可读 → no-op
	}
	root := map[string]any{}
	if err := toml.Unmarshal(data, &root); err != nil {
		logx.For("cliadapters").Warn("grok panes mcp: parse for cleanup", "path", path, "err", err)
		return
	}
	mcpServers, _ := root["mcp_servers"].(map[string]any)
	if mcpServers == nil {
		return
	}
	if _, ok := mcpServers["panes"]; !ok {
		return
	}
	delete(mcpServers, "panes")
	if len(mcpServers) == 0 {
		delete(root, "mcp_servers")
	} else {
		root["mcp_servers"] = mcpServers
	}

	// 文件空了就删，避免在项目里留一个空 .grok/config.toml。
	if len(root) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logx.For("cliadapters").Warn("grok panes mcp: remove empty config", "path", path, "err", err)
		}
		return
	}
	out, err := toml.Marshal(root)
	if err != nil {
		logx.For("cliadapters").Warn("grok panes mcp: marshal for cleanup", "path", path, "err", err)
		return
	}
	if err := filex.WriteAtomic(path, out, 0o644); err != nil {
		logx.For("cliadapters").Warn("grok panes mcp: write cleanup", "path", path, "err", err)
	}
}
