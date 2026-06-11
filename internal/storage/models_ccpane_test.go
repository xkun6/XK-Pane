package storage

import (
	"path/filepath"
	"testing"
)

// TestAutoMigrateAllModels 守门测试：Open() 应为 AllModels 里每个 ModelDescriptor
// 建出对应表（含 cc-pane 移植的核心表）。任何模型结构体写错（GORM tag 非法、
// 类型不支持等）都会让 Open 的 AutoMigrate 失败，在此被捕获。
func TestAutoMigrateAllModels(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	m := db.GORM.Migrator()
	for _, desc := range AllModels {
		if !m.HasTable(desc.TableName) {
			t.Errorf("table %q not created by AutoMigrate", desc.TableName)
		}
	}
}

// TestCoreCcpaneTablesRegistered 确认 cc-pane 核心表都已登记到 AllModels。
func TestCoreCcpaneTablesRegistered(t *testing.T) {
	want := []string{
		"projects", "launch_history", "task_bindings",
	}
	for _, name := range want {
		if FindDescriptor(name) == nil {
			t.Errorf("expected %q registered in AllModels", name)
		}
	}
}
