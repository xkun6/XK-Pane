---
name: foundation-systray
description: XK-Pane 系统托盘实现指南：后端 setupSystray（左键单击显示主窗口 / 右键菜单 显示·设置·退出 / 关闭主窗口=隐藏到托盘）、菜单文案随应用语言切换重建、托盘图标 embed appicon、托盘「设置」跳转链路、退出加速。注意：原 foundation 注册式弹出面板系统（systray.html / src/systray / settings.ts）已整套删除。
---

# XK-Pane 系统托盘

XK-Pane 的托盘是**后端纯原生实现**：一个托盘图标 + 左键单击显示主窗口 + 右键原生菜单。**没有前端面板窗口**。

- **核心**：`internal/app/systray.go` — `setupSystray(...)`（启动期一次性创建，非 Wails Service）
- **装配**：`internal/app/app.go` — 创建主窗口、注册关闭 hook、接线菜单 i18n
- **图标**：`main.go` — `//go:embed build/appicon.png`（与任务栏 / exe 图标同源）

> ⚠️ **原 foundation 注册式弹出面板已删除**。旧版「点击托盘弹 MUI 面板 + 注册式模块」整套——`frontend/systray.html`、`frontend/src/systray/`、`frontend/src/services/systray/`、`frontend/src/settings.ts`、`internal/app/systray_*.go`、后端 `SystrayService` struct——**均已不存在**。若仍需面板形态请重新设计，别按 git 历史里的旧文件找。

---

## 1. 交互行为

| 操作 | 行为 | 实现 |
|------|------|------|
| 左键单击托盘图标 | 显示并聚焦主窗口 | `tray.OnClick(showMain)` |
| 右键单击托盘图标 | 弹出菜单【显示主窗口 / 设置 / 退出】 | `tray.OnRightClick(tray.OpenMenu)` |
| 菜单「显示主窗口」 | 同左键 | `showMain()` |
| 菜单「设置」 | 显示主窗口 + 跳转设置页 | `showMain()` + `Emit(NavigateSettings)` |
| 菜单「退出」 | 真正退出应用 | `quitting.Store(true)` + `app.Quit()` |
| 点标题栏关闭(X) | **隐藏到托盘**（不退出） | 主窗口 `WindowClosing` hook → `Cancel()` + `Hide()` |

`showMain` = `UnMinimise()` + `Show()` + `Focus()`，覆盖「最小化 / 隐藏到托盘 / 仅失焦」三态。

---

## 2. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **托盘动作全走后端直驱窗口 API** | 关闭主窗口会销毁 webview，前端事件无人接收；后端直接 `mainWindow.Show/Hide/Focus` 才可靠 |
| 2 | **关闭=隐藏到托盘必须用 `RegisterHook` 才能 `Cancel`** | `OnWindowEvent` 只是观察者，无法取消关闭 |
| 3 | **真正退出前必须 `quitting.Store(true)`** | 否则 `app.Quit()` 触发的关闭会被 hook 拦成 Hide，应用退不出去 |
| 4 | **托盘图标 embed `build/appicon.png`** | 与任务栏 / exe 图标同源，避免不一致 |
| 5 | **菜单文案随语言切换重建（非右键临时重建）** | 预重建好，`OpenMenu` 时即用最新菜单，无 `SetMenu→OpenMenu` 时序假设 |

---

## 3. 关闭=隐藏到托盘机制（核心）

主窗口创建后注册 `WindowClosing` hook，`quitting`（`atomic.Bool`）守门：

```go
// internal/app/app.go
var quitting atomic.Bool
mainWindow.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
    if !quitting.Load() {
        e.Cancel()        // 拦截关闭
        mainWindow.Hide() // 改为隐藏到托盘
    }
})
```

唯一的真正退出路径是托盘菜单「退出」：

```go
menu.Add(lb.quit).OnClick(func(*application.Context) {
    quitting.Store(true) // 先置位，放行下面 app.Quit 触发的 WindowClosing
    app.Quit()
})
```

> SingleInstance 的 `OnSecondInstanceLaunch` 也调 `UnMinimise/Show/Focus`：隐藏到托盘后再次启动 exe 同样唤回主窗口。

---

## 4. 菜单文案随语言切换（i18n）

后端原生菜单**不在前端 i18n 框架内**，靠「读 `app_settings.locale_choice` + 语言变更时重建菜单」实现：

```go
// internal/app/systray.go
func trayLabelsFor(locale string) trayLabels {
    if strings.HasPrefix(strings.ToLower(locale), "en") {
        return trayLabels{"Show Window", "Settings", "Quit"}
    }
    return trayLabels{"显示主窗口", "设置", "退出"} // zh-CN / auto / 未知 → 中文兜底
}

// setupSystray 返回 rebuildMenu，据当前 locale 整体重建菜单（Wails Rebuild Menu 模式）
return func() { tray.SetMenu(buildMenu()) }
```

切语言 → 重建的接线（app.go）：

```go
rebuildTrayMenu := setupSystray(app, mainWindow, trayIcon, &quitting, trayLocale)
appSettingsSvc.SetLocaleChangeHook(func(string) { rebuildTrayMenu() })
```

`appsettings.SetLocaleChoice` 写库成功后触发 `onLocaleChange`：

```go
// internal/services/appsettings/appsettings.go
func (s *Service) SetLocaleChoice(ctx context.Context, choice string) error {
    if err := /* 写 locale_choice */; err != nil { return err }
    if s.onLocaleChange != nil { s.onLocaleChange(choice) }
    return nil
}
```

> `auto` 后端读不到浏览器 navigator，统一兜底中文（与 reminder 通知一致）。

---

## 5. 托盘「设置」跳转链路

后端发事件 → 前端 hook 接住 → 路由跳转：

```
托盘菜单「设置」OnClick
  └─ showMain() + app.Event.Emit(events.NavigateSettings)        // 后端 internal/app/systray.go
       └─ useTrayNavigation 监听 AppEvents.NavigateSettings       // 前端 src/shared/hooks/useTrayNavigation.ts
            └─ navigate('settings')                               // AppLayout 接入
```

事件名 `"app:navigate-settings"` **三处同步**：`internal/events/events.go` / `frontend/src/shared/events.ts` / `events.test.ts`（`toEqual` 精确断言防漂移）。

---

## 6. setupSystray 签名

```go
// 返回 rebuildMenu（切语言重建菜单）；localeFn 读当前 app_settings.locale_choice。
func setupSystray(
    app *application.App,
    mainWindow *application.WebviewWindow,
    iconData []byte,
    quitting *atomic.Bool,
    localeFn func() string,
) (rebuildMenu func())
```

macOS 用 `SetTemplateIcon`（自适应深色），其余平台 `SetIcon` + `SetDarkModeIcon`。

---

## 7. 文件清单

```
main.go                                       — //go:embed build/appicon.png（托盘图标源）
internal/app/systray.go                       — setupSystray + trayLabelsFor（菜单文案中英）
internal/app/app.go                           — 主窗口 WindowClosing hook + quitting + 菜单 i18n 接线
internal/events/events.go                     — NavigateSettings 事件常量
internal/services/appsettings/appsettings.go  — onLocaleChange hook（切语言通知托盘重建）
frontend/src/shared/events.ts                 — AppEvents.NavigateSettings（+ events.test.ts 断言）
frontend/src/shared/hooks/useTrayNavigation.ts — 监听 NavigateSettings → 回调
frontend/src/components/AppLayout/AppLayout.tsx — 接入 useTrayNavigation → navigate('settings')
```

---

## 8. 常见改动

### 加一个菜单项
在 `internal/app/systray.go` 的 `buildMenu` 闭包里 `menu.Add(...)`；文案加进 `trayLabels` 结构 + `trayLabelsFor`（中英两版）。`OnClick` 里走后端窗口 API 或 `app.Event.Emit` 通知前端。

### 改图标
替换 `build/appicon.png`（PNG，`SetIcon` 接受）。它同时是 exe / 任务栏图标源（`build/` 打包资源），改一处即统一。

### 加一个「后端→前端」托盘动作
仿「设置」：events.go 加常量 → 前端 `events.ts` + `events.test.ts` 同步 → 加监听 hook → 在 RouterProvider 内的组件接入。

---

## 9. 反例

| 错误做法 | 正确做法 | 原因 |
|----------|----------|------|
| 托盘动作 `app.Event.Emit` 让前端自己 Show 窗口 | 后端直接 `mainWindow.Show/Focus` | 关闭后 webview 销毁，前端监听器没了，事件无人接 |
| 用 `OnWindowEvent(WindowClosing)` 拦截关闭 | 用 `RegisterHook` + `e.Cancel()` | OnWindowEvent 是观察者，不能取消 |
| `app.Quit()` 前忘了 `quitting.Store(true)` | 退出路径先置位 | 否则被关闭 hook 拦成 Hide，退不出 |
| 右键回调里 `SetMenu` 临时重建菜单 | 语言变更时预重建（rebuildMenu hook） | 避免 `SetMenu→OpenMenu` 时序竞争 |
| 托盘图标 embed `frontend/public/wails.png` | embed `build/appicon.png` | 前者是 Wails 默认 logo，与应用图标不一致 |
