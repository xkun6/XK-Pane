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

### Phase 1 — 后端地基（PTY / 子进程 / CLI 适配器 / 数据模型 / 事件）🔵 in_progress
**目标**：铺好移植所需的后端底座，无 UI 也能单测。
- [x] `internal/utils/ptyx`：跨平台 PTY 门面（go-pty v0.2.3）。Windows ConPTY + JobObject(CREATE_SUSPENDED→assign→ResumeThread)；Unix 不叠 Setpgid（go-pty 强制 Setsid）kill 用 `Kill(-pid,SIGKILL)`。✅ **R1 spike 在 Win11 实跑通过**（`TestSpawnEchoProducesOutput`/`TestResizeNoError` PASS；Unix 交叉编译 exit=0）
- [ ] `internal/services/cliadapters`：8 个 CLI 适配器（claude/codex/gemini/kimi/glm/opencode/cursor/reclaude）的 build_command 逻辑 + 路径字段铁律（D7）+ 守门测试（27 个，findings J4）。注意 GLM 可执行名是 `crush`
- [ ] `internal/storage/models_*.go`：移植核心 GORM 模型并追加 AllModels（findings J3 已有完整 struct 草案；data.db 表 + 敏感字段 cryptox 加密 hooks）
- [ ] `internal/events`：注册终端输出/退出/状态等类型化事件（`terminal:output`/`:status`/`:exit`）
- [ ] `internal/services/subprocess`：扩展白名单（git 各子命令，供 go-git fallback）
- **verify**：`go test ./internal/...` 通过；✅ ptyx 已起 shell 读到输出；适配器守门测试覆盖 path 语义；AutoMigrate 建表测试

### Phase 2 — 终端核心闭环（首个可运行里程碑）⬜ pending
**目标**：能在某项目目录下用内置终端跑起一个 CLI 并交互——MVP runnable。
- [ ] 后端 `terminal_service`：PTY 会话注册表 + I/O 隧道（Emit 事件）+ resize/kill + launch_cwd 上报
- [ ] 前端保留组件：`TerminalView`(xterm.js)、自实现 `SplitView` 分屏、`TabBar`(@dnd-kit)
- [ ] 前端 MVVM：终端面板 View/VM/Style；`TerminalContext`（面板树/活动会话，替代 usePanesStore）
- [ ] 侧边栏工作区/项目树（最小版）+ 右键启动
- **verify**：打开项目→新建终端标签→看到 shell/CLI 输出→输入回显；分屏可拖拽；关闭标签干净回收 PTY

### Phase 3 — 工作区/项目/会话/设置/主题·i18n ⬜ pending
**目标**：补齐管理面与配置面，达到日常可用。
- [ ] Workspace/Project CRUD、别名、Provider 绑定（后端 service + 前端 Context + 侧边栏）
- [ ] 启动历史（launch_history）+ 会话恢复（terminal_sessions，跨重启）
- [ ] Provider 管理（多 API Provider，敏感字段 cryptox 加密）
- [ ] 设置页：按 SettingsPage 子目录隔离规范新增 `provider/terminal/shortcuts/proxy/about` 子页
- [ ] i18n：移植 cc-pane 12 个命名空间到 foundation 注册式 i18n（zh-CN/en-US）；主题沿用 foundation
- [ ] 窗口形态特性（用户要求全要）：弹出终端窗口(mode=popup)、迷你模式、无边框浮动按钮
- **verify**：重启后会话/布局恢复；设置改动落 SQLite 并生效；中英切换无硬编码字符串；弹出/迷你/无边框可切换

### Phase 4 — 常用增强（Git / Local History / 多运行时 / 启动配置 / Todo·Spec）⬜ pending
**目标**：覆盖 cc-pane 高频增强功能。
- [ ] Git 集成（go-git + 必要时 git CLI）：status/branch/pull/push/fetch/stash + Worktree
- [ ] Local History：file_versions 表 + 快照/Diff/标签/分支感知（独立 history.db）
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
| Phase 1 | 🔵 in_progress | ptyx ✅（R1 已实跑验证）；cliadapters/models/events/whitelist 待做 |
| Phase 2 | ⬜ pending | 终端闭环（MVP runnable） |
| Phase 3 | ⬜ pending | 管理面 + 设置 |
| Phase 4 | ⬜ pending | 常用增强 |
| Phase 5 | ⬜ pending | Bedrock/MCPE/notify/memory |
| Phase 6 | ⬜ pending | 收尾打包 |

状态图例：⬜ pending / 🔵 in_progress / ✅ complete / ⚠️ blocked

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
