//go:build windows

package screenshotsvc

// trigger_windows.go — 截图全流程编排（对齐 cc-pane trigger_screenshot 时序）：
// CAS 防重入 → 主窗口排除捕获 → 截光标所在显示器 → 选区遮罩（阻塞）→ 裁剪 →
// 存 PNG → 路径写剪贴板 → Emit screenshot:saved → 恢复捕获。

import (
	"image"
	"runtime"

	"xkpane/internal/events"
)

func (s *Service) platformStart(shortcut string) error {
	mods, vk, err := parseShortcut(shortcut)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hk, err := startHotkeyLoop(mods, vk, s.trigger)
	if err != nil {
		return err
	}
	s.hk = hk
	logFor().Info("截图全局热键已注册", "shortcut", shortcut)
	return nil
}

func (s *Service) platformStop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hk != nil {
		s.hk.stop()
		s.hk = nil
	}
}

func (s *Service) platformReregister(mods, vk uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 启动期注册失败（键位冲突）时 hk 为 nil：改键即重建热键线程。
	// nil 检查与重建在同一临界区内，防止并发改键双开热键线程泄漏旧热键。
	if s.hk == nil {
		hk, err := startHotkeyLoop(mods, vk, s.trigger)
		if err != nil {
			return err
		}
		s.hk = hk
		return nil
	}
	return s.hk.reregister(mods, vk)
}

// trigger 在热键线程被调用：守门后整个截图流程放到独立 goroutine
// （遮罩消息循环需独占 OS 线程，不能占住热键线程）。
func (s *Service) trigger() {
	if !s.capturing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.capturing.Store(false)

		// 主窗口排除捕获（SetContentProtection 内部 InvokeSync，任意线程可调；
		// 底层即 cc-pane 的 SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)）。
		if s.protect != nil {
			s.protect(true)
			defer s.protect(false)
		}

		// 截屏 + 遮罩共线程：物理像素坐标依赖 Per-Monitor V2 DPI 上下文。
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		prev := setThreadDpi(dpiAwarenessPerMonitorV2)
		defer restoreThreadDpi(prev)

		img, mon, err := captureCursorMonitor()
		if err != nil {
			logFor().Error("截屏失败", "err", err)
			return
		}

		sel, ok := showSelectionOverlay(img, mon)
		if !ok {
			logFor().Debug("用户取消选区")
			return
		}

		cropped := img.SubImage(sel).(*image.RGBA)
		path, err := s.saveScreenshot(cropped)
		if err != nil {
			logFor().Error("截图保存失败", "err", err)
			return
		}
		logFor().Info("截图已保存", "path", path, "size", sel.String())

		// 剪贴板放图片文件本身（CF_HDROP，同 Explorer「复制文件」），不放路径字符串——
		// 粘贴到聊天工具/资源管理器拿到的是图。
		if err := copyFileToClipboard(path); err != nil {
			logFor().Warn("截图文件写剪贴板失败", "err", err)
		}
		if s.emit != nil {
			s.emit(events.ScreenshotSaved, events.ScreenshotSavedPayload{
				FilePath: path,
				Width:    sel.Dx(),
				Height:   sel.Dy(),
			})
		}
	}()
}
