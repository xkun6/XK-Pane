package mcpe

// worldcopy.go — 存档复制 + 重置工具。忠实移植 world_copy.rs。
//
// 用于「导入现成地图」：把源 world 的必要内容复制到 minecraftWorlds/<folderName>，
// 之后操作副本，绝不污染源。

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyWorldEssentials 严格白名单：仅把 db/（递归）+ level.dat 从 src 复制到 dst。
//
// 为什么这么严：网易“联机大厅作品包”把 behavior_packs/ resource_packs/ 与 world 数据
// （db/、level.dat）同级放在项目根下。前者由 createSymlink 接到全局目录供引擎加载，
// 绝不能再复制进 world 内（否则引擎在 symlink 与 world 内嵌两处看到同一 pack_id，整个
// 作品包被忽略）。调用方负责事先清理 dst（见 removeDirSafely）。
func copyWorldEssentials(src, dst string) error {
	if !isDir(src) {
		return fmt.Errorf("源目录不存在或不是目录：%s", src)
	}
	srcLevelDat := filepath.Join(src, "level.dat")
	srcDB := filepath.Join(src, "db")
	if !isFile(srcLevelDat) {
		return fmt.Errorf("源目录缺少 level.dat：%s", src)
	}
	if !isDir(srcDB) {
		return fmt.Errorf("源目录缺少 db/：%s", src)
	}
	if parent := filepath.Dir(dst); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("创建父目录 %s: %w", parent, err)
		}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("创建目标目录 %s: %w", dst, err)
	}
	if err := copyFile(srcLevelDat, filepath.Join(dst, "level.dat")); err != nil {
		return fmt.Errorf("复制 level.dat: %w", err)
	}
	return copyDirRecursive(srcDB, filepath.Join(dst, "db"))
}

// copyDirRecursive 递归复制 src 到 dst。跳过 symlink/junction 条目（避免循环引用 / 别名混乱）。
// dst 父目录自动创建；已有同名 dst 不被清空，调用方按需先 removeDirSafely。
func copyDirRecursive(src, dst string) error {
	if !isDir(src) {
		return fmt.Errorf("源目录不存在或不是目录：%s", src)
	}
	if parent := filepath.Dir(dst); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("创建父目录 %s: %w", parent, err)
		}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("创建目标目录 %s: %w", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取 %s: %w", src, err)
	}
	for _, e := range entries {
		entryPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		// Lstat 不跟随链接，区分 symlink vs 实体。
		fi, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			continue // 跳过 symlink
		case fi.IsDir():
			if err := copyDirRecursive(entryPath, dstPath); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if err := copyFile(entryPath, dstPath); err != nil {
				return fmt.Errorf("复制文件 %s: %w", entryPath, err)
			}
		}
	}
	return nil
}

// removeDirSafely 安全删除目录（含其内 junction/symlink）。仅当 path 存在时操作。
// NTFS reparse point 用 os.Remove（不递归），普通目录 os.RemoveAll。
func removeDirSafely(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil // 不存在
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("删除 reparse point %s: %w", path, err)
		}
	case fi.IsDir():
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("递归删除 %s: %w", path, err)
		}
	default:
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("删除 %s: %w", path, err)
		}
	}
	return nil
}

// copyFile 复制单个文件（Go 无标准单调用文件复制）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
