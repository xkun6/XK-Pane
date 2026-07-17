//go:build windows

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// windowOptions 为 Windows 平台返回无边框窗口配置，
// 标题栏完全由前端 React 组件自绘（拖拽、最小化/最大化/关闭）。
func windowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title:            "XK-Pane",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        600,
		Frameless:        true,
		// 与 foundation-light 的 bg.content (#f7f8fb) 对齐，避免启动闪纯白
		BackgroundColour: application.NewRGB(247, 248, 251),
		URL:              "/",
	}
}
