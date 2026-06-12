//go:build !windows

package screenshotsvc

// screenshot_other.go — 非 Windows 桩：截图快捷键仅 Windows 实现（对齐 cc-pane
// 仅 #[cfg(target_os = "windows")] 注册全局热键）。Start no-op fail-open。

import "errors"

type hotkeyLoop struct{}

func (s *Service) platformStart(string) error {
	logFor().Info("截图快捷键仅支持 Windows，已跳过注册")
	return nil
}

func (s *Service) platformStop() {}

func (s *Service) platformReregister(uint32, uint32) error {
	return errors.New("截图快捷键仅支持 Windows")
}

func (s *Service) platformClipboardPastePaths() ([]string, error) {
	return nil, nil
}
