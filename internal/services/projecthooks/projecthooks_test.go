package projecthooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

const fakeBin = `C:\Program\bin\xkpane-cli-hook.exe`

// 读 settings.local.json 的 hooks[event] 数组。
func readClaudeHooks(t *testing.T, projectPath, event string) []any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectPath, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	hooks, _ := root["hooks"].(map[string]any)
	arr, _ := hooks[event].([]any)
	return arr
}

func TestSyncClaudeHooksWritesSessionStartAndPlanArchive(t *testing.T) {
	dir := t.TempDir()
	if err := syncClaudeHooks(dir, fakeBin); err != nil {
		t.Fatal(err)
	}

	ss := readClaudeHooks(t, dir, "SessionStart")
	if len(ss) != 1 {
		t.Fatalf("SessionStart 条目数=%d want 1", len(ss))
	}
	entry := ss[0].(map[string]any)
	if entry["matcher"] != "startup" {
		t.Errorf("matcher=%v want startup", entry["matcher"])
	}
	cmd := entry["hooks"].([]any)[0].(map[string]any)["command"].(string)
	if !strings.Contains(cmd, "xkpane-cli-hook") || !strings.Contains(cmd, "session-start") {
		t.Errorf("command=%q 缺二进制名或子命令", cmd)
	}

	if pa := readClaudeHooks(t, dir, "PostToolUse"); len(pa) != 1 {
		t.Errorf("PostToolUse 条目数=%d want 1", len(pa))
	}
}

// 重复 sync 应幂等（不堆叠重复条目），且保留外部 hook。
func TestSyncClaudeHooksIdempotentAndPreservesForeign(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 预置一个外部（非本工具）的 SessionStart hook。
	seed := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "startup",
					"hooks": []any{
						map[string]any{"type": "command", "command": "other-tool init"},
					},
				},
			},
		},
		"otherKey": "preserved",
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// sync 两次。
	if err := syncClaudeHooks(dir, fakeBin); err != nil {
		t.Fatal(err)
	}
	if err := syncClaudeHooks(dir, fakeBin); err != nil {
		t.Fatal(err)
	}

	ss := readClaudeHooks(t, dir, "SessionStart")
	// 外部 1 条 + 本工具 1 条（幂等不堆叠）= 2。
	if len(ss) != 2 {
		t.Fatalf("SessionStart 条目数=%d want 2（外部+本工具，不重复）", len(ss))
	}
	foreign, managed := 0, 0
	for _, e := range ss {
		if isManagedEntry(e) {
			managed++
		} else {
			foreign++
		}
	}
	if foreign != 1 || managed != 1 {
		t.Errorf("foreign=%d managed=%d want 1/1", foreign, managed)
	}

	// 外部顶层键保留。
	data, _ = os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	var root map[string]any
	_ = json.Unmarshal(data, &root)
	if root["otherKey"] != "preserved" {
		t.Errorf("外部顶层键被丢失")
	}
}

func TestSyncCodexHooksWritesHooksAndFeatureFlag(t *testing.T) {
	dir := t.TempDir()
	if err := syncCodexHooks(dir, fakeBin); err != nil {
		t.Fatal(err)
	}

	// hooks.json：仅 SessionStart（codex 无 plan-archive）。
	data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	_ = json.Unmarshal(data, &root)
	hooks := root["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; ok {
		t.Error("codex 不该有 PostToolUse hook")
	}
	ss := hooks["SessionStart"].([]any)
	entry := ss[0].(map[string]any)
	if entry["matcher"] != "startup|resume" {
		t.Errorf("codex matcher=%v want startup|resume", entry["matcher"])
	}
	inner := entry["hooks"].([]any)[0].(map[string]any)
	if _, ok := inner["statusMessage"]; !ok {
		t.Error("codex hook 应含 statusMessage")
	}

	// config.toml：[features] codex_hooks=true。
	cfgData, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatal(err)
	}
	features, _ := cfg["features"].(map[string]any)
	if features["codex_hooks"] != true {
		t.Errorf("features.codex_hooks=%v want true", features["codex_hooks"])
	}
}

// .codex 是文件时应报错（冲突检查）。
func TestSyncCodexHooksDotCodexFileConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".codex"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncCodexHooks(dir, fakeBin); err == nil {
		t.Error("期望 .codex 文件冲突报错")
	}
}

// ensureCodexHooksFeature 保留用户既有 config.toml 配置。
func TestEnsureCodexHooksFeaturePreservesExisting(t *testing.T) {
	dir := t.TempDir()
	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("model = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureCodexHooksFeature(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	var cfg map[string]any
	_ = toml.Unmarshal(data, &cfg)
	if cfg["model"] != "gpt-5" {
		t.Errorf("既有 model 配置丢失: %v", cfg["model"])
	}
	if cfg["features"].(map[string]any)["codex_hooks"] != true {
		t.Error("codex_hooks 未写入")
	}
}

func TestSyncNoOpForBareShell(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "none"); err != nil {
		t.Errorf("none 应 no-op, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude")); !os.IsNotExist(err) {
		t.Error("none 不该建 .claude")
	}
}
