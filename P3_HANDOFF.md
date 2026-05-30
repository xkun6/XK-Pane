# P3 交接文档（清空上下文后从这里接手）

> 本文件是「会话状态 + 下一步」的快照。完整计划见 [P3_PLAN.md](./P3_PLAN.md)，逐会话进度见 [progress.md](./progress.md)（末节 = 本阶段），重建事实基线见 [REBUILD_BLUEPRINT.md](./REBUILD_BLUEPRINT.md)，铁律见 [CLAUDE.md](./CLAUDE.md) 与蓝图 §5。
> 总目标：把 `J:\Program\Agent\cc-pane`（Tauri+Rust+React/Zustand）忠实重建到本仓库（Wails v3 + Go 1.25 + React 19 + MUI 9 + foundation MVVM）。

## 当前进度

- **P1 ✅**：终端真跑 Claude / Codex（TUI 兼容层 + 启动菜单 + cliadapters），用户确认。
- **P2 ✅**：删 Todo/Spec/Memory（`dc11388`）+ VSCode 式 Git SCM 面板（`e6d107a`）+ Home 仪表盘（`c47c0a3`）。用户手测通过。
- **P3a ✅（用户手测通过）**：Workspace 两层树。6 单元 `d7c54b8..d1213e4` + 文档 + 手测 5 修复。交付 `workspaces`+`workspace_projects` 两表 → `internal/services/workspace` → 前端 `WorkspaceProvider` + `components/WorkspaceTree`（折叠树 + 收藏星 + 右键菜单 + @dnd-kit 拖拽）+ `components/TextInputDialog`。明细见 progress.md。
- **P3b ✅（本阶段，待用户手测）**：SessionsView + resume 透传 + 扫盘。4 单元 7 提交 `a776650`/`03f8e92`/`f2777d4`/`d44625c`/`d3c12b0`，逐单元全绿（go vet+test / typecheck / 48 vitest / build）。明细见 progress.md 末节。
  - **用户拍板**：① SessionsView 落点 = **复用 history 路由**（删 Local History 页，P4 重做）；② **跟 cc-pane 保持一致**（忠实两面 + WorkspaceItem 不加活跃点）；WSL/SSH 全程跳过（Windows 桌面优先）。
  - 交付：resume 透传链（后端 `CreateSpec.ResumeID`+resolveLaunch / 前端 6 处链路核心是 reconcile effect）；`internal/services/sessionscan`（纯 FS 扫盘 claude+codex，无 DB）；`launchhistory` resume 字段 + LastPrompt cryptox + Delete/Clear；`TerminalStatusContext`（terminal:status store，活跃指示源）；`pages/SessionsPage`（活跃会话 + 最近启动分组 + ResumeDetailPopover）取代 history 路由；WorkspaceTree 项目节点 chevron 展开 `SessionSubList`（按需扫盘 resume）。
  - **留后续**：轮询 backfill（拍板点 9）/ reveal-in-explorer（拍板点 2）/ WorkspaceItem 活跃点（cc-pane 无）。

## 下一步 = 待用户手测 P3b → 然后 P4 或 P5

**先等用户 `wails3 dev` 目检 P3b**（见 progress.md 末节验收清单）：Sessions 页活跃会话/分组折叠/点行 resume（看进程 args `claude --resume`/`codex resume`）/ Popover 复制 id；项目节点 chevron 展开扫盘列表 + 右键 resume。手测发现 bug 当场修（仿 P3a 手测 5 修复节奏）。

手测通过后按蓝图 §2 推进（与用户确认优先级）：
- **P4 Local History 忠实化**：当前 `localhistorysvc` 后端在用（git auto-label），但 UI 已被 SessionsView 取代 → P4 重做 Local History UI（落点待定，可新路由）+ 自动捕获 watcher（需 fsnotify）。
- **P5 终端进阶**：内层分屏（splitTerminalPane）/ Tab 拖拽（moveTab，注意 reconcile 的 setTabSession 跨 pane TODO）/ 回放。

> P3 核心（Workspace 树 + 会话恢复）已全部交付可用。

## 铁律速查（违反即返工，详见 CLAUDE.md / 蓝图 §5）

SQLite-only 持久化（禁前端 localStorage/sessionStorage）；敏感字段（LastPrompt/key/token）落库前 `cryptox.EncryptString`；HTTP 走 `httpx`、子进程 `procx`、日志 `logx`、写配置 `filex`；前端子进程经 `subprocess` 白名单；对话框走 `NativeDialogs`（**应用内文本录入用 `components/TextInputDialog`**，非浏览器 prompt）；图标 `@mui/icons-material *Rounded`（禁 emoji/Unicode/第三方）；人类可见字符串走 `t('key')` + 新组件配 `lang/{zh-CN,en-US}` 并在 App.tsx 注册；颜色取 `theme.palette.foundation.*`（禁硬编码十六进制）；圆角按钮 6 / 容器 8；事件名集中注册 `internal/events`（禁前端自造 window 事件）；平台分支走 build tag 文件。

MVVM：View(无 bindings) → `use<Page>`(无 JSX/无 t()/经 services) → `services/<Domain>Service.ts` → `@bindings/xkpane/...`；导航只用 `useRouter().navigate(id)`；后端业务 service 持 `*storage.Holder` 不 capture `*storage.DB`；加新表把 `&Model{}` 追加 `internal/storage/models.go` 的 `AllModels`（重启自动 AutoMigrate）；改后端方法签名后跑 `wails3 generate bindings`。

提交/推送仅在用户明确要求时；逐单元提交（每个 commit 自洽可编译）；始终用简体中文回复。当前分支 `rebuild/cc-pane`（非 master，可直接提交，不推送）。
