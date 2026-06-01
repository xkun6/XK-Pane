//go:build windows

package mcpe

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// scanLaunchers 扫描所有磁盘根，找
// <drive>:\MCStudioDownload\game\MinecraftPE_Netease\<version>\Minecraft.Windows.exe。
// 对齐 paths.rs 的 #[cfg(windows)] scan_launchers（GetLogicalDrives 位掩码遍历 A-Z）。
func scanLaunchers() []string {
	var results []string
	mask, err := windows.GetLogicalDrives()
	if err != nil || mask == 0 {
		return results
	}
	for i := uint(0); i < 26; i++ {
		if (mask>>i)&1 == 0 {
			continue
		}
		letter := string(rune('A' + i))
		base := fmt.Sprintf("%s:\\MCStudioDownload\\game\\MinecraftPE_Netease", letter)
		if !isDir(base) {
			continue
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			version := filepath.Join(base, e.Name())
			if !isDir(version) {
				continue
			}
			exe := filepath.Join(version, "Minecraft.Windows.exe")
			if isFile(exe) {
				results = append(results, exe)
			}
		}
	}
	return results
}
