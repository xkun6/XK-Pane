package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xkpane/internal/storage"
)

// newTestService 用临时目录里的真实 SQLite（Open 会跑 AutoMigrate 建出 projects 表）
// 构造一个 service。白盒（同包）测试，可直达 svc.db()/existsByPath。
func newTestService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(storage.NewHolder(db))
}

func ptr(s string) *string { return &s }

// ---- validatePath（顺序：empty → traversal → absolute）----

func TestValidatePath(t *testing.T) {
	abs := t.TempDir() // 真实绝对路径
	sep := string(os.PathSeparator)
	traversal := abs + sep + ".." + sep + "x" // 绝对但含 ".." 分量

	cases := []struct {
		name string
		path string
		want error
	}{
		{"empty", "", ErrPathEmpty},
		{"traversal-relative", "sub/../etc", ErrPathTraversal},
		{"traversal-absolute", traversal, ErrPathTraversal},
		{"not-absolute", "relative-dir", ErrPathNotAbsolute},
		{"valid", abs, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validatePath(c.path); !errors.Is(got, c.want) {
				t.Fatalf("validatePath(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestDeriveName(t *testing.T) {
	// 真实目录的末段应作为 name。
	dir := t.TempDir()
	if got := deriveName(dir); got != filepath.Base(dir) {
		t.Fatalf("deriveName(%q) = %q, want %q", dir, got, filepath.Base(dir))
	}
	// 根类/无分量路径回退 "unnamed"。
	for _, p := range []string{".", "/", "\\"} {
		if got := deriveName(p); got != "unnamed" {
			t.Fatalf("deriveName(%q) = %q, want unnamed", p, got)
		}
	}
}

// ---- Add ----

func TestAddSuccess(t *testing.T) {
	svc := newTestService(t)
	dir := t.TempDir()

	p, err := svc.Add(context.Background(), dir)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if p.ID == "" {
		t.Error("expected generated id")
	}
	if p.Path != dir {
		t.Errorf("Path = %q, want verbatim %q", p.Path, dir)
	}
	if p.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want %q", p.Name, filepath.Base(dir))
	}
	if p.Alias != nil {
		t.Errorf("Alias = %v, want nil at creation", p.Alias)
	}
	if p.CreatedAt.IsZero() {
		t.Error("CreatedAt should be auto-set by GORM")
	}
}

func TestAddDuplicateExactPath(t *testing.T) {
	svc := newTestService(t)
	dir := t.TempDir()
	if _, err := svc.Add(context.Background(), dir); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	_, err := svc.Add(context.Background(), dir)
	if !errors.Is(err, errProjectExists) {
		t.Fatalf("second Add = %v, want errProjectExists", err)
	}
}

// 铁律：去重逐字节相等——同一目录的不同写法（带尾分隔符）算不同项目。
func TestAddVerbatimPathDistinct(t *testing.T) {
	svc := newTestService(t)
	dir := t.TempDir()
	withSlash := dir + string(os.PathSeparator)

	if _, err := svc.Add(context.Background(), dir); err != nil {
		t.Fatalf("Add(dir): %v", err)
	}
	if _, err := svc.Add(context.Background(), withSlash); err != nil {
		t.Fatalf("Add(dir+sep) should be a distinct project, got: %v", err)
	}
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2 (byte-exact dedup)", len(list))
	}
}

func TestAddNonExistent(t *testing.T) {
	svc := newTestService(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := svc.Add(context.Background(), missing)
	if !errors.Is(err, errPathNotExist) {
		t.Fatalf("Add(missing) = %v, want errPathNotExist", err)
	}
}

func TestAddNotDirectory(t *testing.T) {
	svc := newTestService(t)
	file := filepath.Join(t.TempDir(), "afile.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := svc.Add(context.Background(), file)
	if !errors.Is(err, errPathNotDir) {
		t.Fatalf("Add(file) = %v, want errPathNotDir", err)
	}
}

func TestAddValidatesFirst(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Add(context.Background(), "")
	if !errors.Is(err, ErrPathEmpty) {
		t.Fatalf("Add(\"\") = %v, want ErrPathEmpty", err)
	}
}

// ---- Get ----

func TestGetNotFound(t *testing.T) {
	svc := newTestService(t)
	p, err := svc.Get(context.Background(), "no-such-id")
	if err != nil {
		t.Fatalf("Get(missing) err = %v, want nil", err)
	}
	if p != nil {
		t.Fatalf("Get(missing) = %v, want nil (JSON null)", p)
	}
}

func TestGetFound(t *testing.T) {
	svc := newTestService(t)
	added, err := svc.Add(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := svc.Get(context.Background(), added.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.ID != added.ID || got.Path != added.Path {
		t.Fatalf("Get = %+v, want match of %+v", got, added)
	}
}

// ---- Remove ----

func TestRemoveNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Remove(context.Background(), "no-such-id"); !errors.Is(err, errProjectMissing) {
		t.Fatalf("Remove(missing) = %v, want errProjectMissing", err)
	}
}

func TestRemoveSuccess(t *testing.T) {
	svc := newTestService(t)
	added, err := svc.Add(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := svc.Remove(context.Background(), added.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := svc.Get(context.Background(), added.ID)
	if got != nil {
		t.Fatalf("after Remove, Get = %v, want nil", got)
	}
}

// ---- UpdateName（存未 trim 值；空名拒绝）----

func TestUpdateNameEmptyRejected(t *testing.T) {
	svc := newTestService(t)
	added, _ := svc.Add(context.Background(), t.TempDir())
	if err := svc.UpdateName(context.Background(), added.ID, "   "); !errors.Is(err, errNameEmpty) {
		t.Fatalf("UpdateName(blank) = %v, want errNameEmpty", err)
	}
}

func TestUpdateNameStoresUntrimmed(t *testing.T) {
	svc := newTestService(t)
	added, _ := svc.Add(context.Background(), t.TempDir())
	const raw = "  spaced name  "
	if err := svc.UpdateName(context.Background(), added.ID, raw); err != nil {
		t.Fatalf("UpdateName: %v", err)
	}
	got, _ := svc.Get(context.Background(), added.ID)
	if got == nil || got.Name != raw {
		t.Fatalf("Name = %q, want untrimmed %q", got.Name, raw)
	}
}

func TestUpdateNameNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.UpdateName(context.Background(), "no-id", "x"); !errors.Is(err, errProjectMissing) {
		t.Fatalf("UpdateName(missing) = %v, want errProjectMissing", err)
	}
}

// ---- UpdateAlias（存 trim 值；空/空白/nil 清成 NULL）----

func TestUpdateAliasStoresTrimmed(t *testing.T) {
	svc := newTestService(t)
	added, _ := svc.Add(context.Background(), t.TempDir())
	if err := svc.UpdateAlias(context.Background(), added.ID, ptr("  my alias  ")); err != nil {
		t.Fatalf("UpdateAlias: %v", err)
	}
	got, _ := svc.Get(context.Background(), added.ID)
	if got == nil || got.Alias == nil || *got.Alias != "my alias" {
		t.Fatalf("Alias = %v, want trimmed \"my alias\"", got.Alias)
	}
}

func TestUpdateAliasClears(t *testing.T) {
	svc := newTestService(t)
	added, _ := svc.Add(context.Background(), t.TempDir())
	// 先设一个别名。
	if err := svc.UpdateAlias(context.Background(), added.ID, ptr("temp")); err != nil {
		t.Fatalf("set alias: %v", err)
	}
	// nil / 空串 / 纯空白 都应清成 NULL。
	for _, clear := range []*string{nil, ptr(""), ptr("   ")} {
		if err := svc.UpdateAlias(context.Background(), added.ID, ptr("temp")); err != nil {
			t.Fatalf("reset alias: %v", err)
		}
		if err := svc.UpdateAlias(context.Background(), added.ID, clear); err != nil {
			t.Fatalf("UpdateAlias(clear): %v", err)
		}
		got, _ := svc.Get(context.Background(), added.ID)
		if got == nil || got.Alias != nil {
			t.Fatalf("Alias = %v, want nil (cleared) for %v", got.Alias, clear)
		}
	}
}

func TestUpdateAliasNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.UpdateAlias(context.Background(), "no-id", ptr("x")); !errors.Is(err, errProjectMissing) {
		t.Fatalf("UpdateAlias(missing) = %v, want errProjectMissing", err)
	}
}

// ---- List ordering / empty ----

func TestListEmpty(t *testing.T) {
	svc := newTestService(t)
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Fatal("List should return non-nil empty slice (JSON []), got nil")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

// 直接插两行带受控 CreatedAt 的记录（绕过 Add 的时钟分辨率不确定性），断言
// List 倒序（最新在前）。
func TestListOrderingDESC(t *testing.T) {
	svc := newTestService(t)
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	if err := svc.db().Create(&storage.Project{ID: "old", Name: "old", Path: "p-old", CreatedAt: older}).Error; err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := svc.db().Create(&storage.Project{ID: "new", Name: "new", Path: "p-new", CreatedAt: newer}).Error; err != nil {
		t.Fatalf("seed new: %v", err)
	}
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != "new" || list[1].ID != "old" {
		t.Fatalf("ordering = %v, want [new, old] (created_at DESC)", []string{list[0].ID, list[1].ID})
	}
}
