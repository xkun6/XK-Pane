---
name: xkpane-launch-task
description: 用 XK-Pane 的 panes MCP 在已注册项目里启动 Claude/Codex 子实例的基础原语：list_projects 选项目、launch_task 起会话、get_session_status 确认；长 prompt 写文件用短引用反黑屏。
---

# 启动任务

在 XK-Pane 管理的项目中启动一个新的 Claude/Codex 子实例——这是所有编排流程的**基础启动原语**。

- **入口**：`launch_task`（建会话 + 注入初始 prompt 或 `--resume` 恢复）
- **选项目**：项目必须已注册，用 `list_projects` 查合法 `project_path`
- **确认**：无独立任务状态查询；用 `get_session_status` 看会话起没起来
- **CLI 选择**：`cli_tool` 缺省继承发起方（Claude 起的子会话默认 `claude`，Codex 起的默认 `codex`），显式传参优先

> 本 skill 的工具均经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。下文统一用【逻辑名】，正文不再重复 `mcp__panes__` 前缀。
> 参数 `$ARGUMENTS`：可带项目路径/关键词 + 任务描述；缺省则交互式补齐。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **长 prompt（>200 字）先写文件，prompt 只传短引用** | 一次性灌大段文本进 PTY 易触发终端"黑屏"/渲染卡死；短引用让子实例自己去读 |
| 2 | **`project_path` 必须是 `list_projects` 列出的已注册路径**，不要凭空编 | 未注册路径会被 `launch_task` 拒绝；DB 项目与工作空间项目都算合法来源 |
| 3 | **`prompt` 与 `resume_id` 互斥**，二选一 | 新任务注入 prompt，恢复走 `--resume` 不注入 prompt；同时传会冲突 |
| 4 | **不臆造任务状态查询工具**，确认一律走 `get_session_status` | XK-Pane 无 `get_task_status`；会话状态（active/exited）就是唯一可靠信号 |

---

## 2. MCP 工具

| 工具 | 用途 |
|------|------|
| `list_projects` | 列出所有已注册项目（DB 项目 + 工作空间项目，按路径去重），拿合法 `project_path` |
| `launch_task` | 启动 Claude/Codex 子实例（新任务传 `prompt`，恢复传 `resume_id`） |
| `get_session_status` | 查指定会话状态（active/exited）+ 最近输出时间，确认是否起来 |
| `list_sessions` | 列全部活跃会话及状态（无 `session_id` 时用它定位刚起的会话） |

---

## 3. 流程

### 3.1 确定目标项目

解析 `$ARGUMENTS`，提取项目路径或关键词。

若未指定项目：
1. 调 `list_projects` 拿项目列表（每项含 `path` / `name` / `source` / 可能的 `workspace`）
2. 展示给用户选择，把选中项的 `path` 作为 `project_path`

### 3.2 确定任务内容

从 `$ARGUMENTS` 提取 prompt。若未指定，询问用户要执行什么任务。

### 3.3 启动

调 `launch_task`，参数：

| 参数 | 必填 | 说明 |
|------|------|------|
| `project_path` | 是 | 目标项目路径（来自 `list_projects`） |
| `prompt` | 二选一 | 新任务的初始 prompt（与 `resume_id` 互斥） |
| `resume_id` | 二选一 | 恢复已有会话的 id（以 `--resume` 启动，不注入 prompt） |
| `cli_tool` | 否 | `claude` / `codex` / `reclaude`…；缺省继承发起方，再回落 `claude` |
| `workspace_name` | 否 | 工作空间名（用于解析 `--add-dir` 的 workspacePath） |
| `title` | 否 | 前端标签页标题 |

> **长任务描述（铁律 1）**：prompt 超过约 200 字时，先把完整描述写到 `.xkpanes/prompts/<descriptive-name>.md`，再让 prompt 只传短引用：
> `Read task from '<文件路径>' and execute it. Delete the file after reading.`

成功返回 `taskId` + `sessionId` + `status: "launching"`。

### 3.4 确认

把 `taskId` / `sessionId` 报告给用户。

可选验证：用上一步的 `sessionId` 调 `get_session_status`，看 `status` 是否 `active`、`lastOutputAt` 是否在更新；若手里没有 `sessionId`，先 `list_sessions` 定位。

---

## 4. 示例

```
/xkpane-launch-task 在 J:\Program\Agent\XK-Pane 中修复登录 bug
/xkpane-launch-task project_path=J:\app prompt="添加单元测试" cli_tool=codex
/xkpane-launch-task               # 交互式选择项目和任务
```

---

## 5. 相关文件 / 参考

```
.claude/skills/xkpane-launch-task/SKILL.md   # 本文件（被启动项目内同时落到 .codex/skills/）
.xkpanes/prompts/<name>.md                   # 长 prompt 落地处（短引用反黑屏）
```

- 监督多个并行子实例：用 `list_sessions` + `get_session_output` 轮询各会话进度
- 向已起会话补发命令/prompt：`submit_to_session`（自动处理 Enter 时序）；发控制字符用 `write_to_session`
- 终止会话：`kill_session`
