package lhwatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/localhistory"
)

// checkBranchSwitch 读取当前分支并与缓存比较：切换时打 branch_switch auto 标签 + 设 3s 静默窗口
// （抑制 git checkout 触发的批量文件事件）。忠实 cc-pane history_service.rs:479-504 + 242-266。
// 每个 fsnotify 事件都会调用本函数（含 .git/HEAD 变更事件）——读 HEAD ~0.01ms，可接受。
func (pw *projectWatcher) checkBranchSwitch() {
	branch := readCurrentBranch(pw.projectPath)
	if branch == "" {
		return // 非 git 仓 / 读取失败：无分支概念
	}
	pw.stateMu.Lock()
	old := pw.branchCache
	if old == "" {
		pw.branchCache = branch // 首次确立，不算切换
		pw.stateMu.Unlock()
		return
	}
	if branch == old {
		pw.stateMu.Unlock()
		return
	}
	pw.branchCache = branch
	pw.silenceUntil = time.Now().Add(silenceWindow)
	pw.stateMu.Unlock()

	// 打 branch_switch 标签（best-effort）。异步执行避免阻塞 fsnotify 事件循环；写锁由 localhistorysvc
	// 内部保证与其它写者串行。
	name := fmt.Sprintf("Branch Switch: %s → %s", old, branch)
	go func() {
		if _, err := pw.mgr.saver.AutoLabel(context.Background(), pw.projectPath, name, "branch_switch", branch); err != nil {
			pw.mgr.log.Warn("branch_switch auto label failed", "project", pw.projectPath, "err", err)
		}
	}()
}

// flush 在 debounce 窗口静默后触发（bep/debounce 尾边缘回调，运行于其内部 goroutine）。
// 处理本窗口积累的变更 / 删除：分支静默丢弃 → 读配置 → 逐文件过滤并保存 → Emit live 刷新事件。
func (pw *projectWatcher) flush() {
	pw.stateMu.Lock()
	silenced := time.Now().Before(pw.silenceUntil)
	pw.stateMu.Unlock()

	changed, deleted := pw.drainPending()
	if silenced {
		if len(changed)+len(deleted) > 0 {
			pw.mgr.log.Debug("drop events in checkout silence window",
				"project", pw.projectPath, "changed", len(changed), "deleted", len(deleted))
		}
		return
	}
	if len(changed) == 0 && len(deleted) == 0 {
		return
	}

	ctx := context.Background()
	cfg, err := pw.mgr.saver.GetConfig(ctx, pw.projectPath)
	if err != nil {
		pw.mgr.log.Warn("read config failed in flush", "project", pw.projectPath, "err", err)
		return
	}
	if !cfg.Enabled {
		return // 运行中被关掉：本批不存（watcher 也会被 SaveConfig 钩子停掉）
	}
	branch := readCurrentBranch(pw.projectPath)

	saved := make([]string, 0, len(changed)+len(deleted))
	for _, rel := range changed {
		if pw.saveChanged(ctx, cfg, rel, branch) {
			saved = append(saved, rel)
		}
	}
	for _, rel := range deleted {
		if pw.saveDeleted(ctx, cfg, rel, branch) {
			saved = append(saved, rel)
		}
	}
	if len(saved) > 0 {
		pw.mgr.emitChanged(events.LocalHistoryChanged, events.LocalHistoryChangedPayload{
			ProjectPath:  pw.projectPath,
			FilePaths:    saved,
			VersionCount: len(saved),
			Timestamp:    time.Now().UnixMilli(),
		})
	}
}

// saveChanged 过滤并保存一个「变更」文件，返回是否真的存了新版本（去重 / 节流跳过返回 false）。
// 过滤顺序忠实 cc-pane process_file_changed（268-331）：ignore → 目录 → 符号链接越界 → 大小 → 二进制。
func (pw *projectWatcher) saveChanged(ctx context.Context, cfg localhistory.HistoryConfig, rel, branch string) bool {
	if ShouldIgnore(rel, cfg.IgnorePatterns) {
		return false
	}
	full := filepath.Join(pw.projectPath, filepath.FromSlash(rel))
	if fi, err := os.Lstat(full); err != nil || fi.IsDir() {
		return false // 已不存在（交给 deleted 处理）或是目录
	}
	// 符号链接越界保护：解析真实路径，必须仍落在项目根内（忠实 cc-pane canonicalize + starts_with）。
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false
	}
	if !pw.withinRoot(real) {
		return false
	}
	rfi, err := os.Stat(real)
	if err != nil || rfi.IsDir() {
		return false
	}
	if cfg.MaxFileSize > 0 && rfi.Size() > cfg.MaxFileSize {
		return false
	}
	content, err := os.ReadFile(real)
	if err != nil {
		return false
	}
	if localhistory.IsBinary(content) {
		return false
	}
	v, err := pw.mgr.saver.SaveWatched(ctx, pw.projectPath, rel, string(content), branch, cfg.MinSaveIntervalSecs)
	if err != nil {
		pw.mgr.log.Warn("save watched version failed", "project", pw.projectPath, "file", rel, "err", err)
		return false
	}
	return v != nil // nil = 内容未变（去重）或节流未到，未产生新版本
}

// saveDeleted 为一个「删除」文件存删除快照（is_deleted=true，保留删前内容），返回是否真的记了。
// 若该路径又存在（重命名 / 重建），视为非删除跳过。
func (pw *projectWatcher) saveDeleted(ctx context.Context, cfg localhistory.HistoryConfig, rel, branch string) bool {
	if ShouldIgnore(rel, cfg.IgnorePatterns) {
		return false
	}
	full := filepath.Join(pw.projectPath, filepath.FromSlash(rel))
	if _, err := os.Lstat(full); err == nil {
		return false // 又存在了 → 不是真删除
	}
	v, err := pw.mgr.saver.SaveDeletion(ctx, pw.projectPath, rel, branch)
	if err != nil {
		pw.mgr.log.Warn("save deletion failed", "project", pw.projectPath, "file", rel, "err", err)
		return false
	}
	return v != nil // nil = 从未跟踪 / 已记过删除
}

// withinRoot 判断 canonicalize 后的 target 是否仍在项目根内（符号链接越界保护）。
func (pw *projectWatcher) withinRoot(target string) bool {
	rel, err := filepath.Rel(pw.realRoot, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
