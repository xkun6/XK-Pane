# P3 交接文档（清空上下文后从这里接手）

> 本文件是「会话状态 + 下一步」的快照。完整计划见 [P3_PLAN.md](./P3_PLAN.md)，逐会话进度见 [progress.md](./progress.md)（末节 = 本阶段），重建事实基线见 [REBUILD_BLUEPRINT.md](./REBUILD_BLUEPRINT.md)，铁律见 [CLAUDE.md](./CLAUDE.md) 与蓝图 §5。
> 总目标：把 `J:\Program\Agent\cc-pane`（Tauri+Rust+React/Zustand）忠实重建到本仓库（Wails v3 + Go 1.25 + React 19 + MUI 9 + foundation MVVM）。

## 当前进度

- **P1 ✅**：终端真跑 Claude / Codex（TUI 兼容层 + 启动菜单 + cliadapters），用户确认。
- **P2 ✅**：删 Todo/Spec/Memory（`dc11388`）+ VSCode 式 Git SCM 面板（`e6d107a`）+ Home 仪表盘（`c47c0a3`）。用户手测通过。
- **P3a ✅（本阶段，用户手测通过）**：Workspace 两层树。6 单元 `d7c54b8..d1213e4` + 文档 `7f5d8b2` + 手测 5 修复 `8e301f6`/`4f9d7b0`/`e895ee0`/`8980da9`/`2279dcf`。逐单元全绿（go test + typecheck + build + 37 vitest）。明细见 progress.md 末节。
  - 交付：`workspaces`+`workspace_projects` 两表 → `internal/services/workspace` → 前端 `WorkspaceProvider` + `components/WorkspaceTree`（折叠树 + 收藏星 + 计数 + 右键菜单 CLI 启动/导入/重命名/别名/移除/复制路径 + @dnd-kit 拖拽排序）+ 通用 `components/TextInputDialog`。旧 `ProjectPanel` 已删。
  - 拍板落地：资源管理器打开=砍掉；排序=@dnd-kit；不生成文件产物 / 不做迁移快照 / resumeId 明文 / 项目软关联（launchProject 用节点 id 作 projectId）/ 新建文本对话框 / 按需扫盘。

## 下一步 = P3b（未开始）

**SessionsView + resume 透传 + 扫盘回填**（XL）。详细骨架见 `P3_PLAN.md` 的「P3b」节。要点（critic 已核实）：

1. **resume 透传链 = 前端 6 处 + 后端 2 处**：
   - 后端（必补 2 处）：`internal/services/terminal/terminal.go` 的 `CreateSpec` 加 `ResumeID`；`resolveLaunch` 的 cctx 字面量加 `ResumeID: spec.ResumeID`。**adapter 层已就绪**（`claude.go`/`codex.go`/`reclaude.go` 的 `if ctx.ResumeID != ""`），不改 adapter。
   - 前端（必补 6 处链路）：`LaunchProjectInput` → `launchProject` 的 addTab opts → `NewTabOptions`(reducer.ts) → `addTab→createTab`(reducer.ts) → `Tab`/`TerminalPaneLeaf`(types.ts/paneTree.ts) 加 `resumeId` → **核心：`TerminalContext` reconcile effect 的 `TerminalService.create` 入参读 `tab.resumeId`**（session 创建的唯一发生点，**不是** TerminalView/useTerminalView）。cliTool 兜底：resume 时不能 'none'。
2. **`terminal:status` store 前置单元**：活跃指示数据源在 XK **不存在**（`TerminalService.onStatus` 已定义但零消费，`useTerminalView` 只订阅 onOutput/onExit）。需新建订阅 `terminal:status`（events.go:42）的状态 store/Context（非事件）+ TerminalView/Context 接订阅，否则活跃指示永不点亮。
3. **扫盘回填**：新建 `internal/services/sessionscan`，扫 `~/.claude/projects/*.jsonl` + `~/.codex/sessions/**/*.jsonl` 解析 session id/cwd/首条 prompt；backfill 到 launch_history（字段基本已齐：`models_ccpane.go` 的 LaunchHistory 有 ResumeSessionID/LastPrompt/WorkspaceName/LaunchCwd/ProviderID）。**LastPrompt 落库必 `cryptox.EncryptString`**（service 层加解密，仿 provider）；**resumeId 明文**。先做「按需扫盘」（前端打开 SessionsView 调一次），轮询 backfill 留后续。
4. **SessionsView 前端**：活跃会话派生（从 `useTerminal().rootPane` 收集 terminal tabs）+ 最近启动分组/折叠/resume + SessionSubList（扫盘历史）+ ResumeDetailPopover。落点建议复用现有 `history` 路由。

> **若用户决定不做 resume**：P3 核心 Workspace 树已交付可用，可转 P4（Local History 忠实化 + 自动捕获 watcher，需 fsnotify）或 P5（终端进阶：内层分屏 / Tab 拖拽 / 回放）。见蓝图 §2。

## 铁律速查（违反即返工，详见 CLAUDE.md / 蓝图 §5）

SQLite-only 持久化（禁前端 localStorage/sessionStorage）；敏感字段（LastPrompt/key/token）落库前 `cryptox.EncryptString`；HTTP 走 `httpx`、子进程 `procx`、日志 `logx`、写配置 `filex`；前端子进程经 `subprocess` 白名单；对话框走 `NativeDialogs`（**应用内文本录入用 `components/TextInputDialog`**，非浏览器 prompt）；图标 `@mui/icons-material *Rounded`（禁 emoji/Unicode/第三方）；人类可见字符串走 `t('key')` + 新组件配 `lang/{zh-CN,en-US}` 并在 App.tsx 注册；颜色取 `theme.palette.foundation.*`（禁硬编码十六进制）；圆角按钮 6 / 容器 8；事件名集中注册 `internal/events`（禁前端自造 window 事件）；平台分支走 build tag 文件。

MVVM：View(无 bindings) → `use<Page>`(无 JSX/无 t()/经 services) → `services/<Domain>Service.ts` → `@bindings/xkpane/...`；导航只用 `useRouter().navigate(id)`；后端业务 service 持 `*storage.Holder` 不 capture `*storage.DB`；加新表把 `&Model{}` 追加 `internal/storage/models.go` 的 `AllModels`（重启自动 AutoMigrate）；改后端方法签名后跑 `wails3 generate bindings`。

提交/推送仅在用户明确要求时；逐单元提交（每个 commit 自洽可编译）；始终用简体中文回复。当前分支 `rebuild/cc-pane`（非 master，可直接提交，不推送）。
