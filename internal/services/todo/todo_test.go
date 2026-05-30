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
