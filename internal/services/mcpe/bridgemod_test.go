package mcpe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBridgeUUIDMatchesManifest(t *testing.T) {
	// manifest.json 里的 UUID 必须与常量一致，否则启动时世界不会自动启用桥模组。
	data, err := bridgeManifest()
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	if !strings.Contains(string(data), BridgeUUID) {
		t.Fatalf("manifest.json 应包含 BridgeUUID %s", BridgeUUID)
	}
}

func TestExtractCreatesFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "bridge")
	if err := ExtractTo(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, rel := range []string{
		"manifest.json",
		filepath.Join("debug_env_scripts", "modMain.py"),
		filepath.Join("debug_env_scripts", "Config.py"),
		filepath.Join("debug_env_scripts", "IPCSystem.py"),
		filepath.Join("debug_env_scripts", "QuModLibs", "__init__.py"), // 验证 _ 开头文件随 all: 一起 embed
	} {
		if !isFile(filepath.Join(target, rel)) {
			t.Errorf("expected extracted file: %s", rel)
		}
	}
}

func TestExtractOverwritesExisting(t *testing.T) {
	target := filepath.Join(t.TempDir(), "bridge")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(target, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("OLD_CONTENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ExtractTo(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), BridgeUUID) || strings.Contains(string(data), "OLD_CONTENT") {
		t.Fatalf("manifest not overwritten: %q", string(data))
	}
}
