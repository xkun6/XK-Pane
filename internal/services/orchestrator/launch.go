package orchestrator

// launch.go：launch_task 的公共化（P7 §6.5）。把 toolLaunchTask 的核心（校验项目注册 +
// 解析 workspace/cliTool 缺省 + CreateSpec + CreateSession + 广播 orchestrator:launch）抽成
// Service.LaunchTask，让 MCP 工具（orchestrator_tools.go:toolLaunchTask）与 webapi 的
// POST /api/launch 共用同一份校验与建会话逻辑，避免双份实现漂移。
//
// 与 MCP 工具的差异：LaunchTask 返回普通 Go error（HTTP 层转 4xx/5xx、MCP 层转「错误: …」
// 文本）；caller_tool 同源继承是 MCP 专属（读 HTTP 请求头），仍留在 toolLaunchTask 里在调用
// LaunchTask 前解析好 cli_tool，故 LaunchTask 本身不感知发起方。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"xkpane/internal/events"
	"xkpane/internal/services/terminal"
)

// LaunchParams 是 LaunchTask 的入参（MCP 工具与 POST /api/launch 共用）。
type LaunchParams struct {
	ProjectPath   string // 必须已注册（DB 项目或工作空间项目）
	Prompt        string // 新任务初始 prompt（与 ResumeID 互斥）
	ResumeID      string // 恢复已有会话 id（与 Prompt 互斥）
	CliTool       string // claude/codex/reclaude…；"" 缺省 claude
	WorkspaceName string // 解析 --add-dir 的 workspacePath（可空）
	ProviderID    string // 接入 Provider id（可空）
	Title         string // 前端标签页标题（可空）
}

// LaunchResult 是 LaunchTask 的成功返回。
type LaunchResult struct {
	TaskID    string `json:"taskId"`
	SessionID string `json:"sessionId"`
}

// LaunchTask 校验并启动一个新会话（或恢复），广播 orchestrator:launch 让所有前端（桌面 +
// Web）开窗附着同一 sessionId。错误以 Go error 返回（不带「错误: 」前缀——由调用方按协议包装）。
func (s *Service) LaunchTask(ctx context.Context, p LaunchParams) (LaunchResult, error) {
	hasPrompt := strings.TrimSpace(p.Prompt) != ""
	hasResume := strings.TrimSpace(p.ResumeID) != ""
	if hasPrompt && hasResume {
		return LaunchResult{}, errors.New("prompt 和 resumeId 互斥，不能同时提供")
	}
	if !hasPrompt && !hasResume {
		return LaunchResult{}, errors.New("必须提供 prompt 或 resumeId 其中之一")
	}
	if !s.isProjectRegistered(ctx, p.ProjectPath) {
		return LaunchResult{}, fmt.Errorf("项目路径 '%s' 未注册", p.ProjectPath)
	}

	// 工作空间解析：workspace_name → workspacePath（未找到则忽略，对齐 cc-pane warn-then-none）。
	var wsPath string
	if p.WorkspaceName != "" {
		if wp, ok := s.workspacePath(ctx, p.WorkspaceName); ok {
			wsPath = wp
		}
	}

	cliTool := p.CliTool
	if cliTool == "" {
		cliTool = "claude"
	}

	spec := terminal.CreateSpec{
		ProjectPath:   p.ProjectPath,
		WorkspacePath: wsPath,
		CliTool:       cliTool,
		ProviderID:    p.ProviderID,
		Cols:          120,
		Rows:          30,
	}
	if hasResume {
		spec.ResumeID = p.ResumeID
	} else {
		spec.InitialPrompt = p.Prompt
	}

	sid, err := s.deps.Terminal.CreateSession(ctx, spec)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("创建会话失败: %w", err)
	}

	taskID := genID()
	// 广播给所有前端开新面板附着该会话：桌面经 Wails 桥、Web 经 /ws（app.go 的双路 emit）。
	s.emitEvent(events.OrchestratorLaunch, events.OrchestratorLaunchPayload{
		TaskID:        taskID,
		SessionID:     sid,
		ProjectPath:   p.ProjectPath,
		WorkspaceName: p.WorkspaceName,
		ProviderID:    p.ProviderID,
		Title:         p.Title,
		ResumeID:      p.ResumeID,
		CliTool:       cliTool,
	})
	return LaunchResult{TaskID: taskID, SessionID: sid}, nil
}
