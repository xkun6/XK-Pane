# Findings：cc-pane → XK-Pane(foundation) 重构架构盘点与映射

> 本文是规划的权威参考。所有内容来自对本地两个仓库（cc-pane / XK-Pane）的代码探索，均为可信来源。

## A. 源项目 cc-pane 规模速览

Tauri 2 + Rust（Cargo workspace 9 crate）+ React 19 + Zustand + shadcn/ui + Tailwind 4。

| 维度 | 规模 |
|------|------|
| Rust crates | 9（src-tauri / cc-panes-core / cc-panes-api / cc-panes-cli-hook / cc-panes-web / cc-memory / cc-memory-mcp / cc-notify / cc-cli-adapters） |
| Tauri 命令 | 269 个（42 个命令文件） |
| Core services | 32 个 |
| SQLite 表 | 27 张（迁移链 V1–V16） |
| 数据模型 | 21 个 |
| Bedrock MCP 工具 | 26 个 + 13 审查规则 + ~22MB 内嵌数据 |
| CLI 适配器 | 8（claude/codex/gemini/kimi/glm/opencode/cursor/reclaude） |
| 通知渠道 | 5（钉钉/飞书/Slack/Telegram/Webhook） |
| 前端组件 | 142 个 .tsx |
| Zustand stores | 50 个 |
| 前端 services | 37 个 |
| 类型定义 | 27 模块 |
| i18n | zh-CN/en，12 命名空间 |

### crate 职责
- **cc-panes-core**：零框架依赖的领域核心（models/repository/services/pty/utils/events）。**移植主战场**。
- **src-tauri**：薄包装（269 命令注册 + 启动链 + 平台特定 Win32/ObjC + orchestrator HTTP + MCP 进程）。逻辑剥离 `#[tauri::command]` 后转 Go。
- **cc-cli-adapters**：8 个 CLI 的命令拼装（路径字段铁律所在）。
- **cc-memory / cc-memory-mcp**：独立 memory.db + MCP server。
- **cc-notify**：多渠道推送（仅 ureq，无 async）。
- **cc-panes-api / cc-panes-web**：HTTP/WS + Docker 部署 → **本轮延后**。
- **cc-panes-cli-hook**：轻量二进制，PTY 内回调 orchestrator 上报 $PWD（launch_cwd）。

## B. 目标框架 XK-Pane(foundation) 现成能力

Wails v3 + Go 1.25 + React 19 + MUI 9 + Vite 8。

### 后端现成（可直接复用）
- **storage/**：modernc 纯 Go SQLite + GORM + 单连接池 + WAL + AutoMigrate + Holder 热路径切换 + AllModels 注册中心 + dbstat 统计 + IntegrityCheck。
- **utils/procx**：子进程 + **Windows JobObject**（`procx_windows.go`，父崩溃连带 kill 整组）+ Unix setpgid。→ PTY 进程组语义底座。
- **utils/{httpx,cryptox,logx,filex}**：HTTP 单例客户端 / AES-256-GCM(.master.key) / slog+rotate / 原子写。
- **services/**：greet / preferences(KV) / appsettings(单行) / storagesvc(路径管理) / subprocess(白名单)。
- **events/**：`RegisterEvent[T]` 类型化事件中心。
- 平台窗口：`window_{windows,darwin,linux}.go`（build tag）。

### 前端现成（可直接复用）
- **router/**：key-based 无 URL 路由 + keepAlive + RouterOutlet；`routes.tsx` 唯一事实源。
- **components/**：AppLayout（TitleBar 自绘 + Sidebar + Outlet）/ Skeleton（首屏骨架）。
- **styles/themes/**：注册式 light/dark/obsidian + 自定义主题 + `foundation.*` palette（bg/text/divider/accent/status）。
- **i18n/**：注册式公用 locale + 页面级 `lang/` 深合并。
- **services/**：bindings 包装层 + NativeDialogs（原生对话框）。
- **MVVM 规范**：View(.tsx) / ViewModel(use*.ts，不返回 JSX、不 import bindings) / Style(.styles.ts) / lang/ / skeleton。
- **persistence Providers**：Preferences/Theme/i18n 三个异步 Provider（Context 范式，本项目全局状态都照此扩展）。

### 前端依赖基线
React 19.2 / @mui/material 9 / @mui/icons-material 9 / @mui/x-charts 9 / emotion 11 / @wailsio/runtime。**需新增**：`@xterm/xterm`+addons、`monaco-editor`/@monaco-editor/react、`@dnd-kit/*`、`react-markdown`+remark-gfm、CodeMirror（或统一用 Monaco）。

## C. 源→目标 技术栈映射

| cc-pane (源) | XK-Pane (目标) | 备注 |
|--------------|----------------|------|
| Tauri IPC `#[tauri::command]` | Wails Service 方法 + 生成 bindings | 269 命令 → 分域 Go service 方法 |
| Rust `AppResult<T>`/AppError | Go `error` + 前端错误翻译 | |
| rusqlite + 手写迁移 V1–V16 | GORM + AutoMigrate（AllModels 追加） | 零迁移文件 |
| portable-pty | `internal/utils/ptyx`（go-pty/ConPTY）+ procx JobObject | **D3，最大风险** |
| tokio async | goroutine + channel | |
| axum HTTP / rmcp MCP | net/http 或 chi + Go MCP SDK | D5 |
| EventEmitter trait | foundation `events` RegisterEvent + app.Event.Emit | |
| include_dir!(22MB) | `embed.FS` | Bedrock |
| reqwest/ureq | `httpx` | |
| keyring | zalando/go-keyring 或 cryptox 落库 | SSH 凭据 |
| Zustand+Immer (50 store) | React Context + Provider（每域一个） | **D2** |
| shadcn/ui + Radix + Tailwind 4 | MUI 9 + sx + emotion | 全量替换 |
| lucide-react | @mui/icons-material `*Rounded` | Icon 铁律 |
| sonner toast | MUI Snackbar+Alert | |
| Allotment（实际未用，自实现 SplitView） | 保留自实现 SplitView | |
| xterm.js / Monaco / CodeMirror / @dnd-kit | **保留不动**（MUI 无等价物） | D6 |
| i18next/react-i18next | foundation 注册式 i18n | |

## D. 路径字段铁律（D7，移植时必须 1:1 保留）

| 字段 | 含义 | 约束 |
|------|------|------|
| `project_path` | PTY 的 cwd，永远等于用户操作目标（右键对象） | 决定 cwd 只能用它 |
| `workspace_path` | 附加目录上下文，仅多目录 CLI(claude/reclaude/kimi)拼 `--add-dir`，**且仅当 ≠ project_path** | 绝不能用作 cwd |
| `launch_cwd` | 运行时由 cli-hook 上报的 $PWD | resume 时优先用作 path；绝不写回 workspace_path |

守门测试需移植：`terminal_service` + `claude/reclaude` 适配器 + `launchHistory` 前端测试。

## E. SQLite 表（V1–V16）移植清单 → GORM 模型

主库 `data.db`：projects / launch_history / todos / todo_subtasks / specs / terminal_sessions / task_bindings / im_bridge_config / im_task_subscription / im_session_state（im_* 延后）。
独立库：Local History（file_versions / labels / label_snapshots，每项目 `.ccpanes/history.db`）；cc-memory（memory_records / memory_tags / memory_categories，memory.db）。

每张表 → `internal/storage/models_<domain>.go` 的 GORM struct，追加到 `AllModels`，重启 AutoMigrate 建表。敏感字段（provider api_key、ssh 凭据）落库前 `cryptox.EncryptString`。

## F. 前端模块 → MVVM 重写映射（Context 替代 Zustand）

| cc-pane Zustand store（域） | XK-Pane 方案 |
|------|------|
| usePanesStore（面板树/标签/分屏，1601 行） | `TerminalContext` Provider（面板树 reducer）+ 各面板 ViewModel |
| useWorkspacesStore / useProjectsStore | `WorkspaceContext` / `ProjectContext` Provider |
| useSettingsStore / useThemeStore / useShortcutsStore | 复用 foundation Theme/i18n Provider + 新 `SettingsContext`/`ShortcutsContext` |
| useTerminalStatusStore / useResourceStatsStore | `TerminalContext` 子状态 + 事件订阅 hook |
| useTodoStore/useSpecStore/useMemoryStore/useProvidersStore/... | 各域一个 Provider |
| 单组件局部状态 | 组件 ViewModel hook（useState/useReducer） |

前端页面（routes.tsx 追加）：Home / Terminal(panes) / Todo / SelfChat / Providers / Files(editor) / Settings。重型面板（Local History/Git/MCPE/Bedrock）作为页面或 Dialog。

保留组件：TerminalView(xterm)、SplitView、TabBar(@dnd-kit)、JsonEditor(CodeMirror)/Monaco、MarkdownPreview(react-markdown)。

## G. 关键风险与待验证

| 风险 | 说明 | 缓解 |
|------|------|------|
| **R1 PTY 跨平台** | Windows ConPTY 与 Unix PTY 差异；go 生态 PTY 库成熟度 | Phase 1 先做 ptyx spike，验证 go-pty 在 Win11 ConPTY + Wails webview 下可用 |
| **R2 MCP server (Go)** | Go MCP SDK 成熟度、StreamableHttp 支持 | Phase 5 前 spike；必要时手写 SSE/JSON-RPC |
| **R3 Bedrock 数据** | 22MB embed.FS 启动/内存；索引懒加载移植 | 沿用 OnceLock→sync.Once 懒加载；embed 不入内存直到首次查询 |
| **R4 Context re-render** | 50 store 改 Context，重状态易触发大范围 re-render | 拆细 Provider + useMemo/selector hook + React.memo |
| **R5 Git 纯 Go 限制** | go-git 不全（worktree/部分 stash） | 缺口走 subprocess 白名单调 git CLI |
| **R6 改名连锁** | bindings 前缀 = module 名，漏改即编译失败 | Phase 0 一次性按 README 12 步走完并验证 |
| **R7 MC PE Win32** | 包管理/热重载依赖 Windows API 与游戏进程 | Windows build tag；非 Win 平台降级 |

## H. 工作量直觉（粗）

核心+常用增强+早期子系统、全新 MVVM 重写，量级约 **数周到 2 个月**（单人）。Phase 2 终端闭环是第一个可演示里程碑；Phase 5 子系统（尤其 Bedrock/MCPE/MCP server）是后段重头。

## I. 关键源文件索引（移植时优先读）

- cc-pane 后端：`cc-panes-core/src/services/terminal_service.rs`、`cc-panes-core/src/pty/mod.rs`、`cc-panes-core/src/repository/db.rs`、`cc-cli-adapters/src/{claude,reclaude}.rs`、`src-tauri/src/lib.rs`(启动链)、`cc-panes-core/src/services/bedrock_query/service.rs`、`src-tauri/src/services/orchestrator_*.rs`。
- cc-pane 文档：`docs/project-analysis.md`、`docs/bedrock-query-mcp.md`、`docs/17-mc-pe-integration.md`、`docs/18-mcp-architecture.md`、`CLAUDE.md`(路径铁律)。
- XK-Pane 基座：`internal/app/app.go`、`internal/utils/procx/procx_windows.go`、`internal/storage/{db,holder,models}.go`、`internal/services/subprocess/{subprocess,commands}.go`、`frontend/src/{routes.tsx,router,components/AppLayout}`、`frontend/CLAUDE.md`、`.claude/skills/foundation-*`。

---

## J. 尽调结论（已验证 / workflow `wf_5345a74a-b7e`，2026-05-29）

> 完整 7 份报告原文见 transcript：`...\tasks\waubq18nb.output`。以下为锁定决策 + 关键纠错。

### J1. 库选型（已定）
| 关注点 | 选定 | 关键约束 |
|------|------|---------|
| **PTY** | `github.com/aymanbagabas/go-pty`（v0.2.3，纯 Go，真 ConPTY+Unix PTY，活跃维护，go.mod=go1.25） | ① `Resize(width,height)`=`(cols,rows)`，门面里固定参数名注释；② **Unix 它强制 `Setsid=true`，与 procx 的 `Setpgid` 互斥**——Unix 侧不能叠 procx setpgid 分支，kill 改 `syscall.Kill(-pid, sig)`（子进程自身即组长）；③ **Windows JobObject 要自己绑**：传 `CreationFlags=CREATE_SUSPENDED`，Start 后 `OpenProcess→AssignProcessToJobObject→ResumeThread`（平移 `procx_windows.go:33-113`）；④ 关闭顺序：kill→`Cmd.Wait()`(非零退出是 `*exec.ExitError`)→`Pty.Close()`，否则丢尾部输出；⑤ ConPTY 0 尺寸竞态：fit 后首帧立刻 Resize 一次 |
| **MCP server** | 官方 `github.com/modelcontextprotocol/go-sdk@v1.6.1` | 泛型 `mcp.AddTool[In,Out]` 自动从 struct tag 推 JSON Schema（26 个 bedrock_* 批量注册样板最少）；`NewStreamableHTTPHandler{Stateless:true}` 是 `http.Handler`，在 app 启动钩子 goroutine 起 `127.0.0.1:<port>`，OnShutdown `srv.Shutdown`。关注 issue #633 长连接卡住，落地前 PoC |
| **Git** | `go-git/v5`（稳定线）为主 + git CLI fallback | 纯 Go 覆盖：status/branch/log/提交间 diff/fetch/push；**必须 fallback CLI**（经 subprocess 白名单+procx）：stash/unstash、worktree 增删查、非 ff 的 pull/merge、工作区未暂存 diff、依赖系统 credential helper 的 push/fetch。**不上 v6-alpha**（worktree 实验包不稳 + 鉴权 API 破坏性变更） |

### J2. ⚠️ 纠错：终端 I/O 必须走 PTY 原始字节流，不是 procx 行回调
尽调里 terminal_service 规格 agent 提议「用 `procx.Spec` 的 `OnStdout/OnStderr` 行回调直接推送」——**这是错的**。Claude Code/codex 是交互式 TUI，需要 **PTY 原始字节流（含 ANSI/光标控制序列）**，行缓冲的 stdout/stderr 会破坏渲染。正确做法：终端 `Service` 包 `internal/utils/ptyx`(go-pty)，一个 reader goroutine 循环 `Read()` 原始字节 → 按 ~16KB/16ms 批量 → `app.Event.Emit("terminal:output", {sessionId,data})` → 前端 xterm.js `write()`。该 agent 报告中除此 I/O 通道外的其余设计（Session 结构、Registry、atomic 状态、跨平台 kill 复用 procx JobObject、launch_cwd 环境变量回路）均可用。procx 只用于**非交互式**子进程（git CLI fallback 等）。

### J3. 数据模型（已得完整 GORM 规格）
迁移链实际到 **V22**（非 V16）。已产出全部表的 GORM struct 草案，分三库：
- **data.db**（主库，storagesvc 管理）：Project / LaunchHistory(V1-15) / Todo(V1-4) / TodoSubtask / Spec(V5) / TerminalSession(V7,9,12-15) / TaskBinding(V8) / ImBridgeConfig(V16) / ImTaskSubscription(V16-17) / MCPackage(V19-21) / MCIncludedDir(V19)
- **history.db**（每项目独立）：FileVersion / HistoryLabel / LabelFileSnapshot
- **memory.db**（可选）：Memory（cc-memory，FTS5 全文搜索 GORM 无原生支持，repository 层走原生 SQL）
- **敏感字段加密**（GORM `BeforeSave`/`AfterFind` + cryptox）：`im_bridge_config.credentials`(强制)、`provider.api_key`、`terminal_sessions.ssh_config`、`launch_history.last_prompt`
- **一次性迁移脚本**：LaunchHistory V9（`resume_session_id ← claude_session_id`）、ImTaskSubscription V17（表结构不兼容，清空或逐行迁移）
- **注意**：cc-pane 用 RFC3339 文本时间戳；建议迁移后统一 `time.Time`，旧数据 ParseTime；SQLite BOOLEAN→INTEGER(0/1)；`PRAGMA foreign_keys=ON`（foundation 已开）

### J4. CLI 适配器（已得移植规格 + 27 守门测试）
8 个适配器 build_command 逻辑已抽取。**易错点**：GLM 可执行名是 **`crush`**（非 glm）；Claude/Reclaude Windows 要解析 npm `.cmd/.bat` shim→`node.exe cli.js`；Codex 用 `-c` dotted-key TOML 覆盖（无 MCP 文件）；env 注入 `CC_PANES_API_PORT/TOKEN/PTY_SESSION_ID`，仅 Claude/Reclaude 移除 `CLAUDECODE`。**27 个守门测试**（claude 9 / codex 6 / reclaude 6 / glm 2 / terminal 1 / web launchHistory 3）必须一并移植，尤其 `add_dir` 三态（workspace≠project 才加 / ==project 跳过 / absent 跳过）与 GLM `--cwd` 永远 project_path。

### J5. 改名清单（已得精确 file:line，待执行前 Grep 复核）
19 处改点已定位（go.mod:1 / main.go:7 / app.go:11-19,64 / window_*.go Title / paths.go:18 / cryptox.go:42 / logx.go:52 / 5 个前端 service 的 @bindings / Taskfile.yml:4 / build/config.yml:8-12 / index.html:7 / package.json:2）。有序执行：改 go.mod→改全部 Go import→改 3 处 AppDirName→`go mod tidy`→改前端 @bindings→`wails3 generate bindings`→`task dev` 验证。agent 对部分 service 文件 import 标了「需实际检查」，**执行前用 Grep 全量复核 `foundation` 与 `@bindings/foundation` 出现处**。
