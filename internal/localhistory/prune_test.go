package localhistory

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func blobCount(t *testing.T, s *Store) int {
	t.Helper()
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		t.Fatalf("readdir blobs: %v", err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n
}

func TestConfigRoundtrip(t *testing.T) {
	s := openTemp(t)

	// 无文件 → 默认。
	cfg, err := s.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if !cfg.Enabled || cfg.MaxVersionsPerFile != 50 || cfg.MaxAgeDays != 30 || cfg.MinSaveIntervalSecs != 300 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}

	// 保存自定义 → 读回。
	cfg.MaxAgeDays = 7
	cfg.MaxVersionsPerFile = 10
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, _ := s.ReadConfig()
	if got.MaxAgeDays != 7 || got.MaxVersionsPerFile != 10 {
		t.Errorf("roundtrip wrong: %+v", got)
	}

	// 部分 JSON：缺失字段保留默认。
	if err := os.WriteFile(s.configPath(), []byte(`{"maxAgeDays": 3}`), 0o644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	partial, _ := s.ReadConfig()
	if partial.MaxAgeDays != 3 || partial.MaxVersionsPerFile != 50 || !partial.Enabled {
		t.Errorf("partial merge wrong: %+v", partial)
	}
}

func TestCleanupByCount(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	for i := 0; i < 5; i++ {
		if _, err := s.SaveVersion(ctx, "a.txt", []byte{byte('0' + i)}, false, "main", 0); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	cfg := DefaultHistoryConfig()
	cfg.MaxAgeDays = 99999 // 不让 age 触发
	cfg.MaxVersionsPerFile = 2
	if err := s.CleanupOldVersions(ctx, cfg); err != nil {
		t.Fatalf("CleanupOldVersions: %v", err)
	}
	if vs, _ := s.ListVersions(ctx, "a.txt"); len(vs) != 2 {
		t.Fatalf("versions after count-prune = %d, want 2", len(vs))
	}
	if n := blobCount(t, s); n != 2 {
		t.Fatalf("blobs after prune = %d, want 2 (orphans removed)", n)
	}
}

func TestCleanupByAge(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	recent, _ := s.SaveVersion(ctx, "a.txt", []byte("recent"), false, "main", 0)
	_ = recent

	// 直接插一条 100 天前的旧版本 + 其 blob。
	oldHash := "0000000000000000000000000000000000000000000000000000000000000001"
	if err := os.WriteFile(s.blobPath(oldHash), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old blob: %v", err)
	}
	old := FileVersion{
		ID: "100", FilePath: "a.txt", Hash: oldHash, Size: 3,
		CreatedAt: time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339), Branch: "main",
	}
	if err := s.gdb.Create(&old).Error; err != nil {
		t.Fatalf("insert old: %v", err)
	}

	cfg := DefaultHistoryConfig()
	cfg.MaxAgeDays = 30
	cfg.MaxVersionsPerFile = 100
	if err := s.CleanupOldVersions(ctx, cfg); err != nil {
		t.Fatalf("CleanupOldVersions: %v", err)
	}
	vs, _ := s.ListVersions(ctx, "a.txt")
	if len(vs) != 1 {
		t.Fatalf("versions after age-prune = %d, want 1 (only recent)", len(vs))
	}
	// 旧 blob 已被孤儿回收。
	if _, err := os.Stat(s.blobPath(oldHash)); !os.IsNotExist(err) {
		t.Errorf("old blob still exists, want removed (err=%v)", err)
	}
}

func TestCleanupByTotalSize(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	// 三个各 1000 字节的不同版本 → 3 blob ~3000。
	for _, c := range []byte{'a', 'b', 'c'} {
		if _, err := s.SaveVersion(ctx, "a.txt", bytes.Repeat([]byte{c}, 1000), false, "main", 0); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	if got, _ := s.blobsDirSize(); got != 3000 {
		t.Fatalf("pre-clean blob size = %d, want 3000", got)
	}
	if err := s.CleanupByTotalSize(ctx, 1500); err != nil {
		t.Fatalf("CleanupByTotalSize: %v", err)
	}
	after, _ := s.blobsDirSize()
	if after > 1500 {
		t.Fatalf("blob size after LRU = %d, want <= 1500", after)
	}
	// 至少保留最新版本（删最旧两个，保留最新一个 = 1000 字节）。
	if vs, _ := s.ListVersions(ctx, "a.txt"); len(vs) != 1 {
		t.Fatalf("versions after size-prune = %d, want 1", len(vs))
	}
}
