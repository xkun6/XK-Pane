package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v1, _ := s.SaveVersion(ctx, "a.txt", []byte("v1"), false, "main", 0)
	if _, err := s.SaveVersion(ctx, "a.txt", []byte("v2"), false, "main", 0); err != nil {
		t.Fatalf("save v2: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("current"), 0o644)

	if err := s.RestoreVersion(ctx, "a.txt", v1.ID, "main"); err != nil {
		t.Fatalf("RestoreVersion: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != "v1" {
		t.Errorf("restored content = %q, want v1", got)
	}
	// 恢复前自动打 "Before Restore: a.txt" 标签，快照含恢复前最新版。
	labels, _ := s.ListLabels(ctx)
	found := false
	for _, l := range labels {
		if l.Source == "restore" && l.Name == "Before Restore: a.txt" {
			found = true
			if len(l.FileSnapshots) != 1 || l.FileSnapshots[0].FilePath != "a.txt" {
				t.Errorf("before-restore snapshots wrong: %+v", l.FileSnapshots)
			}
		}
	}
	if !found {
		t.Error("no 'Before Restore' auto-label created")
	}
}

func TestRestoreCreatesParentDirs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v, _ := s.SaveVersion(ctx, "sub/deep/x.txt", []byte("hi"), false, "main", 0)
	if err := s.RestoreVersion(ctx, "sub/deep/x.txt", v.ID, "main"); err != nil {
		t.Fatalf("RestoreVersion: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sub", "deep", "x.txt"))
	if err != nil || string(got) != "hi" {
		t.Fatalf("restored nested file = %q err=%v, want hi", got, err)
	}
}

func TestRestoreToLabel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.SaveVersion(ctx, "a.txt", []byte("a-v1"), false, "main", 0); err != nil {
		t.Fatalf("save a-v1: %v", err)
	}
	if _, err := s.SaveVersion(ctx, "b.txt", []byte("b-v1"), false, "main", 0); err != nil {
		t.Fatalf("save b-v1: %v", err)
	}
	labelID, err := s.CreateAutoLabel(ctx, "snap1", "user", "main")
	if err != nil || labelID == "" {
		t.Fatalf("CreateAutoLabel: id=%q err=%v", labelID, err)
	}

	// 改两文件 + 工作树写当前内容。
	_, _ = s.SaveVersion(ctx, "a.txt", []byte("a-v2"), false, "main", 0)
	_, _ = s.SaveVersion(ctx, "b.txt", []byte("b-v2"), false, "main", 0)
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a-now"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b-now"), 0o644)

	restored, err := s.RestoreToLabel(ctx, labelID, "main")
	if err != nil {
		t.Fatalf("RestoreToLabel: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("restored %d files, want 2", len(restored))
	}
	if c, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(c) != "a-v1" {
		t.Errorf("a.txt = %q, want a-v1", c)
	}
	if c, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(c) != "b-v1" {
		t.Errorf("b.txt = %q, want b-v1", c)
	}
	// "Before Rollback" 标签已建。
	labels, _ := s.ListLabels(ctx)
	hasRollback := false
	for _, l := range labels {
		if l.Name == "Before Rollback" {
			hasRollback = true
		}
	}
	if !hasRollback {
		t.Error("no 'Before Rollback' label created")
	}

	// 未知标签 → 错误。
	if _, err := s.RestoreToLabel(ctx, "does-not-exist", "main"); err == nil {
		t.Error("RestoreToLabel(unknown) err=nil, want error")
	}
}
