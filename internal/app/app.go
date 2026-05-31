// Package app 封装 Wails 应用的启动逻辑。
// main.go 只保留 embed.FS 与 Run 调用，业务装配集中在这里。
package app

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"xkpane/internal/events"
	"xkpane/internal/services/appsettings"
	"xkpane/internal/services/cliadapters"
	"xkpane/internal/services/clitools"
	"xkpane/internal/services/gitsvc"
	"xkpane/internal/services/greet"
	"xkpane/internal/services/launchhistory"
	"xkpane/internal/services/lhwatcher"
	"xkpane/internal/services/localhistorysvc"
	"xkpane/internal/services/preferences"
	"xkpane/internal/services/project"
	"xkpane/internal/services/provider"
	"xkpane/internal/services/session"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/storagesvc"
	"xkpane/internal/services/subprocess"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/workspace"
	"xkpane/internal/storage"
	"xkpane/internal/utils/httpx"
	"xkpane/internal/utils/logx"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Run 创建 Wails 应用、注册服务、打开主窗口并启动事件循环。
// assets 由调用方（main.go）通过 //go:embed 注入。
func Run(assets embed.FS) error {
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
	// Local History 自动捕获 watcher（决策 D4-1 独立包）。依赖 lhSvc（窄接口 Saver：读配置 + 保存）。
	lhWatcher := lhwatcher.New(lhSvc)

	app := application.New(application.Options{
		Name:        "XK-Pane",
		Description: "Claude Code 多实例分屏管理桌面应用",
		Services: []application.Service{
			application.NewService(greet.New()),
			application.NewService(preferences.New(holder)),
			application.NewService(appsettings.New(holder)),
			application.NewService(storagesvc.New(holder, cfgMgr)),
			application.NewService(projSvc),
			application.NewService(launchhistory.New(holder)),
			application.NewService(provider.New(holder)),
			application.NewService(session.New(holder)),
			application.NewService(gitSvc),
			application.NewService(lhSvc),
			application.NewService(subSvc),
			application.NewService(termSvc),
			application.NewService(clitools.New(cliRegistry)),
			application.NewService(wsSvc),
			application.NewService(sessionscan.New()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	subSvc.AttachApp(app)
	termSvc.AttachApp(app)
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

	app.Window.NewWithOptions(windowOptions())

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
