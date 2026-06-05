package cliadapters

import (
	"os"
	"path/filepath"
	"testing"
)

// 守门：CleanupSessionMCPConfig 只删【本会话】的 .claude/mcp-<sid>.json，
// 同目录里其它会话的 mcp-*.json 必须原样保留（sid 唯一 = per-session 隔离的核心保证）。
func TestCleanupSessionMCPConfigRemovesOnlyOwnFile(t *testing.T) {
	proj := t.TempDir()
	claudeDir := filepath.Join(proj, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(claudeDir, mcpConfigFileName("sid-mine"))
	other := filepath.Join(claudeDir, mcpConfigFileName("sid-other"))
	for _, p := range []string{mine, other} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	CleanupSessionMCPConfig(proj, "sid-mine")

	if _, err := os.Stat(mine); !os.IsNotExist(err) {
		t.Errorf("本会话 %s 应被删除, stat err = %v", filepath.Base(mine), err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("他会话 %s 不应被动, stat err = %v", filepath.Base(other), err)
	}
}

// 守门：文件不存在（codex/none、或 orchestrator 未就绪没写过）/ 空入参时静默 no-op，不报错不 panic。
func TestCleanupSessionMCPConfigNoopWhenAbsent(t *testing.T) {
	proj := t.TempDir() // 连 .claude 都没建
	CleanupSessionMCPConfig(proj, "sid-x")
	CleanupSessionMCPConfig(proj, "")
	CleanupSessionMCPConfig("", "sid-x")
}
