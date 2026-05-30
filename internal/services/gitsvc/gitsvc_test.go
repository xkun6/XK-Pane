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
