package main

import "testing"

func TestDetectCliTool(t *testing.T) {
	t.Run("explicit reclaude", func(t *testing.T) {
		t.Setenv("XKPANE_CLI_TOOL", "reclaude")
		if got := detectCliTool(); got != "reclaude" {
			t.Errorf("got %q want reclaude", got)
		}
	})
	t.Run("codex env fallback", func(t *testing.T) {
		t.Setenv("XKPANE_CLI_TOOL", "")
		t.Setenv("CODEX_HOME", "/x")
		if got := detectCliTool(); got != "codex" {
			t.Errorf("got %q want codex", got)
		}
	})
	t.Run("default claude", func(t *testing.T) {
		t.Setenv("XKPANE_CLI_TOOL", "")
		t.Setenv("CODEX_HOME", "")
		t.Setenv("CODEX_SANDBOX", "")
		t.Setenv("CODEX_REMOTE", "")
		if got := detectCliTool(); got != "claude" {
			t.Errorf("got %q want claude", got)
		}
	})
}

func TestDetectRuntimeKind(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_CLIENT", "")
	if got := detectRuntimeKind(); got != "local" {
		t.Errorf("got %q want local", got)
	}
	t.Setenv("SSH_CONNECTION", "1.2.3.4 22 5.6.7.8 22")
	if got := detectRuntimeKind(); got != "ssh" {
		t.Errorf("got %q want ssh", got)
	}
}

func TestResolveProjectDirFromInput(t *testing.T) {
	dir := t.TempDir()
	in := &sessionStartInput{Cwd: dir}
	got := resolveProjectDir(in)
	// resolveProjectDir 返回绝对路径；TempDir 已是绝对路径。
	if got != dir {
		t.Errorf("resolveProjectDir=%q want %q", got, dir)
	}
}
