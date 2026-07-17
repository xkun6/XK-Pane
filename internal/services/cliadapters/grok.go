package cliadapters

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// GrokAdapter 适配 Grok Build CLI（grok，xAI 终端编码 agent，Rust 二进制）。全局配置走
// ~/.grok/config.toml（agentcfg 域直写切换，本期未接），故不接会话级 Provider env 注入；
// Resume/MCP/项目 hooks 本期不接（Capabilities 全 false）。
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
		caps: Capabilities{}, // 全 false：SupportsProvider/Resume/MCP/SystemPrompt/Workspace/ProjectHooks 均不支持
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
func (a *GrokAdapter) BuildCommand(ctx *Context) (CommandResult, error) {
	path, err := resolveGrok()
	if err != nil {
		return CommandResult{}, err
	}

	var args []string
	if ctx.ResumeID != "" {
		args = append(args, "--resume", ctx.ResumeID) // 契约完整性：上游本期不会传（SupportsResume=false），但传了就正确处理
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
