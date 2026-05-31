// Package lhwatcher 是 Local History 的自动捕获 file-watcher（P4a，灵魂）。
//
// 忠实移植 cc-pane cc-panes-core/src/services/history_service.rs:408-549 的 watcher 管线
// （notify crate → 本仓用 github.com/fsnotify/fsnotify）：
//   - 每项目一个 fsnotify watcher，递归监控项目根（fsnotify 非递归，故 walk 一次 + 对新建目录补 Add）；
//   - trailing-edge 500ms debounce 合并窗口内同文件的多次变更（github.com/bep/debounce）；
//   - 分支感知：每次事件读 .git/HEAD（支持 worktree gitdir: 格式），切换时打 branch_switch 标签
//     并设 3s 静默窗口抑制 git checkout 噪声（见 watcher_pipeline.go，P4a-2）；
//   - 过滤：config.enabled / ignore 规则 / 符号链接越界 / 大小上限 / 二进制（见 P4a-2）；
//   - 单写者：所有保存经 localhistorysvc 的 per-project 写锁串行（与 gitsvc AutoLabel 共用，见 P4a-2）。
//
// 落点决策 D4-1：独立包，不与 localhistorysvc 混居（watcher 是「捕获 + 调度」，localhistorysvc 是
// 「存储 + 锁」）。watcher 通过窄接口 Saver 依赖 localhistorysvc（避免反向 import + 便于单测）。
package lhwatcher

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v3/pkg/application"

	"xkpane/internal/localhistory"
	"xkpane/internal/utils/logx"
)

const (
	// debounceWindow 是 trailing-edge debounce 窗口（cc-pane constants.rs DEBOUNCE_MS = 500）。
	debounceWindow = 500 * time.Millisecond
	// silenceWindow 是分支切换后的静默窗口（cc-pane CHECKOUT_SILENCE_SECS = 3），抑制 checkout 噪声。
	silenceWindow = 3 * time.Second
)

// Saver 是 watcher 对 localhistorysvc 的窄依赖：读配置（启停判定 + 过滤）+ 保存版本 / 删除快照 +
// 分支切换标签——全部经 localhistorysvc 的 per-project 写锁串行（单写者）。
//
// 由 localhistorysvc.Service 实现：GetConfig 读配置（启停判定 + 过滤参数），SaveWatched/
// SaveDeletion/AutoLabel 都经其 per-project 写锁串行（与 gitsvc AutoLabel 共用，单写者）。
type Saver interface {
	GetConfig(ctx context.Context, projectPath string) (localhistory.HistoryConfig, error)
	SaveWatched(ctx context.Context, projectPath, filePath, content, branch string, minIntervalSecs int64) (*localhistory.FileVersion, error)
	SaveDeletion(ctx context.Context, projectPath, filePath, branch string) (*localhistory.FileVersion, error)
	AutoLabel(ctx context.Context, projectPath, name, source, branch string) (string, error)
}

// Manager 管理所有项目的 watcher（启停 + 生命周期）。
type Manager struct {
	saver Saver
	log   *slog.Logger

	mu       sync.Mutex
	watchers map[string]*projectWatcher

	emitMu sync.RWMutex
	emit   func(name string, payload any)
}

// New 创建 watcher 管理器。saver 通常是 *localhistorysvc.Service。
func New(saver Saver) *Manager {
	return &Manager{
		saver:    saver,
		log:      logx.For("lhwatcher"),
		watchers: make(map[string]*projectWatcher),
	}
}

// AttachApp 回填 app 引用，让 watcher 能向前端 Emit localhistory:changed 事件（仿 termSvc 模式）。
// 不调用则前端收不到 live 刷新（保存仍正常）。
func (m *Manager) AttachApp(app *application.App) {
	m.emitMu.Lock()
	m.emit = func(name string, payload any) { app.Event.Emit(name, payload) }
	m.emitMu.Unlock()
}

// StartRegistered 在 app 启动时扫已登记项目：仅对「.xkpanes/history 已存在 且 config.enabled」的
// 启 watcher（决策 D4-4，避免 watch 海量无关目录）。
func (m *Manager) StartRegistered(ctx context.Context, projectPaths []string) {
	for _, p := range projectPaths {
		m.EnsureWatching(ctx, p)
	}
}

// EnsureWatching 确保 projectPath 在被监控（幂等）。内部守门：仅对已启用 Local History 的项目生效。
// 供启动扫描 / SaveConfig(enabled=true) / launchProject 补启共用。
func (m *Manager) EnsureWatching(ctx context.Context, projectPath string) {
	if projectPath == "" || !m.shouldWatch(ctx, projectPath) {
		return
	}
	m.startWatch(projectPath)
}

// StopWatching 停止并移除某项目的 watcher（SaveConfig(enabled=false) 时调）。
func (m *Manager) StopWatching(projectPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pw, ok := m.watchers[projectPath]; ok {
		pw.stop()
		delete(m.watchers, projectPath)
		m.log.Info("stopped watching", "project", projectPath)
	}
}

// StopAll 停止所有 watcher（app 退出时调）。
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for p, pw := range m.watchers {
		pw.stop()
		delete(m.watchers, p)
	}
}

// shouldWatch：仅当项目已有历史存储（.xkpanes/history/history.db 存在）且配置启用时才监控。
// GetConfig 对未初始化项目返回默认配置（enabled=true）——但 Exists==false 已先挡掉，故此处
// 的 enabled 检查只对「已在用但被用户关掉」的项目生效（决策 D4-4）。
func (m *Manager) shouldWatch(ctx context.Context, projectPath string) bool {
	if !localhistory.Exists(projectPath) {
		return false
	}
	cfg, err := m.saver.GetConfig(ctx, projectPath)
	if err != nil {
		m.log.Warn("read config failed, skip watching", "project", projectPath, "err", err)
		return false
	}
	return cfg.Enabled
}

// startWatch 实际创建并启动一个 projectWatcher（幂等，已在监控则跳过）。
func (m *Manager) startWatch(projectPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.watchers[projectPath]; ok {
		return
	}
	pw := m.newProjectWatcher(projectPath)
	if err := pw.start(); err != nil {
		m.log.Warn("start watcher failed", "project", projectPath, "err", err)
		return
	}
	m.watchers[projectPath] = pw
	m.log.Info("watching project", "project", projectPath)
}

// emitChanged 向前端广播一批已保存的版本变更（P4a-2 在保存成功后调）。
func (m *Manager) emitChanged(name string, payload any) {
	m.emitMu.RLock()
	fn := m.emit
	m.emitMu.RUnlock()
	if fn != nil {
		fn(name, payload)
	}
}

// projectWatcher 是单个项目的监控器：一个 fsnotify watcher + per-project debounce + 分支上下文。
type projectWatcher struct {
	mgr            *Manager
	projectPath    string
	realRoot       string   // canonicalize(projectPath)，符号链接越界保护用（withinRoot）
	ignorePatterns []string // watch-add 阶段跳过目录用；保存时过滤每 flush 再读最新配置

	fsw       *fsnotify.Watcher
	debounced func(func()) // bep/debounce：每次事件调一次，窗口静默后触发 flush（最后一次）

	stateMu        sync.Mutex
	pendingChanged map[string]struct{} // 窗口内待处理的「变更」相对路径集
	pendingDeleted map[string]struct{} // 窗口内待处理的「删除」相对路径集
	branchCache    string              // 上次已知分支（检测切换用）
	silenceUntil   time.Time           // 静默截止时间（分支切换后 3s 内的文件事件丢弃）

	stopCh chan struct{}
	closed bool
}

func (m *Manager) newProjectWatcher(projectPath string) *projectWatcher {
	patterns := localhistory.DefaultHistoryConfig().IgnorePatterns
	if cfg, err := m.saver.GetConfig(context.Background(), projectPath); err == nil {
		patterns = cfg.IgnorePatterns
	}
	realRoot := projectPath
	if rr, err := filepath.EvalSymlinks(projectPath); err == nil {
		realRoot = rr
	}
	return &projectWatcher{
		mgr:            m,
		projectPath:    projectPath,
		realRoot:       realRoot,
		ignorePatterns: patterns,
		pendingChanged: make(map[string]struct{}),
		pendingDeleted: make(map[string]struct{}),
		branchCache:    readCurrentBranch(projectPath),
		debounced:      debounce.New(debounceWindow),
		stopCh:         make(chan struct{}),
	}
}

func (pw *projectWatcher) start() error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	pw.fsw = fsw
	pw.addTree(pw.projectPath) // best-effort：部分子目录加不上不致命
	go pw.loop()
	return nil
}

func (pw *projectWatcher) stop() {
	if pw.closed {
		return
	}
	pw.closed = true
	close(pw.stopCh)
	if pw.fsw != nil {
		_ = pw.fsw.Close()
	}
}

// addTree 递归把 root 下的目录加入 fsnotify 监控（fsnotify 本身非递归）。跳过 ignore 命中的目录
// （node_modules/dist/build/target/.xkpanes 等），但顶层 .git 例外：加它（捕获 HEAD 变更做分支
// 检测）却不递归其子目录（objects/refs 噪声极大）。
func (pw *projectWatcher) addTree(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 无法访问的项跳过，不中断整棵树
		}
		if !d.IsDir() {
			return nil
		}
		if path != root {
			if rel, rerr := filepath.Rel(pw.projectPath, path); rerr == nil {
				if d.Name() == ".git" {
					_ = pw.fsw.Add(path) // 监控 .git 顶层（HEAD），不递归
					return filepath.SkipDir
				}
				if ShouldIgnore(rel, pw.ignorePatterns) {
					return filepath.SkipDir
				}
			}
		}
		if aerr := pw.fsw.Add(path); aerr != nil {
			pw.mgr.log.Warn("watch add dir failed", "path", path, "err", aerr)
		}
		return nil
	})
}

func (pw *projectWatcher) loop() {
	for {
		select {
		case <-pw.stopCh:
			return
		case ev, ok := <-pw.fsw.Events:
			if !ok {
				return
			}
			pw.handleEvent(ev)
		case err, ok := <-pw.fsw.Errors:
			if !ok {
				return
			}
			pw.mgr.log.Warn("fsnotify error", "project", pw.projectPath, "err", err)
		}
	}
}

// handleEvent 把一个 fsnotify 事件映射成「变更 / 删除」记入 pending 集，并触发 debounce。
// 分支切换检测与过滤 / 保存在 flush（debounce 后）统一做。
func (pw *projectWatcher) handleEvent(ev fsnotify.Event) {
	// 分支切换检测：每个事件读 .git/HEAD，切换则打 branch_switch 标签 + 设 3s 静默（见 pipeline.go）。
	// 放在记入 pending 之前——这样紧随 checkout 的文件事件会被静默窗口在 flush 时丢弃。
	pw.checkBranchSwitch()

	rel, ok := pw.relPath(ev.Name)
	if !ok {
		return
	}
	// 新建目录：补加监控（fsnotify 非递归，子目录需显式 Add）。
	if ev.Op&fsnotify.Create != 0 {
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			pw.addTree(ev.Name)
			return
		}
	}
	switch {
	case ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
		pw.recordPending(rel, true)
	case ev.Op&(fsnotify.Create|fsnotify.Write) != 0:
		pw.recordPending(rel, false)
	default:
		return // Chmod 等其它事件忽略
	}
	pw.debounced(pw.flush)
}

// recordPending 把一次「变更 / 删除」记入 debounce 窗口的 pending 集（同路径后到覆盖先到：
// 变更后删除→只剩删除；删除后又变更→只剩变更）。
func (pw *projectWatcher) recordPending(rel string, isDelete bool) {
	pw.stateMu.Lock()
	defer pw.stateMu.Unlock()
	if isDelete {
		delete(pw.pendingChanged, rel)
		pw.pendingDeleted[rel] = struct{}{}
	} else {
		delete(pw.pendingDeleted, rel)
		pw.pendingChanged[rel] = struct{}{}
	}
}

// relPath 把 fsnotify 的绝对路径转项目内相对路径（正斜杠）。越界 / 无关返回 false。
func (pw *projectWatcher) relPath(name string) (string, bool) {
	rel, err := filepath.Rel(pw.projectPath, name)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

// drainPending 取走并清空当前 pending 集（在 stateMu 下）。
func (pw *projectWatcher) drainPending() (changed, deleted []string) {
	pw.stateMu.Lock()
	defer pw.stateMu.Unlock()
	changed = mapKeys(pw.pendingChanged)
	deleted = mapKeys(pw.pendingDeleted)
	pw.pendingChanged = make(map[string]struct{})
	pw.pendingDeleted = make(map[string]struct{})
	return changed, deleted
}

// readCurrentBranch 直接读 .git/HEAD 解析当前分支（~0.01ms，忠实 cc-pane read_current_branch）。
// 支持普通仓库（.git 是目录）与 worktree（.git 是文件，内容 "gitdir: <path>"）两种格式。
// 返回 "" 表示非 git 仓 / 读取失败；detached HEAD 返回 "HEAD"。
func readCurrentBranch(projectPath string) string {
	gitPath := filepath.Join(projectPath, ".git")
	fi, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	var headPath string
	if fi.IsDir() {
		headPath = filepath.Join(gitPath, "HEAD")
	} else {
		// worktree：.git 是文件，内容 = "gitdir: /abs/or/rel/.git/worktrees/<name>"
		data, rerr := os.ReadFile(gitPath)
		if rerr != nil {
			return ""
		}
		gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
		if gitdir == "" {
			return ""
		}
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(projectPath, gitdir)
		}
		headPath = filepath.Join(gitdir, "HEAD")
	}
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(data))
	const refPrefix = "ref: refs/heads/"
	if strings.HasPrefix(s, refPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(s, refPrefix))
	}
	return "HEAD" // detached HEAD（HEAD 直接是 commit hash）
}

func mapKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
