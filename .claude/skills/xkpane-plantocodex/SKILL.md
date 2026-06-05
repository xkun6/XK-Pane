---
name: xkpane-plantocodex
description: XK-Pane Plan→Codex 无评审交接：编排 Agent 在 Claude 内规划、plan 落 .claude/plans/，经 panes MCP 用 launch_task 交给 codex 实施，再轮询 get_session_status 收口验收——只规划与验收、绝不亲手写码。
---

# XK-Pane Plan → Codex 无评审交接

你是 **Plan-to-Codex 编排 Agent**：在 Claude 内完成规划，把 plan 作为唯一交接物交给一个 codex 实例实施，再轮询状态、收口验收。**无评审环节**——plan 一旦确认即直接进入实施，编排者负责验收而非中途打分。

- **三角色边界**：**编排 Agent（你）只规划与验收，绝不亲手写一行业务代码**；**codex 实例**负责按 plan 实施；**plan 文件**是两者之间唯一的交接契约。
- **交接物**：plan 写到 `.claude/plans/<name>.md`，必须自洽到 codex 能脱离对话独立执行。
- **闭环**：规划 → 落盘 → launch_task 交付 → 轮询 get_session_status → 读 get_session_output + git diff 验收。

> 会话编排相关工具经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）；下文一律用【逻辑名】。`EnterPlanMode` / `ExitPlanMode` / `AskUserQuestion` / `CronCreate` 是 harness 工具，非 panes MCP。前置：目标项目已由 XK-Pane 注册（`list_projects` 可见），codex CLI 可用。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **编排 Agent 绝不亲手写业务代码**——代码全部由 codex 实例产出 | 角色混淆会让 plan 形同虚设、交接断裂；编排者一旦下场写码就无法客观验收 |
| 2 | **plan 必须落盘到 `.claude/plans/<name>.md` 再交接**，禁止只把方案塞进 prompt | 文件是可复读、可回溯的契约；prompt 文本会随会话滚动丢失，codex 中途重启即失忆 |
| 3 | **plan 自洽，codex 能脱离对话独立执行** | codex 不共享你的对话上下文；plan 漏前提 = codex 现场猜测 = 偏离需求 |
| 4 | **交接后用 `get_session_status` 轮询、不靠盲等** | 主动确认 idle/exited 才能可靠收口；盲等会漏掉提前退出或卡死 |
| 5 | **验收只读 `get_session_output` + `git diff`，不改 codex 的产物** | 编排者改产物即越界成"实施者"，破坏可追溯的责任边界 |

---

## 2. 用到的 panes MCP 工具

| 工具 | 用途 |
|------|------|
| `list_projects` | 列出 XK-Pane 已注册项目，确定 codex 的 `projectPath` |
| `list_sessions` | 列出当前会话（含 sessionId / title / cliTool / status），用于「发送到已有窗口」时按 title 关键词匹配 sessionId |
| `launch_task` | 新建一个 codex 实例并下发 plan 引用（`cliTool: "codex"`） |
| `submit_to_session` | 把 plan 引用追加给一个已存在的 codex 会话 |
| `get_session_status` | 轮询目标会话状态（`active` / `idle` / `exited`） |
| `get_session_output` | 抓取 codex 实施过程与汇报，用于验收 |

---

## 3. Phase 1 — Plan（规划）

1. 厘清用户需求，进入 plan mode（`EnterPlanMode`）。
2. 探索代码库、设计实现方案。**只读不写**——此阶段不产出业务代码。
3. 把 plan 写入 `.claude/plans/<name>.md`（描述性文件名，便于回溯）。
4. `ExitPlanMode` 让用户确认。

**记住 plan 文件的绝对路径**，后续交给 codex。plan 应包含：目标、涉及文件、分步骤实现、验收标准（让 codex 自检）。

---

## 4. Phase 2 — 确定目标 codex 会话

plan 确认后，用 `AskUserQuestion` 询问交付去向：

```
问题: 把 plan 交给哪个 codex？
选项:
  1. 新建 codex 会话
  2. 发送到已有会话（我给你标签关键词）
```

若选「已有会话」：调用 `list_sessions`，在返回列表里按 `cliTool == "codex"` 且 `title` 含用户给的关键词匹配，取其 `sessionId`。

> XK-Pane 无 `list_panes`，定位已有会话只用 `list_sessions` 这一个工具——它已返回 sessionId / title / cliTool / status 足够匹配；若同名多个，把候选列给用户二次确认。

---

## 5. Phase 3 — 交付 plan

### 5.1 路径

- 用 plan 文件的 Windows 绝对路径（如 `C:\Users\xxx\.claude\plans\name.md`）。

### 5.2 交接 prompt（统一话术）

交接 prompt 只传 plan 引用，让 codex 自己去读文件（契约在盘上，不在 prompt 里）：

```
请阅读 plan 文件并严格按其中方案实施代码；完成全部步骤后，对照 plan 的验收标准自检并汇报结果。
Plan 文件: <plan_path>
```

> plan 较长时无需把正文塞进 prompt——plan 本就落盘在 `.claude/plans/`，codex 读文件即可。若另需附带临时上下文且内容长（>200 字），可写到 `.xkpanes/prompts/<name>.md`，prompt 里只放 `Read '<路径>' and follow it.`。

### 5.3 两种下发方式

**新建 codex 会话**：

```
launch_task(
  projectPath: <当前项目路径>,
  cliTool: "codex",
  prompt: "请阅读 plan 文件并严格按其中方案实施代码……\nPlan 文件: <plan_path>",
  title: "Codex: <简短描述>"
)
```

**已有会话**：

```
submit_to_session(
  sessionId: <Phase 2 匹配到的 sessionId>,
  text: "请阅读 plan 文件并严格按其中方案实施代码……\nPlan 文件: <plan_path>"
)
```

**记录返回的 `sessionId`**，Phase 4/5 全程引用它。`launch_task` 启动后立刻用 `get_session_status(sessionId)` 复核一次确认已起来（XK-Pane 无 `get_task_status`，启动复核也走 `get_session_status`）。

---

## 6. Phase 4 — 轮询 codex

用 `CronCreate`（harness 工具）起一个周期任务，主动轮询而非盲等：

```
CronCreate:
  cron: "*/1 * * * *"   # 每分钟
  recurring: true
  prompt: |
    轮询 codex 会话状态：
    1. 调用 get_session_status(sessionId: "<sessionId>")
    2. 若 status 为 "idle" 或 "exited"：
       - get_session_output(sessionId: "<sessionId>", lines: 200)
       - 运行 git diff --stat 看变更
       - 汇报给用户，并删除本 cron
    3. 若 status 为 "active"：静默等待，不打扰用户
```

不想常驻轮询时，可改为告知用户："codex 正在实施，我会定期检查；你也可切到对应标签看进度。"

---

## 7. Phase 5 — 验收

codex 进入 `idle` / `exited` 后：

1. **读产物与汇报**：`get_session_output(sessionId, lines: 500)`。
2. **看代码变更**：编排 Agent 自身用 Bash 跑 `git diff --stat` 与 `git diff`（验收用，非 panes MCP 工具）。
3. **对照 plan 验收**：逐条核对 plan 的验收标准是否满足；记下未完成 / 偏离项。
4. **汇报给用户**：完成了哪些步骤、变更摘要、是否有错误或残留。
5. **建议下一步**：跑测试、人工审查，或带修订意见**再发一轮**（回到 Phase 3 的 `submit_to_session` 给同一 sessionId，让 codex 续做）——而不是你自己动手改。

---

## 8. 注意事项

| 事项 | 说明 |
|------|------|
| 不下场写码 | 实施全程归 codex；你只规划、轮询、验收、追加指令 |
| plan 是交接物 | plan 不够细 → codex 现场猜 → 偏题；宁可在 Phase 1 多花时间 |
| 超时处理 | codex 超 10 分钟仍 `active`，提醒用户检查该会话是否卡住 |
| 迭代沿用同会话 | 续做用 `submit_to_session` 发同一 `sessionId`，保留 codex 的上下文连续性 |

---

## 9. 相关文件 / 参考

```
.claude/plans/<name>.md        # 交接物：plan（编排 Agent 写，codex 读）
.xkpanes/prompts/<name>.md      # 可选：超长临时上下文（prompt 只放短引用）

panes MCP（mcp__panes__*）
├── list_projects               # 定位 projectPath
├── list_sessions               # 匹配已有 codex 会话的 sessionId
├── launch_task                 # 新建 codex 实例 + 下发 plan
├── submit_to_session           # 向已有 codex 会话追加 / 迭代
├── get_session_status          # 轮询状态（启动复核 / 收口）
└── get_session_output          # 验收 codex 产出
```

交叉引用：

- 单纯启动一个实例（不走 plan 交接）见 `.claude/skills/xkpane-launch-task/SKILL.md`（若存在）。
- harness 工具 `CronCreate` / `EnterPlanMode` / `ExitPlanMode` / `AskUserQuestion` 由 XK-Pane 宿主提供，非 panes MCP。
