package mcpe

// symlink.go — 行为包/资源包目录符号链接管理（跨平台部分）。忠实移植 symlink.rs。
//
// MC PE 启动前把项目包目录通过 symlink 接到
// %APPDATA%\MinecraftPE_Netease\games\com.netease\{behavior,resource}_packs\<uuid>\。
// 无 SeCreateSymbolicLinkPrivilege 时创建失败，收集为待提权列表，最后合并一个 .bat →
// PowerShell Start-Process -Verb RunAs 一次性弹 UAC（见 symlink_windows.go sudoCreateSymlinks）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xkpane/internal/utils/logx"
)

// symlinkPair 一对待创建的 (源目录 target, 链接 link)。
type symlinkPair struct {
	target string
	link   string
}

// removeSymlinksIn 清理目录下所有 symlink（保留实体子目录）。
func removeSymlinksIn(dir string) error {
	if !isDir(dir) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取 %s: %w", dir, err)
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		fi, err := os.Lstat(path) // 不跟随 link
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil { // os.Remove 处理目录 symlink（reparse point）
				logx.For("mcpe").Warn("删除 symlink 失败", "path", path, "err", err)
			}
		}
	}
	return nil
}

// createSymlink 创建符号链接 target ← link。
//
// 返回 (true, nil)  = 已存在指向同一目标的 symlink，或新建成功；
//
//	(false, nil) = 创建失败但属权限错误（应记入待提权集合）；
//	(false, err) = 其他错误（路径冲突 / I/O）。
func createSymlink(target, link string) (bool, error) {
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if resolved, err := os.Readlink(link); err == nil && resolved == target {
				return true, nil // 已是指向同目标的 symlink
			}
			_ = os.Remove(link) // symlink 但指向不对 → 删除重建
		} else {
			return false, fmt.Errorf("%s 已存在但不是 symlink，拒绝覆盖", link)
		}
	}
	if parent := filepath.Dir(link); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return false, fmt.Errorf("创建父目录 %s: %w", parent, err)
		}
	}
	err := os.Symlink(target, link)
	if err == nil {
		return true, nil
	}
	if isPrivilegeError(err) {
		return false, nil // 记入待提权
	}
	logx.For("mcpe").Error("symlink 失败", "link", link, "target", target, "err", err)
	return false, err
}

// buildSymlinkBat 校验所有路径（命令注入防御）并生成 mklink /D 批处理内容。
// 提取为独立函数便于测试（无需真的弹 UAC）。
func buildSymlinkBat(pairs []symlinkPair) (string, error) {
	for _, p := range pairs {
		if err := validateCommandLinePath(p.target); err != nil {
			return "", err
		}
		if err := validateCommandLinePath(p.link); err != nil {
			return "", err
		}
	}
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	for _, p := range pairs {
		// mklink /D = 目录符号链接（不是 junction /J），与 cc-pane 一致。
		fmt.Fprintf(&b, "mklink /D \"%s\" \"%s\"\r\n", p.link, p.target)
	}
	return b.String(), nil
}
