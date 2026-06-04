package app

import (
	"runtime"
	"sync/atomic"

	"xkpane/internal/events"
	"xkpane/internal/utils/logx"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// setupSystray 创建系统托盘并接好交互：
//   - 左键单击托盘图标：显示并聚焦主窗口（最常用动作，对应「关闭=隐藏到托盘」后从托盘唤回）。
//   - 右键单击托盘图标：弹出菜单（显示主窗口 / 设置 / 退出）。
//
// quitting 是「真正退出」标志：菜单「退出」先置位再 app.Quit()，主窗口的 WindowClosing hook
// （见 app.go）据此放行关闭；否则用户点窗口关闭键会被拦成「隐藏到托盘」。
func setupSystray(app *application.App, mainWindow *application.WebviewWindow, iconData []byte, quitting *atomic.Bool) {
	tray := app.SystemTray.New()

	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(iconData)
	} else {
		tray.SetIcon(iconData)
		tray.SetDarkModeIcon(iconData)
	}

	// 显示主窗口：取消最小化 + 显示 + 聚焦，三步覆盖「最小化 / 隐藏到托盘 / 仅失焦」各种状态。
	showMain := func() {
		mainWindow.UnMinimise()
		mainWindow.Show()
		mainWindow.Focus()
	}

	menu := app.NewMenu()
	menu.Add("显示主窗口").OnClick(func(*application.Context) { showMain() })
	menu.Add("设置").OnClick(func(*application.Context) {
		showMain()
		app.Event.Emit(events.NavigateSettings, nil)
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		quitting.Store(true)
		app.Quit()
	})
	tray.SetMenu(menu)

	tray.OnClick(showMain)
	tray.OnRightClick(func() { tray.OpenMenu() })

	logx.For("app").Info("systray initialized")
}
