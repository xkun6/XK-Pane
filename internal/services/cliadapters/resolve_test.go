package cliadapters

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeFile 在 t.TempDir 下写一个占位文件，便于 isFile 命中。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// 守门：找不到无扩展名直接命中时，按扩展名候选逐个探测。
func TestFindExecutableInDirsUsesExtensionCandidates(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "claude.cmd")
	writeFile(t, exe, "echo hi")

	got := findExecutableInDirs("claude", []string{dir}, []string{".cmd", ".exe"})
	if got != exe {
		t.Fatalf("findExecutableInDirs = %q, want %q", got, exe)
	}
}

// 守门：重复目录（大小写不敏感）只探测一次，仍能命中。
func TestFindExecutableInDirsSkipsDuplicateDirectories(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "claude.exe")
	writeFile(t, exe, "binary")

	got := findExecutableInDirs("claude", []string{dir, dir}, []string{".exe"})
	if got != exe {
		t.Fatalf("findExecutableInDirs = %q, want %q", got, exe)
	}
}

// 守门：已带扩展名时直接命中，不再追加候选扩展名。
func TestFindExecutableInDirsRespectsExistingExtension(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "claude.exe")
	writeFile(t, exe, "binary")

	got := findExecutableInDirs("claude.exe", []string{dir}, []string{".cmd"})
	if got != exe {
		t.Fatalf("findExecutableInDirs = %q, want %q", got, exe)
	}
}

// 守门：reclaude 同样可被定位（与 claude 共享同一解析逻辑）。
func TestFindExecutableInDirsLocatesReclaude(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "reclaude.cmd")
	writeFile(t, exe, "echo hi")

	got := findExecutableInDirs("reclaude", []string{dir}, []string{".cmd", ".exe"})
	if got != exe {
		t.Fatalf("findExecutableInDirs = %q, want %q", got, exe)
	}
}

// 守门：Windows npm .cmd shim 改写为 `node cli.js ...`，参数原样保留（含带空格的参数）。
func TestWindowsNpmShimInvocationRunsNodeDirectly(t *testing.T) {
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "Program Files", "nodejs")
	pkgDir := filepath.Join(nodeDir, "node_modules", "@anthropic-ai", "claude-code")

	shim := filepath.Join(nodeDir, "claude.cmd")
	node := filepath.Join(nodeDir, "node.exe")
	cliJS := filepath.Join(pkgDir, "cli.js")
	writeFile(t, shim, "@echo off")
	writeFile(t, node, "node")
	writeFile(t, cliJS, "console.log('claude')")

	command, args := windowsNpmShimInvocation(shim, []string{"--resume", "session id with spaces"})
	if command != node {
		t.Fatalf("command = %q, want %q", command, node)
	}
	want := []string{cliJS, "--resume", "session id with spaces"}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

// 守门：非 batch（.exe）命令原样返回，不做改写。
func TestWindowsNpmShimInvocationLeavesNonBatchUnchanged(t *testing.T) {
	command := `C:\Tools\claude.exe`
	resolved, args := windowsNpmShimInvocation(command, []string{"--version"})
	if resolved != command {
		t.Fatalf("command = %q, want %q", resolved, command)
	}
	if !slices.Equal(args, []string{"--version"}) {
		t.Fatalf("args = %v, want [--version]", args)
	}
}
