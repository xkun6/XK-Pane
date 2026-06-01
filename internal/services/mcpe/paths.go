package mcpe

// paths.go — 网易 MC 中国版相关路径解析。忠实移植 paths.rs。
//
// 网易 MC 安装结构：
//   - %APPDATA%\MinecraftPE_Netease\ — 游戏根目录
//     ├── games\com.netease\behavior_packs\ — 行为包目录（symlink 进来）
//     ├── games\com.netease\resource_packs\ — 资源包目录
//     └── minecraftWorlds\ — 存档目录
//   - <drive>:\MCStudioDownload\game\MinecraftPE_Netease\<version>\Minecraft.Windows.exe — 开发启动器
//
// 跨平台可编译（非 Windows 上 APPDATA 为空 → mcDir 返回 false）；扫盘 scanLaunchers 在
// paths_windows.go / paths_other.go 分平台。返回 (路径, 是否存在) 对齐 cc-pane 的 Option。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isDir 报告 path 是否为已存在的目录。
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// isFile 报告 path 是否为已存在的普通文件。
func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// mcDir 网易 MC 游戏根目录 = %APPDATA%\MinecraftPE_Netease。
func mcDir() (string, bool) {
	appData := os.Getenv("APPDATA")
	if !isDir(appData) {
		// fallback: $HOME\AppData\Roaming
		if home, err := os.UserHomeDir(); err == nil {
			appData = filepath.Join(home, "AppData", "Roaming")
		} else {
			return "", false
		}
	}
	dir := filepath.Join(appData, "MinecraftPE_Netease")
	if isDir(dir) {
		return dir, true
	}
	return "", false
}

// gameDir games\com.netease 目录（行为包/资源包符号链接的归宿）。
func gameDir() (string, bool) {
	mc, ok := mcDir()
	if !ok {
		return "", false
	}
	p := filepath.Join(mc, "games", "com.netease")
	if isDir(p) {
		return p, true
	}
	return "", false
}

func behaviorPacksDir() (string, bool) {
	g, ok := gameDir()
	if !ok {
		return "", false
	}
	p := filepath.Join(g, "behavior_packs")
	if isDir(p) {
		return p, true
	}
	return "", false
}

func resourcePacksDir() (string, bool) {
	g, ok := gameDir()
	if !ok {
		return "", false
	}
	p := filepath.Join(g, "resource_packs")
	if isDir(p) {
		return p, true
	}
	return "", false
}

func worldsDir() (string, bool) {
	mc, ok := mcDir()
	if !ok {
		return "", false
	}
	p := filepath.Join(mc, "minecraftWorlds")
	if isDir(p) {
		return p, true
	}
	return "", false
}

// validateLauncher 校验启动器路径：必须存在 + 是文件 + .exe 后缀。
func validateLauncher(path string) error {
	if !isFile(path) {
		return fmt.Errorf("启动器路径不存在或不是文件：%s", path)
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext != "exe" {
		return fmt.Errorf("启动器路径必须是 .exe：%s", path)
	}
	return nil
}
