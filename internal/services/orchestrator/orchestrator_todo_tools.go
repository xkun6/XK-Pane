package orchestrator

// 结合 ccpane：Todo 驱动 + 编排任务绑定的 MCP 工具。让 pane 内的 Agent 经 MCP 查/建/改待办
// （配合已有 list_projects/launch_task/get_session_status 跑通 dispatch-todos skill 的「Todo 驱动」
// 闭环），并把每次分派落库为 TaskBinding 跟踪。忠实移植 cc-pane orchestrator_service.rs 的字段裁剪：
//   - create_todo 仅暴露 6 字段（不含 status/dueDate/reminder/recurrence/todoType）
//   - update_todo 仅暴露 4 个可改字段（不含 scope/scopeRef/tags/dueDate/myDay；Spec 后置钩子省略——XK 无 SpecService）
//   - create_task_binding 不暴露 todoId（对齐 cc-pane MCP 硬编码 None）
//   - update_task_binding 不暴露 prompt
// 返回约定同其它工具：错误以「错误: …」文本作为正常结果返回，成功返回 JSON 文本。

import (
	"context"

	"xkpane/internal/services/taskbinding"
	"xkpane/internal/services/todo"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTodoTools 注册 Todo + TaskBinding 的全部 MCP 工具（在 registerTools 内调用）。
func (s *Service) registerTodoTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_todos",
		Description: "查询待办列表（按 status / priority / scope / scopeRef / search 过滤，limit 限量）。供 dispatch-todos 工作流取未完成待办。返回 {items, total, hasMore}。",
	}, s.toolQueryTodos)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_todo",
		Description: "创建一个待办（仅 title 必填；可选 description/priority/scope/scopeRef/tags）。新建默认 status=todo。返回创建后的待办。",
	}, s.toolCreateTodo)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_todo",
		Description: "更新一个待办（按 id；可改 title/status/priority/description）。分派时用 status=in_progress 标记已派发、status=done 标记完成。返回更新后的待办。",
	}, s.toolUpdateTodo)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_task_binding",
		Description: "创建一条编排任务绑定（记录一次分派：title/projectPath 必填，可选 prompt/sessionId/workspaceName/cliTool）。默认 status=pending、progress=0、cliTool=reclaude。返回创建后的绑定。",
	}, s.toolCreateTaskBinding)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_task_binding",
		Description: "更新一条编排任务绑定（按 id；可改 title/status/progress/completionSummary/sessionId/exitCode）。status∈pending/running/waiting/completed/failed，progress 0-100。返回更新后的绑定。",
	}, s.toolUpdateTaskBinding)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_task_bindings",
		Description: "查询编排任务绑定（按 status / projectPath / search 过滤，limit 限量）。返回 {items, total, hasMore}。",
	}, s.toolQueryTaskBindings)
}

// optStr 把可选字符串参数（空串=未提供）转成 *string（nil=不约束/不更新）。
func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// 枚举白名单：非法/空值降级为 nil（query 不约束 / update 不更新该字段），复刻 cc-pane MCP 层
// `.parse().ok()` 的静默降级语义——非法 status/priority/scope 不会变成 `WHERE col='非法值'`（空集）
// 或触发服务层报错，而是被忽略。
var (
	todoStatuses   = map[string]bool{"todo": true, "in_progress": true, "done": true}
	todoPriorities = map[string]bool{"high": true, "medium": true, "low": true}
	todoScopes     = map[string]bool{"global": true, "workspace": true, "project": true, "external": true, "temp_script": true}
	tbStatuses     = map[string]bool{"pending": true, "running": true, "waiting": true, "completed": true, "failed": true}
)

func enumOrNil(v string, valid map[string]bool) *string {
	if v == "" || !valid[v] {
		return nil
	}
	s := v
	return &s
}

// ── 参数结构 ──────────────────────────────────────────────────────────────

type queryTodosParams struct {
	Status   string `json:"status,omitempty" jsonschema:"按状态筛选：todo/in_progress/done"`
	Priority string `json:"priority,omitempty" jsonschema:"按优先级筛选：high/medium/low"`
	Scope    string `json:"scope,omitempty" jsonschema:"按范围筛选：global/workspace/project/external/temp_script"`
	ScopeRef string `json:"scopeRef,omitempty" jsonschema:"范围引用（工作空间名 或 项目路径）"`
	Search   string `json:"search,omitempty" jsonschema:"标题/描述搜索关键词"`
	Limit    int    `json:"limit,omitempty" jsonschema:"返回数量上限"`
}

type createTodoParams struct {
	Title       string   `json:"title" jsonschema:"任务标题（必填）"`
	Description string   `json:"description,omitempty" jsonschema:"任务描述"`
	Priority    string   `json:"priority,omitempty" jsonschema:"high/medium/low"`
	Scope       string   `json:"scope,omitempty" jsonschema:"global/workspace/project"`
	ScopeRef    string   `json:"scopeRef,omitempty" jsonschema:"工作空间名 或 项目路径"`
	Tags        []string `json:"tags,omitempty" jsonschema:"标签列表"`
}

type updateTodoParams struct {
	ID          string `json:"id" jsonschema:"Todo ID（必填）"`
	Title       string `json:"title,omitempty" jsonschema:"新标题"`
	Status      string `json:"status,omitempty" jsonschema:"新状态：todo/in_progress/done"`
	Priority    string `json:"priority,omitempty" jsonschema:"新优先级：high/medium/low"`
	Description string `json:"description,omitempty" jsonschema:"新描述"`
}

type createTaskBindingParams struct {
	Title         string `json:"title" jsonschema:"任务标题（必填）"`
	Prompt        string `json:"prompt,omitempty" jsonschema:"分派给 Agent 的 prompt"`
	SessionID     string `json:"sessionId,omitempty" jsonschema:"关联的会话 id"`
	ProjectPath   string `json:"projectPath" jsonschema:"项目路径（必填）"`
	WorkspaceName string `json:"workspaceName,omitempty" jsonschema:"工作空间名"`
	CliTool       string `json:"cliTool,omitempty" jsonschema:"CLI 工具 id，缺省 reclaude"`
}

type updateTaskBindingParams struct {
	ID                string `json:"id" jsonschema:"TaskBinding ID（必填）"`
	Title             string `json:"title,omitempty" jsonschema:"新标题"`
	Status            string `json:"status,omitempty" jsonschema:"pending/running/waiting/completed/failed"`
	Progress          *int   `json:"progress,omitempty" jsonschema:"进度 0-100"`
	CompletionSummary string `json:"completionSummary,omitempty" jsonschema:"完成摘要"`
	SessionID         string `json:"sessionId,omitempty" jsonschema:"关联的会话 id"`
	ExitCode          *int   `json:"exitCode,omitempty" jsonschema:"退出码"`
}

type queryTaskBindingsParams struct {
	Status      string `json:"status,omitempty" jsonschema:"按状态筛选"`
	ProjectPath string `json:"projectPath,omitempty" jsonschema:"按项目路径筛选"`
	Search      string `json:"search,omitempty" jsonschema:"标题/prompt 搜索关键词"`
	Limit       int    `json:"limit,omitempty" jsonschema:"返回数量上限"`
}

// ── Todo 工具实现 ─────────────────────────────────────────────────────────

func (s *Service) toolQueryTodos(ctx context.Context, _ *mcp.CallToolRequest, in queryTodosParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Todos == nil {
		return textResult("错误: todo 服务不可用"), nil, nil
	}
	q := todo.TodoQuery{
		Status: enumOrNil(in.Status, todoStatuses), Priority: enumOrNil(in.Priority, todoPriorities),
		Scope: enumOrNil(in.Scope, todoScopes), ScopeRef: optStr(in.ScopeRef), Search: optStr(in.Search),
	}
	if in.Limit > 0 {
		q.Limit = &in.Limit
	}
	res, err := s.deps.Todos.Query(ctx, q)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(res), nil, nil
}

func (s *Service) toolCreateTodo(ctx context.Context, _ *mcp.CallToolRequest, in createTodoParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Todos == nil {
		return textResult("错误: todo 服务不可用"), nil, nil
	}
	ci := todo.CreateTodoInput{
		Title:       in.Title,
		Description: optStr(in.Description),
		Priority:    enumOrNil(in.Priority, todoPriorities),
		Scope:       enumOrNil(in.Scope, todoScopes),
		ScopeRef:    optStr(in.ScopeRef),
	}
	if len(in.Tags) > 0 {
		tags := in.Tags
		ci.Tags = &tags
	}
	dto, err := s.deps.Todos.Create(ctx, ci)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(dto), nil, nil
}

func (s *Service) toolUpdateTodo(ctx context.Context, _ *mcp.CallToolRequest, in updateTodoParams) (*mcp.CallToolResult, any, error) {
	if s.deps.Todos == nil {
		return textResult("错误: todo 服务不可用"), nil, nil
	}
	ui := todo.UpdateTodoInput{
		Title: optStr(in.Title), Status: enumOrNil(in.Status, todoStatuses),
		Priority: enumOrNil(in.Priority, todoPriorities), Description: optStr(in.Description),
	}
	if err := s.deps.Todos.Update(ctx, in.ID, ui); err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	// 对齐 cc-pane update_todo：返回更新后的完整待办。
	dto, err := s.deps.Todos.Get(ctx, in.ID)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	if dto == nil {
		return textResult("错误: 待办 '" + in.ID + "' 不存在"), nil, nil
	}
	return jsonResult(dto), nil, nil
}

// ── TaskBinding 工具实现 ──────────────────────────────────────────────────

func (s *Service) toolCreateTaskBinding(ctx context.Context, _ *mcp.CallToolRequest, in createTaskBindingParams) (*mcp.CallToolResult, any, error) {
	if s.deps.TaskBindings == nil {
		return textResult("错误: task_binding 服务不可用"), nil, nil
	}
	// 对齐 cc-pane MCP：不暴露 todoId（硬编码 None）。
	tb, err := s.deps.TaskBindings.Create(ctx, taskbinding.CreateInput{
		Title:         in.Title,
		Prompt:        optStr(in.Prompt),
		SessionID:     optStr(in.SessionID),
		ProjectPath:   in.ProjectPath,
		WorkspaceName: optStr(in.WorkspaceName),
		CliTool:       optStr(in.CliTool),
	})
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(tb), nil, nil
}

func (s *Service) toolUpdateTaskBinding(ctx context.Context, _ *mcp.CallToolRequest, in updateTaskBindingParams) (*mcp.CallToolResult, any, error) {
	if s.deps.TaskBindings == nil {
		return textResult("错误: task_binding 服务不可用"), nil, nil
	}
	// 对齐 cc-pane MCP：不暴露 prompt（固定不更新）。
	tb, err := s.deps.TaskBindings.Update(ctx, in.ID, taskbinding.UpdateInput{
		Title:             optStr(in.Title),
		Status:            enumOrNil(in.Status, tbStatuses),
		Progress:          in.Progress,
		CompletionSummary: optStr(in.CompletionSummary),
		SessionID:         optStr(in.SessionID),
		ExitCode:          in.ExitCode,
	})
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(tb), nil, nil
}

func (s *Service) toolQueryTaskBindings(ctx context.Context, _ *mcp.CallToolRequest, in queryTaskBindingsParams) (*mcp.CallToolResult, any, error) {
	if s.deps.TaskBindings == nil {
		return textResult("错误: task_binding 服务不可用"), nil, nil
	}
	q := taskbinding.Query{
		Status: enumOrNil(in.Status, tbStatuses), ProjectPath: optStr(in.ProjectPath), Search: optStr(in.Search),
	}
	if in.Limit > 0 {
		q.Limit = &in.Limit
	}
	res, err := s.deps.TaskBindings.Query(ctx, q)
	if err != nil {
		return textResult("错误: " + err.Error()), nil, nil
	}
	return jsonResult(res), nil, nil
}
