# XK-Pane

XK-Pane 是基于 Wails v3 的桌面工作台：以多面板终端（PTY + xterm.js）为核心，编排
claude / codex 等 CLI 会话，配套工作区 / 项目管理、本地文件历史、Todo 分派、SSH 连接
中心、MC PE（Minecraft 中国版）调试桥，以及只读 Web 镜像（viewer）。

## 架构总览

- 后端：Go 1.25（`module xkpane`），`main.go` 仅做 `embed.FS` + `app.Run`，业务装配集中在 `internal/app`。
- 前端：React 19 + TypeScript（strict）+ MUI 9 + Vite 8，主题统一在 `frontend/src/styles/themes/`。
- 通信：通过 `wails3 generate bindings` 生成 `frontend/bindings/xkpane/...`，前端 `services/` 层包装；后端事件集中注册在 `internal/events/events.go`。
- Web 镜像：`internal/webapi/` 提供 viewer HTTP 端点（桌面为 owner，Web 为受限只读 + 白名单操作）。
- 构建：根 `Taskfile.yml` 调用各平台子 Taskfile（Windows/macOS/Linux/Docker）。

通信链路：

```
React 组件 (View)
    └── ViewModel (use<Page>)
        └── services/preferences/PreferencesService.ts
            └── @bindings/xkpane/internal/services/preferences (Wails 生成)
                └── 后端 internal/services/preferences

后端 internal/events/events.go (RegisterEvent[LocalHistoryChangedPayload]("localhistory:changed"))
    └── 后端业务 service: app.Event.Emit(events.LocalHistoryChanged, ...)
        └── 前端 Events.On(AppEvents.LocalHistoryChanged, ...)（事件名见 shared/events.ts）
```

## 模块索引

| 模块 | 路径 | 一句话职责 |
|------|------|------------|
| 应用入口 | `main.go` | 仅含 `embed.FS` 与 `app.Run(assets)`，业务下沉到 `internal/` |
| 应用启动 | `internal/app/` | 服务注册、平台窗口配置（`window_<os>.go` build tag）、托盘、事件循环 |
| 业务服务 | `internal/services/` | Wails Service，30+ 领域子包（terminal / workspace / orchestrator / localhistorysvc / gitsvc / mcpe / mcdkbridge / sshmachine / todosvc / transcriptsvc / preferences / appsettings / storagesvc / …），每个领域一个子包 |
| 持久化层 | `internal/storage/` | SQLite 打开 / PRAGMA / AutoMigrate / 路径配置 / Holder |
| Web 镜像 | `internal/webapi/` | viewer HTTP 端点（会话列表 / 终端 attach / timeline / 白名单操作） |
| 工具层 | `internal/utils/` | httpx（HTTP）/ procx（子进程 + JobObject）/ ptyx（PTY）/ gitx（go-git）/ cryptox（AES-GCM）/ logx（slog + rotate）/ filex（原子写） |
| 事件契约 | `internal/events/` | 集中注册类型化事件，前后端共享名称 |
| 前端 UI | `frontend/` | React 19 + MUI 9 + TS；页面见 `frontend/src/pages/`，规范见 `frontend/CLAUDE.md` |
| 多平台构建 | `build/` | 各平台打包脚本与资源（windows / darwin / linux / docker） |

## 运行与开发

前置条件：Go 1.25+、Node.js + pnpm、`wails3` CLI、`task` (go-task)。

| 命令 | 说明 |
|------|------|
| `wails3 dev` 或 `task dev` | 前后端热重载（Vite 端口默认 9245） |
| `wails3 generate bindings` | 重新生成 `frontend/bindings/<module>/...` |
| `task build` | 当前 OS 生产构建 |
| `task package` | 当前 OS 打包 |
| `cd frontend && pnpm install` | 首次或更新依赖后 |
| `cd frontend && pnpm typecheck` | TypeScript 类型检查 |
| `cd frontend && pnpm build` | 前端单独生产构建 |
| `go build .` | 后端单独构建（需要 `frontend/dist` 已存在） |

## 持久化（SQLite + GORM）

> 详细使用指南见 `.claude/skills/foundation-persistence/SKILL.md`（加表三步、业务 service 接入、路径切换、表统计与清空、Provider 异步化模式、反例）。

应用所有跨会话状态存到 `%AppData%\XK-Pane\xkpane.db`（Windows）/ `~/Library/Application Support/XK-Pane/xkpane.db`（macOS）/ `~/.local/share/XK-Pane/xkpane.db`（Linux）；dev 构建目录带 `-Dev` 后缀（`internal/appinfo.AppDirName`），与 release 数据隔离。位置可由用户在「设置 → 数据存储」里改。配置文件 `storage.json` 与默认数据库放在同目录（独立于 db）。

**驱动 / ORM**：`modernc.org/sqlite`（纯 Go，无 CGO 需求）+ `gorm.io/gorm` + `github.com/glebarez/sqlite`（GORM 对接 modernc 的适配）。

**关键 PRAGMA**：WAL / `synchronous=NORMAL` / `busy_timeout=5000` / `cache_size=-64000` / `mmap_size=256MB` / `temp_store=MEMORY` / `foreign_keys=ON`。

**模型注册中心**：`internal/storage/models.go` 的 `AllModels` 数组。**加新表零迁移文件**：

```
1. 在 internal/storage/ 写 GORM 结构体（建议新文件 models_<domain>.go 拆开）
2. 把 &YourModel{} 追加到 AllModels
3. 重启 → AutoMigrate 自动建表 / 加列 / 加索引
```

GORM AutoMigrate **不会**做：删列、删表、重命名字段；这些保留旧 schema 不丢数据。需要重命名字段时业务方手写一次性数据迁移钩子。

**业务 service 接入持久化**：

```go
// internal/services/<domain>/<domain>.go
type Service struct {
    holder *storage.Holder // 不要 capture *storage.DB —— 切换路径会替换它
}

func (s *Service) DoSomething(ctx context.Context) error {
    return s.holder.Current().GORM.WithContext(ctx).Save(...).Error
}
```

切换路径时（`storagesvc.SetCustomPath`）会原子替换 holder 内的 DB 指针，业务 service 继续从 `holder.Current()` 取永远拿到活跃句柄。

## 测试策略

- Go：标准 `go test ./...`，测试与被测包同目录（`<name>_test.go`），后端各核心包均有覆盖。
  ⚠️ 在 XK-Pane 自己启动的会话内跑 `internal/services/terminal` 测试前必须清掉 `XKPANE_*` 环境变量，否则 BuildEnv 相关断言必败。
- 前端：Vitest（`pnpm test`），覆盖 ViewModel hook 与纯函数。

## 编码规范

- Go：`gofmt`、`go vet`，按 `internal/` 分层，跨文件夹引用走 import 而非相对路径。
- 前端：见 [frontend/DEVELOPMENT.md](./frontend/DEVELOPMENT.md)（MVVM、文件夹规约、theme-only 配色、方圆设计）。
- 文件路径：仓库内统一正斜杠。`.gitignore` 已忽略 `bin`、`frontend/dist`、`frontend/node_modules`、`frontend/bindings`、`.task`。

## AI 使用指引

- **分层准则**：业务逻辑写到 `internal/services/<domain>/`，启动配置写到 `internal/app/`，事件名集中在 `internal/events/events.go`，持久化模型集中在 `internal/storage/models.go`，跨业务横切关注点放 `internal/utils/<name>/`。`main.go` 不接受任何业务代码追加。
- **平台分支**：窗口配置走 `internal/app/window_<os>.go` 的 build tag 文件，不要在 `app.go` 里写 `runtime.GOOS` 判断。
- **持久化铁律**：跨会话状态**只能走 SQLite**（前端 `PreferencesService` / `AppSettingsService`，或自己加业务 service）。**禁止**前端用 `localStorage` / `sessionStorage`。后端业务 service 持有 `*storage.Holder`，**不要** capture `*storage.DB`（切换路径会替换它）。加新表只需把 `&Model{}` 追加到 `internal/storage/models.go` 的 `AllModels`，重启自动 AutoMigrate。详见 `.claude/skills/foundation-persistence/SKILL.md`。
- **工具层铁律**（详见 `.claude/skills/foundation-utils/SKILL.md`）：
  - **HTTP**：禁止 `net/http.DefaultClient` / `http.Get/Post`；用 `httpx.GetJSON` / `httpx.PostJSON` / `httpx.Do`
  - **子进程**：禁止 `os/exec.Cmd.Start` 直接用；用 `procx.StartCtx` / `procx.Run`（自动绑 Windows JobObject / Unix pgid）
  - **加密**：敏感字符串落 SQLite **必须**先 `cryptox.EncryptString`
  - **日志**：禁止 `log.Printf` / `fmt.Println`；用 `logx.For("component").Info(...)`
  - **配置文件**：禁止 `os.WriteFile` 写配置；用 `filex.WriteAtomic`
  - **前端调子进程**：必须经白名单 service（原 `internal/services/subprocess` 实现因白名单恒空已删，需要时按 git 历史恢复），**禁止**给前端开万能 `RunCommand`
- **前端调用边界**：组件 → ViewModel hook → `services/<Domain>Service.ts` → `@bindings/<module>/...`。View 不许直接 import bindings；ViewModel 不许直接 import bindings。
- **原生对话框铁律**：所有"打开 / 保存 / 选择文件夹 / 确认 / 错误提示"必须走 `NativeDialogs`（封装 Wails v3 `Dialogs.*`）。**禁止**使用浏览器 `alert` / `confirm` / `prompt` / `<input type="file">`。
- **Icon 铁律**：UI 中所有图标必须用 `@mui/icons-material` 的 React 组件（推荐 `*Rounded` 系列以匹配方圆设计语言）。**禁止** emoji、Unicode 符号、第三方 icon 包当图标。详见 `frontend/CLAUDE.md` 的「Icon 规范」。
- **样式取色**：永远从 `theme.palette.foundation.*` 取，不要硬编码十六进制。圆角语言：按钮 12 / 容器 8（详见 `frontend/CLAUDE.md`）。
- **i18n 铁律**：任何"人类可见字符串"必须经 `t('key')` —— JSX 文本、`aria-label`、`placeholder`、Tooltip、按钮文字、错误文案、对话框标题 / 消息 / 按钮、常量字符串都不允许硬编码。新增页面必须配套 `pages/<Name>/lang/{<code>.ts}` 并在 `App.tsx` 注册。详见 `.claude/skills/foundation-i18n/SKILL.md`。
- **设置页子目录隔离**：`SettingsPage/<sub>/` 每个子页面独立目录（含 View / VM / Style / lang），文案命名空间 `settings.<sub>.*`，子页面之间不互相 import。详见 `frontend/CLAUDE.md` 的「设置页：子目录隔离规范」。
- **命名空间提醒**：`theme.palette.foundation.*`、`foundation-light` / `foundation-dark` / `foundation-obsidian` 主题 id、`.claude/skills/foundation-*` 目录名是活代码标识符（沿袭自脚手架时期），**不要**当作残留改名。

## 变更记录 (Changelog)

- 脚手架时期（2026-05-28 系列：foundation 基座 / i18n / 骨架屏 / SQLite 持久化 / 工具层）的演进记录见 git 历史。
- 业务化之后的功能演进按 commit 记录（`git log --oneline`），不再在本文件重复维护。
