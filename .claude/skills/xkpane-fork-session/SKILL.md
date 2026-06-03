---
name: xkpane-fork-session
description: XK-Pane 分叉对话:总结上下文,经 panes MCP 在同项目启动新 Claude/Codex 实例继承上下文走分支。铁律是"写文件 .xkpanes/prompts/ + prompt 只放短引用",长 prompt 直塞会令终端黑屏。
---

# XK-Pane 分叉对话

把当前对话的上下文压成摘要,经 panes MCP 在**同一项目**里启动一个新的 Claude/Codex 实例,继承上下文后走另一条分支方向（试不同实现、并行拆子任务等）。

- **核心模式**：**写文件 + 短引用**——上下文写进项目里的 `.xkpanes/prompts/fork-<timestamp>.md`,启动指令只放一句"读这个文件并执行"。
- **工具入口**：经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`,codex 内同名）。正文统一用【逻辑名】（如 launch_task / get_session_status）。
- **分叉方向**：从 `$ARGUMENTS` 取;未指定则向用户确认新实例该做什么。

> 前置：目标项目已在 XK-Pane 中登记（可用 list_projects 确认 projectPath）;新实例的 cliTool 默认 `claude`,需要 codex 实例时显式传 `codex`。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **绝不把完整上下文直接塞进 launch_task 的 `prompt` 参数** | 超长 prompt 写入新终端会导致**黑屏 / 卡死**;必须走"写文件 + 短引用" |
| 2 | **上下文一律写进 `.xkpanes/prompts/fork-<timestamp>.md`** | 项目根已有 `.xkpanes/` 暂存目录;集中存放便于追踪与清理,`prompts/` 不存在时随手创建 |
| 3 | **`prompt` 只放一句短引用 + 让新实例读后自删** | 文件是一次性交接物;读完即删,避免 `.xkpanes/prompts/` 堆积 |
| 4 | **文件路径必须用绝对路径** | 新实例的工作目录未必与父实例一致,相对路径会读不到 |
| 5 | **方向不明时先问,不要自行臆测分支** | 分叉一旦启动就独立演化,方向错了等于白跑一个实例 |

---

## 2. MCP 工具

| 工具 | 用途 |
|------|------|
| `list_projects` | 确认/解析目标项目的 projectPath（可选,projectPath 不确定时用） |
| `launch_task` | 在指定项目启动新 Claude/Codex 实例 |
| `get_session_status` | 启动后确认新实例已就绪 |
| `get_session_output` | 需要时回看新实例首屏输出,核对它确实读到了任务文件 |

---

## 3. 流程

### 3.1 总结当前上下文

回顾当前对话的关键信息,压成一段结构化摘要：

- 正在处理的文件 / 模块
- 已完成的修改
- 当前面临的问题或决策点
- 关键的代码片段与架构决策

### 3.2 确定分叉方向

从 `$ARGUMENTS` 提取新实例要探索的方向。**若未指定,先问用户**（如"试另一种实现方案" / "并行处理前端,父实例继续后端"）。

### 3.3 写入任务文件

用 Write 工具把摘要 + 方向写入 `<projectPath>/.xkpanes/prompts/fork-<timestamp>.md`（`<timestamp>` 用当前时间戳,保证唯一）：

```md
# Fork 任务

## 上下文继承

以下是从父对话继承的上下文：

{上下文摘要}

## 你的任务

{分叉方向 / 具体任务描述}
```

### 3.4 启动新实例

调用 launch_task：

| 参数 | 值 |
|------|-----|
| `projectPath` | 目标项目绝对路径（环境推断,或 list_projects 解析,或问用户） |
| `cliTool` | `claude`（默认）或 `codex` |
| `prompt` | **短引用指令**,见下 |
| `title`（可选） | 描述分叉方向的标签 |

`prompt` 固定写法（注意：是短引用,不是上下文本身）：

```
Read the task description from '<任务文件绝对路径>' and execute it. Delete the file after reading.
```

### 3.5 确认就绪

调 get_session_status 确认新实例已启动;必要时用 get_session_output 回看首屏,核对它确实读到了任务文件（而非黑屏 / 报错）。

### 3.6 报告

向用户汇报：

- 新实例的 sessionId
- 继承的上下文摘要（一两句概括）
- 分叉方向说明

---

## 4. 示例

```
/xkpane-fork-session 尝试用另一种算法实现排序
/xkpane-fork-session 在新实例中处理前端部分,我继续处理后端
/xkpane-fork-session    # 不带参数 → 交互式确定分叉方向
```

---

## 5. 反例（这样写会黑屏 / 失败）

❌ 把整段上下文塞进 `prompt`：

```
launch_task(prompt="这是从父对话继承的全部上下文……（几千字）……请基于此实现 X")
```

✅ 上下文写文件,`prompt` 只放短引用：

```
launch_task(prompt="Read the task description from 'J:/.../.xkpanes/prompts/fork-1717310400.md' and execute it. Delete the file after reading.")
```

---

❌ 文件路径用相对路径：

```
Read the task description from './.xkpanes/prompts/fork-xxx.md' and execute it.
```

✅ 用绝对路径（新实例工作目录可能不同）：

```
Read the task description from 'J:/Program/Agent/XK-Pane/.xkpanes/prompts/fork-xxx.md' and execute it.
```

---

## 6. 相关文件 / 参考

```
<projectPath>/.xkpanes/
└── prompts/
    └── fork-<timestamp>.md   # 一次性交接物,新实例读后自删

.claude/skills/xkpane-fork-session/SKILL.md   # 本文件（同时落到 .codex/skills/）
```

- 同项目并行多实例的监督 → 见 launch_task / get_session_status / get_session_output 配套用法
- 健康自检 → health
