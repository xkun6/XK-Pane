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

func TestParseWorktreeList(t *testing.T) {
	// 合成 porcelain：主(branch) + linked(branch) + detached(无 branch) + 末块无结尾空行。
	// 含 \r 以验 CRLF 容忍。
	out := "worktree /repo/main\r\nHEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\nbranch refs/heads/master\r\n" +
		"\r\n" +
		"worktree /repo/wt-feat\nHEAD bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\nbranch refs/heads/feature/x\n" +
		"\n" +
		"worktree /repo/wt-detached\nHEAD cccccccccccccccccccccccccccccccccccccccc\ndetached\n"
	got := parseWorktreeList(out)
	if len(got) != 3 {
		t.Fatalf("parsed %d worktrees, want 3", len(got))
	}
	if !got[0].IsMain || got[1].IsMain || got[2].IsMain {
		t.Errorf("IsMain flags = [%v %v %v], want [true false false]", got[0].IsMain, got[1].IsMain, got[2].IsMain)
	}
	if got[0].Branch != "master" || got[1].Branch != "feature/x" {
		t.Errorf("branches = %q,%q, want master,feature/x", got[0].Branch, got[1].Branch)
	}
	if got[2].Branch != "" {
		t.Errorf("detached worktree branch = %q, want empty", got[2].Branch)
	}
	if got[0].Commit != "aaaaaaa" || len(got[2].Commit) != 7 {
		t.Errorf("commit shorten wrong: %q / %q", got[0].Commit, got[2].Commit)
	}
	if got[2].Path != "/repo/wt-detached" {
		t.Errorf("final block path = %q (末块无结尾空行应仍 flush)", got[2].Path)
	}
}

func TestWorktrees(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c1")

	r, _ := Open(dir)

	// 初始：仅主工作树。
	wts, err := r.Worktrees(ctx)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(wts) != 1 {
		t.Fatalf("initial worktrees = %d, want 1", len(wts))
	}
	if !wts[0].IsMain {
		t.Errorf("sole worktree IsMain=false, want true")
	}
	if wts[0].Branch == "" {
		t.Errorf("main worktree branch empty, want default branch")
	}
	if len(wts[0].Commit) != 7 {
		t.Errorf("commit %q not 7-char short hash", wts[0].Commit)
	}

	// 加一个带新分支的 worktree（路径在 repo 之外，git 自建该目录）。
	wtPath := filepath.Join(t.TempDir(), "wt1")
	if err := r.WorktreeAdd(ctx, wtPath, "feature-wt"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	wts, err = r.Worktrees(ctx)
	if err != nil {
		t.Fatalf("Worktrees after add: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("worktrees after add = %d, want 2", len(wts))
	}
	mainCount := 0
	var added *WorktreeInfo
	for i := range wts {
		if wts[i].IsMain {
			mainCount++
		} else {
			added = &wts[i]
		}
	}
	if mainCount != 1 {
		t.Errorf("IsMain count = %d, want 1", mainCount)
	}
	if added == nil || added.Branch != "feature-wt" {
		t.Fatalf("added worktree branch = %v, want feature-wt", added)
	}

	// 移除（恒 --force）→ 回到仅主树。
	if err := r.WorktreeRemove(ctx, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	wts, _ = r.Worktrees(ctx)
	if len(wts) != 1 {
		t.Fatalf("worktrees after remove = %d, want 1", len(wts))
	}
}

func TestStash(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a")
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c1")

	r, _ := Open(dir)

	// 改一个已跟踪文件 → dirty。
	writeFile(t, dir, "a.txt", "changed")
	if d, _ := r.IsDirty(ctx); !d {
		t.Fatal("pre-stash IsDirty=false, want true")
	}

	// Stash → 工作树回到干净。
	if _, err := r.Stash(ctx); err != nil {
		t.Fatalf("Stash: %v", err)
	}
	if d, _ := r.IsDirty(ctx); d {
		t.Fatal("post-stash IsDirty=true, want false")
	}

	// Pop → 改动恢复。
	if _, err := r.StashPop(ctx); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	if d, _ := r.IsDirty(ctx); !d {
		t.Fatal("post-pop IsDirty=false, want true")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "changed" {
		t.Fatalf("post-pop a.txt = %q, want changed", b)
	}

	// 无 stash 再 pop → 报错。
	if _, err := r.StashPop(ctx); err == nil {
		t.Fatal("StashPop on empty stash returned nil err, want error")
	}
}
