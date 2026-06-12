package screenshotsvc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touchWithMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupOldScreenshots(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	oldPNG := filepath.Join(dir, "screenshot_old.png")
	newPNG := filepath.Join(dir, "screenshot_new.png")
	oldTxt := filepath.Join(dir, "note.txt")
	touchWithMtime(t, oldPNG, now.Add(-10*24*time.Hour))
	touchWithMtime(t, newPNG, now.Add(-1*24*time.Hour))
	touchWithMtime(t, oldTxt, now.Add(-10*24*time.Hour))

	if err := cleanupOldScreenshots(dir, 7, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPNG); !os.IsNotExist(err) {
		t.Error("过期 png 应被删除")
	}
	if _, err := os.Stat(newPNG); err != nil {
		t.Error("未过期 png 不应被删除")
	}
	if _, err := os.Stat(oldTxt); err != nil {
		t.Error("非 png 文件不应被删除")
	}
}

func TestCleanupRetentionZeroKeepsAll(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	p := filepath.Join(dir, "screenshot_ancient.png")
	touchWithMtime(t, p, now.Add(-365*24*time.Hour))

	if err := cleanupOldScreenshots(dir, 0, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Error("retention=0 应永久保留")
	}
}
