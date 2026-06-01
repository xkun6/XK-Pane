package mcpe

// safety.go — 用户输入路径与字符串的安全校验。忠实移植 safety.rs。
//
// 防御重点：
//  1. 路径遍历：拒绝含 `..` / 路径分隔符的目录名，防止逃出预期根目录
//     （如 worldsDir/<folderName> 若 folderName = "../../"）。
//  2. 命令注入：用户路径会被拼到 PowerShell / cmd 命令行（symlink 提权 .bat），拒绝任何 shell 元字符。

import (
	"fmt"
	"strings"
	"unicode"
)

// forbiddenCmdChars 会破坏引号边界或注入 cmd 的字符（与 safety.rs 的 FORBIDDEN 逐字一致）。
var forbiddenCmdChars = []rune{'"', '\r', '\n', '<', '>', '|', '&', '^', '%', '`', '$', '!'}

// validateRelativeName 校验一个“目录名”（不允许任何路径分隔符 / 上级目录）。
// 用于 world.folderName：它会被 join 到 worldsDir 下，绝不允许逃出。
func validateRelativeName(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("名称不能包含路径分隔符：%s", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("名称非法：%s", name)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("名称含 NUL 字符")
	}
	for _, c := range name {
		if unicode.IsControl(c) {
			return fmt.Errorf("名称含控制字符")
		}
	}
	return nil
}

// validateCommandLinePath 校验路径可安全拼到 .bat / PowerShell 命令行（双引号包裹）。
// 拒绝 forbiddenCmdChars 中任一字符及任何控制字符。常规 Windows 路径不会含这些字符。
func validateCommandLinePath(path string) error {
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}
	for _, c := range path {
		if unicode.IsControl(c) || runeIn(c, forbiddenCmdChars) {
			return fmt.Errorf("路径含禁用字符 %q — 出于命令注入防御不允许：%s", c, path)
		}
	}
	return nil
}

func runeIn(c rune, set []rune) bool {
	for _, r := range set {
		if r == c {
			return true
		}
	}
	return false
}
