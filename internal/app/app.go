// Package app 封装 Wails 应用的启动逻辑。
// main.go 只保留 embed.FS 与 Run 调用，业务装配集中在这里。
package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/appsettings"
	"xkpane/internal/services/bedrock"
	"xkpane/internal/services/bedrockdocsync"
	"xkpane/internal/services/cliadapters"
	"xkpane/internal/services/clitools"
	"xkpane/internal/services/file"
	"xkpane/internal/services/gitsvc"
	"xkpane/internal/services/greet"
	"xkpane/internal/services/launchhistory"
	"xkpane/internal/services/lhwatcher"
	"xkpane/internal/services/localhistorysvc"
	"xkpane/internal/services/mcdkbridge"
	"xkpane/internal/services/mcpe"
	"xkpane/internal/services/notification"
	"xkpane/internal/services/orchestrator"
	"xkpane/internal/services/preferences"
	"xkpane/internal/services/project"
	"xkpane/internal/services/provider"
	"xkpane/internal/services/reminder"
	"xkpane/internal/services/session"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/storagesvc"
	"xkpane/internal/services/subprocess"
	"xkpane/internal/services/taskbinding"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/todo"
	"xkpane/internal/services/transcriptsvc"
	"xkpane/internal/services/workspace"
	"xkpane/internal/storage"
	"xkpane/internal/utils/httpx"
	"xkpane/internal/utils/logx"
	"xkpane/internal/webapi"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// Run 创建 Wails 应用、注册服务、打开主窗口并启动事件循环。
// assets 由调用方（main.go）通过 //go:embed 注入。
func Run(assets embed.FS, trayIcon []byte) error {
	// —— 工具层先就绪：日志 → HTTP UA → 后续模块都能直接用 ——
	if err := logx.Init(logx.Config{
		Level:          slog.LevelInfo,
		ConsoleEnabled: true,
	}); err != nil {
		// 日志初始化失败不应阻断启动；降级到默认 stderr logger（logx.For 内部已兜底）
		fmt.Println("warn: logx init failed:", err)
	}
	httpx.SetUserAgent("XK-Pane/1.0 (+wails-v3)")

	events.Register()

	// —— 持久化层启动顺序 ——
	// 1) 读 storage.json：用户自定义路径 / 默认路径
	// 2) 用 effectivePath 打开 SQLite（PRAGMA + AutoMigrate + IntegrityCheck）
	// 3) 包成 Holder，业务 service 全部走 holder.Current() 拿活跃 DB
	cfgMgr, err := storage.LoadConfig()
	if err != nil {
		return fmt.Errorf("load storage config: %w", err)
	}
	dbPath, err := cfgMgr.EffectiveDBPath()
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	holder := storage.NewHolder(db)
	logx.For("app").Info("storage opened", "path", dbPath)

	// subprocess / terminal service 需要 *application.App 才能向前端 Emit 事件。
	// 但 application.New 又需要 services 列表 —— 用预创建 + Bind 模式：
	//   先 new 一个空 service 占位，然后在 app.New 之后回填 app 引用。
	subSvc := subprocess.New(nil) // app 暂为 nil
	// CLI 工具注册表只建一次，terminal（启动拼装）与 clitools（暴露给前端列表）共享同一实例，
	// 避免两处各自 Default() 导致 DetectAll 重复跑 which + --version 探测。
	cliRegistry := cliadapters.Default()
	termSvc := terminal.New(cliRegistry) // PTY 会话注册表，app 经 AttachApp 回填
	// gitsvc / localhistorysvc 预创建以便互联：gitsvc 在 stash/pull/push 前调 localhistorysvc
	// 打 Local History 标签（cc-pane auto_label_before_git）。两者皆注册，连接经 SetAutoLabeler。
	gitSvc := gitsvc.New()
	lhSvc := localhistorysvc.New()
	// project / workspace service 提为变量：既注册给前端，又供 watcher 启动扫描「已登记项目」。
	projSvc := project.New(holder)
	wsSvc := workspace.New(holder)
	// 扫盘历史会话 service（无状态、纯文件系统只读）。提为变量：既注册给前端 bindings，
	// 又注入 Web 网关供 GET /api/sessions/scan（手机「按项目恢复会话」）。
	scanSvc := sessionscan.New()
	// 结构化时间线门面（卡片视图取数）：把 webapi 的 sessionId→转录解析逻辑包成 Wails 方法，
	// 让桌面（owner）端 SessionTimeline 也能取数（Web 仍走 /api/sessions/timeline，同源同语义）。
	transcriptSvc := transcriptsvc.New(termSvc, scanSvc)
	// Local History 自动捕获 watcher（决策 D4-1 独立包）。依赖 lhSvc（窄接口 Saver：读配置 + 保存）。
	lhWatcher := lhwatcher.New(lhSvc)
	// 桌面通知（P6d-4）：Wails 通知 service 单例（须注册进 Services 跑 Windows ServiceStartup，
	// 注册 toast GUID/激活器）+ XK notification 服务（读 app_settings 开关 + 去重 + 聚焦判定）。
	wailsNotifier := notifications.New()
	notifSvc := notification.New(holder, wailsNotifier)
	// 子窗口系统（foundation 框架能力）：类型化子窗口（confirm / message / blank）+ 事件总线通信。
	// 同 subprocess 用预创建 + AttachApp 回填 app 引用模式。
	childWinSvc := NewChildWindowService()

	// Web 网关（P7 M1）：独立 0.0.0.0 HTTP server，托管同一份 frontend/dist 供手机/平板局域网访问。
	// 与 orchestrator（127.0.0.1 + MCP）物理隔离。webAccessSvc 是薄 Wails 门面（只暴露 GetAccessInfo
	// 给「设置 → Web 访问」卡片渲染配对二维码）；网关生命周期（Start/Stop）由本装配处直接管。
	webFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("sub web assets: %w", err)
	}
	webGateway := webapi.NewGateway(holder, webFS)
	webAccessSvc := webapi.NewService(webGateway)

	// MC PE 控制（P6e）：mcpeSvc 注册给前端（包管理 / 启动 / 日志），经 SetEmitter 双路推日志事件；
	// mcdkBridge 是后端 TCP 桥（127.0.0.1:0，等游戏内 Python 模组反向连入），非 Wails service，
	// 生命周期由本装配处直接管（Start/Stop，fail-open）。
	mcpeSvc := mcpe.New(holder)
	mcdkBridge := mcdkbridge.New()
	// Bedrock 知识库（P6f）：只读查询服务，注入 orchestrator 供 bedrock_* MCP 工具调用。
	// 查询本身无状态、无 storage holder，**不进 Wails Services 列表**（无前端绑定，仅 MCP 暴露）。
	bedrockSvc := bedrock.New()
	// Todo 待办（结合 ccpane 接回）：前端 TodoPage 调 CRUD/Query/Stats；同一实例注入 orchestrator
	// 供 query_todos/create_todo/update_todo MCP 工具（dispatch-todos skill 经此自驱待办）。
	todoSvc := todo.New(holder)
	// TaskBinding 编排任务绑定（结合 ccpane）：记录每次「分派」(Todo→会话) 及其状态/进度。
	// 前端「分派」按钮 + orchestrator create/update/query_task_binding MCP 工具共用同一实例。
	tbSvc := taskbinding.New(holder)
	// Bedrock OTA 文档同步（P6f-9c）：把 gitea/http 引擎 + 索引构建器 + manifest 串成一次同步，
	// 原子换目录到 <UserDataDir>/bedrock-data/ 后经 BedrockHooks 热切 bedrock 查询层。进 Wails
	// Services 列表（前端「设置 → 知识库」面板调用），经 SetEmitter 推 BedrockDocSyncProgress 进度。
	bedrockDataRoot, err := storage.UserDataDir()
	if err != nil {
		logx.For("app").Warn("resolve user data dir for bedrock OTA failed (OTA 落相对目录)", "err", err)
	}
	bedrockDocSyncSvc := bedrockdocsync.New(bedrockDataRoot, bedrockdocsync.BedrockHooks{
		SetDataRoot: bedrock.SetDataRoot,
		RuntimeStats: func() bedrockdocsync.ManifestStats {
			st := bedrock.RuntimeStats()
			return bedrockdocsync.ManifestStats{
				APIEntries:      st.APIEntries,
				TemplateEntries: st.TemplateEntries,
				GuideFiles:      st.GuideFiles,
			}
		},
	})
	// 启动期激活已有 OTA 数据：<root>/bedrock-data/manifest.json 存在即热切到该树（对齐 cc-pane
	// lib.rs 启动检测）；不存在则保持纯内嵌基线。OTA 同步成功后 service 会再次 SetDataRoot 切换。
	if bedrockDataRoot != "" {
		liveDir := filepath.Join(bedrockDataRoot, "bedrock-data")
		if _, statErr := os.Stat(filepath.Join(liveDir, "manifest.json")); statErr == nil {
			bedrock.SetDataRoot(liveDir)
			logx.For("app").Info("bedrock OTA data activated on boot", "dir", liveDir)
		}
	}

	app := application.New(application.Options{
		Name:        "XK-Pane",
		Description: "Claude Code 多实例分屏管理桌面应用",
		Services: []application.Service{
			application.NewService(greet.New()),
			application.NewService(preferences.New(holder)),
			application.NewService(appsettings.New(holder)),
			application.NewService(storagesvc.New(holder, cfgMgr)),
			application.NewService(projSvc),
			application.NewService(file.New(holder)), // 文件浏览器：列目录/读写文件/RevealInOS/最近目录
			application.NewService(launchhistory.New(holder)),
			application.NewService(todoSvc), // 待办 CRUD（结合 ccpane；orchestrator 复用同实例）
			application.NewService(tbSvc),   // 编排任务绑定 CRUD（结合 ccpane；orchestrator 复用同实例）
			application.NewService(provider.New(holder)),
			application.NewService(session.New(holder)),
			application.NewService(gitSvc),
			application.NewService(lhSvc),
			application.NewService(subSvc),
			application.NewService(termSvc),
			application.NewService(clitools.New(cliRegistry)),
			application.NewService(wsSvc),
			application.NewService(scanSvc),
			application.NewService(transcriptSvc), // 结构化时间线（桌面卡片视图取数）
			application.NewService(wailsNotifier),     // Wails 通知 service（跑 ServiceStartup 注册 toast）
			application.NewService(childWinSvc),       // 子窗口系统
			application.NewService(webAccessSvc),      // Web 网关门面（GetAccessInfo → 配对二维码）
			application.NewService(mcpeSvc),           // MC PE 控制（包管理 / 启动 / 日志 / 桥状态）
			application.NewService(bedrockDocSyncSvc), // Bedrock OTA 文档同步（设置 → 知识库面板）
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	subSvc.AttachApp(app)
	// 组合双路 emit（P7 §6.1）：会话事件同时推给 ① 桌面 webview（app.Event.Emit，进程内同步、
	// 零回归）② Web 网关（RouteEvent → 浏览器 WS，白名单过滤 + 慢端剔除，绝不反压回 PTY）。
	// 取代 termSvc/orchSvc.AttachApp（二者原本仅注入 app.Event.Emit 单路；AttachApp 保留供单测）。
	mirrorEmit := func(name string, payload any) {
		app.Event.Emit(name, payload)
		webGateway.RouteEvent(name, payload)
	}
	termSvc.SetEmitter(mirrorEmit)
	mcpeSvc.SetEmitter(mirrorEmit)           // MC PE 日志/退出事件双路（桌面 webview + Web 网关）
	mcpeSvc.SetBridge(mcdkBridge)            // 取桥端口注入 MCDEV_DEBUG_IPC_PORT + 查桥状态
	bedrockDocSyncSvc.SetEmitter(mirrorEmit) // OTA 同步进度事件双路（桌面 webview + Web 网关）
	childWinSvc.AttachApp(app)
	lhWatcher.AttachApp(app) // watcher 经此向前端 Emit localhistory:changed（live 刷新）
	// Local History 启停补启接线（决策 D4-4）：
	//   - SaveConfig(enabled=true) 即时启 / (false) 即时停该项目 watcher；
	//   - 启动项目（terminal.CreateSession）时若已启用本地历史则补启 watcher。
	lhSvc.SetConfigHook(func(ctx context.Context, projectPath string, enabled bool) {
		if enabled {
			lhWatcher.EnsureWatching(ctx, projectPath)
		} else {
			lhWatcher.StopWatching(projectPath)
		}
	})
	termSvc.SetSessionStartHook(func(projectPath string) {
		lhWatcher.EnsureWatching(context.Background(), projectPath)
	})
	// 连接 Git → Local History：stash/pull/push 前给项目历史打 auto 标签（localhistorysvc 内
	// 已守门：仅对已在用 Local History 的项目生效，不会为纯 git 操作凭空创建 .xkpanes/）。
	gitSvc.SetAutoLabeler(func(ctx context.Context, repoPath, name, source, branch string) error {
		_, err := lhSvc.AutoLabel(ctx, repoPath, name, source, branch)
		return err
	})
	// 注入 provider 解析器：启动时据 spec.ProviderID 解析 Tier-A 启动 env +
	// Tier-B adapter Provider 上下文（含解密明文 key）。闭包从 holder 取活跃 DB，
	// 故切换数据路径后仍解析到正确库。provider.ResolveLaunchEnv 非绑定（不暴露明文给前端）。
	termSvc.SetProviderResolver(func(ctx context.Context, providerID string) (map[string]string, *cliadapters.Provider, error) {
		return provider.ResolveLaunchEnv(ctx, holder.Current().GORM, providerID)
	})

	mainWindow := app.Window.NewWithOptions(windowOptions())
	// 通知 only_when_unfocused 判定：注入主窗口聚焦查询（解耦 notification service 与 Wails 窗口 API）。
	notifSvc.SetFocusCheck(func() bool { return mainWindow.IsFocused() })

	// 待办提醒后台轮询（结合 ccpane G2）：每 60s 扫 reminder_at<=now 的未完成待办 → 经 notifSvc 发原生
	// 通知（读 app_settings 开关 + 聚焦判定 + 10min 重提醒去重）。后端 ticker 不受窗口最小化时
	// 浏览器定时器节流影响。locale 取 app_settings 决定通知标题语言（'auto'/未知→中文）。
	// ctx 随 Run 返回时 cancel，让轮询 goroutine 洁净退出（对齐 orchSvc/webGateway 的 defer 清理）。
	reminderCtx, cancelReminders := context.WithCancel(context.Background())
	defer cancelReminders()
	reminder.New(todoSvc, notifSvc, func() string {
		var row storage.AppSettings
		if err := holder.Current().GORM.First(&row, "id = ?", 1).Error; err != nil {
			return "zh-CN"
		}
		return row.LocaleChoice
	}).Start(reminderCtx)

	// 系统托盘（foundation 框架能力，本项目启用）：托盘图标 + 隐藏面板窗口 + 右键菜单。
	setupSystray(app, trayIcon)

	// —— MCP/Orchestrator 地基（P6a）：本地 HTTP+MCP server，让 pane 内 CLI 经 MCP 反向调用宿主。——
	// 启动失败 fail-open（CLI 裸跑无 MCP 注入，不阻断应用启动）；端口/token 注入 terminal 供 P6b 生成 MCP 配置。
	orchSvc := orchestrator.New(orchestrator.Deps{
		Terminal:   termSvc,
		Projects:   projSvc,
		Workspaces: wsSvc,
		Notifier:   notifSvc,
		MCPE:       mcpeSvc,    // P6e-6：get_mc_pe_logs 读日志缓冲（+ P6e-7 capture/click 取 PID）
		Bridge:     mcdkBridge, // P6e-6：execute_mc_code / reload_mc 经桥 fire-and-forget 发帧
		Bedrock:    bedrockSvc, // P6f：bedrock_* 只读知识库查询（API/事件/组件/分类…）
		// 结合 ccpane：todo + task_binding 复用前端同款实例 → query/create/update_todo +
		// create/update/query_task_binding MCP 工具（dispatch-todos skill 的「Todo 驱动」闭环据此跑通）。
		Todos:        todoSvc,
		TaskBindings: tbSvc,
	})
	orchSvc.SetEmitter(mirrorEmit) // 编排工具 Emit orchestrator:launch 经双路达桌面 + Web
	// Web 网关远程端点的下游能力（fail-closed 白名单 §9）：launch（公共化 LaunchTask）+ 只读列表。
	webGateway.AttachServices(webapi.ServiceDeps{
		Launcher:   orchSvc,
		Sessions:   termSvc,
		Projects:   projSvc,
		Workspaces: wsSvc,
		Scanner:    scanSvc,
		Replayer:   termSvc,
		Writer:     termSvc,
	})
	if err := orchSvc.Start(); err != nil {
		logx.For("app").Warn("orchestrator start failed (CLI 将无 MCP 注入，fail-open)", "err", err)
	} else {
		termSvc.SetOrchestrator(orchSvc.Port(), orchSvc.Token())
	}
	defer func() { _ = orchSvc.Stop() }()

	// —— Web 网关（P7 M1）：bind 0.0.0.0，手机/平板经局域网访问。失败 fail-open（Web 访问不可用，
	// 不阻断桌面应用）。端口被占等启动失败仅 warn。——
	if err := webGateway.Start(); err != nil {
		logx.For("app").Warn("webapi start failed (Web 访问不可用，fail-open)", "err", err)
	}
	defer func() { _ = webGateway.Stop() }()

	// —— MCDK 调试桥（P6e）：bind 127.0.0.1:0 等游戏内 Python 桥模组反向连入。失败 fail-open
	// （桥不可用 → 截图/点击/execute/reload 工具报错，但不阻断应用与 MC PE 启动）。——
	if err := mcdkBridge.Start(); err != nil {
		logx.For("app").Warn("mcdk bridge start failed (MC PE 调试桥不可用，fail-open)", "err", err)
	}
	defer func() { _ = mcdkBridge.Stop() }()
	defer mcpeSvc.CleanupAll() // app 退出时杀所有 MC PE 子进程

	go emitTimeLoop(app)

	// Local History 自动捕获：启动时扫已登记项目，仅对已启用本地历史（.xkpanes/history 存在且
	// config.enabled）的项目启 watcher（决策 D4-4）。放后台 goroutine 避免阻塞窗口出现（涉及扫盘 +
	// 逐项目读 config.json）。SaveConfig(enabled) / launchProject 时的补启在 P4a-2 接线。
	go func() {
		paths := registeredProjectPaths(context.Background(), projSvc, wsSvc)
		lhWatcher.StartRegistered(context.Background(), paths)
	}()

	defer logx.Close()
	defer lhWatcher.StopAll() // app 退出时停所有 watcher（defer LIFO：先于 logx.Close 执行）
	return app.Run()
}

// registeredProjectPaths 汇总「已登记项目」的去重路径：全局 projects 表 + 各 workspace 下的
// workspace_projects。供 watcher 启动扫描判定哪些项目需要监控。
func registeredProjectPaths(ctx context.Context, projSvc *project.Service, wsSvc *workspace.Service) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if ps, err := projSvc.List(ctx); err == nil {
		for _, p := range ps {
			add(p.Path)
		}
	} else {
		logx.For("app").Warn("list projects for watcher failed", "err", err)
	}
	if wss, err := wsSvc.List(ctx); err == nil {
		for _, ws := range wss {
			for _, p := range ws.Projects {
				add(p.Path)
			}
		}
	} else {
		logx.For("app").Warn("list workspaces for watcher failed", "err", err)
	}
	return out
}

// emitTimeLoop 演示后端主动推事件给前端的链路。
// 真实业务可参考此结构注册自己的后台 goroutine。
func emitTimeLoop(app *application.App) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		app.Event.Emit(events.Time, time.Now().Format(time.RFC1123))
	}
}
