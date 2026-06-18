// Package filewatch 是「文件浏览器」的通用磁盘变更监听器：监控当前打开的根目录，
// 把外部对文件/目录的增删改（agent 写文件、其它程序改动等）去抖后广播给前端，
// 让文件树自动刷新受影响目录。
//
// 与 lhwatcher 的区别：lhwatcher 只服务「已启用本地历史」的项目、其管线是存版本；
// 本服务是无状态的「目录变更通知」，对任意打开的根目录生效，只发 FileChanged 事件。
// 复用同款思路（fsnotify 非递归 → walk 一次 + 新建目录补 Add；bep/debounce 合并窗口；
// 跳过 node_modules/.git 等噪声目录），但实现独立、不耦合历史存储。
//
// 单活动根：同一时刻只监控一个根（= 前端当前打开的 rootDir）。切根即停旧起新。
package filewatch

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"

	"xkpane/internal/events"
	"xkpane/internal/utils/logx"
)

// debounceWindow 合并窗口内的多次变更（略大于 lhwatcher 的 500ms 无所谓，这里只为刷新树）。
const debounceWindow = 400 * time.Millisecond

// ignoredDirs 是 watch-add 阶段跳过的高噪声目录名（不监控其子树，避免事件风暴）。
var ignoredDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	"target": true, ".xkpanes": true, "vendor": true, "bin": true,
	".task": true, ".next": true, ".cache": true, "out": true,
	"coverage": true, ".idea": true, ".gradle": true,
}

// Service 是 Wails service：前端经 Watch/Unwatch 控制监听哪个根目录。
type Service struct {
	mu  sync.Mutex
	cur *rootWatcher

	emitMu sync.RWMutex
	emit   func(name string, payload any)
	log    *slog.Logger
}

func New() *Service {
	return &Service{log: logx.For("filewatch")}
}

// SetEmitter 注入事件发射器（app 层传 app.Event.Emit），让 watcher 能向前端 Emit file:changed。
// 仿 termSvc/mcpeSvc 的 SetEmitter 模式（注册服务不用 AttachApp，避免把 *App 泄进 bindings）。
func (s *Service) SetEmitter(emit func(name string, payload any)) {
	s.emitMu.Lock()
	s.emit = emit
	s.emitMu.Unlock()
}

// Watch 开始监听 root（替换上一个活动监听）。root 为空 = 仅停止当前监听。
// 前端在 rootDir 变化时调用；监听失败（如目录不存在）返回错误，前端可忽略（仅失去自动刷新）。
func (s *Service) Watch(_ context.Context, root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur != nil {
		s.cur.stop()
		s.cur = nil
	}
	if root == "" {
		return nil
	}
	rw, err := newRootWatcher(s, root)
	if err != nil {
		s.log.Warn("start watch failed", "root", root, "err", err)
		return err
	}
	s.cur = rw
	return nil
}

// Unwatch 停止当前监听（前端文件页卸载时调）。
func (s *Service) Unwatch(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur != nil {
		s.cur.stop()
		s.cur = nil
	}
}

func (s *Service) emitChanged(payload events.FileChangedPayload) {
	s.emitMu.RLock()
	fn := s.emit
	s.emitMu.RUnlock()
	if fn != nil {
		fn(events.FileChanged, payload)
	}
}

// rootWatcher 监控单个根目录：一个 fsnotify watcher + debounce + 待刷新目录集。
type rootWatcher struct {
	svc  *Service
	root string
	fsw  *fsnotify.Watcher

	debounced func(func())
	mu        sync.Mutex
	pending   map[string]struct{} // 待广播刷新的目录绝对路径集（= 变更项的父目录）

	stopCh chan struct{}
	closed bool
}

func newRootWatcher(svc *Service, root string) (*rootWatcher, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	rw := &rootWatcher{
		svc:       svc,
		root:      root,
		fsw:       fsw,
		debounced: debounce.New(debounceWindow),
		pending:   make(map[string]struct{}),
		stopCh:    make(chan struct{}),
	}
	rw.addTree(root)
	go rw.loop()
	return rw, nil
}

func (rw *rootWatcher) stop() {
	if rw.closed {
		return
	}
	rw.closed = true
	close(rw.stopCh)
	if rw.fsw != nil {
		_ = rw.fsw.Close()
	}
}

// addTree 递归把 root 下目录加入 fsnotify 监控（fsnotify 非递归），跳过 ignoredDirs 子树。
func (rw *rootWatcher) addTree(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 无法访问的项跳过，不中断整棵树
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && ignoredDirs[d.Name()] {
			return filepath.SkipDir
		}
		if aerr := rw.fsw.Add(path); aerr != nil {
			rw.svc.log.Warn("watch add dir failed", "path", path, "err", aerr)
		}
		return nil
	})
}

func (rw *rootWatcher) loop() {
	for {
		select {
		case <-rw.stopCh:
			return
		case ev, ok := <-rw.fsw.Events:
			if !ok {
				return
			}
			rw.handleEvent(ev)
		case err, ok := <-rw.fsw.Errors:
			if !ok {
				return
			}
			rw.svc.log.Warn("fsnotify error", "root", rw.root, "err", err)
		}
	}
}

// handleEvent 把一个事件归并为「其父目录需刷新」，并触发 debounce。
func (rw *rootWatcher) handleEvent(ev fsnotify.Event) {
	// 新建目录：补加监控（fsnotify 非递归），并记父目录刷新（新子目录已出现在列表里）。
	if ev.Op&fsnotify.Create != 0 {
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			if !ignoredDirs[filepath.Base(ev.Name)] {
				rw.addTree(ev.Name)
			}
		}
	}
	if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return // Chmod 等忽略
	}
	dir := filepath.Dir(ev.Name)
	rw.mu.Lock()
	rw.pending[dir] = struct{}{}
	rw.mu.Unlock()
	rw.debounced(rw.flush)
}

// flush 取走待刷新目录集并广播一次 FileChanged。
func (rw *rootWatcher) flush() {
	rw.mu.Lock()
	dirs := make([]string, 0, len(rw.pending))
	for d := range rw.pending {
		dirs = append(dirs, d)
	}
	rw.pending = make(map[string]struct{})
	rw.mu.Unlock()
	if len(dirs) == 0 {
		return
	}
	rw.svc.emitChanged(events.FileChangedPayload{Root: rw.root, Dirs: dirs})
}
