# P7 计划 — 纯 Web 界面 + 桌面/Web 会话级互通（仿 mindfs，手机局域网优先）

> 来源：分维度读码（mindfs 多端 server-first 范本 + XK 现状，带 `file:line`）+ 对抗式可行性审查。本文件 = 动手前的事实基线 + 已定决策 + 分期 + 拍板点。**先过目 + 拍板，再实现。**
> 相关：MCP/Orchestrator 地基见 [P6_PLAN.md](./P6_PLAN.md)；总路线见 [REBUILD_BLUEPRINT.md](./REBUILD_BLUEPRINT.md)；铁律见 [CLAUDE.md](./CLAUDE.md)；逐会话进度见 [progress.md](./progress.md)。

---

## 0. 一句话

给 XK-Pane（Wails v3 桌面端）仿 mindfs 加一个**纯 Web 界面**，核心是**会话级互通**：**手机/平板经局域网**打开同一份 UI，能 `launch_task` 新建窗口，**同一 sessionID 的窗口在桌面和 Web 实时同步显示**。后端继续走 Wails 桥服务桌面端（零回归），新增一个 **0.0.0.0 上的 HTTP+WebSocket 网关**叠加服务远程浏览器。桌面是总览驾驶舱（看得见一切），Web 是远程观察窗（从会话列表挑着看）。

---

## 1. 现状事实基线（带 file:line，已读码核对）

### 1.1 架构本就「接近镜像就绪」
- **会话权威状态在 Go 内存（SSOT）**：`internal/services/terminal/terminal.go:88-95` `Service.sessions map[string]*session`；PTY 活在进程内。前端是无状态渲染器。
- **事件本就是按 sessionID 的广播**：`internal/events/events.go:11-13,21` 注册 `terminal:output/status/exit` + `orchestrator:launch`，全走 `app.Event.Emit`（应用级总线 → 所有窗口）。前端 `frontend/src/services/terminal/TerminalService.ts:62-74` 按 `payload.sessionId` 过滤分发。→ **任意多端 attach 同 sid 即镜像**，缺的只是把广播经 WS 送浏览器、动作经 HTTP 暴露。
- **`launch_task` 已在 HTTP server 上**：`internal/services/orchestrator/orchestrator_tools.go:91-153` `toolLaunchTask` = `Terminal.CreateSession()` 拿 sid → `emitEvent("orchestrator:launch", {taskId,sessionId,projectPath,cliTool,title...})`。前端 `frontend/src/terminal/TerminalContext.tsx:214-228` 监听 → **开新 Tab 附着该 sid（不再 CreateSession）**。

### 1.2 既有 HTTP server（webapi 的种子）
- `internal/services/orchestrator/orchestrator.go`：`net/http` server，**bind `127.0.0.1:0`（line 140，随机端口）**；32-hex Bearer token（line 247，**每进程随机重生成**）；`authMiddleware`（line 215）Bearer / `?token=` 后备 + **注入 `Accept: text/event-stream`（line 227，MCP/Codex 专属补丁）**；REST handler `POST /api/terminal/session-started`、`/api/notifications/trigger`（line 155-156，`rest.go` 内联 verifyToken + rateLimit）；token+url 写 `<exe目录>/mcp-orchestrator.json`（line 257-296，浏览器读不到）。
- ⚠️ 这个 server 是给**本机 CLI（claude/codex）反向调 MCP** 用的，bind 127.0.0.1、middleware 改 Accept 头、token 每进程变。**不可直接拿来服务手机**（见 §5 铁律：webapi 另起独立 0.0.0.0 listener）。

### 1.3 终端数据流（最难远程化的部分）
- `terminal.go:470-508` readLoop 读 PTY 原始字节 → `utf8SafeProcess` 跨 read UTF-8 边界修复 → `outBuf.push`（line 486）+ 投 `batchCh`(cap 32, line 49)。
- `terminal.go:510-545` batchLoop：累积 ≥16KB 或 ≥16ms → `emitEvent("terminal:output",{sessionId,data})`（line 520，data 是 ANSI 原始字符串、非 base64）。
- ⚠️ `outBuf` **入缓冲即剥 ANSI**（`terminal.go:148` 注释「纯文本输出（ANSI 已剥离）」，供 `get_session_output` 给 agent 读）→ **重建不了 xterm 画面**，attach 重放需另开保留原始 ANSI 的缓冲（§6.2）。
- `terminal.go:287-296` WriteInput = **裸 `pty.Write` 无锁**；`terminal.go:299-305` Resize；`terminal.go:346-361` SessionSnapshots（list_sessions 用，可直接喂 Web 会话列表）。
- `setEmit`（terminal.go:173）已是可注入字段，`AttachApp`（line 167）注入 `app.Event.Emit` —— **组合双路改造点很干净**。

### 1.4 前端会话生命周期（暗礁所在）
- `frontend/src/terminal/TerminalContext.tsx`：面板树 `rootPane`（Panel/SplitPane 布局）= **per-client UI 状态**，useReducer 管。
- 两条建会话路径：① 手动 `launchProject`（line 105）→ reconcile（line 230-269）`TerminalService.create` 回填 sid，**不广播**；② `orchestrator:launch`（line 214）→ addTab 带 sid，reconcile 跳过创建。
- ⚠️ **暗礁**：reconcile 的孤儿清理（line 263-267）把「本端面板树消失的会话」→ `TerminalService.kill`。即**每个客户端把自己面板树当会话生死权威**。多端镜像下，一端关 Tab 会杀掉另一端正用的会话。

### 1.5 会话布局持久化（已有）
- `internal/services/session/session.go`：`SaveSessions/LoadSessions`（重启恢复布局快照，整体替换）；`LoadSessionsDecrypted` 是**非绑定包级函数**（SshConfig 明文出口，绝不跨 IPC，安全红线，line 69-74）。

### 1.6 前端与 bindings 的耦合（决定移植量）
- `TerminalService.ts:13` `import { Events } from '@wailsio/runtime'`、`import { Service as Binding } from '@bindings/...'`；`OrchestratorService.ts:8` 同。事件名在 `TerminalService.ts:55-59` 内联常量、`OrchestratorService.ts:10` 走 `@/shared/events`。
- bindings 是编译期数字 MethodID（`$Call.ByID(...)`），**浏览器无 Wails runtime**，故 transport 抽象必须插在 `services/` 层（View/VM 不动）。

---

## 2. 架构抉择：Route B（已定）

| | 路线 A：Wails server build / custom transport | **路线 B：独立 chi/HTTP+WS 网关（mindfs 范式）✅** |
|---|---|---|
| 桌面回归 | server build **阉割 systray/dialog/通知/子窗口** | 桌面**继续走 Wails 桥，零回归**；网关是叠加 |
| 鉴权粒度 | 同一 MessageProcessor 按 MethodID 分发，**无 per-method 拦截点**，远程一连即可调全部 service | **路由层精确白名单**（fail-closed，§9） |
| 远程/移动 | 受 Wails 帧格式约束，难做二进制流/PWA | 标准 HTTP+WS，自定义帧、PWA、TLS 全可控 |
| 地基 | — | orchestrator 已证 `net/http` 模式可行（§1.2） |

**结论：Route B。** 在 `internal/webapi/` 新建独立网关，复用既有 service 实例；前端 `services/` 层做 transport 抽象（探针 `'_wails' in window`：桌面 Wails 桥 / 浏览器 WS+HTTP）。不变式 = mindfs 已验证的「**Go 后端是会话状态唯一事实源，所有前端都是无状态对等订阅者**」。

---

## 3. 已定决策（2026-06-01）

| # | 决策 | 内容 |
|---|---|---|
| D1 | **生命周期：桌面为主，非对称** | 桌面关 Tab = 真 kill（维持现状，**不分会话谁发起一律 kill**）；Web「关闭窗口」= 仅 detach（会话后台续跑）；Web「终止会话」= kill。 |
| D2 | **镜像范围：Web 按需** | Web 不自动镜像所有会话，显示**活跃会话列表**按需 attach；Web 自己发起的 launch_task 才在 Web 立即开窗（桌面侧仍按 `orchestrator:launch` 自动开窗）。 |
| D3 | **监听：0.0.0.0 + 手机局域网优先** | webapi 直接 bind `0.0.0.0:<固定端口>`，目标是手机/平板经 `http(s)://<PC-LAN-IP>:<port>` 访问。鉴权与（可选）TLS 不再后置（§5）。 |
| D4 | **kill 永远显式即时** | kill 仅来自：桌面关 Tab / Web 终止会话 / 进程自退。**后端不做引用计数**（D1「桌面为主」绕开「最后一个 client 离开才 kill」难题），`Kill()` 仍是幂等方法，桌面代码零改。 |

**D1 的一个被接受的边界**：Web 发起的 launch_task 桌面会自动开镜像窗，桌面操作员关掉它 = kill 掉 Web 正在用的会话。已确认接受（严格「桌面为主」）。Web 用「关闭窗口」(detach) 保活、「终止会话」彻底结束。

---

## 4. 目标架构

```
        ┌──────────────── Go 后端（唯一事实源 SSOT）────────────────┐
        │  terminal.Service.sessions[sid]（活 PTY）                  │
        │  ┌─ orchestrator MCP server  bind 127.0.0.1（本机 CLI 用，不动）│
        │  └─ ★新增 internal/webapi 网关  bind 0.0.0.0（手机/浏览器用）  │
        │       /  静态托管 frontend/dist（同源）                     │
        │       /api/*  REST（launch/input/resize/kill/sessions/...） │
        │       /ws     WS fan-out（per-conn 订阅 sid→conns）         │
        └───────┬───────────────────────────────────┬───────────────┘
          Wails 桥（进程内同步）                   WS + HTTP（cookie 鉴权）
                │                                     │
        ┌───────▼────────┐                   ┌────────▼─────────┐
        │ 桌面 webview    │                   │ 手机/平板浏览器   │
        │ owner 角色      │  ←── 同 sid 镜像 ──→ │ viewer 角色       │
        │ 关 Tab=kill     │                   │ 关窗=detach/终止=kill│
        └────────────────┘                   └──────────────────┘
```

数据流：
```
Web 新建窗口  → POST /api/launch → Service.LaunchTask(params)
              → CreateSession(sid) → 广播 orchestrator:launch
              → Web(发起方)开窗 attach sid + 桌面(总览)自动开窗 attach sid
              → terminal:output{sid} 经 Wails桥→桌面、经 /ws→Web，两端 xterm 同步

桌面手动建   → CreateSession(sid) + 广播 session:created（轻量）
              → 桌面本地开窗；Web 仅在「会话列表」+1，用户按需 attach（D2）

关闭语义     桌面关 Tab → 现有 reconcile orphan-kill → Kill(sid) → terminal:exit 广播
              → Web 若在 attach 该 sid，收到 exit，显示「会话已被终止」
            Web「关闭窗口」→ WS unsubscribe(sid) + 本地移 Tab，不 kill（viewer 角色）
            Web「终止会话」→ POST /api/sessions/{sid}/kill → Kill(sid)
```

---

## 5. 安全与网关（0.0.0.0 必须前置鉴权）

裸 0.0.0.0 无鉴权 = 同网段任何人可控你的终端，**绝不允许**。基线：

- **D5 独立 listener**：webapi 另起 `0.0.0.0:<固定端口>` 的 `net/http`/chi server，**不复用** orchestrator 的 127.0.0.1 server（后者 middleware 改 Accept 头、token 每进程变、只该给本机 CLI）。固定端口便于手机书签。
- **D6 稳定 token + 配对**：webapi 用**持久化** token（`cryptox.EncryptString` 落 SQLite，跨重启稳定，区别于 orchestrator 的每进程随机 token）。配对流程：桌面 UI 显示 **LAN URL + 二维码**（token 在 URL query）→ 手机扫码打开 → 后端校验 token → **下发 httpOnly Session Cookie** → 后续 HTTP/WS 用 cookie 鉴权（**token 不进 JS / 不进 localStorage**，遵守 CLAUDE.md 持久化铁律；WS upgrade 同源自动带 cookie）。
- **D7 同源免 CORS**：dist 与 /api 由同一 listener 同源提供 → 无需 CORS；WS `CheckOrigin` 按配置 LAN Origin 白名单收紧（不照抄 mindfs 恒 true）。
- **D8 TLS（拍板点 P3）**：自签证书（SAN 覆盖 LAN IP + 主机名），手机首次接受一次 → wss 加密 + secure context。需要它才能：流量加密、PWA 安装（service worker 要 secure context）。**取舍**：
  - 方案(A) M1 就上自签 TLS —— 加密 + PWA 早到，代价：配对要先过浏览器证书警告；PWA 在自签下能否安装因浏览器而异（best-effort，必要时用 mkcert 本地 CA 或真域名）。
  - 方案(B) M1–M6 跑明文 HTTP 验证功能（手机镜像最快可见，LAN 明文 token 风险自担），M7 再上 TLS。
  - **推荐 B**（可信家庭 LAN、最快迭代）；若 LAN 不可信，提前到 A。
- **D9 远程能力 fail-closed**：webapi 只挂**显式白名单**端点（§9），写文件系统/子进程/明文密钥红线一律不注册远程路由。

---

## 6. 关键机制

### 6.1 传输双路解耦（桌面零回归的命门）
`app.go` 装配处把 emit 改组合双路，两路**严格隔离**：
```go
emit := func(name string, payload any) {
    app.Event.Emit(name, payload)          // ① webview：进程内同步、永不阻塞（零回归）
    webGateway.RouteEventAsync(name, payload) // ② 远程：投递各连接「各自的有界队列」，绝不阻塞调用方
}
termSvc.SetEmitter(emit) // setEmit 已存在，导出为 SetEmitter（一行）
orchSvc 同理
```
- webview 路径保持 `app.Event.Emit` 现状，前后不插任何可能阻塞的逻辑。
- 远程背压**只局限「单个远程端 ↔ 该端有界队列」**：可丢流（`session:created`/状态）满即丢最旧；不可丢流（`terminal:output`）满即**断开该端**（重连走 attach 重放），**绝不反压回 batchLoop/PTY**。即一个卡顿的手机绝不拖慢桌面终端。

### 6.2 attach 重放（原始 ANSI 有界缓冲）
现有 `outBuf` 是剥 ANSI 的纯文本（§1.3），重建不了画面。新增**保留原始字节**的环形缓冲：
- **明确字节上限**（远小于 live 文本缓冲，量级几百 KB ~ 1–2MB，M4 压测后定）。
- **按重绘锚点裁剪**（最近一次 `ESC[2J`/`ESC[H`/alt-screen 切换起点），不按行切，免得重放残缺 CSI。
- 重放→live 不错序（抄 mindfs `ReplayPending`：重放期把该 client 排除出 live 扇出，重放完原子切入）。**配本地时序断言测试**（mindfs 代码不在本仓库，属未验证移植）。

### 6.3 resize 权属 = owner（手机不缩桌面）
一个 PTY 只有一组 cols/rows。**PTY 尺寸跟随 owner（桌面）**；viewer（手机）只 fit/scroll/letterbox，**绝不改 PTY 尺寸**——否则手机小屏会把桌面终端缩成一团。无 owner 时取当前主 viewer 尺寸（D1 下基本不出现：桌面关闭即 kill）。

### 6.4 输入仲裁 + TUI 应答去重
- `WriteInput` 加**会话级写锁**串行化（现状裸 write 无锁，terminal.go:292）。
- 多端都能输入（交错可接受），但 TUI 查询应答（DSR/CPR/DA，claude/codex 握手用）**只由 owner 回灌 PTY**（或后端代答一次），旁观端丢弃，否则 N 份重复应答污染握手。

### 6.5 launch 公共化
把 `orchestrator_tools.go:toolLaunchTask` 核心（校验项目注册 + 解析 workspace/cliTool + CreateSpec + CreateSession + emit orchestrator:launch）抽成 `Service.LaunchTask(ctx, params)`，MCP 工具与 `POST /api/launch` 共用，避免双份校验。

---

## 7. 铁律

1. **桌面零回归**：`main.go`、`internal/app/window_*.go`、systray、orchestrator 127.0.0.1 server、桌面 kill 路径**一律不动**；webapi 是纯叠加。（childwindow 系统已于 2026-06-11 减法审计整删。）
2. **webapi 独立 listener**：0.0.0.0 网关与 orchestrator MCP（127.0.0.1）物理隔离，互不污染中间件/token。
3. **fail-closed 白名单**：远程能力默认不暴露，新增 service 方法默认无远程路由，需显式 opt-in 过 review（§9）。
4. **明文密钥红线**：`provider.ResolveLaunchEnv`、`session.LoadSessionsDecrypted` 永不挂任何 HTTP 路由，保持非绑定包级函数现状。
5. **transport 抽象只在 services/ 层**：View/ViewModel 不感知桌面 vs 浏览器；事件名统一 `@/shared/events`（删 `TerminalService.ts:55-59` 内联常量，与 `events.go` 对齐，加 CI 字符串清单断言防漂移）。
6. **token 不进 JS/localStorage**：走 httpOnly cookie（D6）。
7. **持久化走 SQLite**：webapi token、配对状态等跨会话状态落库（cryptox 加密敏感串），不写 localStorage。

---

## 8. 分里程碑（手机局域网优先，桌面全程零回归）

| 里程碑 | 动到的文件 | 做什么 | 验证标准 |
|---|---|---|---|
| **M1 网关地基** | 新建 `internal/webapi/{server,router,middleware,auth}.go`；`internal/app/app.go`（装配 webapi.New + 后台起/优雅关）；`internal/storage/models_*.go`（webapi token/配对表）；桌面 UI 加「Web 访问」卡（显示 LAN URL + 二维码） | bind 0.0.0.0；稳定 token（cryptox 落库）；配对 → httpOnly cookie；同源托管 `frontend/dist`（复用 main.go embed.FS）；健康检查 | **手机连同一 WiFi，打开 `http://<PC-IP>:<port>`，扫码配对后看到 UI 外壳**；未配对 401 |
| **M2 transport 抽象** | `frontend/src/services/transport/{types,wails,http,index}.ts`；`TerminalService.ts`/`OrchestratorService.ts` 改走 transport；`@/shared/events.ts` 单一源 + CI 断言 | `'_wails' in window` 探针；浏览器侧 WS+HTTP（cookie 鉴权、重连退避） | 桌面 `pnpm typecheck` + 手测功能不变；事件名前后端清单一致 |
| **M3 launch + 只读镜像** | `orchestrator_tools.go`（抽 `LaunchTask`）；`webapi`（`POST /api/launch`、`GET /api/projects|workspaces|sessions`、`/ws` 广播 4 事件）；`app.go`（组合双路 emit）；`terminal.go`（`SetEmitter` 导出） | 先只读（Web 能看不能输入） | **手机点「新建」→ 桌面 & 手机同步出现同 sid 窗口，输出实时同步滚动** |
| **M4 attach 重放** | `terminal.go`（原始 ANSI 环形缓冲 + 重放接口）；`webapi/handlers_terminal.go` | attach 时重放→原子切 live（§6.2） | 手机中途 attach 已运行会话，画面正确重建；replay 驻留内存 ≤ 上限（实测）；20 并发会话总内存可控 |
| **M5 viewer 生命周期 + 会话列表** | `frontend/src/terminal/TerminalContext.tsx`（加 `lifecycleRole: 'owner'｜'viewer'`，viewer `closeTab`=detach 不跑 orphan-kill）；Web 端「终止会话」按钮 + 会话列表面板；`terminal.go`（`CreateSession` 末尾广播 `session:created`，`waitLoop` 广播 `session:closed`）；`events.go`（注册新事件） | D1/D2 落地 | 手机从列表按需 attach；**关窗不杀、终止才杀**；桌面关 Tab 杀会话后手机显示「已终止」 |
| **M6 双向输入 + 仲裁 + 移动端键盘** | `terminal.go`（WriteInput 会话级写锁）；`webapi`（resize 权属=owner）；前端 TUI 应答去重；**移动端：软键盘接入 xterm + 虚拟键栏（Ctrl/Esc/Tab/方向/Ctrl+C）** | §6.3/§6.4 | 手机能打字提交 prompt；resize 不缩桌面；TUI agent 握手不重复；人为拖慢手机端不卡桌面终端（背压验证） |
| **M7 TLS + PWA + 移动 UX 打磨** | `internal/utils/tlsutil`（自签 SAN=LAN IP）；`webapi`（升 https/wss）；`frontend`（PWA manifest + service worker、响应式布局、底部操作栏拇指可及） | 拍板点 P3 方案 A/B | 手机经 https 加密访问；（best-effort）PWA 可安装；竖屏布局可用 |

**先做 M1–M3 即见效果**：手机点「新建」→ 桌面和手机同步出现同 sid 窗口、输出同步滚动 —— 正是目标画面。M5 是 D1/D2 的正确性落地，不可跳过。

---

## 9. 远程能力白名单（fail-closed）+ 不可远程化清单

**默认拒绝，只显式注册以下远程路由**（初版，实际以审计 checklist 为准）：
- 会话编排（核心）：`POST /api/launch`（= LaunchTask）、`GET /api/sessions`（SessionSnapshots）、`POST /api/sessions/{sid}/{input,resize,kill}`、`/ws` 订阅。
- 只读列表：`GET /api/projects`、`/api/workspaces`、`/api/launchhistory`、`/api/preferences`、`/api/appsettings`（脱敏）。

**永不挂远程路由**（红线 / 高危，桌面/本机专属）：
- `subprocess.Run`（万能子进程 = RCE）；`gitsvc` 全部写方法（Clone/Worktree*/Push/Pull/Fetch/Stash*/Stage/Unstage/Discard/Commit）；`localhistorysvc` 写方法（Snapshot/Save*/Restore*/*Label）；`storagesvc.SetCustomPath`；`provider.ResolveLaunchEnv`（明文密钥；窄版 EnvVars 与 session 服务已删）；notifications/systray/窗口控制；MC PE 调试桥。
- 远程「添加项目」无原生选目录 → 改「手输路径 + 后端校验存在/是目录」。

---

## 10. 拍板点清单

- **P1（已定 D1）**：生命周期桌面为主、非对称（桌面关=kill / Web 关窗=detach / Web 终止=kill）。
- **P2（已定 D2）**：Web 按需 attach（会话列表），不自动镜像全部。
- **P3（待拍）TLS 时机**：方案 A（M1 就上自签，PWA/加密早到，配对过证书警告）vs 方案 B（M1–M6 明文 HTTP 验证、M7 上 TLS，推荐，可信 LAN）。
- **P4（待拍）SQLite 并发**：远程多端拉列表会与 webview 争用 `db.go` 的 `MaxOpenConns=1` 单连接。是否在 M3 前评估读写连接分离 / 给 webapi DB 访问加并发闸（复用 orchestrator rateLimiter）？验证项：N 个浏览器并发拉列表时 webview 操作 P95 延迟不回归。
- **P5（待拍）二维码库**：桌面端生成配对二维码用哪种方式（Go 侧生成 PNG 经事件传前端 / 前端纯 JS 生成）。

---

## 变更记录 (Changelog)

- 2026-06-01：初稿。确立 Route B + 0.0.0.0 手机局域网优先；固化生命周期决策（D1 桌面为主非对称 / D2 Web 按需 attach / D3 0.0.0.0 / D4 kill 显式即时）；给出网关安全模型（独立 listener + 稳定 token + 二维码配对 + httpOnly cookie + 同源免 CORS + TLS 拍板点）、关键机制（传输双路解耦 / attach 重放 / resize 权属 owner / 输入仲裁 + TUI 去重 / launch 公共化）、M1–M7 分期与验证、fail-closed 白名单。待拍：P3 TLS 时机、P4 SQLite 并发、P5 二维码库。
