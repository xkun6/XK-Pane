//go:build windows

package mcdkbridge

// click_windows.go — 模拟左键点击：归一化坐标 → 客户区像素 → 屏幕坐标 → 虚拟桌面 0..65535
// → SendInput[MOVE,LEFTDOWN,LEFTUP]。忠实移植 cc-pane click.rs。
//
// SendInput 仅对前台窗口生效，故先 AttachThreadInput + SetForegroundWindow 把目标窗口拉前台；
// 客户区物理像素需在 PER_MONITOR_AWARE_V2 DPI 上下文内取，否则高 DPI 下坐标偏移。

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ClickWindowByPID 在 pid 进程的 MC 渲染窗口客户区模拟一次左键点击。
// nx/ny 归一化：(0,0)=客户区左上，(1,1)=右下。
func ClickWindowByPID(pid uint32, nx, ny float32) error {
	if !inUnitRange(nx) || !inUnitRange(ny) {
		return coded(CodeClickOutOfBounds, fmt.Sprintf("click coords out of [0,1]: nx=%v, ny=%v", nx, ny))
	}

	hwnd, ok := findMCWindowForPID(pid)
	if !ok {
		return coded(CodeClickNoWindow, fmt.Sprintf("no Minecraft-like top-level window for pid=%d", pid))
	}

	// 还原最小化窗口，否则 GetClientRect 拿到 0×0。
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
		time.Sleep(50 * time.Millisecond)
	}

	bringToForeground(hwnd)

	// 仅 compute 在 PER_MONITOR_AWARE_V2 DPI 上下文内（对齐 cc-pane with_per_monitor_dpi）。
	prev := setThreadDpi(dpiAwarenessPerMonitorV2)
	pt, err := computeScreenPoint(hwnd, nx, ny)
	restoreThreadDpi(prev)
	if err != nil {
		return err
	}

	// 虚拟桌面归一化（覆盖多显示器）。
	vx, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	vy, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	vw, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	vh, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	vwI, vhI := int32(vw), int32(vh)
	if vwI <= 0 || vhI <= 0 {
		return coded(CodeClickFailed, fmt.Sprintf("invalid virtual screen size: %dx%d", vwI, vhI))
	}
	ax := normalizeVirtualAxis(pt.x, int32(vx), vwI)
	ay := normalizeVirtualAxis(pt.y, int32(vy), vhI)
	return sendLeftClick(ax, ay)
}

// bringToForeground 把窗口强行拉前台（SendInput 只在前台窗口生效）。忠实 cc-pane click.rs:83。
func bringToForeground(hwnd uintptr) {
	cur := windows.GetCurrentThreadId()
	fgWin, _, _ := procGetForegroundWindow.Call()
	fg, _, _ := procGetWindowThreadProcessId.Call(fgWin, 0) // 第二参 None → 返回值即线程 id

	attached := false
	if uint32(fg) != 0 && uint32(fg) != cur {
		if r, _, _ := procAttachThreadInput.Call(fg, uintptr(cur), 1); r != 0 {
			attached = true
		}
	}

	procSetForegroundWindow.Call(hwnd)
	procBringWindowToTop.Call(hwnd)

	if attached {
		procAttachThreadInput.Call(fg, uintptr(cur), 0)
	}

	// 等前台切换实际生效（最多 250ms，与 cc-pane 节奏一致）。
	for i := 0; i < 25; i++ {
		if fgNow, _, _ := procGetForegroundWindow.Call(); fgNow == hwnd {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// computeScreenPoint 客户区归一化坐标 → 屏幕物理像素坐标。
func computeScreenPoint(hwnd uintptr, nx, ny float32) (point, error) {
	var cr rect
	if r, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr))); r == 0 {
		return point{}, coded(CodeClickFailed, fmt.Sprintf("GetClientRect failed (GetLastError=%d)", lastErr()))
	}
	// GetClientRect 的 left/top 恒为 0，故 right=宽、bottom=高（对齐 cc-pane 直接取 right/bottom）。
	if cr.right <= 0 || cr.bottom <= 0 {
		return point{}, coded(CodeClickFailed, fmt.Sprintf("empty client rect: %dx%d", cr.right, cr.bottom))
	}
	pt := point{x: clientToCoord(nx, cr.right), y: clientToCoord(ny, cr.bottom)}
	if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt))); r == 0 {
		return point{}, coded(CodeClickFailed, fmt.Sprintf("ClientToScreen failed (GetLastError=%d)", lastErr()))
	}
	return pt, nil
}

// sendLeftClick 三段事件 MOVE → LEFTDOWN → LEFTUP，全部 ABSOLUTE + VIRTUALDESK，无段间延时。
func sendLeftClick(ax, ay int32) error {
	flagsBase := uint32(mouseeventfAbsolute | mouseeventfVirtualDesk)
	mk := func(flags uint32) inputStruct {
		return inputStruct{
			inputType: inputMouse,
			mi:        mouseInput{dx: ax, dy: ay, dwFlags: flagsBase | flags},
		}
	}
	inputs := [3]inputStruct{mk(mouseeventfMove), mk(mouseeventfLeftDown), mk(mouseeventfLeftUp)}

	sent, _, _ := procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if int(sent) != len(inputs) {
		return coded(CodeClickFailed, fmt.Sprintf("SendInput sent %d/%d (GetLastError=%d)", sent, len(inputs), lastErr()))
	}
	return nil
}
