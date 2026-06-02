package taskbinding

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
func ip(i int) *int       { return &i }

func TestCreateDefaults(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// 仅 title/projectPath → 默认 status=pending、progress=0、cliTool=reclaude。
	tb, err := svc.Create(ctx, CreateInput{Title: "  build feature  ", ProjectPath: "/p"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tb.ID == "" {
		t.Fatalf("missing id")
	}
	if tb.Title != "build feature" {
		t.Errorf("title not trimmed: %q", tb.Title)
	}
	if tb.Status != "pending" || tb.Progress != 0 || tb.CliTool != "reclaude" || tb.SortOrder != 0 {
		t.Errorf("defaults wrong: %+v", tb)
	}
	if tb.CreatedAt.IsZero() || tb.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: %+v", tb)
	}

	// 显式 cliTool + 关联字段。
	tb2, err := svc.Create(ctx, CreateInput{
		Title: "x", ProjectPath: "/p", CliTool: sp("codex"),
		Prompt: sp("do it"), TodoID: sp("todo-1"), SessionID: sp("sess-1"), WorkspaceName: sp("ws"),
	})
	if err != nil {
		t.Fatalf("Create2: %v", err)
	}
	if tb2.CliTool != "codex" || tb2.TodoID == nil || *tb2.TodoID != "todo-1" || tb2.SessionID == nil {
		t.Errorf("explicit fields wrong: %+v", tb2)
	}
}

func TestCreateEmptyTitle(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if _, err := svc.Create(ctx, CreateInput{Title: "   ", ProjectPath: "/p"}); err == nil {
		t.Fatal("expected empty-title error")
	}
}

func TestGetAndFindBySession(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	got, err := svc.Get(ctx, "nope")
	if err != nil || got != nil {
		t.Fatalf("Get(miss) = %v,%v want nil,nil", got, err)
	}

	tb, _ := svc.Create(ctx, CreateInput{Title: "t", ProjectPath: "/p", SessionID: sp("sess-A")})
	got, err = svc.Get(ctx, tb.ID)
	if err != nil || got == nil || got.ID != tb.ID {
		t.Fatalf("Get(hit) wrong: %v,%v", got, err)
	}
	bySess, err := svc.FindBySession(ctx, "sess-A")
	if err != nil || bySess == nil || bySess.ID != tb.ID {
		t.Fatalf("FindBySession wrong: %v,%v", bySess, err)
	}
	if miss, _ := svc.FindBySession(ctx, "ghost"); miss != nil {
		t.Fatalf("FindBySession(miss) = %+v want nil", miss)
	}
}

func TestUpdatePatchAndValidation(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	tb, _ := svc.Create(ctx, CreateInput{Title: "t", ProjectPath: "/p"})

	// 正常 patch：status + progress + completion_summary + session_id。
	upd, err := svc.Update(ctx, tb.ID, UpdateInput{
		Status: sp("running"), Progress: ip(40), SessionID: sp("s1"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Status != "running" || upd.Progress != 40 || upd.SessionID == nil || *upd.SessionID != "s1" {
		t.Errorf("patch not applied: %+v", upd)
	}
	if !upd.UpdatedAt.After(tb.UpdatedAt) && upd.UpdatedAt.Equal(tb.UpdatedAt) {
		t.Errorf("updated_at not bumped")
	}

	// progress 越界。
	if _, err := svc.Update(ctx, tb.ID, UpdateInput{Progress: ip(101)}); err == nil {
		t.Error("expected progress>100 error")
	}
	if _, err := svc.Update(ctx, tb.ID, UpdateInput{Progress: ip(-1)}); err == nil {
		t.Error("expected progress<0 error")
	}
	// 非法 status。
	if _, err := svc.Update(ctx, tb.ID, UpdateInput{Status: sp("bogus")}); err == nil {
		t.Error("expected invalid-status error")
	}
	// 空 title。
	if _, err := svc.Update(ctx, tb.ID, UpdateInput{Title: sp("  ")}); err == nil {
		t.Error("expected empty-title error")
	}
	// 不存在。
	if _, err := svc.Update(ctx, "ghost", UpdateInput{Status: sp("done")}); err == nil {
		// "done" 也非法，换合法的再测 not-found
	}
	if _, err := svc.Update(ctx, "ghost", UpdateInput{Status: sp("completed")}); err == nil {
		t.Error("expected not-found error")
	}
	// 空 patch → 返回现有记录（不报错）。
	same, err := svc.Update(ctx, tb.ID, UpdateInput{})
	if err != nil || same.ID != tb.ID {
		t.Errorf("empty patch wrong: %v,%v", same, err)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	tb, _ := svc.Create(ctx, CreateInput{Title: "t", ProjectPath: "/p"})
	if err := svc.Delete(ctx, tb.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(ctx, tb.ID); err == nil {
		t.Error("expected not-found on second delete")
	}
}

func TestQueryFiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// 6 条：3 个 /a（其中 1 running），3 个 /b。
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(ctx, CreateInput{Title: "alpha task", ProjectPath: "/a"})
	}
	r, _ := svc.Create(ctx, CreateInput{Title: "beta running", ProjectPath: "/b"})
	_, _ = svc.Update(ctx, r.ID, UpdateInput{Status: sp("running")})
	_, _ = svc.Create(ctx, CreateInput{Title: "beta two", ProjectPath: "/b"})
	_, _ = svc.Create(ctx, CreateInput{Title: "gamma", ProjectPath: "/b"})

	// projectPath 过滤。
	res, err := svc.Query(ctx, Query{ProjectPath: sp("/a")})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total != 3 || len(res.Items) != 3 {
		t.Errorf("projectPath filter total=%d items=%d want 3/3", res.Total, len(res.Items))
	}
	// status 过滤。
	res, _ = svc.Query(ctx, Query{Status: sp("running")})
	if res.Total != 1 {
		t.Errorf("status filter total=%d want 1", res.Total)
	}
	// search（title LIKE）。
	res, _ = svc.Query(ctx, Query{Search: sp("beta")})
	if res.Total != 2 {
		t.Errorf("search total=%d want 2", res.Total)
	}
	// 分页 hasMore。
	res, _ = svc.Query(ctx, Query{Limit: ip(2), Offset: ip(0)})
	if res.Total != 6 || len(res.Items) != 2 || !res.HasMore {
		t.Errorf("page1 total=%d items=%d hasMore=%v want 6/2/true", res.Total, len(res.Items), res.HasMore)
	}
	res, _ = svc.Query(ctx, Query{Limit: ip(2), Offset: ip(4)})
	if res.HasMore {
		t.Errorf("page3 hasMore=true want false")
	}
}
