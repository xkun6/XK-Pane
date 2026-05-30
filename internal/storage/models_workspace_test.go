package storage

import (
	"path/filepath"
	"testing"
)

// TestWorkspaceTablesRegistered 确认 P3a 两张表已登记到 AllModels
// （TestAutoMigrateAllModels 会顺带验证它们被 AutoMigrate 真正建出）。
func TestWorkspaceTablesRegistered(t *testing.T) {
	for _, name := range []string{"workspaces", "workspace_projects"} {
		if FindDescriptor(name) == nil {
			t.Errorf("expected %q registered in AllModels", name)
		}
	}
}

// TestWorkspaceProjectPathUnique 守门 idx_ws_proj_path 联合唯一索引：
// 同一 workspace 内 path 去重；不同 workspace 的同 path 允许。
func TestWorkspaceProjectPathUnique(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	g := db.GORM
	if err := g.Create(&Workspace{ID: "ws-1", Name: "W1"}).Error; err != nil {
		t.Fatalf("create workspace ws-1: %v", err)
	}
	if err := g.Create(&WorkspaceProject{ID: "p-1", WorkspaceID: "ws-1", Path: "/a/b"}).Error; err != nil {
		t.Fatalf("create project p-1: %v", err)
	}

	// 同 workspace 同 path → 唯一约束应拒绝。
	if err := g.Create(&WorkspaceProject{ID: "p-2", WorkspaceID: "ws-1", Path: "/a/b"}).Error; err == nil {
		t.Error("expected unique-constraint violation for duplicate (workspace_id, path)")
	}

	// 不同 workspace 同 path → 应允许。
	if err := g.Create(&Workspace{ID: "ws-2", Name: "W2"}).Error; err != nil {
		t.Fatalf("create workspace ws-2: %v", err)
	}
	if err := g.Create(&WorkspaceProject{ID: "p-3", WorkspaceID: "ws-2", Path: "/a/b"}).Error; err != nil {
		t.Errorf("different workspace, same path should be allowed: %v", err)
	}
}
