# XK-Pane 忠实重建蓝图

> 来源：6 个 agent 并行通读 cc-pane 真实前端(`web/`) + Rust 后端 + XK-Pane 现状（~93 万 token，252 次工具调用），全部结论带 `file:line` 证据。本文件是重建的事实基线与计划，动手前请过目。

---

## 0. 核心认知更正（重要——推翻了之前 progress.md 的几处误判）

1. **「功能不对」的真因 = 终端缺 TUI 兼容层，不是终端没建。**
   XK 终端栈其实已有真实 PTY I/O（`internal/services/terminal/terminal.go` + `ptyx`，reader→batch→emit，UTF-8 安全）+ 前端骨架（reducer/Context/xterm view/split/tab，还带单测），裸 shell 和普通命令能跑。
   但 cc-pane 列为「不可省」的 9 项 TUI 能力 XK 几乎全缺：**CPR 光标查询 / Primary-DA + Kitty 键盘握手 / OSC 颜色查询 / alternate-buffer 处理 / 写入流控**。缺这些，**Claude / Codex 这类 agent CLI 必然握手卡死或渲染错乱**——这正是你看到的「功能不对」。

2. **「启动空壳」的真因 = 前端没接，不是后端没做。**
   后端 `terminal.CreateSession` 全字段就绪（cliTool / providerId / workspacePath / initialPrompt 都支持），8 个 CLI 适配器 + 双层 Provider 注入全在。
   但前端 `useTerminalPage.launchProject` 把 `cliTool` **写死成 `'none'`**（只开裸 shell），既没有右键启动菜单，后端也没有一个 Wails service 把 CLI 工具列表暴露给前端。

3. **Git：「对标 VSCode SCM」是超出 cc-pane 的新需求。**
   cc-pane **根本没有** VSCode 式 SCM/staging 面板——它的 git 就是 `pull/push/fetch/stash/clone/worktree` 几条裸命令，「看改动 / staging / commit」是用户在内置终端里手敲 `git add/commit`。
   XK 的 `gitsvc` 其实已是 cc-pane git 能力的**超集**且接线更全。
   **关键技术结论**：`go-git/v5` 做不了 staging area / 逐 hunk / 工作区-暂存区 diff / commit hooks，VSCode SCM 这套**必须走 git CLI**（`git status --porcelain=v2 -z`、`git diff [--cached]`、`git add/reset/restore`、`git apply --cached`、`git commit`）。
   （另：cc-pane 的 `DiffView.tsx` / `RecentChangesPanel.tsx` 属 **Local History**，不是 git。）

4. **Todo / Spec / Memory：cc-pane 里确实有 UI**（`web/components/todo/*`、`memory/MemoryManager.tsx`、`specService` + 项目右键「新建 Spec/查看 Specs」），但 Memory 的主要面孔是 **cc-memory MCP 服务**（给 AI 用），todo/spec 是 AI 工作流的二级功能。**按你的决定：前后端一起删除**，日后若需要再以正确形态（如 memory 走 MCP）接回。

5. **侧栏的真相 = Workspace→Project 两层树**，XK 当前是**扁平项目列表**，完全没有 Workspace 层。cc-pane 的右键启动菜单、收藏、会话分组都挂在这棵树上。

---

## 1. 六大子系统现状对照表

| 子系统 | cc-pane 真实形态 | XK 现状 | 关键差距 | 量级 |
|---|---|---|---|---|
| **侧栏 / Workspace 树** | 两层可折叠树（Workspace→Project），每项超大右键菜单，第三层挂「历史会话」子列表 | 扁平项目列表，无 Workspace 层、无树、无右键菜单 | 整个 Workspace 数据层 + service + 两层树 UI + 右键菜单 | XL |
| **启动菜单 + CLI + 启动档案** | 右键项目 → 打开终端 + 8 CLI（每个带「运行配置(Launch Profile)选择器」二级子菜单）+ 常用收藏 | 后端全就绪；前端写死裸 shell、无菜单、无 CLI 列表 service | 1 个 cliadapters Wails service + 右键菜单 UI + 透传 cliTool/providerId（+ Launch Profile 子系统） | L |
| **终端栈** | 能真跑 Claude/Codex 的 TUI；9 项兼容能力 + 内层分屏 + Tab 拖拽 + 回放 | 真 PTY I/O + 骨架已建，裸 shell 可跑 | **TUI 兼容层（CPR/DA/Kitty/OSC/buffer/流控）= 最高优先**；内层分屏、拖拽、回放为后续 | XL |
| **会话视图 + 最近启动 + 恢复** | 左栏：活跃会话 + 最近启动(按工作空间分组,可 resume) + 后台扫盘回填 resumeId | 后端 launch_history 模型字段齐；无扫盘/回填/恢复透传/前端 | 扫 ~/.claude·~/.codex 服务 + backfill + resumeId 透传链 + SessionsView 前端 | XL |
| **Git → VSCode SCM** | 仅裸命令，无 SCM 面板 | gitsvc 已是超集；只缺「变更文件维度」 | StatusFiles 分组 + per-file diff + stage/unstage/discard/commit（必须 git CLI）+ 前端 SCM 三段式 | L |
| **本地历史 + Home** | LH=自动捕获(file-watcher) + 版本时间线 + 4 tab + 标签 + 分支；Home=仪表盘 | LH 后端忠实甚至超前，前端跑偏成单 diff 页；Home 还是脚手架 demo | LH 前端重构 + **自动捕获 watcher**；Home 整页重做 | XL |

---

## 2. 分阶段重建计划（按「最快拿到能用的真核心」排序）

### P1 — 让终端真能跑 Claude/Codex（headline 闭环，直接解决「功能不对」）
- 后端：新增 **cliadapters Wails service**（包装现有 `cliadapters.Default()/Registry`，暴露 `ListTools()/DetectAll()`，与 cc-pane `list_cli_tools` 对齐；与 terminal 共享单例 registry 避免重复探测）。
- 前端：**项目右键启动菜单**（打开终端 + 8 CLI；MUI `Menu`+`onContextMenu` anchorPosition 自造）。
- 前端：`useTerminalPage.launchProject` **去掉写死的 `'none'`**，接受并透传 `cliTool/providerId/workspacePath/initialPrompt`；同步给 `NewTabOptions/Tab/sessionReconcile/CreateSpec` 补 `providerId` 透传。
- 前端：**终端 TUI 兼容层**——移植 5 个纯函数模块（`terminalCpr / terminalCapabilityReports / terminalOscColor / terminalKeyboard / terminalBufferMode`）+ 写入流控；在 `useTerminalView` mount 时注册 CSI/OSC handler 回写 PTY。
- **验证**：`wails3 dev` 真跑 Claude Code（你可手测，因为你说「能进界面」）。
- 依赖：**无新依赖**。量级 L+。

### P2 — 清场 + Git SCM + Home 仪表盘
- **删除 Todo/Spec/Memory**（前后端，见 §4）。
- **Git → VSCode SCM**：`StatusFiles`(porcelain v2) + `DiffFile`(file, staged) + `Stage/Unstage/Discard/Commit`（全 git CLI，扩 `cli.go` allowlist）；前端 GitPage 改三段式（commit 框 + 变更分组列表 + 复用现有 `DiffView`）。**P0 = per-file**，per-hunk 暂存留 **P2.1**（最贵）。
- **Home 仪表盘**：`HomePage` 重做为问候头 + 快捷动作 + 最近项目卡片 + 活跃会话 + 环境检测（复用 launchhistory/session/cliadapters）。
- 依赖：可能需 toast（notistack 或 MUI Snackbar）做操作提示。量级 L。

### P3 — Workspace 工作空间树 + 会话视图 + 恢复
- **Workspace 层**：新增 Workspace/WorkspaceProject 模型 + workspace service + 两层折叠树 + 右键菜单（把 P1 的启动菜单挂上去）。
- **SessionsView**：活跃会话（前端树派生）+ 最近启动（分组/折叠/resume）+ 扫盘服务 + backfill + `resumeId` 透传链。
- 依赖：拖拽排序可选 @dnd-kit。量级 XL。

### P4 — Local History 忠实化
- 前端：版本时间线 + 4 tab（diff/content/deleted/project-restore）+ 标签 UI + 分支筛选（后端基本已就绪）。
- 后端：**自动捕获 file-watcher**（fsnotify + debounce + 分支感知 + 忽略规则 + 单写者）——LH 的灵魂。
- 依赖：**fsnotify**。量级 L–XL。

### P5 — 终端进阶
- 内层终端分屏、Tab 拖拽（@dnd-kit）、会话回放/跨重启恢复（ReplayBuffer）。量级 XL。

---

## 3. 需你拍板的决策（汇总自 6 份规格）

| # | 决策 | 我的建议 |
|---|---|---|
| Q-顺序 | 阶段顺序 / 第一目标 | **P1 先**（终端真跑），最快验证核心 |
| Q-依赖 | 新依赖授权：@dnd-kit / fsnotify / toast 库 | P1 不需要；P4 需 fsnotify；P2 可能需 toast；P5 需 @dnd-kit。建议按相到了再授权 |
| Q-WS时机 | Workspace 树现在做还是 P3 | **P3**（先让核心闭环，再加组织层） |
| Q-WS存储 | Workspace 走 SQLite（XK 铁律）还是文件 `workspace.json`+`projects.csv`（忠实 cc-pane，CLI/agent 可读 csv） | 倾向 SQLite，但若你的 CLI hook 依赖 projects.csv 文件则需保留文件侧产物 |
| Q-Git范围 | per-hunk 逐块暂存要不要做 | per-file（P0）先做，per-hunk（P2.1）后补 |
| Q-Git上下文 | GitPage 仓库上下文：下拉选项目 vs 绑当前活动项目 | 绑活动项目（避免又成孤立页） |
| Q-LH入口 | HistoryPage 保留独立页 vs 改成文件级 Dialog | 两者并存：页=全项目浏览器，Dialog=文件级快捷入口 |

---

## 4. Todo / Spec / Memory 删除清单（P2 执行，可逆性：删前 git 已留痕）

**前端**：`pages/{TodoPage,SpecsPage,MemorysPage}/`、`services/{todo,spec,memory}/`、`todo/`(TodoProvider)；`App.tsx` 去掉 3 个 `register*Locales` + `TodoProvider` 包裹；`routes.tsx` 去掉 3 条路由 + 图标 import；`i18n/locales/{zh-CN,en-US}.ts` 去掉 `route.{todo,specs,memory}` + 命名空间；`services/index.ts` 去桶导出。
**后端**：`internal/services/{todo,specsvc,memorysvc}/`、`internal/storage/models_memory.go`、`models_ccpane.go` 里的 Todo/Spec 模型；`storage/models.go` 的 `AllModels` 去 `&Todo/&Spec/&Memory{}`；`app.go` 去 3 个 service 注册 + specSvc 构造注入 + todoSvc。
**收尾**：`wails3 generate bindings` 重生 + `pnpm typecheck` + `go build/vet/test`。

---

## 5. 不变的铁律（贯穿所有阶段）
SQLite-only 持久化（禁 localStorage）；敏感字段（LastPrompt/key/token）落库前 `cryptox` 加密；HTTP 走 `httpx`、子进程走 `procx`、日志走 `logx`、配置写 `filex`；前端子进程经 `subprocess` 白名单；对话框走 `NativeDialogs`（禁 alert/confirm/prompt）；图标 `@mui/icons-material *Rounded`（禁 emoji/Unicode/第三方）；人类可见字符串走 `t()`；颜色取 `theme.palette.foundation.*`（禁硬编码十六进制）；事件名集中注册在 `internal/events`（禁前端自造 window 事件）。
