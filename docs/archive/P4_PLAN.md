# P4 实施计划 — Local History 忠实化（自动捕获 watcher + 4-tab UI）

> 产出方式：5-agent workflow 并行通读 cc-pane Local History 全貌（后端 watcher + 前端 4-tab）+ XK 现状基线 + P4 约束，结论带 `file:line`。
> 基准：cc-pane 源 `J:/Program/Agent/cc-pane`，XK 基座 `J:/Program/Agent/XK-Pane`（当前分支 `rebuild/cc-pane`）。
> 总目标：把 cc-pane 的 Local History（自动捕获文件版本 + 时间线 + diff/content/deleted/project-restore 四视图 + 标签 + 分支筛选）忠实重建到 XK。

---

## ✅ 执行状态

- **P4 未开始**。本文件 = 详细骨架。建议逐单元提交（每个 commit 自洽可编译），全绿后请用户 `wails3 dev` 手测。
- 前置：P1/P2/P3 已完成（终端真跑 / Git SCM / Home / Workspace 树 / 会话恢复扫盘子列表）。P3b 回退中**删除了旧 HistoryPage 并移除 history 导航位**——P4 重建 LH UI。

---

## 一、现状基线（critic 已核实，决定 P4 只需补「灵魂 + UI」）

### 后端「忠实甚至超前」，几乎全就绪
- 存储：每项目 `<project>/.xkpanes/history/`（`history.db` SQLite + `blobs/` 内容寻址，SHA256 去重）。对照 cc-pane `.ccpanes/history/`（`history_file_repo.rs:16-20`）。
- 域实现 `internal/localhistory/`：`store.go`（版本表 + SaveVersion 去重/节流）、`diff.go`（行级 + 字符级 inline，**且超前**：换行归一化修了 CRLF↔LF 假阳性）、`labels.go`（PutLabel/ListLabels/DeleteLabel/GetAllLatestSnapshots）、`restore.go`（单文件 + 会话回滚，自动打 Before-Restore/Before-Rollback 标签）、`prune.go`（age/count/LRU + 孤儿 blob 回收）、`compress.go`（CompressBlobs，gzip）、`query.go`（GetRecentChanges/ListDirectoryChanges/ListDeletedFiles）、`config.go`（HistoryConfig，JSON `.xkpanes/config.json`）。
- 分支：`FileVersion.Branch` 字段 + `ListVersionsByBranch` + `GetFileBranches` 已实现（**但未暴露给前端**）。
- service `internal/services/localhistorysvc/localhistorysvc.go` 暴露 **16 方法**：`SnapshotFile / GetRecentChanges / ListVersions / ListDeletedFiles / GetVersionContent / GetVersionDiff / GetVersionsDiff / ListLabels / RestoreVersion / RestoreToLabel / AutoLabel / DeleteLabel / GetConfig / SaveConfig / Cleanup / GetStats`。
- git 联动已接线：`app.go:114-117` `gitSvc.SetAutoLabeler(lhSvc.AutoLabel)`，gitsvc 在 Stash/Pull/Push 前打标签；`AutoLabel` 守门（项目无 `.xkpanes` → 返回空串不凭空创建）。

### 三个明确缺口（= P4 的全部工作）
1. **自动捕获 file-watcher 完全缺失**（LH 的灵魂）。`localhistorysvc.go:19` 注释自承「file-watcher 留后续」。当前唯一写入路径是手动 `SnapshotFile`。`SaveVersion` 接口已为 watcher 预设（`filePath, content, isDeleted, branch, minIntervalSecs`）。
2. **未暴露的后端能力**：`PutLabel`（手动打标签）/ `GetFileBranches`（分支筛选 UI）/ `ListVersionsByBranch`（可选）/ `CompressBlobs`（可选）。
3. **前端 UI 全无**：旧 HistoryPage 被删（`d44625c`，可 `git show d44625c^:frontend/src/pages/HistoryPage/...` 取回），且是**线性单-diff 页**（非 4-tab）；history 导航位已移除。

### 可直接复用的前端资产（无孤儿）
- `frontend/src/services/localhistory/LocalHistoryService.ts`（8 方法 + DTO：RecentChange/FileVersion/DiffResult/HistoryConfig/HistoryStats，仍被 GitService 间接引用，类型完整）。
- `frontend/src/components/DiffView/`（行级 + 字符级 diff 渲染，**GitPage 在用**，零修改可复用）。
- 旧 `pages/HistoryPage/lang/{zh-CN,en-US}.ts`（`history.*` 命名空间，`git show` 恢复后扩 4-tab key）。
- 旧 `useHistoryPage.ts` 架构（项目选择 + 列表加载 + diff + config 模式可参考，但 4-tab 需重写 state）。

---

## 二、P4a — 自动捕获 Watcher 后端（灵魂，先做）

> 忠实移植 cc-pane `history_service.rs:408-549` 的 watcher 管线（notify crate → XK 用 fsnotify）。

### 依赖
- **新增 `github.com/fsnotify/fsnotify`**（最新稳定版，blueprint Q-依赖已授权）。
- **`github.com/bep/debounce v1.2.1`**：go.mod 已 indirect，提为 direct。

### 落点（拍板点 D4-1，建议独立包）
新建 `internal/services/lhwatcher/`：
```
watcher.go   —— Watcher struct（fsnotify.Watcher + per-project debounce + 分支上下文）
ignore.go    —— 忽略规则匹配（复用 config.IgnorePatterns + .xkpanes/ 硬过滤）
```

### 忠实移植的管线（5 阶段，对照 cc-pane）
1. **Dispatch**（`history_service.rs:506-526`）：fsnotify Create/Write → 「文件变更」；Remove → 「文件删除」；其余忽略。递归监控项目根（`RecursiveMode::Recursive` → fsnotify `Add` 递归遍历子目录，注意 fsnotify 不自动递归，需 walk 一次 + 对新建目录补 Add）。
2. **分支切换检测**（`history_service.rs:479-504`）：每个事件到达读 `.git/HEAD`（复用 `internal/utils/gitx` 或直接读，支持 worktree `gitdir:` 格式）；分支变了 → 打 `branch_switch` auto 标签 + 设 **3s 静默窗口**（抑制 `git checkout` 噪声）+ 跳过当前事件。常量对照 `constants.rs:30`。
3. **Debounce 合并**（`history_service.rs:114-186`，trailing-edge **500ms** = `constants.rs:27`）：窗口内同一文件路径只保留最后一次；用 bep/debounce 驱动 flush，自管 per-project pending 文件集。
4. **过滤**（`history_service.rs:268-360`）：config.enabled 检查；目录跳过；**符号链接/越界保护**（canonicalize 后须仍在项目根内）；`config.IgnorePatterns` 匹配（默认含 `node_modules/** .git/** .xkpanes/** *.log` 等，见 `internal/localhistory/config.go`）；`MaxFileSize`（默认 5MB）超限跳过；二进制检测（BOM + `\0`）。
5. **单写者保存 + 去重/节流**（拍板点 D4-3）：所有写经 **per-project 写串行**（见下）；调 `SnapshotFile`/`SaveVersion`（带 `MinSaveIntervalSecs` 默认 300s）；hash 去重（同分支同 hash 跳过）；删除文件 → 以 `is_deleted=true` 存最后快照（`history_service.rs:335-371`，删除不受节流）。

### 并发互斥（拍板点 D4-3，建议 per-project 写串行）
两个写者动同一 LH 仓：① gitsvc 的 `AutoLabel`（Stash/Pull/Push 前，少量）；② watcher 的 SaveVersion（频繁）。cc-pane 用单写者 MPSC（`history_service.rs:64-69`）。XK 建议：在 `localhistorysvc.Service` 给每项目一把写锁（`map[projectPath]*sync.Mutex`），`SnapshotFile`/`SaveVersion`-from-watcher 与 `AutoLabel` 都先取锁；查询走读路径不锁（GORM 单连接 + WAL 已隔离读）。或 watcher 侧 debounce 批提交天然降碰撞 —— 建议**双管齐下**（debounce 降频 + 写锁兜底）。

### 启动时机（拍板点 D4-4，**需用户确认**）
建议默认：app 启动扫已登记项目（`projects` 表 / `workspace_projects`），**仅对「`.xkpanes/history` 已存在 或 config.enabled」的项目启 watcher**（避免 watch 海量无关目录）；`SaveConfig(enabled=true)` 时即时启；`launchProject` 时确保启。停：app 退出 / SaveConfig(enabled=false)。

### 事件推送（前端 live 刷新）
`internal/events/events.go` 注册 `localhistory:changed`（`LocalHistoryChangedPayload{ProjectPath, FilePaths[], VersionCount, Timestamp}`，仿 `TerminalStatusPayload` 风格 + `Register()` 里 `RegisterEvent`）。watcher 保存成功后 `app.Event.Emit`。前端新建 `shared/hooks/useLocalHistoryChangeEvent.ts` 订阅（按 projectPath 过滤）→ 触发页面 refresh。watcher 需 `AttachApp(app)` 拿 Emit 能力（仿 termSvc/subSvc 预创建 + 回填模式，`app.go:72-77`）。

---

## 三、P4b — 暴露缺口后端 API + bindings

`localhistorysvc` 增补（域层已实现，仅补 service wrapper + `wails3 generate bindings`）：
- `PutLabel(ctx, projectPath, label HistoryLabel) error` —— 手动打标签（4-tab 的标签 UI 用）。对照 cc-pane `put_label`。
- `GetFileBranches(ctx, projectPath, filePath) ([]string, error)` —— 分支筛选下拉数据源。对照 cc-pane `get_file_branches`。
- `ListVersionsByBranch(ctx, projectPath, filePath, branch)`（可选，前端也可对 ListVersions 结果本地过滤 —— cc-pane 前端就是本地过滤 `LocalHistoryPanel.tsx:127-140`，故**优先前端过滤，本方法可不暴露**）。
- `CompressBlobs`（可选，低优，给配置页一个「压缩」按钮或留后端定时）。
前端 `LocalHistoryService.ts` 同步补 `putLabel` / `getFileBranches`。

---

## 四、P4c — LocalHistory 前端（4-tab 忠实 UI）

> 对照 cc-pane `web/components/LocalHistoryPanel.tsx`（modal Dialog，4 viewMode）。XK 落点见拍板点 D4-6。**建议：新建 `pages/HistoryPage/` 整页**（route id `history`，namespace `history.*`，git-show 恢复旧 lang 再扩）。

### 两阶段 + 4-tab 结构（`LocalHistoryPanel.tsx:32, 334-600`）
- **项目选择**（顶部下拉，复用旧 HistoryPage 项目下拉 + `useProjects()`）。
- **阶段 1 文件列表**（无选中文件时）：`GetRecentChanges(path, 200)` → 按文件聚合最新变更（文件名 + 目录 + 相对时间 + 大小 + 分支 badge），点击进入阶段 2。对照 `LocalHistoryPanel.tsx:353-397`。
- **阶段 2 版本时间线 + 4-tab 预览**（`LocalHistoryPanel.tsx:518-600`）：
  - 左 260px **版本时间线**：`ListVersions(path, filePath)` 倒序（最新在上）；每项 = 相对时间 + 大小 + 分支 badge + 该版本关联标签 badge；选中高亮 + 键盘上下/Enter 导航（`:310-326`）。
  - **分支筛选下拉**（仅多分支时显示，deleted/project-restore tab 隐藏；`:418-427`）：`GetFileBranches` 取分支列表，前端 filter。
  - 右 4 个 tab（`type ViewMode = 'diff'|'content'|'deleted'|'project-restore'`）：
    1. **diff**：相邻版本 diff（`GetVersionsDiff(old,new)`）；最早版与工作树比（`GetVersionDiff`）。**复用 `<DiffView>`**（新建 `history.diff.*` labels）。
    2. **content**：版本全文（`GetVersionContent`）`<pre>` 渲染。
    3. **deleted**：`ListDeletedFiles` → 已删文件列表 + 单文件恢复（`RestoreVersion`）。
    4. **project-restore**：`ListLabels` → 项目级标签/快照列表（source 彩色 badge + 文件数 + 「恢复到快照」`RestoreToLabel`，**confirm 走 NativeDialogs**）。
- **标签 UI**（`:279-298, 605-640`）：版本右键 / 「Add Tag」按钮 → **应用内文本对话框（复用 `components/TextInputDialog`）** → `PutLabel`（source='user', type='manual'）。
- **操作区**：手动快照（复用旧 `SnapshotFile`）/ 单文件恢复（`RestoreVersion`，confirm）/ 配置编辑 + stats + cleanup（复用旧 HistoryPage 底部面板）。
- **live 刷新**：`useLocalHistoryChangeEvent(selectedPath)` 命中 → 重拉当前列表（watcher 自动捕获后时间线即时更新）。

### 恢复流（confirm 一律走 NativeDialogs，非浏览器 confirm）
| 类型 | 触发 | 确认 | 后端 |
|---|---|---|---|
| 文件级（diff/content/deleted） | Restore 按钮 | 视情况（删除文件可免） | `RestoreVersion` |
| 项目级（project-restore） | 「恢复到快照」 | **必 confirm** | `RestoreToLabel` |

### MVVM / 铁律
- `pages/HistoryPage/{HistoryPage.tsx, useHistoryPage.ts, HistoryPage.styles.ts, HistoryPage.skeleton.tsx, index.ts, lang/{zh-CN,en-US,index}.ts}`；tab 内容可拆 `components/` 或页内子组件。
- View 不碰 bindings；VM 经 `LocalHistoryService`、不持 t()；图标 `@mui/icons-material *Rounded`；取色 `theme.palette.foundation.*`；圆角 按钮 6 / 容器 8；keepAlive 建议 `true`（保留滚动/展开）。
- routes.tsx 注册 + i18n `route.history`（git-show 恢复）+ App.tsx `registerHistoryPageLocales()`。

---

## 五、可提交单元切分

| 单元 | 范围 | 依赖 | 验收 |
|---|---|---|---|
| **P4a-1 事件 + watcher 骨架** | events.go 注册 `localhistory:changed`；`lhwatcher` 包（fsnotify 启停 + 递归 watch + Dispatch）；app.go AttachApp + 启动扫描；go.mod 加 fsnotify/提 debounce | 无 | `go build/vet`；手测：编辑文件后后端日志出现捕获（先不存版本也可） |
| **P4a-2 管线（debounce/分支/过滤/单写者）** | debounce 500ms 合并 + 分支检测 + 3s 静默 + ignore/size/binary/symlink 过滤 + per-project 写锁 + SaveVersion 节流去重 + 删除快照 + Emit 事件 | P4a-1 | `go test`（lhwatcher 纯过滤/debounce 单元 + 并发写锁）；手测：连续编辑只按节流存版本、删文件进 deleted、切分支不刷屏 |
| **P4b 暴露 API** | localhistorysvc + `PutLabel`/`GetFileBranches`（+可选 ListVersionsByBranch/CompressBlobs）；前端 LocalHistoryService 补；bindings 重生 | P4a 可并行 | `go build/vet`；typecheck |
| **P4c-1 HistoryPage 骨架 + 文件列表** | git-show 恢复 lang/styles 基线；项目下拉 + 阶段1 文件列表 + 路由注册 + skeleton | P4b | typecheck；手测：选项目→列出最近变更文件 |
| **P4c-2 版本时间线 + 4-tab + 分支筛选** | 阶段2 时间线 + diff/content/deleted/project-restore 四 tab + 分支下拉 + DiffView 集成 | P4c-1 | typecheck；手测：四 tab 切换、diff 正确、deleted/项目恢复 |
| **P4c-3 标签 UI + 操作区 + live 刷新** | 打标签（TextInputDialog→PutLabel）+ 手动快照/恢复/config/stats/cleanup + useLocalHistoryChangeEvent 订阅刷新 | P4c-2 | typecheck；手测：打标签、watcher 捕获后时间线自动刷新 |

---

## 六、需用户拍板的点（建议默认已写入正文，开工前确认下列）

| # | 决策 | 建议 |
|---|---|---|
| D4-1 | watcher 落点 | **独立包 `internal/services/lhwatcher`** |
| D4-2 | debounce | **bep/debounce（已 indirect 提 direct）+ per-project pending 集**，窗口 500ms |
| D4-3 | 并发互斥 | **双管齐下**：debounce 降频 + localhistorysvc per-project 写锁（watcher SaveVersion 与 gitsvc AutoLabel 共用） |
| D4-4 | **watcher 启动时机** | **扫已登记项目，仅对已初始化 LH（`.xkpanes` 存在）或 config.enabled 的启**；SaveConfig(enabled)/launchProject 时补启（**性能/触发点是真实分歧，请确认**） |
| D4-5 | 忽略规则 | **复用 config.IgnorePatterns（已含默认）+ `.xkpanes/` 硬过滤防自噬** |
| D4-6 | **LH 页面 nav 位** | **primary（id `history` + HistoryRounded 图标）重新上 Sidebar**（注：P3b 刚移除过该位，请确认是否重新放，或先 `hidden` 仅文件级入口） |
| D4-7 | fsnotify 依赖 | **添加 `github.com/fsnotify/fsnotify` 最新稳定版** |
| D4-8 | 分支感知 | **每 capture 读 `.git/HEAD` + watch `.git/HEAD` 检测切换设 3s 静默**（忠实 cc-pane；可拆 P4a-2） |

> 另：cc-pane Q-LH入口 = 「页 + 文件级 Dialog 并存」。P4 先做整页（P4c）；**文件级 Dialog 入口（工作空间树/Git 右键「查看历史」）留 P4d 后续**。

---

## 七、铁律速查（违反即返工，详见 CLAUDE.md / 蓝图 §5）

SQLite-only（LH 走每项目 `.xkpanes/` 独立库，不进 app 主库）；HTTP `httpx` / 子进程 `procx` / 日志 `logx` / 写配置 `filex`（LH config 已用 filex 原子写）；对话框走 `NativeDialogs`，应用内文本录入用 `components/TextInputDialog`（打标签）；图标 `@mui/icons-material *Rounded`（禁 emoji/Unicode/第三方/内联 SVG）；人类可见字符串走 `t()` + 新页配 `lang/{zh-CN,en-US}` 并 App.tsx 注册；颜色取 `theme.palette.foundation.*`（禁硬编码十六进制）；圆角 按钮 6 / 容器 8；事件名集中注册 `internal/events`（禁前端自造 window 事件）；平台分支走 build tag 文件。

MVVM：View(无 bindings) → `use<Page>`(无 JSX/无 t()/经 services) → `LocalHistoryService`（多数已就绪）→ `@bindings/xkpane/...`；导航只用 `useRouter().navigate('history')`；后端业务 service 持 `*storage.Holder` 不 capture `*storage.DB`（LH 是每项目独立 Store，经 localhistorysvc 缓存）；改后端方法签名后跑 `wails3 generate bindings`。

提交/推送仅在用户明确要求时；逐单元提交（每个 commit 自洽可编译）；始终用简体中文回复。当前分支 `rebuild/cc-pane`（非 master，可直接提交，不推送）。
