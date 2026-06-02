---
name: xkpane-browse-sessions
description: XK-Pane 会话浏览原语：经 panes MCP 列出全部终端会话、查单个会话状态、读会话输出三件套，监控并调试其它实例的闭环。
---

# XK-Pane 会话浏览

枚举 XK-Pane 中所有终端标签页（会话）的状态，读取指定会话的输出内容，用于**监控、调试、定位**由你或其它实例启动的任务。

- **三件套**：`list`（列全部会话）/ `status`（查单个会话）/ `read`（读会话输出），覆盖“看见 → 定位 → 读取”的完整闭环。
- **只读不写**：本 skill 只观察会话，不向会话写入；要发指令 / 提交回车 / 杀进程见 `write_to_session` / `submit_to_session` / `kill_session`；要启动新任务见 `/xkpane-launch-task`。
- **调用方式**：以下工具均**经 panes MCP 调用**（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名），正文统一用【逻辑名】，不再重复写前缀。

> 参数：`$ARGUMENTS`（子命令 + 可选 sessionId / lines，见第 3 节）。
> 会话由 XK-Pane 的 panes 进程维护，不依赖任何本地文件；会话 ID 在 `list` 输出里取。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **会话 ID 必须先 `list_sessions` 取**，不要凭记忆或猜测拼 ID | ID 由 panes 进程分配，跨重启 / 跨标签会变；猜测会读到错误会话或报 not found |
| 2 | **读输出统一走 `get_session_output`**，禁止去翻本地日志文件或终端缓存 | 会话输出由 panes 进程持有；绕过工具拿到的是过期 / 不完整快照 |
| 3 | **本 skill 不向会话写任何内容**（含回车），只 list / status / read | 浏览语义必须无副作用；误写会打断目标实例正在跑的任务 |
| 4 | **MC PE 不在终端会话里**，不要用本 skill 查 MC 运行时 | MC PE 走 `get_mc_pe_logs`（环形缓冲），与 Terminal sessions 是两套通道，见 `/xkpane-mc-pe-debug` |

---

## 2. MCP 工具

本 skill 只用到三个会话浏览工具（均在 panes MCP）：

| 工具 | 用途 |
|------|------|
| `list_sessions` | 列出所有活跃终端会话（拿 sessionId 的唯一入口） |
| `get_session_status` | 查询指定会话的详细状态 |
| `get_session_output` | 读取指定会话的输出内容（可指定行数） |

> 相邻能力（不属本 skill）：启动任务 `launch_task`、向会话写入 `write_to_session`、提交回车 `submit_to_session`、结束会话 `kill_session`。

---

## 3. 子命令

解析 `$ARGUMENTS`，执行对应操作：

### `list` / 无参数 — 列出所有活跃会话

调用 `list_sessions`，以表格展示：

| 列 | 说明 |
|----|------|
| sessionId | 会话 ID（后续 status / read 的入参） |
| status | Active / Idle / WaitingInput / Exited |
| lastOutputAt | 最后输出时间 |

### `status <sessionId>` — 查看会话详细状态

调用 `get_session_status`（参数：`sessionId`）。展示该会话当前状态、是否在等待输入、最近活动时间。

### `read <sessionId> [lines]` — 读取会话输出

调用 `get_session_output`：

- `sessionId`：目标会话（必填，从 `list` 取）
- `lines`（可选）：返回最近 N 行，默认 100

展示输出内容，**高亮其中的错误 / 异常 / 失败信息**。

---

## 4. 示例

```
/xkpane-browse-sessions                       # 列出所有活跃会话
/xkpane-browse-sessions list                  # 同上
/xkpane-browse-sessions status abc123         # 查看会话状态
/xkpane-browse-sessions read abc123 50        # 读取最近 50 行输出
```

---

## 5. 典型用途

| 场景 | 流程 |
|------|------|
| **监控其它实例进度** | `list` 看全局 → 对感兴趣会话 `read` 看输出 |
| **调试失败任务** | `list` 找到 status=Exited / 异常的会话 → `read` 拉错误栈定位根因 |
| **确认会话是否卡住** | `status` 看是否 WaitingInput（卡在交互提示）→ 需要回复则改用 `submit_to_session`（不在本 skill 内） |
| **盘点并行任务** | `list` 一次性看清当前有几个会话、各自状态，决定下一步监督动作（见 `/xkpane-parallel-run`） |

---

## 6. 相对移植来源（ccpanes 版）的裁剪

本 skill 由 ccpanes 版「会话浏览」移植而来。XK-Pane 的 panes MCP 白名单**不提供**“启动历史”/“Claude 历史会话清单”这两类查询工具，故移植时砍掉了对应的两个子命令：

| 原子命令 | XK-Pane 处置 |
|----------|--------------|
| `history [projectPath]` | **已砍**。无启动历史查询工具；只能用 `list_sessions` 看**当前活跃**会话，看不到已退出且已清理的历史启动记录 |
| `claude-sessions [projectPath]` | **已砍**。无 Claude 历史会话清单工具 |

**“恢复中断会话”的替代路径**：原流程是先查历史会话 ID → 再用 launch-task 的 resume 恢复。XK-Pane 下因无历史清单工具，改为：先 `list_sessions` 确认目标会话**是否仍活跃**——若活跃，直接 `read` / 继续交互即可；若已退出，则需通过 `/xkpane-launch-task` 重新发起任务（无法靠历史 ID 自动 resume）。

---

## 7. 相关 skill / 参考

```
.claude/skills/  (= .codex/skills/)
├── xkpane-browse-sessions/   # 本 skill：list / status / read 三件套
├── xkpane-launch-task/       # 启动新任务（launch_task）
├── xkpane-parallel-run/      # 监督并行：多会话编排
└── xkpane-mc-pe-debug/       # MC PE 调试（get_mc_pe_logs，独立于终端会话）
```

- 向会话发指令 / 回车 / 结束：`write_to_session` / `submit_to_session` / `kill_session`（panes MCP，非本 skill 范畴）。
- 项目 / 工作空间盘点：`list_projects` / `list_workspaces`。
