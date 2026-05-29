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

// 守门：Default() 注册全部 8 个适配器，顺序对齐 cc-pane 生产注册点（Reclaude 紧跟 Claude）。
func TestDefaultRegistersAllEightInOrder(t *testing.T) {
	want := []string{"claude", "reclaude", "codex", "gemini", "kimi", "glm", "opencode", "cursor"}
	got := Default().IDs()
	if !slices.Equal(got, want) {
		t.Fatalf("Default().IDs() = %v, want %v", got, want)
	}
}

// 守门：关键元信息/能力正确（易错点：GLM 可执行=crush；reclaude 版本子命令=version；多目录能力）。
func TestAdapterMetadataAndCapabilities(t *testing.T) {
	r := Default()

	glm, ok := r.Get("glm")
	if !ok {
		t.Fatal("glm not registered")
	}
	if glm.Info().Executable != "crush" {
		t.Fatalf("glm executable = %q, want crush", glm.Info().Executable)
	}

	reclaude, _ := r.Get("reclaude")
	if !slices.Equal(reclaude.Info().VersionArgs, []string{"version"}) {
		t.Fatalf("reclaude versionArgs = %v, want [version]", reclaude.Info().VersionArgs)
	}

	cursor, _ := r.Get("cursor")
	if cursor.Info().Executable != "cursor-agent" {
		t.Fatalf("cursor executable = %q, want cursor-agent", cursor.Info().Executable)
	}

	// 多目录能力：claude/reclaude/kimi 支持 workspace；cursor/gemini/opencode 不支持。
	for _, id := range []string{"claude", "reclaude", "kimi"} {
		a, _ := r.Get(id)
		if !a.Capabilities().SupportsWorkspace {
			t.Errorf("%s should support workspace", id)
		}
	}
	for _, id := range []string{"cursor", "gemini", "opencode"} {
		a, _ := r.Get(id)
		if a.Capabilities().SupportsWorkspace {
			t.Errorf("%s should NOT support workspace", id)
		}
	}
}
