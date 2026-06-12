package screenshotsvc

// save.go — 截图落盘与写时清理（对齐 cc-pane screenshot_service.rs）：
// <UserDataDir>/screenshots/screenshot_YYYYMMDD_HHMMSS_mmm.png；每次保存后异步删除
// 超过保留天数的旧 .png（0 = 永久保留）。PNG 属数据文件，直写即可（filex 仅用于配置）。

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xkpane/internal/storage"
)

// screenshotsDir 返回截图目录（不创建，落地交给调用方，对齐 storage.UserDataDir 约定）。
func screenshotsDir() (string, error) {
	root, err := storage.UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "screenshots"), nil
}

// saveScreenshot 把裁剪后的图像编码为 PNG 落盘，返回绝对路径（Windows 反斜杠格式，
// 可直接粘进 Claude Code）。保存成功后异步触发写时清理。
func (s *Service) saveScreenshot(img image.Image) (string, error) {
	dir, err := screenshotsDir()
	if err != nil {
		return "", fmt.Errorf("解析截图目录失败: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建截图目录失败: %w", err)
	}

	now := time.Now()
	name := fmt.Sprintf("screenshot_%s_%03d.png", now.Format("20060102_150405"), now.Nanosecond()/1e6)
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("创建截图文件失败: %w", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("PNG 编码失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("写入截图文件失败: %w", err)
	}

	retention := s.retentionDays()
	go func() {
		if err := cleanupOldScreenshots(dir, retention, time.Now()); err != nil {
			logFor().Warn("清理旧截图失败", "err", err)
		}
	}()
	return path, nil
}

// cleanupOldScreenshots 删除 dir 下 mtime 早于 now - retentionDays 的 .png。
// retentionDays <= 0 表示永久保留（跳过）。
func cleanupOldScreenshots(dir string, retentionDays int, now time.Time) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				logFor().Warn("删除过期截图失败", "file", e.Name(), "err", err)
			}
		}
	}
	return nil
}
