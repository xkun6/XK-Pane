package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

// 守门：CleanupMCPConfigs 删掉退出那刻所有活跃会话的 <cwd>/.claude/mcp-<sid>.json
// （app 退出兜底，补 waitLoop 没赶上的）。
func TestCleanupMCPConfigsSweepsActiveSessions(t *testing.T) {
	proj := t.TempDir()
	claudeDir := filepath.Join(proj, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := New(nil)
	ids := []string{"sid-a", "sid-b"}
	for _, id := range ids {
		p := filepath.Join(claudeDir, "mcp-"+id+".json")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		s.sessions[id] = &session{id: id, cwd: proj}
	}

	s.CleanupMCPConfigs()

	for _, id := range ids {
		if _, err := os.Stat(filepath.Join(claudeDir, "mcp-"+id+".json")); !os.IsNotExist(err) {
			t.Errorf("退出兜底应删除 mcp-%s.json, stat err = %v", id, err)
		}
	}
}

// 守门：无活跃会话时 no-op，不报错不 panic。
func TestCleanupMCPConfigsNoopWhenNoSessions(t *testing.T) {
	New(nil).CleanupMCPConfigs()
}
