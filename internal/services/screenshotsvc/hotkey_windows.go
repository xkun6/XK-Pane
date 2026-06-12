//go:build windows

package screenshotsvc

// hotkey_windows.go — 系统级全局热键：RegisterHotKey + 专用 OS 线程消息循环。
// Wails v3 的 KeyBinding 是应用内快捷键（挂各窗口 WndProc），应用失焦不触发；
// 截图热键要求"在任何应用里都能按"，必须走 RegisterHotKey（对齐 cc-pane 的
// tauri-plugin-global-shortcut，其底层同为 RegisterHotKey）。
//
// 改键：另一 goroutine 经 PostThreadMessageW(wmAppReregister) 投递到本线程，
// 在消息循环内完成 Unregister + Register（RegisterHotKey 要求与注册线程一致）。

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type hotkeyLoop struct {
	threadID  uint32
	onTrigger func()

	mu          sync.Mutex
	pendingMods uint32
	pendingVK   uint32
	resCh       chan error
}

// startHotkeyLoop 在专用线程注册热键并进入消息循环。注册失败（键位被其它应用占用）
// 返回 error，线程退出。
func startHotkeyLoop(mods, vk uint32, onTrigger func()) (*hotkeyLoop, error) {
	h := &hotkeyLoop{onTrigger: onTrigger, resCh: make(chan error, 1)}
	ready := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		// 先建消息队列（PostThreadMessageW 要求目标线程已有队列）。
		var m msgStruct
		procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, wmApp, wmApp, pmNoRemove)
		h.threadID = windows.GetCurrentThreadId()

		if r, _, _ := procRegisterHotKey.Call(0, hotkeyID, uintptr(mods|modNoRepeat), uintptr(vk)); r == 0 {
			ready <- fmt.Errorf("注册全局热键失败（键位可能已被其它应用占用，GetLastError=%d）", lastErr())
			return
		}
		ready <- nil

		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // 0 = WM_QUIT，-1 = 错误
				break
			}
			switch m.message {
			case wmHotkey:
				h.onTrigger()
			case wmAppReregister:
				h.mu.Lock()
				newMods, newVK := h.pendingMods, h.pendingVK
				h.mu.Unlock()
				// 对齐 cc-pane screenshot_update_shortcut：先注销旧键（忽略错误）再注册新键；
				// 新键冲突时返回 error，旧键已注销（热键暂时空挡，用户改键重试即可恢复）。
				procUnregisterHotKey.Call(0, hotkeyID)
				var err error
				if r, _, _ := procRegisterHotKey.Call(0, hotkeyID, uintptr(newMods|modNoRepeat), uintptr(newVK)); r == 0 {
					err = fmt.Errorf("注册全局热键失败（键位可能已被其它应用占用，GetLastError=%d）", lastErr())
				}
				h.resCh <- err
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		procUnregisterHotKey.Call(0, hotkeyID)
	}()

	if err := <-ready; err != nil {
		return nil, err
	}
	return h, nil
}

// reregister 投递改键请求到热键线程并等待结果。调用方（Service.mu）已串行化。
func (h *hotkeyLoop) reregister(mods, vk uint32) error {
	h.mu.Lock()
	h.pendingMods, h.pendingVK = mods, vk
	h.mu.Unlock()
	// 清掉上一轮超时后滞留的旧结果（resCh 缓冲 1），否则本轮会立刻读到错位的旧成败。
	select {
	case <-h.resCh:
	default:
	}
	if r, _, _ := procPostThreadMessage.Call(uintptr(h.threadID), wmAppReregister, 0, 0); r == 0 {
		return fmt.Errorf("热键线程不可达（GetLastError=%d）", lastErr())
	}
	select {
	case err := <-h.resCh:
		return err
	case <-time.After(3 * time.Second):
		return fmt.Errorf("改键超时：热键线程无响应")
	}
}

// stop 退出消息循环（循环退出前注销热键）。
func (h *hotkeyLoop) stop() {
	procPostThreadMessage.Call(uintptr(h.threadID), wmQuit, 0, 0)
}
