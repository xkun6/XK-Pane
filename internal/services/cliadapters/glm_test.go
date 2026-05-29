package cliadapters

import "testing"

// 守门（路径铁律）：GLM 透传 crush 的 --cwd 永远 = ProjectPath，即便有 workspace。
func TestGlmLaunchCwdAlwaysUsesProjectPath(t *testing.T) {
	got := glmLaunchCwd(&Context{ProjectPath: "/a/proj", WorkspacePath: "/a"})
	if got != "/a/proj" {
		t.Fatalf("glmLaunchCwd = %q, want /a/proj", got)
	}
}

// 守门（路径铁律）：workspace 缺省时 --cwd 仍 = ProjectPath。
func TestGlmLaunchCwdUsesProjectPathWhenWorkspaceAbsent(t *testing.T) {
	got := glmLaunchCwd(&Context{ProjectPath: "/a/proj"})
	if got != "/a/proj" {
		t.Fatalf("glmLaunchCwd = %q, want /a/proj", got)
	}
}
