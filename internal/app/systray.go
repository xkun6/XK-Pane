package app

import (
	"runtime"
	"strings"
	"sync/atomic"

	"xkpane/internal/events"
	"xkpane/internal/utils/logx"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayLabels 托盘菜单三项文案（随应用语言切换）。
type trayLabels struct {
	show, settings, quit string
}

// trayLabelsFor 按 locale 返回菜单文案。en* → 英文；其余（zh-CN / auto / 未知）→ 中文兜底
// （与 reminder 的 'auto'→中文 一致：后端原生菜单读不到浏览器 navigator，auto 默认中文）。
func trayLabelsFor(locale string) trayLabels {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return trayLabels{show: "Show Window", settings: "Settings", quit: "Quit"}
	}
	return trayLabels{show: "显示主窗口", settings: "设置", quit: "退出"}
}

// setupSystray 创建系统托盘并接好交互：
//   - 左键单击托盘图标：显示并聚焦主窗口（最常用动作，对应「关闭=隐藏到托盘」后从托盘唤回）。
//   - 右键单击托盘图标：弹出菜单（显示主窗口 / 设置 / 退出）。
//
// quitting 是「真正退出」标志：菜单「退出」先置位再 app.Quit()，主窗口的 WindowClosing hook
// （见 app.go）据此放行关闭；否则用户点窗口关闭键会被拦成「隐藏到托盘」。
//
// localeFn 返回当前应用语言（读 app_settings.locale_choice）。返回的 rebuildMenu 供 app.go
// 注册到 appsettings 语言变更 hook：用户切语言时据新文案整体重建菜单（Wails systray 推荐的
// Rebuild Menu 方式——预先重建好，OpenMenu 时即用最新菜单，无 SetMenu→OpenMenu 时序假设）。
func setupSystray(app *application.App, mainWindow *application.WebviewWindow, iconData []byte, quitting *atomic.Bool, localeFn func() string) (rebuildMenu func()) {
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

	// buildMenu 据当前 locale 生成菜单；切语言时整体重建（见 rebuildMenu）。
	buildMenu := func() *application.Menu {
		lb := trayLabelsFor(localeFn())
		menu := app.NewMenu()
		menu.Add(lb.show).OnClick(func(*application.Context) { showMain() })
		menu.Add(lb.settings).OnClick(func(*application.Context) {
			showMain()
			app.Event.Emit(events.NavigateSettings, nil)
		})
		menu.AddSeparator()
		menu.Add(lb.quit).OnClick(func(*application.Context) {
			quitting.Store(true)
			app.Quit()
		})
		return menu
	}

	tray.SetMenu(buildMenu())
	tray.OnClick(showMain)
	tray.OnRightClick(func() { tray.OpenMenu() })

	logx.For("app").Info("systray initialized")

	return func() { tray.SetMenu(buildMenu()) }
}
