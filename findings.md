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

### J6. Phase 2 后端 terminal_service 落地记（保真度 + 两处待清理）
- **保真度（对照 `cc-panes-core/src/services/terminal_service.rs` + `terminal_service/shell.rs`）**：
  - `defaultShell` 忠实移植 `resolve_default_shell`（Win `pwsh>powershell>%ComSpec%` / Unix `$SHELL` else `/bin/sh`）。`detect_shells`/`resolve_shell`（按 shell_id 选 git-bash/wsl/zsh/fish 等）属用户可配项 → **Phase 3 设置页**再补。
  - `buildEnv` 对齐 cc-pane 1147-1204 env 注入：TERM/COLORTERM(or_insert)、CC_PANES_PTY_SESSION_ID(强制) 一致；snapshot_id/launch_id/runtime_kind/orchestrator(CC_PANES_API_*)/wsl_distro/launch_profile_skills 均 Phase 3/4/5 特性，MVP 延后并标 TODO。
- **微差异（良性，有意保留）**：cc-pane 对裸 shell 也注入 `CC_PANES_CLI_TOOL=none`（`cli_tool.as_id()`），本实现对 "none"/"" 跳过。无害——该 env 仅供 cli-hook 上报 launch_cwd（Phase 5），裸 shell 无 hook 消费方。
- **⚠️ 待 Phase 6 统一清理**：`AttachApp(app *application.App)` 会被 `wails3 generate bindings` 暴露给前端（terminal 与 **既有 subprocess 服务**皆如此，subprocess 连 `EventNamePrefix` 也漏）。属 foundation 脚手架既定模式，无害（前端无法构造 `*application.App`）。Phase 6 可调研 Wails v3「Internal/隐藏方法」机制对两个服务一并隐藏 app-wiring 方法。
- **环境坑**：`go mod tidy` 经 goproxy.cn 拉取 go-pty 的深层 test 依赖（u-root/gobusybox）偶发 504；**与改动无关**，build/test 不受影响。后果仅 uuid 在 go.mod 仍标 `// indirect`（陈旧无害，下次联网 tidy 自动转直接依赖）。

### J7. 前端面板树模型（两级嵌套分屏，移植自 cc-pane `web/types/{pane,terminal}.ts` + `paneTreeHelpers.ts`）
- **关键结构**：cc-pane 终端布局是**两层独立的分屏树**，移植时务必都建模：
  - **外层 `PaneNode`（窗口布局）** = `Panel`（`type:"panel"`，含 `tabs: Tab[]` + `activeTabId`）| `SplitPane`（`type:"split"`，`direction:"horizontal"|"vertical"` + `children:PaneNode[]` + `sizes:number[]`）。`SplitDirection="right"|"down"` 是**操作**语义（向右/向下拆）。
  - **内层 `TerminalPaneNode`（单个 Tab 内）** = `TerminalPaneLeaf`（`type:"leaf"`，持 `sessionId:string|null`）| `TerminalPaneSplit`（同 split 结构）。即**每个 Tab 自身又是一棵终端分屏树**：`Tab.terminalRootPane` + `Tab.activeTerminalPaneId`。
  - 含义：用户既能把窗口拆成多个 Panel（每个 Panel 一排 tab），又能在**单个 Tab 内**把终端再拆分。两个 SplitView 渲染层（外层渲染 PaneNode，TabContent 内渲染 terminalRootPane）。
- **MVP 落点（会话 4，commit 见 progress）**：`frontend/src/terminal/{types.ts,paneTree.ts}` 移植两级类型 + 外层纯 helpers（findPane/findParent/collectPanels/createPanel/createTab/generateId）。**裁剪** provider/resume/ssh/wsl/snapshot/launch-profile/非 terminal contentType/pinned 等 Phase 3/4/5 字段（types.ts 文件头列明）。`createTab` 可选参换成 MVP 的 `{cliTool,workspacePath,title}`。
- **下一步（TerminalContext）需补的 usePanesStore 关键逻辑**（1743 行里挑 MVP 子集）：split（在目标 Panel 外包一层 SplitPane）、close（移除节点后**折叠只剩单子的 split**——回退成单 Panel/单 leaf，避免空壳 split 堆积）、activate（setActivePanel/setActiveTab）、内层 terminalRootPane 的对应操作。这些是树操作的 bug 高发区，建议 Workflow 多视角对抗验证 + 考虑引入 Vitest 单测纯树函数。

### J8. Project / Workspace 后端模型（已验证 / workflow `wf_79a917da-67a`，2026-05-30）

> 完整三份调查 + 综合规格原文见 transcript task 输出 `...\tasks\wbk3e0q5k.output`。Phase 3 第一单元（后端 `internal/services/project`）据此落地。

- **cc-pane 有「两套并行的 project 概念」，只有一套被 live UI 消费**：
  1. **扁平 Project**（`projects` 表，ProjectService/ProjectRepository/project_commands）——全局「已知目录登记表」。**已移植**为 `internal/services/project`（本单元）。其前端 `projectService.ts`/`useProjectsStore` 在 cc-pane 里**已包装但 live 侧边栏不消费**（仅自带单测）。
  2. **WorkspaceProject**（内嵌在 workspace.json 的 `projects[]` 数组里，`{id,path,alias?,launchProfileId?,wslRemotePath?,ssh?}`）——cc-pane **live 侧边栏实际渲染的就是这套**（ExplorerView→WorkspaceTree→WorkspaceItem→ProjectListView，按 workspace 分组）。
- **扁平 Project 保真铁律**（本单元已逐字锁，对应守门测试）：path **原样存储绝不规范化**、去重逐字节相等、Add 非 upsert（"Project already exists"）、List `created_at DESC`、alias trim→NULL、name=basename（"unnamed" 回退）、UpdateName 存未 trim 值（与 alias 存 trim 值非对称）、id-keyed、affected-rows==0→"Project does not exist"、validate_path 结构化码 PATH_EMPTY/TRAVERSAL/NOT_ABSOLUTE。**XK-Pane 原生差异（非 drift）**：`time.Time` created_at + 自动 UpdatedAt（替 RFC3339 文本列，models_ccpane.go 头已声明）。
- **⚠️ Workspace 是「文件存储」不是 SQLite 表（决策待定，见下）**：cc-pane 的 workspace **没有 `workspaces` 表**——它是文件实体 `<data_dir>/workspaces/<name>/workspace.json`（目录名=身份，rename=`fs::rename`），由 `WorkspaceService` 纯文件 I/O 管理（list 遍历目录 / create mkdir+写 json / 读改写 json + notify 文件监视发 `WORKSPACES_CHANGED` + 迁移快照）。data.db 只在 launch_history / terminal_sessions 上存 `workspace_name`/`workspace_path`/`workspace_snapshot_id` 的**反规范化快照串**，从不存 workspace 定义本身。另有 `WorkspaceSnapshot`（布局/面板快照，`workspace_snapshot_id` 指它，≠ workspace 定义）。
- **🚧 待定决策 D9（Workspace 持久化模型，决策门）**：忠实移植 = 文件存储（workspace.json + 文件监视），但**违背 XK-Pane「跨会话状态只能走 SQLite」铁律**；XK-Pane 原生 = 加 `Workspace`+`WorkspaceProject` GORM 表。本次是**全新重构**（不迁移 cc-pane 旧数据），倾向 SQLite 原生，但这是**显著架构选择 + 触及铁律**，**到 workspace 单元时停在决策门询问用户**，不静默定。**当前不阻塞**：近期 Phase 3 单元（前端扁平 project service/Context/侧边栏树、launch_history、会话恢复）都不依赖它。
- **MVP 侧边栏路径选择**：先做**扁平 project 侧边栏**（顶层项目列表，复用本单元后端，替换临时「打开文件夹」工具条）——比 cc-pane 的 workspace 分组树简单且更快可用；workspace 分组作为后续增强（届时触 D9 决策门）。

### J9. Provider 后端模型 + 移植决策（已验证 / workflow `wf_a31b9406-129`，2026-05-30）

> 完整三份调查 + 综合规格见 task 输出 `...\tasks\wy9bqvhx8.output`。Phase 3 provider 后端单元据此落地（commit `c86bc60`）。

- **cc-pane 现状**：providers 存**明文 `providers.json` 文件**（无 SQLite 表、无加密、无 keyring；keyring 仅用于 SSH 凭据）。字段：id（前端供，不校验唯一）、name、providerType（snake_case：anthropic/bedrock/vertex/proxy/config_profile/open_ai/gemini/kimi/glm/cursor/opencode；**无 model 字段**）、apiKey?、baseUrl?、region?、projectId?、awsProfile?、configDir?、isDefault。default 不变式（恰一个默认）。前端把**明文整条**回前端。`to_env_vars()` 按 type 映射启动 env（ANTHROPIC_API_KEY/BASE_URL 等）。
- **XK-Pane 移植决策（5 项偏离，均安全/一致性加固，与 D8 对齐；已实现并 15 测锁；用户可否决）**：
  1. **持久化**：SQLite `providers` 表 + **cryptox 加密 apiKey**（D8 铁律，非明文文件）。**service 层加解密**（非 GORM 钩子）——读返回脱敏 DTO 无需解密、写为 patch 保留；storage 不引 cryptox。`Provider.APIKey` 字段**恒为密文**（json:"-"），消费方读出须 DecryptString。
  2. **脱敏 DTO**：List/Get 返回 `ProviderDTO` 不含明文 key（仅 `hasApiKey`）——**明文绝不跨 IPC**（cc-pane 每次 list 都回明文，泄漏面大）。
  3. **Update = patch**：apiKey 空则保留原密钥；且 **Update 不动 is_default**（默认只由 Add/SetDefault/Remove 管）——避免编辑误清默认。无需 GetSecret（编辑表单不回填 key，提示「留空保持不变」）。
  4. **id 服务端生成 UUID**（同 project；避 cc-pane 前端供 id 不去重的撞 id）。Add 返回创建出的 DTO。
  5. **SetDefault 未知 id 报错**（偏 cc-pane 静默留零默认；与本仓 not-found 约定一致）。
- **default 提升用 `ORDER BY rowid`（插入序）**：等价 cc-pane providers[0]；**不可用 created_at**——同 tick 多次 Add 时间戳撞、字典序非确定（**实测**：created_at 版在 prod test 偶发倒挂，rowid 版 3×dev+3×prod 稳定）。同教训见 storage UTC 修复。
- **延后到「启动接线」单元（非本单元）**：`GetEnvVars`（to_env_vars 中央映射，11 type→env）+ `ToCliProvider` 桥接——**它们暴露明文 env，绝不能进 Wails bindings**，做成非绑定 helper 由 terminal 启动路径直接调。届时还需：`terminal.resolveLaunch` 设 `cctx.Provider` + `buildEnv` 合 provider env（cc-pane 中央 Tier-A 当前 XK-Pane 完全缺失，claude/codex/gemini 等拿不到 ANTHROPIC_*/CODEX_* 等）。Tier-B（glm/cursor/kimi 自注入）已在 cliadapters 移植就位。
- **codex 易错点**：OpenAI(codex)→`CODEX_API_KEY`+`OPENAI_BASE_URL`（**非** OPENAI_API_KEY）；OpenCode→`OPENAI_API_KEY`+`OPENAI_BASE_URL`。移植 GetEnvVars 时注意。
