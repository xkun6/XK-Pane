package orchestrator

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"xkpane/internal/services/taskbinding"
	"xkpane/internal/services/todo"
	"xkpane/internal/storage"
)

// newTodoToolsService 构造一个挂真 todo + taskbinding 服务（临时库）的 orchestrator.Service。
func newTodoToolsService(t *testing.T) *Service {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	holder := storage.NewHolder(db)
	return New(Deps{Todos: todo.New(holder), TaskBindings: taskbinding.New(holder)})
}

func TestTodoToolsNilDeps(t *testing.T) {
	ctx := context.Background()
	s := New(Deps{}) // 无 Todos/TaskBindings
	for _, got := range []string{
		resultText(t, mustCall(s.toolQueryTodos(ctx, nil, queryTodosParams{}))),
		resultText(t, mustCall(s.toolCreateTodo(ctx, nil, createTodoParams{Title: "x"}))),
		resultText(t, mustCall(s.toolUpdateTodo(ctx, nil, updateTodoParams{ID: "x"}))),
		resultText(t, mustCall(s.toolCreateTaskBinding(ctx, nil, createTaskBindingParams{Title: "x", ProjectPath: "/p"}))),
		resultText(t, mustCall(s.toolUpdateTaskBinding(ctx, nil, updateTaskBindingParams{ID: "x"}))),
		resultText(t, mustCall(s.toolQueryTaskBindings(ctx, nil, queryTaskBindingsParams{}))),
	} {
		if !strings.HasPrefix(got, "错误:") {
			t.Errorf("nil deps 应返回错误文本，实际 %q", got)
		}
	}
}

func TestTodoToolsCreateQueryUpdate(t *testing.T) {
	ctx := context.Background()
	s := newTodoToolsService(t)

	// create_todo：仅 title → 默认 status=todo（忠实裁剪，不暴露 status）。
	created := resultText(t, mustCall(s.toolCreateTodo(ctx, nil, createTodoParams{
		Title: "fix bug", Priority: "high", Scope: "project", ScopeRef: "/proj", Tags: []string{"urgent"},
	})))
	var todoObj map[string]any
	if err := json.Unmarshal([]byte(created), &todoObj); err != nil {
		t.Fatalf("create_todo 返回非 JSON: %v (%s)", err, created)
	}
	id, _ := todoObj["id"].(string)
	if id == "" || todoObj["status"] != "todo" || todoObj["priority"] != "high" || todoObj["scope"] != "project" {
		t.Fatalf("create_todo 结果不符: %s", created)
	}

	// query_todos：按 status=todo 应命中。
	q := resultText(t, mustCall(s.toolQueryTodos(ctx, nil, queryTodosParams{Status: "todo"})))
	var qr struct {
		Total int `json:"total"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(q), &qr); err != nil {
		t.Fatalf("query_todos 返回非 JSON: %v (%s)", err, q)
	}
	if qr.Total != 1 || len(qr.Items) != 1 || qr.Items[0].ID != id {
		t.Fatalf("query_todos 结果不符: %s", q)
	}

	// update_todo：改 status=in_progress（dispatch 标记已派发），返回更新后的待办。
	upd := resultText(t, mustCall(s.toolUpdateTodo(ctx, nil, updateTodoParams{ID: id, Status: "in_progress"})))
	var updObj map[string]any
	if err := json.Unmarshal([]byte(upd), &updObj); err != nil {
		t.Fatalf("update_todo 返回非 JSON: %v (%s)", err, upd)
	}
	if updObj["status"] != "in_progress" {
		t.Fatalf("update_todo 未改 status: %s", upd)
	}
	// 不存在的 id。
	if got := resultText(t, mustCall(s.toolUpdateTodo(ctx, nil, updateTodoParams{ID: "ghost", Title: "y"}))); !strings.HasPrefix(got, "错误:") {
		t.Errorf("update_todo(ghost) 应报错，实际 %q", got)
	}
}

func TestTaskBindingTools(t *testing.T) {
	ctx := context.Background()
	s := newTodoToolsService(t)

	// create_task_binding：默认 status=pending、cliTool=reclaude（不暴露 todoId）。
	created := resultText(t, mustCall(s.toolCreateTaskBinding(ctx, nil, createTaskBindingParams{
		Title: "dispatch A", ProjectPath: "/proj", Prompt: "do it",
	})))
	var tb map[string]any
	if err := json.Unmarshal([]byte(created), &tb); err != nil {
		t.Fatalf("create_task_binding 返回非 JSON: %v (%s)", err, created)
	}
	id, _ := tb["id"].(string)
	if id == "" || tb["status"] != "pending" || tb["cliTool"] != "reclaude" {
		t.Fatalf("create_task_binding 默认值不符: %s", created)
	}
	if _, hasTodo := tb["todoId"]; hasTodo {
		t.Errorf("create_task_binding 不应带 todoId（对齐 cc-pane 硬编码 None）: %s", created)
	}

	// update_task_binding：status=running + progress=50。
	upd := resultText(t, mustCall(s.toolUpdateTaskBinding(ctx, nil, updateTaskBindingParams{
		ID: id, Status: "running", Progress: ptrInt(50),
	})))
	var tbu map[string]any
	if err := json.Unmarshal([]byte(upd), &tbu); err != nil {
		t.Fatalf("update_task_binding 返回非 JSON: %v (%s)", err, upd)
	}
	if tbu["status"] != "running" || tbu["progress"].(float64) != 50 {
		t.Fatalf("update_task_binding 结果不符: %s", upd)
	}
	// progress 越界 → 错误。
	if got := resultText(t, mustCall(s.toolUpdateTaskBinding(ctx, nil, updateTaskBindingParams{ID: id, Progress: ptrInt(150)}))); !strings.HasPrefix(got, "错误:") {
		t.Errorf("progress>100 应报错，实际 %q", got)
	}

	// query_task_bindings：按 projectPath 命中。
	q := resultText(t, mustCall(s.toolQueryTaskBindings(ctx, nil, queryTaskBindingsParams{ProjectPath: "/proj"})))
	var qr struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(q), &qr); err != nil || qr.Total != 1 {
		t.Fatalf("query_task_bindings 结果不符: %s (err %v)", q, err)
	}
}

func ptrInt(i int) *int { return &i }

// TestRegisterTodoToolsSchema 强制 buildMCPHandler 走 AddTool 注册全部工具（含 *int 参数的
// jsonschema 生成），确保 go-sdk 反射不 panic（直接调 handler 的其它测试覆盖不到注册路径）。
func TestRegisterTodoToolsSchema(t *testing.T) {
	if h := New(Deps{}).buildMCPHandler(); h == nil {
		t.Fatal("buildMCPHandler 返回 nil")
	}
}
