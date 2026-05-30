package todo

import (
	"context"
	"path/filepath"
	"testing"

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
		if _, err := svc.Create(ctx, CreateTodoInput{
			Title: title, Status: sp(status), Priority: sp(priority), Scope: sp(scope), Tags: &tags,
		}); err != nil {
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
