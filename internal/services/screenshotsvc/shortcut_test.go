package screenshotsvc

import "testing"

func TestParseShortcut(t *testing.T) {
	cases := []struct {
		in      string
		mods    uint32
		vk      uint32
		wantErr bool
	}{
		{"Ctrl+Alt+S", modControl | modAlt, 'S', false},
		{"ctrl+shift+s", modControl | modShift, 'S', false},
		{"Control+Shift+F12", modControl | modShift, 0x70 + 11, false},
		{"Win+9", modWin, '9', false},
		{" Ctrl + Alt + A ", modControl | modAlt, 'A', false},
		{"S", 0, 0, true},        // 无修饰键
		{"Ctrl+Alt", 0, 0, true}, // 无主键
		{"Ctrl+S+A", 0, 0, true}, // 两个主键
		{"Ctrl+Esc", 0, 0, true}, // 不支持的键
		{"Ctrl++S", 0, 0, true},  // 空 token
		{"", 0, 0, true},         // 空串
		{"Ctrl+F25", 0, 0, true}, // F 键越界
	}
	for _, c := range cases {
		mods, vk, err := parseShortcut(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseShortcut(%q) 应失败，实际 mods=%#x vk=%#x", c.in, mods, vk)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseShortcut(%q) 意外失败: %v", c.in, err)
			continue
		}
		if mods != c.mods || vk != c.vk {
			t.Errorf("parseShortcut(%q) = (%#x, %#x)，期望 (%#x, %#x)", c.in, mods, vk, c.mods, c.vk)
		}
	}
}
