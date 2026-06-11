# XK-Pane

[![Wails v3](https://img.shields.io/badge/Wails-v3.0.0--alpha-red)](https://v3.wails.io)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![React](https://img.shields.io/badge/React-19.2-61DAFB?logo=react)](https://react.dev)
[![MUI](https://img.shields.io/badge/MUI-9.0-007FFF?logo=mui)](https://mui.com)
[![TypeScript](https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript)](https://www.typescriptlang.org)

**XK-Pane** 是基于 Wails v3（Go + React 19 + MUI 9）的桌面工作台：以多面板终端为核心，
编排 claude / codex 等 CLI 会话，配套工作区 / 项目管理、本地文件历史、Todo 分派、
SSH 连接中心与 MC PE（Minecraft 中国版）调试桥。

---

## 目录

- [XK-Pane](#xk-pane)
  - [目录](#目录)
  - [✨ 特性一览](#-特性一览)
  - [🛠 安装 Wails CLI](#-安装-wails-cli)
    - [系统要求](#系统要求)
    - [必装依赖](#必装依赖)
    - [安装 wails3 CLI](#安装-wails3-cli)
    - [验证环境](#验证环境)
  - [🚀 快速开始](#-快速开始)
    - [1. 装依赖](#1-装依赖)
    - [2. 生成 bindings](#2-生成-bindings)
    - [3. 跑起来](#3-跑起来)
  - [📂 项目结构](#-项目结构)
  - [⌨️ 常用命令速查](#️-常用命令速查)
  - [📐 工程规范](#-工程规范)
  - [🧩 加新功能：常见路径](#-加新功能常见路径)
    - [加一个页面](#加一个页面)
    - [加一张数据表](#加一张数据表)
    - [加一个后端 service](#加一个后端-service)
    - [发起 HTTP 请求](#发起-http-请求)
    - [启动子进程](#启动子进程)
  - [📚 文档](#-文档)
  - [📜 License](#-license)

---

## ✨ 特性一览

| 类别 | 内容 |
|-----|------|
| **窗口** | Windows/Linux 完全无边框（前端自绘标题栏 + 窗口控制按钮）；macOS 沉浸式标题栏 + 系统红绿灯 |
| **布局** | 左侧 64px Sidebar（自动从路由表渲染）+ 自定义 TitleBar + 主内容区，方圆设计语言（按钮 12 / 容器 8） |
| **路由** | 唯一事实源 `routes.tsx` + `<RouterOutlet />` + `useRouter()`；支持 `keepAlive` 页面缓存 |
| **主题** | `foundation-light` / `foundation-dark` / `foundation-obsidian` + 用户自定义调色板（17 个语义槽位） |
| **i18n** | `zh-CN` / `en-US`（默认 zh-CN）；`'auto'` 跟随浏览器；公用 + 页面级语言包深合并 |
| **首屏优化** | `index.html` 内联静态骨架（消除白屏）+ React 骨架组件 + lazy import + Suspense |
| **持久化** | SQLite + GORM AutoMigrate；用户可在「设置 → 数据存储」改数据库位置（原生「另存为」对话框） |
| **数据可视化** | 设置页内置环形 + 条形图（@mui/x-charts）展示各表占用 + 逐表清空 |
| **工具层** | `httpx` HTTP / `procx` 子进程（Windows JobObject）/ `cryptox` AES-GCM / `logx` slog+rotate / `filex` 原子写 |
| **原生对话框** | `NativeDialogs` 服务封装 Wails v3 `Dialogs.*`，禁用浏览器 `alert/confirm/prompt` |

---

## 🛠 安装 Wails CLI

### 系统要求

| 平台 | 版本 |
|------|------|
| Windows | 10 / 11（AMD64 或 ARM64） |
| macOS | 10.15+（开发）/ 部署到 10.13+ |
| Linux | Ubuntu 24.04 AMD64/ARM64（其他发行版亦可） |

### 必装依赖

| 依赖 | 版本 | 验证 / 安装 |
|------|------|------------|
| **Go** | ≥ 1.25 | `go version` |
| **Node.js** | ≥ 20 | `node --version` |
| **pnpm** | latest | `npm i -g pnpm` |
| **task** (go-task) | latest | `go install github.com/go-task/task/v3/cmd/task@latest` |
| **WebView2 Runtime** | Windows 必装 | Win10 ≥1803 内置；老系统从 [Microsoft 下载](https://developer.microsoft.com/microsoft-edge/webview2/) |
| **Xcode CLT** | macOS 必装 | `xcode-select --install` |
| **gcc + gtk4 + webkit2gtk-6.0** | Linux 必装 | `wails3 doctor` 会给出本机系统的精确安装命令 |

> Linux 旧栈用户：可以加 `-tags gtk3` 切回 GTK3 + WebKit2GTK 4.1（v3.1 之前保留兼容）。

### 安装 wails3 CLI

```sh
go install -v github.com/wailsapp/wails/v3/cmd/wails3@latest
```

确保 `$GOPATH/bin` 或 `$HOME/go/bin` 在 PATH 里：

```sh
# 临时
export PATH=$PATH:$(go env GOPATH)/bin

# 永久（bash/zsh）
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc

# Windows PowerShell：去"环境变量"图形界面把 %USERPROFILE%\go\bin 加进 PATH
```

### 验证环境

```sh
wails3 doctor
```

输出会列出所有检测项 + 缺失依赖的安装提示。**全绿之后再继续。**

---

## 🚀 快速开始

### 1. 装依赖

```sh
# Go 依赖
go mod tidy

# 前端依赖
cd frontend && pnpm install && cd ..
```

### 2. 生成 bindings

```sh
wails3 generate bindings
```

> bindings 是 Wails 自动生成的前端 ↔ 后端 RPC 胶水代码，路径在 `frontend/bindings/xkpane/...`。**改了 service 签名 / 事件注册后必须重新生成。** 已加入 `.gitignore`。

### 3. 跑起来

```sh
# 开发（前后端热重载，Vite dev server 默认端口 9245）
wails3 dev
# 等价：task dev

# 生产构建（当前平台）
task build

# 打包
task package
```

应用首次启动会在用户数据目录建立 `xkpane.db`（dev 构建目录带 `-Dev` 后缀，与 release 隔离）：

| 平台 | 默认数据库位置 |
|------|---------------|
| Windows | `%AppData%\XK-Pane\xkpane.db` |
| macOS | `~/Library/Application Support/XK-Pane/xkpane.db` |
| Linux | `~/.local/share/XK-Pane/xkpane.db` |

可在应用内「设置 → 数据存储」修改位置（原生「另存为」对话框，自动迁移现有数据）。

---

## 📂 项目结构

```
XK-Pane/
├── main.go                            # 仅 embed.FS + app.Run；不接受业务代码
├── go.mod                             # module xkpane
├── Taskfile.yml                       # 顶层任务（dev/build/package）
│
├── internal/                          # 后端业务（package private）
│   ├── app/                           # 启动装配 + 平台窗口（window_<os>.go build tag）
│   ├── events/events.go               # 集中注册类型化事件
│   ├── services/                      # Wails Service，30+ 领域子包（terminal / workspace /
│   │                                  #   orchestrator / localhistorysvc / mcpe / sshmachine /
│   │                                  #   todosvc / preferences / appsettings / storagesvc / …）
│   ├── storage/                       # SQLite 持久化层（Open/Holder/PRAGMA/AllModels）
│   ├── webapi/                        # Web 镜像（viewer）HTTP 端点
│   └── utils/                         # 跨业务工具层
│       ├── httpx/                     # HTTP 客户端 + JSON helper + 代理 client
│       ├── procx/                     # 子进程 + Windows JobObject
│       ├── ptyx/ gitx/                # PTY 封装 / go-git 封装
│       ├── cryptox/                   # AES-GCM 对称加密
│       ├── logx/                      # slog + 控制台 + 文件 rotate
│       └── filex/                     # 原子写
│
├── frontend/                          # React 19 + MUI 9 + Vite 8
│   ├── index.html                     # 内联首屏骨架（消除白屏）
│   ├── src/
│   │   ├── App.tsx                    # Provider 链装配
│   │   ├── routes.tsx                 # 路由表（唯一事实源）
│   │   ├── components/                # AppLayout / TitleBar / Sidebar / 终端与树等公共组件
│   │   ├── pages/                     # Terminal / Home / Todo / Files / Git / History /
│   │   │                              #   Ssh / Mcpe / Settings（页面 = View+VM+Style+lang）
│   │   ├── router/ services/ styles/ i18n/ preferences/ shared/
│   │   └── main.tsx
│   ├── bindings/                      # wails3 自动生成（gitignore）
│   └── package.json
│
├── build/                             # 多平台构建配置 + 资源（windows/darwin/linux/docker）
├── .claude/skills/                    # wails-*（Wails v3 文档）+ foundation-*（基建指南）
│                                      #   + xkpane-*（业务 skill）
├── CLAUDE.md                          # 项目级 AI 协作约定
└── README.md                          # 本文
```

---

## ⌨️ 常用命令速查

| 命令 | 作用 |
|------|------|
| `wails3 dev` | 前后端热重载开发模式 |
| `wails3 build` | 直接编译当前平台二进制（无打包） |
| `wails3 generate bindings` | 重新生成 `frontend/bindings/` |
| `wails3 doctor` | 检测开发环境依赖 |
| `task dev` | = `wails3 dev` |
| `task build` | 当前平台生产构建 |
| `task package` | 当前平台打包（含安装包） |
| `task run` | 跑已构建的二进制 |
| `task <os>:build` | 指定平台构建（windows/darwin/linux） |
| `task build:server` | 服务器模式（无 GUI，HTTP 后端） |
| `cd frontend && pnpm typecheck` | 单独跑前端类型检查（不出文件） |
| `cd frontend && pnpm build` | 单独构建前端（产物到 `frontend/dist`） |
| `go build .` | 单独构建后端（需要 `frontend/dist` 已存在） |
| `go vet ./...` | 后端静态检查 |

---

## 📐 工程规范

下面这些规范在 `CLAUDE.md` 与 `frontend/CLAUDE.md` 里都有详细说明。简要列出：

| 类别 | 铁律 |
|------|------|
| **MVVM** | View 不写业务（只消费 ViewModel）；ViewModel 不返回 JSX、不直接 import bindings；Service 层包装 bindings |
| **i18n** | 任何人类可见字符串必须经 `t()`（含 `aria-label` / `placeholder` / 对话框文案） |
| **持久化** | 跨会话状态只走 SQLite（`PreferencesService` / `AppSettingsService` / 自定义 service）；**禁止** `localStorage` / `sessionStorage` |
| **业务 service** | 持有 `*storage.Holder`，**不要** capture `*storage.DB`（切换路径会替换） |
| **HTTP** | **禁止** `net/http.DefaultClient`；用 `httpx.GetJSON / PostJSON / Do` |
| **子进程** | **禁止** `os/exec.Cmd.Start`；用 `procx.StartCtx`（自动绑 JobObject / pgid） |
| **加密** | 敏感字符串落 SQLite 必须 `cryptox.EncryptString` |
| **日志** | **禁止** `log.Printf` / `fmt.Println`；用 `logx.For("component").Info(...)` |
| **配置文件** | **禁止** `os.WriteFile` 写配置；用 `filex.WriteAtomic` |
| **原生对话框** | 文件 / 文件夹 / 确认 / 错误 提示必须走 `NativeDialogs`，**禁止** `alert/confirm/prompt` |
| **Icon** | UI 图标必须用 `@mui/icons-material` 的 React 组件（推荐 `*Rounded` 系列）；**禁止** emoji / Unicode / 第三方 icon 包 |
| **样式取色** | 永远从 `theme.palette.foundation.*` 取；圆角语言：按钮 12 / 容器 8 |
| **平台分支** | 走 `internal/app/window_<os>.go` 的 build tag 文件，**禁止** `runtime.GOOS` 判断 |

---

## 🧩 加新功能：常见路径

### 加一个页面

详见 `frontend/CLAUDE.md` 的「路由规范」。

```ts
// 1. 写页面：frontend/src/pages/AccountPage/{AccountPage.tsx, AccountPage.styles.ts, AccountPage.skeleton.tsx}
//    有页面级状态再加 useAccountPage.ts；不建 index.ts 桶
//    若有人类可见文案：再加 lang/{zh-CN.ts, en-US.ts, index.ts} + register<Name>Locales

// 2. 在 src/routes.tsx 追加一项（直 import 具体文件）
import PersonRoundedIcon from '@mui/icons-material/PersonRounded';
import { AccountPage } from '@/pages/AccountPage/AccountPage';
import { AccountPageSkeleton } from '@/pages/AccountPage/AccountPage.skeleton';

export const routes: RouteDefinition[] = [
  // ...
  {
    id: 'account',
    labelKey: 'route.account',
    label: 'Account',
    icon: PersonRoundedIcon,
    element: <AccountPage />,
    fallback: <AccountPageSkeleton />,
    slot: 'primary',
  },
];

// 3. 公用 locale 加 route.account
// 4. App.tsx 调一次 registerAccountPageLocales()
```

### 加一张数据表

详见 `.claude/skills/foundation-persistence/SKILL.md`。

```go
// 1. internal/storage/models.go
type Message struct {
    ID        uint   `gorm:"primaryKey;autoIncrement"`
    Content   string `gorm:"type:text;not null"`
    CreatedAt int64  `gorm:"autoCreateTime:milli;index"`
}

// 2. 同文件 AllModels 数组追加
var AllModels = []ModelDescriptor{
    // ... 已有
    { Model: &Message{}, TableName: "messages", LabelKey: "messages", Clearable: true },
}

// 3. 重启 → AutoMigrate 自动建表 + 设置页双图自动展示
```

### 加一个后端 service

```go
// 1. internal/services/messages/messages.go
type Service struct { holder *storage.Holder }
func New(holder *storage.Holder) *Service { return &Service{holder: holder} }
func (s *Service) Send(ctx context.Context, content string) error { /* ... */ }

// 2. internal/app/app.go 注册
Services: []application.Service{
    // ... 已有
    application.NewService(messages.New(holder)),
},

// 3. wails3 generate bindings

// 4. frontend/src/services/messages/MessagesService.ts 包装
import { Service as Binding } from '@bindings/xkpane/internal/services/messages';
export const MessagesService = {
    async send(content: string) { return Binding.Send(content); },
};
```

### 发起 HTTP 请求

详见 `.claude/skills/foundation-utils/SKILL.md`。

```go
import "xkpane/internal/utils/httpx"

var resp WeatherResp
if err := httpx.GetJSON(ctx, url, &resp); err != nil {
    if he, ok := httpx.IsHTTPError(err); ok && he.StatusCode == 404 {
        return ErrNotFound
    }
    return err
}
```

### 启动子进程

```go
import "xkpane/internal/utils/procx"

err := procx.Run(ctx, procx.Spec{
    Name: "ffmpeg",
    Args: []string{"-i", input, output},
    CaptureOutput: true,
    OnStdout: func(line string) { logx.For("ffmpeg").Debug(line) },
})
// 父进程崩溃时子进程自动连带 kill（Windows JobObject / Unix pgid）
```

要让前端能调子进程，必须经白名单 service（原 `internal/services/subprocess` 实现已删，需要时按 git 历史恢复），禁止开万能 `RunCommand`。

---

## 📚 文档

| 文档 | 内容 |
|------|------|
| [CLAUDE.md](./CLAUDE.md) | 项目级 AI 协作约定（架构概览 + 工程规范汇总） |
| [frontend/CLAUDE.md](./frontend/CLAUDE.md) | 前端模块详细规范（MVVM / 路由 / 持久化 / 原生对话框 / Icon / 子目录隔离） |
| [frontend/DEVELOPMENT.md](./frontend/DEVELOPMENT.md) | 前端 MVVM 落地细则 |
| [build/CLAUDE.md](./build/CLAUDE.md) | 多平台构建配置说明 |
| `.claude/skills/wails-*/` | Wails v3 官方文档（按主题拆分的 12 个 skill） |
| `.claude/skills/foundation-theme/` | 主题系统使用指南 |
| `.claude/skills/foundation-i18n/` | i18n 使用指南 |
| `.claude/skills/foundation-persistence/` | 持久化层使用指南 |
| `.claude/skills/foundation-utils/` | 工具层使用指南（httpx/procx/cryptox/logx/filex） |
| [Wails v3 官网](https://v3.wails.io) | 上游文档 |
| [MUI 官网](https://mui.com) | UI 组件库 |

---

## 📜 License

MIT
