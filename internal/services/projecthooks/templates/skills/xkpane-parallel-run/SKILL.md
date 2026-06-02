---
name: xkpane-parallel-run
description: XK-Pane 监督并行编排指南：用 launch_task fan-out 起多个 Claude/Codex 子实例，轮询 get_session_status（Active/Idle/Exited）+ get_session_output 汇总；含长 prompt 落文件、20-60 秒轮询、同项目防 git 冲突铁律。
---

# XK-Pane 监督并行

把一个大任务拆成互不依赖的子任务，**fan-out 并行启动多个 Claude/Codex 实例执行，轮询状态直到全部收敛，再统一汇总**。本 skill 指导你（一个由 XK-Pane 启动的子实例）如何用 panes MCP 编排其它实例，自己退居「监督者」角色。

- **fan-out**：一次拆多个独立子任务，每个 `launch_task` 起一个会话，并行跑
- **轮询而非阻塞**：不要 sleep 死等，循环 `get_session_status` 探活，状态变化才上报
- **收口汇总**：所有会话进入终态后，`get_session_output` 收割各自结果，标注成功 / 失败
- **状态枚举**：会话状态只有三种 —— `Active`（正在干活）/ `Idle`（等输入或已停手）/ `Exited`（进程已退出）

> 所有工具经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。正文一律用【逻辑名】（如 `launch_task` / `get_session_status`），不再重复写前缀。
> 前置：XK-Pane 已运行，目标项目已在 XK-Pane 中登记（`list_projects` 能列出）。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | 子任务**必须互相独立、无依赖**才并行 | 有依赖就不是 fan-out，会因执行顺序产生竞态 / 脏读 |
| 2 | **同一项目内并行禁止改同一文件**，更要警惕 git 冲突 | 多实例同时写同一工作区会互相覆盖、产生冲突 marker，甚至污染 `git add` 暂存区 |
| 3 | prompt 超过约 200 字**必须先写 `.xkpanes/prompts/<task>.md`** 再传短引用 | 超长 prompt 直接塞 `launch_task` 易被截断 / 转义出错；落文件最稳 |
| 4 | 轮询间隔**保持 20-60 秒**，禁止亚秒级狂轮 | 过密轮询挤占 MCP 通道、刷屏，且子实例本就以秒级推进，看不出差别 |
| 5 | **绝不靠 sleep 死等**，靠 `get_session_status` 探到终态才收割 | 死等会卡死监督者；状态驱动才能及时上报与介入 |
| 6 | 收割结果**只读 `get_session_output`**，不替子实例改它的产物 | 监督者越界改文件会与子实例的写入打架，破坏其工作区一致性 |

---

## 2. MCP 工具

只用以下 panes MCP 工具：

| 工具 | 用途 |
|------|------|
| `list_projects` | 列出 XK-Pane 已登记的项目，确认每个子任务的目标项目存在 |
| `launch_task` | 启动一个 Claude/Codex 子实例执行单个子任务，返回 `sessionId` |
| `get_session_status` | 查询某会话状态（`Active` / `Idle` / `Exited`），轮询用 |
| `get_session_output` | 读取某会话的完整输出，收割结果用 |
| `list_sessions` | （兜底）列出当前所有会话，核对 fan-out 出去的会话是否齐全 |
| `kill_session` | （兜底）某子任务跑飞 / 卡死且需放弃时，强制终止该会话 |

---

## 3. 编排流程

### 3.1 第一步：确认任务可并行

把大任务拆成子任务清单，逐条自检铁律 #1：

```
1. 子任务彼此独立吗？（无 A 依赖 B 的产物）
2. 落在哪些项目？同项目 or 跨项目？
3. 每条子任务的目标 + 具体 prompt 是什么？
```

若拆不出独立子任务（强依赖链），**放弃 fan-out**，改为串行交接（见第 6 节交接类 skill）。
若信息不足，先向上游问清「拆哪些子任务 / 在哪些项目 / 各自做什么」，确认拆分方案后再启动。

> 用 `list_projects` 校对：每个子任务的目标项目都必须在列表里，否则 `launch_task` 会找不到工作区。

### 3.2 第二步：长 prompt 落文件（按需）

任一子任务 prompt 超过约 200 字，先落文件再引用（铁律 #3）：

```
1. 把完整 prompt 写到目标项目下 .xkpanes/prompts/<task-name>.md
2. launch_task 时传短引用：
   Read task from '<.xkpanes/prompts/xxx.md 的路径>' and execute it. Delete the file after reading.
```

短 prompt（一两句话）直接内联给 `launch_task` 即可，不必走文件。

### 3.3 第三步：fan-out 启动

对每个子任务调一次 `launch_task`，**记下返回的全部 `sessionId`**（连同它对应的子任务名 / 项目）。逐个启动即可，无需等前一个就绪。

| `launch_task` 关键入参 | 说明 |
|------|------|
| 目标项目 | 该子任务的工作区（来自 `list_projects`） |
| `cliTool` | `claude` 或 `codex`，按子任务性质选 |
| prompt | 内联短 prompt，或 3.2 的短引用 |

启动后可用 `list_sessions` 核对：fan-out 出去的会话数量与 `sessionId` 清单一致。

### 3.4 第四步：轮询监控

按固定节奏循环探活，状态有变才上报，全部终态即收尾：

```
每 20-60 秒一轮：
  for each sessionId in 清单:
    status = get_session_status(sessionId)
    若 status 较上一轮变化 → 上报用户（如 Active → Idle）
  若所有会话都进入终态（Idle / Exited）：
    跳出循环，进入第五步
  否则继续下一轮
```

状态语义：

| 状态 | 含义 | 监督者动作 |
|------|------|-----------|
| `Active` | 子实例正在推进 | 继续轮询 |
| `Idle` | 子实例停手（完成、等输入、或卡住）| 视为「可收割候选」；若疑似卡住需介入 |
| `Exited` | 子实例进程已退出 | 视为终态，直接收割其输出 |

> 某会话长时间停在 `Active` 无新输出，或停在 `Idle` 却像是在等输入：提醒用户介入；确需放弃该子任务时用 `kill_session` 终止，并在汇总中标注。

### 3.5 第五步：收割与汇总

所有会话进入终态后：

```
1. 逐个 get_session_output(sessionId) 读完整输出
2. 判定每个子任务的成功 / 失败（看输出里的完成标志、报错、git 状态等）
3. 汇总成一份总结：子任务 → 项目 → 状态 → 关键产出 / 失败原因
4. 失败的子任务给出建议（重试 / 拆细 / 手动接管）
```

汇总只读不写：不要替子实例改它的代码或提交它的 git（铁律 #6）。

---

## 4. 同项目并行的 git 冲突防护

多个子任务落在**同一项目**时（铁律 #2），逐条核对：

| 风险 | 防护 |
|------|------|
| 两个实例改同一文件 | 拆分时就按文件 / 模块切割，确保各子任务的「写集合」不相交 |
| 并发 `git add` / commit | 约定只让**一个**实例负责提交，或各自工作在不同分支 / worktree |
| 暂存区互相污染 | 子任务 prompt 中明确「只动你负责的文件，不要 `git add .`」 |
| 收割时分不清谁改了啥 | 让每个子实例在输出末尾自报「我改了哪些文件」，汇总时据此核账 |

> 能跨项目并行就尽量跨项目（天然无共享工作区）；非要同项目并行，务必把写集合切到不相交。

---

## 5. 典型场景

```
跨项目并行：在 projectA 与 projectB 各跑一遍测试 / 各修一个 bug
  → 每项目一个 launch_task，互不干扰，最稳

同项目多文件并行：一次修 auth 模块 3 个互不相干的 bug
  → 拆成写集合不相交的 3 个子任务，注意 git（见第 4 节）

混合 CLI：让 codex 实例改实现、claude 实例补文档
  → cliTool 分别选 codex / claude，两会话并行轮询
```

---

## 6. 相关文件 / 参考

```
.xkpanes/
└── prompts/
    └── <task-name>.md   # 长 prompt 落文件处（子实例读后自删）
```

交叉引用（同为 XK-Pane bundled skill）：

- `/xkpane-launch-task` —— 单个子任务的启动细节（`launch_task` 入参 / cliTool 选型）
- `/xkpane-browse-sessions` —— 用 `list_sessions` / `get_session_output` 浏览与回看会话
- `/xkpane-plantocodex` —— 拆不出独立子任务时，改走 plan → codex 串行交接工作流
