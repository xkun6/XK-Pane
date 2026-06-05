---
name: xkpane-plantocodex-spec
description: XK-Pane 编排原语：把已有 plan md 交 Codex 用 xkpane-spec-review 评审，按 Verdict 程序化放行（Total≥90+GO+0 Critical/Major）循环改，单 PTY 复用、每轮 /new 清 history，过审后 Claude 自实施。
---

# Plan Spec Review → Claude 实施

你是 Plan-Spec-Review 编排 Agent：把一份已有的 plan markdown 交给 Codex 用 `xkpane-spec-review` skill 评审，按反馈循环修改，**通过后由 Claude（你自己）实施**。

- **角色边界**：Codex 只评审，Claude 改 plan + 最终实施；不许 Claude 自评代替 Codex。
- **洁净复用**：全程复用 1 个 Codex PTY，每轮之间发 `/new` 清空 conversation history。
- **程序化放行**：Verdict 必须按固定格式解析（`Total ≥ 90` + `Decision: GO` + 0 Critical/Major），不靠"感觉过了"。
- **3 轮兜底**：评审最多 3 轮，仍不达标转人工决策。
- **解析失败甩人工**：output 不含可解析的 `Total:` / `Decision:` 行时，原文交用户判定，**不要自己猜分数**。

> 全部会话操作经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。正文统一用【逻辑名】：launch_task / submit_to_session / get_session_status / get_session_output / list_sessions / kill_session。
> 参数 `$ARGUMENTS` 期望是 plan md 路径（如 `docs/plans/foo.md` 或 `.xkpanes/plans/foo.md`）；为空则从对话上下文或向用户询问获取。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **单 PTY 复用 + 每轮 `/new` 清 history**，禁止跳过 `/new` 直接发下一轮评审 | 同 conversation 多轮评审会被上一轮评分 / findings / 写作风格污染，评分趋同甚至倾向好评；`/new` 清当前对话但保留 launch 时注入的 developer_instructions |
| 2 | **`/new` 后必须等输出停稳再发新 prompt**，不许立刻 submit | XK-Pane 活跃会话状态恒 `active`（无 idle 态），抢发会丢字符 / 乱序 |
| 3 | **Verdict 走程序化解析放行**，禁止凭印象判 GO | 防 Codex 自评漏判：即便给了 GO，只要还剩 Critical/Major 也按 NO-GO |
| 4 | **iteration 号必须通过 prompt 显式传给 spec-review** | `/new` 清掉了上轮对话，Codex 不知道这是第几轮，全靠 prompt 告知 |
| 5 | **prompt 必须单行 + 以 `[SPEC-REVIEW ONLY ...]` 开头**，禁止多段 / `\n` | 多段 prompt 易"只发第一段"；首 token 标记是 spec-review skill 识别"立即评审"的路由信号 |
| 6 | **Claude 改 plan、Codex 只评审；review 窗口用完即弃不 resume** | 避免"评审者顺手实施"角色混乱；多次 `/new` 后最新对话落在不同 jsonl，resume 易拿到旧上下文 |

---

## 2. 用到的 panes MCP 工具

| 工具 | 用途 |
|------|------|
| launch_task | 第 1 轮起一个 Codex PTY 送审（传 `prompt`，返回 `sessionId`） |
| submit_to_session | 发 `/new` 与后续每轮评审 prompt（自动处理 Enter 时序） |
| get_session_status | 查会话 `active` / `exited`（判断评审会话是否还活着 / 已退出） |
| get_session_output | 读最近输出：判断输出停稳、抓 Verdict 文本 |
| list_sessions | 需要时枚举活跃会话，核对 review 会话仍在 |
| kill_session | 收尾时终止 review PTY（可选） |

> 注：XK-Pane 的 get_session_status 对活跃会话恒返回 `active`（不推断 idle）。"Codex 处理完了吗"一律靠 **get_session_output 输出停稳**判断，不要等 idle。

---

## 3. Phase 1：确认 plan 文件

1. 从 `$ARGUMENTS` 取 plan 路径；为空则回顾对话上下文找最近提到的 plan md；都没有就问用户要哪份（推荐 `docs/plans/*.md` 或 `.xkpanes/plans/*.md`）。
2. 用 Read 确认文件存在、非空。
3. 备好 plan 的 Windows 绝对路径（送审 prompt 用，如 `C:\...\docs\plans\foo.md`）。
4. 初始化 `iteration = 1`（仅在对话上下文维护，不强制落文件）。

---

## 4. Phase 2：起 / 复用 Codex 窗口送审

### 第一次（iteration = 1）

调用 launch_task：

```
launch_task(
  project_path: <项目路径>,        # 必须已注册（list_projects 可查）
  cli_tool: "codex",
  prompt: "[SPEC-REVIEW ONLY · DO NOT EDIT FILES · DO NOT IMPLEMENT · DO NOT RUN IMPACT ANALYSIS] 调用 xkpane-spec-review skill 评审 plan: <plan_path> | iteration 1/3 | single-doc mode (skip cross-doc checks) | 按 4 维度 rubric 评分并输出完整 verdict 后停下",
  title: "Review #1: <plan basename>"
)
```

**发前自检 3 件事**：
1. prompt 是**一行**（无真换行、无 `\n` 字面量）。
2. `<plan_path>` 已换成真实绝对路径（如 `J:\...\foo.md`），不是占位符、不是仅文件名。
3. 第一个 token 是 `[SPEC-REVIEW ONLY ...]`。

记下返回的 `sessionId` 为 `current_review_session`。

### 后续轮次（iteration ≥ 2）—— 复用同一 PTY，三步走

**第 1 步**　发 `/new` 清上一轮对话：
```
submit_to_session(session_id: <current_review_session>, text: "/new")
```

**第 2 步**　等输出停稳（XK 无 idle 态，靠输出不再变化判断就绪）：
```
循环最多 ~10 次（每次间隔 ~2 秒）：
  out = get_session_output(session_id: <current_review_session>, lines: 40)
  比较本次 content 与上次：
    若连续 2 次内容无新增（或 lastOutputAt 不再前进）→ 视为就绪，break
  超时（~20 秒）仍在刷 → 告诉用户「Codex 处理 /new 超时，建议手动检查 Review 窗口」并暂停
```

**第 3 步**　发新一轮评审 prompt（同样单行、同样以 `[SPEC-REVIEW ONLY ...]` 开头，仅替换 `<plan_path>` 与 `<n>`）：
```
submit_to_session(
  session_id: <current_review_session>,
  text: "[SPEC-REVIEW ONLY · DO NOT EDIT FILES · DO NOT IMPLEMENT · DO NOT RUN IMPACT ANALYSIS] 调用 xkpane-spec-review skill 评审 plan: <plan_path> | iteration <n>/3 | single-doc mode | history 已通过 /new 清空，请在干净上下文重新评分；前 <n-1> 轮 findings 已写入 plan 末尾的 Review History"
)
```

`current_review_session` 与 PTY 进程始终不变。

> 例外：用户明确要每轮独立窗口（想用 XK-Pane 标签栏看完整评审历史）时，回退到「每轮 launch_task 新窗口」模式，不复用、不发 `/new`。

---

## 5. Phase 3：监控评审进度

XK-Pane 无后台定时巡检工具，由你主动轮询：

```
循环（间隔 ~30-60 秒）：
  st = get_session_status(session_id: <current_review_session>)
  active → 继续等
  exited → get_session_output(session_id, lines: 300)，进 Phase 4 解析
```

或直接告诉用户「评审 #N 进行中，可切到 Review 标签实时看，我会主动接管结果」。评审一般 1-3 分钟；超过 10 分钟仍 `active` 提醒用户检查 Review 窗口。需要确认会话还在时用 list_sessions 核对。

---

## 6. Phase 4：解析 Verdict

`get_session_output` 拿到 output 后，抓三个信号：

| 信号 | 提取方式 |
|------|----------|
| 总分 | 匹配 `**Total: XX / 100**`，取 XX |
| Verdict | 匹配 `Decision: **GO**` 或 `Decision: **NO-GO**` |
| 严重 finding 数 | 统计 `[Critical]` 与 `[Major]` 出现次数 |

**通过条件（全部满足）**：
1. `Total >= 90`
2. `Decision == GO`
3. `[Critical] == 0` 且 `[Major] == 0`（即便 Codex 给了 GO，只要 Findings 里还有 Critical/Major，按 NO-GO 处理）

满足 → 跳 Phase 6；不满足 → 进 Phase 5。

**解析失败兜底**：output 里找不到 `Total:` 或 `Decision:` 行（Codex 跑偏没按 skill 格式输出），把完整 output 原样甩给用户人工判定，**不要自己猜分数或 verdict**。

---

## 7. Phase 5：迭代 plan

1. 整理本轮 Findings（特别是 Critical / Major），打成结构化清单。
2. **由你（Claude）按 Fix 建议直接改 plan 文件**：
   - 每改一处对应一条 finding。
   - 在文件末尾追加 / 更新一段 `## Review History`，记录本轮迭代号、被采纳的 finding、改了哪几节。
3. 一句话向用户通报本轮变更（无需审批，例：「已据 2 critical / 3 major 改 plan，准备第 <n+1> 轮评审」）。
4. `iteration += 1`，检查：
   - `iteration <= 3` → 回 **Phase 2「后续轮次」分支**（复用同 PTY：先 `/new`、等输出停稳、再发新评审 prompt）。
   - `iteration > 3` → 进入下方 3 轮兜底。

### 3 轮兜底

附上最新一轮完整 verdict + findings，问用户怎么办（任选其一）：
1. 追加 1 轮评审（不推荐，已接近死循环）。
2. 用户自己看 Codex 反馈手动改 plan（暂停工作流，改完通知你）。
3. 跳过评审，按当前 plan 直接进入实施（接受风险）。
4. 放弃，归档 plan。

---

## 8. Phase 6：Claude 实施

评审通过后：

1. 通报：「评审通过（第 N 轮，Total: XX/100），开始按 plan 实施」。
2. 退出本工作流框架，回到正常编辑流。
3. 推荐链路：把 plan 当输入交给 `implement` skill，或直接按 plan 章节顺序逐步实现；用你自身的待办机制跟踪进度。
4. 实施中若发现 plan 与现实有偏差，**先停下来更新 plan**，必要时回 Phase 2 重审，避免边写边偏。
5. 完成后通报：实施的文件清单 + `git diff --stat` + 建议下一步（test / review / commit）。
6. 收尾可用 kill_session 关掉 review PTY（用完即弃，不要 resume）。

---

## 9. 退出条件总结

| 出口 | 条件 |
|------|------|
| **进入 Claude 实施** | 某轮 `Total ≥ 90` + GO + 0 Critical/Major |
| **暂停问用户** | 3 轮未通过 |
| **解析失败** | 任一轮 output 无法解析 verdict |
| **用户放弃** | Phase 5 兜底选「放弃」 |

---

## 10. 注意事项

- **不要绕过 spec-review** —— 不许 Claude 自评代替 Codex，否则退化成「Claude 自己拍板」。
- **不要省 `/new`** —— 哪怕只为少发一条命令，跳过 `/new` 直接发新评审会污染评分。
- **判就绪靠输出停稳，不靠 idle** —— XK-Pane 活跃会话状态恒 `active`，等 idle 会永远等不到。
- **review 窗口用完即弃** —— 多次 `/new` 后最新对话在不同 jsonl，resume 可能拿到旧上下文。

---

## 11. 相关文件 / 参考

```
.claude/skills/  ·  .codex/skills/    # 本 skill 在被 XK-Pane 启动的项目里同时落到这两处
├── xkpane-plantocodex-spec/SKILL.md  # 本 skill（评审通过 → Claude 实施）
└── xkpane-spec-review/SKILL.md       # Codex 端 rubric 评审 skill（本 skill 显式调用它）

<project>/.xkpanes/plans/             # plan md 常用归档落点（XK-Pane 本地，不回连）
<project>/docs/plans/                 # plan md 另一常见存放处
```

交叉引用：评审 rubric 与 verdict 格式见 `/xkpane-spec-review`；实施阶段可接 `/implement`。
