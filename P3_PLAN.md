# P3 实施计划 — Workspace 两层树 + SessionsView/resume

> 产出方式：9-agent workflow（7 路并行通读 cc-pane 目标形态 + XK 现状基座 → 综合 → critic 审查），全部结论带 `file:line`。本文件已把 critic 的 7 项修订**直接纳入计划正文**（不是附录）。
> 基准：cc-pane 源 `J:/Program/Agent/cc-pane`，XK 基座 `J:/Program/Agent/XK-Pane`。
> 切分（用户已定）：**P3a = Workspace 两层树（先做，执行就绪）** → **P3b = SessionsView + resume（后做，详细骨架）**。

---

## ✅ 执行状态（清空上下文后从这里看 P3a 落地情况）

- **P3a 全部完成**，6 个提交 `d7c54b8..d1213e4`，逐单元全绿（go test + pnpm typecheck + pnpm build）：
  - `d7c54b8` P3a-1 数据模型（`internal/storage/models_workspace.go` 两表 + 联合唯一 `idx_ws_proj_path` + AutoMigrate 守门测试；数据存储页 lang 补 2 LabelKey）
  - `20a9f7a` P3a-2 `internal/services/workspace`（12 方法 + DTO-free 直返模型 + 归一化幂等去重 + 4 测试 + app.go 注册）
  - `41167c1` P3a-3 前端 `services/workspace/WorkspaceService.ts` + `workspace/WorkspaceProvider` 同构 Context（挂在 TerminalProvider 内）
  - `4e1bec7` P3a-5a 通用 `components/TextInputDialog`（补 NativeDialogs 缺的文本录入）
  - `4f7761c` P3a-4/5b `components/WorkspaceTree`（两层折叠树 + 右键菜单 CLI 启动/导入/重命名/别名/移除/复制路径 + 替换并**删除 ProjectPanel**）
  - `d1213e4` P3a-6 @dnd-kit 拖拽排序（工作空间组内 + 项目工作空间内）
- **拍板落地**：#2 资源管理器打开=**砍掉**（菜单不含，归后续）；#3 排序=**@dnd-kit 拖拽**（已引入 core 6.3.1 / sortable 10.0.0 / utilities 3.2.2）；#1 不生成文件产物、#4 不做迁移快照、#5 resumeId 明文、#6 项目软关联（`launchProject` 用节点 id 作 `projectId`，不外键全局 projects）、#7 新建文本对话框、#9 按需扫盘 —— 均按建议默认。
- **关键实现说明**：WorkspaceProject 自带 path 软关联全局 projects；`launch` 用节点（workspace/project）自身 id 作 LaunchHistory.projectId；workspace 无 path 时启动项 disabled；模型保留完整 schema（含无 UI 的 ProviderID/Hidden/DefaultEnvironment/LaunchProfileID 列），service 只实现 P3a 消费的方法。
- **待手测**（desktop 窗口，只能用户做）：建/删/收藏/折叠/导入项目/右键启动 CLI/重命名/别名/移除/拖拽排序。
- **下一步**：P3b（见下文骨架），动手前先手测 P3a。

---

# P3a — Workspace 两层树（执行就绪）

## 一、数据模型

### 决策：SQLite-only 关系建模，不照搬 cc-pane 的 `workspace.json` 文件形态

cc-pane 每个 workspace 一个 `<data>/workspaces/<name>/workspace.json`，项目**内联**在 `Workspace.projects: Vec<WorkspaceProject>`（`cc-panes-core/src/models/workspace.rs:50-77`）。XK 铁律 SQLite-only，且已有独立 `Project` 表（`models_ccpane.go:51-58`）。因此 **拆成两张关系表**：`Workspace` + `WorkspaceProject`（外键 `workspace_id` + `sort_order`），不内联 JSON。

> 新文件头注释须写明偏离（仿 `models_ccpane.go:5-15` 风格）：内联 `Vec<WorkspaceProject>` → 两张关系表；cc-pane 的 workspace name 同时是磁盘目录名 → XK 用 `id` 主键，`name` 仅显示+唯一约束，无磁盘目录语义。
>
> **模型字段保持忠实的完整 schema**（含暂不接 UI 的列），但 **service 方法 / UI 只实现 P3a 真正消费的部分**（见 critic S3）。列是 AutoMigrate 友好的零成本忠实映射；无调用方的 service 方法才算投机代码。

### 模型 1：`Workspace`（对照 `workspace.rs:50-77` / `web/types/workspace.ts:44-59`）

| XK 字段 (GORM) | cc-pane 来源 | P3a UI | 备注 |
|---|---|---|---|
| `ID string` `gorm:"primaryKey;size:36"` | workspace.rs:53 | — | UUID |
| `Name string` `not null;uniqueIndex` | workspace.rs:54 | ✓ | 去掉目录名语义，仅显示+唯一 |
| `Alias *string` | workspace.rs:55 | ✓ | |
| `Path *string` | workspace.rs:63 | ✓ | 工作空间根目录，可空 |
| `ProviderID *string` `index` | workspace.rs:59 | 列留，无 UI | Provider 绑定 UI 归后续 |
| `LaunchProfileID *string` | workspace.rs:61 | 列留，无 UI | LaunchProfile 子系统未落地 |
| `DefaultEnvironment string` `not null;default:'local'` | workspace.rs:65 / enum:5-12 | 仅 local | service 校验枚举；UI 只给 local |
| `Pinned bool` `not null;default:false;index` | workspace.rs:71 | ✓ | 收藏/置顶 |
| `Hidden bool` `not null;default:false` | workspace.rs:73 | 列留，无 UI | 隐藏 UI 归后续 |
| `SortOrder *int` | workspace.rs:75 | 列留 | None 垫底；排序见拍板点 |
| `CreatedAt time.Time` | workspace.rs:57 | — | XK 用 time.Time |
| `UpdatedAt time.Time` | — | — | GORM 自管 |

**裁剪（不落地，归 Phase 后续）**：`wsl`(workspace.rs:67)、`ssh_launch`(:69) 嵌套配置随 WSL/SSH 启动环境子系统进入；`WorkspaceEnvironmentPanel` 不纳入 P3a（依赖 `discover_wsl_distros`/SSH 列表，XK 均无）。

### 模型 2：`WorkspaceProject`（对照 `workspace.rs:35-47` / `web/types/workspace.ts:35-42`）

| XK 字段 (GORM) | cc-pane 来源 | P3a UI | 备注 |
|---|---|---|---|
| `ID string` `primaryKey;size:36` | workspace.rs:38 | — | UUID |
| `WorkspaceID string` `not null;index` | 反向外键（XK 关系化新增） | — | 联合唯一见下 |
| `Path string` `not null` | workspace.rs:39 | ✓ | 归一化后存 |
| `Alias *string` | workspace.rs:40 | ✓ | |
| `LaunchProfileID *string` | workspace.rs:41 | 列留 | |
| `SortOrder int` `not null;default:0` | （cc-pane 数组顺序） | 列留 | |
| `CreatedAt time.Time` | — | — | |

- **联合唯一索引** `uniqueIndex:idx_ws_proj_path`（`WorkspaceID` + 归一化 `Path`），等价 cc-pane `add_project` 去重（`workspace_service.rs:330-340` + `normalize_project_path:312-319`）：windows 转小写、统一 `/`、去尾斜杠；归一化在 service 层做，存归一化值。
- **裁剪**：`wsl_remote_path`(:43)、`ssh`(:45) 随 WSL/SSH 子系统进入；`add_ssh_project` 整体归后续。

### `AllModels` 注册

新建 `internal/storage/models_workspace.go`（仿文件头注释风格），`init()` 里 `AllModels = append(...)`：

```go
ModelDescriptor{Model: &Workspace{}, TableName: "workspaces", LabelKey: "workspaces", Clearable: false},
ModelDescriptor{Model: &WorkspaceProject{}, TableName: "workspace_projects", LabelKey: "workspaceProjects", Clearable: false},
```

- `Clearable: false`（组织结构，不进设置页一键清空，同 `projects` 档 `models_ccpane.go:19`）。
- 显式 `TableName()` 防复数化（仿 `models_ccpane.go:130-134`）。
- **检查点**：加表零迁移文件，重启 AutoMigrate（`models.go:82-84`）。

### projects.csv / workspace.json 取舍

cc-pane 的 `.ccpanes/projects.csv` **无任何代码读取**（全仓 grep 零命中），纯给 CLI hook/agent 看；真正被 CLI hook 程序化消费的是 `workspace.json`（`cc-panes-cli-hook/src/session_start.rs:316-358`）。**P3a 不生成任何文件侧产物**（XK 无 CLI hook 二进制消费方 + 生成 csv 要跑 git 子进程 + SQLite-only 铁律）。→ 拍板点 1。

---

## 二、后端 service

新建 `internal/services/workspace/workspace.go`，持 `*storage.Holder`（不 capture `*DB`，仿 `project.go:42-52`）。

**仅实现 P3a UI 消费的方法**（critic S3：删掉无调用方的 `MoveProject`/`Reorder`/`ReorderProjects`/`SetProvider`/`SetHidden`，随对应 UI 后续再加）：

```go
type Service struct { holder *storage.Holder }
func New(h *storage.Holder) *Service { return &Service{holder: h} }
func (s *Service) db() *gorm.DB { return s.holder.Current().GORM }

type WorkspaceDTO struct {
    ID, Name string; Alias, Path, ProviderID *string
    DefaultEnvironment string; Pinned, Hidden bool; SortOrder *int
    Projects []WorkspaceProjectDTO `json:"projects"`
    CreatedAt, UpdatedAt time.Time
}
type WorkspaceProjectDTO struct { ID, Path string; Alias *string; SortOrder int }

func (s *Service) List(ctx) ([]WorkspaceDTO, error)  // pinned 优先 → sort_order(None 垫底) → created_at（对照 workspace_service.rs:193-206），预加载 projects
func (s *Service) Create(ctx, name string, path *string) (WorkspaceDTO, error)  // name 唯一校验
func (s *Service) Rename(ctx, id, name string) error  // 仅改 name 列（XK 无目录 rename）+ 唯一校验
func (s *Service) Delete(ctx, id string) error  // 删 workspace + 级联删 workspace_projects（事务）
func (s *Service) SetAlias(ctx, id string, alias *string) error
func (s *Service) SetPath(ctx, id string, path *string) error  // nil = 清除路径
func (s *Service) SetPinned(ctx, id string, pinned bool) error
func (s *Service) AddProject(ctx, workspaceID, path string) (WorkspaceProjectDTO, error)  // 归一化去重（联合唯一）
func (s *Service) RemoveProject(ctx, projectID string) error
func (s *Service) SetProjectAlias(ctx, projectID string, alias *string) error
```

**与全局 `Project` 表关系（决策）**：`WorkspaceProject` **自带 path + 软关联**全局 `projects`（不外键 `projects.id`）。理由：避开 `Project.Add` 的"已存在报错"（`project.go:67`）阻断幂等加项目；`AddProject` 时可顺手 upsert 全局 `projects` 以便 `launchProject` 拿稳定 `projectId`。→ 拍板点 6（建议软关联）。

**事件**：P3a **不加新事件**。cc-pane 的 `WORKSPACES_CHANGED` watcher（`workspace_service.rs:139`）是为"外部改 workspace.json，UI 自刷"服务的；XK SQLite-only 无此场景，前端 mutation 后自己 `refresh()`（仿 `ProjectProvider.tsx:38-41`）。

**app.go 注册**：`internal/app/app.go:84-98` 的 Services 数组加 `application.NewService(workspace.New(holder))`（无需 AttachApp）。改完跑 `wails3 generate bindings`。

---

## 三、前端

### 复用 / 重构 / 不动

| XK 现有 | 处置 | 依据 |
|---|---|---|
| 全局 `Sidebar`（64px 导航栏）`Sidebar.tsx:19` | **不动**（只渲染路由图标） | [xk:frontend]§1a |
| `ProjectPanel`（240px 扁平项目表）`ProjectPanel.tsx:33` | **重构为 WorkspaceTree** | P3 衔接小结 |
| `ProjectPanel` 右键启动菜单 `ProjectPanel.tsx:40-158` | **逻辑整体复用**，迁到节点 | P1 产物 |
| `project/ProjectContext` `ProjectProvider.tsx:23` | **保留**（全局项目登记表），新增并列 `WorkspaceProvider` | |
| `provider/ProviderContext` | **保留** | |
| `PaneTree`（外层窗口分屏树）`PaneTree.tsx:7` | **不动**（窗口布局层，与 Workspace 树无关） | [xk:frontend]§6 |
| `useTerminal().launchProject` `TerminalContext.tsx:93` | **复用**为节点启动唯一入口 | [xk:frontend]§4 |

> **WorkspaceItem 收藏星/活跃指示是 XK 主动增强**：cc-pane WorkspaceItem 不渲染这些（[cc:ws-tree-ui]§2）。P3a 给 pinned 加可见星标 + 右键"置顶/取消置顶"（cc-pane 有 service 无 UI 入口 [cc:ws-tree-ui]§5）；**活跃指示归 P3b**（依赖会话派生 + status store，见 P3b-3a）。

### 新增/改动文件清单（XK 路径，圆角：**按钮 6 / 容器 8**，取色 `theme.palette.foundation.*`）

**service 封装**
- 新增 `frontend/src/services/workspace/WorkspaceService.ts`（包 `@bindings/xkpane/internal/services/workspace` + 类型）
- 改 `frontend/src/services/index.ts`（桶 re-export）

**全局 Context**
- 新增 `frontend/src/workspace/{WorkspaceProvider.tsx, types.ts, index.ts}`（仿 `ProjectProvider` 同构）：`useWorkspaces()` 暴露 `workspaces/loading/refresh/create/rename/delete/setAlias/setPath/setPinned/addProject/removeProject/setProjectAlias`；mutation 后 `refresh()`，**禁 localStorage**
- 改 `frontend/src/App.tsx`：挂 `<WorkspaceProvider>`（在 `TerminalProvider` 之内，以便 launchProject 可用）

**树组件**（新目录 `frontend/src/components/WorkspaceTree/`，View/VM/Style/lang 三件分离）
- `WorkspaceTree.tsx`：pinned 分组置顶 + 普通组（XK 增强）；空态 `t('workspaceTree.empty')`；底部"新建工作空间"虚线按钮（`AddRounded`）
- `WorkspaceItem.tsx`：`ChevronRightRounded`(展开转90°) + 名(`alias||name`) + 收藏星 `StarRounded`/`StarBorderRounded`(toggle pinned) + 项目计数 pill；展开态边框+accent（仿 `WorkspaceItem.tsx:353-358`）；**活跃指示留空位**（P3b 填）
- `WorkspaceProjectItem.tsx`：第二层 `FolderRounded` + 名 + 路径 + 右键菜单
- `useWorkspaceTree.ts`(VM)：折叠状态 `expandedWorkspaceId` 单选互斥（仿 `useWorkspacesStore.ts:297-303`，存组件本地/Context，**禁 localStorage**）；VM 无 t()/无 JSX/经 services
- `*.styles.ts`、`lang/{zh-CN,en-US}.ts` + `lang/index.ts`(`registerWorkspaceTreeLocales()`，命名空间 `workspaceTree.*`)

**右键菜单**（MUI `<Menu>` anchorEl 行 DOM，仿 `ProjectPanel.tsx:127`，对照 cc-pane `WorkspaceItem.tsx:409-586` 做 XK 裁剪）

工作空间节点：

| # | key | 图标 | 动作 | 来源/处置 |
|---|---|---|---|---|
| 1 | `openTerminal` | `TerminalRounded` | `launchProject(cliTool='none')` 对 ws.path | cc-pane:471 |
| 2 | 每个已检测 CLI（claude/codex/reclaude…） | `TerminalRounded` | `launchProject(tool.id)`，`disabled=!installed` | **复用 P1** `ProjectPanel.tsx:142-157`+`useCliTools()`；LaunchProfile 三段式子菜单裁剪 |
| 3 | `pin`/`unpin` 置顶 | `StarRounded` | `setPinned(id,!pinned)` | **XK 增强** |
| 4 | `importProject` 导入项目 | `FolderRounded` | `NativeDialogs.openDirectory()`→`addProject` | cc-pane:498 |
| 5 | `setAlias` | — | 文本对话框→`setAlias` | cc-pane:535 |
| 6 | `renameWorkspace` | — | 文本对话框→`rename` | cc-pane:538 |
| 7 | `setPath`/`clearPath` | — | `openDirectory`→`setPath` / `setPath(nil)` | cc-pane:521-528 |
| 8 | `openFolder` 在资源管理器打开 | `FolderOpenRounded` | **→ 拍板点 2**（XK 无 reveal 能力） | cc-pane:481 |
| 9 | `deleteWorkspace`（destructive） | `DeleteRounded` | `NativeDialogs.confirm`→`delete` | cc-pane:583 |

项目节点：`openTerminal` / 每个 CLI / `copyPath`(绝对·相对，`navigator.clipboard`，cc-pane:263-275) / `setAlias`(`setProjectAlias`) / `removeProject`(confirm)。

**裁剪明示**（归 Phase 后续）：常用收藏启动子菜单（依赖 launchFavorites）、扫描导入/Git克隆/SSH项目、运行环境面板、Hooks 子菜单、迁移 To WSL、文件浏览器/Worktree/Spec。

**对话框**
- 文本输入（新建/重命名/别名）：`NativeDialogs` 无文本输入（仅 open/save/dir/confirm/info/error，`NativeDialogs.ts:50-153`）。**新建应用内 MUI `<Dialog>` 文本输入组件**（拆为独立单元 P3a-5a，critic S1）。**合规**：原生对话框铁律只覆盖 打开/保存/选目录/确认/提示，文本录入用应用内 Dialog 不违规。→ 拍板点 7（确认新建）
- 删除/移除确认：`NativeDialogs.confirm`（仿 `useProjectPanel.ts:46`）

**i18n**（`workspaceTree.*`，配 zh/en，`App.tsx:23-29` 注册 `registerWorkspaceTreeLocales()`，公用 locale 之后）：`empty / newWorkspace / openTerminal / launch.* / pin / unpin / importProject / setAlias / renameWorkspace / setPath / clearPath / openFolder / deleteWorkspace(+Title/Confirm) / removeProject(+Title/Confirm) / copyPathAbsolute / copyPathRelative / projectCount / dialog.{createTitle,namePlaceholder,browse} / error.*`

**桶导出补缺**（critic G5）：`frontend/src/terminal/index.ts` re-export `LaunchProjectInput`（P3a-5 节点构造它要 import）。

**LabelKey 现状**（critic G4，纠正"只缺 2 个"误判）：`SettingsPage/database/lang/{zh-CN,en-US}.ts` 的 `settings.database.tables.*` **当前只有 `preferences`/`appSettings`**，而 `AllModels` 早注册了 `projects/launchHistory/terminalSessions/taskBindings/providers` 五表——这些 key **本就缺失**（回退到字面量）。新增 `workspaces`/`workspaceProjects` 时如实说明，**不强制本期补全无关 5 表**（简洁铁律）。

**bindings 重生**：后端加 service 后 `wails3 generate bindings`。

---

## 四、可提交单元切分（P3a，已纳入 critic S1/S4）

| 单元 | 范围 | 依赖 | 验收 |
|---|---|---|---|
| **P3a-1 数据模型** | `models_workspace.go`(两模型+TableName)；追加 AllModels；数据存储页 lang 补 2 个 LabelKey | 无 | `go build`+`go vet`；删 db 重启后 `workspaces`/`workspace_projects` 表自动建出；加 `models_ccpane_test.go` 风格 AutoMigrate 断言 |
| **P3a-2 后端 service** | `workspace/workspace.go`(全方法+DTO+归一化去重)；app.go 注册；bindings 重生；**`workspace_test.go`** 覆盖 List 排序 + AddProject 去重（critic S4） | P3a-1 | `go build`+`go vet`；test 绿；bindings 生成 `workspace/` |
| **P3a-3 前端 service+Context** | `WorkspaceService.ts`+桶；`WorkspaceProvider` 同构；App.tsx 挂载 | P3a-2 | `pnpm typecheck`；`useWorkspaces()` 能拉列表 |
| **P3a-4 两层树 UI（无菜单）** | `WorkspaceTree/WorkspaceItem/WorkspaceProjectItem`+VM+Style+lang+注册；折叠/展开/收藏星/计数 pill；`TerminalPage` 里 `<ProjectPanel>` 挂载点换 `<WorkspaceTree>` | P3a-3 | `pnpm typecheck`；手测：展开收起、点星 toggle pinned 持久、pinned 置顶、空态 |
| **P3a-5a 文本输入对话框**（critic S1 拆出） | MUI 受控 `<Dialog>` 文本输入组件（弹出/输入/确认取消/Enter 提交） | P3a-4 | `pnpm typecheck`；手测独立弹出 |
| **P3a-5b 右键菜单+接线** | 两套右键菜单（复用 P1 CLI 逻辑）；NativeDialogs 确认；节点点 CLI→`launchProject`；补 `terminal/index.ts` 导出 | P3a-5a | `pnpm typecheck`；手测全菜单项 + 点 CLI 真起会话跳 terminal 页 |

---

# P3b — SessionsView + resume（详细骨架）

## 一、resume 透传链（critic G1/C1 重写：**6 处链路**，核心是 reconcile effect）

**已核实**：XK 的 session 创建**唯一发生点**是 `TerminalContext` 的 reconcile effect（`TerminalContext.tsx:160-199`），其 `TerminalService.create({projectPath, workspacePath, cliTool})` 硬编码三字段——**`TerminalView`/`useTerminalView` 全程不创建 session**（计划原心智模型沿用了 cc-pane 的 `TerminalView.tsx:1060-1073` 直接 createSession，XK 已改）。

**后端断点（必补，2 处）**：
1. `CreateSpec` 加 `ResumeID string \`json:"resumeId"\``（`terminal.go:138-153`）
2. `resolveLaunch` 的 `cctx` 字面量加 `ResumeID: spec.ResumeID`（`terminal.go:313-322`）
→ adapter 层 `if ctx.ResumeID != ""` **已就绪**（`claude.go:69-71`→`--resume <id>`；`codex.go:58-60`→`resume <id>`；`reclaude.go:67-69`），**不改 adapter**。裸 shell 早返回分支（terminal.go:302-306）无需处理。

**前端断点（必补，6 处链路，critic G1）**：
1. `LaunchProjectInput` 加 `resumeId?: string`（`TerminalContext.tsx:38-44`）
2. `launchProject` 派发 `addTab` 时 opts 带上 `resumeId`（`TerminalContext.tsx:97-101`，当前仅 `{projectId,projectPath,cliTool,title}`）
3. `NewTabOptions` 加 `resumeId`（`reducer.ts:30-36`）
4. `addTab` reducer→`createTab` 传 `resumeId`（`reducer.ts:165-169`）
5. `Tab`/`TerminalPaneLeaf` 加 `resumeId` 字段（`paneTree.ts:32-62`、`types.ts:21-31,45-61`，已标 Phase3 待补）
6. **reconcile effect** 的 `TerminalService.create` 入参读 `tab.resumeId`（`TerminalContext.tsx:166-170`）← **最易遗漏**
- **cliTool 兜底**：resume 时 cliTool 不能 'none'，须带会话引擎对应 tool（仿 cc-pane `App.tsx:608` `effectiveCliTool = opts.cliTool ?? (resumeId ? defaultTool : undefined)`）

## 二、扫盘回填（后端 service）

新建 `internal/services/sessionscan/sessionscan.go`，持 `*storage.Holder`：
- **Claude 扫盘**（对照 `claude_session_service.rs:116`+`claude_path.rs:5`）：扫 `~/.claude/projects/<编码目录>/*.jsonl`（目录名 `: \ / _`→`-`）；id=文件名 stem；mtime；description=前 100 行首条 `type=="user"` 无 `data` 的 prompt 截 80 字
- **Codex 扫盘**（对照 `codex_session_service.rs:186`）：递归 `~/.codex/sessions/**/*.jsonl`；id+cwd 从首 20 行 `session_meta`；description 前 200 行 user message
- **backfill**：扩 `launchhistory` service（当前仅 `Record`+`List`）补 `UpdateSessionID`/`TouchBySessionID`(resume 去重，对照 `history_repo.rs:478`)/`UpdateLastPrompt`；`RecordSpec` 补 `ResumeSessionID/LastPrompt/ProviderID/LaunchCwd` 字段
- **触发**：P3b **先做按需扫盘**（前端打开 SessionsView 调一次 `ListProjectSessions(path)`）；轮询 backfill（cc-pane 150×2s 兜底 Codex 延迟写）归后续 → 拍板点 9
- **cryptox**（铁律）：`LastPrompt` 落库**必须** `cryptox.EncryptString`（`models_ccpane.go:70` TODO），读出（Popover 展示）`DecryptString`；用 **service 层加解密**（仿 provider `models_ccpane.go:31-33`，非 GORM 钩子，critic V2）。`ResumeSessionID` **不加密**（CLI resume 必需明文，与 cc-pane 一致）→ 拍板点 2

## 三、SessionsView 前端

落点：复用现有 `history` 路由重做 vs 新路由 → 拍板点 8。

- **(a) 活跃会话派生**（对照 `SessionsView.tsx:11-72`）：从 `useTerminal().rootPane` 递归收集 panel tabs，过滤 `tab.sessionId && contentType==='terminal'`，派生 `{tabId,paneId,title,sessionId,status}`。
  - **⚠ critic G2 前置单元 P3b-3a**：status 数据源在 XK **不存在**——`TerminalService.onStatus` 已定义（`TerminalService.ts:123-132`）但**零消费**，`useTerminalView` 只订阅 `onOutput`/`onExit`（:272-295）。必须**新建订阅 `terminal:status`（`events.go:42`）的状态 store/Context（非事件）+ 在 Context/TerminalView 层接订阅写入**，否则 status 恒 `idle`、活跃指示永不点亮。这是 WorkspaceItem 活跃指示的数据源。
- **(b) 最近启动分组/折叠/resume**（对照 `RecentLaunches.tsx`+`groupLaunches.ts:12-39`）：`LaunchHistoryService.list(30)`→`filter(resumeSessionId 非空)`→按 `workspaceName||未分组` 入 Map（ungrouped 沉底+localeCompare）；折叠组件本地 `Set`（默认全展，**禁 localStorage**）；行=`projectName + id.slice(0,8) + lastPrompt.slice(0,40) + 相对时间`
- **(c) SessionSubList**（对照 `SessionSubList.tsx`）：`sessionscan.ListProjectSessions(path)`→`ResumeSession[]`；右键 Resume / Resume with reclaude(仅 claude 会话) / Copy id；Resume→取裸 id（校验非空·不含 `:`）→`launchProject({resumeId, cliTool})`
- **(d) ResumeDetailPopover**（对照 `ResumeDetailPopover.tsx`）：MUI `<Popover>`：项目名+path+id(首8…尾8+复制)+启动时间+LastPrompt(line-clamp-3)+三按钮(Resume/Open folder/Delete)；无 git 分支字段

## 四、可提交单元切分（P3b，已纳入 critic G1/G2/S2）

| 单元 | 范围 | 依赖 | 验收 |
|---|---|---|---|
| **P3b-1 resume 透传链** | 后端 2 处 + 前端 6 处链路 + cliTool 兜底 + bindings | P3a | `go build/vet`+`typecheck`；**端到端**：从真实树节点点 Resume → 后端日志/进程 args 出现 `claude --resume <id>` / `codex resume <id>`（critic S2：不用"手造"绕过前端链） |
| **P3b-2 扫盘 service** | `sessionscan.go`(claude+codex)；launchhistory 扩方法+RecordSpec 扩字段；LastPrompt cryptox；app.go+bindings | P3b-1 | `go build/vet`；手测：本机有 ~/.claude 会话时 `ListProjectSessions` 非空、description 截断正确；LastPrompt 落库密文 |
| **P3b-3a status store**（critic G2 前置） | 新建订阅 `terminal:status` 的状态 store/Context + TerminalView/Context 接订阅 | P3b-2 | `typecheck`；手测：起会话后 status 由 starting→running，store 可读 |
| **P3b-3b SessionsView UI** | 活跃会话派生(点亮 WorkspaceItem) + 最近启动分组折叠 + SessionSubList + ResumeDetailPopover + lang + 路由 | P3b-3a | `typecheck`；手测：分组折叠、点 Resume 真恢复、Popover 复制 id、活跃会话点亮节点 |

---

# 需你拍板的点

| # | 决策 | 建议 | 影响 |
|---|---|---|---|
| 1 | projects.csv / workspace.json 文件产物 | **不生成**（YAGNI+SQLite-only） | P3a 模型 |
| 2 | reveal-in-explorer "在资源管理器打开"菜单 | **新建专用 `OpenInExplorer` service 走 procx**（不经 subprocess 白名单）或砍掉归后续 | P3a 菜单#8（critic G3，"灰置"非合法态） |
| 3 | 排序方式 | **本期不做 / 上移下移菜单项**（@dnd-kit 是新依赖，blueprint Q-依赖） | P3a 树 |
| 4 | WorkspaceMigration / Snapshot | **全不做**（依赖 WSL/SSH/迁移子系统） | 全程 |
| 5 | resumeId 加密 | **不加密**（CLI resume 必需明文）；仅 LastPrompt cryptox | P3b |
| 6 | WorkspaceProject ↔ 全局 Project | **自带 path 软关联**（避开 Project.Add 阻断） | P3a service |
| 7 | 文本输入对话框 | **新建应用内 MUI Dialog**（合规） | P3a-5a |
| 8 | SessionsView 落点 | **复用 history 路由重做** | P3b |
| 9 | 轮询 backfill | **本期否**（先按需扫盘） | P3b |

**仍需核实**：数据存储页 lang 路径已确认为 `SettingsPage/database/lang/`；reveal 能力见拍板 2；status store 见 P3b-3a；resumeId 前端流经路径已由 critic 核实为 6 处链路（见 P3b-1）。
