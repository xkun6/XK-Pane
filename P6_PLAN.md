# P6 计划 — MCP / Orchestrator 子系统（cc-pane → XK 忠实重建）

> 来源：6-agent 并行研究（`wf_88dab7c4`，~744k token）通读 cc-pane MCP/Orchestrator 全栈 + XK 现状，全部带 `file:line`。本文件 = 动手前的事实基线 + 分期 + 拍板点。**先过目 + 拍板，再实现。**
> 相关：总路线见 [REBUILD_BLUEPRINT.md](./REBUILD_BLUEPRINT.md)；逐会话进度见 [progress.md](./progress.md)；铁律见 [CLAUDE.md](./CLAUDE.md)。

---

## 0. 一句话

cc-pane 的「Orchestrator」= **进程内的本地 HTTP+MCP 服务器**，让在 pane 里跑的 Claude/Codex 通过 MCP **反向调用宿主**（开新会话编排别的 Agent、读写其它 PTY、控制 Minecraft、查 Bedrock 知识库…）。XK 已把 **CLI 适配器侧的「接线盒」全部预留好**（Context 字段 + env 注入 + MCP-config stub），但 **「电源」——HTTP server / MCP server / token 管理 / cli-hook / REST 回调 / 前端 —— 全部为零**。P6 = 补上这套电源。

---

## 1. cc-pane 架构（带 file:line）

### 1.1 Orchestrator 服务（核心）
`src-tauri/src/services/orchestrator_service.rs`（~4266 行）：
- `OrchestratorService::start()` line 285-447：独立 std::thread + 2-worker tokio runtime，axum bind **`0.0.0.0:0`**（随机端口，0.0.0.0 是为 WSL 回连）line 362；拿到端口写 `<data_dir>/mcp-orchestrator.json`（`{url:"http://127.0.0.1:{port}/mcp?token={token}", headers.Authorization: Bearer}`）line 415-438。
- `generate_token()` line 3908-3913：OsRng 16 字节 → 32-hex Bearer token。
- `build_router()` line 452-513：同一个 Router 挂两支 ——
  - **REST `/api/*`** line 480-503：`launch-task / projects / task-status / sessions / session-status / write-to-session / submit-to-session / kill-session / terminal/session-started / im-bridge/ingest / notifications/trigger / health`，每个 handler 内联 `verify_token` + `check_rate_limit`（10s≤20）+ launch 类还查 `is_project_registered` 白名单 line 3005-3032。
  - **MCP `/mcp`** line 504：rmcp `StreamableHttpService` + `LocalSessionManager`；`mcp_auth_middleware` line 516-542 校验 Bearer（**header 或 `?token=` query 后备**，应对 Claude Code Issue #7290）；`inject_mcp_accept_headers` line 545-564 补 Codex 缺的 `Accept` 头避免 rmcp 406。CORS 仅放行 localhost/127.0.0.1/tauri line 458。
- **MCP 工具**（`McpToolHandler` + `#[tool_router]` line 1612-2810，rmcp `#[tool]` 宏自动生成 JSON Schema）：~64 个 = **33 编排**（launch_task / list_sessions / write_to_session / submit_to_session / get_session_output / kill_session / list_projects / list_workspaces / create_runtime_config / list_panes(oneshot 桥) …）+ **5 MC PE**（`#[cfg(windows)]` capture_window/click_window/execute_mc_code/reload_mc/get_mc_pe_logs line 2661-2809）+ **26 Bedrock**（`orchestrator_bedrock_tools.rs`，转发 `BedrockQueryService`）。
- 装配：`lib.rs` setup line 1239-1262 `orch_svc.start(...)`（传 ~17 个下游 service）→ `term_svc.set_orchestrator_info(port, token)`。前端命令 `get_orchestrator_port/token` + `respond_orchestrator_query` line 1670-1672。
- 注：`cc-panes-api` crate 是「把这层抽成框架无关」的**未完成占位**（lib.rs 注释「routes will be incrementally migrated」），真身仍在 src-tauri。`cc-panes-web` 是另一个独立 headless WS 终端二进制，**与 Orchestrator 无关**、无 MCP。

### 1.2 MCP 注入被启动的 CLI
`cc-cli-adapters/src/{claude,reclaude,codex,glm}.rs` + `cc-panes-core/.../shared_mcp_service.rs` + `terminal_service.rs`：
- **claude/reclaude**：`generate_mcp_config(ctx)` 写 `data_dir/mcp-{session}.json`（`{"mcpServers":{...}}`，含 ccpanes 端点 [type=http,url 带 `?token=`,headers Bearer 双通道] + `~/.claude.json` 用户 stdio server + shared HTTP），命令行加 `--mcp-config <path>`；>1h 旧文件清理。claude.rs line 100-210。
- **codex**：不写文件，用 `-c mcp_servers.ccpanes.url=...`、`.bearer_token_env_var=CC_PANES_API_TOKEN`、`.enabled=true`（codex.rs line 145-163）；隔离 `disable_unlisted_mcp_servers` 时解析 `~/.codex/config.toml` 的 `[mcp_servers]` 名字逐个 `enabled=false`。
- **env**（`terminal_service.rs` line 1156-1201，本地非 SSH 统一注入）：`CC_PANES_PTY_SESSION_ID / CC_PANES_LAUNCH_ID / CC_PANES_CLI_TOOL / CC_PANES_RUNTIME_KIND / CC_PANES_API_PORT / CC_PANES_API_TOKEN / CC_PANES_API_BASE_URL`。
- **shared MCP**（`shared_mcp_service.rs`）：把用户的 stdio MCP server 经 `npx -y mcp-proxy --port {p} -- cmd args` 桥成本地 HTTP（端口池 3100-3199），`get_running_servers_urls()` 返回 `{name→http://127.0.0.1:{p}/mcp}`。

### 1.3 cli-hook 二进制
独立 crate `cc-panes-cli-hook`（编出独立 exe，sidecar 随包）：
- 被 CLI 在生命周期事件 spawn 的**一次性子进程**（非常驻），4 个 clap 子命令：`session-start`（SessionStart：stdout 注入项目上下文 + POST `/api/terminal/session-started` 上报 `launch_cwd`/resumeId，session_start.rs line 227-274）、`plan-archive`（PostToolUse+Write：归档 ~/.claude/plans → .ccpanes/plans，纯本地）、`im-forward`（PreToolUse[AskUserQuestion]/Stop → POST `/api/im-bridge/ingest`）、`notify`（读 mcp-orchestrator.json → POST `/api/notifications/trigger`）。
- 注册：`ProjectCliHooksService` 写每项目偏好 `.ccpanes/cli-hooks.json`，CLI adapter（claude.rs `sync_project_hooks` line 607）把启用的 hook 条目写进项目 `.claude/settings.local.json` 的 `hooks` 块，command = `"<二进制绝对路径>" <subcommand>`。hook 读 env 的 port/token 回调，**750ms 超时、失败静默 exit 0、绝不阻塞 CLI**。
- 构建：`scripts/build-hook.cjs`（cargo build -p cc-panes-cli-hook）+ `copy-hook.cjs`（复制到 `src-tauri/binaries/`）。

### 1.4 MC PE 控制（用户的 Minecraft 落点）
`cc-panes-core/src/services/mc_pe/service.rs` + `mcdk_bridge/{bridge,capture,click,window,protocol}.rs`：
- `MCPEService.launch()` 复刻 MCStudio 12 步启动；`include_test_mod=true` 时把桥模组解压进 behavior_packs + 注入 env `MCDEV_DEBUG_IPC_PORT={bridge port}`。日志环形缓冲 cap=2000 供 `get_mc_pe_logs` 读。
- `McdkBridgeService` bind `127.0.0.1:0` 等游戏内 Python mod 反向 TCP 连入；`execute_mc_code/reload_mc` 把指令编成 **6 字节大端帧**（`[2B type][4B len][payload]`）fire-and-forget 发给桥模组（在游戏进程内 exec Python2.7）。
- `capture_window/click_window` **不走桥**：按 MC PE PID 定位顶层窗口，Win32 `PrintWindow/BitBlt`→480p JPEG→base64（唯一返回 MCP image content 的工具）/ `SendInput` 点击。

### 1.5 Bedrock 知识库
`bedrock_query/service.rs`：`include_dir!` 编译期内嵌 ~22MB ModSDK 数据 → `OnceLock<Indices>` 惰性建索引；26 个 `bedrock_*` 只读查询工具（search_api/get_api/validate_call/review_code/search_guide/validate_json_definitions…）。OTA 更新 `bedrock_doc_sync`（**非 MCP**，UI 命令，从 git.mcplugin.top + mc.163.com 抓→原子换目录）。`cc-memory-mcp` 是**独立 stdio JSON-RPC 二进制**（不挂 orchestrator）。

### 1.6 前端
- **无启停 UI**（orchestrator 后端开机自启、不可前端关）。`mcpService.getOrchestratorInfo()` 只读 port/token，`CcpanesMcpCard` 渲染只读卡（URL+token+复制）。
- `useOrchestratorListener` 订阅 6 个后端 emit 的「遥控副作用」事件（launch-task→addTab / open-folder / open-file / close-file / query-open-files / query-panes），query 类是请求-响应（后端带 requestId emit + oneshot，前端读 store 后 `respond_orchestrator_query` 回填）。
- `OrchestratorInput` + `OrchestratorView` + `useOrchestratorStore`/`Sync` = TaskBinding 编排面板（写 SQLite TaskBinding + 终端状态映射成 binding 状态）。**用了 localStorage（违反 XK 铁律，移植须改 PreferencesService）**。

---

## 2. XK 现状（接线盒已就绪，电源全缺）

**已就绪（形）**，带 file:line：
- `internal/services/cliadapters/adapter.go` line 81-90：`Context.OrchestratorPort/OrchestratorToken/DataDir/SharedMCPURLs/AllowedMCPServerIDs/DisableUnlistedMCPServers` 全部预留；line 182-195 `orchestratorEnv()` 恒注 `CC_PANES_PTY_SESSION_ID`，port/token 非空时注 `CC_PANES_API_PORT/TOKEN` —— **与 cc-pane hook 读的 env 名 1:1 对齐**。
- `claude.go` line 49-58 / `reclaude.go`：`generateMCPConfig()` port/token 为空恒返回 `""`（TODO phase5）；`codex.go` line 74-86 `pushMcpOverrides` 已写 `mcp_servers.ccpanes.url=...`+bearer，line 113-123 `configuredMCPServerNames()` 返回 nil（TODO phase5 解析 config.toml）。隔离/TOML 编码算法已 1:1 移植且有 `codex_test.go` 守门。
- `terminal.go` line 330-340：cctx 写死 `SkipMCP:true` + 一串 TODO(phase5)；line 501-503 已注 `CC_PANES_PTY_SESSION_ID`，注释明说「接收端=orchestrator HTTP 回调，尚未移植」。
- `internal/storage/models_ccpane.go`：`TaskBinding`/`task_bindings` 表已建（line 109-126），`launch_history`/`terminal_sessions`/`projects`/`providers` 在；**`mc_pe_*` / `im_bridge_*` 表未建**（line 15 注明 Phase 5 才加）。
- `app.go` line 89-143：15 service 注册 + `AttachApp/SetXxxHook` 预创建+回填模式（orchestrator 可照此接线）；line 152-158 后台 goroutine + `defer StopAll` = server 启停落点模板。`events.go` line 52-58 `RegisterEvent[T]`（orchestrator 事件在此加）。

**全缺（电源，grep 确认 0）**：本地 HTTP server / MCP server / token+端口管理 / `mcp-orchestrator.json` 落地 / cli-hook 二进制 / REST 回调 handler / `orchestrator-*` 事件 / 前端 orchestrator 任何代码。`httpx` 仅客户端（不冲突，server 另起 `net/http`）。

---

## 3. Tauri/Rust → Wails/Go 映射

| cc-pane（Rust/Tauri） | XK（Go/Wails） |
|---|---|
| axum `bind 0.0.0.0:0` + 独立 tokio runtime | 标准库 `net/http.Server`，`net.Listen("tcp","127.0.0.1:0")` 拿随机端口，后台 goroutine `Serve`（无需独立 runtime）。REST 用 Go 1.22+ 原生 `mux.HandleFunc("POST /api/launch-task", ...)` |
| rmcp `StreamableHttpService`（MCP over Streamable HTTP） | **官方 `github.com/modelcontextprotocol/go-sdk` v1.6.1** 的 `StreamableHTTPHandler`（备选 `mark3labs/mcp-go` v0.54.1），直接给 `http.Handler` 挂进同一 `ServeMux` |
| `#[tool]` 宏自动 schema | go-sdk `mcp.AddTool(server, &mcp.Tool{...}, handler)`（结构体 tag 生成 schema） |
| `generate_token()` OsRng→32hex | `crypto/rand` 16 字节 → hex；比对用 `crypto/subtle.ConstantTimeCompare` |
| Bearer header **+ `?token=` query 后备** + Accept 头注入 | 一层 `http.Handler` 中间件 1:1 复刻（这两个兼容补丁要保留，否则 Claude/Codex 连不上） |
| 写 `mcp-orchestrator.json`（`std::fs::write`） | `filex.WriteAtomic`（配置写铁律），落 `%AppData%\XK-Pane\` |
| `set_orchestrator_info(port,token)` 注入 TerminalService | 新增 `terminal.Service.SetOrchestrator(port,token)`（镜像已有 `SetProviderResolver` 回填模式） |
| `app_handle.emit(name,payload)` | `app.Event.Emit(name,payload)`，但**需先在 `events.go` `RegisterEvent[T]` 注册**（多一步） |
| oneshot + `respond_orchestrator_query`（query 类工具阻塞等前端） | `map[requestId]chan + RespondQuery binding + select+time.After(5s)` |
| cli-hook = 独立 Rust crate + sidecar | `cmd/xkpane-cli-hook/main.go`（同 module 复用 internal）独立 `go build`，NSIS 打包进安装目录 |
| shared MCP `npx mcp-proxy` 子进程 | `procx.StartCtx`（铁律：子进程走 procx，非 PTY 的） |
| MC PE Win32 `windows-rs` | `golang.org/x/sys/windows` + `image/jpeg`（缩放 `x/image/draw`，无 Lanczos3 近似） |
| Bedrock `include_dir!` 22MB | Go `embed.FS` + `sync.Once` 建索引 |

---

## 4. 分期单元（建议）

> 每单元自洽可编译、逐个提交、各带验证。**P6a 是地基也是最高风险，先做去风险 spike。**

### P6a — Orchestrator 服务地基 + MCP 传输打通（L，**spike 优先**）
- 新建 `internal/services/orchestrator`：`net/http.Server` bind `127.0.0.1:0`；`crypto/rand` 32-hex token；`filex.WriteAtomic` 写 `mcp-orchestrator.json`；`/api/health`；go-sdk `StreamableHTTPHandler` 挂 `/mcp` + Bearer 中间件（含 `?token=` 后备 + Accept 头注入）；注册 1-2 个 trivial tool（`health` / `list_projects` 读 project service）。`app.go` 装配（窗口就绪后台启动 + `defer Stop`）+ `terminal.SetOrchestrator`。
- **去风险 spike 先行**：先起 server，手动让真实 `claude` CLI 指过来（mcp config），确认能连上 + 列出工具。验证 SDK 选型 + 传输 + 鉴权兼容（Accept/token-query 坑）。**若官方 SDK 有坑，趁早回退 mark3labs**。
- 验证：go build/vet/test；手测：claude / MCP inspector 连 `http://127.0.0.1:{port}/mcp?token=...` 看到工具。bindings 生成。

### P6b — MCP 注入闭环（M，依赖 P6a）
- 实现 claude/reclaude `generateMCPConfig`（写 `mcp-{sid}.json`：ccpanes 端点双通道 token + 合并 `~/.claude.json` 用户 mcpServers；shared 暂空）；codex `pushMcpOverrides` 转真（stub 已近完）。`terminal.go` 填 ctx 的 port/token/DataDir，**orchestrator 就绪才翻 `SkipMCP=false`**（未就绪 fail-open 裸跑）；`buildEnv` 补 `CC_PANES_API_BASE_URL`(+`LAUNCH_ID`)。
- 验证：在 pane 里启 Claude → 自动连上 ccpanes MCP → 能从 pane 内调通 trivial 工具。

### P6c — 核心编排工具（L，依赖 P6b）— **自我编排闭环（headline 价值）**
- MCP 工具，全由 XK 既有 service 兜底：`launch_task`（→ `terminal.CreateSession`，项目白名单守门查 project/workspace service）/ `list_projects` / `list_workspaces` / `list_sessions` / `get_session_output` / `write_to_session` / `submit_to_session` / `kill_session`。+ 事件 `orchestrator:launch` → 前端开新面板（`events.go` 注册 + 前端 listener）。
- ⚠️ `get_session_output` 需要 per-session 输出缓冲——XK 终端目前直喂 xterm、未缓冲。可做**轻量 tail 环形缓冲**（也是被推迟的「会话回放 ReplayBuffer」的前身/子集）。
- 验证：pane A 的 Claude 调 `launch_task` → 弹出 pane B 跑另一个 CLI；write/get_session_output 通。

### P6d — cli-hook 二进制 + REST 回调（M–L，依赖 P6a，与 P6c 较独立）
- `cmd/xkpane-cli-hook`（Go）：`session-start` 读 env port/token → POST `/api/terminal/session-started`（launch_cwd/resumeId，750ms 超时）。adapter 写 `.claude/settings.local.json` hooks（二进制绝对路径）。REST handler 更新 `launch_history`。Taskfile 加 build + NSIS 打包进安装目录。（`im-forward`/`notify`/`plan-archive` 后续可选。）
- 验证：启 Claude → hook 触发 → 后端日志见 launch_cwd → launch_history 更新。

### P6e — MC PE 控制（XL，你的 Minecraft 落点，**建议独立子规划**）
- McdkBridge TCP server（`net.Listen` + 二进制帧 `encoding/binary`）+ 内嵌 Python 桥模组（`embed.FS`）+ `MCPEService.launch`（MCStudio 12 步）+ Win32 capture/click（`x/sys/windows`）+ MCP 工具 capture_window/click_window/execute_mc_code/reload_mc/get_mc_pe_logs + `mc_pe_*` 表。
- 难点 = Win32 GDI/SendInput 重写（PrintWindow GPU 合成窗口/DPI/句柄释放逐一复刻）。**XL，建议像 P3/P4 那样单独出一份 P6e_PLAN 再动手。**

### P6f — Bedrock 知识库 MCP（L–XL，独立）
- `embed.FS` 内嵌 baseline 数据 + `sync.Once` 索引 + 26 个 `bedrock_*` 查询工具（纯算法直译）。OTA 同步后置。

### 本轮全部推迟
shared-MCP（npx mcp-proxy）；cc-memory-mcp（独立 stdio + Memory DB）；IM 桥；桌面通知（cc-notify）；前端 TaskBinding 编排面板（L3 UI，需整套 task_binding Wails service）；codex `config.toml` 隔离解析（需 TOML 依赖，高级特性默认关）。

---

## 5. 拍板点（请逐条确认）

1. **【最重要】Go MCP SDK 选型**：建议官方 `modelcontextprotocol/go-sdk` **v1.6.1**（已 v1 稳定、Streamable HTTP、spec 对齐、最接近 cc-pane rmcp）；备选 `mark3labs/mcp-go` v0.54.1（更早成熟但仍 v0.x）。P6a spike 会实测连通性，若官方有坑就回退。**确认引哪个进 go.mod。**
2. **【最重要】本轮范围/顺序**：P6a 地基后走哪条？
   - (X) **自我编排核心**（P6b→P6c→P6d）：让 pane 里的 Agent 能开/读写别的 Agent + hook 回报。是 cc-pane 的 headline。
   - (Y) **直奔 MC PE**（P6a→P6e）：跳过编排工具/hook，最快到你的 Minecraft 落点（但 P6e XL、Win32 重，且要先 spike P6a 打通 MCP）。
   - 二者都依赖 P6a 地基；可先做 X 再做 Y，或反之。
3. **bind 地址**：建议 `127.0.0.1`（本轮不做 WSL/SSH，更安全）；要支持 WSL 回连才用 `0.0.0.0`（macOS 防火墙弹窗 + 暴露面大）。
4. **hook 二进制打包**：建议方案 A `cmd/xkpane-cli-hook` 独立二进制（干净、复用 internal）；方案 B argv[0] 分流塞主 exe（少一个二进制但耦合 + GUI 子系统无 stdout 麻烦）。
5. **`get_session_output` 缓冲**：本轮做轻量 tail 环形缓冲，还是把它并进「会话回放 ReplayBuffer」一起做（那块本是 P5 推迟项）。
6. **端口策略**：建议随机 `:0` + 写 `mcp-orchestrator.json` 发现（与已预留的 env/codex `-c url` 配套）。

---

## 6. 风险 / 待澄清

- go-sdk v1.x 仍在快速迭代，API 可能微调 → P6a spike 锁版本 + 验证 Claude/Codex 真能连。
- `orchestrator_service.rs` 仅精读到 ~line 1518/4266，REST handler 细节（write/submit-to-session 的提交时序、session-started 如何更新 launch history）实现前需补读 1519-4266。
- 编排工具依赖 ~15 个下游 service，XK 缺 todo/spec/skill/notification/launchprofile/sharedmcp；P6c 只做 XK 现有 service 能兜底的那批工具（terminal/project/workspace 类），其余工具留到对应 service 落地。
- MC PE 桥模组（游戏内 Python mod）反向握手/帧 type id（3-8）/env 名 `MCDEV_DEBUG_IPC_PORT` 是否可改 —— P6e 实现前需读 `mc_pe/resources/include_test_mod/`。
- 前端 orchestrator 遥控事件（open-file/open-folder/query-panes）依赖 panes/fileBrowser/editorTabs store —— XK 多 pane 体系部分有（PaneTree），文件浏览器/编辑器无 → L2/L3 前端大部分留到那些子系统落地。

---

## 变更记录
- 2026-05-31：6-agent 研究产出本计划；待用户拍板 §5 决策点后进入 P6a。
- 2026-05-31（拍板 + P6a 落地）：用户拍板「先只做 P6a 地基（spike 验证），两个 SDK 都试」。spike（`cmd/mcp-spike`，throwaway，`4fbc574`）实测两个 SDK 协议层均通（官方走 SSE、mark3labs 走纯 JSON）；**真实 Claude CLI 连通确认 ping 工具 → 选定官方 `go-sdk v1.6.1`**。P6a 落地（`abb74cf`）：`internal/services/orchestrator` 地基（127.0.0.1:0 HTTP + 32-hex token + `/api/health` + `/mcp` StreamableHTTPHandler + Bearer/`?token=`/Accept 中间件 + `health` 工具 + 写 `mcp-orchestrator.json`）+ `storage.UserDataDir()` + `terminal.SetOrchestrator` + app fail-open 接线；删 spike + `go mod tidy` 去 mark3labs。次要决策按默认：bind `127.0.0.1` / 端口随机 / hook 独立二进制（P6d）/ `get_session_output` 轻量缓冲（P6c）。**下一步**：用户手测 P6a（`wails3 dev` 后把 Claude 连到 `%AppData%\XK-Pane\mcp-orchestrator.json`、调 `health` 工具）通过 → 再定 §5 决策 2 的 X（自我编排 P6b→d）/ Y（直奔 MC PE P6e）方向。
- 2026-05-31（P6a 调整：配置落点 + skills 归属）：用户手测前提出「跟 cc-pane 不同」——`mcp-orchestrator.json` **不写 %AppData%**，改写**可执行文件所在目录**（"软件目录"；dev=`bin\`〔已 .gitignore 忽略〕、prod=安装目录）。落地：`orchestrator.configDir()`=`filepath.Dir(os.Executable())`，去掉对 `storage.UserDataDir()` 的依赖；「orchestrator started」日志加 `config=<完整路径>` 字段便于发现；回收 P6a 新增的导出 `storage.UserDataDir()`（改回仅未导出 `userDataDir`）。**另定**：后续被启动 CLI 的 skills / 每会话 MCP 注入落到**被启动项目的 `.claude/`**（`<proj>/.claude/skills`、`<proj>/.claude/mcp-*.json`），不再用 `~/.claude/skills`（P6b/P6d 落实，§5 决策点据此细化）。手测路径相应改为 `bin\mcp-orchestrator.json`（以后端日志 `config=` 字段为准）。
