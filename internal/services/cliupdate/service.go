// Package cliupdate 是 CLI 工具（claude / reclaude / codex / grok）版本检测与一键升级的 Wails 门面。
//
// 命令全部在后端拼装，不信任前端传入的任何命令字符串——前端只传 tool 名，本包按白名单
// （toolDefs）校验后再拼出实际要执行的命令行。
//
// reclaude 是自管理工具：版本查询走 `reclaude version`、升级走 `reclaude update`，不走 npm。
package cliupdate

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"xkpane/internal/utils/logx"
	"xkpane/internal/utils/procx"
)

// toolDef 描述一个受支持的 CLI 工具：显示名、对应 npm 包、官方自升级命令。
type toolDef struct {
	displayName string
	npmPackage  string
	selfUpdate  string // 官方自升级命令；空 = 不支持。
	// codex 刻意不用 `codex update`——上游有「报成功但未实际更新」的假成功问题，
	// 走 npm 重装更可靠（借鉴 cc-switch 的结论）。
	versionCmd string // 取当前版本的命令；空 = 默认 "<tool> --version"。
	latestCmd  string // 取/检查最新版本的命令；空 = 走 npm view <npmPackage>。
}

var toolDefs = map[string]toolDef{
	"claude":   {displayName: "Claude Code", npmPackage: "@anthropic-ai/claude-code", selfUpdate: "claude update"},
	"reclaude": {displayName: "Reclaude", versionCmd: "reclaude version", latestCmd: "reclaude update --check", selfUpdate: "reclaude update"},
	"codex":    {displayName: "Codex", npmPackage: "@openai/codex"},
	"grok":     {displayName: "Grok Build", npmPackage: "@xai-official/grok", selfUpdate: "grok update"},
}

// ToolStatusDTO 是前端可见的版本检测结果。
type ToolStatusDTO struct {
	Tool            string `json:"tool"`
	Installed       bool   `json:"installed"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Detail          string `json:"detail"` // 非致命异常的中文提示（如 npm 不可用）
}

// UpgradeCliResultDTO 是前端可见的一键升级结果。
type UpgradeCliResultDTO struct {
	Tool       string `json:"tool"`
	Ok         bool   `json:"ok"`
	NewVersion string `json:"newVersion"`
	Message    string `json:"message"` // 失败时=输出尾部若干行；假成功时=中文警告
}

// Service 是 cliupdate 的 Wails 门面。
type Service struct {
	log *slog.Logger
}

// New 创建 Service。
func New() *Service {
	return &Service{log: logx.For("cliupdate")}
}

// runShell 把一条完整命令行交给系统 shell 执行并捕获合并输出（stdout+stderr 按行）。
// Windows 用 cmd /C——claude/npm 在 Windows 是 .cmd 批处理，直接 CreateProcess 不可靠；
// 其余平台 sh -c。
func (s *Service) runShell(ctx context.Context, timeout time.Duration, cmdline string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name := "sh"
	args := []string{"-c", cmdline}
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/C", cmdline}
	}

	var mu sync.Mutex
	var lines []string
	collect := func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}

	proc, err := procx.StartCtx(ctx, procx.Spec{
		Name:          name,
		Args:          args,
		CaptureOutput: true,
		OnStdout:      collect,
		OnStderr:      collect,
	})
	if err != nil {
		return "", err
	}
	defer proc.Stop()
	waitErr := proc.Wait()

	mu.Lock()
	out := strings.Join(lines, "\n")
	mu.Unlock()
	return out, waitErr
}

// versionCmdFor 返回取当前版本的命令：优先用 def.versionCmd，否则回退 "<tool> --version"。
func versionCmdFor(tool string, def toolDef) string {
	if def.versionCmd != "" {
		return def.versionCmd
	}
	return tool + " --version"
}

// latestVersion 取最新版本号：优先跑 def.latestCmd（自管理工具），否则走 npm view <npmPackage>；
// 两者都不可用则返回 ""。尽力而为，不返回 error。
func (s *Service) latestVersion(ctx context.Context, def toolDef) string {
	if def.latestCmd != "" {
		out, err := s.runShell(ctx, 30*time.Second, def.latestCmd)
		if err != nil {
			return ""
		}
		return extractVersion(out)
	}
	if def.npmPackage != "" {
		// 用 npm view 而非直连 registry.npmjs.org，是有意为之：它遵守用户 .npmrc 的 registry
		// 镜像配置（中国网络环境更可靠），不要改成 httpx 直连。
		out, err := s.runShell(ctx, 30*time.Second, "npm view "+def.npmPackage+" version")
		if err != nil {
			return ""
		}
		return extractVersion(out)
	}
	return ""
}

// CheckCliTool 检测本地 tool 的当前版本与最新版本。检测是尽力而为——除白名单
// 校验外，任何一步失败都不返回 error，降级进 Detail。
func (s *Service) CheckCliTool(ctx context.Context, tool string) (*ToolStatusDTO, error) {
	def, ok := toolDefs[tool]
	if !ok {
		return nil, fmt.Errorf("cliupdate: 不支持的工具 %q", tool)
	}

	dto := &ToolStatusDTO{Tool: tool}

	verOut, verErr := s.runShell(ctx, 20*time.Second, versionCmdFor(tool, def))
	if verErr == nil {
		if v := extractVersion(verOut); v != "" {
			dto.Installed = true
			dto.CurrentVersion = v
		}
	}

	dto.LatestVersion = s.latestVersion(ctx, def)
	if dto.LatestVersion == "" {
		if def.latestCmd != "" {
			dto.Detail = "无法解析最新版本，可直接点升级由 " + def.selfUpdate + " 自检"
		} else {
			dto.Detail = "npm 不可用或查询超时，无法获取最新版本"
		}
	}

	dto.UpdateAvailable = dto.Installed && dto.LatestVersion != "" && compareSemver(dto.LatestVersion, dto.CurrentVersion) > 0
	return dto, nil
}

// UpgradeCliTool 执行一键升级：有官方自升级命令的（claude）先跑自升级，失败再回退 npm
// 重装；没有的（codex）直接 npm 重装。成功后复检版本，识别「报成功但未实际更新」的假成功。
func (s *Service) UpgradeCliTool(ctx context.Context, tool string) (*UpgradeCliResultDTO, error) {
	def, ok := toolDefs[tool]
	if !ok {
		return nil, fmt.Errorf("cliupdate: 不支持的工具 %q", tool)
	}

	s.log.Info("cli upgrade start", "tool", tool)

	latest := s.latestVersion(ctx, def)

	result := &UpgradeCliResultDTO{Tool: tool}
	const upgradeTimeout = 10 * time.Minute
	var lastOutput string
	succeeded := false

	if def.selfUpdate != "" {
		out, err := s.runShell(ctx, upgradeTimeout, def.selfUpdate)
		lastOutput = out
		succeeded = err == nil
	}
	if !succeeded && def.npmPackage != "" {
		out, err := s.runShell(ctx, upgradeTimeout, "npm i -g "+def.npmPackage+"@latest")
		lastOutput = out
		succeeded = err == nil
	}

	if !succeeded {
		result.Message = lastLines(lastOutput, 8)
		s.log.Warn("cli upgrade failed", "tool", tool, "message", result.Message)
		return result, nil
	}

	verOut, _ := s.runShell(ctx, 20*time.Second, versionCmdFor(tool, def))
	newVersion := extractVersion(verOut)
	result.Ok = true
	result.NewVersion = newVersion

	if latest != "" && newVersion != "" && compareSemver(latest, newVersion) > 0 {
		result.Message = fmt.Sprintf(
			"升级命令执行成功，但当前版本仍为 v%s（最新 v%s），上游升级器可能报告成功但未实际更新",
			newVersion, latest,
		)
		s.log.Warn("cli upgrade false success", "tool", tool, "current", newVersion, "latest", latest)
	} else {
		s.log.Info("cli upgrade done", "tool", tool, "newVersion", newVersion)
	}

	return result, nil
}
