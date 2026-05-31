# P6d 计划 — cli-hook 全栈（cc-pane cc-panes-cli-hook → XK 忠实重建）

> 来源：动手前精读 cc-pane 全栈真身（`../cc-pane`）—— cli-hook crate 4 子命令、cc-cli-adapters hook 注册、ProjectCliHooksService、orchestrator_service REST handler、notification_service、im_bridge_service、NSIS 打包 —— 全部带 `file:line`。本文件 = 架构映射 + 分单元 + **拍板点（先拍板再实现）**。
> 相关：MCP/Orchestrator 总图见 [P6_PLAN.md](./P6_PLAN.md)；逐会话进度见 [progress.md](./progress.md)；铁律见 [CLAUDE.md](./CLAUDE.md)。

---

## 0. 一句话

cc-pane 的 cli-hook = 一个**独立 exe**，被在 pane 里跑的 Claude/Codex 在生命周期事件（SessionStart / PostToolUse[Write] / PreToolUse[AskUserQuestion] / Stop）作**一次性子进程** spawn，经继承的 env（port/token/pty-session-id）**回连宿主 orchestrator HTTP**，把「会话起来了 / 在问用户 / 答完了」上报回去，或往项目本地归档 plan / 触发桌面通知。XK 已把 **env 注入侧**（`XKPANE_API_PORT/TOKEN/BASE_URL/PTY_SESSION_ID/CLI_TOOL`）在 P6b 全部接好，但 **「hook 二进制本体 / 注册进 settings.local.json / orchestrator REST 回调 handler / launch_cwd 回填 / 桌面通知 service / IM ingest 落库」全部为零**。P6d = 补上这套回路。

---

## 1. cc-pane 架构（带 file:line）

### 1.1 cli-hook 二进制（`cc-panes-cli-hook` crate）
独立 exe，`main.rs:30-38` clap 分发 4 子命令：

| 子命令 | 触发 hook | 行为 | 关键 file:line |
|---|---|---|---|
| `session-start` | SessionStart(startup[\|resume]) | ①读 stdin JSON（session_id/cwd）②写 legacy `.ccpanes/session-state.json` ③POST `/api/terminal/session-started`（launch_id+pty_session_id+resume_session_id+cli_tool+runtime_kind+cwd）④**stdout 注入项目上下文**（context header / launch-profile skills / current-state〔time+git branch+status〕 / workspace CLAUDE.md / memory / workflow.md / journal / ready） | `session_start.rs:92-274` |
| `plan-archive` | PostToolUse(Write) | 若 Write 的 file_path 落在 `~/.claude/plans/` → 复制到 `<proj>/.ccpanes/plans/<sid前8>_<时戳>_<原名>.md`（同 session+同名去重覆盖）。**纯本地，不回连** | `plan_archive.rs:26-143` |
| `im-forward` | PreToolUse(AskUserQuestion) / Stop | PreToolUse[AskUserQuestion]→`IngestRequest::Ask{questions}`；Stop→`extract_final_text`（fast-path last_assistant_message → transcript_path jsonl 反向扫最近 assistant text 块 → message/assistant_message 别名）→`Final{text}`。POST `/api/im-bridge/ingest` Bearer 750ms | `im_forward.rs:93-372` |
| `notify` | 显式 CLI（非生命周期） | 读 `mcp-orchestrator.json`（找 `mcpServers.ccpanes` 的 url+Authorization，token 走 header 或 `?token=` 后备）→ POST `/api/notifications/trigger`（kind/title/body/source/scope/dedupe_key/only_when_unfocused/metadata） | `notify.rs:52-148` |

**共性铁律**：750ms 全局超时、失败**静默 exit 0**（stderr 留 `[ccpanes-hook]` 日志）、**绝不阻塞 CLI**。`CLAUDE_NON_INTERACTIVE=1` 时 session-start 直接 return。env 解析顺序：`CC_PANES_API_BASE_URL` → 回退 `http://127.0.0.1:${CC_PANES_API_PORT}`；token 取 `CC_PANES_API_TOKEN`。

### 1.2 hook 注册机制
- **每项目偏好**：`ProjectCliHooksService`（`project_cli_hooks_service.rs`）写 `<proj>/.ccpanes/cli-hooks.json`（`{tools:{claude:{session-inject:true,...}}}`），缺省视为 `true`（全启用）。
- **adapter 落地**：`claude.rs` `HOOK_DEFS:27-60`（4 条：session-inject/plan-archive/im-forward-ask/im-forward-stop，各带 event/matcher/timeout/label）；`sync_project_hooks:607-662` 把启用项写进 `<proj>/.claude/settings.local.json` 的 `hooks` 块（`{event:[{matcher,hooks:[{type:"command",command:"\"<二进制绝对路径>\" <subcommand>",timeout,async:true}]}]}`）——`merge_ccpanes_hook_entry`（同 event 先剔除旧 ccpanes 条目再 push）/ `unregister_ccpanes_hook_entries`（关闭时移除）/ `is_ccpanes_hook_entry`（命令含 hook 二进制名或 "ccpanes" 即判定为托管条目）。`get_settings_path:220` = `<proj>/.claude/settings.local.json`。
- **二进制发现**：`get_hook_binary_path:64-167`：env `CC_PANES_CLI_HOOK_BINARY`/`CC_PANES_HOOK_BINARY` 覆盖 → `current_exe` 同目录 → 同目录 `binaries/` → macOS `Resources/binaries/` → workspace `target/{release,debug}/`。
- **触发时机**：`terminal_service.rs:1233-1236`（启动前 `sync_project_cli_hooks(project_path, cli_tool)`，failure 仅 warn 不阻断）。
- **codex 分支**：`codex.rs` 也 `supports_project_hooks=true` 但只支持 session-start（plan-archive `supported:false`、无 im-forward）；落点是 `<proj>/.codex/hooks.json` + `<proj>/.codex/config.toml` 写 `[features] codex_hooks=true`（实验开关）——与 claude 的 settings.local.json **完全不同的文件 / 格式**。

### 1.3 orchestrator REST handler（`orchestrator_service.rs`）
`build_router:452-513` 同一 axum Router 挂 REST + MCP；本期相关 3 个 REST：
- `POST /api/terminal/session-started`（`handle_session_started:3683-3748`）：verify_token + check_rate_limit → `launch_history_service.update_session_started(launch_id, pty_session_id, resume_session_id, cli_tool, runtime_kind, wsl_distro, cwd)` → emit `history-updated`。404 = launch_id 找不到。
- `POST /api/im-bridge/ingest`（`handle_im_bridge_ingest:3794-3903`）：verify_token + rate_limit → 409 若 pty 不在活 → 503 若 IM sidecar 没跑 → `im_bridge_service.ingest_outgoing(pty_session_id, payload)`。Ask/Final 标签枚举（`ImBridgeIngestRequest`）。
- `POST /api/notifications/trigger`（`handle_trigger_notification:3642-3681`）：verify_token + rate_limit → `notification_service.trigger(app, settings, req)`。
- `verify_token:3036`（Authorization: Bearer 比对）、`check_rate_limit:3049`（10s ≤ 20）每 handler 内联。

### 1.4 通知 service（`notification_service.rs`，382 行）
- `trigger:91-138`：normalize → 查 `settings.notification.enabled`（否则 skip）→ `only_when_unfocused` 且窗口聚焦则 skip → dedupe_key 去重（10s）→ `send_notification`（tauri `app.notification().builder().title().body().show()`）→ emit `notification-sent`。
- 另有 `notify_session_exited` / `notify_waiting_input`（终端事件触发，非 hook，本期不做）。
- 依赖 `settings.notification`：`{enabled, only_when_unfocused, on_exit, on_waiting_input}`。

### 1.5 IM 桥（`im_bridge_service.rs`，3059 行 + `cc-notify` crate 5 通道）
**整个子系统**：cc-im-bridge sidecar 子进程（newline-JSON 双向 stdio）+ jsonl watcher（监听 `~/.claude/projects/...` / `~/.codex/sessions/...` 写盘）+ `im_bridge_config`/`im_task_subscription` 两表 + PTY↔jsonl 绑定 + 出站派发到 `cc-notify`（dingtalk/lark/slack/telegram/webhook）。`im-forward` hook 只是把 Ask/Final **送进** ingest 端点，真正的「发到你手机的 Telegram」由这整套 sidecar 完成。**P6d 完整移植不现实**（见 §5 拍板点 D6d-3）。

### 1.6 构建打包
`scripts/build-hook.cjs`（`cargo build -p cc-panes-cli-hook --release`）+ `copy-hook.cjs`（复制到 `src-tauri/binaries/`，tauri sidecar 随包）。NSIS 经 `wails.files`/tauri bundler 把 binaries 打进安装目录，与主 exe 同目录 → `current_exe` 同目录发现命中。

---

## 2. XK 现状（env 注入已就绪，回路全缺）

**已就绪**（带 file:line）：
- `terminal/terminal.go buildEnv:608-647`：对所有本地 CLI 会话注入 `XKPANE_PTY_SESSION_ID`（恒）/ `XKPANE_CLI_TOOL` / `XKPANE_API_PORT/TOKEN/BASE_URL`（orchestrator 就绪时）。**hook 读的 env 名 1:1 对齐（前缀 XKPANE_）**。注释已标「接收端=orchestrator HTTP 回调，尚未移植」。
- `orchestrator/orchestrator.go`：`net/http` server + `authMiddleware`（Bearer + `?token=` + Accept 注入 + `?caller_tool=`→header 桥接）+ `genToken`（32-hex）+ `writeConfig`（写 `mcp-orchestrator.json` 到 exe 目录，server 名 **panes**，含 url+`?token=`+headers Bearer）。**当前 mux 只挂 `/api/health` + `/mcp`**，无 REST 业务路由、无 rate limiter。
- `storage/models_ccpane.go`：`LaunchHistory` 表已建，**含 `LaunchCwd *string`/`PtySessionID *string`/`ResumeSessionID *string`/`ClaudeSessionID *string` 字段**（line 69-79）——回填的目标列**全在**。但**无 `launch_id` 列**，PK 是自增 `ID uint`。注释 line 15「im_bridge_* 表延后」。
- `app.go:150-161`：orchestrator `New(Deps{term/proj/ws})` + `AttachApp` + `Start`（fail-open）+ `SetOrchestrator` + `defer Stop`。15 service 注册 + `SetSessionStartHook`/`SetConfigHook`/`SetProviderResolver` 回填模式可照搬。
- Wails v3 自带 `pkg/services/notifications.NotificationService`（`SendNotification(NotificationOptions{Title,Subtitle,Body,...})`，Windows 实现在）→ **notify 通道的现成落点**。

**全缺**（grep 确认 0）：
- `cmd/`（目录不存在）—— 无任何独立二进制。
- launch_history **后端无写入方**：`launchhistory.go` 只有 `Record`（自增 ID）+ `List`，**且 Record 由前端 `TerminalContext.tsx:128` 调**，后端 `CreateSession` 不写。**无 `update_session_started` / 无关联键 / 无 cwd 回填**。
- orchestrator REST 业务 handler / rate limiter / verify_token helper（MCP 鉴权在 middleware 里，REST 需各自校验）。
- hook 注册机制（adapter 无 `SyncProjectHooks`，无 HOOK_DEFS，无 settings.local.json 读写）。
- 通知 service / `notification` 设置 / im_bridge_* 表 / ingest 落库。
- Taskfile 编 hook 二进制 + NSIS 打包条目。

---

## 3. Tauri/Rust → Wails/Go 映射

| cc-pane（Rust） | XK（Go） |
|---|---|
| `cc-panes-cli-hook` crate（clap 4 子命令）| `cmd/xkpane-cli-hook/main.go`（同 module 复用 internal 工具；stdlib `flag` 或 `os.Args[1]` switch 分发，不引 cobra） |
| `ureq::Agent timeout 750ms` POST | **不能用 `httpx`**（铁律的 httpx 是单例长连客户端，hook 是一次性进程要硬 750ms 超时）→ hook 内用裸 `&http.Client{Timeout:750*time.Millisecond}`（hook 不在 internal 主进程，铁律「禁 DefaultClient」针对主进程；hook 是独立 exe，显式短超时 client 合规且必要）。**在计划里显式标注此偏离** |
| `serde_json` stdin/反向 jsonl 扫描 | `encoding/json` + 按行 split 反向遍历 |
| `dirs::home_dir` | `os.UserHomeDir` |
| `#[cfg(windows)] creation_flags(0x08000000)` 不弹窗 | hook 内 `git` 子进程走 `procx`（自带 Windows no-window + JobObject）或裸 `exec.Command`+`SysProcAttr{CreationFlags:0x08000000}`（hook 是独立 exe，可不引 procx；倾向 procx 复用） |
| axum `route(post(handler))` + 内联 verify_token/rate_limit | orchestrator `mux.HandleFunc("POST /api/...", h)`（Go 1.22 method 路由）；新增 `Service` 字段 `rate *rateLimiter` + 抽 `verifyToken(r, token)` / `checkRate()` helper |
| `launch_history_service.update_session_started(launch_id, ...)` | `launchhistory.UpdateSessionStarted(...)` **按 pty_session_id 关联**（XK 无 launch_id，见 D6d-1） |
| tauri `app.notification().builder().show()` | Wails v3 `notifications.NotificationService.SendNotification(...)`（注册为 Wails service） |
| `settings.notification`（tauri store）| appsettings 单行表加 `notification_*` 字段（SQLite-only 铁律）或独立小表（见 D6d-4） |
| `im_bridge_service.ingest_outgoing`（→ sidecar → IM）| **数据面 only**：`im_bridge_events` 表落库 + emit 前端事件（IM 通道层延后，见 D6d-3） |
| `ProjectCliHooksService` + `claude.rs sync_project_hooks` | 新包 `internal/services/projecthooks`（HOOK_DEFS + settings.local.json 读写合并 + 二进制发现）；`terminal.CreateSession` 启动前调（镜像 terminal_service:1233） |
| `scripts/build-hook.cjs` + tauri sidecar | Taskfile `build:hook`（`go build -o bin/xkpane-cli-hook.exe ./cmd/xkpane-cli-hook`）+ dev 依赖 + NSIS `project.nsi` 加 `File` 条目 |

---

## 4. 分单元（建议，逐个自洽可编译 + 中文 conventional commit）

> 依赖序：P6d-1（二进制本体，可独测）→ P6d-2（注册）+ P6d-3（REST 回填）成对构成 session-start 闭环 → P6d-4（notify）→ P6d-5（im 数据面）→ P6d-6（打包）。P6d-4/5 取决于 §5 拍板。

### P6d-1 — cli-hook 二进制 + 4 子命令本体（L）
`cmd/xkpane-cli-hook/main.go` + 4 子命令文件：
- `session-start`：读 stdin JSON（session_id/cwd）；`XKPANE_PTY_SESSION_ID`/`XKPANE_CLI_TOOL` 等 env 探测；POST `/api/terminal/session-started`（750ms 静默）；**stdout 注入**（XK 子集：`<xkpane-context>` header + `<current-state>` time+git branch+status + `<ready>`；workspace/memory/journal 因 XK 无对应数据源而省，见 §6）。
- `plan-archive`：Write 落 `~/.claude/plans/` → 复制到 `<proj>/.xkpanes/plans/`（XK 项目本地目录是 `.xkpanes`，与本地历史同处；同 session+同名去重）。读 `CLAUDE_PROJECT_DIR`。
- ~~`im-forward`~~：**D6d-3 拍板砍掉**，本期不实现。
- `notify`：读 exe 目录的 `mcp-orchestrator.json`（server 名 **panes**）→ POST `/api/notifications/trigger`。
- 共性：裸 `http.Client{Timeout:750ms}`（显式标注偏离 httpx 铁律的理由）；失败静默 exit 0（notify 例外，沿用 cc-pane exit 1）；`[xkpane-hook]` stderr 前缀。
- **3 个子命令**：session-start / plan-archive / notify（im-forward 已砍）。
- **验证**：`go build ./cmd/xkpane-cli-hook`；单测 helper（plan-archive 去重命名、orchestrator.json 解析、stdin 解析）；手动 `echo '{...}' | xkpane-cli-hook session-start` 看 stdout。**此单元 POST 都会连不上（端点未建）→ 正是设计的静默失败，不阻塞**。

### P6d-2 — hook 注册机制（settings.local.json sync）+ 启动时自动注册（M）
- 新包 `internal/services/projecthooks`：
  - **claude/reclaude**：`HookDef`（session-start/plan-archive，im-forward 已砍）；`syncClaudeHooks(projectPath, hookBinary, desired)` 写 `<proj>/.claude/settings.local.json` 的 hooks 块（忠实移植 merge/unregister/is-xkpane-entry，托管标记用命令含 `xkpane-cli-hook`）。
  - **codex（D6d-5）**：`syncCodexHooks` 写 `<proj>/.codex/hooks.json`（仅 session-start，plan-archive 不支持）+ `<proj>/.codex/config.toml` 写 `[features] codex_hooks=true`（实验开关）。⚠️ Windows 触发未经 cc-pane 验证，P6d-2 手测须确认 codex 真能读 `.codex/hooks.json`。
  - `resolveHookBinary()`（env `XKPANE_CLI_HOOK_BINARY` 覆盖 → exe 同目录 → `binaries/` → dev fallback `bin/`）。
- 偏好：缺省全启用，**本期不做 toggle UI / 偏好文件**（D6d-2 拍板，全自动）。
- 接线：`terminal.CreateSession` 启动 claude/reclaude/codex 前调 `projecthooks.Sync*`（failure 仅 warn）。
- **验证**：启 claude → `<proj>/.claude/settings.local.json` 出现 2 条 hooks；启 codex → `<proj>/.codex/hooks.json` + config.toml 出现 session-start。单测：sync 写入/重复剔除/关闭移除（claude+codex 各一组）。

### P6d-3 — orchestrator REST `/api/terminal/session-started`（打通回路 + log/emit）（M）
> **二次拍板**：实现中发现 ①XK 无 LaunchCwd 消费方（纯 schema 字段）②前端 `launchProject` 记 launch_history 时会话尚未创建（sessionId 由后续 reconcile effect 异步产生），**前端记账拿不到 pty_session_id**。按 pty_session_id 回填 launch_history 要么搬记账到后端（中等重构、为没人读的数据），故拍板：**只打通回路 + log/emit，不动 launch_history**。
- orchestrator `Service` 加 `rateLimiter`（10s≤20，`ratelimit.go`）+ `verifyToken`（常量时间）；mux 加 `POST /api/terminal/session-started`（内联鉴权+限流，`rest.go`）→ logx 记录 + emit `orchestrator:session-started`（`events.go` 新事件 + payload）。
- **不动 launch_history**（无消费方）、**不改前端**（前端订阅事件留后续）。launch_cwd 落库等有 resume-by-cwd 消费方时再做。
- **验证**：go build/vet/test（rateLimiter 窗口、handler 401/200-emit/400）全绿。手测：pane 内启 claude → hook 触发 → app.log 见 `session-started 上报 cwd=...`。

### P6d-4 — notify 通道（Wails 桌面通知 service + `/api/notifications/trigger`）（M，已落地）
- 新 `internal/services/notification`：包装 Wails `notifications.NotificationService`（经 `sender` 窄接口便于测试）+ dedupe（10s）+ only_when_unfocused（注入 focusFn 回调，解耦 Wails 窗口 API）。规则对齐 cc-pane：enabled→only_when_unfocused→dedupe→send。**非 Wails service**（内部，orchestrator 依赖）；Wails `notifications.New()` 单例注册进 Services 以跑 Windows ServiceStartup（注册 toast GUID/激活器）。
- 设置（D6d-4 A）：`AppSettings` 加 `NotificationEnabled`(默认 true)/`NotificationOnlyWhenUnfocused`(默认 false)；**不进前端 Snapshot**（无设置 UI），notification 服务经 holder 直读表（不 capture *DB）。
- orchestrator `POST /api/notifications/trigger`（鉴权+限流；Notifier nil→503）→ notification.Trigger。app.go 注入 `mainWindow.IsFocused()` 作 focusFn。
- 依赖：go-toast/v2 v2.0.3（wails 通知 windows 实现间接引入）。
- **验证**：go build/vet/test 全绿（Trigger enabled/disabled/focus-skip/dedupe/per-call-override + rateLimiter）。手测：`xkpane-cli-hook notify --title X --body Y` → 桌面 toast（⚠️ dev 未安装态 Windows toast 可能不弹，见 §6 风险）。

### ~~P6d-5 — im-forward 数据面~~（**D6d-3 拍板取消**）
本期完全不做 im-forward：不实现 im-forward 子命令、不注册该 hook、不建 `/api/im-bridge/ingest` 端点、不加 `im_bridge_events` 表。整个 IM 桥（含外部通道）留作未来独立阶段。

### P6d-6 — 构建打包（Taskfile build:hook + NSIS）（S）
- 根/Windows Taskfile：`build:hook` = `go build -o {{.BIN_DIR}}/xkpane-cli-hook.exe ./cmd/xkpane-cli-hook`；`build:native` 与 dev 前置依赖它（dev 下 hook 必须先在 `bin/` 才能被 resolveHookBinary 命中）。
- `build/windows/nsis/project.nsi`：Section `SetOutPath $INSTDIR` 后加 `File "...\bin\xkpane-cli-hook.exe"`（与主 exe 同目录 → prod 下 exe 同目录发现命中）。
- **验证**：`task build` → `bin/xkpane-cli-hook.exe` 存在；NSIS 安装态目检留打包时（dev 不验证安装器）。

---

## 5. 拍板点（已锁定 2026-05-31）

> **拍板结果**：D6d-1=**(A) pty_session_id 关联** / D6d-2=**(A) 启动自动注册全部，无 UI** / D6d-3=**(B) 完全不做 im-forward**（砍子命令 + ingest 端点 + im_bridge 表 → P6d-5 取消） / D6d-4=**(A) appsettings 加通知字段** / D6d-5=**移植 codex hooks，落 `<proj>/.codex/`**（claude/reclaude 落 `.claude/`，Windows 触发由 XK 自验，cc-pane 未验证） / D6d-6=**`<proj>/.xkpanes/plans/`**。下方为拍板时的完整选项留档。

### 备选选项留档

### 【D6d-1，最重要】session-started 回填的关联键
cc-pane 用 `launch_id`（后端启动时写 launch_history 行 + 注入 `CC_PANES_LAUNCH_ID`，hook 回填用 launch_id 匹配）。**XK 现状不同**：launch_history 由**前端**写、无 launch_id 列、后端启动不写。两条路：
- **(A) 推荐：按 `pty_session_id` 关联**。hook 恒有 `XKPANE_PTY_SESSION_ID`；前端 Record 补传 ptySessionId（model 已有列）；`UpdateSessionStarted` 按 pty_session_id 找最近一条回填。**零新增 launch_id 链路、零 env 新增、最贴合 XK 现有「前端记账」架构**。
- **(B) 忠实 cc-pane：引入 launch_id**。后端 CreateSession 生成 launch_id、注入 `XKPANE_LAUNCH_ID`、写一条 launch_history（与前端 Record 二选一或合并）、加 `launch_id` 列。**改动大、与前端记账冲突需重构**。
> 倾向 **(A)**——回填的真实价值是拿到运行时 cwd + agent resume id，关联键只需稳定且 hook 可见，pty_session_id 正好。

### 【D6d-2】hook 偏好 / toggle UI
- **(A) 推荐：启动时自动注册全部支持的 hook**（缺省全启用，偏好文件 `<proj>/.claude/xkpane-hooks.json` 留存但本期不做前端开关 UI）。
- **(B) 完整移植**：前端项目 hooks 设置面板（逐 hook 开关）+ ProjectCliHooksService 全套 binding。**L3 UI，工作量大**。
> 倾向 **(A)**，UI 留后续。

### 【D6d-3，最重要】im-forward 的「IM 出口」做到什么程度
cc-pane 的 IM 出口是 3059 行 sidecar + 5 通道 + 订阅表的独立子系统。三档：
- **(A) 推荐：数据面 only**（P6d-5）——ingest 端点 + `im_bridge_events` 落库 + emit 前端事件。hook 真触发、数据真落库、前端可见「agent 在问/答完」，但**不发到外部 IM**。真正的 Telegram/Slack 通道层独立成未来 **P6g**。
- **(B) 完全不做**——本期不注册 im-forward hook、不建端点/表，只做 session-start + plan-archive + notify。
- **(C) 最小单通道**——数据面 + 一个 webhook/桌面通知出口（把 Ask/Final 转成桌面通知，让你知道离屏 pane 需要你）。**与 notify 通道语义重叠**。
> 倾向 **(A)**：保留完整 hook 回路 + 落库地基，外部 IM 通道（重）延后。若你近期不需要「手机远程应答」，也可选 (B) 砍掉 im-forward。

### 【D6d-4】notify 桌面通知 + 设置
notify 通道用 **Wails v3 自带 `notifications.NotificationService`**（已确认 Windows 实现在）。设置存哪：
- **(A) 推荐**：appsettings 单行表加 `notification_enabled`(默认 true) + `notification_only_when_unfocused`(默认 false)。SQLite-only 铁律。
- **(B) 极简**：硬编码恒发（不读设置），设置留后续。
> 倾向 **(A)**（一处小表字段，顺手做对）。⚠️ **风险**：Windows toast 通知通常要求应用有 AppUserModelID / 已安装 Start Menu 快捷方式，`wails3 dev`（未安装态）下 toast 可能不弹——见 §6 风险，手测以「安装后」或「能否调通 API 不报错」为准。

### 【D6d-5】codex hooks 是否本期做
codex hook 是**完全不同的机制**（`<proj>/.codex/hooks.json` + `config.toml` 写 `[features] codex_hooks=true` 实验开关，仅 session-start）。
- **(A) 推荐**：本期 **claude/reclaude only**，codex hooks 延后（codex 用户仍可经 MCP 工具 + env 上报，只是无生命周期 hook）。
- **(B)** 一并移植 codex hooks。
> 倾向 **(A)**（codex hook 格式异构 + 实验开关，单独一单元更干净）。

### 【D6d-6】plan-archive 归档落点
cc-pane 归档到 `<proj>/.ccpanes/plans/`。XK 项目本地目录是 `.xkpanes`（本地历史在用）。建议归档到 **`<proj>/.xkpanes/plans/`**（与本地历史同处，一致）。确认或改 `<proj>/.claude/plans-archive/`。

---

## 6. 风险 / 待澄清

- **Windows 桌面通知（D6d-4 风险）**：Wails v3 alpha 的 toast 在未安装（dev）态可能不显示（缺 AUMID / 快捷方式）。需 P6d-4 实测；若 dev 不弹，手测降级为「API 200 + 日志确认」，prod 安装后再目检。
- **hook 二进制在 dev 的存在性**：`wails3 dev` 只重编主 app，不编 `cmd/xkpane-cli-hook`。P6d-2 的 `resolveHookBinary` 必须能在 `bin/` 找到；P6d-6 把 build:hook 设为 dev/build 前置，且**开发时改 hook 代码需手动重跑 build:hook**（类比「改 Go 后端必完全重启 dev」）。
- **httpx 铁律偏离**：hook 是独立 exe（非 internal 主进程），用裸 `http.Client{Timeout:750ms}`。计划在代码注释 + 本文件显式标注理由（一次性进程 + 硬超时 + 不复用连接池），不算违铁律。
- **session-start stdout 注入子集**：cc-pane 注入 workspace CLAUDE.md（`~/.cc-panes/workspaces/*/workspace.json`）/ memory（cc-memory-mcp）/ journal —— **XK 无对应数据源**（workspaces 在 SQLite、hook 进程不便读库；无 memory 子系统）。本期注入 = context header + current-state（time+git）+ ready。workspace CLAUDE.md 若要做需 hook 经 REST 反查（投机，延后）。
- **`update_session_started` 多行匹配**：按 pty_session_id 可能匹配多条历史（同一 pty id 罕见复用）；取 `launched_at DESC` 最近一条回填。
- **rate limiter 与 MCP 共享**：cc-pane 是单个全局 `last_request_times`，REST + 编排工具共用。XK orchestrator 加一个 Service 级 limiter，REST handler 内联调用即可（MCP 工具暂不限流，与现状一致）。

---

## 变更记录
- 2026-05-31：精读 cc-pane cli-hook 全栈（4 子命令 + 注册 + REST handler + notification/im_bridge service + NSIS）后产出本计划。待用户拍板 §5（D6d-1~6）后进入 P6d-1。
- 2026-05-31（拍板锁定）：D6d-1 pty_session_id 关联 / D6d-2 启动自动注册全部无 UI / **D6d-3 完全不做 im-forward（砍子命令+ingest 端点+im_bridge 表，P6d-5 取消）** / D6d-4 appsettings 加通知字段 / **D6d-5 移植 codex hooks，落 `<proj>/.codex/`**（澄清：codex 物理不读 `.claude/` 且 cc-pane Windows 不支持 codex hook，Windows 触发由 XK 自验）/ D6d-6 归档 `<proj>/.xkpanes/plans/`。**有效单元：P6d-1（3 子命令）→ P6d-2（claude/reclaude .claude + codex .codex 注册）→ P6d-3（session-started 回填）→ P6d-4（notify 通道）→ P6d-6（打包）**。进入 P6d-1。
- 2026-05-31（P6d 全部落地，5 提交，待手测）：P6d-1 cli-hook 二进制（`cmd/xkpane-cli-hook`，3 子命令）/ P6d-2 hook 注册（projecthooks，claude/reclaude→.claude、codex→.codex+config.toml）/ P6d-3 **二次拍板：REST session-started 只打通回路 + log/emit，不回填 launch_history**（发现 XK 无 LaunchCwd 消费方 + 前端记账拿不到 pty id）/ P6d-4 notify 通道（Wails 桌面通知 service + appsettings 两字段 + /api/notifications/trigger）/ P6d-6 打包（Taskfile build:hook + dev 前置 + NSIS File）。全单元 go build/vet/test 全绿。**待用户手测**（见 progress.md 手测清单）。
