package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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

func ptr(s string) *string { return &s }

func sess(id, tab, pane, proj string) storage.TerminalSession {
	now := time.Now().UTC()
	return storage.TerminalSession{
		SessionID: id, TabID: tab, PaneID: pane, ProjectPath: proj,
		CliTool: "none", RuntimeKind: "local",
		CreatedAt: now, SavedAt: now,
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	s1 := sess("s1", "t1", "p1", "C:\\work\\a")
	s2 := sess("s2", "t2", "p1", "C:\\work\\b")
	s2.CliTool = "claude"
	s2.SshConfig = ptr("host=example.com user=bob") // 敏感

	if err := svc.SaveSessions(ctx, []storage.TerminalSession{s1, s2}); err != nil {
		t.Fatalf("SaveSessions: %v", err)
	}
	loaded, err := svc.LoadSessions(ctx)
	if err != nil {
		t.Fatalf("LoadSessions: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded = %d, want 2", len(loaded))
	}
	byID := map[string]storage.TerminalSession{}
	for _, s := range loaded {
		byID[s.SessionID] = s
	}
	if byID["s2"].CliTool != "claude" {
		t.Errorf("s2 cliTool = %q, want claude", byID["s2"].CliTool)
	}
	// sshConfig 解密回明文。
	if byID["s2"].SshConfig == nil || *byID["s2"].SshConfig != "host=example.com user=bob" {
		t.Errorf("s2 sshConfig roundtrip wrong: %v", byID["s2"].SshConfig)
	}
	if byID["s1"].SshConfig != nil {
		t.Errorf("s1 sshConfig should be nil, got %v", byID["s1"].SshConfig)
	}
}

func TestSshConfigEncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	plain := "host=secret.internal key=/home/bob/.ssh/id_ed25519"
	s := sess("s1", "t1", "p1", "/p")
	s.SshConfig = ptr(plain)
	if err := svc.SaveSessions(ctx, []storage.TerminalSession{s}); err != nil {
		t.Fatalf("SaveSessions: %v", err)
	}
	// 直接读裸行（绕过 LoadSessions 的解密），确认落库的是密文。
	var raw storage.TerminalSession
	if err := svc.db().First(&raw, "session_id = ?", "s1").Error; err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw.SshConfig == nil || *raw.SshConfig == plain {
		t.Fatalf("sshConfig stored in plaintext! got %v", raw.SshConfig)
	}
	if *raw.SshConfig == "" {
		t.Fatal("sshConfig stored empty")
	}
}

func TestSaveReplacesAll(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if err := svc.SaveSessions(ctx, []storage.TerminalSession{sess("s1", "t1", "p1", "/a")}); err != nil {
		t.Fatalf("save 1: %v", err)
	}
	if err := svc.SaveSessions(ctx, []storage.TerminalSession{sess("s2", "t2", "p1", "/b")}); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	loaded, _ := svc.LoadSessions(ctx)
	if len(loaded) != 1 || loaded[0].SessionID != "s2" {
		t.Fatalf("replace-all failed: %+v", loaded)
	}
}

func TestSaveEmptyAndClear(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	if err := svc.SaveSessions(ctx, []storage.TerminalSession{sess("s1", "t1", "p1", "/a"), sess("s2", "t2", "p1", "/b")}); err != nil {
		t.Fatalf("save: %v", err)
	}
	// 空保存 = 清空。
	if err := svc.SaveSessions(ctx, nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if loaded, _ := svc.LoadSessions(ctx); len(loaded) != 0 {
		t.Fatalf("after empty save = %d, want 0", len(loaded))
	}
	// Clear 幂等。
	if err := svc.ClearSessions(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
}
