//go:build !windows

package orchestrator

// 非 Windows：截图 / 点击依赖 Win32 GDI/User32 + SendInput，不注册 capture_window / click_window
// （对齐 cc-pane —— 这两个工具是 #[cfg(target_os = "windows")]）。

import mcp "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Service) registerMCPECaptureTools(_ *mcp.Server) {}
