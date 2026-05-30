package workspace

import (
	"context"
	"path/filepath"
	"testing"

	"xkpane/internal/storage"
)

func newTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(storage.NewHolder(db)), context.Background()
}

// TestListOrdering 验证 List 排序：Pinned 优先 → SortOrder 升序 → CreatedAt。
func TestListOrdering(t *testing.T) {
	s, ctx := newTestService(t)

	a, err := s.Create(ctx, "A", nil)
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := s.Create(ctx, "B", nil)
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	c, err := s.Create(ctx, "C", nil)
	if err != nil {
		t.Fatalf("create C: %v", err)
	}

	if err := s.SetPinned(ctx, c.ID, true); err != nil {
		t.Fatalf("pin C: %v", err)
	}
	// 把 B 排到 A 前面（B.sort_order=0, A.sort_order=1）。
	if err := s.Reorder(ctx, []string{b.ID, a.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{c.ID, b.ID, a.ID} // C 置顶；其余按 sort_order
	if len(list) != len(want) {
		t.Fatalf("want %d workspaces, got %d", len(want), len(list))
	}
	for i, w := range want {
		if list[i].ID != w {
			t.Errorf("pos %d: want %s got %s", i, w, list[i].ID)
		}
		if list[i].Projects == nil {
			t.Errorf("pos %d: Projects should be non-nil empty slice", i)
		}
	}
}

// TestAddProjectDedup 验证同一 workspace 内归一化 path 去重 + 幂等返回。
func TestAddProjectDedup(t *testing.T) {
	s, ctx := newTestService(t)
	ws, err := s.Create(ctx, "W", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	p1, err := s.AddProject(ctx, ws.ID, `C:\Work\proj`)
	if err != nil {
		t.Fatalf("add p1: %v", err)
	}
	// 斜杠 / 尾斜杠差异 → 归一化后相同 → 幂等返回同一条。
	p2, err := s.AddProject(ctx, ws.ID, "C:/Work/proj/")
	if err != nil {
		t.Fatalf("add p2: %v", err)
	}
	if p2.ID != p1.ID {
		t.Errorf("expected idempotent dedup, got different ids %s vs %s", p1.ID, p2.ID)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || len(list[0].Projects) != 1 {
		t.Fatalf("expected 1 workspace with 1 project, got %d ws / %d projects",
			len(list), func() int {
				if len(list) > 0 {
					return len(list[0].Projects)
				}
				return 0
			}())
	}
	// 存储保留原始 path（不写归一化值）。
	if list[0].Projects[0].Path != `C:\Work\proj` {
		t.Errorf("expected original path stored, got %q", list[0].Projects[0].Path)
	}
}

// TestDeleteCascade 验证删除工作空间级联删其下项目节点。
func TestDeleteCascade(t *testing.T) {
	s, ctx := newTestService(t)
	ws, err := s.Create(ctx, "W", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.AddProject(ctx, ws.ID, "/a"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := s.Delete(ctx, ws.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 workspaces after delete, got %d", len(list))
	}
}

// TestCreateDuplicateName 验证重名拒绝。
func TestCreateDuplicateName(t *testing.T) {
	s, ctx := newTestService(t)
	if _, err := s.Create(ctx, "Dup", nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Create(ctx, "Dup", nil); err == nil {
		t.Error("expected duplicate-name error")
	}
}
