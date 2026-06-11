# P6e 交接文档（清空上下文后从这里接手）

> 本文件 = 「会话状态 + 下一步」快照，专为开启 **P6e — MC PE 控制（用户的 Minecraft 落点）**。
> 完整阶段计划见 [P6_PLAN.md](./P6_PLAN.md)（§1.4 MC PE 真身映射约 line 40-44 / §P6e line 111-113 / §开放问题 line 143 / §84 Go 移植技术映射表），逐会话进度见 [progress.md](./progress.md)（末节 P6a~P6d），铁律见 [CLAUDE.md](./CLAUDE.md)。
> 总目标：把 `J:\Program\Agent\cc-pane`（Tauri+Rust+React）忠实重建到本仓库（Wails v3 + Go 1.25 + React 19 + MUI 9 + foundation MVVM）。分支 `rebuild/cc-pane`。

## 当前进度

- **P1~P5 ✅**：终端真跑 CLI + 启动菜单 / 删 Todo·Spec·Memory + Git SCM + Home / Workspace 两层树 + 会话恢复 / Local History 忠实化 / 终端进阶（Tab 拖拽等）。均用户手测通过。
- **P6a~P6d ✅（全部 live 验过）**：
  - P6a orchestrator 地基（127.0.0.1:0 HTTP + 32-hex token + `/mcp` StreamableHTTP + 写 `mcp-orchestrator.json` 到可执行文件目录）。
  - P6b MCP 注入被启动 CLI（`terminal.resolveLaunch` 填 port/token，per-session 配置落 `<proj>/.claude/`）。
  - P6c 33 编排 MCP 工具（`orchestrator_tools.go`）。
  - P6d cli-hook 全栈（独立二进制 `cmd/xkpane-cli-hook` 3 子命令 + `projecthooks` 注册 + REST session-started + notify 通道）。**三引擎 claude·reclaude·codex** 的 session-start 注入 / session-started 上报 / plan-archive / notify 全部手测通过。codex Windows 经 PowerShell 执行 hook，命令需 `&` 调用运算符（见 `projecthooks/projecthooks.go` powershellHookCommand）。
- **P6e（本次）= MC PE 控制，未开始。XK 侧目前零 MC PE 代码。** P6f Bedrock 留更后。

## 本次目标

完整做 **P6e — MC PE 控制**，忠实移植 cc-pane MC PE 子系统全栈，**不以最小改动为目的**。这是 P6_PLAN 标注的 **XL 阶段**（「建议像 P3/P4 那样单独出一份 P6e_PLAN 再动手」）。

**执行顺序铁律**：先精读（下方清单）→ 产出 `P6e_PLAN.md`（cc-pane/XK 双侧 file:line 映射 + 单元切分 + 决策点）→ 用户拍板 → 再逐单元实现。

## 动手前必读（上下文已清空，先读这些建立事实，勿凭记忆/训练数据臆测）

**XK 侧**
- `CLAUDE.md` —— 项目铁律：SQLite-only（禁 localStorage）、工具层（httpx/procx/cryptox/logx/filex，禁直接 net/http·exec.Cmd·log·os.WriteFile 配置）、NativeDialogs、`@mui/icons-material` *Rounded（禁 emoji/第三方 icon）、`t()` i18n、`theme.palette.foundation.*`、MVVM 调用边界（View→VM→services→bindings）、分层准则、平台 build tag 文件。**注：用户可能调整了 wails3 工具链版本，以仓库现状与 `go.mod`/`Taskfile.yml` 为准。**
- `P6_PLAN.md` —— §1.4（MC PE 真身映射）、§P6e（line 111-113）、§开放问题（line 143）、§84（Go 移植技术映射表）。
- `progress.md` 末尾若干会话条目 —— 了解 orchestrator/terminal/storage/app 的现有接线方式。
- 现有代码：`internal/services/orchestrator/`（`orchestrator.go` Deps/Start、`orchestrator_tools.go` MCP 工具注册法、`rest.go` REST handler 注册法）、`internal/storage/models*.go`（加表 = `&Model{}` 进 `AllModels` 即 AutoMigrate）、`internal/app/app.go`（service 注册 + 平台窗口 + 各 service 依赖装配）、`internal/services/terminal/terminal.go`（launch 链路）。

**cc-pane 源（忠实移植对象，逐文件精读）**
- 后端 `J:/Program/Agent/cc-pane/cc-panes-core/src/services/mc_pe/`：`service.rs`（`MCPEService.launch` 复刻 MCStudio 12 步启动）、`launcher.rs`、`paths.rs`、`pack.rs`、`world_copy.rs`、`symlink.rs`、`level_data.rs`、`log_buffer.rs`（日志环形缓冲 cap=2000）、`log_filter.rs`、`safety.rs`、`bridge_mod.rs`、`mod.rs`，以及 `resources/include_test_mod/`（游戏内 Python2.7 桥模组：`modMain.py`/`IPCSystem.py`/`Config.py`/`Game.py`/`QuModLibs/…` + `manifest.json` + `entities/`）、`resources/level.dat`、`resources/license/`。
- 后端 `…/services/mcdk_bridge/`：`bridge.rs`（TCP server，bind `127.0.0.1:0` 等游戏内 mod 反向连入）、`protocol.rs`（**6 字节大端帧** `[2B type][4B len][payload]` fire-and-forget）、`capture.rs`（按 MC PE PID 定位顶层窗口 → Win32 `PrintWindow/BitBlt` → 480p JPEG → base64）、`click.rs`（`SendInput`）、`window.rs`、`mod.rs`。
- 后端 MCP 工具：`orchestrator_service.rs` 里 `#[cfg(windows)]` 的 **5 个 MC PE 工具**（约 line 2661-2809）：`capture_window` / `click_window` / `execute_mc_code` / `reload_mc` / `get_mc_pe_logs`。`execute_mc_code`/`reload_mc` 把指令编成 6 字节帧发给桥模组（游戏进程内 exec Python2.7）；`capture_window` 是唯一返回 MCP image content 的工具。
- 前端 `J:/Program/Agent/cc-pane/web/components/mc_pe/`：`MCPELaunchPanel.tsx` / `MCPEView.tsx` / `MCPackageConfigPanel.tsx` / `MCPackageDetail.tsx` / `MCPackageList.tsx` / `knowledge/`。

**全局规则**（`~/.claude/CLAUDE.md`）：Minecraft 中国版/MCStudio 运行时操作须走 CC-Panes Orchestrator MCP 工具——这是「用现有 cc-pane 驱动游戏」的规矩，与「在 XK 里重建 MC PE」是两件事，但 P6e 手测时可参考它理解期望行为。

## Go 移植技术映射（P6_PLAN §84 已定，沿用）

- Win32（windows-rs）→ `golang.org/x/sys/windows`；截图 JPEG 用 `image/jpeg`，缩放用 `golang.org/x/image/draw`（无 Lanczos3，取近似）。
- TCP server + 二进制帧 → `net.Listen` + `encoding/binary`（大端）。
- 游戏内 Python 桥模组 → `embed.FS` 内嵌整个 `include_test_mod` 树（含 QuModLibs，一字不改），launch 时解压进 behavior_packs 并注入 env（cc-pane 用 `MCDEV_DEBUG_IPC_PORT={bridge port}`）。
- `mc_pe_*` 表 → 新建 GORM 模型进 `internal/storage/AllModels`（P6_PLAN §62：这些表此期才建）。
- 子进程启动走 `procx`；HTTP（若有）走 `httpx`；日志走 `logx`；配置原子写走 `filex`；敏感串落库先 `cryptox`。

## 工作流铁律（务必遵守）

1. 先出 `P6e_PLAN.md` 并让用户拍板，再分单元实现。每单元独立可编译 + 独立提交（**中文 conventional commit**），**不要 push**。全程**中文回复**。
2. 改 Go 后端必须**完整重启 wails3 dev** 才生效（提醒用户）；改独立二进制/embed 资源要重 build。
3. 架构有歧义/多解先停下问用户（CLAUDE.md「Think Before Coding」），不要静默选路。
4. **忠实复刻 cc-pane 优先**，任何偏离必须显式标注理由（如 Win32 GDI 差异、无 Lanczos3、Go 无 cfg 等价物等）。
5. MC PE 手测需真机（MCStudio + MC PE 已装、MC PE Panel 勾「启用 MCDK 调试桥」、Bridge 显示「已连入」），由用户实跑、agent 盯 `get_mc_pe_logs`/`app.log` 核对（同 P6d 验法）。

## P6e_PLAN 须先回答的决策点（精读后用 AskUserQuestion 收口，至少含）

- **范围与分期**：Win32 `capture_window`/`click_window`（PrintWindow/SendInput，最大难点）是 Day-1 一并做，还是先做 桥 + launch + 日志、capture/click 拆后续单元？
- **前端 MC PE 面板**移植到什么程度（5 组件全做 / 只做 Launch + 日志查看 / 暂不做 UI、先打通后端 MCP 工具）？
- **MCStudio 安装/世界定位**：检测路径与 12 步启动有哪些 Windows 特定假设需向用户确认？
- **桥协议细节**：帧 type id（cc-pane 用 3-8）、`MCDEV_DEBUG_IPC_PORT` env 名是否照搬不改——读 `include_test_mod` 的 `IPCSystem.py`/`Config.py` 确认。
- **内嵌 Python 桥模组**：embed 整个 `include_test_mod`（含 QuModLibs）一字不改，对吧？许可证（`resources/license/`）随附是否要一并带上？

## 建议单元起点骨架（以精读后判断为准，可在 P6e_PLAN 调整）

- **P6e-1** 桥协议 + TCP server：`protocol` 帧编解码（`encoding/binary` 大端）+ `bridge` `net.Listen` + 握手 + 日志环形缓冲（cap=2000）。
- **P6e-2** embed Python 桥模组 + `MCPEService.launch`（MCStudio 12 步 + 解压桥模组进 behavior_packs + 注入 env）。
- **P6e-3** `mc_pe_*` 表 + `execute_mc_code`/`reload_mc`/`get_mc_pe_logs` MCP 工具接线（进 orchestrator）。
- **P6e-4** Win32 `capture_window`（PrintWindow→JPEG→MCP image content）+ `click_window`（SendInput）。
- **P6e-5** 前端 MC PE 面板（按拍板范围）。
- **P6e-6** 打包资源（embed 校验 + 真机手测清单）。

---

**触发方式**：新会话只需说「读 P6E_HANDOFF.md，按它开始 P6e」即可。先精读上述 XK + cc-pane 源，再产出 `P6e_PLAN.md` 给用户拍板；读的过程中遇任何架构歧义随时问。
