package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionPrefix(t *testing.T) {
	cases := map[string]string{
		"a1b2c3d4e5": "a1b2c3d4",
		"short":      "short",
		"":           "",
		"12345678":   "12345678",
	}
	for in, want := range cases {
		if got := sessionPrefix(in); got != want {
			t.Errorf("sessionPrefix(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFindExistingArchiveDedup(t *testing.T) {
	dir := t.TempDir()
	// 既有归档：同前缀 + 同原名 → 应命中（去重覆盖）。
	existing := filepath.Join(dir, "a1b2c3d4_20260101_120000_plan.md")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := findExistingArchive(dir, "a1b2c3d4", "plan.md")
	if got != existing {
		t.Errorf("findExistingArchive 命中=%q want %q", got, existing)
	}

	// 不同原名 → 不命中。
	if got := findExistingArchive(dir, "a1b2c3d4", "other.md"); got != "" {
		t.Errorf("findExistingArchive 不该命中 other.md, got %q", got)
	}
	// 空前缀 → 不命中（避免误覆盖）。
	if got := findExistingArchive(dir, "", "plan.md"); got != "" {
		t.Errorf("findExistingArchive 空前缀不该命中, got %q", got)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.md")
	dst := filepath.Join(dir, "dst.md")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("copyFile 内容=%q want hello", got)
	}
}
