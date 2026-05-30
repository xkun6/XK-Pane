package memorysvc

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

func TestStoreDefaultsValidationAndClamp(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// global scope 无需 ref；默认 category=fact / source=user / importance=3。
	m, err := svc.Store(ctx, StoreMemoryInput{Title: "G", Content: "c", Scope: sp("global")})
	if err != nil {
		t.Fatalf("Store(global): %v", err)
	}
	if m.Scope != "global" || m.Category != "fact" || m.Source != "user" || m.Importance != 3 {
		t.Errorf("defaults wrong: %+v", m)
	}

	// importance 钳到 1-5。
	if hi, _ := svc.Store(ctx, StoreMemoryInput{Title: "hi", Content: "c", Scope: sp("global"), Importance: ip(99)}); hi.Importance != 5 {
		t.Errorf("importance 99 clamped = %d, want 5", hi.Importance)
	}
	if lo, _ := svc.Store(ctx, StoreMemoryInput{Title: "lo", Content: "c", Scope: sp("global"), Importance: ip(0)}); lo.Importance != 1 {
		t.Errorf("importance 0 clamped = %d, want 1", lo.Importance)
	}

	// 空标题 / 空内容报错。
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "  ", Content: "c", Scope: sp("global")}); err == nil {
		t.Error("Store(empty title) err=nil, want error")
	}
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: " ", Scope: sp("global")}); err == nil {
		t.Error("Store(empty content) err=nil, want error")
	}
}

func TestStoreScopeConstraints(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	// project（默认）无 projectPath → 报错。
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c"}); err == nil {
		t.Error("Store(project, no projectPath) err=nil, want error")
	}
	// workspace 无 workspaceName → 报错。
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c", Scope: sp("workspace")}); err == nil {
		t.Error("Store(workspace, no name) err=nil, want error")
	}
	// session 无 sessionId → 报错。
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c", Scope: sp("session")}); err == nil {
		t.Error("Store(session, no id) err=nil, want error")
	}
	// project + projectPath → ok。
	if _, err := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c", ProjectPath: sp("/p")}); err != nil {
		t.Errorf("Store(project + path) err=%v, want nil", err)
	}
}

func seed(t *testing.T, svc *Service, ctx context.Context) {
	t.Helper()
	mk := func(title, content, category string, importance int, tags []string) {
		if _, err := svc.Store(ctx, StoreMemoryInput{
			Title: title, Content: content, Scope: sp("global"),
			Category: sp(category), Importance: ip(importance), Tags: &tags,
		}); err != nil {
			t.Fatalf("seed %q: %v", title, err)
		}
	}
	mk("login bug decision", "we chose JWT", "decision", 5, []string{"auth"})
	mk("naming lesson", "avoid abbreviations", "lesson", 2, []string{"style"})
	mk("editor preference", "spaces over tabs", "preference", 3, []string{"style"})
}

func TestSearchFilterPaginate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	seed(t, svc, ctx)

	// 全部（无过滤）：3。
	if r, err := svc.Search(ctx, MemoryQuery{}); err != nil || r.Total != 3 {
		t.Fatalf("Search(all) = (%d, %v), want 3", r.Total, err)
	}
	// 按 category。
	if r, _ := svc.Search(ctx, MemoryQuery{Category: sp("decision")}); r.Total != 1 {
		t.Errorf("Search(category=decision) = %d, want 1", r.Total)
	}
	// minImportance >= 3 → decision(5) + preference(3) = 2。
	if r, _ := svc.Search(ctx, MemoryQuery{MinImportance: ip(3)}); r.Total != 2 {
		t.Errorf("Search(minImportance>=3) = %d, want 2", r.Total)
	}
	// 全文搜索（content LIKE）。
	if r, _ := svc.Search(ctx, MemoryQuery{Search: sp("JWT")}); r.Total != 1 || r.Items[0].Title != "login bug decision" {
		t.Errorf("Search(JWT) = %+v, want only login bug decision", r)
	}
	// 按 tag。
	if r, _ := svc.Search(ctx, MemoryQuery{Tags: &[]string{"style"}}); r.Total != 2 {
		t.Errorf("Search(tag=style) = %d, want 2", r.Total)
	}
	// 分页：limit 2 → 首页 2 + hasMore。
	r1, _ := svc.Search(ctx, MemoryQuery{Limit: ip(2), Offset: ip(0)})
	if r1.Total != 3 || len(r1.Items) != 2 || !r1.HasMore {
		t.Errorf("Search(limit=2) = total %d / %d items / more %v, want 3/2/true", r1.Total, len(r1.Items), r1.HasMore)
	}
	r2, _ := svc.Search(ctx, MemoryQuery{Limit: ip(2), Offset: ip(2)})
	if len(r2.Items) != 1 || r2.HasMore {
		t.Errorf("Search(offset=2) = %d items / more %v, want 1/false", len(r2.Items), r2.HasMore)
	}
}

func TestGetTouchesWithoutBumpingUpdated(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	m, _ := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c", Scope: sp("global")})

	got, err := svc.Get(ctx, m.ID)
	if err != nil || got == nil {
		t.Fatalf("Get = (%v, %v)", got, err)
	}
	// 读裸行确认 touch 生效但未 bump updated_at。
	var raw storage.Memory
	if err := svc.db().First(&raw, "id = ?", m.ID).Error; err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw.AccessCount != 1 {
		t.Errorf("accessCount after Get = %d, want 1", raw.AccessCount)
	}
	if !raw.UpdatedAt.Equal(raw.CreatedAt) {
		t.Errorf("touch bumped updated_at: created=%v updated=%v (want equal)", raw.CreatedAt, raw.UpdatedAt)
	}
	// accessedAt 应被 touch 更新到 now（>= created；不可早于）。注意不能断言「严格晚于」——
	// Windows 计时器分辨率下相邻 time.Now() 可能落同一刻度（access_count==1 已证 touch 执行）。
	if raw.AccessedAt.Before(raw.CreatedAt) {
		t.Errorf("accessedAt %v before createdAt %v", raw.AccessedAt, raw.CreatedAt)
	}
}

func TestUpdateAndValidate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	m, _ := svc.Store(ctx, StoreMemoryInput{Title: "old", Content: "c", Scope: sp("global")})

	if err := svc.Update(ctx, m.ID, UpdateMemoryInput{Title: sp("new"), Importance: ip(5)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := svc.Get(ctx, m.ID)
	if got.Title != "new" || got.Importance != 5 {
		t.Errorf("after update = %+v, want title=new importance=5", got)
	}
	// 越界 importance 报错。
	if err := svc.Update(ctx, m.ID, UpdateMemoryInput{Importance: ip(9)}); err == nil {
		t.Error("Update(importance=9) err=nil, want error")
	}
	// 未知 id 报错。
	if err := svc.Update(ctx, "nope", UpdateMemoryInput{Title: sp("x")}); err == nil {
		t.Error("Update(unknown) err=nil, want error")
	}
}

func TestSoftDelete(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	m, _ := svc.Store(ctx, StoreMemoryInput{Title: "t", Content: "c", Scope: sp("global")})

	if err := svc.Delete(ctx, m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// 软删后 Get 视为不存在、Search 不返回。
	if got, _ := svc.Get(ctx, m.ID); got != nil {
		t.Error("Get after soft-delete returned a row, want nil")
	}
	if r, _ := svc.Search(ctx, MemoryQuery{}); r.Total != 0 {
		t.Errorf("Search after soft-delete total = %d, want 0", r.Total)
	}
	// 行仍在但 is_deleted=1。
	var raw storage.Memory
	if err := svc.db().First(&raw, "id = ?", m.ID).Error; err != nil || !raw.IsDeleted {
		t.Errorf("raw isDeleted = %v (err %v), want true", raw.IsDeleted, err)
	}
	// 再删报错（已删）。
	if err := svc.Delete(ctx, m.ID); err == nil {
		t.Error("Delete(already deleted) err=nil, want error")
	}
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	seed(t, svc, ctx) // 3 global: decision/lesson/preference

	st, err := svc.Stats(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Total != 3 {
		t.Errorf("total = %d, want 3", st.Total)
	}
	if st.ByScope["global"] != 3 {
		t.Errorf("byScope = %+v, want global:3", st.ByScope)
	}
	if st.ByCategory["decision"] != 1 || st.ByCategory["lesson"] != 1 || st.ByCategory["preference"] != 1 {
		t.Errorf("byCategory = %+v, want decision/lesson/preference each 1", st.ByCategory)
	}
}
