package screenshotsvc

// shortcut.go — "Ctrl+Alt+S" 风格快捷键字符串 → Win32 RegisterHotKey 参数。
// 纯函数、跨平台编译（常量为 Win32 MOD_* / VK 值，仅 Windows 路径消费）。

import (
	"fmt"
	"strings"
)

// Win32 RegisterHotKey 修饰键（winuser.h MOD_*）。
const (
	modAlt      uint32 = 0x0001
	modControl  uint32 = 0x0002
	modShift    uint32 = 0x0004
	modWin      uint32 = 0x0008
	modNoRepeat uint32 = 0x4000 // 按住不连发（对齐 cc-pane 仅 Pressed 触发）
)

// parseShortcut 解析 "Ctrl+Alt+S" → (修饰键位掩码, 虚拟键码)。
// 规则：至少 1 个修饰键 + 恰好 1 个主键；主键支持 A-Z / 0-9 / F1-F24。
func parseShortcut(s string) (mods uint32, vk uint32, err error) {
	parts := strings.Split(s, "+")
	for _, raw := range parts {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return 0, 0, fmt.Errorf("快捷键格式无效: %q", s)
		}
		switch strings.ToLower(tok) {
		case "ctrl", "control":
			mods |= modControl
		case "alt":
			mods |= modAlt
		case "shift":
			mods |= modShift
		case "win", "super", "meta", "cmd":
			mods |= modWin
		default:
			if vk != 0 {
				return 0, 0, fmt.Errorf("快捷键只能有一个主键: %q", s)
			}
			vk, err = parseKey(tok)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	if mods == 0 {
		return 0, 0, fmt.Errorf("快捷键至少需要一个修饰键（Ctrl/Alt/Shift/Win）: %q", s)
	}
	if vk == 0 {
		return 0, 0, fmt.Errorf("快捷键缺少主键: %q", s)
	}
	return mods, vk, nil
}

// parseKey 单个主键 → 虚拟键码。A-Z/0-9 的 VK 值即其大写 ASCII；F1-F24 为 0x70 起。
func parseKey(tok string) (uint32, error) {
	up := strings.ToUpper(tok)
	if len(up) == 1 {
		c := up[0]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return uint32(c), nil
		}
	}
	if strings.HasPrefix(up, "F") && len(up) >= 2 && len(up) <= 3 {
		var n int
		if _, err := fmt.Sscanf(up[1:], "%d", &n); err == nil && n >= 1 && n <= 24 {
			return uint32(0x70 + n - 1), nil
		}
	}
	return 0, fmt.Errorf("不支持的按键: %q（支持 A-Z / 0-9 / F1-F24）", tok)
}
