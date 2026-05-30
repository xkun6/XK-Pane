package specsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestTitleToFilename(t *testing.T) {
	cases := map[string]string{
		"Add Dark Mode": "add-dark-mode.spec.md",
		"  Fix  Bug  ":  "fix-bug.spec.md",
		"添加暗黑模式":       "添加暗黑模式.spec.md", // CJK 保留
		"Fix Bug 修复":   "fix-bug-修复.spec.md",
	}
	for in, want := range cases {
		if got := titleToFilename(in); got != want {
			t.Errorf("titleToFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateMakesFileAndRow(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	sp, err := svc.Create(ctx, proj, "Add Dark Mode")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sp.ID == "" || sp.Status != "draft" || sp.FileName != "add-dark-mode.spec.md" {
		t.Fatalf("created spec wrong: %+v", sp)
	}
	// 文件已生成且含模板标题。
	path := filepath.Join(specDir(proj), sp.FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spec file not created: %v", err)
	}
	if !strings.Contains(string(data), "# Spec: Add Dark Mode") {
		t.Errorf("template missing title heading; got: %q", string(data))
	}
	// DB 行可查。
	got, err := svc.Get(ctx, sp.ID)
	if err != nil || got == nil || got.Title != "Add Dark Mode" {
		t.Fatalf("Get(created) = (%+v, %v)", got, err)
	}
	// 空标题报错。
	if _, err := svc.Create(ctx, proj, "   "); err == nil {
		t.Error("Create(empty title) err=nil, want error")
	}
}

func TestListByProjectAndStatus(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	a, _ := svc.Create(ctx, proj, "Spec A")
	_, _ = svc.Create(ctx, proj, "Spec B")
	_, _ = svc.Create(ctx, proj, "Spec C")
	if _, err := svc.Update(ctx, a.ID, nil, ptr("active")); err != nil {
		t.Fatalf("activate A: %v", err)
	}

	all, err := svc.List(ctx, proj, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("List(all) = (%d, %v), want 3", len(all), err)
	}
	if act, _ := svc.List(ctx, proj, "active"); len(act) != 1 || act[0].ID != a.ID {
		t.Errorf("List(active) = %+v, want only A", act)
	}
	if dr, _ := svc.List(ctx, proj, "draft"); len(dr) != 2 {
		t.Errorf("List(draft) = %d, want 2", len(dr))
	}
}

func TestContentRoundtrip(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	sp, _ := svc.Create(ctx, proj, "Doc")
	if err := svc.SaveContent(ctx, proj, sp.ID, "hello\nworld\n"); err != nil {
		t.Fatalf("SaveContent: %v", err)
	}
	got, err := svc.GetContent(ctx, proj, sp.ID)
	if err != nil || got != "hello\nworld\n" {
		t.Fatalf("GetContent = (%q, %v), want hello/world", got, err)
	}
}

func TestActivateDeactivatesOthers(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	a, _ := svc.Create(ctx, proj, "First")
	b, _ := svc.Create(ctx, proj, "Second")
	if _, err := svc.Update(ctx, a.ID, nil, ptr("active")); err != nil {
		t.Fatalf("activate A: %v", err)
	}
	if _, err := svc.Update(ctx, b.ID, nil, ptr("active")); err != nil {
		t.Fatalf("activate B: %v", err)
	}
	ga, _ := svc.Get(ctx, a.ID)
	gb, _ := svc.Get(ctx, b.ID)
	if ga.Status != "draft" {
		t.Errorf("A status = %q, want draft (deactivated by B)", ga.Status)
	}
	if gb.Status != "active" {
		t.Errorf("B status = %q, want active", gb.Status)
	}
	active, _ := svc.GetActive(ctx, proj)
	if active == nil || active.ID != b.ID {
		t.Errorf("GetActive = %+v, want B", active)
	}
}

func TestArchiveMovesFile(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	sp, _ := svc.Create(ctx, proj, "Archive Me")
	oldPath := filepath.Join(specDir(proj), sp.FileName)

	updated, err := svc.Update(ctx, sp.ID, nil, ptr("archived"))
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if updated.Status != "archived" || updated.ArchivedAt == nil {
		t.Fatalf("archived spec wrong: status=%q archivedAt=%v", updated.Status, updated.ArchivedAt)
	}
	// 原文件已移走。
	if _, statErr := os.Stat(oldPath); !os.IsNotExist(statErr) {
		t.Errorf("original file still present at %s", oldPath)
	}
	// archived 子目录恰有一个文件。
	entries, _ := os.ReadDir(filepath.Join(specDir(proj), archivedSub))
	if len(entries) != 1 {
		t.Fatalf("archived dir has %d files, want 1", len(entries))
	}
	// 归档后内容仍可读（resolveSpecPath 走 archived 分支）。
	if _, err := svc.GetContent(ctx, proj, sp.ID); err != nil {
		t.Errorf("GetContent after archive: %v", err)
	}
}

func TestDeleteRemovesFileAndRow(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	proj := t.TempDir()

	sp, _ := svc.Create(ctx, proj, "Doomed")
	path := filepath.Join(specDir(proj), sp.FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist pre-delete: %v", err)
	}
	if err := svc.Delete(ctx, proj, sp.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("spec file still present after delete")
	}
	if got, _ := svc.Get(ctx, sp.ID); got != nil {
		t.Error("spec row still present after delete")
	}
}

func TestGetActiveNone(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if got, err := svc.GetActive(ctx, t.TempDir()); err != nil || got != nil {
		t.Errorf("GetActive(no active) = (%+v, %v), want (nil, nil)", got, err)
	}
}

func ptr(s string) *string { return &s }
