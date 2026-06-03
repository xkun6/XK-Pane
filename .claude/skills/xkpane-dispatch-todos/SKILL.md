---
name: xkpane-dispatch-todos
description: Todo 驱动的批量分派编排：query_todos 取未完成待办 → 按 Todo 生成 prompt → update_todo 标 in_progress → launch_task 起 Claude/Codex 实例执行 → create_task_binding 落库跟踪。含 list/create/done 子命令与长 prompt 写文件反黑屏铁律。
---

# Todo 驱动

查询未完成的 Todo 待办，按 Todo 生成 prompt，**批量启动 Claude/Codex 实例执行**，并把每次分派落库为 TaskBinding 跟踪进度。这是把「待办清单」直接变成「并行执行」的编排闭环。

- **取待办**：`query_todos`（按 `status="todo"` / `priority` / `scope` / `scopeRef` 过滤）拉未完成项
- **标在途**：分派前 `update_todo` 把该 Todo 状态改 `in_progress`，避免重复派发
- **起实例**：`launch_task` 在目标项目里起子实例，prompt 携带 Todo 内容
- **落跟踪**：`create_task_binding` 记录一次分派（title/projectPath + sessionId），后续 `update_task_binding` 推进度
- **收口**：`get_session_status` 探活；完成后 `update_todo` 标 `done`

> 所有工具经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。下文统一用【逻辑名】（如 `query_todos` / `launch_task`），正文不再重复 `mcp__panes__` 前缀。
> 参数 `$ARGUMENTS`：可带子命令（`list` / `dispatch` / `create` / `done`）+ 过滤条件（如 `--priority high`）；无参默认走完整「查询并分派」。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | **分派前先 `update_todo` 标 `in_progress`，再 `launch_task`** | 顺序反了会出现「起了实例但 Todo 仍是 todo」的窗口，重跑本 skill 会重复派发同一条 |
| 2 | **长 prompt（>200 字）先写 `.xkpanes/prompts/<todo-id>.md`，prompt 只传短引用** | Todo 描述 + 要求拼起来常超长；直接灌 PTY 易触发终端黑屏/渲染卡死 |
| 3 | **`project_path` 必须来自 `list_projects`**，不要从 Todo 的 `scopeRef` 直接当路径用 | `scopeRef` 可能是工作空间名而非项目路径，也可能指向未注册项目；必须经 `list_projects` 校验 |
| 4 | **同一项目内并行分派的多条 Todo 禁止改同一文件** | 多实例同时写同一工作区会互相覆盖、产生 git 冲突（参见 `xkpane-parallel-run` 铁律） |
| 5 | **不臆造任务状态查询**，确认一律 `get_session_status` + `query_task_bindings` | XK-Pane 无 `get_task_status`；会话状态（active/exited）+ 绑定记录是唯一可靠信号 |

---

## 2. MCP 工具

| 工具 | 用途 |
|------|------|
| `query_todos` | 查询待办列表（按 status/priority/scope/scopeRef/search 过滤），取未完成项 |
| `create_todo` | 创建待办（仅 title 必填，可选 description/priority/scope/scopeRef/tags） |
| `update_todo` | 更新待办（按 id 改 title/status/priority/description）；分派标 `in_progress`、完成标 `done` |
| `list_projects` | 列出已注册项目，把 Todo 的 scope/scopeRef 解析为合法 `project_path` |
| `launch_task` | 在目标项目起 Claude/Codex 子实例，注入携带 Todo 内容的 prompt，返回 `sessionId` |
| `get_session_status` | 查会话状态（active/exited），确认实例是否起来 |
| `create_task_binding` | 记录一次分派（title/projectPath 必填，可选 prompt/sessionId/workspaceName/cliTool） |
| `update_task_binding` | 推进绑定进度（status∈pending/running/waiting/completed/failed，progress 0-100） |
| `query_task_bindings` | 查已有分派记录（按 status/projectPath/search 过滤），核对哪些 Todo 已派发 |

---

## 3. 流程

### 3.1 查询待办

调 `query_todos`：
- `status`: `"todo"`（只取未完成）
- `priority`（可选）：从 `$ARGUMENTS` 解析 `--priority high|medium|low`
- `scope` / `scopeRef`（可选）：按工作空间或项目限定（如只分派某项目的待办）

把返回的 `items` 列表展示给用户确认。

> 可选去重：先 `query_task_bindings` 看哪些 Todo 已有在途绑定，避免重复分派。

### 3.2 筛选分派

用户确认要分派哪些 Todo（可全选或部分选）。对每条待选 Todo：
1. **定目标项目**：从 Todo 的 `scope`/`scopeRef` 推断 → 经 `list_projects` 校验得到合法 `project_path`；推断不出就询问用户
2. **生成 prompt**：按 Todo 的 `title` / `description` / `priority` 套用第 5 节模板

### 3.3 批量启动

对每条待分派 Todo，按顺序（**铁律 1**）：
1. `update_todo`：`id=<todoId>`、`status="in_progress"`（先标在途）
2. 长 prompt 先落 `.xkpanes/prompts/<todoId>.md`，prompt 传短引用（**铁律 2**）
3. `launch_task`：`project_path=<校验后路径>`、`prompt=<内容或短引用>`、可选 `cli_tool`，拿回 `sessionId`
4. `create_task_binding`：`title=<Todo 标题>`、`projectPath=<同上>`、`sessionId=<上一步>`，落库跟踪
5. 记录 `todoId ↔ sessionId ↔ bindingId` 映射

### 3.4 汇报与收口

展示分派结果：

| Todo | 项目 | sessionId | 绑定 | 状态 |
|------|------|-----------|------|------|
| ... | ... | ... | ... | 已启动 |

后续可选监督（参见 `xkpane-parallel-run`）：轮询 `get_session_status` 探各会话终态 → `update_task_binding` 推进度 → 该 Todo 完成后 `update_todo` 标 `done`。

---

## 4. 子命令

### 无参数 / `dispatch` — 查询并分派
完整执行第 3 节流程。`dispatch --priority high` 只分派高优先级。

### `list` — 仅查询待办
调 `query_todos` 展示列表，**不**执行分派。

### `create <title> [--priority high] [--project /path]` — 创建待办
调 `create_todo`（`--project` 映射为 `scope="project"` + `scopeRef=<path>`）。

### `done <todoId>` — 标记完成
调 `update_todo`（`id=<todoId>`、`status="done"`）。

---

## 5. 示例

```
/xkpane-dispatch-todos                         # 查询未完成 Todo 并分派
/xkpane-dispatch-todos list                    # 仅列出待办
/xkpane-dispatch-todos dispatch --priority high  # 仅分派高优先级
/xkpane-dispatch-todos create "修复登录 bug" --priority high
/xkpane-dispatch-todos done abc123             # 标记完成
```

---

## 6. Prompt 模板

分派时生成的 prompt 格式（超长则落 `.xkpanes/prompts/<todoId>.md` 后传短引用）：

```
## 任务来源

来自 XK-Pane Todo 系统的待办任务。

## 任务内容

标题: {todo.title}
描述: {todo.description}
优先级: {todo.priority}

## 要求

完成上述任务。完成后告知用户。
```

---

## 7. 相关 skill

- `xkpane-launch-task`：启动子实例的基础原语（长 prompt 写文件、cli_tool 选择、project_path 校验）
- `xkpane-parallel-run`：分派多条后的并行监督（fan-out + 轮询 `get_session_status` + 收口汇总）
