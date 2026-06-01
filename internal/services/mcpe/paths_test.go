package mcpe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLauncherRejectsNonExe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "launcher.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateLauncher(p)
	if err == nil || !strings.Contains(err.Error(), ".exe") {
		t.Fatalf("expected .exe error, got %v", err)
	}
}

func TestValidateLauncherRejectsMissing(t *testing.T) {
	err := validateLauncher(filepath.Join(t.TempDir(), "nope", "launcher.exe"))
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("expected missing error, got %v", err)
	}
}

func TestScanLaunchersNoPanic(t *testing.T) {
	_ = scanLaunchers() // 不预设环境，仅验证不 panic
}

func TestMcDirOptional(t *testing.T) {
	_, _ = mcDir() // 无网易 MC 环境返回 false；有则 true。仅验证不 panic
}
