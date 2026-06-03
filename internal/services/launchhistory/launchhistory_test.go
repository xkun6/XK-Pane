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

// ── 回填（会话恢复）─────────────────────────────────────────────────────────

type fakeDetector struct{ id string }

func (f fakeDetector) DetectResumeSession(_, _, _, _ string, _ int64) string { return f.id }

// updateSessionStarted 只回填「最近一条窗口内、resume 仍空、project+cli 匹配」的记录，三列写入；
// 更旧的匹配行 / 已回填行 / 异 cli 行均不动。
func TestUpdateSessionStartedBackfillsMostRecentMatch(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()
	older := storage.LaunchHistory{ProjectPath: "/proj", CliTool: "claude", RuntimeKind: "local", LaunchedAt: now.Add(-30 * time.Second)}
	newer := storage.LaunchHistory{ProjectPath: "/proj", CliTool: "claude", RuntimeKind: "local", LaunchedAt: now.Add(-5 * time.Second)}
	resumed := storage.LaunchHistory{ProjectPath: "/proj", CliTool: "claude", RuntimeKind: "local", LaunchedAt: now, ResumeSessionID: ptr("already")}
	otherCli := storage.LaunchHistory{ProjectPath: "/proj", CliTool: "codex", RuntimeKind: "local", LaunchedAt: now}
	for _, r := range []*storage.LaunchHistory{&older, &newer, &resumed, &otherCli} {
		if err := svc.db().Create(r).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	id, err := svc.updateSessionStarted("/proj", "claude", "pty-1", "resume-xyz", time.Now().Unix())
	if err != nil {
		t.Fatalf("updateSessionStarted: %v", err)
	}
	if id != newer.ID {
		t.Fatalf("backfilled id = %d, want newer %d", id, newer.ID)
	}

	var got storage.LaunchHistory
	svc.db().First(&got, newer.ID)
	if got.ResumeSessionID == nil || *got.ResumeSessionID != "resume-xyz" {
		t.Errorf("ResumeSessionID = %v, want resume-xyz", got.ResumeSessionID)
	}
	if got.PtySessionID == nil || *got.PtySessionID != "pty-1" {
		t.Errorf("PtySessionID = %v, want pty-1", got.PtySessionID)
	}
	if got.ClaudeSessionID == nil || *got.ClaudeSessionID != "resume-xyz" {
		t.Errorf("ClaudeSessionID = %v, want resume-xyz", got.ClaudeSessionID)
	}

	var oldRow storage.LaunchHistory
	svc.db().First(&oldRow, older.ID)
	if oldRow.ResumeSessionID != nil {
		t.Errorf("older row should stay NULL, got %v", *oldRow.ResumeSessionID)
	}
	var resumedRow storage.LaunchHistory
	svc.db().First(&resumedRow, resumed.ID)
	if resumedRow.ResumeSessionID == nil || *resumedRow.ResumeSessionID != "already" {
		t.Errorf("resumed row changed: %v", resumedRow.ResumeSessionID)
	}
}

// 窗口外（launched_at 早于 afterUnix-slack）的记录不回填，返回 0。
func TestUpdateSessionStartedRespectsWindow(t *testing.T) {
	svc := newTestService(t)
	stale := storage.LaunchHistory{ProjectPath: "/proj", CliTool: "claude", RuntimeKind: "local", LaunchedAt: time.Now().UTC().Add(-2 * time.Hour)}
	if err := svc.db().Create(&stale).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, err := svc.updateSessionStarted("/proj", "claude", "pty", "rid", time.Now().Unix())
	if err != nil {
		t.Fatalf("updateSessionStarted: %v", err)
	}
	if id != 0 {
		t.Fatalf("id = %d, want 0 (stale row out of window)", id)
	}
}

// StartBackfill 守门：无 detector / none / 空 cli / 空路径 / 空 pty 均同步 no-op 返回 nil。
func TestStartBackfillNoopGuards(t *testing.T) {
	svc := newTestService(t)
	if err := svc.StartBackfill(context.Background(), "/p", "", "claude", "pty"); err != nil {
		t.Fatalf("nil detector should no-op: %v", err)
	}
	svc.SetResumeDetector(fakeDetector{id: "x"})
	cases := []struct{ proj, cli, pty string }{
		{"/p", "none", "pty"},
		{"/p", "", "pty"},
		{"", "claude", "pty"},
		{"/p", "claude", ""},
	}
	for _, c := range cases {
		if err := svc.StartBackfill(context.Background(), c.proj, "", c.cli, c.pty); err != nil {
			t.Errorf("StartBackfill(%+v) = %v, want nil", c, err)
		}
	}
}

// 端到端：Record 一行 → StartBackfill（假 detector 立即返回 id）→ 后台 goroutine 回填该行。
func TestStartBackfillEndToEnd(t *testing.T) {
	svc := newTestService(t)
	svc.SetResumeDetector(fakeDetector{id: "rid-1"})
	rec, err := svc.Record(context.Background(), RecordSpec{ProjectPath: "/proj", CliTool: "claude", RuntimeKind: "local"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := svc.StartBackfill(context.Background(), "/proj", "", "claude", "pty-9"); err != nil {
		t.Fatalf("StartBackfill: %v", err)
	}
	// 首轮无前置 sleep，回填应很快落地；轮询等待避免硬编码时延。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var got storage.LaunchHistory
		if err := svc.db().First(&got, rec.ID).Error; err == nil && got.ResumeSessionID != nil {
			if *got.ResumeSessionID != "rid-1" {
				t.Fatalf("ResumeSessionID = %q, want rid-1", *got.ResumeSessionID)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("backfill did not complete within deadline")
}
