# 任务计划：基于 foundation 框架在 XK-Pane 工作区重构 cc-pane

## 目标声明（Goal）

把 `J:\Program\Agent\cc-pane`（Tauri 2 + Rust + React/Zustand/shadcn）的核心与常用功能，**全新重构**到当前工作区 `J:\Program\Agent\XK-Pane`（Wails v3 + Go + React 19 + MUI 9 + foundation MVVM 脚手架）之上。

**已确认的重构基调（来自用户决策）**：
- 深度：**核心 + 常用增强**（覆盖 cc-pane ≈70% 高频功能），完整 1:1 作为长期路线图。
- 前端：**全新 MVVM 重写**，遵循 foundation 的 View/ViewModel/Style 三件分离 + MUI 9 + 路由/主题/i18n 体系。
- 状态管理：**纯 foundation MVVM + React Context**，**弃用 Zustand**。
- 早期纳入子系统：**Bedrock 知识库 / MC PE 集成 / cc-notify / cc-memory**。
- 延后：**IM 桥接（im_bridge）**、cc-panes-web（Docker 部署）、cc-panes-api（独立 HTTP 服务）。

**成功判据（高层）**：能在 XK-Pane 应用里打开工作区→项目→在内置终端里以选定 Provider 启动 Claude Code 等 CLI 并交互；具备 Git/Local History/会话恢复/设置/主题/i18n；Bedrock 查询、MC PE、通知、记忆四个子系统可用。

> 详细架构盘点、源→目标映射表、铁律、风险见 `findings.md`。会话日志见 `progress.md`。

---

## 关键架构决策（Decisions）

| # | 决策 | 选择 | 依据 |
|---|------|------|------|
| D1 | 项目改名 | `module foundation` → `module xkpane`（AppDirName/Taskfile/bindings 同步） | foundation README 改名流程；bindings 前缀 = module 名 |
| D2 | 状态管理 | React Context + Provider（每域一个 Provider，内含 useReducer/useState），**无 Zustand** | 用户决策 D；与 foundation 现有 Theme/i18n/Preferences Provider 一致 |
| D3 | PTY 方案 | **已定** `github.com/aymanbagabas/go-pty` v0.2.3，门面 `internal/utils/ptyx`。Win 绑 JobObject 走 CREATE_SUSPENDED+平移 procx_windows；**Unix 不叠 procx setpgid（go-pty 强制 Setsid）**，kill 用 `Kill(-pid,sig)`。终端 I/O 走 PTY 原始字节流（非 procx 行回调，见 findings J2） | 尽调验证唯一满足跨平台+真ConPTY+纯Go+暴露os.Process |
| D4 | Git 方案 | **已定** `go-git/v5` 为主（status/branch/log/diff/fetch/push）+ git CLI fallback（stash/worktree/非ff-pull/credential-helper，经 subprocess 白名单+procx）。**不上 v6-alpha** | 纯 Go 无 CGO；v6 worktree 实验包不稳+鉴权 API 破坏性变更 |
| D5 | MCP server | **已定** 官方 `github.com/modelcontextprotocol/go-sdk@v1.6.1`，泛型 `AddTool` 自动 schema，`StreamableHTTPHandler{Stateless:true}` 同进程起 localhost；Bedrock 22MB 用 `embed.FS` | go.mod=go1.25；批量注册 26 工具样板最少；关注 issue #633 |
| D6 | 不可替代前端依赖保留 | xterm.js(+addons) / Monaco / 自实现 SplitView 分屏 / @dnd-kit；其余 shadcn+Tailwind→MUI 9 | MUI 无 TTY/代码编辑器/分屏等价物 |
| D7 | 路径字段铁律 | 严格沿用 cc-pane 的 `project_path`(=PTY cwd) / `workspace_path`(仅多目录 CLI 拼 --add-dir 且 ≠project_path) / `launch_cwd`(运行时观测) 三字段语义 | cc-pane CLAUDE.md「铁律」；守门测试必须一并移植 |
| D8 | 数据持久化 | 全部走 foundation 的 GORM+SQLite（AllModels 追加模型），敏感字段(API key/token)经 cryptox 加密 | foundation 持久化铁律；禁 localStorage |
| D10 | Local History 存储 | **已定**（用户 2026-05-30）：每项目独立 `<proj>/.xkpanes/history/`（history.db + 内容寻址磁盘 blob），目录名 `.xkpanes`（非 cc-pane 的 `.ccpanes`）。GORM 按项目多 db（不走 Holder） | 文件历史是每项目/随项目走/内容大的独立域，是对 D8（app 级单库）的有意分层；忠实 cc-pane 每项目存储。详见 findings J12 |

---

## 阶段（Phases）

### Phase 0 — 立项基线与改名 ✅ complete
**目标**：把 foundation 脚手架改名为 xkpane，确立应用身份、数据目录、Dev/Release 隔离，跑通空壳。
- [x] 改名：go.mod / 全局 import / bindings 路径 / Taskfile APP_NAME / storage·cryptox·logx 的 AppDirName → `XK-Pane`（含尽调漏掉的 DBFileName→`xkpane.db` 与 write-test 名）
- [x] `wails3 generate bindings` ✅ exit=0 → `frontend/bindings/xkpane/`（5 Services/22 Methods/24 Models/1 Event），**bindings 前缀=xkpane**
- [x] `pnpm typecheck` ✅ exit=0（`xkpane-frontend@0.0.0`，前端 @bindings/xkpane 解析通过）
- [x] **Dev/Release 数据目录隔离**：新建零依赖叶子包 `internal/appinfo`（build tag `production` 区分；dev→`XK-Pane-Dev` / release→`XK-Pane`），storage/cryptox/logx 共享 `appinfo.AppDirName`（顺带消除「改名同步 3 处」痛点）。守门测试 `appinfo_test.go` 双模式通过
- **verify** ✅：`go build`(dev+prod)/`go vet`(dev+prod)/`go test`(dev+prod) 全 exit=0；wails3 bindings 前缀=xkpane；pnpm typecheck 通过
- **注**：保留主题 `foundation.*` 命名空间（设计系统 API，非模块名）；`-framework Foundation`（iOS Apple 框架）不动；`build/linux/nfpm` 的 `foundation.exe` 待 Phase 6 由 build-assets 重生成

### Phase 1 — 后端地基（PTY / 子进程 / CLI 适配器 / 数据模型 / 事件）✅ complete
**目标**：铺好移植所需的后端底座，无 UI 也能单测。
- [x] `internal/utils/ptyx`：跨平台 PTY 门面（go-pty v0.2.3）。Windows ConPTY + JobObject(CREATE_SUSPENDED→assign→ResumeThread)；Unix 不叠 Setpgid（go-pty 强制 Setsid）kill 用 `Kill(-pid,SIGKILL)`。✅ **R1 spike 在 Win11 实跑通过**（`TestSpawnEchoProducesOutput`/`TestResizeNoError` PASS；Unix 交叉编译 exit=0）
- [x] `internal/events`：注册终端类型化事件 `terminal:output`/`:status`/`:exit`（+payload struct）。bindings 重生成确认 4 Events
- [x] `internal/storage/models_ccpane.go`：移植 7 张 data.db 核心表 GORM 模型（Project/LaunchHistory/Todo/TodoSubtask/Spec/TerminalSession/TaskBinding），`init()` 追加 AllModels，显式 `TableName()`。✅ AutoMigrate 守门测试通过。时间戳用 time.Time（greenfield）；敏感字段 last_prompt/ssh_config 标 TODO 待 Phase 3 写入方落地加 cryptox 钩子
- [x] `internal/services/cliadapters`：8 个 CLI 适配器 build_command + 路径字段铁律（D7）+ Registry/Detect + 可执行解析（Windows npm shim）。✅ 18 守门测试（双模式 ok），GLM 可执行=`crush`、reclaude versionArgs=`version`。**延后**（标 TODO(phase5)，依赖未落地属投机）：claude/reclaude `generate_mcp_config`（需 orchestrator port/token）、codex `~/.codex/config.toml` 解析（disable_unlisted_mcp 默认关）、项目 hooks 机制（需 cc-panes-cli-hook 二进制 + 已延后的 IM 桥接，对应 5 个 hooks 守门测试一并延后）
- [~] ~~`internal/services/subprocess`：扩展白名单（git 各子命令）~~ **改判延后到 Phase 4**（用户确认）。理由：git CLI fallback 的消费方是后端 Git 服务（Phase 4），需**同步取输出**，后端 Go 直接用 procx 即可（procx 是后端原语，铁律只约束前端）；而 `subprocess.Service` 是**前端异步白名单**（按 ID 发事件），层次不匹配。Git 服务尚未建立时确定子命令/参数模式属投机。前端白名单保持空，待前端用例再加
- ⚠️ Phase 2 待办（ptyx 测试暴露）：ConPTY 输出是异步屏幕刷新，终端服务在子进程退出后须留 drain 宽限再 Close PTY，否则丢尾部输出
- **verify** ✅：`go test ./internal/...`(dev+prod) 全 ok；ptyx 已起 shell 读到输出；cliadapters 18 守门覆盖 path 铁律 + 经 workflow 保真度审查；AutoMigrate 建表测试通过；`go vet`/`wails3 generate bindings`/`pnpm typecheck` 全绿

### Phase 2 — 终端核心闭环（首个可运行里程碑）🔵 in_progress
**目标**：能在某项目目录下用内置终端跑起一个 CLI 并交互——MVP runnable。
- [x] 后端 `terminal_service`：PTY 会话注册表 + I/O 隧道（batch Emit `terminal:output`/`:status`/`:exit`）+ resize/kill + drain 宽限关闭 + UTF-8 跨 read 安全处理；裸 shell / cliadapters 启动；路径铁律 D7（cwd≡ProjectPath）。已注册进 app.go + AttachApp + bindings 重生成（6 Services/28 Methods）。**12 单测全绿**（含真 ConPTY 闭环 round-trip ×5 稳定，dev+prod）。launch_cwd 上报接收端属 Phase 5（已注入 CC_PANES_PTY_SESSION_ID 备用）。
- [~] 前端保留组件：`TerminalView`(xterm.js)、自实现 `SplitView` 分屏、`TabBar`(@dnd-kit)。**布局调度器 `layoutScheduler.ts` ✅**（会话6：逐字移植 cc-pane terminalLayoutScheduler，fit/resize 双 rAF 去抖 + 抖动跳过 + 可见性门控，jsdom 4 测；装 `@xterm/xterm@6`+`@xterm/addon-fit@0.11`+`jsdom`）。WebGL `terminalRendererController` 延后（MVP 用 DOM 渲染器）。**`TerminalView`(xterm.js) ✅**（会话7：`components/TerminalView/` View/VM/Style，create+open+fit+ResizeObserver+onData→write+onOutput→write+onExit+dispose；typecheck+build 绿；2-lens 效果生命周期审查 0 真实 bug；Win 写入流控经审查 MVP 不需要）。**自实现 `SplitView` ✅**（会话8：`components/SplitView/` flex+sash 拖拽，纯几何 `computeSashResize`/`normalizeSizesToHundred` 7 测；忠实移植 cc-pane SplitView 拖拽，弃 allotment）。**MVP `TabBar` ✅**（会话9：`components/TabBar/` 选择/关闭/新建 + 活动态 + `terminal.*` i18n aria-label；MUI icons。**@dnd-kit 拖拽重排延后**——耦合父级 DndContext + reducer.reorderTabs，单列一单元）。待做 递归 `PaneTree` 渲染器 + 生命周期整合
- [~] 前端 MVVM：**服务层 `TerminalService.ts` ✅** + **面板树类型/纯 helpers `terminal/{types,paneTree}.ts` ✅**（移植 cc-pane pane.ts/terminal.ts/paneTreeHelpers.ts 的 MVP 子集：外层 PaneNode 树〔Panel 含 tabs / SplitPane〕+ 内层 TerminalPaneNode 树〔leaf 持 sessionId / split〕+ generateId/createPanel/createTab/findPane/findParent/collectPanels；裁剪 provider/resume/ssh/wsl 等 Phase 3/4 字段；typecheck 绿）。**`TerminalContext`（Provider+reducer）✅**（会话5：clone-then-mutate 移植 8 个外层树操作 split/closePane/addTab/closeTab/selectTab/setActivePane/resizePanes + normalizePaneTree 折叠 + setTabSession 回填钩子；引入 Vitest，**21 测全绿**；3-lens 保真度审查 0 真实 drift）。待做：终端面板 View/VM/Style（`TerminalView` xterm.js）+ wire TerminalService 生命周期 effect（newTab→create→回填 / onExit→清理，随 View 接入）
- [ ] 侧边栏工作区/项目树（最小版）+ 右键启动
- **verify**：打开项目→新建终端标签→看到 shell/CLI 输出→输入回显；分屏可拖拽；关闭标签干净回收 PTY

### Phase 3 — 工作区/项目/会话/设置/主题·i18n 🔵 in_progress
**目标**：补齐管理面与配置面，达到日常可用。
- Project/Workspace CRUD、别名、Provider 绑定（详见 findings J8：cc-pane 有扁平 Project 表 + workspace.json 文件存储两套；live UI 用 workspace 分组，但本重构 MVP 先走扁平 project 侧边栏）：
  - [x] **后端扁平 `project` service**（`internal/services/project`：List/Add/Get/Remove/UpdateName/UpdateAlias + 非导出 existsByPath + validatePath 结构化码）。忠实移植 cc-pane ProjectService/Repository（spec workflow `wf_79a917da-67a`），19 守门测试锁全部保真铁律（path 原样存/逐字节去重/非 upsert/created_at DESC/alias trim→NULL/name basename·"unnamed"/UpdateName 未 trim·非对称/affected-rows→"Project does not exist"）。接进 app.go（纯 CRUD 无 AttachApp），bindings=7 Services/34 Methods（existsByPath 非导出未绑）。✅ go build/vet/test(dev+prod)+全量+typecheck+前端 37 测全绿，commit `c8df705`。对抗保真审查 workflow `wf_7bc251b0-c1f` 进行中。
  - [ ] 前端 `ProjectService.ts` + `ProjectContext`（Context，弃 Zustand）+ 侧边栏扁平项目树（替换 TerminalPage 临时「打开文件夹」工具条）+ 右键/工具条启动（选 CLI 工具 → newTab(projectPath)）。
  - [ ] 🚧 **Workspace 子系统（含 D9 决策门：文件存储 vs SQLite 原生）** —— 到此单元**停下询问用户**（findings J8）。含 workspace 分组、Provider 绑定。
- 启动历史 + 会话恢复：
  - [x] **后端 `launch_history` service**（Record/List 倒序，LaunchedAt UTC；7 测，commit `8869111`）+ 前端薄接线（`LaunchHistoryService.ts` + launchProject record-on-launch fire-and-forget，commit `6c187e9`）。
  - [ ] 「最近启动」UI 消费方 + 会话恢复（terminal_sessions，跨重启）。
- Provider 管理（多 API Provider，敏感字段 cryptox 加密）：
  - [x] **后端 `provider` service**（List/Get/Add/Update/Remove/SetDefault，**cryptox 加密 apiKey**，脱敏 DTO，default 不变式事务；15 测，安全审查 clean，commit `c86bc60`；决策见 findings J9）。
  - [x] **provider→启动 env 注入打通**（`provider.ResolveLaunchEnv` Tier-A env+Tier-B cli 上下文 非绑定；terminal `CreateSpec.ProviderID` + ProviderResolver + resolveLaunch/buildEnv 合并；app 装配 closure）。**「选定 Provider 启动」后端能力完成**，commit `0114099`（10 测）。
  - [x] **前端 provider 三段**：`ProviderService.ts`（脱敏 DTO，commit `4fc8f6d`）+ `ProviderProvider`（Context，镜像 ProjectProvider，`cd86d7b`）+ **provider 设置子页**（`SettingsPage/provider/`：列表+新增/编辑 Dialog 表单+删除+设默认，走已验证设置框架，`ff99674`）。**provider 管理 UI 可达可验**。
  - [x] provider 设置表单**条件云字段**（3b：FIELDS_BY_TYPE 按类型渲染 apiKey/baseUrl/region/awsProfile/projectId/configDir，commit `5a9f03f`）。**provider 管理对 11 类全可配，feature 完整**。
  - [ ] **启动时选 Provider/CLI 的 UI**（传 providerId/cliTool）依赖未验证终端渲染 → 与视觉 checkpoint 一并（后端已通）。
- [ ] 设置页：按 SettingsPage 子目录隔离规范新增 `provider/terminal/shortcuts/proxy/about` 子页
- [ ] i18n：移植 cc-pane 12 个命名空间到 foundation 注册式 i18n（zh-CN/en-US）；主题沿用 foundation
- [ ] 窗口形态特性（用户要求全要）：弹出终端窗口(mode=popup)、迷你模式、无边框浮动按钮
- **verify**：重启后会话/布局恢复；设置改动落 SQLite 并生效；中英切换无硬编码字符串；弹出/迷你/无边框可切换

### Phase 4 — 常用增强（Git / Local History / 多运行时 / 启动配置 / Todo·Spec）🔵 in_progress
**目标**：覆盖 cc-pane 高频增强功能。
- Git 集成（go-git/v5 为主 + git CLI 回退，D4；spec workflow `wf_93487368-df5` 确认 cc-pane git 是 100% CLI、范围仅 12 命令无 diff/commit/stage——终端里手动做）：
  - [x] **gitx 只读包**（`internal/utils/gitx`：Open/IsRepo/CurrentBranch/IsDirty/Branches/Log；go-git 原生 branch/log + CLI 回退 IsDirty 经 procx `status --porcelain`+只读 allowlist；go-git v5.19.1 indirect→direct，已随 Wails 引入零新依赖）。4 测对真临时 repo（go-git+CLI 双路），dev+prod 全绿，commit `bf279a6`。
  - [ ] git 面板单元：结构化 per-file status（XY porcelain）+ stage/unstage/commit（+ Local History auto-label 钩子）+ diff（go-git Patch 或 CLI worktree-vs-HEAD）→ 需 Wails service 包装 + 前端面板（部分依赖视觉验证）。
  - [x] **network 单元**（CLI-only，commit `4d6f9d4`）：`gitx` 加 Clone(包级,`git clone [--depth 1] -- url dst`,dst 存在报错)/Fetch(`fetch --all`)/Pull/Push。**偏离计划的 go-git PlainClone → 改 CLI**（理由：与 gitx 其余 CLI 一致 + 走系统凭据助手/SSH 一条码路覆盖公/私库，D4 本就要求 credential-helper 走 CLI）。allowlist 追加 clone/fetch/pull/push。1 集成测（本地 bare 远端 clone/push/fetch/pull，无真网络/鉴权）。**延后**（findings J11）：显式 user/pass（argv 明文凭据反模式 → 留 askpass UI 单元）、细粒度 \r 进度事件（procx.scan 按 \n 切，需 \r-aware reader + UI）、Pull/Push 前 Local History 标签钩子（gitsvc 服务层）。
  - [x] **worktree + stash 单元**（CLI-only，commit `c8f6d8c`）：`internal/utils/gitx` 扩 Worktrees(list --porcelain 解析)/WorktreeAdd(-b 新分支 + GIT_LFS_SKIP_SMUDGE=1 + `--` 护路径)/WorktreeRemove(恒 --force)/Stash/StashPop。allowlist 加 worktree·stash；runGit 拆 runGitEnv。3 新测(真 repo worktree/stash + 纯函数解析边界)，dev+prod 全绿。**有意偏离**：IsMain 用 porcelain 首条(比 cc-pane 路径相等健壮)；分组目录策略/名校验/stash 前 Local History auto-label 钩子 留 gitsvc 服务层。
  - **git CLI 回退层次**（Phase 1 改判落点）：后端 Git 服务**直接用 procx** 同步执行 + 服务内部 git 子命令 allowlist（gitx cli.go 已落地）+ 同步捕获 stdout/stderr，**不经**前端 `subprocess.Service` 白名单（后者是前端异步事件机制）
- Local History（**每项目 `<proj>/.xkpanes/history/`** 独立 history.db + 内容寻址磁盘 blob，决策门 **D10** 已定；移植 cc-pane history_file_repo.rs）：
  - [x] **存储基座**（`internal/localhistory`，commit `43d6f80`）：Open/Close + FileVersion(复合 PK) + SaveVersion(SHA256 blob 去重 + branch 维度 hash 去重 + min_interval 节流 + is_deleted 不去重 + filex 原子写) + ListVersions/ListVersionsByBranch/GetFileBranches + GetVersionContent(gzip 兼容)。GORM 多 db 模式(不走 Holder)。6 测 dev+prod 全绿。
  - [x] **标签**（`labels.go`，commit `a089f0b`）：Label + LabelSnapshot 表 + HistoryLabel 组合视图；PutLabel(ON CONFLICT upsert + 整体替换快照)/ListLabels(timestamp DESC,避 N+1)/DeleteLabel/GetAllLatestSnapshots(每文件 MAX(id))。是 gitx auto-label 钩子的直接依赖。7 测 dev+prod 全绿。
  - [ ] diff 引擎（行级 hunk + 字符级 inline，**Go diff 库选型 = 后续决策点**）/ 回滚 restore（写回工作树）/ 最近变更 RecentChange / 清理 pruning(max_age/max_versions/max_total_size LRU + 孤儿 blob) / gzip 压缩。
  - [ ] gitsvc 服务层（Wails 包装 + 按项目缓存 Store + gitx Pull/Push/Stash 的 auto-label 钩子）；`.xkpanes` 自动加 .gitignore 的 UX 决定。
- [ ] 运行时：Local/WSL/SSH（ssh_machine + wsl_discovery）
- [ ] 启动配置 launch_profile + 启动配置文件管理
- [ ] Todo / Spec / Plan 工作流
- **verify**：Git 面板操作生效；文件改动产生历史版本可 Diff/回滚；WSL/SSH 运行时能启动

### Phase 5 — 早期子系统（Bedrock / MC PE / notify / memory）⬜ pending
**目标**：移植用户要求尽早的四个子系统 + MCP server 底座。
- [ ] MCP server 底座（D5）+ orchestrator HTTP 回调（CLI hook 上报）
- [ ] Bedrock 知识库：`embed.FS` 内嵌 ~22MB 数据 + 索引/模糊搜索/26 查询工具/13 审查规则
- [ ] MC PE 集成（Windows）：包管理 + 桥模组热重载 + Monaco 断点调试 + Bedrock 文档同步
- [ ] cc-notify：多渠道推送（钉钉/飞书/Slack/Telegram/Webhook）
- [ ] cc-memory：记忆 CRUD（独立 memory.db）+ cc-memory-mcp
- **verify**：Bedrock 查询工具返回结果；MC PE 包列表/热重载可用；通知能发出；记忆可存取

### Phase 6 — 收尾 / 打包 / 测试 / 文档 ⬜ pending
**目标**：可发布质量。
- [ ] Dev/Release 隔离最终化；启动链工程化（PATH 加载/panic hook/boot marks）
- [ ] 关键路径测试（Go test + 前端 Vitest 覆盖 ViewModel）
- [ ] `task package` 各平台打包
- [ ] 更新 CLAUDE.md / README，迁移 cc-pane 的「铁律」文档
- **verify**：打包产物可运行；CI 级 `go vet`/`tsc --noEmit`/测试全绿

---

## 进度跟踪

| 阶段 | 状态 | 备注 |
|------|------|------|
| Phase 0 | ✅ complete | 改名 + Dev/Release 隔离，全验证通过 |
| Phase 1 | ✅ complete | ptyx ✅ / events ✅ / models ✅ / cliadapters ✅（18 守门 ok，MCP/hooks 延后 Phase 5）；subprocess git 白名单改判延后 Phase 4（层次修正） |
| Phase 2 | 🟡 待视觉验证 | **结构完成、可运行**：后端 service + 状态层(reducer/Context+reconcile) + TerminalView/SplitView/TabBar + PaneTree + App 接线（TerminalProvider/终端页/路由）+ MVP 启动入口（打开文件夹→裸 shell）。全程 typecheck/build/单测(37)/对抗审查全绿，但**前端从未在真 app 跑过**→ **loop 停在此里程碑，待用户首个视觉 checkpoint**（开文件夹→shell 输出→回显→分屏拖拽→关标签回收 PTY）。验证 OK 后转 ✅。侧边栏树/右键启动/CLI 选择/TabBar 拖拽重排 → Phase 3 或另列单元 |
| Phase 3 | 🔵 in_progress | 后端扁平 `project` service ✅(c8df705，19 测+对抗审查 wf_7bc251b0-c1f)；前端 project Context/侧边栏树 + 会话恢复/设置/i18n/窗口形态待做；**Workspace 持久化模型 = D9 决策门（文件存储 vs SQLite，到该单元停问用户，findings J8）** |
| Phase 4 | 🔵 in_progress | Git: gitx 只读+worktree·stash+network ✅（`bf279a6`/`c8f6d8c`/`4d6f9d4`，8 测）；Local History 存储基座 ✅（`43d6f80`，每项目 .xkpanes/history，D10 已定）+ 标签 ✅（`a089f0b`），7 测。**剩**：git 面板(结构化 status/stage/commit/diff，UI 门控) + gitsvc 服务层；Local History diff/标签/restore/最近变更/pruning/压缩 + 服务层。多运行时 / 启动配置 / Todo·Spec 未起 |
| Phase 5 | ⬜ pending | Bedrock/MCPE/notify/memory |
| Phase 6 | ⬜ pending | 收尾打包 |

状态图例：⬜ pending / 🔵 in_progress / 🟡 待人工验证 / ✅ complete / ⚠️ blocked

---

## 遇到的错误

| 错误 | 尝试次数 | 解决方案 |
|------|---------|---------|
| (暂无) | | |

---

## 已确认（用户 2026-05-29）

- [x] 应用名称与窗口标题：**XK-Pane**（D1 改名目标即 `XK-Pane`）
- [x] **所有特性都要**：弹出终端窗口 / 迷你模式 / 无边框浮动按钮全部纳入（移入 Phase 3 正式范围，非可选）
- [x] Bedrock 22MB 内嵌数据：**直接从 cc-pane 现有 `resources/` 拷贝复用**（Phase 5）
