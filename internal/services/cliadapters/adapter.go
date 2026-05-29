// Package cliadapters 移植 cc-pane 的 CLI 工具适配器层（cc-cli-adapters crate）。
//
// 每个受支持的命令行 Agent（Claude Code / Codex / Gemini / Kimi / GLM / OpenCode /
// Cursor / Reclaude）实现 Adapter 接口，提供元信息、能力声明与「启动命令拼装」
// （BuildCommand）。Phase 2 的终端服务据此在 PTY 里以选定 Provider 启动 CLI。
//
// 移植范围（Phase 1）与刻意延后项：
//   - 移植：types / 接口 / Registry / 8 个 BuildCommand / 路径字段铁律（D7）/
//     可执行解析 + Windows npm shim / Detect（LookPath + 版本探测）。
//   - 延后：claude/reclaude 的 generate_mcp_config（写 mcp-*.json，依赖 Phase 5
//     orchestrator 的 port/token）；codex disable_unlisted_mcp 的 ~/.codex/config.toml
//     解析（默认关，Phase 5 才置真）；项目 hooks 机制（依赖尚未移植的 cc-panes-cli-hook
//     二进制 + 已确认延后的 IM 桥接）。相关 hook 点以 TODO(phase5)/TODO(hooks) 标注。
//
// 路径字段铁律（D7，移植时 1:1 保留）：
//   - ProjectPath = PTY 的 cwd，永远等于用户操作目标，决定 cwd 只能用它。
//   - WorkspacePath 仅多目录 CLI（claude/reclaude/kimi）拼 --add-dir，且仅当 ≠ProjectPath。
//   - launch_cwd（运行时观测）由 cli-hook 上报，属 Phase 5。
package cliadapters

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

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
	OrchestratorPort   int    // Orchestrator HTTP 端口；0 = 无（Phase 5）
	OrchestratorToken  string // Orchestrator Bearer Token；"" = 无（Phase 5）
	DataDir            string // 数据目录（写 MCP / adapter 配置文件用）

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

func (b *base) Info() ToolInfo         { return b.info }
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

// orchestratorEnv 拼装 claude/reclaude 注入的 orchestrator 环境变量。
// CC_PANES_PTY_SESSION_ID 永远注入；port/token 仅在 Phase 5 提供时注入。
func orchestratorEnv(ctx *Context) map[string]string {
	env := map[string]string{
		"CC_PANES_PTY_SESSION_ID": ctx.SessionID,
	}
	if ctx.OrchestratorPort != 0 {
		env["CC_PANES_API_PORT"] = strconv.Itoa(ctx.OrchestratorPort)
	}
	if ctx.OrchestratorToken != "" {
		env["CC_PANES_API_TOKEN"] = ctx.OrchestratorToken
	}
	return env
}
