// Package cliadapters 移植 cc-pane 的 CLI 工具适配器层（cc-cli-adapters crate）。
//
// 每个受支持的命令行 Agent（Claude Code / Codex / Gemini / Kimi / GLM / OpenCode /
// Cursor / Reclaude）实现 Adapter 接口，提供元信息、能力声明与「启动命令拼装」
// （BuildCommand）。Phase 2 的终端服务据此在 PTY 里以选定 Provider 启动 CLI。
//
// 移植范围（Phase 1）与刻意延后项：
//   - 移植：types / 接口 / Registry / 8 个 BuildCommand / 路径字段铁律（D7）/
//     可执行解析 + Windows npm shim / Detect（LookPath + 版本探测）。
//   - P6b 落地：claude/reclaude 的 generate_mcp_config（orchestrator 就绪时写
//     <proj>/.claude/mcp-<sid>.json，合并 ~/.claude.json 用户全局 mcpServers + panes 端点）；
//     codex pushMcpOverrides 转真（SkipMCP 由 terminal 据 orchestrator 就绪翻转）。
//   - 仍延后：codex disable_unlisted_mcp 的 ~/.codex/config.toml 解析（默认关）；shared MCP
//     （npx mcp-proxy 桥，SharedMCPURLs 恒空）；项目 hooks 机制（依赖 cc-panes-cli-hook 二进制，P6d）。
//
// 路径字段铁律（D7，移植时 1:1 保留）：
//   - ProjectPath = PTY 的 cwd，永远等于用户操作目标，决定 cwd 只能用它。
//   - WorkspacePath 仅多目录 CLI（claude/reclaude/kimi）拼 --add-dir，且仅当 ≠ProjectPath。
//   - launch_cwd（运行时观测）由 cli-hook 上报，属 Phase 5。
package cliadapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xkpane/internal/utils/filex"
	"xkpane/internal/utils/logx"
	"xkpane/internal/utils/procx"
)

// Capabilities 是能力声明，前端据此决定 UI 展示（是否显示 Provider 菜单 / Resume 按钮等）。
type Capabilities struct {
	SupportsProvider        bool     `json:"supportsProvider"`
	SupportsResume          bool     `json:"supportsResume"`
	SupportsMCP             bool     `json:"supportsMcp"`
	SupportsSystemPrompt    bool     `json:"supportsSystemPrompt"`
	SupportsWorkspace       bool     `json:"supportsWorkspace"`
	SupportsProjectHooks    bool     `json:"supportsProjectHooks"`
	CompatibleProviderTypes []string `json:"compatibleProviderTypes"`
}

// ToolInfo 是工具元信息。Version/Path 为 Detect 后填充；Capabilities 在 Detect 时附上。
type ToolInfo struct {
	ID           string        `json:"id"`
	DisplayName  string        `json:"displayName"`
	Executable   string        `json:"executable"`
	VersionArgs  []string      `json:"versionArgs"`
	Installed    bool          `json:"installed"`
	Version      string        `json:"version,omitempty"`
	Path         string        `json:"path,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
}

// Provider 是供 adapter 使用的轻量 Provider 视图（避免依赖主数据模型）。
// 可空字段以空字符串表示「无」。
type Provider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderType string `json:"providerType"`
	APIKey       string `json:"apiKey,omitempty"`
	BaseURL      string `json:"baseUrl,omitempty"`
	Region       string `json:"region,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	AWSProfile   string `json:"awsProfile,omitempty"`
	ConfigDir    string `json:"configDir,omitempty"`
	IsDefault    bool   `json:"isDefault"`
}

// Context 是构建启动命令的上下文（扁平字段，避免依赖主数据模型）。
// 可空语义：字符串字段以 "" 表示「无」；OrchestratorPort 以 0 表示「无」；Provider 用 nil。
type Context struct {
	SessionID          string
	ProjectPath        string // = PTY cwd（铁律）
	WorkspacePath      string // 仅 ≠ProjectPath 时拼 --add-dir（铁律）；"" = 无
	Provider           *Provider
	ResumeID           string // "" = 无
	SkipMCP            bool
	AppendSystemPrompt string // "" = 无
	InitialPrompt      string // 初始用户 prompt，作为位置参数；"" = 无
	OrchestratorPort   int    // Orchestrator HTTP 端口；0 = 无（未就绪 → SkipMCP fail-open）
	OrchestratorToken  string // Orchestrator Bearer Token；"" = 无（未就绪 → SkipMCP fail-open）
	DataDir            string // 数据目录（保留字段，per-session MCP 配置已改落 <proj>/.claude/）

	// 共享 MCP Server URL 映射（name → http url）。Phase 5。
	SharedMCPURLs map[string]string
	// 运行配置允许保留的 MCP server id（codex 隔离用）。Phase 5。
	AllowedMCPServerIDs []string
	// 为 true 时 codex 把 config.toml 里不在 AllowedMCPServerIDs 的 MCP 显式 disabled。Phase 5。
	DisableUnlistedMCPServers bool
}

// CommandResult 是命令构建结果。
type CommandResult struct {
	Command   string            // 可执行文件绝对路径
	Args      []string          // 参数（不含 argv[0]）
	EnvRemove []string          // 需要从子进程环境移除的变量名
	EnvInject map[string]string // 需要注入子进程的变量
}

// Adapter 是 CLI 工具适配器接口。新增工具 = 实现接口 + 在 Default() 注册一行。
type Adapter interface {
	// Info 返回工具静态元信息（未填 installed/version/path）。
	Info() ToolInfo
	// Capabilities 返回能力声明。
	Capabilities() Capabilities
	// Detect 检测安装状态与版本（默认走 LookPath + --version 探测，带超时）。
	Detect() ToolInfo
	// BuildCommand 构建启动命令（核心方法）。
	BuildCommand(ctx *Context) (CommandResult, error)
}

// base 为各 adapter 提供 Info/Capabilities/Detect 的默认实现（Go 无 trait 默认方法，用内嵌）。
type base struct {
	info ToolInfo
	caps Capabilities
}

func (b *base) Info() ToolInfo             { return b.info }
func (b *base) Capabilities() Capabilities { return b.caps }

// Detect 默认实现：PATH 解析可执行 + 跑 version_args 取版本。Claude/Reclaude 覆盖以走自定义解析。
func (b *base) Detect() ToolInfo { return detectDefault(b.info, b.caps) }

// detectDefault 用 exec.LookPath 判定安装与路径，再经 procx 探测版本（honor 子进程铁律）。
func detectDefault(info ToolInfo, caps Capabilities) ToolInfo {
	out := info
	if path, err := exec.LookPath(info.Executable); err == nil {
		out.Installed = true
		out.Path = path
		out.Version = probeVersion(path, info.VersionArgs, 5*time.Second)
	} else {
		out.Installed = false
	}
	c := caps
	out.Capabilities = &c
	return out
}

// probeVersion 带超时执行 `<exe> <args...>` 并返回 trim 后的 stdout；超时或启动失败返回 ""。
// 走 procx（子进程铁律）：CaptureOutput + OnStdout 收集行；非交互、关 stdin（procx 默认不接 stdin）。
func probeVersion(exe string, args []string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var (
		mu sync.Mutex
		b  strings.Builder
	)
	_ = procx.Run(ctx, procx.Spec{
		Name:            exe,
		Args:            args,
		CaptureOutput:   true,
		KillGracePeriod: time.Second,
		OnStdout: func(line string) {
			mu.Lock()
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line)
			mu.Unlock()
		},
	})
	// 超时（ctx 取消导致子进程被 kill）视为探测失败，返回 ""（与 cc-pane run_with_timeout 一致）。
	if ctx.Err() != nil {
		return ""
	}
	mu.Lock()
	defer mu.Unlock()
	return strings.TrimSpace(b.String())
}

// buildAddDirArgs 是路径字段铁律（D7）的共享实现：仅当 WorkspacePath 存在且 ≠ProjectPath
// 时返回 ["--add-dir", workspace]，否则空。claude / reclaude / kimi 共用。
func buildAddDirArgs(ctx *Context) []string {
	if ctx.WorkspacePath != "" && ctx.WorkspacePath != ctx.ProjectPath {
		return []string{"--add-dir", ctx.WorkspacePath}
	}
	return nil
}

// generateClaudeMCPConfig 是 claude/reclaude 共享的 per-session MCP 配置生成（移植 cc-pane
// claude::generate_mcp_config）。orchestrator 就绪（port/token 非空）时，往【被启动项目的 .claude/】
// 写 mcp-<session>.json，返回配置文件绝对路径供 --mcp-config；未就绪或写失败返回 ""（调用方据此
// 不加 --mcp-config，fail-open 裸跑）。
//
// 内容：合并 ~/.claude.json 用户全局 mcpServers（低优先级，shared 命中则跳过）+ shared MCP（HTTP）
// + panes 端点（最高优先级，覆盖同名）。token 经 header 与 ?token= 双通道（Claude Issue #7290 后备）。
//
// 落点偏离 cc-pane（其写 data_dir）：用户要求 per-session 注入落到被启动项目的 .claude/（与 skills 同处）。
// shared MCP（npx mcp-proxy 桥）本轮推迟，SharedMCPURLs 恒空。
func generateClaudeMCPConfig(ctx *Context) string {
	if ctx.OrchestratorPort == 0 || ctx.OrchestratorToken == "" {
		return ""
	}
	dir := filepath.Join(ctx.ProjectPath, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logx.For("cliadapters").Warn("mcp config: ensure .claude dir", "dir", dir, "err", err)
		return ""
	}
	fileName := fmt.Sprintf("mcp-%s.json", ctx.SessionID)
	configPath := filepath.Join(dir, fileName)
	cleanupStaleMCPConfigs(dir, fileName)

	servers := map[string]any{}
	// 合并用户全局 ~/.claude.json mcpServers（低优先级）；shared 命中跳过（本轮 shared 恒空）。
	for name, cfg := range readUserGlobalMCPServers() {
		if _, ok := ctx.SharedMCPURLs[name]; ok {
			continue
		}
		servers[name] = cfg
	}
	// shared MCP（HTTP 模式），本轮恒空。
	for name, url := range ctx.SharedMCPURLs {
		servers[name] = map[string]any{"type": "http", "url": url}
	}
	// panes 端点（最高优先级，覆盖同名）。MCP server 名 = panes（与 cc-pane 的 ccpanes 区分，避免共存撞名）。
	servers["panes"] = map[string]any{
		"type":    "http",
		"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp?token=%s", ctx.OrchestratorPort, ctx.OrchestratorToken),
		"headers": map[string]string{"Authorization": "Bearer " + ctx.OrchestratorToken},
	}

	data, err := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	if err != nil {
		return ""
	}
	if err := filex.WriteAtomic(configPath, data, 0o644); err != nil {
		logx.For("cliadapters").Warn("mcp config: write", "path", configPath, "err", err)
		return ""
	}
	return configPath
}

// readUserGlobalMCPServers 读 ~/.claude.json 的 mcpServers（移植 cc-pane read_user_global_mcp_servers）。
// 任何错误（无 home / 无文件 / 解析失败 / 无 mcpServers）返回 nil。RawMessage 保留用户配置原样。
func readUserGlobalMCPServers() map[string]json.RawMessage {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := filex.ReadLimit(filepath.Join(home, ".claude.json"), 64<<20)
	if err != nil {
		return nil
	}
	var parsed struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	return parsed.MCPServers
}

// cleanupStaleMCPConfigs 删除 dir 里 >1h 的旧 mcp-*.json（保留 current），防 per-session 文件堆积。
func cleanupStaleMCPConfigs(dir, current string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Hour)
	for _, e := range entries {
		name := e.Name()
		if name == current || !strings.HasPrefix(name, "mcp-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
