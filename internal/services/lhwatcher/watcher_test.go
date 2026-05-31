package lhwatcher

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"xkpane/internal/localhistory"
)

// fakeSaver 是 Saver 的测试替身：记录被调用的写操作，GetConfig 返回固定配置。
type fakeSaver struct {
	cfg localhistory.HistoryConfig

	mu      sync.Mutex
	saved   []string // SaveWatched 的 filePath
	deleted []string // SaveDeletion 的 filePath
	labels  []string // AutoLabel 的 name
}

func (f *fakeSaver) GetConfig(_ context.Context, _ string) (localhistory.HistoryConfig, error) {
	return f.cfg, nil
}

func (f *fakeSaver) SaveWatched(_ context.Context, _, filePath, _, _ string, _ int64) (*localhistory.FileVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, filePath)
	return &localhistory.FileVersion{ID: "v", FilePath: filePath}, nil
}

func (f *fakeSaver) SaveDeletion(_ context.Context, _, filePath, _ string) (*localhistory.FileVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, filePath)
	return &localhistory.FileVersion{ID: "d", FilePath: filePath, IsDeleted: true}, nil
}

func (f *fakeSaver) AutoLabel(_ context.Context, _, name, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels = append(f.labels, name)
	return "label-id", nil
}

func (f *fakeSaver) savedPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.saved...)
	sort.Strings(out)
	return out
}

func newTestWatcher(t *testing.T, dir string, cfg localhistory.HistoryConfig) (*Manager, *projectWatcher, *fakeSaver) {
	t.Helper()
	fs := &fakeSaver{cfg: cfg}
	m := New(fs)
	pw := m.newProjectWatcher(dir)
	return m, pw, fs
}

// TestRecordPendingMergeAndDrain 验证 debounce 窗口内同路径事件的合并语义。
func TestRecordPendingMergeAndDrain(t *testing.T) {
	_, pw, _ := newTestWatcher(t, t.TempDir(), localhistory.DefaultHistoryConfig())

	// 变更 + 再变更 → 仍只 1 条变更（窗口合并）。
	pw.recordPending("a.txt", false)
	pw.recordPending("a.txt", false)
	// 变更后删除 → 只剩删除。
	pw.recordPending("b.txt", false)
	pw.recordPending("b.txt", true)
	// 删除后又变更（重建）→ 只剩变更。
	pw.recordPending("c.txt", true)
	pw.recordPending("c.txt", false)

	changed, deleted := pw.drainPending()
	sort.Strings(changed)
	sort.Strings(deleted)

	wantChanged := []string{"a.txt", "c.txt"}
	wantDeleted := []string{"b.txt"}
	if !equalStrs(changed, wantChanged) {
		t.Errorf("changed = %v, want %v", changed, wantChanged)
	}
	if !equalStrs(deleted, wantDeleted) {
		t.Errorf("deleted = %v, want %v", deleted, wantDeleted)
	}

	// 再次 drain 应为空（已取走）。
	c2, d2 := pw.drainPending()
	if len(c2) != 0 || len(d2) != 0 {
		t.Errorf("second drain not empty: changed=%v deleted=%v", c2, d2)
	}
}

// TestFlushSilenceWindowDropsEvents 验证分支切换静默窗口内 flush 丢弃文件事件（不保存）。
func TestFlushSilenceWindowDropsEvents(t *testing.T) {
	_, pw, fs := newTestWatcher(t, t.TempDir(), localhistory.DefaultHistoryConfig())
	pw.silenceUntil = time.Now().Add(time.Hour) // 处于静默窗口
	pw.recordPending("x.txt", false)
	pw.recordPending("y.txt", true)

	pw.flush()

	if got := fs.savedPaths(); len(got) != 0 {
		t.Errorf("expected no saves during silence, got %v", got)
	}
	if len(fs.deleted) != 0 {
		t.Errorf("expected no deletions during silence, got %v", fs.deleted)
	}
	// pending 应被清空（丢弃）。
	if c, d := pw.drainPending(); len(c) != 0 || len(d) != 0 {
		t.Errorf("pending not drained after silenced flush: changed=%v deleted=%v", c, d)
	}
}

// TestFlushFiltersAndSaves 验证 flush 的过滤管线：文本保存、二进制 / 忽略项 / 超大跳过。
func TestFlushFiltersAndSaves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.txt", "hi\n")        // 3 字节，≤ 上限 → 保存
	writeFile(t, dir, "skip.bin", "ab\x00cd")    // 含 NUL → 二进制，跳过
	mustMkdir(t, filepath.Join(dir, "node_modules"))
	writeFile(t, dir, filepath.Join("node_modules", "dep.js"), "x=1\n") // 忽略规则命中
	writeFile(t, dir, "big.txt", "0123456789")                         // 10 字节 > 上限 5 → 跳过

	cfg := localhistory.DefaultHistoryConfig()
	cfg.MaxFileSize = 5 // big.txt(10) 超限；keep.txt(3) 不超限

	_, pw, fs := newTestWatcher(t, dir, cfg)
	pw.recordPending("keep.txt", false)
	pw.recordPending("skip.bin", false)
	pw.recordPending(filepath.ToSlash(filepath.Join("node_modules", "dep.js")), false)
	pw.recordPending("big.txt", false)

	pw.flush()

	got := fs.savedPaths()
	want := []string{"keep.txt"}
	if !equalStrs(got, want) {
		t.Errorf("saved = %v, want %v (binary/ignored/oversize must be filtered)", got, want)
	}
}

// TestFlushDeletionRecorded 验证已删除（磁盘上不存在）的路径走 SaveDeletion。
func TestFlushDeletionRecorded(t *testing.T) {
	dir := t.TempDir()
	cfg := localhistory.DefaultHistoryConfig()
	_, pw, fs := newTestWatcher(t, dir, cfg)
	pw.recordPending("gone.txt", true) // 磁盘上不存在

	pw.flush()

	if len(fs.deleted) != 1 || fs.deleted[0] != "gone.txt" {
		t.Errorf("deleted = %v, want [gone.txt]", fs.deleted)
	}
}

// TestReadCurrentBranch 覆盖普通仓库 / detached / worktree 三种 .git 形态。
func TestReadCurrentBranch(t *testing.T) {
	// 普通仓库：.git 是目录，HEAD = ref: refs/heads/<branch>
	normal := t.TempDir()
	mustMkdir(t, filepath.Join(normal, ".git"))
	writeFile(t, normal, filepath.Join(".git", "HEAD"), "ref: refs/heads/main\n")
	if got := readCurrentBranch(normal); got != "main" {
		t.Errorf("normal branch = %q, want main", got)
	}

	// detached HEAD：HEAD 直接是 commit hash
	detached := t.TempDir()
	mustMkdir(t, filepath.Join(detached, ".git"))
	writeFile(t, detached, filepath.Join(".git", "HEAD"), "0123456789abcdef0123456789abcdef01234567\n")
	if got := readCurrentBranch(detached); got != "HEAD" {
		t.Errorf("detached branch = %q, want HEAD", got)
	}

	// worktree：.git 是文件，内容 gitdir: <绝对路径>
	wt := t.TempDir()
	realGitDir := filepath.Join(wt, "maingit", "worktrees", "wt1")
	mustMkdir(t, realGitDir)
	writeFile(t, realGitDir, "HEAD", "ref: refs/heads/feature-x\n")
	writeFile(t, wt, ".git", "gitdir: "+realGitDir+"\n")
	if got := readCurrentBranch(wt); got != "feature-x" {
		t.Errorf("worktree branch = %q, want feature-x", got)
	}

	// 非 git 仓：返回空串
	if got := readCurrentBranch(t.TempDir()); got != "" {
		t.Errorf("non-git branch = %q, want empty", got)
	}
}

// —— 测试辅助 ——

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	mustMkdir(t, filepath.Dir(full))
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
