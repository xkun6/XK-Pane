package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xkpane/internal/utils/procx"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping fixture-based gitx tests")
	}
}

// gitDo 用真 git CLI（经 procx）构造测试夹具——独立于被测的 go-git，使夹具可信。
func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	err := procx.Run(context.Background(), procx.Spec{
		Name:          "git",
		Args:          args,
		Dir:           dir,
		Env:           append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0"),
		CaptureOutput: true,
	})
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// initRepo: git init + 本地 user 配置（不依赖全局 config）。
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitDo(t, dir, "init")
	gitDo(t, dir, "config", "user.email", "t@t.test")
	gitDo(t, dir, "config", "user.name", "tester")
	return dir
}

func TestCurrentBranchBranchesLog(t *testing.T) {
	requireGit(t)
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c1")

	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	def, err := r.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if def == "" || def == "HEAD" {
		t.Fatalf("default branch unexpected: %q", def)
	}

	writeFile(t, dir, "b.txt", "b")
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c2")
	gitDo(t, dir, "checkout", "-b", "feature")

	r2, _ := Open(dir)
	if cb, _ := r2.CurrentBranch(); cb != "feature" {
		t.Fatalf("CurrentBranch after checkout = %q, want feature", cb)
	}

	brs, err := r2.Branches()
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	names := map[string]bool{}
	currentCount := 0
	var currentName string
	for _, b := range brs {
		names[b.Name] = true
		if b.IsCurrent {
			currentCount++
			currentName = b.Name
		}
	}
	if !names[def] || !names["feature"] {
		t.Fatalf("branch set = %v, want {%s, feature}", names, def)
	}
	if currentCount != 1 || currentName != "feature" {
		t.Fatalf("current branch count=%d name=%q, want 1/feature", currentCount, currentName)
	}

	logs, err := r2.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("Log len=%d, want >=2", len(logs))
	}
	if logs[0].Subject != "c2" {
		t.Errorf("newest subject = %q, want c2", logs[0].Subject)
	}
	if len(logs[0].ShortHash) != 7 || !strings.HasPrefix(logs[0].Hash, logs[0].ShortHash) {
		t.Errorf("shortHash %q not a 7-char prefix of %q", logs[0].ShortHash, logs[0].Hash)
	}
	if n, err := r2.Log(1); err != nil || len(n) != 1 {
		t.Errorf("Log(1) len=%d err=%v, want 1", len(n), err)
	}

	// detached HEAD
	gitDo(t, dir, "checkout", logs[0].Hash)
	r3, _ := Open(dir)
	if cb, _ := r3.CurrentBranch(); cb != "HEAD" {
		t.Fatalf("detached CurrentBranch = %q, want HEAD", cb)
	}
}

func TestIsDirty(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c1")

	r, _ := Open(dir)
	if d, err := r.IsDirty(ctx); err != nil || d {
		t.Fatalf("clean tree IsDirty=%v err=%v, want false", d, err)
	}
	writeFile(t, dir, "a.txt", "changed")
	if d, err := r.IsDirty(ctx); err != nil || !d {
		t.Fatalf("modified IsDirty=%v err=%v, want true", d, err)
	}
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c2")
	if d, err := r.IsDirty(ctx); err != nil || d {
		t.Fatalf("after commit IsDirty=%v err=%v, want false", d, err)
	}
	writeFile(t, dir, "untracked.txt", "x")
	if d, err := r.IsDirty(ctx); err != nil || !d {
		t.Fatalf("untracked IsDirty=%v err=%v, want true", d, err)
	}
}

func TestEmptyRepo(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := initRepo(t) // init only, no commit

	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open empty repo: %v", err)
	}
	if cb, err := r.CurrentBranch(); err != nil || cb != "" {
		t.Errorf("empty repo CurrentBranch=%q err=%v, want \"\"", cb, err)
	}
	if logs, err := r.Log(10); err != nil || logs != nil {
		t.Errorf("empty repo Log=%v err=%v, want nil", logs, err)
	}
	if d, err := r.IsDirty(ctx); err != nil || d {
		t.Errorf("empty repo IsDirty=%v err=%v, want false", d, err)
	}
}

func TestNotARepo(t *testing.T) {
	// 不需要 git 二进制：go-git PlainOpen 即可判定。
	dir := t.TempDir()
	if _, err := Open(dir); !errors.Is(err, ErrNotARepo) {
		t.Fatalf("Open(non-repo) = %v, want ErrNotARepo", err)
	}
	if IsRepo(dir) {
		t.Fatal("IsRepo(non-repo) = true, want false")
	}
}
