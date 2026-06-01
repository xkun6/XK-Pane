// Package mcpe 是网易版 Minecraft（中国版 MC PE）开发启动子系统（对齐 cc-pane
// cc-panes-core/src/services/mc_pe）。复刻 MCStudio 的启动流程：定位游戏 / 启动器路径、
// 解析行为包/资源包 manifest、用符号链接把项目包接进游戏目录、把内嵌的游戏内 Python
// 调试桥模组解压进 behavior_packs、改写 level.dat、起 MC PE 进程并按行过滤日志。
//
// 大部分逻辑跨平台可编译（路径拼接 / JSON 解析 / NBT 读写），仅扫盘（GetLogicalDrives）、
// 符号链接提权（mklink /D + UAC）、ConPTY 启动需 Windows，经 build tag 文件分平台。
//
// 本文件 = 桥模组解压，忠实移植 bridge_mod.rs（include_dir → embed.FS）。
package mcpe

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BridgeUUID 桥模组在网易 behavior_packs/ 下的固定 UUID（与 manifest.json 一致）。
// 改它会导致启动时世界不自动启用桥模组。
const BridgeUUID = "183b0dc7-ae4a-48fb-800d-9d68f7162e7e"

// bridgeRoot 是 embed 树内桥模组根（embed 路径恒用 "/"）。
const bridgeRoot = "resources/include_test_mod"

// 整目录嵌入。必须用 all: 前缀，否则以 `_` 开头的 __init__.py（Python 包标记）会被
// go:embed 默认忽略，导致解压出的模组缺包标记无法加载。
//
//go:embed all:resources/include_test_mod
var bridgeFS embed.FS

// ExtractTo 把桥模组（连同 manifest.json）解压到目标目录，强制覆盖已存在文件。
// 每次启动 MC PE 都先解压，确保模组内容与本程序版本一致。
func ExtractTo(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("创建桥模组目录失败 %s: %w", targetDir, err)
	}
	return fs.WalkDir(bridgeFS, bridgeRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, bridgeRoot), "/")
		if rel == "" {
			return nil // 根目录已由 MkdirAll 建好
		}
		dst := filepath.Join(targetDir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := bridgeFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("读取嵌入 %s 失败: %w", p, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", dst, err)
		}
		return nil
	})
}

// bridgeManifest 返回内嵌 manifest.json 的内容（供校验 UUID 一致）。
func bridgeManifest() ([]byte, error) {
	return bridgeFS.ReadFile(bridgeRoot + "/manifest.json")
}
