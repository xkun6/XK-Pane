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
	"xkpane/internal/services/gitsvc"
	"xkpane/internal/services/greet"
	"xkpane/internal/services/launchhistory"
	"xkpane/internal/services/localhistorysvc"
	"xkpane/internal/services/preferences"
	"xkpane/internal/services/project"
	"xkpane/internal/services/provider"
	"xkpane/internal/services/storagesvc"
	"xkpane/internal/services/subprocess"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/todo"
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
	subSvc := subprocess.New(nil)             // app 暂为 nil
	termSvc := terminal.New(cliadapters.Default()) // PTY 会话注册表，app 经 AttachApp 回填
	// gitsvc / localhistorysvc 预创建以便互联：gitsvc 在 stash/pull/push 前调 localhistorysvc
	// 打 Local History 标签（cc-pane auto_label_before_git）。两者皆注册，连接经 SetAutoLabeler。
	gitSvc := gitsvc.New()
	lhSvc := localhistorysvc.New()

	app := application.New(application.Options{
		Name:        "XK-Pane",
		Description: "Claude Code 多实例分屏管理桌面应用",
		Services: []application.Service{
			application.NewService(greet.New()),
			application.NewService(preferences.New(holder)),
			application.NewService(appsettings.New(holder)),
			application.NewService(storagesvc.New(holder, cfgMgr)),
			application.NewService(project.New(holder)),
			application.NewService(launchhistory.New(holder)),
			application.NewService(provider.New(holder)),
			application.NewService(todo.New(holder)),
			application.NewService(gitSvc),
			application.NewService(lhSvc),
			application.NewService(subSvc),
			application.NewService(termSvc),
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

	defer logx.Close()
	return app.Run()
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
