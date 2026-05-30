# P4 交接文档（清空上下文后从这里接手）

> 本文件 = 「会话状态 + 下一步」快照。完整计划见 [P4_PLAN.md](./P4_PLAN.md)（带 cc-pane / XK 双侧 file:line + 单元切分 + 拍板点），逐会话进度见 [progress.md](./progress.md)（末节），重建事实基线见 [REBUILD_BLUEPRINT.md](./REBUILD_BLUEPRINT.md)（P4 行在 §2、铁律在 §5），铁律见 [CLAUDE.md](./CLAUDE.md)。
> 总目标：把 `J:\Program\Agent\cc-pane`（Tauri+Rust+React）忠实重建到本仓库（Wails v3 + Go 1.25 + React 19 + MUI 9 + foundation MVVM）。

## 当前进度

- **P1 ✅**：终端真跑 Claude/Codex（TUI 兼容层 + 启动菜单 + cliadapters），用户确认。
- **P2 ✅**：删 Todo/Spec/Memory + VSCode 式 Git SCM 面板 + Home 仪表盘。用户手测通过。
- **P3 ✅**：Workspace 两层树（P3a，用户手测通过）+ 会话恢复（P3b）。**会话恢复最终形态**（用户手测后拍板回退）：**仅工作空间树项目节点 chevron 的按需扫盘子列表**（SessionSubList，左键恢复/右键 恢复·以 reclaude 恢复·复制 id → `launchProject(resumeId)` → CLI resume）。独立 Sessions 页 + status store + launchhistory resume 附加均已撤；`history` 导航位已移除。保留 P3b-1 resume 透传链 + `sessionscan` 后端 + `SessionScanService`。明细见 progress.md。
- **P4 未开始 = 本交接目标**：Local History 忠实化。

## 下一步 = P4（详细骨架见 P4_PLAN.md）

**一句话**：LH **后端忠实甚至超前、几乎全就绪**，P4 只需补三样 —— ①**自动捕获 file-watcher**（灵魂，缺）②**暴露少量后端 API**（PutLabel/GetFileBranches，已实现未 wrap）③**重建 4-tab 前端页**（旧 HistoryPage 被删且是线性单-diff 页，非 4-tab）。

### 三个缺口（= P4 全部工作）
1. **Watcher 缺失**（P4a，灵魂）：当前唯一写入是手动 `SnapshotFile`。需新建 `internal/services/lhwatcher/`（fsnotify + 500ms debounce + 分支感知 + 3s checkout 静默 + ignore/size/binary/symlink 过滤 + per-project 写锁单写者 + 删除快照 + Emit `localhistory:changed` 事件）。忠实移植 cc-pane `cc-panes-core/src/services/history_service.rs:408-549`。`SaveVersion` 接口已为 watcher 预设。
2. **未暴露 API**（P4b）：`internal/localhistory/labels.go` 的 `PutLabel`、`store.go` 的 `GetFileBranches`/`ListVersionsByBranch`、`compress.go` 的 `CompressBlobs` 均实现但 `localhistorysvc` 没 wrap。4-tab 的标签 UI 需 `PutLabel`，分支筛选需 `GetFileBranches`（或前端对 ListVersions 本地过滤，cc-pane 即如此）。
3. **前端 UI 全无**（P4c）：建议新建 `pages/HistoryPage/` 整页（route id `history`，namespace `history.*`），忠实 cc-pane `web/components/LocalHistoryPanel.tsx` 的两阶段 + 4-tab（diff/content/deleted/project-restore）+ 版本时间线 + 分支筛选 + 标签 UI。

### 现成可复用（无孤儿，直接 import）
- 后端 `internal/services/localhistorysvc/`（16 方法）+ `internal/localhistory/`（store/diff/labels/restore/prune/compress/query/config，diff 还超前修了 CRLF）。
- 前端 `services/localhistory/LocalHistoryService.ts`（8 方法 + DTO，类型完整）+ `components/DiffView/`（GitPage 在用，零改复用）。
- 旧 lang / styles / VM 架构：`git show d44625c^:frontend/src/pages/HistoryPage/...`（lang 直接恢复扩 4-tab；旧页是线性单-diff，**不要 git revert 整页**，全新 4-tab 更合算，VM state 不兼容）。
- git 联动已接线：`app.go:114-117` `gitSvc.SetAutoLabeler(lhSvc.AutoLabel)`（Stash/Pull/Push 前打标签）。

### 单元切分（详见 P4_PLAN §五，逐单元提交全绿）
P4a-1 事件+watcher 骨架 → P4a-2 管线(debounce/分支/过滤/单写者/Emit) → P4b 暴露 API → P4c-1 页骨架+文件列表 → P4c-2 时间线+4-tab+分支筛选 → P4c-3 标签 UI+操作区+live 刷新。

### 开工前需用户确认的拍板点（其余已写入计划默认，详见 P4_PLAN §六）
- **D4-4 watcher 启动时机**：建议「扫已登记项目，仅对已初始化 LH（`.xkpanes` 存在）或 config.enabled 的启」+ SaveConfig/launchProject 时补启。性能/触发点是真实分歧，**请确认**。
- **D4-6 LH 页 nav 位**：建议重新上 Sidebar primary（id `history`）。注意 **P3b 刚移除过该 history 位**，请确认是否重新放（或先 `hidden` 仅文件级入口）。
- 其余默认：D4-1 独立包 lhwatcher / D4-2 bep/debounce 500ms / D4-3 双管齐下(debounce+per-project 写锁) / D4-5 复用 config.IgnorePatterns+`.xkpanes` 硬过滤 / D4-7 加 fsnotify / D4-8 读 .git/HEAD+watch HEAD 检测切换 3s 静默。
- Q-LH入口「页 + 文件级 Dialog 并存」：P4 先做整页，**文件级 Dialog 入口（树/Git 右键「查看历史」）留 P4d 后续**。

## 铁律速查（违反即返工，详见 CLAUDE.md / 蓝图 §5）

SQLite-only（LH 走每项目 `.xkpanes/` 独立库，不进 app 主库）；HTTP `httpx`/子进程 `procx`/日志 `logx`/写配置 `filex`；对话框走 `NativeDialogs`，应用内文本录入用 `components/TextInputDialog`（打标签）；图标 `@mui/icons-material *Rounded`（禁 emoji/Unicode/第三方/内联 SVG）；人类可见字符串走 `t()` + 新页配 `lang/{zh-CN,en-US}` 并 App.tsx 注册；颜色取 `theme.palette.foundation.*`（禁硬编码十六进制）；圆角 按钮 6 / 容器 8；事件名集中注册 `internal/events`（禁前端自造 window 事件）；平台分支走 build tag 文件。

MVVM：View(无 bindings) → `use<Page>`(无 JSX/无 t()/经 services) → `LocalHistoryService`（多数已就绪）→ `@bindings/xkpane/...`；导航只用 `useRouter().navigate('history')`；后端业务 service 持 `*storage.Holder` 不 capture `*storage.DB`（LH 是每项目独立 Store，经 localhistorysvc 缓存）；改后端方法签名后跑 `wails3 generate bindings`。

提交/推送仅在用户明确要求时；逐单元提交（每个 commit 自洽可编译）；始终用简体中文回复。当前分支 `rebuild/cc-pane`（非 master，可直接提交，不推送）。
