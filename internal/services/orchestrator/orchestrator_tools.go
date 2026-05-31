package orchestrator

// P6c 核心编排工具：让 pane 内的 Agent 经 MCP 反向操控宿主——开/读/写/杀别的会话，
// 实现「一个 Claude 指挥一群 Claude/Codex」的自我编排闭环。全部由 XK 既有 service 兜底
// （terminal/project/workspace），忠实移植 cc-pane orchestrator_service.rs 的工具语义。
//
// 返回约定（对齐 cc-pane）：错误以「错误: ...」文本作为正常结果返回（不返回 Go error，
// 否则 MCP 协议层判为工具调用失败）；成功返回 JSON 文本。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/terminal"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools 把全部编排工具注册到 MCP server（health 在 buildMCPHandler 单独注册）。
func (s *Service) registerTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "launch_task",
		Description: "启动一个新的 CLI Agent（Claude/Codex…）执行任务，或恢复已有会话。新任务传 prompt（在 XK-Pane 中创建会话并把 prompt 作为初始输入）；恢复传 resumeId（以 --resume 启动，不注入 prompt）。prompt 与 resumeId 互斥。project_path 必须已注册（list_projects 可查）。",
	}, s.toolLaunchTask)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "列出所有已注册的项目（DB 项目 + 工作空间项目，按路径去重），供 launch_task 选择合法 project_path。",
	}, s.toolListProjects)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "列出所有工作空间及其基本信息（名称、项目数、根路径、是否置顶）。",
	}, s.toolListWorkspaces)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sessions",
		Description: "列出所有活跃的终端会话及其状态，返回 sessionId、status、lastOutputAt。",
	}, s.toolListSessions)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_session_status",
		Description: "查询指定终端会话的当前状态（active/exited）及最近输出时间。",
	}, s.toolGetSessionStatus)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_session_output",
		Description: "读取终端会话的最近输出内容（纯文本，ANSI 已剥离）。用途：监控其他 Agent 进度、提取错误、判断任务完成。已退出的会话在 5 分钟内仍可读取。",
	}, s.toolGetSessionOutput)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "write_to_session",
		Description: "向指定会话写入原始字节（不做时序处理）。适合发送控制字符（如 Ctrl+C = \"\\u0003\"）。若要提交命令或 prompt，请改用 submit_to_session。",
	}, s.toolWriteToSession)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "submit_to_session",
		Description: "向会话提交文本（自动处理 Enter 键时序：先写文本、等待若干毫秒、再单独发 Enter，确保 Claude Code 正确识别为提交）。适用于发 slash command 或 prompt。",
	}, s.toolSubmitToSession)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "kill_session",
		Description: "终止指定的终端会话（PTY 进程被立即关闭）。",
	}, s.toolKillSession)
}

// ── 参数结构（json 名 + jsonschema 描述；omitempty = 可选）─────────────────

type launchTaskParams struct {
	ProjectPath   string `json:"project_path" jsonschema:"项目路径（必须已注册：DB 项目或工作空间项目）"`
	Prompt        string `json:"prompt,omitempty" jsonschema:"新任务的初始 prompt（与 resume_id 互斥）"`
	ResumeID      string `json:"resume_id,omitempty" jsonschema:"恢复已有会话的 id（与 prompt 互斥）"`
	CliTool       string `json:"cli_tool,omitempty" jsonschema:"CLI 工具 id（claude/codex/reclaude…），默认 claude"`
	WorkspaceName string `json:"workspace_name,omitempty" jsonschema:"工作空间名（用于解析 --add-dir 的 workspacePath）"`
	ProviderID    string `json:"provider_id,omitempty" jsonschema:"接入 Provider id"`
	Title         string `json:"title,omitempty" jsonschema:"前端标签页标题"`
}

type sessionIDParams struct {
	SessionID string `json:"session_id" jsonschema:"目标会话 id"`
}

type getOutputParams struct {
	SessionID string `json:"session_id" jsonschema:"目标会话 id"`
	Lines     int    `json:"lines,omitempty" jsonschema:"返回最近多少行（0 或省略 = 全部）"`
}

type writeParams struct {
	SessionID string `json:"session_id" jsonschema:"目标会话 id"`
	Text      string `json:"text" jsonschema:"要写入的文本/原始字节"`
}

// ── 工具实现 ────────────────────────────────────────────────────────────

func (s *Service) toolLaunchTask(ctx context.Context, req *mcp.CallToolRequest, in launchTaskParams) (*mcp.CallToolResult, any, error) {
	hasPrompt := strings.TrimSpace(in.Prompt) != ""
	hasResume := strings.TrimSpace(in.ResumeID) != ""
	if hasPrompt && hasResume {
		return textResult("错误: prompt 和 resumeId 互斥，不能同时提供"), nil, nil
	}
	if !hasPrompt && !hasResume {
		return textResult("错误: 必须提供 prompt 或 resumeId 其中之一"), nil, nil
	}
	if !s.isProjectRegistered(ctx, in.ProjectPath) {
		return textResult(fmt.Sprintf("错误: 项目路径 '%s' 未注册", in.ProjectPath)), nil, nil
	}

	// 工作空间解析：workspace_name → workspacePath（未找到则忽略，对齐 cc-pane warn-then-none）。
	var wsPath string
	if in.WorkspaceName != "" {
		if p, ok := s.workspacePath(ctx, in.WorkspaceName); ok {
			wsPath = p
		}
	}
	// cli_tool 缺省时同源继承发起方 CLI（reclaude 启动的子会话仍走 reclaude）；
	// 无法识别发起方（如裸 shell / 直连）才回落 claude。显式传参始终优先。
	cliTool := in.CliTool
	if cliTool == "" {
		cliTool = callerTool(req)
	}
	if cliTool == "" {
		cliTool = "claude"
	}

	spec := terminal.CreateSpec{
		ProjectPath:   in.ProjectPath,
		WorkspacePath: wsPath,
		CliTool:       cliTool,
		ProviderID:    in.ProviderID,
		Cols:          120,
		Rows:          30,
	}
	if hasResume {
		spec.ResumeID = in.ResumeID
	} else {
		spec.InitialPrompt = in.Prompt
	}

	sid, err := s.deps.Terminal.CreateSession(ctx, spec)
	if err != nil {
		return textResult(fmt.Sprintf("错误: 创建会话失败: %v", err)), nil, nil
	}

	taskID := genID()
	// 通知前端开新面板附着该会话（events.go 注册的 orchestrator:launch；前端 listener 见 P6c-3）。
	s.emitEvent(events.OrchestratorLaunch, events.OrchestratorLaunchPayload{
		TaskID:        taskID,
		SessionID:     sid,
		ProjectPath:   in.ProjectPath,
		WorkspaceName: in.WorkspaceName,
		ProviderID:    in.ProviderID,
		Title:         in.Title,
		ResumeID:      in.ResumeID,
		CliTool:       cliTool,
	})
	return jsonResult(map[string]any{"taskId": taskID, "sessionId": sid, "status": "launching"}), nil, nil
}

func (s *Service) toolListProjects(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	infos := []map[string]any{}
	seen := map[string]bool{}
	if ps, err := s.deps.Projects.List(ctx); err == nil {
		for _, p := range ps {
			infos = append(infos, map[string]any{"id": p.ID, "name": p.Name, "path": p.Path, "source": "db"})
			seen[normalizePath(p.Path)] = true
		}
	}
	if wss, err := s.deps.Workspaces.List(ctx); err == nil {
		for _, ws := range wss {
			for _, p := range ws.Projects {
				norm := normalizePath(p.Path)
				if seen[norm] {
					continue
				}
				seen[norm] = true
				infos = append(infos, map[string]any{
					"id": p.ID, "name": baseName(p.Path), "path": p.Path,
					"workspace": ws.Name, "source": "workspace",
				})
			}
		}
	}
	return jsonResult(map[string]any{"projects": infos}), nil, nil
}

func (s *Service) toolListWorkspaces(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	wss, err := s.deps.Workspaces.List(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("错误: %v", err)), nil, nil
	}
	items := []map[string]any{}
	for _, ws := range wss {
		items = append(items, map[string]any{
			"name": ws.Name, "alias": ws.Alias, "projectCount": len(ws.Projects),
			"providerId": ws.ProviderID, "path": ws.Path, "pinned": ws.Pinned,
		})
	}
	return jsonResult(map[string]any{"workspaces": items}), nil, nil
}

func (s *Service) toolListSessions(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	sessions := []map[string]any{}
	for _, sn := range s.deps.Terminal.SessionSnapshots() {
		sessions = append(sessions, map[string]any{
			"sessionId": sn.ID, "status": sn.Status, "lastOutputAt": sn.LastOutputAt,
		})
	}
	return jsonResult(map[string]any{"sessions": sessions}), nil, nil
}

func (s *Service) toolGetSessionStatus(_ context.Context, _ *mcp.CallToolRequest, in sessionIDParams) (*mcp.CallToolResult, any, error) {
	for _, sn := range s.deps.Terminal.SessionSnapshots() {
		if sn.ID == in.SessionID {
			return jsonResult(map[string]any{
				"sessionId": sn.ID, "status": sn.Status, "lastOutputAt": sn.LastOutputAt,
			}), nil, nil
		}
	}
	return textResult(fmt.Sprintf("错误: 会话 '%s' 不存在", in.SessionID)), nil, nil
}

func (s *Service) toolGetSessionOutput(_ context.Context, _ *mcp.CallToolRequest, in getOutputParams) (*mcp.CallToolResult, any, error) {
	out, err := s.deps.Terminal.SessionOutput(in.SessionID, in.Lines)
	if err != nil {
		return textResult(fmt.Sprintf("错误: 会话 '%s' 不存在或已退出超过 5 分钟", in.SessionID)), nil, nil
	}
	return jsonResult(map[string]any{
		"sessionId": out.SessionID,
		"lines":     out.Lines,
		"content":   strings.Join(out.Lines, "\n"),
		"lineCount": len(out.Lines),
	}), nil, nil
}

func (s *Service) toolWriteToSession(ctx context.Context, _ *mcp.CallToolRequest, in writeParams) (*mcp.CallToolResult, any, error) {
	if err := s.deps.Terminal.WriteInput(ctx, in.SessionID, in.Text); err != nil {
		return textResult(fmt.Sprintf("错误: 写入会话 '%s' 失败: %v", in.SessionID, err)), nil, nil
	}
	return jsonResult(map[string]any{"success": true, "sessionId": in.SessionID}), nil, nil
}

func (s *Service) toolSubmitToSession(ctx context.Context, _ *mcp.CallToolRequest, in writeParams) (*mcp.CallToolResult, any, error) {
	// 去掉换行防意外提交，写文本 → 等 ink 处理 → 单独发 Enter（移植 cc-pane submit_text_to_session）。
	clean := strings.NewReplacer("\r", "", "\n", "").Replace(in.Text)
	if err := s.deps.Terminal.WriteInput(ctx, in.SessionID, clean); err != nil {
		return textResult(fmt.Sprintf("错误: 提交到会话 '%s' 失败: %v", in.SessionID, err)), nil, nil
	}
	time.Sleep(computeEnterDelay(len(clean)))
	if err := s.deps.Terminal.WriteInput(ctx, in.SessionID, "\r"); err != nil {
		return textResult(fmt.Sprintf("错误: 提交到会话 '%s' 失败: %v", in.SessionID, err)), nil, nil
	}
	return jsonResult(map[string]any{"success": true, "sessionId": in.SessionID}), nil, nil
}

func (s *Service) toolKillSession(ctx context.Context, _ *mcp.CallToolRequest, in sessionIDParams) (*mcp.CallToolResult, any, error) {
	if err := s.deps.Terminal.Kill(ctx, in.SessionID); err != nil {
		return textResult(fmt.Sprintf("错误: 终止会话 '%s' 失败: %v", in.SessionID, err)), nil, nil
	}
	return jsonResult(map[string]any{"success": true, "sessionId": in.SessionID}), nil, nil
}

// ── 辅助 ────────────────────────────────────────────────────────────────

// textResult 把字符串包成 MCP TextContent 结果。
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// jsonResult 把值 JSON 序列化后作为 TextContent 返回（对齐 cc-pane 工具返回 JSON 字符串）。
func jsonResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return textResult("错误: 序列化失败")
	}
	return textResult(string(data))
}

// normalizePath 复刻 cc-pane normalize_path：`\`→`/`、去尾 `/`（用于白名单 / 去重比对）。
func normalizePath(p string) string {
	return strings.TrimRight(strings.ReplaceAll(p, "\\", "/"), "/")
}

// baseName 取归一化路径的末段（工作空间项目无 Name 字段时用作展示名）。
func baseName(p string) string {
	p = normalizePath(p)
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// computeEnterDelay 复刻 cc-pane compute_enter_delay_ms：base 200ms + 每 512 字节加 30ms，封顶 5s。
func computeEnterDelay(textLen int) time.Duration {
	ms := 200 + (textLen/512)*30
	if ms > 5000 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond
}

// isProjectRegistered 复刻 cc-pane is_project_registered：归一化后比对 DB 项目 + 工作空间项目。
func (s *Service) isProjectRegistered(ctx context.Context, path string) bool {
	norm := normalizePath(path)
	if ps, err := s.deps.Projects.List(ctx); err == nil {
		for _, p := range ps {
			if normalizePath(p.Path) == norm {
				return true
			}
		}
	}
	if wss, err := s.deps.Workspaces.List(ctx); err == nil {
		for _, ws := range wss {
			for _, p := range ws.Projects {
				if normalizePath(p.Path) == norm {
					return true
				}
			}
		}
	}
	return false
}

// workspacePath 据 workspace 名取其根路径（用于 launch_task 解析 workspacePath）。
func (s *Service) workspacePath(ctx context.Context, name string) (string, bool) {
	wss, err := s.deps.Workspaces.List(ctx)
	if err != nil {
		return "", false
	}
	for _, ws := range wss {
		if ws.Name == name && ws.Path != nil {
			return *ws.Path, true
		}
	}
	return "", false
}

// genID 生成 32-hex 随机 id（task id 用，复用 genToken 的 crypto/rand）。
func genID() string { return genToken() }

// callerTool 从 MCP 请求头取「发起方 CLI」（authMiddleware 由 ?caller_tool= 桥接而来）；
// 非 HTTP 传输 / 未自报时返回 ""。供 launch_task 同源继承缺省 cli_tool。
func callerTool(req *mcp.CallToolRequest) string {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return ""
	}
	return req.Extra.Header.Get(callerToolHeader)
}
