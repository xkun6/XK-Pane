---
name: xkpane-plantocodex-exec
description: XK-Pane bundled skill：把 plan md 交 Codex 经 xkpane-spec-review 评审，Claude 循环改 plan，达标后另起独立 Codex 窗口实施的 spec-gated 编排闭环。
---

# Plan Spec Review → Codex 实施

你是 Plan-Spec-Exec 编排 Agent，用 XK-Pane 的 panes MCP 调度另外两个 Codex 实例：一个**评审** plan，一个**实施** plan。流程是「把已有 plan md 送 Codex 用 `xkpane-spec-review` skill 评审 → 按反馈循环改 plan → 达标后转交另一个独立 Codex 窗口落地代码」。

- **角色边界**：Claude 编排 + 改 plan；评审 Codex 只打分；实施 Codex 只落地。三方互不越界。
- **spec gate**：未通过 rubric 阈值（≥90 + 零 Critical/Major）**不许**进入实施。
- **窗口隔离**：实施窗口必须与评审窗口物理隔离，实施者看不到自己作为评审者的内部讨论。
- **工具入口**：本 skill 引用的 launch_task / submit_to_session / get_session_status / get_session_output 等均经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。正文统一用【逻辑名】，不再重复前缀。

> 与 `xkpane-plantocodex-spec` 的**唯一**区别在最后一步：本 skill 评审通过后由**另一个 Codex** 实施；`-spec` 则由 Claude 自己实施。想跳过评审直接派发 → 用 `xkpane-plantocodex`（无评审版）。
> 参数：`$ARGUMENTS`（期望 plan md 路径，如 `docs/plans/foo.md`；为空则从对话上下文或 `AskUserQuestion` 取）。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **launch_task 的 prompt 必须是单行字符串**，禁止换行 / `\n` 字面量 | 多段 prompt 易"只发第一段"，Codex 收不到关键禁令与路径 |
| 2 | **评审 prompt 第一个 token 必须是 `[SPEC-REVIEW ONLY ...]` 标记** | 这是 xkpane-spec-review 识别"立即评审、不碰代码"的路由关键 |
| 3 | **评审阶段单 PTY 复用，每轮之间发 `/new` 清空 conversation** | 同 conversation 多轮评审会被上轮评分污染，评分趋同甚至倾向好评 |
| 4 | **`/new` 后必须等 idle 才能发新 prompt** | 否则字符可能丢失或顺序混乱 |
| 5 | **实施阶段（Phase 6）必须 launch_task 起独立窗口**，禁止 `/new` 复用评审窗口 | 实施者不应看到自己作为评审者的内部讨论 |
| 6 | **iteration 号靠 prompt 显式传递**，禁止依赖 Codex 自身记忆 | `/new` 清掉了上轮对话，Codex 不知道这是第几轮 |
| 7 | **解析不出 verdict 时禁止自己猜分数 / verdict**，甩给用户人工判定 | 防止编排器在 Codex 跑偏时虚构通过结论、误放行实施 |

---

## 2. 用到的 panes MCP 工具

| 工具 | 用途 |
|------|------|
| launch_task | 起新 Codex 窗口（评审窗 / 实施窗各一次） |
| submit_to_session | 向已有 PTY 发文本（`/new`、后续轮评审 prompt） |
| get_session_status | 轮询窗口 idle / active / exited |
| get_session_output | 拉窗口输出，供解析 verdict / 实施摘要 |
| kill_session | 收尾时关掉用完的评审窗（可选） |

> 监控用 `CronCreate`（Claude Code harness 工具，非 panes MCP）。XK-Pane 无独立的任务状态 / 启动历史查询工具，所有进度判定一律走 get_session_status + get_session_output。

---

## 3. 上下文洁净原则

**评审阶段全程复用 1 个 Codex PTY 窗口，每轮之间发 `/new`**：

- `/new` 是 Codex CLI 内置 slash 命令，清掉当前 conversation 但**保留** `developer_instructions`（XK-Pane launch 时经 `-c` 注入的系统级提示），下一条 prompt 就是干净的第 1 条 user message。
- PTY 进程不变，XK-Pane 的 sessionId 不变 —— 后续 submit_to_session / get_session_status / get_session_output 都用同一 sessionId。
- spec-review skill 靠 prompt **显式调用**（不依赖 Codex 自动 trigger），`/new` 后不影响加载。

**实施阶段（Phase 6）必须 launch_task 起独立 Codex 窗口**，与评审窗口隔离 —— 这一条不能用 `/new` 复用评审窗。

例外：用户明确要求每轮评审独立窗口（如想用 XK-Pane 标签栏看完整评审历史）时，回退到「每轮 launch_task 新窗口」模式。

---

## 4. Phase 1：确认 plan 文件

1. 从 `$ARGUMENTS` 取 plan 路径；为空则回顾对话上下文找最近提到的 plan md；都找不到：
   ```
   AskUserQuestion:
     问题: 要评审并交给 Codex 实施的 plan 文件是哪份？
     选项: 推荐若干 docs/plans/*.md 或让用户输入路径
   ```
2. 用 Read 工具确认文件存在、非空。
3. 准备好两种路径形式备用：
   - Windows：`C:\...\docs\plans\foo.md`
   - WSL：`/mnt/c/.../docs/plans/foo.md`
4. 初始化迭代计数器 `iteration = 1`（仅在对话上下文中维护，不强制写入文件）。

---

## 5. Phase 2：起评审窗口送审

### 5.1 第一次（iteration = 1）

用户选评审 Codex 环境：
```
AskUserQuestion:
  问题: 用哪个 Codex 评审 plan？
  选项:
    1. 新建本地 Codex（推荐）
    2. 新建 WSL Codex
```

调用 launch_task（**prompt 单行**，仅替换 `<plan_path>` / `<plan basename>` / `<项目路径>` 三个占位符为实际值，绝对路径优先）：

```
launch_task(
  projectPath: <项目路径>,
  cliTool: "codex",
  prompt: "[SPEC-REVIEW ONLY · DO NOT EDIT FILES · DO NOT IMPLEMENT · DO NOT RUN IMPACT ANALYSIS] 调用 xkpane-spec-review skill 评审 plan: <plan_path> | iteration 1/3 | single-doc mode (skip cross-doc checks) | 评审通过后会有另一个 Codex 窗口实施，findings 请写得让实施者能直接落地",
  title: "Review #1: <plan basename>"
)
```

**自检 3 件事**：
1. prompt 是**一行**（无真换行符、无 `\n` 字面量）
2. `<plan_path>` 已替换为真实绝对路径（`J:\...\foo.md` 或 `/mnt/c/.../foo.md`），不是占位符字面量也不是仅文件名
3. 第一个 token 是 `[SPEC-REVIEW ONLY ...]` 标记

记录返回的 `sessionId` 为 `current_review_session`。

### 5.2 后续轮次（iteration ≥ 2）

**复用 `current_review_session`，三步走**：

第 1 步 —— 发 `/new` 清掉上一轮 conversation：
```
submit_to_session(
  sessionId: <current_review_session>,
  text: "/new"
)
```

第 2 步 —— 等 Codex 回到 idle（`/new` 处理完、新 conversation 就绪），轮询：
```
循环最多 10 次（每次间隔 ~2 秒）：
  status = get_session_status(sessionId: <current_review_session>)
  if status == "idle": break
  否则继续等
```

第 3 步 —— 发新一轮评审 prompt（**同样单行、同样以 `[SPEC-REVIEW ONLY ...]` 开头**，仅替换 `<plan_path>` 和 `<n>`）：
```
submit_to_session(
  sessionId: <current_review_session>,
  text: "[SPEC-REVIEW ONLY · DO NOT EDIT FILES · DO NOT IMPLEMENT · DO NOT RUN IMPACT ANALYSIS] 调用 xkpane-spec-review skill 评审 plan: <plan_path> | iteration <n>/3 | single-doc mode | history 已通过 /new 清空，请在干净上下文重新评分；前 <n-1> 轮 findings 已写入 plan 末尾的 Review History"
)
```

`current_review_session` 不变，PTY 始终是同一个。

---

## 6. Phase 3：监控评审

```
CronCreate:
  cron: "*/1 * * * *"
  recurring: true
  prompt: |
    检查评审会话:
    1. get_session_status(sessionId: "<current_review_session>")
    2. active → 静默等
    3. idle/exited:
       - get_session_output(sessionId, lines: 300)
       - 进 Phase 4 解析
       - 删除本 cron
```

或者告诉用户「评审 #N 进行中，可切到 Review 标签实时看，我会自动接管结果」。

---

## 7. Phase 4：解析 Verdict

从 output 抓三个信号：

| 信号 | 提取方式 |
|------|----------|
| 总分 | 匹配 `**Total: XX / 100**`，取 XX |
| Verdict | 匹配 `Decision: **GO**` 或 `Decision: **NO-GO**` |
| 严重 finding 数 | 统计 `[Critical]` 和 `[Major]` 出现次数 |

**通过条件（全部满足）**：
1. `Total >= 90`
2. `Decision == GO`
3. `[Critical] == 0` 且 `[Major] == 0`（即使 Codex 给了 GO，只要 Findings 里还有 critical/major，按 NO-GO 处理 —— 防 Codex 自评漏判）

满足 → 跳 Phase 6；不满足 → 进 Phase 5。

**解析失败兜底**：output 里找不到 `Total:` 或 `Decision:` 行（Codex 跑偏没按 skill 格式输出），把完整 output 甩给用户人工判定，**不要自己猜分数或 verdict**（铁律 7）。

---

## 8. Phase 5：迭代 plan

1. 整理本轮 Findings（特别是 Critical / Major），打成结构化清单。
2. **由你（Claude）按 Fix 建议直接修改 plan 文件**：
   - 每改一处对应一条 finding
   - 在文件末尾追加（或更新）一段 `## Review History`，记录本轮迭代号、被采纳的 finding、改了哪几节
3. 一句话向用户通报本轮变更（无需审批，例如「已根据 2 critical / 3 major 修改 plan，准备第 <n+1> 轮评审」）。
4. `iteration += 1`，检查：
   - `iteration <= 3` → 回 **Phase 2 的「后续轮次」分支**（复用同 PTY，先发 `/new` 再发新评审 prompt）
   - `iteration > 3` → 进入下方用户兜底

### 8.1 三轮兜底

```
AskUserQuestion:
  问题: 评审 3 轮仍未达到 ≥90 + 零 critical/major。怎么办？
  选项:
    1. 追加 1 轮评审（不推荐，已接近死循环）
    2. 我自己看 Codex 反馈手动改 plan（暂停工作流，我改完通知你）
    3. 跳过评审，按当前 plan 直接交给 Codex 实施（接受风险）
    4. 放弃，归档 plan
```

附上最新一轮的完整 verdict + findings 让用户决策。

---

## 9. Phase 6：起独立 Codex 窗口实施

评审通过后：

1. 通报用户：「评审通过（第 N 轮，Total: XX/100），起 Codex 实施窗口」。
2. 用户选实施环境（默认与评审窗口同环境）：
   ```
   AskUserQuestion:
     问题: 用哪个 Codex 实施？
     选项:
       1. 新建本地 Codex（推荐，与评审窗口隔离）
       2. 新建 WSL Codex
   ```
3. 起独立 Codex 窗口（这里 prompt 不需要 `[SPEC-REVIEW ONLY]` 标记，因为是实施而非评审）：
   ```
   launch_task(
     projectPath: <项目路径>,
     cliTool: "codex",
     prompt: "请阅读以下 plan 文件并按方案实现代码。plan 已经过 xkpane-spec-review skill 审核通过（≥90 分，零 critical/major），可放心按方案落地。完成所有步骤后汇报：改了哪些文件、是否有测试、有无未完成项。\n\nPlan 文件: <plan_path>",
     title: "Exec: <plan basename>"
   )
   ```
   记录 sessionId 为 `exec_session`。
4. 启动监控：
   ```
   CronCreate:
     cron: "*/1 * * * *"
     recurring: true
     prompt: |
       1. get_session_status(sessionId: "<exec_session>")
       2. idle/exited:
          - get_session_output(sessionId, lines: 500)
          - 跑 git diff --stat
          - 汇报给用户
          - 删除本 cron
       3. active: 静默等待
   ```
5. 实施完成后汇总给用户：
   - Codex 输出摘要（关键决策、踩到的坑）
   - 代码变更摘要（`git diff --stat` + 主要修改点）
   - 是否有错误或未完成
   - 建议下一步（test / review / commit / 继续迭代）
6. （可选）评审窗口用完即弃，可 `kill_session(sessionId: <current_review_session>)` 释放资源。

---

## 10. 退出条件总结

| 出口 | 条件 |
|------|------|
| **进入 Codex 实施** | 某轮 Total ≥ 90 + GO + 0 Critical/Major |
| **暂停问用户** | 3 轮未通过 |
| **解析失败** | 任一轮 output 无法解析 verdict |
| **用户放弃** | Phase 5 兜底选「放弃」 |

---

## 11. 注意事项

- **评审阶段单 PTY 复用 + `/new`** —— 资源 + 洁净两头都要。不要因为「省一次发命令」就 submit_to_session 直接发新评审 prompt 跳过 `/new`。
- **`/new` 后必须等 idle 才能发新 prompt** —— 否则字符可能丢失或顺序混乱。最长等 ~20 秒，超时则告诉用户「Codex 处理 /new 超时，建议手动检查窗口」。
- **实施窗口与评审窗口隔离** —— Phase 6 必须 launch_task 新窗口，不能 `/new` 复用评审窗口。
- **Claude 改 plan，Codex 评审 / 实施** —— 三个角色边界清楚。
- **iteration 号必须通过 prompt 显式传给 spec-review** —— `/new` 清掉了上轮对话，全靠 prompt 告知。
- **WSL 路径转换** —— `C:\Users\xxx\docs\plans\foo.md` → `/mnt/c/Users/xxx/docs/plans/foo.md`。
- **超时处理** —— Codex 评审一般 1-3 分钟；实施时间不定，超过 30 分钟仍 active 提醒用户检查 Exec 窗口。
- **评审窗本就用完即弃，不要 resume** —— 多次 `/new` 后窗口内部最新对话已不在 launch 时那条 session 记录里，resume 可能拿到旧对话；要继续评审就走 submit_to_session 复用同 sessionId，不要 resume。
- **想跳过评审直接派发** —— 用 `xkpane-plantocodex` skill（无评审版），不是这个。

---

## 12. 相关文件 / 参考

```
.claude/skills/  与  .codex/skills/  下的同族 skill（XK-Pane 启动项目时同时落地）：
├── xkpane-plantocodex-exec/    # ← 本 skill：评审通过 → 另一个 Codex 实施
├── xkpane-plantocodex-spec/    # 评审通过 → Claude 自己实施（唯一区别在末步）
├── xkpane-plantocodex/         # 无评审版：plan 直接派发 Codex
├── xkpane-spec-review/         # 被本 skill 调用的 rubric 评审器（GO/NO-GO + Total + Findings）
└── xkpane-spec/                # 双文档 spec（requirements + architecture）工作流
```

交叉引用：`/xkpane-spec-review`（评审 rubric 与输出格式）· `/xkpane-plantocodex-spec`（Claude 实施版）· `/xkpane-plantocodex`（无评审直派版）。
