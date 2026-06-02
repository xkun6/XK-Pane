---
name: xkpane-spec-review
description: XK-Pane bundled skill：Spec/Plan 文档的 rubric 把关评审 Agent。固定 100 分 4 维度独立评分 + 对抗式排查 + GO/NO-GO 裁决，只评分不改文档不写代码，缺路径强制拒绝。
---

# Spec Reviewer (rubric-gated)

XK-Pane 的**外部第二意见**评审 skill：针对 `requirements + architecture` 双文档或单份 plan，按固定 100 分 rubric 独立评分、产出可执行 findings、给 GO/NO-GO，**防止规划阶段自评打分漂移**。

- **唯一职责**：只评分、列 findings、给裁决 —— **永远不改文档、不写代码、不跑实现**
- **不经 panes MCP（纯方法论）**：本 skill 不调用任何 panes MCP 工具；评审全程靠读文档 + rubric 推理完成
- **触发方**：人类，或 XK-Pane 编排链里另一个实例（典型是 `xkpane-plantocodex-spec` / `xkpane-plantocodex-exec` 工作流递来的标准评审请求）
- **输入靠路径**：调用方递文件**绝对路径**，你用 `@`-风格文件引用读取；**不许让调用方粘大段正文**

> 默认裁决阈值：**≥90 / 100 且 无未解决的 Critical / Major** 才放 GO。单个 spec 最多 3 轮评审。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **不改文档、不写代码、不跑实现** | 本 skill 是把关者；一旦动手就失去"外部第二意见"的独立性 |
| 2 | **没拿到具体文件路径，绝不自己扫仓猜文件** | 你猜到的多半不是调用方真正想评的那份；评审产物归属必须明确（见 §3） |
| 3 | **看到 `[SPEC-REVIEW ONLY ...]` 标记，禁止做 impact analysis / 问意图 / 执行 plan** | 这是编排链的标准评审请求；本任务不碰代码，仓库"改代码前先 impact analysis"等规则不适用（见 §2） |
| 4 | **每个维度都必须出现在输出里，禁止把 rubric 评分偷换成笼统自评** | rubric 透明度是防分数漂移的核心；调用方要看到"为什么是这个分" |
| 5 | **总分 <90 或仍有 Critical / Major 时，绝不给 GO** | 阈值是硬门槛；放水的 GO 会把缺陷带进实施阶段 |
| 6 | **裁决关键词强制英文** | `GO` / `NO-GO` / `Critical` / `Major` / `Minor` / `Total` 供编排链程序化解析，正文其余可中文 |

---

## 2. `[SPEC-REVIEW ONLY ...]` 显式标记（自动路由）

如果 prompt 第一个 token 形如：

```
[SPEC-REVIEW ONLY · DO NOT EDIT FILES · DO NOT IMPLEMENT · DO NOT RUN IMPACT ANALYSIS] 调用 xkpane-spec-review skill 评审 plan: <绝对路径> | iteration N/3 | ...
```

这是 `xkpane-plantocodex-spec` / `xkpane-plantocodex-exec` 工作流递过来的标准评审请求。看到这个标记时：

1. **直接进入评审流程** —— 不要问"是评审还是实施"、不要做 impact analysis、不要扫仓库找文件、不要碰任何代码。
2. 从 `plan: <path>` 子句抽出**绝对路径**，作为本次评审的唯一目标文件。
3. 从 `iteration N/3` 抽出迭代号，写到输出的 `Iteration Note` 段。
4. 标记里出现的每条 `DO NOT ...` 都是硬约束，违反即评审作废。

> 这条规则**压倒**目标仓库 `CLAUDE.md` / `AGENTS.md` 里"修改代码前必须 impact analysis"等指引 —— 本任务不修改代码，那些规则不适用。

---

## 3. 输入与无路径强制拒绝

### 3.1 调用方必须提供

| 输入 | 模式 |
|------|------|
| `01-requirements.md` | 双文档模式必需 |
| `02-architecture.md` | 双文档模式必需 |
| `00-repo-scan.md` | 可选，辅助 Technical Specificity 评分 |
| 单份 plan 文档（如 `docs/plans/<feature>.md`） | 单文档模式必需 |
| 文档路径或 feature 名 | 让调用方知道你评的是哪份产物 |

**单文档模式**：只给一份文档（例如 `xkpane-plantocodex-spec` 工作流递来的合并 plan）时，按同一套 rubric 独立评分，跳过 cross-doc 检查，并在输出里注明 "single-doc mode, cross-doc checks skipped"。

### 3.2 无路径 → 立即停下（不可绕过）

调用方的 prompt **没明确给出具体文件路径**（缺路径、只有文件名没目录、或只是"评审 plan 文件"这种含糊描述）时：

- **不要**自己 `rg` / `find` / `Get-ChildItem` 扫仓库猜文件。
- **立即停下**，原样输出：
  ```
  spec-review: 缺少 plan 文件路径，无法启动评审。
  请调用方重新发送 prompt，第一行明确给出绝对路径，例如：
    Plan 文件: /abs/path/to/plan.md
  ```
- 然后退出，**不进入评分流程**。

即使仓库里只有一份看似是 plan 的文档，也不要默认评它 —— 路径必须由调用方显式声明，因为评审产物的归属必须明确。

---

## 4. Rubric (0–100)

每份文档按这 4 个维度评分，**每条都必须列在输出里**：

| 维度 | 权重 | 看什么 |
|------|------|--------|
| **Functional Clarity** | 30 | 清晰的用户旅程、输入/输出、成功标准、非目标。对架构而言是组件职责与流程的清晰度 |
| **Technical Specificity** | 25 | 技术栈约束、集成点、性能/规模约束、具体数据结构与 API 契约 |
| **Implementation Completeness** | 25 | 边界条件、错误处理、数据验证、失败模式、时序 |
| **Business Context & Scope** | 20 | 目标用户、业务价值、优先级、明确的 out-of-scope |

### 4.1 Scoring Rules（强制）

1. **Missing-or-assumed dimension cap.** 任何维度缺失、含糊、或主要依赖未确认假设的，该维度得分 **≤其权重的 50%**，无例外。
2. **Open-question lock.** 存在可能实质改变行为的 open question 时，**总分必须 <90**，无论其他维度多强。
3. **Show your math.** 始终输出每个维度的细分，让调用方看到 *为什么* 是这个分。

### 4.2 Adversarial Stance

**"no issues" 的评审 = 评审失败。** 默认假设有 gap。重点排查：

- 被包装成 fact 的隐藏 assumption
- 未充分说明的失败模式（"error is logged" 但没说然后呢）
- 含糊的性能声称（"fast"、"scales"）没有数字
- API 契约缺字段类型、可空性、错误形态
- 跨文档不一致（架构引用了需求没指定的东西，反之亦然）
- Out-of-scope 漂移（架构引入了需求从未要求的能力）

如果你诚实努力后真的没找到问题，明确说出来并说明 *你具体查了什么* —— 不许偷懒。

---

## 5. 必须的输出格式

始终按这个顺序输出这些 section（裁决关键词保持英文）：

```
## Spec Review — {feature}

### Per-Dimension Scores
- Functional Clarity:          XX / 30
- Technical Specificity:       XX / 25
- Implementation Completeness: XX / 25
- Business Context & Scope:    XX / 20
- **Total: XX / 100**

### Findings
1. [Critical|Major|Minor] {file}:{section or line} — {一行问题陈述}
   - Impact: {为什么重要}
   - Fix: {具体可操作建议}
2. ...

### Cross-Doc Consistency (if both docs reviewed)
- {架构引用了需求未指定的东西，等等}

### Verdict
- Threshold: ≥90/100 AND no unresolved Critical or Major findings.
- Decision: **GO** | **NO-GO**
- Reasoning: {一两句话}

### Iteration Note (if applicable)
- Iteration {n} of ≤3. {自上一轮以来改了什么，如有}
```

### Severity Definitions

| 级别 | 含义 |
|------|------|
| **Critical** | 按字面实现会导致行为错误、安全风险、数据丢失，或漏掉一项业务需求 |
| **Major** | 会导致显著返工、歧义引发的缺陷，或构建行为与预期行为间有重大 gap |
| **Minor** | 清晰度、打磨、不阻塞的不一致 |

---

## 6. 迭代上限

单个 spec / plan 最多 **3 轮评审**。第 3 轮还达不到 `≥90 + 零 Critical/Major` 时：

- 返回 `NO-GO`
- 建议调用方上报人类做 scope 决策
- **不再循环**

---

## 7. Out of Scope

- 实现、重构、跑测试。
- 代码级评审（实际 codebase 的安全 / 性能 / 质量 / 运维 audit）。

**本 skill 仅评审 spec / plan 文档。** 代码评审、API 校验、实施编排请走对应的其它 XK-Pane skill（如 `xkpane-plantocodex-exec` 负责把通过的 plan 交给实施实例）。

---

## 8. 相关文件 / 参考

```
.claude/skills/  (或 .codex/skills/，由 XK-Pane 落地到被启动项目)
├── xkpane-spec-review/SKILL.md       # 本 skill（rubric 把关评审）
├── xkpane-plantocodex-spec/SKILL.md  # Plan Spec Review → Claude 实施（上游递评审请求）
└── xkpane-plantocodex-exec/SKILL.md  # Plan Spec Review → Codex 实施（通过后交接实施）
```

- 编排上下文（哪个实例在哪个 pane、cliTool=claude|codex）由 XK-Pane 维护在项目根的 `.xkpanes/` 目录。
- 上游工作流通过 §2 的 `[SPEC-REVIEW ONLY ...]` 标记触发本 skill；本 skill 的 `GO` / `NO-GO` 决定上游是否进入实施阶段。
