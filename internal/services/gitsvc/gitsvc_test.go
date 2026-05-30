package gitsvc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"xkpane/internal/utils/gitx"
	"xkpane/internal/utils/procx"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping fixture-based gitsvc tests")
	}
}

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

func initRepoCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitDo(t, dir, "init")
	gitDo(t, dir, "config", "user.email", "t@t.test")
	gitDo(t, dir, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "c1")
	return dir
}

// GetStatus 是 service 在 gitx 之上**新增**的组合逻辑 —— 重点验它。
func TestGetStatus(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	svc := New()
	dir := initRepoCommit(t)

	// 干净仓库：IsRepo=true / 有分支 / 不脏。
	st, err := svc.GetStatus(ctx, dir)
	if err != nil {
		t.Fatalf("GetStatus(clean): %v", err)
	}
	if !st.IsRepo {
		t.Fatal("clean repo IsRepo=false, want true")
	}
	if st.Branch == "" || st.Branch == "HEAD" {
		t.Errorf("clean repo branch=%q, want a real branch name", st.Branch)
	}
	if st.IsDirty {
		t.Error("clean repo IsDirty=true, want false")
	}

	// 改一个已跟踪文件 → IsDirty=true。
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatalf("rewrite a.txt: %v", err)
	}
	st, err = svc.GetStatus(ctx, dir)
	if err != nil {
		t.Fatalf("GetStatus(dirty): %v", err)
	}
	if !st.IsDirty {
		t.Error("modified repo IsDirty=false, want true")
	}
}

// 非仓库：GetStatus **友好降级**为 {IsRepo:false} 且不报错（对齐 cc-pane null-status）。
func TestGetStatusNonRepo(t *testing.T) {
	ctx := context.Background()
	svc := New()
	dir := t.TempDir() // 普通目录，非 git 仓库

	st, err := svc.GetStatus(ctx, dir)
	if err != nil {
		t.Fatalf("GetStatus(non-repo) err=%v, want nil (friendly degrade)", err)
	}
	if st.IsRepo {
		t.Error("non-repo IsRepo=true, want false")
	}
	if svc.IsRepo(dir) {
		t.Error("IsRepo(non-repo)=true, want false")
	}
}

// 明细方法对非仓库**透传 ErrNotARepo**（与 GetStatus 的降级行为有意不同）。
func TestDetailMethodsNonRepoPropagate(t *testing.T) {
	ctx := context.Background()
	svc := New()
	dir := t.TempDir()

	if _, err := svc.Branches(ctx, dir); !errors.Is(err, gitx.ErrNotARepo) {
		t.Errorf("Branches(non-repo) err=%v, want ErrNotARepo", err)
	}
	if _, err := svc.Log(ctx, dir, 10); !errors.Is(err, gitx.ErrNotARepo) {
		t.Errorf("Log(non-repo) err=%v, want ErrNotARepo", err)
	}
}

// auto-label 钩子：stash/pull/push 前调注入的 labeler（带正确 name/source/branch），
// **即使 git 操作本身失败**（如 push 无上游）钩子也应已先触发。
func TestAutoLabelerInvokedBeforeMutations(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	svc := New()
	dir := initRepoCommit(t)

	type call struct{ repoPath, name, source, branch string }
	var calls []call
	svc.SetAutoLabeler(func(_ context.Context, repoPath, name, source, branch string) error {
		calls = append(calls, call{repoPath, name, source, branch})
		return nil
	})

	// 改动文件使 stash 有内容（非必需，仅更真实）。
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, err := svc.Stash(ctx, dir); err != nil {
		t.Fatalf("Stash: %v", err)
	}
	// Push 无上游会失败，但 labelBefore 应已在失败前触发。
	_ = svc.Push(ctx, dir)

	if len(calls) != 2 {
		t.Fatalf("labeler calls = %d, want 2 (stash + push)", len(calls))
	}
	if calls[0].name != "Before Stash" || calls[0].source != "git_stash" || calls[0].repoPath != dir {
		t.Errorf("stash label call = %+v, want Before Stash/git_stash/%s", calls[0], dir)
	}
	if calls[0].branch == "" {
		t.Error("stash label branch empty, want default branch")
	}
	if calls[1].name != "Before Push" || calls[1].source != "git_push" {
		t.Errorf("push label call = %+v, want Before Push/git_push", calls[1])
	}
}

// 未注入 labeler：变更操作不因缺少钩子而 panic。
func TestNoLabelerNoPanic(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	svc := New() // 无 SetAutoLabeler
	dir := initRepoCommit(t)
	if _, err := svc.Stash(ctx, dir); err != nil {
		t.Fatalf("Stash without labeler: %v", err)
	}
}

// validateWorktreeName：纯校验逻辑（service 在原语之上新增），不需 git 二进制。
func TestValidateWorktreeName(t *testing.T) {
	bad := []string{"", "   ", "feat/x", `feat\x`, "..", "a..b", "-rf", "-"}
	for _, n := range bad {
		if err := validateWorktreeName(n); err == nil {
			t.Errorf("validateWorktreeName(%q) = nil, want error", n)
		}
	}
	good := []string{"feature-wt", "wt1", "my_branch", "v2.1"}
	for _, n := range good {
		if err := validateWorktreeName(n); err != nil {
			t.Errorf("validateWorktreeName(%q) = %v, want nil", n, err)
		}
	}
}

// WorktreeAddUnder：service 层的分组目录策略 —— 校验名 → join(parent,name) → 建树。
func TestWorktreeAddUnder(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	svc := New()
	dir := initRepoCommit(t)

	parent := t.TempDir()
	created, err := svc.WorktreeAddUnder(ctx, dir, parent, "feature-wt", "feature-wt")
	if err != nil {
		t.Fatalf("WorktreeAddUnder: %v", err)
	}
	if created != filepath.Join(parent, "feature-wt") {
		t.Errorf("created path = %q, want %q", created, filepath.Join(parent, "feature-wt"))
	}
	if _, statErr := os.Stat(created); statErr != nil {
		t.Errorf("worktree dir not created: %v", statErr)
	}

	// 出现在 Worktrees 列表里，且带新分支。
	wts, err := svc.Worktrees(ctx, dir)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	// 注意：filepath.Join 在 Windows 产出反斜杠，而 `git worktree list --porcelain`
	// 回正斜杠 —— 按分隔符归一后比较（git 在 Windows 两种分隔符都接受，故移除仍按列表路径可行）。
	var found bool
	for _, w := range wts {
		if filepath.ToSlash(w.Path) == filepath.ToSlash(created) && w.Branch == "feature-wt" {
			found = true
		}
	}
	if !found {
		t.Errorf("added worktree not found in list %+v", wts)
	}

	// 坏名直接被校验挡下（不触达 git）。
	if _, err := svc.WorktreeAddUnder(ctx, dir, parent, "bad/name", ""); err == nil {
		t.Error("WorktreeAddUnder(bad name) = nil err, want validation error")
	}
}

// 委托链通畅：Branches / Log 经 service 拿到与底层一致的结果。
func TestBranchesAndLogDelegate(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	svc := New()
	dir := initRepoCommit(t)

	brs, err := svc.Branches(ctx, dir)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if len(brs) != 1 || !brs[0].IsCurrent {
		t.Fatalf("Branches=%v, want single current branch", brs)
	}

	logs, err := svc.Log(ctx, dir, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(logs) != 1 || logs[0].Subject != "c1" {
		t.Fatalf("Log=%v, want single commit c1", logs)
	}
}
