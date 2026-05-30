package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreLayout(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	for _, p := range []string{
		filepath.Join(dir, ".xkpanes", "history", "history.db"),
		filepath.Join(dir, ".xkpanes", "history", "blobs"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestSaveDedupListContent(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	v1, err := s.SaveVersion(ctx, "a.txt", []byte("v1"), false, "main", 0)
	if err != nil || v1 == nil {
		t.Fatalf("save v1: v=%v err=%v", v1, err)
	}
	// 内容未变 → 去重跳过（返回 nil,nil）。
	if skip, err := s.SaveVersion(ctx, "a.txt", []byte("v1"), false, "main", 0); err != nil || skip != nil {
		t.Fatalf("dedup save: got %v err=%v, want nil,nil", skip, err)
	}
	v2, err := s.SaveVersion(ctx, "a.txt", []byte("v2-changed"), false, "main", 0)
	if err != nil || v2 == nil {
		t.Fatalf("save v2: v=%v err=%v", v2, err)
	}

	vs, err := s.ListVersions(ctx, "a.txt")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(vs) != 2 {
		t.Fatalf("versions = %d, want 2", len(vs))
	}
	if vs[0].ID != v1.ID || vs[1].ID != v2.ID {
		t.Errorf("order wrong: %s,%s want %s,%s", vs[0].ID, vs[1].ID, v1.ID, v2.ID)
	}
	if vs[1].Size != int64(len("v2-changed")) {
		t.Errorf("size = %d, want %d", vs[1].Size, len("v2-changed"))
	}

	// 内容回放。
	if c, err := s.GetVersionContent(ctx, "a.txt", v1.ID); err != nil || string(c) != "v1" {
		t.Errorf("content v1 = %q err=%v, want v1", c, err)
	}
	if c, err := s.GetVersionContent(ctx, "a.txt", v2.ID); err != nil || string(c) != "v2-changed" {
		t.Errorf("content v2 = %q err=%v, want v2-changed", c, err)
	}
	// 不存在的版本。
	if _, err := s.GetVersionContent(ctx, "a.txt", "nope"); err == nil {
		t.Error("GetVersionContent(missing) err=nil, want error")
	}
}

func TestThrottle(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("v1"), false, "main", 3600); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	// 不同内容但在节流间隔内 → 跳过。
	skip, err := s.SaveVersion(ctx, "a.txt", []byte("v2"), false, "main", 3600)
	if err != nil || skip != nil {
		t.Fatalf("throttled save = %v err=%v, want nil,nil", skip, err)
	}
	if vs, _ := s.ListVersions(ctx, "a.txt"); len(vs) != 1 {
		t.Fatalf("versions = %d, want 1 (second throttled)", len(vs))
	}
}

func TestIsDeletedBypassesDedup(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("same"), false, "main", 0); err != nil {
		t.Fatalf("save: %v", err)
	}
	// 相同内容但 is_deleted=true → 不去重，应记录。
	del, err := s.SaveVersion(ctx, "a.txt", []byte("same"), true, "main", 3600)
	if err != nil || del == nil {
		t.Fatalf("deleted save = %v err=%v, want a version", del, err)
	}
	if !del.IsDeleted {
		t.Error("saved version IsDeleted=false, want true")
	}
	if vs, _ := s.ListVersions(ctx, "a.txt"); len(vs) != 2 {
		t.Fatalf("versions = %d, want 2", len(vs))
	}
}

func TestBranchIsolation(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("main-1"), false, "main", 0); err != nil {
		t.Fatalf("save main: %v", err)
	}
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("feat-1"), false, "feature", 0); err != nil {
		t.Fatalf("save feature: %v", err)
	}
	if vs, _ := s.ListVersionsByBranch(ctx, "a.txt", "main"); len(vs) != 1 || vs[0].Branch != "main" {
		t.Fatalf("main branch versions wrong: %v", vs)
	}
	if vs, _ := s.ListVersionsByBranch(ctx, "a.txt", "feature"); len(vs) != 1 || vs[0].Branch != "feature" {
		t.Fatalf("feature branch versions wrong: %v", vs)
	}
	brs, err := s.GetFileBranches(ctx, "a.txt")
	if err != nil {
		t.Fatalf("GetFileBranches: %v", err)
	}
	if len(brs) != 2 || brs[0] != "feature" || brs[1] != "main" {
		t.Errorf("branches = %v, want [feature main]", brs)
	}
}

func TestBlobDedupAcrossFiles(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	// 两个不同文件、相同内容 → 共享同一个 blob 文件。
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("shared"), false, "main", 0); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if _, err := s.SaveVersion(ctx, "b.txt", []byte("shared"), false, "main", 0); err != nil {
		t.Fatalf("save b: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(s.projectPath, ".xkpanes", "history", "blobs"))
	if err != nil {
		t.Fatalf("readdir blobs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("blob count = %d, want 1 (content-addressed dedup)", len(entries))
	}
}

func TestLabels(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	// 两文件；a.txt 存两版（GetAllLatestSnapshots 应取最新 a2）。
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("a1"), false, "main", 0); err != nil {
		t.Fatalf("save a1: %v", err)
	}
	a2, _ := s.SaveVersion(ctx, "a.txt", []byte("a2"), false, "main", 0)
	b1, _ := s.SaveVersion(ctx, "b.txt", []byte("b1"), false, "main", 0)

	snaps, err := s.GetAllLatestSnapshots(ctx)
	if err != nil {
		t.Fatalf("GetAllLatestSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("latest snapshots = %d, want 2", len(snaps))
	}
	got := map[string]string{}
	for _, sn := range snaps {
		got[sn.FilePath] = sn.VersionID
	}
	if got["a.txt"] != a2.ID {
		t.Errorf("a.txt latest = %s, want %s (newest)", got["a.txt"], a2.ID)
	}
	if got["b.txt"] != b1.ID {
		t.Errorf("b.txt latest = %s, want %s", got["b.txt"], b1.ID)
	}

	// 建标签 L1（含两快照）。
	l1 := HistoryLabel{ID: "L1", Name: "first", LabelType: "manual", Source: "user", Timestamp: "2026-05-30T10:00:00Z", Branch: "main", FileSnapshots: snaps}
	if err := s.PutLabel(ctx, l1); err != nil {
		t.Fatalf("put L1: %v", err)
	}
	labels, _ := s.ListLabels(ctx)
	if len(labels) != 1 || labels[0].ID != "L1" || len(labels[0].FileSnapshots) != 2 {
		t.Fatalf("after put L1: %+v", labels)
	}

	// upsert L1：改名 + 快照只留 a.txt（验证整体替换 + ON CONFLICT 更新）。
	l1b := l1
	l1b.Name = "first-edited"
	l1b.FileSnapshots = []LabelFileSnapshot{{FilePath: "a.txt", VersionID: a2.ID}}
	if err := s.PutLabel(ctx, l1b); err != nil {
		t.Fatalf("upsert L1: %v", err)
	}
	labels, _ = s.ListLabels(ctx)
	if len(labels) != 1 || labels[0].Name != "first-edited" || len(labels[0].FileSnapshots) != 1 {
		t.Fatalf("after upsert L1: %+v", labels)
	}

	// L2 时间更晚 → ListLabels 按 timestamp DESC 排在前。
	l2 := HistoryLabel{ID: "L2", Name: "second", LabelType: "auto", Source: "git_commit", Timestamp: "2026-05-30T11:00:00Z", Branch: "main"}
	if err := s.PutLabel(ctx, l2); err != nil {
		t.Fatalf("put L2: %v", err)
	}
	labels, _ = s.ListLabels(ctx)
	if len(labels) != 2 || labels[0].ID != "L2" || labels[1].ID != "L1" {
		t.Fatalf("order (want L2,L1): %+v", labels)
	}
	if labels[0].FileSnapshots == nil || len(labels[0].FileSnapshots) != 0 {
		t.Errorf("L2 snapshots = %v, want empty non-nil slice", labels[0].FileSnapshots)
	}

	// 删 L1 → 仅剩 L2，且 L1 快照一并删除。
	if err := s.DeleteLabel(ctx, "L1"); err != nil {
		t.Fatalf("delete L1: %v", err)
	}
	labels, _ = s.ListLabels(ctx)
	if len(labels) != 1 || labels[0].ID != "L2" {
		t.Fatalf("after delete L1: %+v", labels)
	}
}
