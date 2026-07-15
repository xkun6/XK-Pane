# XK-Pane「bot 连接器」设计文档（供 codex 直接实施）

> 来源：分析 cc-connect 源码 + XK-Pane web 镜像/AskUserQuestion 机制后产出。
> 所有行号锚点均已在 XK-Pane 当前代码核实（app.go:239 mirrorEmit / answerKeys.ts / TerminalAskPendingPayload 无 toolUseId / orchestrator.PendingAsk / terminal.WriteInput / SessionSnapshot.ClaudeSessionID）。

## TL;DR

可以做，但**绝不能照搬 cc-connect 的 spawn-agent 模式**。XK-Pane 的 agent 已经跑在 PTY 真 TUI 里，cc-connect 那条「干净」的 `--input-format/--output-format stream-json + control_response` 双向通道在真 TUI 模式下根本没有协议入口，无法对接当前会话。正确做法是把 bot 连接器做成一个**进程内 observer 服务** `internal/services/botbridge`，复用 web 镜像同款三路数据源（① `mirrorEmit` 事件总线拿实时态、② `transcript.Parse/ParseCodex` 轮询拿结构化卡片、③ `orchestrator.PendingAsk` + `TerminalAskPending` 事件拿 AskUserQuestion 待答），借鉴 cc-connect 的 **Platform 接口 + Card 抽象 + askq:/perm: 回调协议**（只取「右半边」适配器层，丢掉 agent/claudecode 驱动半边）。AskUserQuestion 作答沿用 XK-Pane 已验证的「逐键写 PTY 驱动 Ink 选择器」路子——把前端 `answerKeys.ts` 的 `buildAnswerKeys` 移植成 Go，对每个键单独调 `terminal.Service.WriteInput` 且键间 sleep ≥100ms。

> **已拍板（2026-06-30）**：① **第一落地平台 = Feishu（飞书）**，走 WebSocket 长连接模式（`larkws`，只要 `app_id`/`app_secret` 出站、无公网、原生交互卡片+按钮，AskUserQuestion 体验最完整、国内网络稳）；Telegram 降级为后续可选适配器。② **会话绑定 = IM 内 `/sessions` 命令切换**（在飞书里列出活跃会话、点选切换，借鉴 cc-connect SessionKey），不走「唯一活跃自动绑」。这意味着 `cmd:/sessions`、`cmd:/switch <sid>` 回调从 MVP 起就要做，不再是「PoC 可不做」。

---

## 0. codex 评审结论（2026-06-30，已应用）

由将要实施的 codex（gpt-5.5 high）以「实施工程师」视角评审，结论 **⚠️ 有条件批准**：核心架构可行、承重锚点大多真实，但提出 6 个开工前必须修订项。**本文档已全部应用这 6 项**：

1. **askq 下标 0 基**：`askq:<qIdx>:<optIdx>` 一律 0 基；⚠️ 不要照搬 cc-connect 的 **1 基** askq（`core/engine.go:11305`）。→ 见 §2.3
2. **事件快路径补 toolUseId**：`TerminalAskPending` 不带 toolUseId，收到事件后必须立即 `PendingAsk(sid)` 补取，才能拼 `asksubmit`/`selections`。→ 见 §2.3 硬规则
3. **首落地适配器文件改 Feishu**：原 §5.1 误写 `platform/telegram/telegram.go`，已改 `platform/feishu/feishu.go`。
4. **配置/加密持久化提前**：`bot_bridge_config` 表 + 后端 `SetConfig`/读取从 M7 提前到 **M1**（M4 跑飞书 larkws 必须先有凭据来源）。→ 见 §5.4
5. **绑定作用域**：`boundSessionID` 按 Feishu chat/user 维度保存（非单一全局值），避免群/用户互相影响。→ 见 §5.2 / §6
6. **共用 per-session 写锁**：普通文本写入与 AskUserQuestion 逐键作答必须共用同一把写锁串行化。→ 见 §3.3 硬规则

codex 同时确认：不-spawn 判断正确（`cc-connect/agent/claudecode/session.go:219` 确为 spawn headless + stream-json）；前端 `answerKeys.ts:31` + `AskQuestionCard.tsx:52` 确为逐键+100ms；`effectivePending` 逻辑在 `frontend/src/components/SessionTimeline/useSessionTimeline.ts:168`；飞书 larkws 长连接 `cc-connect/platform/feishu/feishu.go:574`、按钮 value JSON map `card.go:134`。

**复审（2026-06-30，用时 1m01s）**：codex 确认上述 6 项已闭环，另发现 2 处「我修订时引入的新不一致」，**已一并修掉**：
- **[Major] 存储模型残留全局单绑定**：§5.1 原写 `BoundSessionID string` 与「按 chat/user 维度绑定」冲突 → 已改为 `BindingsJSON`（`map[feishuScopeKey]sessionID`）。
- **[Minor] 手测清单残留 TG 文案**：§5.5 第 1/3/4 条「TG」→「飞书」。

codex 明示「**修掉这两点后可以批准实施**」——两点均已修，本文档现已满足 codex 的批准实施门槛。

---

## 1. 结论先行

### 1.1 可行性判定

**能做。** 四项需求逐项有现成锚点：

| 需求 | 可行性 | 关键依据（已核实） |
|------|--------|--------------------|
| 实时对接当前终端 | ✅ | `app.go:239-242` 的 `mirrorEmit` 已是双路 fan-out（桌面 webview + Web 网关），加第三路给 bot 即可，绝不触碰 PTY 主链路 |
| 同步「当前卡片内容」 | ✅ | `transcript.Parse`(claude) / `transcript.ParseCodex`(codex) 把 JSONL 转录解析成 `transcript.Event` 数组，webapi 已用同一套（`rest.go:314-323`） |
| 把卡片推给 bot | ✅ | 复刻 webapi 的 REST 轮询语义（`after` 游标增量），进程内直接调 `transcript.Parse`，无需 HTTP 自环 |
| AskUserQuestion 作答回灌 | ✅（有约束） | pending 取自 `orchestrator.PendingAsk(sessionID)` + `TerminalAskPending` 事件；作答 = 移植 `buildAnswerKeys` 逐键写 `terminal.Service.WriteInput`，键间 ≥100ms |

唯一「不能做」的是：**用 cc-connect 的 control_response 结构化通道对接当前 TUI 会话**——那需要 claude 跑 headless stream-json，与「占着 PTY 的真 TUI」互斥。

### 1.2 核心架构取舍：observer 服务，不 spawn agent

**推荐**：新增 `internal/services/botbridge` 业务服务，作为 `mirrorEmit` 的进程内 observer，复用 web 镜像同款数据源；适配器层照搬 cc-connect 的 `core.Platform` / `core.Card` / 回调前缀协议。

**为什么不照搬 cc-connect 的 spawn-agent 模式**（这是全篇最关键的判断）：

- cc-connect 的 `agent/claudecode/session.go` 是**自己 spawn 一个 claude 进程**（`newClaudeSession` 拼 `--output-format stream-json --input-format stream-json --permission-prompt-tool stdio`），它的全部干净红利（`readLoop` 解析 stdout、`RespondPermission` 写 `control_response` 回灌 stdin、零按键模拟）都建立在「claude 跑 headless、stdin/stdout 是程序管道」之上。
- XK-Pane 的 agent **已经在 PTY 里跑真 TUI**，stdin 被 Ink 选择器占用、stdout 是 ANSI 绘制指令，**没有 stream-json 协议**可读、**没有 control_request 入口**可写。若 botbridge 另 spawn 一个 headless claude，那是**另一个会话**，不是用户眼前 PTY 里那个——违背「对接当前终端、实时镜像」的核心诉求。
- 因此：**取 cc-connect 的「右半边」**（platform 适配器 + Card IR + 按钮回调分派），**丢「左半边」**（agent/claudecode 整包、core/engine.go 6w+ 行阻塞式事件循环、cron/relay/provider 切换）。agent 的「事件」由 XK-Pane 现有数据源供给，与 web 镜像同源同语义。

数据流总览：

```
                                    ┌──────────────────────────────────────┐
                                    │  internal/services/botbridge (新)     │
 PTY 里的 claude/codex (真 TUI)      │                                       │
   │                                │  ① observer: RouteEvent(name,payload) │◄── app.go mirrorEmit 第三路
   │ emit terminal:output/...       │     过滤 TerminalAskPending/Exit       │
   ├──► terminal.batchLoop ─────────┤                                       │
   │                                │  ② 卡片轮询: transcript.Parse(file,    │
   │ JSONL 转录落盘                  │     after) ── 经 SessionSnapshots 定位 │
   ├──► ~/.claude/projects ─────────┤                                       │
   │                                │  ③ 待答: orchestrator.PendingAsk(sid)  │
   │ PreToolUse hook 上报待答        │                                       │
   └──► orchestrator.handleAskPending│  ──► Platform 适配器 (telegram/feishu) │──► IM 平台
                                    │                                       │
        作答按键 ◄── WriteInput ◄────┤  ④ 反向: buildAnswerKeys 逐键(≥100ms) │◄── 用户在 IM 点选/输入
                                    └──────────────────────────────────────┘
```

---

## 2. cc-connect 可借鉴/可直接搬运的部分

### 2.1 Platform 接口契约（直接搬，移植成 Go 同构）

源：`cc-connect/core/interfaces.go:10-16`（必选）+ `266-355`（可选能力）。搬运到 `internal/services/botbridge/platform.go`，**裁剪规则**：

| cc-connect 类型/方法 | 搬运动作 | 裁剪说明 |
|----------------------|----------|----------|
| `core.Platform`（Name/Start/Reply/Send/Stop） | ✅ 全搬 | 5 必选方法，bot 适配器最小契约 |
| `CardSender`（SendCard/ReplyCard）| ✅ 搬 | 飞书/钉钉真卡片 |
| `InlineButtonSender`（SendWithButtons）| ✅ 搬 | Telegram inline keyboard |
| `MessageUpdater`（UpdateMessage）| ✅ 搬 | 流式编辑卡片（增量更新转录） |
| `CommandRegistrar` / `TypingIndicator` | 🔶 可选 | PoC 可不实现 |
| `ReplyContextReconstructor` / `AsyncRecoverablePlatform` / `CardNavigable` / `CardRefresher` | ❌ 暂不搬 | cron/relay 主动推消息场景，XK 暂无 |
| `ImageSender`/`FileSender`/`AudioSender` | ❌ 暂不搬 | 卡片/作答不依赖 |

**鸭子类型降级**照搬：Engine 侧用 `p.(CardSender)` / `p.(InlineButtonSender)` 类型断言探测能力，平台只实现自己支持的子集。XK 的 `supportsCards(p) = 类型断言 CardSender`（对应 `engine.go:11561`）直接平移。

注册接线照搬 `core/registry.go:16-34` 的 `RegisterPlatform(name, factory)` + 各包 `init()`，但**用 build tag 接线那套（`cmd/.../plugin_platform_telegram.go` 的 `//go:build !no_telegram` + blank-import）可以省**——XK 是单体，直接在 botbridge 装配处显式注册即可。

### 2.2 Card 抽象（直接搬，对接 XK 的 transcript.Event）

源：`cc-connect/core/card.go:11-310`。搬到 `internal/services/botbridge/card.go`：

- `Card{Header, Elements}` + 元素类型 `CardMarkdown/CardDivider/CardActions/CardNote/CardListItem`
- `CardButton{Text, Type, Value, Extra}`、`CardBuilder` fluent API
- `RenderText()`（无 CardSender 平台的纯文本降级）
- `CollectButtons() → [][]ButtonOption`（喂 `InlineButtonSender`）

**对接点**：XK 已有 `transcript.Event`（`transcript.go:78-94`，字段 `Kind/Text/ToolName/ToolInput/ToolUseID/IsError/Questions/Detail`）就是「平台无关卡片 IR」。写一个 `eventToCard(ev transcript.Event) Card` 映射（详见 §3.2），下游各平台 `renderCardMap`（参照 `feishu/card.go:91-260`）翻成原生卡片。

### 2.3 按钮回调协议（直接搬前缀约定）

源：`cc-connect/core/bridge.go:976-1046`。回调 `Value` 前缀约定**直接复用**：

- `askq:<qIdx>:<optIdx>` — AskUserQuestion 选项点选。**`qIdx`/`optIdx` 一律 0 基下标**（`buildAnswerKeys` 按 0 基消费；⚠️ **不要照搬 cc-connect 的 askq——它是 1 基选项值**，见 `core/engine.go:11305`，照搬会全体选错位）。
- `asksubmit:<toolUseId>` — （XK 新增）多选/多题的「提交」按钮（见 §4 说明为何 XK 必须有这个，cc-connect 没有）
- `cmd:/sessions` — 列出活跃会话（`SessionSnapshots()`）成可点列表（**已拍板从 MVP 起做**，会话绑定靠它）
- `cmd:/switch <sid>` — 切换 bot 当前绑定会话（设 `boundSessionID`）
- `cmd:/<other>` — 其它用户命令（按需）

> **硬规则（codex 评审 #2）**：`TerminalAskPending` 事件**不带 toolUseId**（已核实 `events.go:89` 仅 `SessionID`+`Questions`）。botbridge 收到事件后**必须立即** `AskPend.PendingAsk(sid)` 补取 `toolUseId`，才能拼出 `asksubmit:<toolUseId>` 与 `selections[toolUseId]`；否则 M6 的作答提交回调无法稳定生成。**不要依赖事件自身给 toolUseId**。

**与 cc-connect 的关键差异**：cc-connect 按钮点一次就经 `control_response` resolve（`engine.go:3220-3361 handlePendingPermission`）。XK **不能**这样——XK 作答是批量翻译成按键序列驱动 Ink 选择器，必须「累积所有题的所有选项 → 一次性 `buildAnswerKeys`」。所以 XK 的按钮回调要在 botbridge 内**累积 selection 状态**（`map[toolUseId]map[qIdx][]optIdx`），由 `asksubmit:` 按钮触发最终作答。单题单选可走「点击即提交」快路径。

### 2.4 防御细节（移植时别当冗余删）

照搬 cc-connect 这几处踩坑补丁（`engine.go:3220-3361` / `bridge.go:976-987`）：

- **空消息拒收**（#1086 平台已读回执）：平台回调里空 content 直接丢，防误答。
- **stale 按钮丢弃**（#826）：无对应 pending 的 `askq:` 回调直接丢，防当普通输入塞给 agent。
- **multiSelect 不用单击 resolve**：XK 天然规避（因走累积+Submit），但 UI 上多选项要做成 toggle 态而非「点一下就走」。

### 2.5 cc-connect 的 AskUserQuestion 通道 vs XK-Pane 机制对照

| 维度 | cc-connect（headless） | XK-Pane（真 TUI，本方案） |
|------|------------------------|----------------------------|
| 待答来源 | `handleControlRequest` 收 `can_use_tool` + `parseUserQuestions` | `orchestrator.handleAskPending` 收 PreToolUse hook 上报（`rest.go:119-147`） |
| 待答存储 | engine `pendingPermission` 阻塞 `<-pending.Resolved` | `orchestrator.pendingAsk[sessionID]`（`rest.go:101-114`），**非阻塞** map |
| 推卡片 | `sendAskQuestionPrompt`（`engine.go:11268-11355`）三级降级 | botbridge 复刻三级降级（CardSender/InlineButtonSender/纯文本编号） |
| 作答回灌 | `RespondPermission` 写 `control_response` 到 stdin（零按键） | `buildAnswerKeys` 逐键写 PTY（`WriteInput`），键间≥100ms |
| 失效判定 | `pending.resolve()` 解阻塞 | toolUseId 现身累积转录即失效（前端 `effectivePending` 逻辑，botbridge 复刻） |

**取舍结论**：推卡片/三级降级/前缀解析这套**搬 cc-connect**；待答存取/作答回灌**用 XK 现有机制**（非阻塞 map + 逐键写 PTY），不要引入阻塞式事件循环（cc-connect `<-pending.Resolved` 会卡住整条会话流，与 XK 多会话并发模型不兼容）。

---

## 3. XK-Pane 集成锚点（最关键）

### 3.1 实时同步注入点：`app.go` 的 `mirrorEmit` 加第三路

**锚点**：`internal/app/app.go:239-242`

```go
// 现状（已核实）：
mirrorEmit := func(name string, payload any) {
    app.Event.Emit(name, payload)        // 桌面 webview
    webGateway.RouteEvent(name, payload) // Web WS
}
```

**改动**：加 botbridge 第三路（仿 `webGateway.RouteEvent` 的「白名单过滤 + 绝不反压」语义）：

```go
mirrorEmit := func(name string, payload any) {
    app.Event.Emit(name, payload)
    webGateway.RouteEvent(name, payload)
    botBridge.RouteEvent(name, payload)  // 新增：bot 进程内 observer
}
```

`botBridge.RouteEvent(name, payload)` 内部：

- 只关心 `events.TerminalAskPending`（秒级拿待答）、`events.TerminalExit`（会话结束清理卡片绑定）、可选 `events.TerminalStatus`。`events.TerminalOutput` 是原始 ANSI 字节（`events.go:53-57`），**对卡片无用、不订阅**（卡片走 §3.2 转录轮询）。
- **绝不反压**：照抄 `hub.go:80-98 broadcast` 的有界队列 + 慢端剔除语义（bot observer 自持一个 `chan`，满即丢/重连，绝不在 `RouteEvent` 里同步阻塞——否则反压回 `terminal.batchLoop`/PTY，一个卡顿的 IM 拖垮整机终端）。

**为什么不走 webGateway 的 HTTP/cookie（姿势 A）**：botbridge 是进程内服务，走 `GET /ws` + cookie 自环既绕远又要管 token，且 `mirroredEvents` 白名单（`rest.go:28-34`）会挡住将来 bot 想要的事件。进程内 observer 直接拿 `events.*Payload` 结构体，零 HTTP、零鉴权、绕过白名单——干净。

**装配**：仿 `webGateway.AttachServices`（`gateway.go:80-84` + `rest.go:81-91 ServiceDeps`），给 botbridge 也做 `AttachServices(BotDeps{...})`，注入下游窄接口：

```go
// internal/services/botbridge/botbridge.go
type BotDeps struct {
    Sessions SessionLister // terminal.Service：SessionSnapshots()
    Writer   SessionWriter // terminal.Service：WriteInput(ctx, sid, data)
    AskPend  AskPender     // orchestrator.Service：PendingAsk(sid)
    Scanner  SessionScanner// sessionscan：ListProjectSessions（codex/旧 claude 回退定位转录）
}
```

这几个接口 `terminal.Service` / `orchestrator.Service` / `sessionscan` **已满足**（web 网关已在用，见 `rest.go:44-91`），botbridge 直接复用同样的窄接口定义。

### 3.2 「当前卡片内容」数据来源：transcript.Event → Card

**锚点**：`internal/services/transcript/transcript.go:78-122`（`Event` / `ToolResultDetail` / `AskQuestion`）+ webapi 已验证的定位逻辑 `webapi/rest.go:247-323`。

botbridge 起一个**按会话的轮询 goroutine**（仿 webapi `handleSessionTimeline`，间隔 1-2s）：

1. `SessionSnapshots()` 把绑定的 `sessionId → cwd / cliTool / claudeSessionID`（`terminal.go:455-467 SessionSnapshot`）。
2. 定位转录文件：
   - claude 且 `claudeID != ""` → `transcript.ResolveClaudePath(cwd, claudeID)`（精确，`transcript.go:25-38`），文件未就绪返回空继续轮询，**不回退扫最新**（避免误取同目录并发会话）。
   - codex 恒、claude 旧会话 → `Scanner.ListProjectSessions(ctx, cwd, 0)` 按 mtime 取最新匹配 `Source`。
3. 增量解析：claude 走 `transcript.Parse(file, after)`、codex 走 `transcript.ParseCodex(file, after)`，均产出 `transcript.Result{Events, Total, Title}`。`after = 上次 Total` 做游标。
4. 把**新增 events** 经 `eventToCard` 映射后推给平台。

**`eventToCard(ev transcript.Event) Card` 映射表**（字段来自 `transcript.go:78-108`）：

| `ev.Kind` | 映射成 Card | 取字段 |
|-----------|-------------|--------|
| `KindUser` | `CardMarkdown`（用户气泡） | `ev.Text` |
| `KindAssistant` | `CardMarkdown`（助手气泡） | `ev.Text` |
| `KindThinking` | `CardNote`（折叠/弱化） | `ev.Text` |
| `KindToolUse` | `CardMarkdown`（工具调用摘要，仿 cc-connect `summarizeInput`） | `ev.ToolName` + `ev.ToolInput`（紧凑 JSON）；`ev.ToolName=="AskUserQuestion"` 时**跳过普通渲染**（它走 §4 交互卡，且转录里只在答完后才出现） |
| `KindToolResult` | `CardMarkdown`/差异块 | `ev.Detail` 非空：`Kind=="edit"` → diff（`FilePath/Diff/Added/Removed`）；`Kind=="bash"` → `Stdout`+`Stderr`；否则回退 `ev.Text`；`ev.IsError` 标红 |

**关键去重**：`KindToolResult` 经 `ev.ToolUseID` 关联回对应 `KindToolUse` 的 `ToolName`（`transcript.Event` 的 result 块不带工具名，靠 `ToolUseID` 配对）。

**避坑（webmirror 风险 #7）**：webapi 的转录定位逻辑（`rest.go:247-308`）没走 `transcriptsvc.Service` 而是直接调包级 `transcript.Parse`，已被 SUBTRACTION_AUDIT 标记为重复。**建议**抽一个共享窄函数（如 `internal/services/timelinesrc/Resolve(snap, after) (Result, pending)`），webapi 与 botbridge 共用，别两端各写一份定位逻辑。MVP 阶段允许 botbridge 内联复刻，但要留 TODO。

### 3.3 反向通道：bot 文本/作答如何写回当前 PTY

**锚点**：`internal/services/terminal/terminal.go:390-399`

```go
func (s *Service) WriteInput(_ context.Context, sessionID string, data string) error {
    sess, ok := s.get(sessionID)
    if !ok { return fmt.Errorf("terminal: unknown session %q", sessionID) }
    _, err := sess.pty.Write([]byte(data))
    ...
}
```

两类反向写：

- **普通文本消息**（用户在 IM 打字 → 当前 claude）：`WriteInput(ctx, sid, text + "\r")`。这是整句提交，**不受**逐键 focus 坑影响（那只针对 Ink 选择器），可一次写完。注意 `data` 是裸字节直写 PTY stdin（非进程 spawn），未知/已退出会话报错。
- **AskUserQuestion 作答**（点选 → Ink 选择器）：见 §4，**必须逐键单写 + 键间≥100ms**。

botbridge 内 `BotDeps.Writer` 即 `terminal.Service`（web 网关同款 `SessionWriter`，`rest.go:70-72`）。**不需要**新开 PTY 写端点，也**不要**给 bot 开万能 `RunCommand`（项目铁律）。

> **硬规则（codex 评审 #6）**：普通文本写入与 AskUserQuestion 逐键作答**必须共用同一把 per-session 写锁**串行化。否则一条普通 IM 文本可能插进逐键序列中间，污染 Ink 选择器（选错项/错位）。这把锁按 PTY `sessionID` 维度持有，覆盖「文本整句写」和「逐键作答循环」两条写路径。

---

## 4. AskUserQuestion 端到端方案（重中之重）

### 4.1 完整链路图

```
① claude 触发 AskUserQuestion（PreToolUse，工具执行前）
   └─► cmd/xkpane-cli-hook/askpending.go：读 tool_input{questions[]} + tool_use_id
       └─► POST /api/terminal/ask-pending 回连 orchestrator

② orchestrator.handleAskPending（rest.go:119-147，已存在）
   ├─► s.pendingAsk[ptySessionId] = PendingAsk{ToolUseID, Questions}   // 内存 map，供回灌
   └─► emitEvent(TerminalAskPending, {SessionID, Questions})           // fire-and-forget
       └─► mirrorEmit 第三路 ─► botBridge.RouteEvent(TerminalAskPending, payload)

③ botbridge 渲染待答卡（双源，仿前端 effectivePending）
   ├─ 实时态：RouteEvent 收到 TerminalAskPending → 立即建 pending 卡
   └─ 兜底：轮询 goroutine 读 orchestrator.PendingAsk(sid)（晚到/重连补）
       └─► sendAskQuestionPrompt 三级降级（CardSender / InlineButtonSender / 纯文本编号）

④ 用户在 IM 点选项（回调 askq:<qIdx>:<optIdx>）
   └─► botbridge dispatch：累积 selections[toolUseId][qIdx] = []optIdx
       ├─ 单题单选 → 直接进 ⑤（点击即提交快路径）
       └─ 多题/多选 → 等用户点 asksubmit:<toolUseId> 再进 ⑤

⑤ 作答回灌（移植 answerKeys.ts + 逐键写 PTY）
   ├─► keys := buildAnswerKeys(questions, selections)   // Go 移植
   └─► for k := range keys { Writer.WriteInput(ctx, sid, k); time.Sleep(100ms) }
       └─► 驱动 PTY 里阻塞着的 Ink 选择器逐键选定

⑥ pending 清除
   └─ 该 AskUserQuestion 的 tool_use 现身后续 transcript 轮询（toolUseId 出现）
      → 作答/取消都让阻塞轮次落盘 → 清 botbridge pending 卡 + 清 selections
```

### 4.2 复用 XK 已有的 pending 存取与作答入口

| 环节 | 复用什么（已存在） | botbridge 要补什么 |
|------|---------------------|---------------------|
| 待答存储 | `orchestrator.PendingAsk(sessionID) (PendingAsk, bool)`（`rest.go:109-114`），`PendingAsk{ToolUseID, Questions}` | 注入为 `BotDeps.AskPend`（web 网关同款 `AskPender` 接口 `rest.go:77-79`） |
| 待答事件 | `events.TerminalAskPending` + `events.TerminalAskPendingPayload{SessionID, Questions}`（`events.go:89-92`）——⚠️ 事件**不带 toolUseId** | botbridge 收事件后即时建卡，但 toolUseId 要从 `PendingAsk` 轮询补（事件路径缺 toolUseId） |
| 作答出口 | `terminal.Service.WriteInput`（`terminal.go:390`）/ 或 MCP `write_to_session` | — |
| 作答按键翻译 | ❌ **后端无等价方法**，只活在前端 `answerKeys.ts:buildAnswerKeys` | **必须新建 `internal/services/botbridge/answerkeys.go` 移植** |

**结论**：后端要补的关键 service 方法/文件就是 **`buildAnswerKeys` 的 Go 移植 + 逐键写循环**。其余存取全用现成。

### 4.3 `buildAnswerKeys` 的 Go 移植（逐字对照前端 `answerKeys.ts:31-64`）

按键字节序列**逐字照搬**前端 `KEY_SEQUENCES`（`useTerminalKeyBar.ts:6-16`，已核实）：`down=\x1b[B`、`right=\x1b[C`、`enter=\r`、`space=空格`。

```go
// internal/services/botbridge/answerkeys.go
package botbridge

const (
    keyDown  = "\x1b[B"
    keyRight = "\x1b[C"
    keyEnter = "\r"
    keySpace = " "
)

type AnswerableQuestion struct {
    MultiSelect bool
    OptionCount int // 只取选项数
}

// buildAnswerKeys 与前端 answerKeys.ts 同规则：返回【逐个按键】。
// 调用方必须逐键单独 WriteInput + 键间 sleep ≥100ms（陈旧 focus 坑）。
func buildAnswerKeys(qs []AnswerableQuestion, selections map[int][]int) []string {
    n := len(qs)
    hideSubmitTab := n == 1 && !qs[0].MultiSelect // 单题单选无 Submit 复核 tab，选中即提交
    var keys []string
    for i, q := range qs {
        sel := dedupSortFilter(selections[i], q.OptionCount) // 去重+升序+越界过滤
        if q.MultiSelect {
            pos := 0
            for _, t := range sel { // 从第0项逐 ↓ 到各目标、Space 切换勾选
                for d := 0; d < t-pos; d++ { keys = append(keys, keyDown) }
                keys = append(keys, keySpace)
                pos = t
            }
            keys = append(keys, keyRight) // 末尾 → 推进下一 tab（答案已由切换记录）
        } else {
            t := 0
            if len(sel) > 0 { t = sel[0] }
            for d := 0; d < t; d++ { keys = append(keys, keyDown) } // ↓ 到目标
            keys = append(keys, keyEnter)                            // Enter 选中并自动前进
        }
    }
    if !hideSubmitTab { keys = append(keys, keyEnter) } // 多题/多选末尾停 Submit tab → Enter 提交
    return keys
}
```

作答执行循环（注意 sleep）：

```go
func (s *Service) answerAsk(ctx context.Context, sid string, qs []AnswerableQuestion, sel map[int][]int) error {
    for _, k := range buildAnswerKeys(qs, sel) {
        if err := s.deps.Writer.WriteInput(ctx, sid, k); err != nil { return err }
        time.Sleep(100 * time.Millisecond) // KEY_DELAY_MS：让 Ink 在键间重渲染刷新 focus 闭包
    }
    return nil
}
```

> ⚠️ 移植铁律：`buildAnswerKeys` 的真实键位规则**以前端 `answerKeys.ts` 当前实现为唯一权威**。上面是结构示意，codex 必须对着实际文件逐行核对（单选/多选/多题/单题快路径 + Submit 复核 tab），并用单测断言两端逐字一致（见 §5.5）。

### 4.4 memory 已知坑（codex 务必规避）

1. **逐键单写 + 键间 ≥100ms（最脆的点）**：Ink 选择器的 `useInput` 回调闭包捕获当前渲染的 `focusedValue`。同一次 `WriteInput`（同一 stdin 块）里多个键被 Ink 在一个 tick 内同步逐个处理，React 来不及重渲 → 所有键读到陈旧 focus → ↓ 失效、Enter 永远选第 0 项。**绝不能把整串按键一次性写入**（`answerKeys.ts` 注释）。
2. **待答期不落 JSONL**：AskUserQuestion 阻塞期间整个 assistant 轮次（文本 + 该 tool_use）都不写 JSONL，要等作答/取消才一起落盘。所以**不能从转录派生待答**，只能靠 `TerminalAskPending` 事件或 `orchestrator.PendingAsk` 轮询（memory `xkpane-askuserquestion-card` 的铁证）。
3. **fire-and-forget 漏 pending 的回灌修法（a294635）**：`TerminalAskPending` 事件无重放，晚连/重启的 bot 错过就再也收不到。**唯一可靠恢复**是轮询时读 `orchestrator.PendingAsk(sid)` 把待答带回（webapi `rest.go:263-273,330` 已这么做，botbridge 照做）。botbridge 必须复刻 `effectivePending = 事件 ?? 轮询(polledPending)`，其中 `polledPending` 仅当其 `toolUseId` **尚未现身累积转录**时有效。
4. **只对真正 pending 的会话作答**：往已结束/不在前台的选择器注入按键会污染会话。botbridge 必须严格按 `toolUseId` 守住「只对当前那条待答作答」（对应前端 `SessionTimeline.tsx` 在 pending 在场时抑制未答只读卡）。
5. **AskUserQuestion 双重出现去重**：它同时作为 `assistant.tool_use`（转录里）和 hook 上报（待答）出现。转录映射时若 `ToolName=="AskUserQuestion"` 已在 pending 渲染过，普通卡要跳过，避免双渲染（对应 cc-connect `handleAssistant` 的 `name==AskUserQuestion continue`，`session.go:706`）。

---

## 5. 落地实施计划（给 codex 的分阶段 checklist）

### 5.1 新增/改动文件清单

**后端 `internal/services/botbridge/`（新包）**：

| 文件 | 职责 |
|------|------|
| `botbridge.go` | `Service` 结构体（持 `*storage.Holder` + `BotDeps`），`AttachServices(BotDeps)`、`RouteEvent(name,payload)` observer、`Start/Stop` 生命周期、按会话轮询 goroutine 装配 |
| `platform.go` | 移植 cc-connect `core.Platform`（Name/Start/Reply/Send/Stop）+ 可选 `CardSender`/`InlineButtonSender`/`MessageUpdater` 鸭子接口 + `ButtonOption` + `RegisterPlatform/CreatePlatform` 注册中心 |
| `card.go` | 移植 cc-connect `core.Card` IR + `CardBuilder` + `RenderText` + `CollectButtons` |
| `timeline.go` | `eventToCard(transcript.Event) Card` 映射 + 按会话定位转录（复用 `SessionSnapshots`/`transcript.ResolveClaudePath`/`ParseCodex`，见 §3.2，建议抽 `timelinesrc` 共享） |
| `dispatch.go` | 回调前缀分派（`askq:`/`asksubmit:`/`cmd:/sessions`/`cmd:/switch`/空消息拒收/stale 丢弃）+ 累积 `selections[toolUseId]` + `effectivePending` 失效判定 + 当前绑定会话状态（`boundSessionID`，`/switch` 改写） |
| `answerkeys.go` | §4.3 的 `buildAnswerKeys` Go 移植 + `dedupSortFilter` |
| `answerkeys_test.go` | 对照前端 `answerKeys.ts` 的单测（单选/多选/多题/单题快路径，断言按键序列逐字一致） |
| `service.go` | 薄 Wails 门面：`GetStatus()`（已连/未连）、`SetConfig(platform, appID, appSecret, enabled)` —— 暴露给前端配置页（会话绑定由 IM `/switch` 驱动，不在此门面） |
| `platform/feishu/feishu.go` + `platform/feishu/card.go` | **第一落地适配器（飞书 larkws，已拍板，见 §5.2）**；`card.go` 把 `core.Card` 翻成飞书 interactive card。Telegram 留作后续可选适配器 |

**改动既有文件**：

| 文件 | 改动 |
|------|------|
| `internal/app/app.go` | ① `botBridge := botbridge.New(holder)` + `application.NewService(botBridge)`；② `mirrorEmit` 加第三路 `botBridge.RouteEvent(name,payload)`（§3.1）；③ orchSvc/termSvc 建好后 `botBridge.AttachServices(BotDeps{Sessions:termSvc, Writer:termSvc, AskPend:orchSvc, Scanner:scanSvc})`；④ `botBridge.Start()` fail-open + `defer botBridge.Stop()` |
| `internal/storage/models.go` | `AllModels` 追加 `{Model:&BotBridgeConfig{}, TableName:"bot_bridge_config", LabelKey:"botBridge", Clearable:false}`（含敏感凭据，不可清空） |
| `internal/storage/models_botbridge.go`（新） | `BotBridgeConfig` GORM 结构体：`Platform string` / `TokenEnc string`（cryptox 加密，飞书存 app_secret）/ `AppID string` / `Enabled bool` / **`BindingsJSON string`**（JSON：`map[feishuScopeKey]sessionID`，按 Feishu chat/user 维度绑定——**不要**用全局单值 `BoundSessionID`，与 §5.2/§6 的多维度绑定要求一致）/ `Extra string`(JSON) |
| `internal/events/events.go`（可选） | 若配置页要实时显示连接状态，加 `BotBridgeStatus = "botbridge:status"` 事件 + payload + `Register()` 注册一行；**桌面专属不进 `mirroredEvents` 白名单** |

**新事件**：MVP 可不加（状态用 Wails 方法轮询）。要 live 状态再加 `botbridge:status`。

**storage 表**：要加 `bot_bridge_config`（存 bot token 凭据 + 启用开关 + 会话绑定）。token 落库**必须 `cryptox.EncryptString`**（敏感凭据铁律），不进前端 Snapshot（仿 `WebGatewayTokenEnc`，`models.go:75-78`）。

**Wails 绑定**：`service.go` 有公开方法供前端调，需 `wails3 generate bindings`，前端配 `frontend/src/services/botbridge/BotBridgeService.ts` 包装 `@bindings/xkpane/internal/services/botbridge`。

### 5.2 平台适配器：首落地 Feishu（飞书，已拍板）

**为什么 Feishu 先行**（用户已拍板，参照 cc-platform 深挖结论）：飞书 **WebSocket 长连接模式**（`larkws`）只要 `app_id`/`app_secret` 出站、无需公网回调端点、国内网络稳定，且**原生交互卡片 + 按钮 + 下拉**齐全——AskUserQuestion 的「按钮/选择器」体验最完整（`CardSender` + 真按钮）。直接参照 cc-connect 的 `platform/feishu/feishu.go`（连接/收发）+ `platform/feishu/card.go:91-260`（`renderCardMap`：把 `core.Card` 翻成飞书 interactive card schema）。

**为什么不先 Telegram**：Telegram 虽接入最薄，但大陆需代理、网络不稳；作者在国内，飞书才是稳态首选。Telegram 留作后续可选适配器（`platform/telegram/`，长轮询 + inline keyboard）。**QQ（OneBot）天然不行**——无 `CardSender`/`InlineButtonSender`，AskUserQuestion 只能退化纯文本编号兜底。

**配置字段**（`BotBridgeConfig`：独立列 + `Extra` JSON）：

| 字段 | 必填 | 说明 |
|------|------|------|
| `platform` | ✅ | `"feishu"` |
| `app_id` | ✅ | 飞书自建应用 App ID |
| `app_secret` | ✅ | App Secret（cryptox 加密落 `TokenEnc`，与 `app_id` 一起做凭据） |
| `mode` | ✅ | `"websocket"`（长连接，无公网）；不用 webhook 模式（需 encrypt_key + 公网端口，桌面不适合） |
| `allowFrom` | 🔶 | 白名单 open_id/user_id（仿 cc-connect `core.AllowList` 过滤，防陌生人驱动会话） |
| `chatId` | 🔶 | 默认推送的会话/群 id（不填则回复触发方） |

依赖：飞书官方 SDK `github.com/larksuite/oapi-sdk-go/v3` + `larkws`（长连接订阅消息/卡片回调事件）。卡片按钮回调走飞书 `card.action.trigger` 事件，`value` 字段直接承载 §2.3 的回调约定（`askq:`/`asksubmit:`/`cmd:/...`）。⚠️ 飞书卡片按钮 `value` 是 JSON 对象、**无 Telegram 那种 64 字节硬限**，`asksubmit:<toolUseId>` 可直接放完整 toolUseId（不必查表映射短 token——那是 Telegram 专属约束）。

**鉴权**：`app_id`+`app_secret` 是平台凭据；WebSocket 模式全走出站长连接，无需暴露任何回调端点。`app_secret` 落库必须 `cryptox.EncryptString`，不进前端 Snapshot。

**`/sessions` 会话切换（已拍板从 MVP 起做）**：飞书适配器要支持 `cmd:/sessions`——把 `BotDeps.Sessions.SessionSnapshots()` 渲染成 `CardListItem` 列表（每行一个会话，按钮 `value=cmd:/switch <sid>`），用户点选即 `dispatch.go` 改写 `boundSessionID`，后续卡片推送/作答都对准新绑定会话。

> **绑定作用域（codex 评审 #5）**：`boundSessionID` **不要做成单一全局值**——按 **Feishu chat/user 维度**保存（`map[chatOrUserID]boundSessionID`），避免一个群/用户切换绑定影响到另一个。MVP 即便简化为「单用户单 chat」，也要在代码注释里写明这一约束，便于后续扩多用户。

### 5.3 前端配置页

按「设置页子目录隔离规范」加 `SettingsPage/botbridge/` 子页面（**不新建顶层路由页**）：

- 文件：`botbridge/{Botbridge.tsx, useBotbridge.ts, Botbridge.styles.ts, index.ts, lang/{zh-CN.ts, en-US.ts, index.ts}}`
- 控件：启用开关（Switch）、平台选择（Select，默认 `feishu`，Telegram 后补）、飞书 `app_id` + `app_secret` 输入（TextField，secret masked）、连接状态 Chip。**「绑定会话」不放配置页**——已拍板由 IM 内 `/sessions` 切换（M4），配置页只管启停与凭据。
- 走 `BotBridgeService.{getStatus, setConfig}`（`bindSession` 改由 IM `/switch` 驱动，前端无需此方法）。
- **i18n 铁律**：所有文案走 `t('settings.botbridge.*')`，namespace 严格 `settings.botbridge.*`。
- **持久化铁律**：token 经后端 `SetConfig` 落 SQLite 加密列，**禁止** localStorage。
- **Icon 铁律**：用 `@mui/icons-material` 的 `*Rounded`（如 `SmartToyRounded`/`HubRounded`），禁 emoji。
- 注册：`SettingsPage/useSettingsPage.ts` 的 `items` 加一项；`SettingsPage/lang/index.ts` 调 `registerBotbridgeLocales()`；`SettingsPage.tsx` 右列分发加 `{vm.activeItem.id === 'botbridge' && <Botbridge />}`。

### 5.4 分阶段里程碑（逐单元独立提交、不 push、停在手测点）

| 里程碑 | 可验证目标 | 验证方式 |
|--------|-----------|----------|
| **M1 地基 + 配置持久化** | botbridge 服务骨架 + `mirrorEmit` 第三路 + `RouteEvent` 收 `TerminalAskPending`（仅打 log）+ **`bot_bridge_config` 表（cryptox 加密）+ 后端 `SetConfig`/读取**（凭据须在 M4 前就能落库/读出） | `go build .` 通过；触发 AskUserQuestion 见日志；`SetConfig` 写入的飞书 app_id/secret 能加密落库并读回 |
| **M2 卡片轮询** | `eventToCard` + 按会话轮询 `transcript.Parse`，新卡片打到日志（无平台） | 跑一段 claude 会话，日志按 `after` 游标增量出 user/assistant/tool 卡，无重复无遗漏 |
| **M3 answerkeys** | `answerkeys.go` 移植 + `answerkeys_test.go` 对照前端逐字一致 | `go test ./internal/services/botbridge/` 全绿 |
| **M4 Feishu 适配器 + /sessions** | `Platform`/`CardSender`/`InlineButtonSender`（飞书 `larkws` 长连接）+ `cmd:/sessions` 列会话 + `cmd:/switch` 切绑定，卡片推到飞书 | 手测★：飞书里 `/sessions` 列出活跃会话、点选切换后实时收到该会话卡片镜像 |
| **M5 反向文本** | 飞书打字 → `WriteInput(text+\r)` 写回当前绑定 claude | 手测★：飞书发消息，桌面终端里 claude 收到并响应 |
| **M6 AskUserQuestion 闭环** | 待答推飞书交互卡按钮 → 点选累积 → `asksubmit` → 逐键作答 → pending 清除 | 手测★：claude 触发 AskUserQuestion，飞书出按钮卡，点选后桌面选择器被正确驱动、答案落地 |
| **M7 配置页（前端 UI）** | `SettingsPage/botbridge/` 子页面（启停开关 + 飞书凭据录入 UI，后端持久化已在 M1 就位） | 手测：设置页填飞书 app_id/secret、开关启停、重启后配置仍在 |

> **里程碑调整（codex 评审 #4）**：原 M7 的 `bot_bridge_config` 加密落库 + 启停**后端能力已提前到 M1**——因为 M4 跑飞书 larkws 必须先有凭据来源，配置在 M7 太晚会倒置。M7 只剩前端设置页这层 UI。若想在 M7 前更早手测 M4，可临时从 env/`config` 文件读飞书凭据，但不作为正式路径（正式路径=M1 的 `SetConfig` 加密落库）。

每个里程碑一个独立 commit，不 push，M4/M5/M6 停在手测点等用户验证。

### 5.5 测试策略

**Go 单测**（与被测包同目录）：

- `answerkeys_test.go`：单选（`{0:[2]}` → `[down,down,enter]`）、单题快路径（`hideSubmitTab` 不补尾 Enter）、多选（`{0:[1,3]}` → 对照前端实际序列）、多题，**逐字断言对照前端 `answerKeys.ts`**。
- `timeline_test.go`：`eventToCard` 各 Kind 映射 + Detail（edit diff / bash）分支；AskUserQuestion tool_use 去重跳过。
- `dispatch_test.go`：`askq:`/`asksubmit:` 解析、空消息拒收、stale 丢弃、`effectivePending`（toolUseId 现身即失效）、selection 累积。
- platform 用 fake（仿 webapi 窄接口 fake 测法）。
- ⚠️ **铁律**：在 XK 自启动会话内跑 botbridge 测试若间接拉起 `terminal` 包，须先清 `XKPANE_*` 环境变量（CLAUDE.md 测试策略）。

**手测清单**：

1. 实时同步：桌面跑 claude，飞书里实时见同会话卡片流（user/assistant/tool/diff/bash）。
2. 推卡片增量：长会话不重不漏（`after` 游标正确）。
3. 反向文本：飞书发消息 → claude 收到。
4. AskUserQuestion 闭环：单选/多选/多题三种，飞书点选后桌面 Ink 选择器被正确驱动、答案落库、pending 卡撤掉。
5. 晚连恢复：先触发 AskUserQuestion，再让 bot 连上，靠 `PendingAsk` 轮询补出待答卡（验证 a294635 修法）。
6. codex 会话：验证 `ParseCodex` 路径卡片正常（codex 的 AskUserQuestion 不一定走同选择器，标注为已知边界）。

---

## 6. 待用户拍板的开放决策

| # | 决策点 | 选项 | 推荐 + 理由 |
|---|--------|------|-----------------|
| 1 | **第一落地平台** | Telegram / Feishu / QQ | ✅ **已拍板 = Feishu（飞书）WebSocket**（无公网+原生交互卡片按钮+完整 AskUserQuestion+国内网络稳）。Telegram 后续可选（大陆需代理）。QQ 排除（无按钮能力）。 |
| 2 | **内嵌 XK-Pane vs 旁路跑 cc-connect** | 内嵌 botbridge / 外挂 cc-connect | **内嵌 botbridge**。需求是「对接当前 PTY 终端」，cc-connect 是 spawn 独立 headless 会话，对接不到眼前会话；且内嵌才能复用 `mirrorEmit`/`PendingAsk`/`WriteInput`，零 HTTP 自环。 |
| 3 | **单向只读推送 vs 双向可写回** | 只读 / 双向 | **双向**，但分阶段：M4 先只读推卡片，M5/M6 再开写回（文本 + AskUserQuestion 作答）。写回是用户核心诉求（AskUserQuestion 作答），但风险最高（逐键 focus 坑/污染会话），放后面单独手测。 |
| 4 | **bot 绑定哪个会话** | 唯一活跃 / 用户在 IM 选 / 最近活跃 | ✅ **已拍板 = IM 内 `/sessions` 命令切换**（飞书里列活跃会话、点选切换，`dispatch.go` 改写 `boundSessionID`，借鉴 cc-connect SessionKey）。从 MVP 起做（M4）。**绑定按 Feishu chat/user 维度存**（评审 #5），非单一全局值。 |
| 5 | **多选/多题 UX** | 点击即提交 / 累积+Submit | **累积+Submit**（XK 必须）。因 XK 走 `buildAnswerKeys` 批量翻按键，不能像 cc-connect 每点一次 resolve。单题单选给「点击即提交」快路径。 |
| 6 | **共享 timeline 定位逻辑是否现在抽** | 现在抽 `timelinesrc` / 先内联留 TODO | **先内联留 TODO**（surgical），M2 跑通后再抽共享函数让 webapi 与 botbridge 复用（消 SUBTRACTION_AUDIT 标记的重复）。 |

---

## 7. 风险与边界

**模式/协议风险**：

1. **模式不可混用**（最根本）：cc-connect 全部能力建立在 headless stream-json 之上，真 TUI 会话**没有 control_request/control_response 入口**。别试图把 cc-connect 的 `RespondPermission` 套到现有终端镜像——它在真 TUI 里不存在协议入口。
2. **AskUserQuestion 强耦合 Ink 选择器**：`buildAnswerKeys` 是按 claude CLI 当前 Ink 选择器行为（tab 式多题、单选 Enter 自动前进、多选 Space 切换+RIGHT 切题、末尾 Submit 复核 tab）硬编码。**claude CLI 升级改 TUI 键位/tab 结构就整体失效**——版本漂移高危区。codex CLI 的 AskUserQuestion 不一定走同选择器（仅 claude/reclaude 验证过），codex 会话作答按已知边界处理。
3. **待答态零持久化/零重放**：`TerminalAskPending` fire-and-forget，阻塞期整轮不落 JSONL。bot 只读转录永远拿不到待答，必须订阅事件 + 轮询 `PendingAsk` 双源兜底（事件路径还缺 toolUseId，得靠轮询补）。
4. **后端只记最近一条 pendingAsk**：`s.pendingAsk[sessionID]` 按 sessionID 单值覆盖，同会话连续多个待答可能丢早的。bot 失效判定靠 toolUseId 现身转录，不要假设 map 里恒是当前那条。

**并发/线程/反压**：

5. **绝不反压回 PTY**：botbridge observer 的 `RouteEvent` 和卡片轮询都不能同步阻塞 `mirrorEmit`/`terminal.batchLoop`。照抄 `hub.go:80-98 broadcast` 的有界队列 + 慢端剔除（队列满即丢/重连，绝不等待）。一个卡顿的 IM 平台不能拖垮整机终端。
6. **逐键 100ms 延时是串行的**：作答一道多题多选可能写十几个键、耗 1-2s。这期间该会话被「占用」，别让轮询/其它写并发插入按键污染选择器。作答用 per-session 锁串行化。
7. **同 cwd 并发会话定位歧义**：claude 旧会话（未带 `--session-id`）走「扫最新 jsonl」回退（`rest.go:295-308`）会取错文件。bot 定位转录**优先依赖钉死的 `ClaudeSessionID`**（`SessionSnapshot.ClaudeSessionID`），codex 恒无钉死 id（已知边界）。
8. **codex 转录格式随版本漂移**：老文件无 `exec_command_end` 旁路（memory `xkpane-codex-timeline-cards`）。bot **复用 `transcript.ParseCodex`**，别另写一份解析器承接同样的兼容风险。

**平台/网络/Windows**：

9. **公网入站边界**：XK 本机桌面通常无公网。优先「出站长连接」形态（Telegram 长轮询 / Feishu WebSocket）。WeCom 必须公网回调端点、Feishu webhook 模式需 encrypt_key+公网端口——不适合本机桌面。
10. **Telegram 大陆需代理**：`New` 里 proxy 字段，且属境外平台网络不稳。
11. **回调 data 长度**：Telegram CallbackData ≤64 字节。`asksubmit:<toolUseId>` 若 toolUseId 超长要服务端查表映射短 token。
12. **能发交互按钮是硬门槛**：QQ/微博只有 `ImageSender`/`FileSender`，AskUserQuestion 退化纯文本编号。要按钮作答，QQ 这条线天然不行。
13. **Windows 进程树**：cc-connect 的 `proc_windows.go`（`CREATE_NEW_PROCESS_GROUP` + `taskkill /T` 收 MCP 子孙）是 **spawn-agent 模式专属**——本方案不 spawn agent，**不需要**这套。但 botbridge 起的 goroutine/平台连接要随 `Stop()` 洁净退出（ctx cancel）。
14. **token 凭据红线**：bot token 落库必须 `cryptox.EncryptString`，不进前端 Snapshot；别给前端开万能写端点（项目「不给万能 RunCommand」铁律），反向写只走 `WriteInput` 这一窄通道。

---

## 核心源码锚点速查（均已核实）

- 实时注入：`internal/app/app.go:239-242`（mirrorEmit）、`356-372`（AttachServices 范式）
- 事件镜像范式：`internal/webapi/gateway.go:96-101`（RouteEvent）、`internal/webapi/hub.go:80-98`（非阻塞 broadcast）、`internal/webapi/rest.go:28-34`（白名单）
- 卡片数据源：`internal/services/transcript/transcript.go:78-122`（Event/Detail/AskQuestion）、`internal/webapi/rest.go:247-323`（定位+解析）
- 待答存取：`internal/services/orchestrator/rest.go:101-147`（PendingAsk/handleAskPending）、`internal/events/events.go:80-92`（AskQuestion/TerminalAskPendingPayload）
- 反向写：`internal/services/terminal/terminal.go:390-399`（WriteInput）、`455-467`（SessionSnapshot）
- 作答翻译移植源：`frontend/src/components/SessionTimeline/answerKeys.ts` + `frontend/src/components/TerminalKeyBar/useTerminalKeyBar.ts`（KEY_SEQUENCES）
- 加表范式：`internal/storage/models.go`（AllModels）

---

## cc-connect 参照源码锚点（J:/Program/Agent/cc-connect）

- 接口契约：`core/interfaces.go`（Platform / CardSender / InlineButtonSender / MessageUpdater）
- 卡片抽象：`core/card.go`（Card / CardBuilder / RenderText / CollectButtons）
- 桥接与回调分派：`core/bridge.go`、`core/engine.go`（handlePendingPermission / sendAskQuestionPrompt）
- agent 驱动（**仅参照，不搬**）：`agent/claudecode/session.go`、`agent/claudecode/cc_hooks.go`
- 平台适配器范例：`platform/telegram/`、`platform/feishu/`（card.go 卡片渲染）
- 注册接线：`core/registry.go`、`cmd/cc-connect/plugin_platform_*.go`
