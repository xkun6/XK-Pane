package worktreesvc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xkpane/internal/storage"
	"xkpane/internal/utils/procx"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping worktreesvc tests")
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

// initRepo：git init + 首提交（worktree add 需要已生 HEAD）。
// t.Setenv 隔离全局/系统 git 配置：被测 service 内部 runGit 用 os.Environ()，开发机若
// 全局开了 commit.gpgsign / core.hooksPath，自动提交会调 gpg/hook 导致测试失败或挂死。
func initRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	gitDo(t, dir, "init")
	gitDo(t, dir, "config", "user.email", "t@t.test")
	gitDo(t, dir, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDo(t, dir, "add", ".")
	gitDo(t, dir, "commit", "-m", "init")
	return dir
}

// stubRegistry 记录登记调用（WorkspaceRegistry 窄接口 stub）。
type stubRegistry struct {
	workspaces []storage.Workspace
	added      []string // AddProject 收到的 path
	aliases    []string
}

func (s *stubRegistry) List(context.Context) ([]storage.Workspace, error) {
	return s.workspaces, nil
}

func (s *stubRegistry) AddProject(_ context.Context, workspaceID, path string) (storage.WorkspaceProject, error) {
	s.added = append(s.added, path)
	return storage.WorkspaceProject{ID: "wp-1", WorkspaceID: workspaceID, Path: path}, nil
}

func (s *stubRegistry) SetProjectAlias(_ context.Context, _ string, alias *string) error {
	if alias != nil {
		s.aliases = append(s.aliases, *alias)
	}
	return nil
}

func TestCreateForProjectFullFlow(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	reg := &stubRegistry{workspaces: []storage.Workspace{{
		ID:       "ws-1",
		Name:     "测试空间",
		Projects: []storage.WorkspaceProject{{ID: "p-1", WorkspaceID: "ws-1", Path: repoDir}},
	}}}
	svc := New(reg)
	svc.setWorktreesRoot(t.TempDir())

	res, err := svc.CreateForProject(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("CreateForProject: %v", err)
	}

	// ① 首次创建应补 .gitignore 并提交，且工作树保持干净（提交后无未跟踪/未提交残留）。
	if !res.IgnoreCommitted {
		t.Fatal("first create should commit .gitignore")
	}
	gi, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, want := range []string{".claude/", ".codex/", ".xkpanes/"} {
		if !strings.Contains(string(gi), want) {
			t.Errorf(".gitignore missing %q; got:\n%s", want, gi)
		}
	}

	// ② worktree 落在集中根、分支 = slug、目录存在且是 git 工作树。
	if res.Branch == "" || res.WorktreePath == "" {
		t.Fatalf("empty result: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(res.WorktreePath, ".git")); err != nil {
		t.Fatalf("worktree .git missing: %v", err)
	}
	if filepath.Base(res.WorktreePath) != res.Branch {
		t.Errorf("dir name %q should equal branch %q", filepath.Base(res.WorktreePath), res.Branch)
	}

	// ③ 登记进了包含主仓的 workspace，alias = slug。
	if res.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want ws-1", res.WorkspaceID)
	}
	if len(reg.added) != 1 || reg.added[0] != res.WorktreePath {
		t.Errorf("AddProject paths = %v, want [%s]", reg.added, res.WorktreePath)
	}
	if len(reg.aliases) != 1 || reg.aliases[0] != res.Branch {
		t.Errorf("alias = %v, want [%s]", reg.aliases, res.Branch)
	}

	// ④ 第二次创建：.gitignore 已覆盖 → 不再提交，直接建第二棵树（不同 slug）。
	res2, err := svc.CreateForProject(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("second CreateForProject: %v", err)
	}
	if res2.IgnoreCommitted {
		t.Error("second create should not commit .gitignore again")
	}
	if res2.WorktreePath == res.WorktreePath || res2.Branch == res.Branch {
		t.Errorf("second worktree should differ: %+v vs %+v", res2, res)
	}
}

func TestCreateSkipsCommitWhenAlreadyIgnored(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	// 预置 .gitignore 已覆盖三个目录（用户拍板：在 .gitignore 就正常创建）。
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"),
		[]byte(".claude/\n.codex/\n.xkpanes/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repoDir, "add", ".gitignore")
	gitDo(t, repoDir, "commit", "-m", "ignore tool dirs")

	svc := New(nil) // 无 workspace 注册器：登记降级跳过
	svc.setWorktreesRoot(t.TempDir())
	res, err := svc.CreateForProject(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("CreateForProject: %v", err)
	}
	if res.IgnoreCommitted {
		t.Error("should not commit when dirs already ignored")
	}
	if res.WorkspaceID != "" {
		t.Errorf("WorkspaceID should be empty without registry, got %q", res.WorkspaceID)
	}
	if _, err := os.Stat(res.WorktreePath); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
}

func TestSkipsIgnoreForTrackedToolDirs(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	// .claude 下有被跟踪文件（如本仓自己的 bundled skills）→ 不得把 .claude/ 写进
	// .gitignore（否则用户后续新增文件被 git add 静默漏收）；其余两个目录照常补。
	if err := os.MkdirAll(filepath.Join(repoDir, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".claude", "skills", "a.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repoDir, "add", ".claude")
	gitDo(t, repoDir, "commit", "-m", "track claude assets")

	svc := New(nil)
	svc.setWorktreesRoot(t.TempDir())
	res, err := svc.CreateForProject(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("CreateForProject: %v", err)
	}
	if !res.IgnoreCommitted {
		t.Fatal("should still commit ignore for .codex/.xkpanes")
	}
	gi, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gi), ".claude/") {
		t.Errorf(".claude/ must NOT be ignored (has tracked files); got:\n%s", gi)
	}
	for _, want := range []string{".codex/", ".xkpanes/"} {
		if !strings.Contains(string(gi), want) {
			t.Errorf(".gitignore missing %q; got:\n%s", want, gi)
		}
	}
	if _, err := os.Stat(res.WorktreePath); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
}

func TestDirtyGitignoreBlocksAutoCommit(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	// 用户对 .gitignore 自身有未提交 WIP → 必须明确报错，绝不把 WIP 卷进自动提交。
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("*.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(nil)
	svc.setWorktreesRoot(t.TempDir())
	_, err := svc.CreateForProject(context.Background(), repoDir)
	if err == nil || !strings.Contains(err.Error(), ".gitignore has uncommitted changes") {
		t.Fatalf("expected dirty-gitignore error, got: %v", err)
	}
	// .gitignore 原样未动、未产生任何提交。
	gi, _ := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if string(gi) != "*.log\n" {
		t.Errorf(".gitignore should be untouched, got:\n%s", gi)
	}
}

func TestSubdirProjectPathAnchorsToRoot(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	// workspace 登记的可以是 monorepo 子目录：.gitignore 必须写到仓库根并在根提交，
	// 子目录自己的 .gitignore 绝不能被卷进自动提交。
	sub := filepath.Join(repoDir, "packages", "app")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("dist/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repoDir, "add", ".")
	gitDo(t, repoDir, "commit", "-m", "sub gitignore")
	// 子目录 .gitignore 再加未提交改动（审查实证场景：曾被错误卷进自动提交）。
	if err := os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("dist/\nwip-edit/\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := New(nil)
	svc.setWorktreesRoot(t.TempDir())
	res, err := svc.CreateForProject(context.Background(), sub)
	if err != nil {
		t.Fatalf("CreateForProject(subdir): %v", err)
	}
	if !res.IgnoreCommitted {
		t.Fatal("expected ignore commit at repo root")
	}
	if res.RepoRoot == sub {
		t.Errorf("RepoRoot should be repo root, got subdir %q", res.RepoRoot)
	}
	// 根 .gitignore 已建且含三目录；子目录 .gitignore 的 WIP 原样保留未被提交。
	gi, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatalf("root .gitignore missing: %v", err)
	}
	for _, want := range []string{".claude/", ".codex/", ".xkpanes/"} {
		if !strings.Contains(string(gi), want) {
			t.Errorf("root .gitignore missing %q", want)
		}
	}
	var status strings.Builder
	if err := procx.Run(context.Background(), procx.Spec{
		Name: "git", Args: []string{"status", "--porcelain"}, Dir: repoDir,
		Env:           append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1"),
		CaptureOutput: true,
		OnStdout:      func(line string) { status.WriteString(line + "\n") },
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status.String(), "packages/app/.gitignore") {
		t.Errorf("subdir .gitignore WIP should survive uncommitted; status:\n%s", status.String())
	}
}

func TestCreateDoesNotSweepStagedChanges(t *testing.T) {
	requireGit(t)
	repoDir := initRepo(t)
	// 用户已暂存的无关改动绝不能被自动 .gitignore 提交带走（pathspec 限定提交）。
	if err := os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("wip\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repoDir, "add", "feature.txt")

	svc := New(nil)
	svc.setWorktreesRoot(t.TempDir())
	res, err := svc.CreateForProject(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("CreateForProject: %v", err)
	}
	if !res.IgnoreCommitted {
		t.Fatal("expected ignore commit")
	}
	// feature.txt 应仍处于暂存态（未被一并提交）。
	var out strings.Builder
	err = procx.Run(context.Background(), procx.Spec{
		Name: "git", Args: []string{"status", "--porcelain"}, Dir: repoDir,
		Env:           append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1"),
		CaptureOutput: true,
		OnStdout:      func(line string) { out.WriteString(line + "\n") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "A  feature.txt") {
		t.Errorf("staged feature.txt should survive; status:\n%s", out.String())
	}
}
