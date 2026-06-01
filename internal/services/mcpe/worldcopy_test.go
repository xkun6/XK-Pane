package mcpe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyWorldEssentialsCopiesOnlyDBAndLevelDat(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	// world 必备
	mustWrite(t, filepath.Join(src, "db", "000003.log"), []byte("chunk-data"))
	mustWrite(t, filepath.Join(src, "db", "MANIFEST-000001"), []byte("manifest"))
	mustWrite(t, filepath.Join(src, "level.dat"), []byte("level"))
	// 同级噪音（作品包规范布局：BP/RP/文档/源码与 world 数据同级）
	mustWrite(t, filepath.Join(src, "behavior_packs", "bp1", "manifest.json"), []byte("{}"))
	mustWrite(t, filepath.Join(src, "resource_packs", "rp1", "keep"), []byte("x"))
	mustWrite(t, filepath.Join(src, "docs", "README.md"), []byte("doc"))
	mustWrite(t, filepath.Join(src, "AGENTS.md"), []byte("agents"))
	mustWrite(t, filepath.Join(src, "world_behavior_packs.json"), []byte("[]"))

	if err := copyWorldEssentials(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}

	// 必备保留
	for _, rel := range []string{"level.dat", filepath.Join("db", "000003.log"), filepath.Join("db", "MANIFEST-000001")} {
		if !isFile(filepath.Join(dst, rel)) {
			t.Errorf("expected kept: %s", rel)
		}
	}
	// 其余一概不复制
	for _, rel := range []string{"behavior_packs", "resource_packs", "docs", "AGENTS.md", "world_behavior_packs.json"} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err == nil {
			t.Errorf("should NOT copy: %s", rel)
		}
	}
}

func TestCopyWorldEssentialsRejectsMissingLevelDat(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(src, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := copyWorldEssentials(src, filepath.Join(tmp, "dst"))
	if err == nil || !strings.Contains(err.Error(), "level.dat") {
		t.Fatalf("expected level.dat error, got %v", err)
	}
}

func TestCopyWorldEssentialsRejectsMissingDB(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	mustWrite(t, filepath.Join(src, "level.dat"), []byte("x"))
	err := copyWorldEssentials(src, filepath.Join(tmp, "dst"))
	if err == nil || !strings.Contains(err.Error(), "db") {
		t.Fatalf("expected db error, got %v", err)
	}
}

func TestCopyDirRecursiveBasic(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), []byte("hello"))
	mustWrite(t, filepath.Join(src, "sub", "b.txt"), []byte("world"))

	if err := copyDirRecursive(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.txt")); string(b) != "hello" {
		t.Errorf("a.txt=%q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "b.txt")); string(b) != "world" {
		t.Errorf("sub/b.txt=%q", b)
	}
}

func TestCopyDirRecursiveMissingSrc(t *testing.T) {
	tmp := t.TempDir()
	err := copyDirRecursive(filepath.Join(tmp, "nope"), filepath.Join(tmp, "dst"))
	if err == nil || (!strings.Contains(err.Error(), "不存在") && !strings.Contains(err.Error(), "不是目录")) {
		t.Fatalf("expected missing-src error, got %v", err)
	}
}

func TestRemoveDirSafelyNoopForMissing(t *testing.T) {
	if err := removeDirSafely(filepath.Join(t.TempDir(), "definitely-not-here")); err != nil {
		t.Fatalf("missing dir should be no-op, got %v", err)
	}
}

func TestRemoveDirSafelyDeletesRealDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "victim")
	mustWrite(t, filepath.Join(dir, "nested", "f.txt"), []byte("x"))
	if err := removeDirSafely(dir); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("dir should be gone")
	}
}
