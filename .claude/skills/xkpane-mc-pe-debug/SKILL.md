---
name: xkpane-mc-pe-debug
description: XK-Pane MCDK 调试桥对运行中的 MC PE 做截图 / 点击 / 热重载 / 读日志 / 进程内执行 Python 的调试闭环原语，并守住 Python 2.7 与归一化坐标的铁律。
---

# MC PE 调试闭环

XK-Pane 自带全套 MC PE（我的世界中国版）调试原语，子实例可经它对**正在运行**的游戏做截图、点击、热重载、读日志、在游戏进程内 exec Python 2.7，组成 Addon 调试闭环。再配 `pe_*` 知识库工具（API / 事件 / 组件 / 模板查询 + 代码体检），写代码前先核对、跑代码后看效果。

- **调用方式**：以下工具均经 panes MCP 调用（Claude Code 内工具名 `mcp__panes__<tool>`，codex 内同名）。正文统一用【逻辑名】（如 `execute_mc_code`），不再重复写前缀。
- **平台**：仅 Windows。
- **生命周期**：fire-and-forget —— `execute_mc_code` / `reload_mc` / `click_window` 都不回传执行结果，效果一律靠 `capture_window` + `get_mc_pe_logs` 观察。
- **日志独立**：MC PE 日志走 `get_mc_pe_logs`，**不是** Terminal session —— 不要用 `list_sessions` / `get_session_output` 找它。

> 前置（缺一不可，否则工具调用返回「桥未连接」）：
> 1. MC PE 已**经 XK-Pane 启动**（`launch_task` 或 MC PE Panel）。
> 2. MC PE Panel 已勾选「启用 MCDK 调试桥」。
> 3. Bridge 状态栏显示「已连入」。
> 不满足时不要反复重试，直接提示用户去 MC PE Panel 检查桥状态。

---

## 1. 铁律

| # | 规则 | 原因 |
|---|------|------|
| 1 | `execute_mc_code` 的代码**必须是 Python 2.7**，禁止 f-string / type hints / `async` / `await` / `:=` | 桥模组在游戏进程内 `exec`，2.7 语法外的写法直接语法报错，且无回显只能靠日志事后捞 traceback |
| 2 | **`print` 必须函数形式** `print(x)`，禁止 `print x` 语句形式 | Py2.7 在 `from __future__` 缺失时仍按语句解析裸 `print x`，行为不一致易踩坑 |
| 3 | **客户端 / 服务端严格分离**：`isClient` 选错会 import 不到对应 API | ServerSystem 不能 `import clientApi`，反之亦然；跨端 import 直接抛 ImportError |
| 4 | 日志查 MC PE **只用 `get_mc_pe_logs`**，禁止 `list_sessions` / `get_session_output` | MC PE 不挂在 Terminal sessions 上，那两个工具查不到它的输出 |
| 5 | `click_window` 坐标**必须归一化 `[0.0, 1.0]`**，禁止传像素值 | 坐标是窗口客户区比例（0=左/上，1=右/下），传像素会落到错误位置或越界 |
| 6 | **写代码前先用 `pe_*` 核对**（`pe_search_api` / `pe_validate_call` / `pe_review_code`） | MCDK API 签名繁杂，盲写易拼错组件名 / 参数；先查再写省掉一轮 reload 验证 |

---

## 2. MCP 工具

### 2.1 运行时调试桥

| 工具 | 用途 |
|------|------|
| `capture_window` | 截取 MC PE 窗口（480p JPEG，多模态可直接看图），无参数 |
| `click_window` | 在 MC PE 窗口客户区模拟左键点击（归一化坐标，先强制窗口前置） |
| `execute_mc_code` | 向桥模组发送 Python 2.7 代码，在游戏进程内 `exec`（区分客户端 / 服务端） |
| `get_mc_pe_logs` | 读最近 N 行 MC PE 日志（环形缓冲，cap=2000，等同 MC PE Panel 左下日志区） |
| `reload_mc` | 热重载世界 / addon 资源 / 着色器（四种 kind） |

### 2.2 知识库（写代码前核对）

| 工具 | 用途 |
|------|------|
| `pe_search_api` / `pe_get_api` | 搜 / 取 MCDK API 详情（签名、参数、返回值） |
| `pe_search_event` / `pe_get_event` | 搜 / 取引擎事件（名称、触发时机、回调字段） |
| `pe_search_component` / `pe_get_component` | 搜 / 取实体组件（CompFactory 下各组件方法） |
| `pe_match_template` | 按意图匹配代码模板（常见玩法骨架） |
| `pe_validate_call` | 校验一次 API 调用是否签名正确 |
| `pe_review_code` | 体检整段 MC 代码（端分离 / Py2.7 / 常见误用） |

> 启动 / 接管 MC PE 进程用 `launch_task`；查 XK-Pane 当前会话 / 项目用 `list_sessions` / `list_projects` / `list_workspaces` / `health`（与 MC PE 调试桥无关，按需）。

---

## 3. 调试闭环

纯文本主循环：

```
改代码 → reload_mc → capture_window 看效果 → get_mc_pe_logs 查报错 → 定位 → 改代码 → 回到第一步
```

### 3.1 截图核对状态

调用 `capture_window`（无参数），返回 `image/jpeg`，可直接视觉理解当前画面。用于确认玩家位置、UI 状态、方块 / 实体是否生成、特效是否触发。

### 3.2 读日志定位问题

调用 `get_mc_pe_logs`：

| 参数 | 说明 |
|------|------|
| `lines`（可选） | 默认 200，最大 2000 |

返回纯文本，每行 `HH:MM:SS [Category] [LEVEL] message`（同 MC Studio 白名单过滤）。

> 裸 `print('xxx')` 可能被白名单滤掉看不到。验证输出建议带大写 mod-tag 前缀，如 `print('[DEBUG] applied')`，更易在日志里捞到。

### 3.3 在游戏进程内执行 Python

调用 `execute_mc_code`：

| 参数 | 说明 |
|------|------|
| `code` | Python 2.7 源代码（受第 1 节铁律约束） |
| `isClient`（可选） | `true` 客户端执行，`false` / 缺省 服务端执行 |

fire-and-forget，执行结果靠 `get_mc_pe_logs` 观察。建议先 `pe_validate_call` / `pe_review_code` 核对再发。

**典型用法（服务端给玩家加 buff）**：

```python
import mod.server.extraServerApi as serverApi
CF = serverApi.GetEngineCompFactory()
playerId = serverApi.GetPlayerList()[0]
CF.CreateEffect(playerId).AddEffectToEntity('speed', 30, 2, True)
print('[DEBUG] buff applied to {}'.format(playerId))
```

**Python 2.7 强约束清单**（违规即语法报错）：

- 禁止 f-string —— 用 `'{}'.format(x)` 或 `'%s' % x`
- 必须 `print(x)` 函数形式，不能 `print x`
- 禁止 type hints（`def f(x: int) -> str` 不合法）
- 禁止 `async` / `await` / `:=` 海象运算符
- 客户端 / 服务端严格分离：ServerSystem 不能 `import clientApi`，反之亦然

### 3.4 热重载

调用 `reload_mc`：

| 参数 | 说明 |
|------|------|
| `kind` | 必填，四选一（见下表） |
| `fileName`（可选） | `kind=single_shader` 时必填 |

四种 kind：

| kind | 作用 |
|------|------|
| `game` | 重启游戏世界 |
| `addon_and_game` | 重载 addon 资源 + 世界 |
| `all_shaders` | 重编译全部着色器 |
| `single_shader` | 重编译指定着色器（配 `fileName`） |

fire-and-forget，重载完调 `capture_window` + `get_mc_pe_logs` 验证。

### 3.5 模拟点击交互

调用 `click_window`：

| 参数 | 说明 |
|------|------|
| `x` | 归一化 `[0.0, 1.0]`，0=左 / 1=右 |
| `y` | 归一化 `[0.0, 1.0]`，0=上 / 1=下 |

会先把 MC PE 窗口强制前置再点击。常用来触发 UI 按钮、对话框确认。坐标算不准时先 `capture_window` 目测目标在画面中的相对位置。

---

## 4. 常见动作配方

| 意图 | 步骤 |
|------|------|
| 完整闭环 | `capture_window` → `get_mc_pe_logs 200` → 与用户确认下一步 → 按需 `reload_mc` / `execute_mc_code` / `click_window` → 循环 |
| 仅截图 | `capture_window`，把图返回用户 |
| 仅读日志 | `get_mc_pe_logs`，`lines=N`（默认 200） |
| 服务端执行 | （可选 `pe_review_code` 核对 →）`execute_mc_code` `isClient=false` → `get_mc_pe_logs 50` 观察 |
| 客户端执行 | `execute_mc_code` `isClient=true` → `get_mc_pe_logs 50` 观察 |
| 仅热重载 | `reload_mc <kind> [fileName]` → `capture_window` + `get_mc_pe_logs 50` 验证 |
| 仅点击 | `click_window <x> <y>` |
| 写前查 API | `pe_search_api` / `pe_get_api` →（`pe_validate_call`）→ 再 `execute_mc_code` |

---

## 5. 失败处理

| 现象 | 原因 | 处理 |
|------|------|------|
| 调用返回「桥未连接」 | MC PE 未经 XK-Pane 启动 / 未勾选 MCDK 调试桥 / 桥未连入 | 提示用户在 MC PE Panel 检查桥状态（见前置三条），不要反复重试 |
| `execute_mc_code` 后日志没新输出 | Py2.7 语法错 / 运行时异常 / 输出被白名单滤掉 | `get_mc_pe_logs 100` 找 traceback；带 `[DEBUG]` 前缀重发；对照第 1 节约束；必要时 `pe_review_code` 复查 |
| `reload_mc` 后画面没变 | kind 选错（改了 addon 资源却用 `kind=game`） | 改用 `addon_and_game`；着色器改动用 `all_shaders` / `single_shader` |
| `click_window` 没生效 | 归一化坐标算错 / 误传像素 | 先 `capture_window` 确认目标在画面中的相对比例，坐标须落在 `[0.0, 1.0]` |
| 在日志里找不到 MC PE 输出 | 误用了 Terminal session 工具 | 只用 `get_mc_pe_logs`，别用 `list_sessions` / `get_session_output` |
| API 调用报参数 / 组件名错 | 盲写 MCDK 调用 | 写前 `pe_search_api` / `pe_search_component` 核对，`pe_validate_call` 校验签名 |

---

## 6. 相关文件 / 参考

```
.claude/skills/xkpane-mc-pe-debug/SKILL.md   # 本文件（同步落 .codex/skills/）
```

- 调试桥状态、日志区、「启用 MCDK 调试桥」勾选项均在 **XK-Pane 的 MC PE Panel**。
- 进程启动 / 接管：`launch_task`；会话 / 项目 / 健康查询：`list_sessions` / `list_projects` / `list_workspaces` / `health`。
- 知识库查询前缀 `pe_*`（API / 事件 / 组件 / 模板 / 校验 / 体检），写 MC 代码前优先核对。
