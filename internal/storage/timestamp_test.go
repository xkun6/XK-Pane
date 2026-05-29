package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAutoTimestampsStoredUTC 回归守门：GORM 自动填充的时间戳必须以 UTC 写入。
//
// glebarez 把 time.Time 按其自身时区序列化为 TEXT；GORM 默认 NowFunc 用本地
// 时区，跨 DST/换时区的两行会带不同偏移，而 SQLite 对 created_at 做字典序 TEXT
// 排序 → 绝对更新的行可能排到更旧的行之后（破坏「最新在前」）。Open 里把
// NowFunc 固定 UTC 后，自动时间戳一律带 "+00:00" 偏移，字典序==绝对时序，且与
// cc-pane（chrono::Utc::now）一致。详见 project service 对抗审查 wf_7bc251b0-c1f。
func TestAutoTimestampsStoredUTC(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ts.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// 不显式给 CreatedAt → 由 GORM 经 NowFunc 自动填充。
	p := Project{ID: "ts-1", Name: "ts", Path: "/ts"}
	if err := db.GORM.Create(&p).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var raw string
	if err := db.GORM.Raw("SELECT created_at FROM projects WHERE id = ?", "ts-1").Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan created_at: %v", err)
	}
	// UTC 在 RFC3339 下记为 "Z"，部分驱动版本记为 "+00:00"；二者皆可。本地时区
	// 会记成 "+08:00" 之类非零偏移——那正是要防的回归。
	if !strings.HasSuffix(raw, "Z") && !strings.HasSuffix(raw, "+00:00") {
		t.Fatalf("created_at = %q, want UTC marker (Z / +00:00); a non-zero offset means NowFunc regressed to local time", raw)
	}
}
