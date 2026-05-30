package localhistorysvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// 端到端走 service 层：快照 → 最近变更 → 列版本 → 取内容 → 版本间 diff → 回滚。
// 这覆盖 service 在 localhistory 之上新增的部分（按项目缓存 Store + SnapshotFile 读盘包装 + 委托）。
func TestServiceEndToEnd(t *testing.T) {
	ctx := context.Background()
	svc := New()
	proj := t.TempDir()
	// closeAll 必须在 t.TempDir() 之后注册：cleanup 是 LIFO，后注册者先跑 —— 确保关库
	// 句柄先于 TempDir 删除执行（否则 Windows 下 WAL 句柄占用导致删除失败）。
	t.Cleanup(svc.closeAll)
	writeFile(t, proj, "a.txt", "hello\n")

	v1, err := svc.SnapshotFile(ctx, proj, "a.txt", "main")
	if err != nil {
		t.Fatalf("SnapshotFile v1: %v", err)
	}
	if v1 == nil {
		t.Fatal("first snapshot returned nil, want a version")
	}

	// 内容未变 → 去重跳过（返回 nil）。
	if dup, err := svc.SnapshotFile(ctx, proj, "a.txt", "main"); err != nil || dup != nil {
		t.Fatalf("unchanged snapshot = (%v, %v), want (nil, nil)", dup, err)
	}

	writeFile(t, proj, "a.txt", "hello world\n")
	v2, err := svc.SnapshotFile(ctx, proj, "a.txt", "main")
	if err != nil || v2 == nil {
		t.Fatalf("SnapshotFile v2 = (%v, %v), want a version", v2, err)
	}

	// 最近变更：2 条，最新（v2）在前。
	rc, err := svc.GetRecentChanges(ctx, proj, 50)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(rc) != 2 {
		t.Fatalf("recent changes = %d, want 2", len(rc))
	}
	if rc[0].VersionID != v2.ID {
		t.Errorf("newest recent change = %q, want v2 %q", rc[0].VersionID, v2.ID)
	}

	// 列版本：2 条。
	if vs, err := svc.ListVersions(ctx, proj, "a.txt"); err != nil || len(vs) != 2 {
		t.Fatalf("ListVersions = (%d, %v), want 2", len(vs), err)
	}

	// 取 v1 内容。
	if c, err := svc.GetVersionContent(ctx, proj, "a.txt", v1.ID); err != nil || c != "hello\n" {
		t.Fatalf("GetVersionContent(v1) = (%q, %v), want hello", c, err)
	}

	// 版本间 diff：一行被替换（1 删 + 1 增）。
	diff, err := svc.GetVersionsDiff(ctx, proj, "a.txt", v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("GetVersionsDiff: %v", err)
	}
	if diff.IsBinary || len(diff.Hunks) == 0 {
		t.Fatalf("diff = %+v, want non-binary with hunks", diff)
	}
	if diff.Stats.Additions == 0 || diff.Stats.Deletions == 0 {
		t.Errorf("diff stats = %+v, want both additions and deletions", diff.Stats)
	}

	// 回滚到 v1：工作树文件内容回到 hello，并自动打了 Before-Restore 标签。
	if err := svc.RestoreVersion(ctx, proj, "a.txt", v1.ID, "main"); err != nil {
		t.Fatalf("RestoreVersion: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(proj, "a.txt")); string(b) != "hello\n" {
		t.Fatalf("after restore a.txt = %q, want hello", b)
	}
	labels, err := svc.ListLabels(ctx, proj)
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) == 0 {
		t.Error("ListLabels empty after restore, want a Before-Restore auto label")
	}

	// Store 缓存：多次操作同一项目只 Open 一次。
	if n := len(svc.stores); n != 1 {
		t.Errorf("cached stores = %d, want 1", n)
	}
}

// 第二个项目独立缓存（多 db 模式）。
func TestSeparateProjectsCached(t *testing.T) {
	ctx := context.Background()
	svc := New()
	p1, p2 := t.TempDir(), t.TempDir()
	t.Cleanup(svc.closeAll) // 在 TempDir 之后注册（LIFO：先关库再删目录）
	writeFile(t, p1, "x.txt", "1")
	writeFile(t, p2, "y.txt", "2")
	if _, err := svc.SnapshotFile(ctx, p1, "x.txt", ""); err != nil {
		t.Fatalf("snapshot p1: %v", err)
	}
	if _, err := svc.SnapshotFile(ctx, p2, "y.txt", ""); err != nil {
		t.Fatalf("snapshot p2: %v", err)
	}
	if n := len(svc.stores); n != 2 {
		t.Errorf("cached stores = %d, want 2 (per-project db)", n)
	}
	// p1 看不到 p2 的文件（独立 db）。
	if rc, _ := svc.GetRecentChanges(ctx, p1, 50); len(rc) != 1 || rc[0].FilePath != "x.txt" {
		t.Errorf("p1 recent = %+v, want only x.txt", rc)
	}
}
