package launchhistory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xkpane/internal/storage"
)

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

func TestRecordCreatesRow(t *testing.T) {
	svc := newTestService(t)
	rec, err := svc.Record(context.Background(), RecordSpec{
		ProjectID:   "p1",
		ProjectName: "proj",
		ProjectPath: "C:\\work\\proj",
		CliTool:     "claude",
		RuntimeKind: "local",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.ID == 0 {
		t.Error("expected autoincrement ID > 0")
	}
	if rec.ProjectPath != "C:\\work\\proj" || rec.CliTool != "claude" {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.LaunchedAt.IsZero() {
		t.Error("LaunchedAt should be set")
	}
}

func TestRecordDefaults(t *testing.T) {
	svc := newTestService(t)
	rec, err := svc.Record(context.Background(), RecordSpec{ProjectPath: "/p"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.CliTool != "none" {
		t.Errorf("CliTool default = %q, want none", rec.CliTool)
	}
	if rec.RuntimeKind != "local" {
		t.Errorf("RuntimeKind default = %q, want local", rec.RuntimeKind)
	}
}

// LaunchedAt 是手填字段且用于倒序排序——必须以 UTC 存储（否则跨时区字典序倒挂）。
func TestRecordLaunchedAtStoredUTC(t *testing.T) {
	svc := newTestService(t)
	rec, err := svc.Record(context.Background(), RecordSpec{ProjectPath: "/p"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	var raw string
	if err := svc.db().Raw("SELECT launched_at FROM launch_history WHERE id = ?", rec.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasSuffix(raw, "Z") && !strings.HasSuffix(raw, "+00:00") {
		t.Fatalf("launched_at = %q, want UTC marker (Z / +00:00)", raw)
	}
}

func TestWorkspaceFieldsOptional(t *testing.T) {
	svc := newTestService(t)
	// 不传 workspace → NULL
	a, _ := svc.Record(context.Background(), RecordSpec{ProjectPath: "/a"})
	if a.WorkspaceName != nil || a.WorkspacePath != nil {
		t.Errorf("expected nil workspace fields, got %v / %v", a.WorkspaceName, a.WorkspacePath)
	}
	// 传 workspace → 存值
	b, _ := svc.Record(context.Background(), RecordSpec{
		ProjectPath:   "/b",
		WorkspaceName: ptr("ws"),
		WorkspacePath: ptr("/ws"),
	})
	if b.WorkspaceName == nil || *b.WorkspaceName != "ws" {
		t.Errorf("WorkspaceName = %v, want ws", b.WorkspaceName)
	}
}

func TestListOrdersByLaunchedAtDESC(t *testing.T) {
	svc := newTestService(t)
	older := time.Now().Add(-time.Hour).UTC()
	newer := time.Now().UTC()
	if err := svc.db().Create(&storage.LaunchHistory{ProjectPath: "/old", CliTool: "none", RuntimeKind: "local", LaunchedAt: older}).Error; err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := svc.db().Create(&storage.LaunchHistory{ProjectPath: "/new", CliTool: "none", RuntimeKind: "local", LaunchedAt: newer}).Error; err != nil {
		t.Fatalf("seed new: %v", err)
	}
	list, err := svc.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ProjectPath != "/new" || list[1].ProjectPath != "/old" {
		t.Fatalf("ordering wrong: %+v", list)
	}
}

func TestListLimit(t *testing.T) {
	svc := newTestService(t)
	for i := 0; i < 3; i++ {
		if _, err := svc.Record(context.Background(), RecordSpec{ProjectPath: "/p"}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	list, err := svc.List(context.Background(), 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2 (limit)", len(list))
	}
}

func TestListEmpty(t *testing.T) {
	svc := newTestService(t)
	list, err := svc.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Fatal("List should return non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}
