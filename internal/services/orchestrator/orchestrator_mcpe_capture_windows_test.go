//go:build windows

package orchestrator

import (
	"strings"
	"testing"

	"xkpane/internal/storage"
)

// resolveMCPEPID：runId → PID 解析（多开消歧走 pickMCPERun，本文件只测 PID 层差异）。
func TestResolveMCPEPID(t *testing.T) {
	pidA := 4242
	sums := []storage.MCRunSummary{
		{RunID: "run-a", PackageName: "包A", PID: &pidA},
		{RunID: "run-b", PackageName: "包B"}, // PID nil
	}
	s := New(Deps{MCPE: &fakeMCPE{summaries: sums}})

	// runId 命中 → 返回对应 PID。
	pid, err := s.resolveMCPEPID("run-a")
	if err != nil || pid != 4242 {
		t.Errorf("run-a 应解析出 PID=4242：pid=%d err=%v", pid, err)
	}

	// PID 为 nil → 报「无有效 PID」。
	if _, err := s.resolveMCPEPID("run-b"); err == nil || !strings.Contains(err.Error(), "无有效 PID") {
		t.Errorf("PID nil 应报无有效 PID，实际 %v", err)
	}

	// 多 run 未传 runId → 消歧报错（透传自 pickMCPERun）。
	if _, err := s.resolveMCPEPID(""); err == nil || !strings.Contains(err.Error(), "请传 runId") {
		t.Errorf("多 run 未传 runId 应报请传 runId，实际 %v", err)
	}
}
