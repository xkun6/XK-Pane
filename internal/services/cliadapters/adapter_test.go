package cliadapters

import (
	"slices"
	"testing"
)

// ── 路径字段铁律（D7）：--add-dir 三态（claude/reclaude/kimi 共享 buildAddDirArgs）──

// 守门：workspace ≠ project 时拼 --add-dir <workspace>，且不会误用 project_path。
func TestAddDirUsesWorkspaceWhenDifferentFromProject(t *testing.T) {
	args := buildAddDirArgs(&Context{ProjectPath: "/a/proj", WorkspacePath: "/a"})
	if !slices.Equal(args, []string{"--add-dir", "/a"}) {
		t.Fatalf("args = %v, want [--add-dir /a]", args)
	}
	if slices.Contains(args, "/a/proj") {
		t.Fatalf("args must not contain project_path: %v", args)
	}
}

// 守门：workspace == project 时不拼 --add-dir。
func TestAddDirSkippedWhenWorkspaceEqualsProject(t *testing.T) {
	args := buildAddDirArgs(&Context{ProjectPath: "/a/proj", WorkspacePath: "/a/proj"})
	if len(args) != 0 {
		t.Fatalf("args = %v, want empty", args)
	}
}

// 守门：workspace 缺省时不拼 --add-dir。
func TestAddDirSkippedWhenWorkspaceAbsent(t *testing.T) {
	args := buildAddDirArgs(&Context{ProjectPath: "/a/proj"})
	if len(args) != 0 {
		t.Fatalf("args = %v, want empty", args)
	}
}

// ── 注册表 ──

// 守门：Default() 注册 Claude / Reclaude / Codex / Grok Build（用户 2026-05-30 移除其余 5 个
// CLI；2026-07-17 新增 Grok Build），顺序对齐 cc-pane 生产注册点（Reclaude 紧跟 Claude，Grok
// Build 紧跟 Codex）。
func TestDefaultRegistersClaudeReclaudeCodexGrok(t *testing.T) {
	want := []string{"claude", "reclaude", "codex", "grok"}
	tools := Default().ListTools()
	got := make([]string, 0, len(tools))
	for _, ti := range tools {
		got = append(got, ti.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Default().ListTools() ids = %v, want %v", got, want)
	}
}

// 守门：保留的适配器关键元信息/能力正确（reclaude 版本子命令=version；codex 可执行=codex；
// claude/reclaude/codex 均支持多目录 --add-dir/cwd 能力）。
func TestAdapterMetadataAndCapabilities(t *testing.T) {
	r := Default()

	reclaude, ok := r.Get("reclaude")
	if !ok {
		t.Fatal("reclaude not registered")
	}
	if !slices.Equal(reclaude.Info().VersionArgs, []string{"version"}) {
		t.Fatalf("reclaude versionArgs = %v, want [version]", reclaude.Info().VersionArgs)
	}

	codex, ok := r.Get("codex")
	if !ok {
		t.Fatal("codex not registered")
	}
	if codex.Info().Executable != "codex" {
		t.Fatalf("codex executable = %q, want codex", codex.Info().Executable)
	}

	for _, id := range []string{"claude", "reclaude", "codex"} {
		a, _ := r.Get(id)
		if !a.Capabilities().SupportsWorkspace {
			t.Errorf("%s should support workspace", id)
		}
	}
}
