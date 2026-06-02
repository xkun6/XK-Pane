package todo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xkpane/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(storage.NewHolder(db))
}

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func ip(i int) *int       { return &i }

func TestCreateDefaultsAndTags(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// 仅 title → 默认 status/priority/scope。
	a, err := svc.Create(ctx, CreateTodoInput{Title: "first"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == "" || a.Status != "todo" || a.Priority != "medium" || a.Scope != "global" {
		t.Fatalf("defaults wrong: %+v", a)
	}
	if a.SortOrder != 0 {
		t.Errorf("first sortOrder = %d, want 0", a.SortOrder)
	}
	if len(a.Tags) != 0 {
		t.Errorf("default tags = %v, want empty", a.Tags)
	}

	// 全字段 + tags。
	b, err := svc.Create(ctx, CreateTodoInput{
		Title: "second", Description: sp("desc"), Status: sp("in_progress"),
		Priority: sp("high"), Tags: &[]string{"x", "y"},
	})
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}
	if b.SortOrder != 1 {
		t.Errorf("second sortOrder = %d, want 1 (append)", b.SortOrder)
	}
	if len(b.Tags) != 2 || b.Tags[0] != "x" {
		t.Errorf("tags = %v, want [x y]", b.Tags)
	}

	// 空 title 报错。
	if _, err := svc.Create(ctx, CreateTodoInput{}); err == nil {
		t.Error("Create empty title err=nil, want error")
	}

	// List 按 sortOrder 升序。
	list, _ := svc.List(ctx)
	if len(list) != 2 || list[0].Title != "first" || list[1].Title != "second" {
		t.Fatalf("list order wrong: %+v", list)
	}

	// Get miss → nil,nil。
	if got, err := svc.Get(ctx, "nope"); err != nil || got != nil {
		t.Errorf("Get(miss) = %v err=%v, want nil,nil", got, err)
	}
}

func TestUpdatePatch(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	a, _ := svc.Create(ctx, CreateTodoInput{Title: "t", Priority: sp("low")})

	// 仅改 status + tags，其余不动。
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{Status: sp("done"), Tags: &[]string{"z"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(ctx, a.ID)
	if got.Status != "done" || got.Priority != "low" || len(got.Tags) != 1 || got.Tags[0] != "z" {
		t.Fatalf("patch wrong: %+v", got)
	}
	if got.Title != "t" {
		t.Errorf("title changed unexpectedly: %q", got.Title)
	}

	// 未知 id → 报错。
	if err := svc.Update(ctx, "nope", UpdateTodoInput{Status: sp("done")}); err == nil {
		t.Error("Update(unknown) err=nil, want error")
	}
}

func TestSubtasksAndCascadeDelete(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	a, _ := svc.Create(ctx, CreateTodoInput{Title: "parent"})

	s1, err := svc.AddSubtask(ctx, a.ID, "sub1")
	if err != nil {
		t.Fatalf("AddSubtask: %v", err)
	}
	s2, _ := svc.AddSubtask(ctx, a.ID, "sub2")
	if s1.SortOrder != 0 || s2.SortOrder != 1 {
		t.Errorf("subtask sortOrder = %d,%d want 0,1", s1.SortOrder, s2.SortOrder)
	}

	// 子任务随 Get 加载，按序。
	got, _ := svc.Get(ctx, a.ID)
	if len(got.Subtasks) != 2 || got.Subtasks[0].Title != "sub1" || got.Subtasks[1].Title != "sub2" {
		t.Fatalf("subtasks load wrong: %+v", got.Subtasks)
	}

	// 完成 s1。
	if err := svc.UpdateSubtask(ctx, s1.ID, nil, bp(true)); err != nil {
		t.Fatalf("UpdateSubtask: %v", err)
	}
	got, _ = svc.Get(ctx, a.ID)
	if !got.Subtasks[0].Completed {
		t.Error("s1 not marked completed")
	}

	// 删 s2。
	if err := svc.DeleteSubtask(ctx, s2.ID); err != nil {
		t.Fatalf("DeleteSubtask: %v", err)
	}
	got, _ = svc.Get(ctx, a.ID)
	if len(got.Subtasks) != 1 {
		t.Fatalf("after delete subtask = %d, want 1", len(got.Subtasks))
	}

	// 删 todo → 子任务连带删除。
	if err := svc.Delete(ctx, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var remaining int64
	svc.db().WithContext(ctx).Model(&storage.TodoSubtask{}).Where("todo_id = ?", a.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("subtasks after todo delete = %d, want 0", remaining)
	}
	// 未知 id 删除报错。
	if err := svc.Delete(ctx, "nope"); err == nil {
		t.Error("Delete(unknown) err=nil, want error")
	}
	// AddSubtask 到不存在的 todo 报错。
	if _, err := svc.AddSubtask(ctx, "nope", "x"); err == nil {
		t.Error("AddSubtask(unknown todo) err=nil, want error")
	}
}

// 播种若干待办供 Query / Stats 测试用。
func seedForQuery(t *testing.T, svc *Service, ctx context.Context) {
	t.Helper()
	mk := func(title, status, priority, scope string, tags []string) {
		in := CreateTodoInput{
			Title: title, Status: sp(status), Priority: sp(priority), Scope: sp(scope), Tags: &tags,
		}
		if scope == "project" || scope == "workspace" {
			in.ScopeRef = sp("/seed") // scope_ref 必填，播种统一给一个引用
		}
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("seed %q: %v", title, err)
		}
	}
	mk("fix login bug", "todo", "high", "global", []string{"bug"})
	mk("add feature", "in_progress", "medium", "global", []string{"feature"})
	mk("write docs", "done", "low", "project", []string{"docs"})
	mk("review PR", "todo", "high", "project", []string{"bug", "review"})
}

func TestQueryFilterSearchAndPaginate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	seedForQuery(t, svc, ctx)

	// 无过滤：全 4 条。
	if r, err := svc.Query(ctx, TodoQuery{}); err != nil || r.Total != 4 || len(r.Items) != 4 || r.HasMore {
		t.Fatalf("Query(all) = (%+v, %v), want total 4 / 4 items / no more", r, err)
	}

	// 按状态过滤。
	if r, _ := svc.Query(ctx, TodoQuery{Status: sp("todo")}); r.Total != 2 {
		t.Errorf("Query(status=todo) total = %d, want 2", r.Total)
	}
	// 按优先级过滤。
	if r, _ := svc.Query(ctx, TodoQuery{Priority: sp("high")}); r.Total != 2 {
		t.Errorf("Query(priority=high) total = %d, want 2", r.Total)
	}
	// 按 scope 过滤。
	if r, _ := svc.Query(ctx, TodoQuery{Scope: sp("project")}); r.Total != 2 {
		t.Errorf("Query(scope=project) total = %d, want 2", r.Total)
	}
	// 全文搜索（标题）。
	if r, _ := svc.Query(ctx, TodoQuery{Search: sp("login")}); r.Total != 1 || r.Items[0].Title != "fix login bug" {
		t.Errorf("Query(search=login) = %+v, want only 'fix login bug'", r)
	}
	// 按标签（tags JSON 精确匹配）。
	if r, _ := svc.Query(ctx, TodoQuery{Tag: sp("bug")}); r.Total != 2 {
		t.Errorf("Query(tag=bug) total = %d, want 2", r.Total)
	}
	// 分页：limit 3 → 首页 3 条 + hasMore。
	r1, _ := svc.Query(ctx, TodoQuery{Limit: ip(3), Offset: ip(0)})
	if r1.Total != 4 || len(r1.Items) != 3 || !r1.HasMore {
		t.Errorf("Query(limit=3) = total %d / %d items / hasMore %v, want 4 / 3 / true", r1.Total, len(r1.Items), r1.HasMore)
	}
	r2, _ := svc.Query(ctx, TodoQuery{Limit: ip(3), Offset: ip(3)})
	if len(r2.Items) != 1 || r2.HasMore {
		t.Errorf("Query(offset=3) = %d items / hasMore %v, want 1 / false", len(r2.Items), r2.HasMore)
	}
}

func TestStatsGrouping(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	seedForQuery(t, svc, ctx)

	st, err := svc.Stats(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Total != 4 {
		t.Errorf("stats total = %d, want 4", st.Total)
	}
	if st.ByStatus["todo"] != 2 || st.ByStatus["in_progress"] != 1 || st.ByStatus["done"] != 1 {
		t.Errorf("byStatus = %+v, want todo:2 in_progress:1 done:1", st.ByStatus)
	}
	if st.ByScope["global"] != 2 || st.ByScope["project"] != 2 {
		t.Errorf("byScope = %+v, want global:2 project:2", st.ByScope)
	}
	if st.ByPriority["high"] != 2 || st.ByPriority["medium"] != 1 || st.ByPriority["low"] != 1 {
		t.Errorf("byPriority = %+v, want high:2 medium:1 low:1", st.ByPriority)
	}
	if st.Overdue != 0 {
		t.Errorf("overdue = %d, want 0 (no due dates seeded)", st.Overdue)
	}

	// scope 限定：仅 project（2 条）。
	if sp2, _ := svc.Stats(ctx, sp("project"), nil); sp2.Total != 2 {
		t.Errorf("Stats(scope=project) total = %d, want 2", sp2.Total)
	}
}

func TestStatsOverdue(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	past := "2000-01-01T00:00:00Z"
	future := "2999-01-01T00:00:00Z"
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "overdue", DueDate: sp(past)}); err != nil {
		t.Fatalf("create overdue: %v", err)
	}
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "future", DueDate: sp(future)}); err != nil {
		t.Fatalf("create future: %v", err)
	}
	// 过期但已完成的不计入 overdue。
	doneOverdue, err := svc.Create(ctx, CreateTodoInput{Title: "done-overdue", DueDate: sp(past)})
	if err != nil {
		t.Fatalf("create done-overdue: %v", err)
	}
	if err := svc.Update(ctx, doneOverdue.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	st, err := svc.Stats(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Overdue != 1 {
		t.Errorf("overdue = %d, want 1 (past+notdone only; future and done-overdue excluded)", st.Overdue)
	}
}

func TestReorder(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	a, _ := svc.Create(ctx, CreateTodoInput{Title: "A"})
	b, _ := svc.Create(ctx, CreateTodoInput{Title: "B"})
	c, _ := svc.Create(ctx, CreateTodoInput{Title: "C"})

	// 重排为 C, A, B。
	if err := svc.Reorder(ctx, []string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	list, err := svc.List(ctx) // List 按 sort_order, created_at 升序
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 || list[0].ID != c.ID || list[1].ID != a.ID || list[2].ID != b.ID {
		t.Fatalf("order after reorder = [%s %s %s], want [C A B]", list[0].Title, list[1].Title, list[2].Title)
	}
	if list[0].SortOrder != 0 || list[1].SortOrder != 1 || list[2].SortOrder != 2 {
		t.Errorf("sortOrder after reorder = %d,%d,%d want 0,1,2", list[0].SortOrder, list[1].SortOrder, list[2].SortOrder)
	}
}

func TestBatchUpdateStatus(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	a, _ := svc.Create(ctx, CreateTodoInput{Title: "A"})
	b, _ := svc.Create(ctx, CreateTodoInput{Title: "B"})
	c, _ := svc.Create(ctx, CreateTodoInput{Title: "C"})

	// 批量把 A、B 标 done → 影响 2 行。
	n, err := svc.BatchUpdateStatus(ctx, []string{a.ID, b.ID}, "done")
	if err != nil {
		t.Fatalf("BatchUpdateStatus: %v", err)
	}
	if n != 2 {
		t.Errorf("affected = %d, want 2", n)
	}
	ga, _ := svc.Get(ctx, a.ID)
	gb, _ := svc.Get(ctx, b.ID)
	gc, _ := svc.Get(ctx, c.ID)
	if ga.Status != "done" || gb.Status != "done" {
		t.Errorf("A/B status = %q/%q, want done", ga.Status, gb.Status)
	}
	if gc.Status != "todo" {
		t.Errorf("C status = %q, want unchanged (todo)", gc.Status)
	}

	// 含未知 id：仅存在的计入。
	if n, _ := svc.BatchUpdateStatus(ctx, []string{c.ID, "nope"}, "in_progress"); n != 1 {
		t.Errorf("affected with unknown id = %d, want 1", n)
	}
	// 空 ids → 0；空 status → 报错。
	if n, err := svc.BatchUpdateStatus(ctx, nil, "done"); err != nil || n != 0 {
		t.Errorf("BatchUpdateStatus(empty) = (%d, %v), want (0, nil)", n, err)
	}
	if _, err := svc.BatchUpdateStatus(ctx, []string{a.ID}, ""); err == nil {
		t.Error("BatchUpdateStatus(empty status) err=nil, want error")
	}
}

// TestTitleTrim 锁保真修复：Create/Update/AddSubtask 对 title 去空后判空、存 trim 后值（对齐 cc-pane）。
func TestTitleTrim(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// Create：首尾空格被 trim 落库。
	a, err := svc.Create(ctx, CreateTodoInput{Title: "  hello  "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.Title != "hello" {
		t.Errorf("Create title = %q, want trimmed %q", a.Title, "hello")
	}
	// Create：纯空白被拒。
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "   "}); err == nil {
		t.Error("Create(whitespace-only) err=nil, want error")
	}
	// Update：纯空白 title 被拒。
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{Title: sp("   ")}); err == nil {
		t.Error("Update(whitespace-only title) err=nil, want error")
	}
	// AddSubtask：纯空白被拒；正常 trim。
	if _, err := svc.AddSubtask(ctx, a.ID, "  "); err == nil {
		t.Error("AddSubtask(whitespace-only) err=nil, want error")
	}
	sub, err := svc.AddSubtask(ctx, a.ID, "  task  ")
	if err != nil || sub.Title != "task" {
		t.Errorf("AddSubtask trim = (%q, %v), want (task, nil)", sub.Title, err)
	}
}

// listByTitle 返回所有同标题待办（轮转克隆会复制 title），用于断言下一实例已生成。
func listByTitle(t *testing.T, svc *Service, title string) []TodoDTO {
	t.Helper()
	all, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var out []TodoDTO
	for _, td := range all {
		if td.Title == title {
			out = append(out, td)
		}
	}
	return out
}

func TestRecurrenceRotation(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// daily：完成带 recurrence 的待办 → 生成一条 status=todo、due 前推一天的新实例。
	a, err := svc.Create(ctx, CreateTodoInput{
		Title: "daily-task", Priority: sp("high"), Scope: sp("project"), ScopeRef: sp("/p"),
		Tags: &[]string{"x"}, DueDate: sp("2026-06-10"), Recurrence: sp("daily"), TodoType: sp("chore"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Fatalf("Update done: %v", err)
	}
	got := listByTitle(t, svc, "daily-task")
	if len(got) != 2 {
		t.Fatalf("daily rotation: got %d todos, want 2", len(got))
	}
	// 找出新实例（非 a.ID）。
	var next *TodoDTO
	for i := range got {
		if got[i].ID != a.ID {
			next = &got[i]
		}
	}
	if next == nil {
		t.Fatal("no next instance created")
	}
	if next.Status != "todo" {
		t.Errorf("next status = %q, want todo", next.Status)
	}
	if next.DueDate == nil || *next.DueDate != "2026-06-11" {
		t.Errorf("next due = %v, want 2026-06-11", next.DueDate)
	}
	// 字段克隆。
	if next.Priority != "high" || next.Scope != "project" || next.ScopeRef == nil || *next.ScopeRef != "/p" ||
		len(next.Tags) != 1 || next.Tags[0] != "x" || next.TodoType != "chore" ||
		next.Recurrence == nil || *next.Recurrence != "daily" {
		t.Errorf("clone fields wrong: %+v", next)
	}

	// 已 done 再 Update done（非跃迁）不再克隆。
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Fatalf("Update done again: %v", err)
	}
	if n := len(listByTitle(t, svc, "daily-task")); n != 2 {
		t.Errorf("re-done should not clone: got %d, want 2", n)
	}

	// 非重复任务完成不克隆。
	b, _ := svc.Create(ctx, CreateTodoInput{Title: "plain", DueDate: sp("2026-06-10")})
	if err := svc.Update(ctx, b.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Fatalf("Update plain: %v", err)
	}
	if n := len(listByTitle(t, svc, "plain")); n != 1 {
		t.Errorf("non-recurring should not clone: got %d, want 1", n)
	}
}

func TestNextDueDate(t *testing.T) {
	cases := []struct {
		due, rec, want string
	}{
		{"2026-06-10", "daily", "2026-06-11"},
		{"2026-06-10", "weekly", "2026-06-17"},
		{"2026-06-10", "monthly", "2026-07-10"},
		{"2026-01-31", "monthly", "2026-02-28"},  // 月末钳制（2026 非闰年）
		{"2026-06-10T09:00:00Z", "daily", "2026-06-11"}, // 容忍时间部分
	}
	for _, c := range cases {
		got := nextDueDate(sp(c.due), c.rec)
		if got == nil || *got != c.want {
			t.Errorf("nextDueDate(%q,%q) = %v, want %q", c.due, c.rec, got, c.want)
		}
	}
	if nextDueDate(nil, "daily") != nil {
		t.Error("nil due should yield nil")
	}
}

func TestNextReminderAt(t *testing.T) {
	// reminder 比 due 早 1 小时 → 下一实例同样早 1 小时。
	due := "2026-06-10T10:00:00Z"
	reminder := "2026-06-10T09:00:00Z"
	nextDue := "2026-06-11"
	got := nextReminderAt(sp(due), sp(reminder), sp(nextDue))
	if got == nil || *got != "2026-06-10T23:00:00Z" {
		t.Errorf("nextReminderAt = %v, want 2026-06-10T23:00:00Z (next_due 00:00 - 1h)", got)
	}
	// 日期型 due（非 RFC3339）→ nil。
	if nextReminderAt(sp("2026-06-10"), sp(reminder), sp(nextDue)) != nil {
		t.Error("date-only due should yield nil reminder")
	}
}

func TestScopeRefRequired(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// Create: global 无需 ref。
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "g", Scope: sp("global")}); err != nil {
		t.Fatalf("global create: %v", err)
	}
	// Create: project 无 ref → 报错；空白 ref → 报错；有 ref → 成功。
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "p", Scope: sp("project")}); err == nil {
		t.Error("project without scopeRef should error")
	}
	if _, err := svc.Create(ctx, CreateTodoInput{Title: "p", Scope: sp("workspace"), ScopeRef: sp("   ")}); err == nil {
		t.Error("blank scopeRef should error")
	}
	a, err := svc.Create(ctx, CreateTodoInput{Title: "p", Scope: sp("project"), ScopeRef: sp("/proj")})
	if err != nil {
		t.Fatalf("project with ref: %v", err)
	}

	// Update: 纯状态切换（不碰 scope）即使……此处 a 有 ref，应通过。
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{Status: sp("in_progress")}); err != nil {
		t.Errorf("status-only update should pass: %v", err)
	}
	// Update: 改 scopeRef 为空 → 报错。
	if err := svc.Update(ctx, a.ID, UpdateTodoInput{ScopeRef: sp("")}); err == nil {
		t.Error("clearing scopeRef on project todo should error")
	}
	// Update: global todo 改 scope=project 但不给 ref → 报错。
	g, _ := svc.Create(ctx, CreateTodoInput{Title: "g2"})
	if err := svc.Update(ctx, g.ID, UpdateTodoInput{Scope: sp("project")}); err == nil {
		t.Error("switching to project without ref should error")
	}
	// Update: 同时给 scope + ref → 成功。
	if err := svc.Update(ctx, g.ID, UpdateTodoInput{Scope: sp("project"), ScopeRef: sp("/x")}); err != nil {
		t.Errorf("scope+ref update should pass: %v", err)
	}
	// Update: 纯状态切换 global todo（不碰 scope）→ 通过。
	g3, _ := svc.Create(ctx, CreateTodoInput{Title: "g3"})
	if err := svc.Update(ctx, g3.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Errorf("global status-only update should pass: %v", err)
	}
}

func TestCheckReminders(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	past := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)

	// 到期（过去）未完成 → 命中。
	due, _ := svc.Create(ctx, CreateTodoInput{Title: "due", ReminderAt: sp(past)})
	// 未到期（将来）→ 不命中。
	svc.Create(ctx, CreateTodoInput{Title: "later", ReminderAt: sp(future)})
	// 无提醒 → 不命中。
	svc.Create(ctx, CreateTodoInput{Title: "noremind"})
	// 到期但已完成 → 不命中。
	done, _ := svc.Create(ctx, CreateTodoInput{Title: "donedue", ReminderAt: sp(past)})
	if err := svc.Update(ctx, done.ID, UpdateTodoInput{Status: sp("done")}); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	got, err := svc.CheckReminders(ctx)
	if err != nil {
		t.Fatalf("CheckReminders: %v", err)
	}
	if len(got) != 1 || got[0].ID != due.ID {
		ids := make([]string, len(got))
		for i, g := range got {
			ids[i] = g.Title
		}
		t.Fatalf("CheckReminders = %v, want only [due]", ids)
	}
}
