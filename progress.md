# Progress：cc-pane → XK-Pane 重构

## 会话 1 — 2026-05-29（规划）

### 做了什么
- 探索 cc-pane（Tauri/Rust 9 crate，269 命令/32 服务/27 表/142 组件/50 store）与 XK-Pane(foundation, Wails/Go/MUI9) 两侧架构。
- 确认用户决策：核心+常用增强 / 全新 MVVM 重写 / 纯 Context（弃 Zustand）/ 早期纳入 Bedrock·MCPE·notify·memory，延后 IM 桥接。
- 产出三份规划文件：`task_plan.md`（6 阶段 + 8 决策）、`findings.md`（架构盘点 + 源→目标映射 + 风险）、`progress.md`。

### 关键决策落点
- D3 PTY：新建 ptyx（go-pty/ConPTY + procx JobObject）—— 最大技术风险，Phase 1 先 spike。
- D7 路径字段铁律 1:1 保留（project_path / workspace_path / launch_cwd）+ 守门测试移植。
- D2 状态：每域一个 React Context Provider 替代 Zustand。

### 下一步
- 等用户确认 task_plan 末尾「待澄清」项（应用名/弹出窗口取舍/Bedrock 数据复用）。
- 启动 Phase 0：改名基线（go.mod/import/bindings/AppDirName/Taskfile）。

### 用户确认（2026-05-29 续）
- 应用名 = XK-Pane；所有窗口形态特性（弹出/迷你/无边框）全要；Bedrock 22MB 数据拷贝复用。
- 已更新 task_plan「已确认」段 + Phase 3 加窗口形态特性。

### 启动尽调 workflow（ultracode）
- Run ID `wf_5345a74a-b7e`：7 个并行调查 = Research(PTY/MCP/Git 库选型) + Spec(改名清单/CLI适配器/terminal_service设计/数据模型GORM规格)。
- 目的：动代码前消除计划中标记为 spike/评估 的高风险未知项。
- 完成后将结果合并进 findings.md，并据改名清单执行 Phase 0。

### Phase 0 改名执行（2026-05-29）
- 工具链：go 1.25.3 ✓ / node v24.15.0 ✓ / pnpm 10.12.4 ✓ / git ✓；**wails3、task 未安装**（GOPATH/bin 空）；`frontend/{dist,bindings,node_modules}` 全无（全新检出）。
- Grep 全量复核后改名（比尽调 agent 多查出 2 处：`paths.go` 的 `DBFileName="foundation.db"`、`storagesvc.go` 的 `.foundation-write-test`）。已改：
  - Go：go.mod module、main.go + app.go(9 import)+subprocess+storagesvc+config+preferences+appsettings import 全量；AppDirName×3(storage/cryptox/logx)→`XK-Pane`；UserAgent、app Name/Description、3 个 window Title→`XK-Pane`；DBFileName→`xkpane.db`、write-test 临时名。
  - 前端：5 个 service 的 `@bindings/foundation/*`→`@bindings/xkpane/*`；index.html title、package.json name；i18n app.title + sidebar.brand(F→X)；HomePage 演示文案；Database 默认文件名→`xkpane.db`。
  - 配置：Taskfile APP_NAME→`xkpane`；build/config.yml info.*；build/windows/info.json 元信息。
- **决策：保留主题系统的 `foundation` 命名空间**（`theme.palette.foundation.*`/preset 名/`FoundationThemeProvider`）——这是设计系统内部 API 名，非模块/bindings 路径，改它是无收益高风险的大重构。`build/ios` 的 `-framework Foundation/CoreFoundation` 是 Apple 系统框架，绝不能改。`build/linux/nfpm` 的 `foundation.exe` 待 Phase 6 由 build-assets 重生成。
- **验证**：`go build ./internal/...` exit=0 ✓；`go mod tidy` exit=0 ✓；`go vet ./internal/...` exit=0 ✓。
- wails3 已装（pin alpha.96，在 `~/go/bin`）；`wails3 generate bindings` exit=0 ✓ → 生成 `frontend/bindings/xkpane/`（5 Services/22 Methods/24 Models/1 Event），**bindings 前缀=xkpane**，改名端到端验证通过。
- `pnpm typecheck` 待 `pnpm install` 完成（后台）。`task`(go-task) 仍未装——非验证必需（已直接用 wails3+go）；用户要 `task dev/build` 快捷入口可 `go install github.com/go-task/task/v3/cmd/task@latest`。

### Phase 0 收口（2026-05-29）✅
- `pnpm typecheck` exit=0（`xkpane-frontend@0.0.0`，@bindings/xkpane 解析通过）。
- **Dev/Release 隔离**：新建 `internal/appinfo`（3 文件：appinfo.go + mode_dev.go(`//go:build !production`→`-Dev`) + mode_prod.go(`//go:build production`→空)）。机制来源：`build/windows/Taskfile.yml` build:native 的 release 分支加 `-tags production`，`wails3 dev` 走 DEV=true 不加。storage/cryptox/logx 三处 `const AppDirName = appinfo.AppDirName`（单一事实源 + 无环：appinfo 是零依赖叶子包）。
- 守门测试 `internal/appinfo/appinfo_test.go`：断言 dev→`XK-Pane-Dev` / release→`XK-Pane`，双 tag 模式均 ok。
- **Phase 0 完成。** 下一步 Phase 1：ptyx（先做 ConPTY spike，R1 最大风险）。

### 测试结果（Phase 0 全绿）
- `go build ./internal/...`（dev）exit=0；`go build -tags production ./internal/...` exit=0
- `go vet`（dev + production）exit=0；`go mod tidy` exit=0
- `go test ./internal/appinfo/...`（dev + production）ok
- `wails3 generate bindings` exit=0（前缀=xkpane）；`pnpm typecheck` exit=0

## 会话 1 — Phase 1 开工（2026-05-29）

### ptyx（R1 最大风险）✅ 已实跑验证
- 加依赖 `github.com/aymanbagabas/go-pty v0.2.3`（+creack/pty、u-root 间接）。`go doc` 确认 API 与尽调一致：`Pty`=io.ReadWriteCloser+`CommandContext/Resize(w,h)/Fd`；`Cmd` 暴露 `SysProcAttr/Process`。
- 新建 `internal/utils/ptyx/`：ptyx.go(门面 Spec/PTY/Spawn/Read/Write/Resize/PID/Wait/Kill/Close) + ptyx_windows.go(JobObject 绑定) + ptyx_unix.go(不叠 Setpgid) + ptyx_test.go。
- Windows 决策：只用 `CREATE_SUSPENDED`（**不加** CREATE_NEW_PROCESS_GROUP——交互式 PTY 的 Ctrl+C 走字节流 0x03，进程组标志会干扰 TUI 信号语义）；绑 Job 后 ResumeThread。
- **验证（Win11 实跑）**：`TestSpawnEchoProducesOutput` PASS（ConPTY 起 cmd.exe + Job 绑定 + 从原始字节流读到 marker）、`TestResizeNoError` PASS；`go build` exit=0；`GOOS=linux go build` exit=0（Unix 路径编过）。
- 结论：findings R1（ConPTY 在 Wails/Go 下可用性）**实测消除**。

### events + GORM 核心模型（2026-05-29）✅
- `internal/events/events.go`：加 `terminal:output`/`:status`/`:exit` 三事件 + payload struct，`Register()` 注册。bindings 重生成 → 4 Events / 27 Models，OK。
- `internal/storage/models_ccpane.go`：7 张 data.db 核心表 GORM 模型 + `init()` 追加 AllModels + 显式 `TableName()`。
  - **踩坑**：GORM 默认把 `LaunchHistory` 复数化成 `launch_histories`，与描述符 `launch_history` 不符致建表测试失败 → 给全 7 个加 `TableName()` 显式表名修复。
  - 时间戳用 Go `time.Time`（greenfield，不导入 cc-pane 旧库）；敏感字段标 TODO 待写入方落地加 cryptox。
- 测试 `models_ccpane_test.go`：AutoMigrate 建表 + 注册校验，通过。
- **ptyx 测试 flaky 修复**：上次侥幸通过的 echo 测试这次因 ConPTY 异步刷新 race 失败（Wait 后立刻 Close 丢尾部输出）。改为**轮询缓冲区等 marker**，`-count=3` 稳定通过。教训记入 Phase 2 待办（终端服务进程退出后须留 drain 宽限）。

### cliadapters（8 适配器 + 守门测试）✅（2026-05-29）
- 读全 9 个 cc-pane Rust 源（lib + 8 适配器）后忠实移植到 `internal/services/cliadapters/`（11 个源文件 + 4 个测试文件）：
  - `adapter.go`：types（ToolInfo/Capabilities/Provider/Context/CommandResult，camelCase json tag）+ `Adapter` 接口 + `base`（内嵌提供 Info/Capabilities/Detect 默认）+ `detectDefault`/`probeVersion`（LookPath + procx 版本探测，honor 子进程铁律）+ 共享 `buildAddDirArgs`（路径铁律 D7）/`orchestratorEnv`。
  - `resolve.go`：`findExecutableInDirs`/`windowsNpmShimInvocation`/`isWindowsBatchFile`/`claudeCliJsForNpmShim`/`nodeForNpmShim`/`windowsExecutableExtensions`/`resolveClaudeLike`/`findWindowsExecutable`（与 Rust `#[cfg(any(windows,test))]` 一致——纯函数跨平台可测；真实 PATH 解析按 runtime.GOOS 分支，免拆 build-tag 文件）。
  - 8 适配器：claude/reclaude（共享 npm shim + add-dir + CLAUDECODE 移除 + orchestrator env）、codex（MCP `-c` dotted-key 覆盖 + JSON 编码 + 隔离排序）、glm（crush + CRUSH_*/ZAI_* env + crush.json，--cwd 恒 project_path）、kimi（per-session config + add-dir + KIMI_SHARE_DIR）、cursor/gemini/opencode（位置参数）。
  - `registry.go`：Registry + `Default()` 按 claude/codex/gemini/kimi/glm/opencode/cursor/reclaude 顺序注册。
- **范围决策**（CLAUDE.md 简洁铁律 + 用户延后项）：
  - ⏸ 延后：claude/reclaude `generate_mcp_config`（写 mcp-*.json，依赖 Phase 5 orchestrator port/token）→ stub 返回 ""；codex `~/.codex/config.toml` 解析（`disable_unlisted_mcp` 默认关）→ `configuredMCPServerNames` 返回 nil，免引入 TOML 依赖；**项目 hooks 机制**（依赖未移植的 `cc-panes-cli-hook` 二进制 + 已确认延后的 IM 桥接）→ 暂不进接口。均以 `TODO(phase5)` 标注 hook 点。
  - 写配置文件（glm crush.json / kimi config）走 `filex.WriteAtomic`+`EnsureDir`（配置文件铁律）；npm shim 回退告警走 `logx`。
- **测试**（守门 18 个 = build_command/路径铁律/exe 解析；hooks 相关 5 个随机制延后，terminal 1 属 Phase 2、web 3 属前端）：
  - claude/reclaude 在 Rust 里重复的 add_dir(各3)/find_exec/npm_shim 因 Go 共享辅助而**去重合并**（共享函数测一次），断言串与 Rust 测试逐字对齐。
  - `resolve_test.go`(6)：find_exec 4 态 + npm shim 2；`adapter_test.go`(5)：add-dir 3 态 + 注册表顺序 + 元信息/能力（GLM=crush、reclaude versionArgs=[version]、多目录能力矩阵）；`codex_test.go`(3)：dotted-key/dev-instructions/隔离排序；`glm_test.go`(2)：launch_cwd 恒 project_path。
- **验证全绿**：`go build`(dev+prod) / `go test ./internal/services/cliadapters/...`(dev+prod) / `go test ./internal/...`(全量) / `go vet ./internal/...` 全 exit=0。
- 启动 workflow `wf_9968c0f6-862` 做 Rust↔Go 保真度审查（6 并行对照器，覆盖未被守门测试实跑的完整 BuildCommand 参数装配 + env + 全 8 适配器 info/caps）。

### cliadapters 保真度审查结论（workflow wf_9968c0f6-862）✅
- codex / glm / kimi / cursor+gemini+opencode / resolve+registry-fields：**全部 faithful**（完整 BuildCommand 参数顺序、env、flag 名、能力/兼容 Provider 类型逐一核对一致）。
- **1 个实质 drift（已修）**：注册顺序。审查指出 cc-pane **生产注册点** `src-tauri/src/lib.rs:920-927` 为 Claude→**Reclaude**→Codex→Gemini→Kimi→Glm→OpenCode→Cursor（Reclaude 紧跟 Claude），而我误用 findings 的随手枚举顺序把 Reclaude 放末位。**已 Grep 核实真实源码后修正** `Default()` 与守门测试 `TestDefaultRegistersAllEightInOrder`。
- **2 个 low 备注（裁定不改 / 加注）**：
  - kimi 配置 0o600 vs Rust 0o644：该文件含 `api_key`（密钥），无法加密（CLI 需明文读），0o600 仅属主是更安全的**有意加固**——保留并加注释说明。
  - kimi `Some("")` vs `APIKey==""`：全适配器一致的 `""==None` 约定，非 drift。
- 修正后 `go test`（dev+prod）/`go vet` 全 exit=0。

### Phase 1 收口决策：subprocess git 白名单 → 改判延后 Phase 4（用户确认）✅
- 动手前发现**层次错配**：`subprocess.Service` 是**前端异步白名单**（按 ID 发事件给前端订阅，铁律只约束「前端」调子进程）；而 D4 的 git CLI fallback **消费方是后端 Git 服务**（Phase 4），需**同步捕获 git 输出**——后端 Go 直接用 procx 即可（procx 是后端原语）。把 git 子命令塞进前端白名单是错的层，且 Git 服务未建时确定子命令/参数属投机（违 CLAUDE.md 简洁铁律）。
- 用 AskUserQuestion 提了三选项（延后 Phase 4 / 现建后端 gitx / 字面扩前端白名单），**用户选「延后到 Phase 4」**。
- 落点：Phase 4 的后端 Git 服务用 procx 直连 + 服务内部 git 子命令 allowlist + 同步取输出，不经前端 subprocess。前端白名单保持空（待前端用例）。已在 task_plan Phase 1（标 [~] 延后）/ Phase 4（加层次说明）/ 状态表更新。

### Phase 1 完成 ✅（2026-05-29）
- 四个实质单元全部落地并验证：**ptyx**（R1 ConPTY spike 实跑）/ **events**（终端事件）/ **models_ccpane**（7 表 GORM）/ **cliadapters**（8 适配器 + 18 守门 + 保真度审查）。
- subprocess git 白名单改判延后 Phase 4（层次修正，非遗漏）。
- 全验证：`go build`/`go test`/`go vet`（dev + production）+ `wails3 generate bindings` + `pnpm typecheck` 全 exit=0。
- 提交：`1ee9771`(cliadapters) / `21b2c64`(events+models) / `9df58c1`(ptyx) / `b7281a3`(Phase 0 改名) / `97751b6`(init)，均在 `rebuild/cc-pane` 分支。

### 下一步：Phase 2 — 终端核心闭环（首个可运行里程碑）
- 后端 `terminal_service`：PTY 会话注册表 + I/O 隧道（Emit `terminal:output`）+ resize/kill + launch_cwd 上报。**硬性**：进程退出后留 drain 宽限再 Close PTY（ptyx 测试暴露的 ConPTY 异步刷新教训）。
- 前端：`TerminalView`(xterm.js) + 自实现 SplitView + TabBar(@dnd-kit) + `TerminalContext`（替代 usePanesStore）+ 侧边栏项目树最小版 + 右键启动。

### 测试结果（Phase 1 累计）
- `go test ./internal/...`（dev + production）全 ok：appinfo / storage / ptyx（×3 稳定）/ cliadapters（18 守门，双模式）。
- `go vet`（dev+prod）exit=0；`wails3 generate bindings` exit=0（5 Services/27 Models/4 Events）；`pnpm typecheck` exit=0。

### 遇到的错误
- Bash 工具默认走 /usr/bin/bash 致 `$null` 重定向报错；改用 PowerShell 工具解决。
- Edit 报「File has not been read yet」：首批改名漏读文件；补 Read 后重做成功。
- Write 工具调用偶发 malformed（一次）；重发成功。

## 会话 2 — Phase 2 后端 terminal_service（2026-05-29，autonomous /loop）

### 入场盘点
- 上一轮在工作树里留了**未提交、未验证**的 `internal/services/terminal/terminal.go`（536 行，写得很完整但编不过），progress.md 未记录。本轮职责＝把这半成品收口（补缺 + 验证 + 提交 + 记录）。
- `go build` 暴露 4 个错：`logx.Logger` 未定义、`AttachApp` 把 `app.Event.Emit`(变参+bool 返回) 直接喂给 `func(name,payload any)` 字段签名不符、`newSessionID`/`defaultShell` 未定义（平台/工具辅助未落地）。

### 做了什么（一个单元：后端 terminal_service + 单测 + 接线 + bindings）
- **补缺 2 个错**：① `logger *logx.Logger` → `*slog.Logger`（`logx.For` 实返 `*slog.Logger`）；② `AttachApp` 包一层闭包把 Wails `Emit(name, data ...any) bool` 收敛成内部 `emit(name, payload any)`。
- **补 3 个辅助文件**（对照 cc-pane 源忠实移植）：
  - `id.go`：`newSessionID()` = `uuid.NewString()`（对齐 cc-pane `Uuid::new_v4()`；uuid 已在依赖图中）。
  - `shell_windows.go` / `shell_unix.go`（build tag 分平台，符铁律「平台分支走 build-tag 文件」）：`defaultShell()` 忠实移植 `terminal_service/shell.rs::resolve_default_shell`——Win `pwsh>powershell>%ComSpec%`，Unix `$SHELL` else `/bin/sh`（用 LookPath 既探测又解析绝对路径，对 ConPTY 更确定）。
- **单测 `terminal_test.go`（12 个）**：`utf8SafeProcess` 三分支（全有效/不完整尾部跨 read 拼回/中段非法 lossy）+ `isIncompleteTrailing` 边界 + `exitCodeOf`（nil/通用/真 `*exec.ExitError`=3）+ `buildEnv`（TERM/COLORTERM/session-id 注入、inject 覆盖、remove）+ `resolveLaunch`（裸 shell ""/none、未知工具报错、空 projectPath 报错）+ **集成 round-trip**：起真 pwsh(ConPTY)→写 `echo MARKER`→轮询 `terminal:output` 见 marker→Kill→`terminal:exit`→注册表回收。仿 ptyx_test 轮询风格规避 ConPTY 异步刷新 race；**round-trip ×5 稳定**。
- **接线**：`app.go` 预创建 `terminal.New(cliadapters.Default())` + 进 Services 列表 + `termSvc.AttachApp(app)`（镜像 subSvc 模式）。

### 验证（§4 全绿）
- `go build`/`go vet`/`go test ./internal/...` 均 **dev + production 双模式 exit=0**（terminal 12 测 + 既有 appinfo/cliadapters/storage/ptyx 全 ok）。
- `wails3 generate bindings` exit=0 → 6 Services / 28 Methods / 29 Models / 4 Events（+terminal 服务，+CreateSpec/SessionInfo 模型）。
- `cd frontend; pnpm typecheck` exit=0（新 bindings 解析通过）。

### 保真度 spot-check（对照 cc-pane terminal_service.rs:1147-1204）
- `buildEnv` 与 cc-pane env 注入逐项核对：TERM/COLORTERM 用 `or_insert`（=Go「不存在才设」）、`CC_PANES_PTY_SESSION_ID` 强制写——**一致**。其余 env（snapshot/launch_id/runtime_kind/orchestrator API/WSL distro）均 Phase 3/4/5 特性，MVP 正确延后并标 TODO。spawn cwd≡ProjectPath（铁律 D7）一致。
- **1 个良性微差异**（记 findings J6）：cc-pane 对裸 shell 也注入 `CC_PANES_CLI_TOOL=none`，本实现对 "none"/"" 跳过——无害（裸 shell 无 cli-hook 消费该值），保留实现者本意不改。

### commit / 下一步
- commit：见下方 hash（`Phase 2: 后端 terminal_service`）。一个单元一个 commit。
- **下一单元（Phase 2 续）**：前端 `services/TerminalService.ts`（包 @bindings/xkpane terminal）→ `TerminalContext`（面板树/活动会话，替代 usePanesStore）→ `TerminalView`(xterm.js) + 自实现 SplitView + TabBar(@dnd-kit) → 侧边栏项目树最小版 + 右键启动。

### 遇到的坑
- `go mod tidy` 因 goproxy.cn 对 go-pty 深层 test 依赖（u-root/gobusybox）返回 504 网络超时而中断——**与本改动无关**，go.mod/go.sum 未被改动（无 diff），uuid 解析正常、build/test 全过。后果：uuid 仍带 `// indirect` 标记（陈旧但无害，下次联网 tidy 自动消除）。记 findings J6。
- `AttachApp` 漏进前端 bindings（+1 方法）——**既有 subprocess 服务同样如此**（连 EventNamePrefix 也漏），属 foundation 脚手架既定约定；无害（前端无法构造 `*application.App` 调它）。按外科手术原则匹配既有约定不单独改，记 findings J6 为 Phase 6 统一清理候选。

## 会话 3 — Phase 2 前端 TerminalService.ts（2026-05-29，autonomous /loop）

### 选取单元（§2 一轮一个）
- 上轮收口了后端 terminal_service。本轮取 Phase 2「前端 MVVM」的**最小首增量＝服务层 `TerminalService.ts`**（推进顺序：后端→bindings→**前端 services/<Domain>Service**→Context→View）。`TerminalContext`（含 usePanesStore 面板树移植 + 保真度审查）刻意留到下轮单做，避免一轮贪多带病推进。

### 做了什么
- 探索前端约定：以 `services/subprocess/SubprocessService.ts` 为模板（最相似——同为事件驱动的进程控制服务）。读 `shared/events.ts`/`useTimeEvent.ts`/`services/index.ts`/`frontend/CLAUDE.md`（MVVM 铁律）。
- 核对生成的 bindings：`terminal/service.js`（CreateSession/WriteInput/Resize/Kill/List + 漏出的 AttachApp）+ `models.js`（CreateSpec{projectPath,workspacePath,cliTool,cols,rows,initialPrompt} / SessionInfo{id,pid,cwd,cliTool}）。
- 新建 `frontend/src/services/terminal/TerminalService.ts`：
  - 5 个方法 `create/write/resize/kill/list`（包 Binding，list 做 number 兜底 map）。
  - 3 个事件订阅 `onOutput/onStatus/onExit`，返回取消函数。**关键差异**：terminal 事件名是固定单名（terminal:output/:status/:exit），sessionId 在 payload 里（vs subprocess 把 id 拼进事件名后缀）→ 抽 `onSession<T>` helper 订阅固定名 + 按 `payload.sessionId` 过滤。
  - 导出强类型 `CreateSpec/SessionInfo/TerminalOutput/TerminalStatus/TerminalStatusKind/TerminalExit`。
- `services/index.ts` barrel 追加 TerminalService + 6 个类型导出（镜像 subprocess 块）。

### 验证
- `cd frontend; pnpm typecheck` exit=0（纯前端单元，无 Go 改动，§4 后端门不适用）。

### commit / 下一步
- commit：见下方 hash（`Phase 2: 前端 TerminalService.ts`）。
- **下一单元**：`TerminalContext`——对照 `cc-pane` 前端 `usePanesStore`（面板树/分屏嵌套/活动会话）忠实移植成 React Context（D2 弃 Zustand），含保真度审查。之后才是 `TerminalView`(xterm.js，需先 `pnpm add` xterm 系依赖) + 自实现 SplitView + TabBar(@dnd-kit)。

## 会话 4 — Phase 2 面板树类型 + 纯 helpers（2026-05-29，autonomous /loop）

### 选取单元（§2 拆分过大项）
- 原定下一单元 `TerminalContext` 的源 `usePanesStore.ts` **达 1743 行**，且大半是 Phase 3/4/5 范围（SSH/WSL、provider 选择、会话恢复、拖拽重排）——单轮做不完且会硬塞未落地功能。按 §2「过大→拆最小可独立验证增量」，本轮只做**面板树类型 + 纯 helpers**（Context 状态化部分留下轮）。

### 关键发现：两级嵌套分屏模型（记 findings J7）
- 读 cc-pane `web/types/{pane,terminal}.ts` + `web/stores/paneTreeHelpers.ts` 后确认是**两层**分屏树：
  - **外层 `PaneNode`**（窗口布局）：`Panel`（type:"panel"，含 tabs[] + activeTabId）| `SplitPane`（type:"split"，direction + children + sizes）。
  - **内层 `TerminalPaneNode`**（单个 Tab 内）：`TerminalPaneLeaf`（type:"leaf"，持 sessionId）| `TerminalPaneSplit`。即每个 Tab 自身又是一棵终端分屏树（`Tab.terminalRootPane` + `activeTerminalPaneId`）。
- paneTreeHelpers 操作的是**外层**树（findPane/findParent/collectPanels/createPanel/createTab）。

### 做了什么
- 新建 `frontend/src/terminal/types.ts`：移植两级树全部类型 + `CliTool/KnownCliTool`（对齐后端 8 适配器）。**MVP 裁剪**（文件头列明 TODO）：provider/launch-profile（Phase 3）、ssh/wsl/machineName/disconnected/restoring/savedSessionId（Phase 4）、resumeId（Phase 3）、contentType 非 terminal 变体（Phase 4/5）、pinned/minimized/dirty 等 UI 细节。
- 新建 `frontend/src/terminal/paneTree.ts`：6 个纯函数 `generateId/createPanel/createTab/findPane/findParent/collectPanels`，**near-1:1 移植** paneTreeHelpers.ts。**有意调整**（非 drift）：`createTab` 可选参从 cc-pane 的 resumeId/workspaceName/providerId（Phase 3）换成 MVP 相关的 `{cliTool, workspacePath, title}`；findParent 加 `noUncheckedIndexedAccess` 守门（`if(!child)continue`）。
- 放在 `frontend/src/terminal/`（终端域 Context 的家，与 foundation 的 `preferences/`/`i18n/` 顶层域目录一致）。

### 验证
- `cd frontend; pnpm typecheck` exit=0（纯前端，无 Go 改动）。纯函数无 Vitest（前端测试框架未配；§4 前端门=typecheck，未强制单测）——pane-tree 状态化 reducer 落地时再评估是否引入 Vitest。

### commit / 下一步
- commit：见下方 hash（`Phase 2: 面板树类型 + 纯 helpers`）。
- **下一单元**：`TerminalContext`（Provider + reducer）——状态化树操作（newTab/closeTab/setActiveTab/setActivePanel/splitPanel/closePanel，需移植 usePanesStore 对应 reducer 逻辑 + 折叠单子 split 等关键逻辑，建议 Workflow 保真度审查 + 考虑引入 Vitest 测树操作）+ wire TerminalService 生命周期（create→sessionId 回填 leaf、exit→清理）。

## 会话 5 — Phase 2 TerminalContext reducer + Provider + Vitest（2026-05-29，autonomous /loop）

### 选取单元（§2）
- 取上轮 teed-up 的 `TerminalContext`。**范围裁剪**（不贪多）：本轮做**纯状态层**——外层面板树 reducer（8 个 in-scope action）+ React Context Provider（D2 弃 Zustand 用 useReducer）。**刻意延后**异步 `TerminalService` 生命周期 effect（create→回填、exit→清理）到下一单元（与 `TerminalView` 同落，需内层 terminalPane 树 + effect）。

### 做了什么
- `frontend/src/terminal/reducer.ts`：**clone-then-mutate** 手法忠实移植 usePanesStore 的 8 个外层操作（split/closePane/addTab/closeTab/selectTab/setActivePane/resizePanes + normalizePaneTree 折叠）。对 rootPane 做 `structuredClone` 后套用 cc-pane 可变逻辑再返回——1:1 复刻 immer 语义同时保持 reducer 纯函数可测。`setTabSession` 作为生命周期回填钩子先行落地。延后项（closedTabs 快照 / syncPtyRegistryUnregister / pinned / moveTab·reorderTabs / 内层分屏）文件头列明。
- `frontend/src/terminal/TerminalContext.tsx`：`<TerminalProvider>` + `useTerminal()`，与 foundation `PreferencesProvider` 同构（useReducer + useCallback 绑定 + useMemo value）。生命周期 effect 标 TODO（下一单元）。
- `frontend/src/terminal/index.ts`：域出口（Provider/hook + 公共类型）。
- **引入 Vitest**（frontend/CLAUDE.md 推荐 + 上轮 teed-up）：`pnpm add -D vitest@4.1.7`（Vite 8 兼容）+ `vitest.config.ts`（node 环境，相对 import 无需别名）+ package.json `test`/`test:watch` 脚本。
- `reducer.test.ts`：先 15 测（基础行为），保真度审查后**补 6 测**至 **21 测**覆盖 critic 标出的高危路径。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm test`（vitest run）**21/21 pass**。纯前端单元，无 Go 改动（后端门不适用，无需 bindings 重生成）。

### 保真度审查（workflow wf_ad8d281f-012，3 并行 lens）✅
- **fidelity**：8 action 逐行对照 usePanesStore → **0 真实 drift**（4 条 low 均裁定 benign/intentional）。
- **adversary**（clone-then-mutate 正确性）：**0 真实 bug**，自写 ~10 probe 测验证：返回树不与入参 state 共享引用；closeTab 委托 closePane 传原 state 安全（closePane 自己 fresh clone）；findParent indexOf 在 clone 上引用一致；normalizePaneTree 返回值均被接住；structuredClone 对这些纯对象判别联合安全。
- **coverage**：6 条 `isReal=true` 的**测试缺口**（非 bug）。**当轮即补**（completeness-critic 输出转为即时工作）：3→2 子 sizes 重整([40,60])、activePaneId 兜底下钻 split、深层同方向合并、关末位子 Math.min 钳位、split 内多 Tab 面板 closeTab 非折叠、嵌套 split resize 不扰兄弟。补后 21/21 全绿——独立印证最易藏 bug 路径正确。
- **微加固**（adversary 提示）：`resizePanes` 拷贝入参数组（`[...sizes]`）——reducer 须独占 state，恢复 immer 冻结曾给的保护（弃 immer 后显式拷贝）。

### commit / 下一步
- commit `a34bcf3`（`Phase 2: TerminalContext reducer + Provider + Vitest (21 tests, fidelity-reviewed)`）。
- **下一单元**：`TerminalView`(xterm.js) ——需先 `pnpm add @xterm/xterm @xterm/addon-fit`（等）+ View/VM/Style 三件；随后 `TerminalService` 生命周期 effect 接进 Provider（newTab→create→setTabSession；订阅 onOutput 喂 xterm、onExit 清理）。再后 自实现 SplitView（消费 rootPane/resizePanes）+ TabBar(@dnd-kit) + 侧边栏项目树最小版 + 右键启动。

## 会话 6 — Phase 2 终端布局调度器（2026-05-29，autonomous /loop）

### 选取单元（§2）+ 关键发现
- 探 cc-pane `web/components/panes/` 发现终端渲染**已拆成可测控制器**：`terminalRendererController.ts`（WebGL 渲染器+上下文丢失恢复+纹理图集）/ `terminalLayoutScheduler.ts`（fit/resize 调度）/ `TerminalView.tsx`（React 胶水），前两者各带 `.test.ts`。这是因为它们用**注入式依赖**（getter/callback），可用 fake term/fit 单测——正好破解「前端视觉无法自动验证」的难题。
- **范围裁剪**（不贪多 + 简洁铁律）：本轮只做**最可自动验证**的 `layoutScheduler`（+ 移植其 test）。**WebGL `terminalRendererController` 延后**（性能增强，MVP 用 xterm DOM 渲染器即可，且依赖 terminalRenderer 决策 + settings 类型 + addon-webgl）。`TerminalView.tsx` React 胶水（视觉，仅 typecheck/build 间接验证）留下一轮。

### 做了什么
- `pnpm add @xterm/xterm@^6 @xterm/addon-fit@^0.11`（对齐 cc-pane 版本）+ `pnpm add -D jsdom@^28`（调度器 test 需 DOM 宿主）。
- `frontend/src/terminal/layoutScheduler.ts`：**逐字忠实移植** cc-pane `terminalLayoutScheduler.ts`（双 rAF + delay 去抖调度、容器抖动跳过 minContainerDelta、非活动/不可渲染门控、cols/rows 变化才回调 resizeBackend、isTerminalHostRenderable）。全依赖注入，零业务耦合。
- `frontend/src/terminal/layoutScheduler.test.ts`：逐字移植 cc-pane 的 4 个 test（可见性探测 / fit·repaint·resize / 非活动延后 / 抖动跳过+force）。**per-file `// @vitest-environment jsdom`** 切环境（reducer.test 仍跑 node 默认环境）。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm test` **25/25 pass**（2 文件：reducer 21 node + layoutScheduler 4 jsdom）。纯前端，无 Go 改动。
- 逐字移植 + cc-pane 原 test 通过 = 强验证，**无需重型保真度 workflow**（与 reducer 的 clone-then-mutate 语义翻译不同，此处零翻译风险）。

### 顺手 hygiene
- `frontend/tsconfig.tsbuildinfo`（tsc 增量缓存）此前被 track，每次 typecheck 都让工作树变脏污染 §1「上次提交干净」校验 → 加进 .gitignore + `git rm --cached`。commit `653eb00`。

### commit / 下一步
- commit `d71ac8f`（layout scheduler）+ `653eb00`（gitignore tsbuildinfo）。
- **下一单元**：`TerminalView`(xterm.js) View/VM/Style ——VM 创建真 Terminal+FitAddon、open 进 host、用 layoutScheduler 接 ResizeObserver、onData→TerminalService.write、onOutput(sessionId 过滤)→term.write、dispose 回收；View 渲染 `<div ref>` + 样式。随后把生命周期 effect（newTab→create→setTabSession / onExit→清理）接进 TerminalProvider。该单元视觉为主，自动验证 = typecheck + `pnpm build`（视觉正确性留用户 checkpoint）。

## 会话 7 — Phase 2 TerminalView（xterm.js）（2026-05-30，autonomous /loop）

### 做了什么（一个单元：TerminalView View/VM/Style）
- `frontend/src/components/TerminalView/`（放 foundation 既有组件目录，与 AppLayout/Sidebar 同级；终端**域逻辑**仍在 `src/terminal/`）：
  - `useTerminalView.ts`（VM）：mount effect 创建 `Terminal`+`FitAddon`→`open(host)`→建 `layoutScheduler`（resizeBackend→TerminalService.resize）→`term.onData`→TerminalService.write→`ResizeObserver`(debounce 150 / minDelta 5)→schedule；sessionId effect 订阅 `onOutput(sid)`→term.write、`onExit`→writeln+回调，attach 时 force-flush；active effect flush+focus；cleanup 全回收。ref-sync effect 避免过期闭包。
  - `TerminalView.tsx`（View）：`<Box ref sx={styles.host}>` + `import '@xterm/xterm/css/xterm.css'`。
  - `TerminalView.styles.ts`：host 取 `foundation.bg.content`，沿用 `(theme)=>Record<string,SxProps>` 约定。
- **MVP 裁剪**（cc-pane TerminalView 全量 1256 行）：延后 WebGL 渲染器 / 运行时切主题热更（mount 捕获主题色）/ ConPTY buildNumber 调优 / 写入流控 / reconnect·restore / wheel→方向键 / drag-fit 节流 / unicode11；会话**创建**不在本组件（消费给定 sessionId，创建在后续 Provider 生命周期 effect）。

### 验证（§4 前端门 = typecheck + build）
- `pnpm typecheck` exit=0；`pnpm build` exit=0（1831 模块，xterm 接入主 bundle 847KB）；`pnpm test` 25/25 无回归。视觉正确性留用户 checkpoint。

### 效果生命周期审查（workflow wf_8b582fe9-39a，2 lens）✅
- **lifecycle 对抗**：**0 真实 bug**。逐一追证并裁定 benign：cleanup dispose 顺序（同步 null 赋值 + ?. 守门覆盖 output-after-dispose 窗口）、StrictMode mount→dispose→mount（xterm.dispose 卸 DOM，open 干净重挂）、effect 声明序（mount 先于 sub/active；ref-sync 声明最先）、所有订阅 unmount/重订阅各释放一次无泄漏。
- **completeness vs cc-pane**：**0 MVP-critical 遗漏**。重点裁定 **Windows 写入流控（createTerminalWriteFlowControl）MVP 不需要**——它只节流前端→xterm 的 promise 链，**无法**反压后端；而本后端 output 走 fire-and-forget 批量 Wails Events（无 ack/backpressure 通道），且 `xterm.write` 内部自带队列、不丢/不乱序。故 `term.write(data)` 对 MVP 正确，流控纯属大爆发(cat 大文件)内存平滑优化，正确延后。初始 resize 经 session.attach force-flush 落实（lastSize=null → 首 fit 必回调 resizeBackend，后端学到真 cols/rows）。
- **1 个 cosmetic nit（裁定不改）**：`repaint()` 的 rAF 未纳入 scheduler.cancel() 跟踪——审查确认 null-guard + 单帧自解，不崩不漏，纯洁癖；按简洁铁律不为已验证的非问题加代码，记此备查。

### commit / 下一步
- commit `4df3cca`（TerminalView）。
- **下一单元**：自实现 `SplitView` 分屏——消费 `TerminalContext` 的 `rootPane`（递归渲染 Panel/SplitPane）+ `resizePanels`（拖拽分隔条），叶子 Panel 渲染 TabBar + 活动 Tab 的 TerminalView。之后 TabBar(@dnd-kit) + 把生命周期 effect（newTab→create→setTabSession / onExit→清理）接进 TerminalProvider + 侧边栏项目树最小版 + 右键启动 → Phase 2 闭环可演示。

## 会话 8 — Phase 2 自实现 SplitView 分屏组件（2026-05-30，autonomous /loop）

### 关键发现
- cc-pane 的分屏其实**已自实现**（非 allotment）：`PaneContainer`(递归分发) → `SplitContainer`(归一化 + 调 resizePanes) → `SplitView`(flex + sash 拖拽)。其 `allotment` 依赖另作他用。故 D6「自实现 SplitView」= 忠实移植其 SplitView 的拖拽机制。

### 选取单元（§2）
- 本轮只做**低层 `SplitView` widget**（N 子 + 拖拽 sash + sizes，children/onDragEnd 进出，**不依赖** TerminalContext/TabBar/session）。递归树渲染器（PaneContainer/SplitContainer 等价物，消费 TerminalContext + 委托 panel）留与 TabBar 同做的整合单元。

### 做了什么（`frontend/src/components/SplitView/`）
- `splitGeometry.ts`（纯，可测）：`computeSashResize`（拖拽 delta% 重算相邻两 pane + 最小百分比约束，溢出转嫁）+ `normalizeSizesToHundred`（末位吸收舍入，和恰 100）——抽自 cc-pane SplitView 拖拽 + SplitContainer.handleDragEnd。
- `useSplitView.ts`（VM）：忠实移植 cc-pane 指针拖拽——pointerdown→pointermove 双 rAF 节流→直接改 DOM flexBasis（不 re-render）→pointerup 回调 onDragEnd；卸载清理。**未移植** cc-pane 全局 `setDragging`（供 TerminalView drag-fit 节流；本 MVP 用固定 debounce 无需）。
- `SplitView.styles.ts` + `SplitView.tsx`（View）：容器/pane 纯布局走内联 CSSProperties，sash 走 sx（`foundation.accent` hover）；`<div data-splitview-pane>` 供 VM querySelector；`keys` 防 2→1 误 remount。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm test` **32/32**（reducer 21 + scheduler 4 + **splitGeometry 7**）；`pnpm build` exit=0。
- 几何数学 7 测覆盖（最易藏 bug 处）+ 拖拽逻辑逐字移植 cc-pane 工作代码 = 充分验证，**无需重型 workflow**（同 scheduler 判断）。bundle 未变（SplitView 尚未被入口 import，tree-shaken；待整合单元接入）。

### commit / 下一步
- commit `559fe13`（SplitView）。
- **下一单元**：递归 `PaneTree` 渲染器 + 把 Phase 2 拼起来——消费 `TerminalContext.rootPane` 递归渲染（split→SplitView 包 PaneTree，panel→挂活动 Tab 的 TerminalView）+ 生命周期 effect（newTab→create→setTabSession / onExit）接进 TerminalProvider。TabBar(@dnd-kit) 可与此同做或紧随。然后 侧边栏项目树最小版 + 右键启动 → 接进 App（路由/页面）→ **Phase 2 闭环可演示**（首个可运行里程碑）。

## 会话 9 — Phase 2 MVP TabBar（2026-05-30，autonomous /loop）

### 选取单元（§2）+ 关键裁决
- cc-pane `TabBar.tsx` 528 行（密度档/pin/重命名/右键菜单/状态点/@dnd-kit）。**关键realization**：@dnd-kit 拖拽重排**强耦合**父级 `DndContext` + `reducer.reorderTabs`（两者皆未落地）——独立 TabBar 无法自给。故本单元 = **MVP TabBar（选择/关闭/新建 + 活动态）**；拖拽重排单列一单元（reducer.reorderTabs + DndContext）。同时延后 pin/重命名/右键菜单/状态点/密度档（增强，非 MVP verify 所需）。

### 做了什么（`frontend/src/components/TabBar/`）
- `TabBar.tsx`（View）：横向可滚动标签条 + 固定「+」；标签 = 标题 + 关闭 X（hover/active 显），`role=tab/tablist/button`；图标用 `@mui/icons-material` `AddRounded`/`CloseRounded`（铁律），取色 `foundation.*`。
- `useTabBar.ts`（VM）：仅管活动标签 `scrollIntoView`（多标签溢出时滚入可见）+ tabNode 注册；选择/关闭/新建纯透传。
- `TabBar.styles.ts`：用 `SystemStyleObject<Theme>` 显式接口（非 Record）——`sx={[styles.tab, active?styles.tabActive:styles.tabInactive]}` 数组形式要求元素为确定 SystemStyleObject，规避 noUncheckedIndexedAccess 的 `|undefined` 与 SxProps 联合含数组的双重坑。
- i18n：公用 locale 加 `terminal.tab.{new,close}`（zh-CN/en-US 对齐）——terminal 属核心跨切 UI，类比 titleBar/sidebar 放公用 locale（aria-label 走 t()，铁律）。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0（修了 2 次 sx 类型坑）；`pnpm test` 32/32 无回归；`pnpm build` exit=0。纯展示组件 + 透传逻辑，typecheck+build 即足，无 workflow。

### commit / 下一步
- commit `fa312f0`（TabBar + locale）。
- **下一单元**：递归 `PaneTree` 渲染器 + 生命周期整合——消费 `TerminalContext.rootPane` 递归渲染（split→`SplitView` 包 PaneTree 子节点 + `resizePanels`+`normalizeSizesToHundred`；panel→`TabBar` + 活动 Tab 的 `TerminalView`）+ 把生命周期 effect（newTab→`TerminalService.create`→`setTabSession` 回填；onExit→清理）接进 `TerminalProvider`。随后 接进 App（终端页/路由）+ 侧边栏项目树最小版 + 右键启动 → **Phase 2 闭环可演示**。届时 dev 跑通后可请用户做首个视觉 checkpoint。

## 会话 10 — Phase 2 递归 PaneTree 渲染器（2026-05-30，autonomous /loop）

### 选取单元（§2）
- 本轮只做**纯组合渲染层 `PaneTree`**（机械、可 typecheck/build 验证）。把异步**生命周期 reconcile effect**（创建/清理 session）+ **App 接线/启动入口**留下一单元（那里一并解决「projectPath 从哪来」——后端 `resolveLaunch` 对空 projectPath 报错，故需项目树/启动提供真实 cwd）。

### 做了什么（`frontend/src/components/PaneTree/`）
- `PaneTree.tsx`：递归——`split`→`<SplitView>` 包裹各子 `<PaneTree>`，`onDragEnd`→`normalizeSizesToHundred`→`resizePanels(node.id, …)`；`panel`→`<PanelView>`。等价 cc-pane PaneContainer+SplitContainer（D2 消费 Context 非 store）。
- `PanelView.tsx`：`<TabBar>`（活动 Tab，onSelect/onClose/onAdd→useTerminal 回调）+ 内容区（**每 Tab 一个 `<TerminalView sessionId active=visible>`，活动 display:block 其余 none，保持挂载以保留终端状态**；隐藏时 host 0 尺寸→layoutScheduler 自动延后 fit，切回 active-effect flush）。`onAdd` 以活动 Tab 的 project 派生新 Tab（MVP，待启动单元给真实 projectPath）。
- `PaneTree.styles.ts`：SystemStyleObject 接口（同 TabBar 规避 sx 数组坑）；panel 列布局 + content relative + tabPane absolute inset:0。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm build` exit=0；`pnpm test` 32/32 无回归。纯组合，无 workflow（机械）。运行时尚未接 Provider（与其他 widget 同，待 App 接线），编译验证即足。

### commit / 下一步
- commit `daf413f`（PaneTree）。
- **下一单元（接近 Phase 2 闭环）**：① `TerminalProvider` 生命周期 **reconcile effect**——扫 rootPane：对 `projectPath!=='' && sessionId===null` 且未在创建中的 Tab 调 `TerminalService.create`→`setTabSession` 回填（creatingRef 防重复）；对树中已消失的 live session `kill`（knownSessions diff）。空 projectPath 跳过（初始空 Tab 不误启）。② 接进 App：终端页 + 路由（routes.tsx）+ `<TerminalProvider>` 进 Provider 链。③ 启动入口（侧边栏项目树最小版 + 右键启动 / 或临时 toolbar 用后端默认 cwd）。完成后 **dev 跑通 → 请用户做首个视觉 checkpoint**（loop 无法自验视觉）。

## 会话 11 — Phase 2 生命周期 reconcile effect（2026-05-30，autonomous /loop）

### 做了什么
- `sessionReconcile.ts`（纯，可测）：`collectTabsWithPane`（扁平 (paneId,tab)）/ `tabsNeedingSession`（projectPath≠'' && session===null && 不在创建中）/ `treeSessionIds` / `orphanedSessionIds`（集合 diff）。+ `sessionReconcile.test.ts` 5 测。
- `TerminalContext.tsx` 加 reconcile effect（deps=[rootPane]）：① 为缺会话的 Tab `TerminalService.create`→resolve 时**用 rootPaneRef 二次校验 tab 仍在且仍缺 session**（在→`setTabSession` 回填；不在→`kill` 防「创建途中关闭」泄漏）；② 孤儿清理 `kill(knownRef − currentTreeSessions)`，`knownRef=currentTreeSessions`。`creatingRef` 防重复创建；空 projectPath 跳过（初始空 Tab 不误启）。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm test` **37/37**（+sessionReconcile 5）；`pnpm build` exit=0。

### race/leak 审查（workflow wf_651d3b47-e99，单 agent）✅ 0 真实 bug
- 逐一精确事件序裁定 6 项全 benign：① StrictMode 双挂载——creatingRef 跨「无 cleanup 的 reconcile effect」存活 → 阻断双创建；aliveRef 同步翻回 true 后 promise 才 resolve → 正常回填（**浪费但自愈，非破坏**）。② 孤儿 diff **不会**误杀刚回填的 session（sid 只在「出现在树后」才进 knownRef）。③ 真卸载/创建途中关闭 → else 分支 kill，不泄漏。④ 无紧循环（回填后 sessionId≠null → tabsNeedingSession 排除，到达不动点）。⑤ resolve 读 rootPaneRef（deps-less effect 保鲜）非过期闭包。
- **1 个前瞻性 caveat（已就地标 TODO 注释）**：resolve 用**捕获的 t.paneId** 回填，当前安全仅因 reducer 未实现 moveTab（Tab 不跨 pane 迁移）；**移植 moveTab 时**须改为按 tabId 查当前 pane 再回填，否则落旧 paneId 会 no-op 泄漏 session。

### commit / 下一步
- commit `06a3720`（reconcile effect + 评审 TODO 注释）。
- **下一单元（到达 Phase 2 闭环）**：接进 App —— ① `<TerminalProvider>` 进 App.tsx Provider 链；② 新建终端页（`pages/<TerminalPage>/`，渲染 `PaneTree` 消费 rootPane）+ routes.tsx 注册（slot primary，图标 MUI Rounded）；③ MVP 启动入口：页内「打开文件夹」动作走 `NativeDialogs.openDirectory()`→`newTab(activePane, {projectPath:dir, cliTool:'none'})`→reconcile 起裸 shell。完成后 **dev 跑通 → 请用户做首个视觉 checkpoint**（验证：打开文件夹→见 shell 输出→输入回显→分屏拖拽→关标签回收 PTY）。侧边栏项目树/右键启动/CLI 工具选择留 Phase 3。

## 会话 12 — Phase 2 接进 App（终端可运行闭环）（2026-05-30，autonomous /loop）

### 做了什么（整合到可运行）
- `pages/TerminalPage/`：View（顶部工具条「打开文件夹」+ `<PaneTree node={rootPane}>`）/ VM（`openFolder(title)`→`NativeDialogs.openDirectory`→`newTab(activePane,{projectPath:dir,cliTool:'none'})`，dialogTitle 由 View 传，遵 MVVM）/ Style / skeleton / lang（`terminalPage.*` + 公用 locale 加 `route.terminal`）/ index。
- `routes.tsx`：terminal 路由（slot primary，**lazy**——xterm 拆出主 bundle，**keepAlive**——保活 xterm 实例与滚动回溯）+ MUI `TerminalRounded` 图标 + skeleton fallback。
- `App.tsx`：`<TerminalProvider>` 入 Provider 链（I18n>Theme>Pref>**Terminal**>Router，全局跨路由保活；reconcile effect 随之全局生效，初始空 Tab projectPath='' 不会误启）+ `registerTerminalPageLocales()`。

### 验证（§4 前端门）
- `pnpm typecheck` exit=0；`pnpm build` exit=0 —— **xterm 成功 lazy 拆分**：`TerminalPage-*.js` 354KB（按需）+ `TerminalPage-*.css` 3.9KB（xterm.css），主 bundle 856KB（MUI 为主，>500KB 警告留 Phase 6 code-split）；`pnpm test` 37/37 无回归。

### Phase 2 状态：**结构完成、可运行，待首个视觉 checkpoint**
- 后端 service + 状态层(reducer/Context+reconcile) + TerminalView/SplitView/TabBar + PaneTree + App 接线 + 启动入口 **全部就位且编译/单测/构建全绿**。
- **但全部前端从未在真实 app 跑过**（loop 仅能 typecheck/build/单测 + 对抗审查，无法自验视觉/运行时）。**遂在此里程碑停 loop，交还用户做首个视觉 checkpoint**——这是 ~10 个前端单元后第一个可运行点，验证整条终端链路（事件桥 terminal:output 投递 / xterm 在 Wails webview 渲染 / reconcile 起会话 / 拖拽分屏 / 关标签回收 PTY）是否如设计工作，再在其上建 Phase 3 才稳妥。

### commit / 下一步
- commit `44b9116`（App 接线，可运行）。
- **停 loop（不调度下一轮）**，请用户验证。验证 OK → 重新发 `/loop` 续到 Phase 3（管理面/设置/i18n/窗口形态）；若发现运行时问题 → 反馈，下一轮修。
- 续跑后下一单元候选：依验证结果定（修问题 优先）；否则 Phase 3 起步 = Workspace/Project 后端 service + 侧边栏项目树（替换临时「打开文件夹」工具条）+ CLI 工具选择启动。

## 会话 13 — Phase 3 起步：后端扁平 project service（2026-05-30，autonomous /loop）

### 入场
- 用户重发 `/loop`（未显式回报 Phase 2 视觉验证；按既定协议「重发 /loop = 续到 Phase 3」推进）。Phase 2 仍 🟡 待视觉验证——本单元是**纯 Go 后端**，与 Phase 2 视觉层正交，即便 Phase 2 有运行时问题也不在其上复利，故安全推进。
- 收口工作树 `M go.mod/go.sum`：是 findings J6 预告的「联网 tidy」（go-pty/uuid 转直接依赖 + go-pty test 依赖校验和 + x/crypto·sys·term 小升），benign，go build/vet 验证后单独 `chore` 提交 `8ffa23d` 复位干净树。

### 选取单元（§2 后端→bindings→service→Context→View 序的最小首增量）
- Phase 3 首单元 = **后端扁平 `project` CRUD service**。先跑尽调+规格 workflow `wf_79a917da-67a`（3 并行全源阅读 cc-pane project/workspace 后端 + 前端消费面 → 综合 Go 移植规格），动代码前消除「workspace 是什么」未知。

### 关键发现（→ findings J8，含决策门 D9）
- cc-pane **两套并行 project**：扁平 Project（`projects` 表，本单元移植）+ WorkspaceProject（内嵌 workspace.json，live 侧边栏实际用）。
- **Workspace 是文件存储不是 SQLite 表**（`<data_dir>/workspaces/<name>/workspace.json`，目录名=身份 + 文件监视 WORKSPACES_CHANGED）。→ **DEFER**（违 XK-Pane SQLite 铁律；忠实移植=文件存储 vs 原生=GORM 表 = **D9 决策门，到 workspace 单元停问用户**）。MVP 侧边栏先走扁平 project 树。

### 做了什么（`internal/services/project/`）
- `project.go`：Service{holder} + List/Add/Get/Remove/UpdateName/UpdateAlias + 非导出 existsByPath + isUniqueViolation 兜底。
- `validate.go`：validatePath（empty→traversal→absolute，结构化码 PATH_EMPTY/TRAVERSAL/NOT_ABSOLUTE）+ deriveName（basename，"unnamed" 回退）。
- `project_test.go`：**19 守门测试**逐条锁保真铁律——path 原样存/逐字节去重（含带尾分隔符算不同项目）/非 upsert/created_at DESC/空→非 nil []/alias trim→NULL（nil·空·空白三态）/name basename·"unnamed"/UpdateName 存未 trim·非对称/affected-rows==0→"Project does not exist"/Get miss→nil/validate 码。
- 接进 `app.go`（`application.NewService(project.New(holder))`，纯 CRUD 无 AttachApp）。

### 验证（§4 全绿）
- `go build`/`go vet`/`go test`（dev+prod）+ 全量 `go test ./internal/...` 全 exit=0；`wails3 generate bindings`=**7 Services/34 Methods**（existsByPath 非导出**未绑**，确认外科手术式绑定面）/Project 模型新增；`pnpm typecheck` exit=0；`pnpm test` 37/37 无回归。
- **踩坑**：`go build ./...` 含 build/ios target（Win 上 `main is undeclared`，预存无关）→ 用 `./internal/...`；validate.go 的 switch 重复 case（Win 上 `string(filepath.Separator)`=="\\" 撞字面 "\\"）→ 去冗余 case。

### commit / 下一步
- commit `8ffa23d`（chore tidy）+ `c8df705`（project service）。
- 对抗保真审查 workflow **`wf_7bc251b0-c1f`** 后台跑（2 lens：cc-pane 保真 + GORM/SQLite 对抗——重点 UpdateAlias nil→NULL 绑定 / no-op update RowsAffected / UNIQUE 串匹配 / DESC 排序），refute-by-default 校验。**下一轮**：读审查结论，有真 bug 先修；否则下一单元 = **前端 `ProjectService.ts` + `ProjectContext`（弃 Zustand）+ 侧边栏扁平项目树**（替换 TerminalPage 临时「打开文件夹」工具条）+ CLI 工具选择启动。
- ⚠️ 仍欠用户的 **Phase 2 视觉 checkpoint**：后端推进不依赖它，但前端 project 单元接侧边栏时会与终端链路交汇——届时视觉验证价值更高，可一并请用户验。

## 会话 14 — project service 对抗审查收口（UTC 时间戳修复）（2026-05-30，autonomous /loop）

### 审查结论（workflow `wf_7bc251b0-c1f`，2 lens + refute-by-default 校验）
- **1 真实 bug（low，已修）**：`created_at` 排序可跨时区倒挂。GORM 默认 NowFunc 返回本地时区，glebarez 把 time.Time 按其自身偏移序列化成 RFC3339 TEXT；SQLite 对 created_at 字典序排序 → 跨 DST/换时区两行偏移不同时，绝对更新的行可能排到更旧行之后（破坏 List「最新在前」），且偏离 cc-pane（`chrono::Utc::now`）。校验 agent 用真 SQLite 两路复现确认。
- **1 驳回（isReal=false）**：「同 tick 快速 Add 无 tie-break」——校验确认是**忠实移植**（cc-pane 同样 `ORDER BY created_at DESC` 无次级键；加 `id DESC` 反而是 drift）。与我 spec 的风险注解一致，不改。

### 修复（`internal/storage/db.go`，一行 + 回归测试）
- gorm.Config 加 `NowFunc: func() time.Time { return time.Now().UTC() }`——全模型 CreatedAt/UpdatedAt 统一 UTC，字典序==绝对时序，且与 cc-pane 一致。读路径仍能解析旧的带偏移文本（time.ParseInLocation）。
- `internal/storage/timestamp_test.go`：建一行（自动填 CreatedAt）→ 读原始 created_at TEXT，断言带 UTC 标记（"Z"/"+00:00"）。**修复前后实证**：存储文本从本地偏移翻成 `...Z`（测试断言里我先按审查给的 "+00:00" 格式写错，实测此 glebarez 版本用 RFC3339 'T'…'Z'，改断言接受两者）。

### 验证 / commit / 下一步
- `go build`/`go vet`/`go test`（dev+prod）+ 全量 + project 测全绿；storage 内部改动不触 binding 面，无需 regen/typecheck。commit `b7ac0e2`。
- **project 后端单元彻底收口**（实现 + 19 测 + 对抗审查 + 修 1 bug）。**下一单元** = 前端 `ProjectService.ts`（包 7 Services/34 Methods 里的 project：List/Add/Get/Remove/UpdateName/UpdateAlias）——作为前端 project 三段（Service→Context→侧边栏树）的第一段。

## 会话 15 — Phase 3 前端 ProjectService.ts（2026-05-30，autonomous /loop）

### 做了什么（前端 project 三段之第一段：Service）
- `frontend/src/services/project/ProjectService.ts`：包 project bindings 6 方法（list/add/get/remove/updateName/updateAlias）。定义对外纯接口 `Project{id,name,path,alias:string|null,createdAt,updatedAt}`（与生成类解耦，供 ViewModel 消费）+ `toProject` 规整（生成模型 `time.Time`→`any`→string；`alias` string|null|undefined→null 兜底）。
- 范式对齐既有 TerminalService/PreferencesService：`import { Service as Binding }` + 纯对象 + async 透传。`init_project_history`（cc-pane add 后的 Local History 文件监视 best-effort 副作用）**不接**——独立子系统留 Phase 4/5（service 头注明）。
- `services/index.ts` barrel 追加 `ProjectService` + `type Project`。

### 验证 / commit / 下一步
- `pnpm typecheck` exit=0；`pnpm test` 37/37 无回归。纯薄包装透传层（同 TerminalService），typecheck+test 即足，无 workflow。commit `2571ee0`。
- **下一单元** = `ProjectContext`（Provider+state，弃 Zustand；list/add/remove/rename/alias 动作 + 启动时拉取 + 乐观更新或拉取刷新），第二段。之后第三段 = 侧边栏扁平项目树（替换 TerminalPage 临时「打开文件夹」工具条）+ 选 CLI 启动。

## 会话 16 — Phase 3 ProjectProvider（project 三段之第二段：Context）（2026-05-30，autonomous /loop）

### 做了什么
- `frontend/src/project/{ProjectProvider.tsx,types.ts,index.ts}`：`<ProjectProvider>` + `useProjects()`，范式同 PreferencesProvider（useState + aliveRef + mount 异步拉取，首屏空列表不阻塞）。动作集移植 cc-pane useProjectsStore：`refresh/addProject/removeProject/renameProject/setProjectAlias`（addProject 返回新建记录供调用方选中/启动）。
- **MVP 取舍（简洁优先，已在 types.ts 注明）**：① **mutation 后 reload 刷新**（与后端 created_at DESC 排序 + alias trim 归一保持一致，不乐观拼接以免与后端语义分裂）；② **错误向上抛**（add 重复路径→"Project already exists" 等由 View 用 NativeDialogs 提示，Provider 不吞）；③ **省略 selectedId**（cc-pane store 有，但属投机全局态——侧边栏高亮/启动用本地态或终端活动 tab 派生即可，待消费方需要再加）。
- 接进 App 链：I18n>Theme>Pref>Terminal>**Project**>Router（全局，侧边栏消费）。

### 验证 / commit / 下一步
- `pnpm typecheck` exit=0；`pnpm test` 37/37。Provider 逻辑薄（service 调用 + refetch），范式同已审 PreferencesProvider，无 workflow。commit `c81a58f`。
- **下一单元（project 第三段，较重）** = 侧边栏扁平项目树 **View**：列项目 + 「添加项目」（NativeDialogs.openDirectory→addProject）+ 每项启动（点项目→选 CLI→TerminalProvider.newTab(projectPath)）+ 删除/改名/别名。**布局决策（次轮定）**：foundation 的 AppLayout 是「图标栏 Sidebar + RouterOutlet」无次级面板槽——拟把项目面板做进 **TerminalPage 左列**（替换临时「打开文件夹」工具条，不动 AppLayout，最小侵入），右列仍 PaneTree。
- **该单元落地后 = 强视觉 checkpoint 时机**：项目列表→点击→选 CLI→终端起，一条完整可视链路（兼顾欠用户的 Phase 2 视觉验证）。届时拟暂停 loop 请用户验。

## 会话 17 — Phase 3 ProjectPanel + 终端页整合（project 三段之第三段 3a）（2026-05-30，autonomous /loop）

### 做了什么（完整可视链路落地）
- `components/ProjectPanel/`（自包含 View/VM/Style/lang）：列项目（useProjects）+ 头部「添加」（NativeDialogs.openDirectory→addProject）+ 点行 `onLaunchProject` + 移除（NativeDialogs.confirm，文案说明「仅从列表移除不删盘」）。显示名 = alias||name；后端错误经 NativeDialogs.error 提示（VM 从 View 取串，遵 MVVM）。自带 `projectPanel.*` locale + register。
- `TerminalPage`：临时「打开文件夹」工具条**替换**为左列 ProjectPanel（root 改 row 布局）；`useTerminalPage` 改 `launchProject(project)`→newTab({projectPath, title})，**MVP 裸 shell（cliTool 'none'）**，选 CLI 留 3b。
- **清理（§3 我的改动产生的孤儿）**：删 `terminalPage.*` lang 三件 + 其 register（grep 确认仅工具条/App/index 引用），App.tsx 改注册 `registerProjectPanelLocales`。

### 验证 / commit
- `pnpm typecheck` + `pnpm test` 37/37 + `pnpm build`（ProjectPanel 入 TerminalPage 懒块 353KB）全绿。纯展示+透传+已审 Provider，无 workflow。commit `fd653fa`。

### 决策：continue（不停 loop）+ 醒目标注 checkpoint 机会
- **现状**：项目面板→点击→终端起 的完整可视链路已就位，是**强视觉 checkpoint 时机**（兼顾欠用户的 Phase 2 视觉验证）。
- **但不停 loop**：用户既定 stop 粒度 = 「连续跑到底」（仅 ①决策门 ②连续失败 ③全阶段完成 才停）；且用户在会话12 checkpoint 后**已重发 /loop 续跑、其后 5 轮未干预**——demonstrated「持续推进」偏好。视觉 checkpoint 非三类停止条件之一。故**继续**，仅在消息+此处醒目标注：随时可 `wails3 dev` 验证「点项目→终端起/回显/分屏/关标签回收」，发现问题反馈即下一轮修。
- **下一单元（候选，次轮定）**：倾向**转后端可测单元**以平衡「未运行验证的前端」堆积——`launch_history` 记录（点启动→写一条启动历史，后端 service + 守门测试）或 Provider 管理 service（后端 CRUD + cryptox 加密 api_key）。或 3b（选 CLI 工具启动，前端为主）。下一轮按新鲜判断选。

## 会话 18 — Phase 3 后端 launch_history service（2026-05-30，autonomous /loop）

### 选取理由
- 头条能力（选 CLI 启动）**依赖尚未视觉验证的终端渲染**，在其上叠 3b 风险大 → 本轮选**不依赖终端渲染、可测**的后端单元。`launch_history` 复用既有模型（Phase 1）+ 补足刚建的启动流的写路径，优于需新模型+调查的 Provider。

### 做了什么（`internal/services/launchhistory/`）
- `launchhistory.go`：`Service{holder}` + `Record(spec)→row` + `List(limit)` 倒序。cc-pane LaunchHistoryService 的 MVP 子集——会话恢复方法（update_session_started/touch/last_prompt/find_by_pty）属 Phase 4，不含。
- **要点**：路径铁律 D7（ProjectPath=cwd，WorkspacePath 可选）；`LaunchedAt = time.Now().UTC()`——**手填排序字段，主动套用审查刚教训的 UTC 排序教训**（否则跨时区字典序倒挂）；`last_prompt` cryptox 仍延后（MVP 启动无 prompt）；CliTool 缺省 none / RuntimeKind 缺省 local。
- `launchhistory_test.go`：**7 守门测试**（创建/默认值/launched_at 存 UTC/workspace 可选/launched_at DESC 排序/limit/空→非 nil []）。接进 app.go（纯 CRUD）。

### 验证 / commit
- `go build`/`go vet`/`go test`（dev+prod）+ 全量全绿；`wails3 generate bindings`=**8 Services/36 Methods**（+Record/List，+RecordSpec 模型）；`pnpm typecheck` exit=0。纯 CRUD + 复用已验证 storage 范式 + 主动套 UTC 教训，无 workflow。commit `8869111`。

### 下一单元
- **先薄前端接线**：`LaunchHistoryService.ts`（包 Record/List）+ 在 `launchProject` 里 record-on-launch（让 launch_history 写路径真正被调用；record 调用独立于终端渲染，低风险）。
- **之后转 Provider 管理 service**（后端，**首次用 cryptox** 加密 api_key，头条「选定 Provider」路径）——较大，需先跑 Provider spec 调查 workflow（同 project 范式：cc-pane provider 模型/加密/启动 env 注入）。
- 前端视觉验证债仍挂（用户「连续跑到底」、未干预）——继续推进可测后端，用户回来随时验。

## 会话 19 — launch_history 前端接线 + 启动 Provider spec 调查（2026-05-30，autonomous /loop）

### 做了什么
- **launch_history 写路径接线（薄）**：`services/launchhistory/LaunchHistoryService.ts`（包 Record/List，纯 RecordSpec/LaunchRecord 接口）+ `useTerminalPage.launchProject` 内 record-on-launch（**fire-and-forget**：`.catch(()=>{})`，记录失败绝不阻断终端启动）。barrel 追加。让 launch_history 后端真正被调用；「最近启动」UI 消费方留后。
- 验证：`pnpm typecheck` + `pnpm test` 37/37 全绿。薄透传，无 workflow。commit `6c187e9`。

### 下一单元：Provider 管理 service（后端，首次 cryptox）
- 本轮**后台启动 Provider spec 调查 workflow**（同 project 范式）：并行全源阅读 cc-pane provider 模型/repo/service/commands + 启动 env 注入（provider→CLI env，交叉对 XK-Pane cliadapters 既有 Provider 类型）+ 前端 provider 消费面 → 综合 Go 移植规格（GORM 模型含 cryptox 加密 apiKey / service CRUD / 启动 env 映射 / MVP 范围）。
- D8 铁律已定：apiKey 落库**走 cryptox 加密**（非 keyring）。下一轮读 spec → 实现（首次落地 cryptox BeforeSave/AfterFind 钩子模式）。

## 会话 20 — Phase 3 后端 provider service（首次 cryptox）（2026-05-30，autonomous /loop）

### spec 调查结论（wf_a31b9406-129）→ 决策（详见 findings J9）
- cc-pane providers 是**明文 providers.json 文件**（无表无加密）。XK-Pane 按 D8 落 SQLite + cryptox 加密 apiKey。**spec 抛出的多个决策门均有原则性默认**（与本仓约定 + D8 对齐）→ 不阻塞 loop，**决策 + 文档 + 可否决**：脱敏 DTO（明文 key 绝不跨 IPC）/ Update patch 保留 key 且不动 default / 服务端 UUID id / SetDefault 未知 id 报错。
- **service 层加解密**（非 GORM 钩子）：读返回脱敏 DTO 无需解密、写 patch 保留；`Provider.APIKey` 恒密文（json:"-"）。

### 做了什么
- `storage.Provider` 模型（providers 表，Clearable:false，apiKey json:"-"）+ AllModels。
- `internal/services/provider`：List/Get/Add/Update/Remove/SetDefault（脱敏 DTO + Input；cryptox 加密；default 不变式走事务）。`GetEnvVars`/`ToCliProvider`（暴露明文 env）**延后启动接线单元**（绝不进 bindings）。
- **15 守门测试**：default 不变式（首个强制/加默认清其它/删默认提升最早/SetDefault）+ **cryptox 加密落库 round-trip**（raw SELECT 非明文、DecryptString 还原）+ 脱敏 + patch 保留 key + 不动 default + 各 not-found。

### 踩坑（实测捕获）
- `TestRemovePromotesOldestRemaining` **dev 过 / prod 挂**——`Order("created_at")` 在同 tick 多次 Add 时间戳撞 → 非确定。改 **`Order("rowid")`（插入序）** = cc-pane providers[0]，3×dev+3×prod 稳定。再次印证 created_at 排序教训。

### 验证 / commit / 下一步
- `go build`/`go vet`/`go test`（dev+prod，-count=3）+ 全量全绿；`wails3 generate bindings`=**9 Services/42 Methods**；`pnpm typecheck` exit=0。commit `c86bc60`。
- 后台启动**安全+保真对抗审查** workflow `wf_4dfb9634-31c`（2 lens：fidelity/deviations + security/cryptox/GORM——重点「明文 key 是否可能泄漏到前端」），refute-by-default。**下一轮**：读审查结论，有真 bug 先修。
- 之后候选：前端 ProviderService.ts + provider 设置子页（按脱敏 DTO 契约设计）；或**启动接线单元**（GetEnvVars + resolveLaunch 设 Provider + buildEnv 合 provider env，让「选定 Provider 启动」头条能力真正打通）。

## 会话 21 — provider 审查收口 + provider.EnvVars（2026-05-30，autonomous /loop）

### 审查结论（wf_4dfb9634-31c）：clean
- **1 finding，0 真实**。唯一 finding（get_default_provider 未移植）自评 nit，校验确认是**有意范围决策**（它是暴露明文的启动侧 helper，移植现在要么死代码要么违反脱敏偏离）→ 不改。provider service 安全+保真**全绿**。

### 做了什么：provider.EnvVars（启动接线的 Tier-A 核心）
- `internal/services/provider/envvars.go`：`EnvVars(ctx,db,id)→map[string]string` 复刻 cc-pane to_env_vars——解密 apiKey 后按 11 种 ProviderType 产出启动 env（每变量仅源字段非空才写）。**包级函数非 Service 方法 → 不进 bindings**（暴露明文，仅后端启动路径调）。id""/未知→空 map；解密失败→报错。codex 易错点保留（open_ai→CODEX_API_KEY）；config_profile 仅 dir-mode。
- **8 测**（含 加密落库→解密注入 完整往返、bedrock 无 key、open_ai vs opencode key 名、空字段省略）。dev+prod 全绿（~30s：每测 storage.Open AutoMigrate + cryptox 取密钥）。EnvVars 非绑定 → 无需 regen bindings。commit `9dde34b`。

### 下一单元：终端启动接线（打通「选定 Provider 启动」）
- 改 `terminal.CreateSpec` 加 `providerId`；给 terminal 服务注入「provider env 解析器」（holder 或 resolver 函数）；`resolveLaunch` 据 providerId 设 `cctx.Provider`（ToCliProvider 字段拷贝——在 terminal 包做，避 provider→cliadapters 耦合）；`buildEnv` 合并 `provider.EnvVars`（中央 Tier-A 在 adapter EnvInject 之前，Tier-B 覆盖）。+ 守门测试（给定 providerId，PTY env 含对应 provider 变量）。需读 terminal.go（会话2 代码）后改。**纯后端可测**，仍不依赖终端渲染。

## 会话 22 — Phase 3 终端启动接线：打通「选定 Provider 启动」（2026-05-30，autonomous /loop）

### 做了什么（provider → PTY 启动 env 端到端后端打通，头条能力）
- `provider`：`ResolveLaunchEnv(ctx,db,id)→(Tier-A env, *cliadapters.Provider, err)`——一次加载+解密，产出中央 env + Tier-B adapter Provider 上下文（含明文 key）。非绑定；provider→cliadapters 单向无环。envvars.go 重构出 loadDecrypted/envVarsFor。决定把 ToCliProvider 放 provider 包（返回 *cliadapters.Provider，单次加载，避双解密；耦合无环，优于会话21 设想的「terminal 侧拷贝」）。
- `terminal`：`CreateSpec.ProviderID`；注入式 `ProviderResolver`（SetProviderResolver，nil 安全）；`resolveLaunch` 解析 provider → Tier-A providerEnv + Tier-B `cctx.Provider`；`buildEnv` 在 adapter EnvInject **之前**合并 providerEnv（cc-pane 顺序，adapter 覆盖冲突键）。裸 shell+provider 仍得 Tier-A env；resolver 报错→启动报错。**明文 key 不入日志（只记 command/cwd）、不入错误串**。
- `app.go`：SetProviderResolver closure → provider.ResolveLaunchEnv（holder 取活跃 DB）。前端 `TerminalService` CreateSpec.providerId 接通（缺省 ''）。

### 验证 / commit
- go build/vet/test（dev+prod）+ 全量全绿；bindings **9 Services/43 Methods**；typecheck + 前端 37 测全绿。+4 terminal 测（fake resolver Tier-A 透传+合并/resolver 报错）+2 provider 测（ResolveLaunchEnv）；既有 buildEnv/resolveLaunch 测随签名更新。commit `0114099`。

### 下一步
- 后台起**聚焦安全审查**（单 agent，查新接线明文泄漏路径 + Tier-A/B 合并顺序保真），下一轮读结论，有真问题先修。
- 之后候选：会话恢复（terminal_sessions）/ i18n 命名空间 / 设置页其它子页（皆后端或可测）；前端 ProviderService.ts + 启动选 Provider/CLI 的 UI 依赖未验证终端渲染，宜与视觉 checkpoint 一并。

## 会话 23 — provider 接线安全审查 clean + 前端 ProviderService.ts（2026-05-30，autonomous /loop）

### 安全审查（聚焦 agent）：CLEAN
- 5 点全 OK，0 真实问题。明文 key 仅在非绑定后端启动路径（resolver→buildEnv→PTY），绑定面只回脱敏 DTO；日志只记 command/cwd（不记 env）；错误串不含 key；Tier-A/B 合并顺序与 cc-pane 一致；provider→cliadapters 无环。**1 良性非 finding**：`CC_PANES_PTY_SESSION_ID` 在 providerEnv 合并之前设（cc-pane 在之后），但 provider 键集与 CC_PANES_*/TERM 不相交 → 不可能被覆盖，按简洁不改。

### 做了什么：前端 ProviderService.ts（薄）
- `services/provider/ProviderService.ts`：包 list/get/add/update/remove/setDefault。脱敏 `Provider`（hasApiKey 无明文）+ `ProviderInput`（明文 apiKey，编辑留空保留）接口。明文不回前端（守 J9 契约）。barrel 追加。typecheck + 37 测全绿。commit `4fc8f6d`。

### 下一单元（候选，次轮定）
- **provider 设置子页**（按 SettingsPage 子目录隔离规范，复用**已验证的**设置框架——不依赖未验证的终端渲染；让 provider CRUD 在 UI 可达且用户可验）：list/add/edit/remove/setDefault 表单。需 ProviderContext 或子页本地态。
- 或 **会话恢复后端**（terminal_sessions Save/List service，可测）。
- **注**：已积累 3 个「后端就绪、前端消费方待建」的服务（launch_history/provider/[会话恢复]）+ Phase2 前端从未真机跑。视觉验证债大。settings 子页走已验证框架是低风险的「让后端可达」推进；启动选 Provider/CLI 的 UI（依赖终端渲染）仍宜与视觉 checkpoint 一并。

## 会话 24 — Phase 3 ProviderProvider（provider 三段之 Context）（2026-05-30，autonomous /loop）

### 做了什么
- `frontend/src/provider/{ProviderProvider.tsx,types.ts,index.ts}`：`<ProviderProvider>` + `useProviders()`，**镜像 ProjectProvider**（useState providers/loading + aliveRef + mount 异步拉取；refetch-after-mutation；错误向上抛）。动作：refresh/addProvider/updateProvider/removeProvider/setDefaultProvider。脱敏 Provider 列表（hasApiKey），明文仅 input 上行。
- 接进 App 链：…>Project>**Provider**>Router（全局，设置子页 + 后续启动选 Provider 共同消费）。
- 验证：typecheck + 37 测全绿。镜像已审 ProjectProvider 范式，无 workflow。commit `cd86d7b`。

### 下一单元：provider 设置子页（走已验证设置框架）
- `SettingsPage/provider/`（按子目录隔离规范：View/VM/Style/lang/index + register + useSettingsPage items 加项 + SettingsPage.tsx 分发）。列表（useProviders）+ 新增/编辑表单（name/type 下拉/apiKey/baseUrl + 按 type 显条件字段 region/projectId/awsProfile/configDir）+ 删除（确认）+ 设默认。编辑时 apiKey 占位「留空保持不变」（脱敏契约）。**不依赖终端渲染**——让 provider CRUD 在 UI 可达可验。可能拆「列表+CRUD 动作」与「表单」两单元。

## 会话 25 — Phase 3 provider 设置子页（provider 管理 UI 可达可验）（2026-05-30，autonomous /loop）

### 做了什么
- `SettingsPage/provider/`（按子目录隔离规范，组件名 `ProviderSettings` 避与 Provider 类型混淆）：列表（useProviders；name + 类型 chip + 默认 chip + 设密钥提示 + 编辑/删除/设默认行内动作）+ 新增/编辑 **MUI Dialog 表单**（name / type 下拉 11 类 / apiKey password「编辑留空保留」/ baseUrl / 仅新增显 isDefault 开关——因后端 Update 不动 default，编辑用行内「设默认」动作，**与后端语义一致**）+ 删除（NativeDialogs 原生确认）。全字符串走 t()，图标 *Rounded。
- 接线：useSettingsPage items 加 provider 项（在 database 前）+ SettingsPage.tsx 分发 + SettingsPage/lang/index 注册 registerProviderSettingsLocales + settings.list.provider 条目（zh/en）。lang 命名空间 `settings.provider.*`。
- **范围**：MVP 表单 = 通用字段（name/type/apiKey/baseUrl/isDefault）；条件云字段（region/projectId/awsProfile/configDir）= 3b（draft 已携带，仅缺渲染）。

### 验证 / commit
- typecheck + 37 测 + build 全绿（provider 子页入 SettingsPage 懒块）。标准 MUI 表单走已验证设置框架 + 无复杂逻辑，无 workflow。commit `ff99674`。
- **里程碑**：provider 管理成为**首个完整、用户可达可验的 Phase-3 管理 UI**（设置→接入 Provider→增删改/设默认），且全在已验证的设置框架上（不依赖未验证终端渲染）。「选定 Provider 启动」后端已通，仅差启动时选 Provider 的 UI（依赖终端渲染 → 视觉 checkpoint 一并）。

### 下一单元（候选）
- 3b provider 条件云字段（小，补表单渲染）；或会话恢复后端（terminal_sessions Save/List service，可测）；或 i18n 命名空间梳理。启动选 Provider/CLI UI 仍待视觉 checkpoint。

## 会话 27 — Phase 4 起步：gitx 只读包（Git unit 1）（2026-05-30，autonomous /loop）

### 决策窗口 + spec 结论
- 上轮 Git spec workflow `wf_93487368-df5`（只读）留了用户转向窗口；用户未干预。spec **确认 D4**（无决策门）：cc-pane git 是 **100% CLI**、范围仅 12 命令（branch/dirty-bit/clone/worktree/pull·push·stash），**无** diff/log/commit/stage（终端手动做）。go-git/v5 v5.19.1 **已随 Wails indirect 引入**——加它只是 indirect→direct 零新依赖。→ 实现 gitx 只读核心。

### 做了什么（`internal/utils/gitx`）
- `gitx.go`：Open/IsRepo + CurrentBranch（detached→"HEAD"，空仓→""，对齐 cc-pane Option）/ IsDirty / Branches / Log。go-git 原生 branch/log；**IsDirty 走 CLI `status --porcelain`**（忠实 cc-pane + 避开 go-git Worktree.Status() 大仓性能热点 #181/#844，使 CLI 回退 load-bearing 非死码）。
- `cli.go`：`runGit` 经 procx + **只读子命令 allowlist**（status/rev-parse/branch/log/for-each-ref）+ GIT_TERMINAL_PROMPT=0；非零退出→stderr 错误，spawn 失败→透传。
- `gitx_test.go`：**4 测**对真临时 repo（夹具用真 git CLI 经 procx 建，独立于被测 go-git；skip-if-no-git）——branch/branches/log/detached、dirty bit（clean/改/提交/未跟踪 走 CLI 路）、空仓、非仓（go-git 无需 git 二进制）。
- go.mod：go-git/v5 indirect→direct（go mod tidy，go.sum 不变）。

### 验证 / commit
- go build/vet/test（dev+prod）+ 全量全绿。无 bindings（内部 util，git service+面板留后续 unit）、无前端、无 workflow（spec 已调查 + go-git/CLI 直白且真 repo 测充分）。commit `bf279a6`。

### 下一单元（候选）
- Git 后续 unit（git 面板：结构化 status+stage/commit/diff / network / worktree·stash）——面板部分依赖视觉验证；或 **Local History 后端**（file_versions 快照，独立 history.db，可测）；或会话恢复后端。Git network/worktree 后端可测不依赖渲染。**视觉验证债持续**：终端栈 + 项目面板从未真机跑，建议用户 `wails3 dev` 验。

### ⏸ LOOP 暂停于此（会话27 末，2026-05-30）
- **暂停理由**（非失败、非全完成）：连续 ~15 轮已交付 Phase 0-3 全部 + provider 管理 UI（可达可验）+ git 只读包，量大且全部 tested/committed/文档化。但**余下所有高价值工作都被「用户验证/方向」门控**：① 几乎所有面向用户的下一步是「依赖未验证终端渲染的前端」；② 其余可测后端（git network/worktree/panel、Local History、会话恢复）都是「后端就绪、前端消费方待建」——即时用户价值递减；③ 大新子系统（Local History）或**用户强调的早期子系统（Bedrock/MCPE/notify/memory，现排 Phase 5）**涉及方向选择。最高杠杆动作 = 用户 2 分钟视觉验证 + 方向指示，我做不了。
- **续跑路径**：用户随时重发 `/loop` 即继续（默认按计划走 Git network/worktree → Local History；或用户指定方向，如优先早期子系统）。所有状态在本文件 + task_plan + findings，resume 无缝。
- **欠用户验证**：`wails3 dev` → 终端页（项目面板「+」开目录→点项目→shell 起/回显/关标签回收 PTY）+ 设置→接入 Provider（增删改/设默认）。发现问题反馈，下一轮优先修。

## 会话 28 — ▶ LOOP 恢复：用户真机验证暴露 2 个 Windows 真 bug（2026-05-30，/loop 续）

### 触发：用户做了视觉验证（兑现暂停 note 的「续跑路径」），报告 provider 删除链路
1. **「provider 删除→弹确认框→点是后删不掉」**（前端 confirm 失效）。
2. 修复后用户贴出后端日志 `provider.go:213 record not found / rows:0`（误以为是错误）。

### Bug 1：`NativeDialogs.confirm` 在 Windows 上永不返回 true（commit `6505173`）
- **根因（读 Wails v3 alpha.96 源码全平台坐实）**：`dialogs_windows.go` 的 Question 对话框硬编码 `MB_YESNO`，**忽略**自定义按钮 label，只显示系统本地化「是/否」；且仅当某按钮 `Label` 字面等于映射回的 `"Yes"/"No"` 才触发其回调（`messageprocessor_dialog.go` 靠该回调把 label 推入 resp 通道）。我们传 `"删除"/"取消"` → 无匹配 → resp 收不到值 → **前端 Promise 永久挂起**（表现为点是后毫无反应）。mac(NSAlert)/linux(GTK) 原生支持自定义 label，回传 label 本身，故无此问题。
- **修复**：`confirm` 内平台分支——`isWindows` 时用 `"Yes"/"No"` 作 label（反正不可见）并以 `choice==="Yes"` 判定；mac/linux 保留自定义 label。**集中一处修好全部 4 个 confirm 调用点**（删项目 / 删 provider / 数据库重置 / 逐表清空——这些之前在 Windows 上全坏）。verify：pnpm typecheck + build 通过。
- **教训写 findings J10**：Wails v3 alpha 的 confirm 在 Windows 是 native Yes/No、自定义按钮文案不可渲染、且 label 不匹配会挂起——任何走 `NativeDialogs.confirm` 的新代码不要依赖自定义按钮文案语义。

### Bug 2（实为噪声非错误）：GORM 把 ErrRecordNotFound 当 error 打日志（commit `97d44f3`）
- 那条 `provider.go:213` 日志其实**证明 Bug1 修复生效、删除已成功**：Remove 删掉默认 provider 后试图提升「最早剩余项」为新默认、但已无剩余 → 命中 `errors.Is(err, ErrRecordNotFound)→return nil` 正常分支。
- ErrRecordNotFound 在本项目是「查不到」的正常控制流（Get 类 First+errors.Is→nil；Remove 提升默认）。`db.go` 给 GORM logger 开 `IgnoreRecordNotFoundError:true` 全局静音此类噪声（写端仍 stdout，与原 `logger.Default` 一致）；真错误照常记录，代码处理不受影响。verify：go build/vet/test（dev+prod，storage+provider）全 ok。

### 状态
- 两 commit 在 `rebuild/cc-pane`，未 push。**确认前端 confirm + 后端噪声两类问题已修**。等用户重测删除（应：点「是」即删；删最后一个默认项不再刷红日志）+ 反馈其余验证项（项目面板开目录/点项目/shell 起回显、设置→Provider 其余增改/设默认）。

### ▶ 续：用户确认「删 provider 能正常删了，继续按计划推进」→ Phase 4 Git unit 2（worktree + stash）
- confirm 修复**真机验证通过**（删除可用）。loop 恢复，按计划走 Phase 4 可测 Git 后端。
- 查 cc-pane（`cc-panes-core/.../worktree_service.rs` + `commands/git_commands.rs`）取精确语义 → `internal/utils/gitx` 扩 CLI-only 变更原语：
  - `gitx_worktree.go`：`Worktrees`(list --porcelain 解析,空行分块+末块flush+CRLF容忍) / `WorktreeAdd`(newBranch→`-b`；`GIT_LFS_SKIP_SMUDGE=1`；path 前 `--` 护选项) / `WorktreeRemove`(恒 `--force`)。`WorktreeInfo{path,branch(去 refs/heads/),commit(7位),isMain}`。
  - `gitx_stash.go`：`Stash`(`git stash`) / `StashPop`(`git stash pop`)。
  - `cli.go`：allowlist 追加 worktree·stash；`runGit` 拆出 `runGitEnv`(额外 env 给 LFS)。
- **有意偏离 cc-pane**（已记 commit `c8f6d8c`）：① IsMain = porcelain 首条（git 保证主树列首位；实测 Windows porcelain 路径用正斜杠，cc-pane 的「路径字符串相等」会因分隔符/大小写误判）。② 分组目录策略 `{repo}-worktrees/{name}` + worktree 名校验 + stash 前 Local History auto-label 钩子 → 属上层 UX/历史子系统，留 gitsvc 服务层（本 unit 只出 CLI 原语，同 read-only unit 分层）。
- **针对性加固（代替泛泛审查；ultracode 下做了实证验证）**：实测 `git worktree add` 支持 `--` 终止选项 → 用它护住 path；实测 detached worktree porcelain 用 `detached` 行（无 branch）→ 解析器忽略→branch="" 正确。
- 测试：+`TestWorktrees`(真 repo list/add带新分支/remove) +`TestStash`(改→stash→干净→pop→恢复→空 stash 再 pop 报错) +`TestParseWorktreeList`(纯函数边界:主/linked/detached/末块无尾空行/CRLF)。**7 测** go build/vet/test dev+prod 全绿。commit `c8f6d8c`。
- **下一单元**：network（clone/fetch/push/pull——clone 可对本地源 repo 测；push/pull 需 bare 远端夹具+鉴权，FF/非FF CLI 回退）；或 git 面板后端（结构化 status/stage/commit/diff，net-new 增强，面板 UI 视觉门控）。仍不依赖终端渲染。

### ▶ 续 2：Phase 4 Git unit 3（network）—— 同 tick 连做（autonomous /loop）
- `gitx_network.go`：`Clone`(包级,`git clone [--depth 1] -- url dst`,dst 存在报错,对齐 cc-pane) + `Fetch`(`fetch --all`)/`Pull`/`Push`(*Repo 方法)。allowlist 追加 clone/fetch/pull/push。
- **偏离计划的 go-git PlainClone → 改 CLI**：与 gitx 其余 CLI 一致 + 走系统 git 凭据助手/SSH **一条码路覆盖公/私库**（go-git PlainClone 需自己接 auth、不用用户凭据助手）；D4 本就规定 credential-helper 场景走 CLI。记 commit `4d6f9d4`。
- **安全/范围延后（findings J11）**：① 显式 user/pass —— cc-pane 注入 URL（`https://user:pass@host`），但 **argv 带明文凭据会泄漏到进程列表/日志/reflog（反模式）**；优先系统凭据助手/SSH，显式凭据留 clone UI 单元用 **askpass / credential-helper** 设计，不走 argv。② 细粒度进度 —— git 进度用 `\r` 覆盖行，**procx.scan 用 bufio.Scanner 按 `\n` 切分**（实读源码确认）无法增量流出 phase/percent；留 clone UI 单元引入 `\r`-aware reader + Wails 事件。③ Pull/Push 前 Local History auto-label 钩子 → gitsvc 服务层。
- 测试：+`TestNetworkCloneFetchPullPush`（本地 bare 远端：work1 push→gitx.Clone→work2，work1 再 push→work2 fetch+pull 拿到 b.txt + clone 到已存在目录报错）——无真网络/鉴权，也证实 `git clone` 支持 `--`。**8 测** go build/vet/test dev+prod 全绿。commit `4d6f9d4`。
- **Git 后端原语阶段性收口**（只读 + worktree/stash + network 三 unit，8 测）。**剩**：git 面板（结构化 per-file status XY + stage/unstage/commit + diff，net-new 增强，**面板 UI 视觉门控**）；gitsvc 服务层（Wails 包装 + bindings + Local History 标签钩子，待 Local History 子系统）。**下一单元倾向**：转 Local History 后端（file_versions 快照，独立 history.db，纯可测，且正好补上 Git 的 auto-label 钩子依赖方），或 git 面板结构化 status 后端（可测，不含 UI）。

## 会话 29 — Phase 4 Local History 存储基座 + 决策门 D10（2026-05-30，autonomous /loop）

### 决策门 D10（读 cc-pane history_file_repo.rs 后向用户提问 → 已定）
- Local History 与 D8（app 级单一 xkpane.db + Holder + AllModels）**根本不同**：cc-pane 是**每项目独立** `<proj>/.ccpanes/history/{history.db, blobs/}` + 内容寻址磁盘 blob。属真决策门（动数百行 + 偏离铁律 + 写用户项目目录）→ 按停止粒度停下 AskUserQuestion。完整盘点见 findings J12。
- **用户定**：① 存储位置 = 每项目独立（**目录名改 `.xkpanes`**，对齐品牌，非 `.ccpanes`）；② 现在开工，先存储基座。记 task_plan 决策表 D10。

### 做了什么（`internal/localhistory/store.go` + `store_test.go`，commit `43d6f80`）
- `FileVersion`{id(时间戳ms+4位计数,复合 PK 之一), file_path(复合 PK), hash(SHA256), size, created_at(RFC3339 UTC), is_deleted, branch}。
- `Open(projectPath)`：建 `.xkpanes/history/blobs` + GORM/glebarez 开 history.db + WAL/NORMAL/busy_timeout + AutoMigrate。**「按项目多 db」模式**（不走 Holder——Holder 是单 app db；这是 D10 的有意分层）。
- `SaveVersion`：SHA256 内容寻址 blob（filex 原子写，同 hash 不重写）+ 同 file_path+branch 最新版 hash 相同→去重跳过 + min_interval 节流跳过 + is_deleted 版本不去重/不节流。返回 (nil,nil) 表跳过。
- `ListVersions`/`ListVersionsByBranch`/`GetFileBranches`/`GetVersionContent`(gzip 魔数兼容读，前向兼容未来压缩单元)。
- 测试：6 测（目录布局 / 保存+去重+列举+内容回放+缺失版本 / 节流 / is_deleted 绕过去重 / 分支隔离 / 跨文件 blob 去重）。go build/vet/test dev+prod 全绿。
- **延后单元**（findings J12 列全）：diff 引擎（**Go diff 库选型=后续决策点**）/ 标签 / restore / 最近变更 / pruning / gzip 压缩 / gitsvc 服务层（按项目缓存 Store + gitx auto-label 钩子）/ `.xkpanes` 自动 gitignore 的 UX。
- **下一单元倾向**：Local History 标签(labels+label_snapshots，纯可测，且是 gitx auto-label 钩子的直接依赖) 或 pruning(可测) 或 restore(可测)；diff 引擎需先定 Go diff 库（到该单元再问用户）。

### ▶ 续：Local History unit 2 — 标签（同 /loop 自主续）
- `labels.go`：Label + LabelSnapshot 表 + HistoryLabel/LabelFileSnapshot 组合视图。PutLabel(ON CONFLICT upsert + 整体替换快照,事务,实证 glebarez/modernc 支持 ON CONFLICT) / ListLabels(timestamp DESC,两查询+Go 分组避 N+1,空快照非 nil) / DeleteLabel(显式删两表不靠 FK cascade) / GetAllLatestSnapshots(每文件 MAX(id))。AutoMigrate 追加两表。
- 标签是 **gitx Pull/Push/Stash auto-label 钩子的直接依赖**（钩子在 gitsvc 服务层补）。+TestLabels（snapshots 取最新 / put / upsert 改名+整体替换 / timestamp DESC 排序 / 删除连快照）。localhistory **7 测** dev+prod 全绿。commit `a089f0b`。
- **下一单元倾向**：restore（按标签/版本写回工作树，可测——cc-pane 有 restore_version/restore_label）或 pruning（cleanup_old_versions + cleanup_by_total_size LRU + cleanup_orphan_blobs，可测）；diff 引擎仍待先定 Go diff 库。

### ▶ 续：Local History unit 3 — restore + CreateAutoLabel（同 /loop 自主续）
- `restore.go`：RestoreVersion（恢复前自动打 "Before Restore: <file>" 标签 + 写回工作树 + mkdir 父目录）/ RestoreToLabel（"Before Rollback" + 逐快照 best-effort 写回 + 返回已恢复列表 + 未知标签报错）/ CreateAutoLabel（快照全部最新 + uuid auto 标签——**gitx Pull/Push/Stash auto-label 钩子的服务层入口已就位**）。
- **写回工作树用 os.WriteFile 不用 filex.WriteAtomic**：对已存在文件截断写入保留原权限（可执行位等），原子写 temp+rename 会重置权限——恢复须保权限（blob 内部写仍原子）。branch 由服务层经 gitx.CurrentBranch 传入（Store 与 git 解耦）。
- +3 测（RestoreVersion 写回+恢复前标签 / 恢复嵌套路径建父目录 / RestoreToLabel 回滚+Before Rollback+未知标签报错）。localhistory **10 测** dev+prod 全绿。commit `49ad357`。
- **Local History 已具备**：存储基座 + 标签 + restore/auto-label。**剩**：diff 引擎（先定 Go diff 库 = 决策点）/ 最近变更 + 目录变更 + 已删文件列表（纯查询，可测）/ pruning（可测）/ gzip 压缩 / HistoryConfig（config.toml）/ Wails 服务层（按项目缓存 Store）。
- **下一单元倾向**：最近变更/目录变更/已删文件（get_recent_changes + list_directory_changes + list_deleted_files，纯 SQL 查询、可测、无门）或 pruning（可测）；diff 引擎到该单元再定 Go diff 库。

### ▶ 续：Local History unit 4 — 查询（同 /loop 自主续）
- `query.go`：GetRecentChanges(跨文件最新在前 + label_name 相关子查询 COALESCE 避 NULL + limit) / ListDirectoryChanges(LIKE+ESCAPE '\\'，dirPath 空=全部，since 过滤) / ListDeletedFiles(最新版本=删除标记的文件,MAX(id) 判定) + escapeLikePattern(\\ % _)。排序 created_at DESC, id DESC（id 作同秒 tiebreak）。
- +4 测（escape 6 例 / 最近变更顺序+label_name 回填+limit / 目录 src 含嵌套不含 root+空=全部+since 远古远未来 / 已删 MAX(id) 判定+复活排除）。localhistory **14 测** dev+prod 全绿。commit `ff04e53`。
- **Local History 剩**：pruning（cleanup_old_versions max_age/max_versions + cleanup_by_total_size LRU + cleanup_orphan_blobs，可测）/ gzip 压缩 compress_blobs / HistoryConfig(config.toml) / diff 引擎（**Go diff 库选型=决策点**）/ Wails 服务层（按项目缓存 Store + gitx auto-label 钩子接线）。
- **下一单元倾向**：pruning（可测、防无限增长、收口存储核心）；之后 diff（先定 Go diff 库）或转 gitsvc 服务层把 Local History + gitx 接进 Wails。

### ▶ 续：Local History unit 5 — pruning + HistoryConfig（同 /loop 自主续）
- `prune.go`：CleanupOldVersions(max_age_days 删过期 + max_versions_per_file 每文件留最新 N)/CleanupByTotalSize(超上限按 id 旧→新删,共享 blob 仅最后引用删时计释放,LRU)/cleanupOrphanBlobs(回收无引用 blob)/Cleanup(读 config 跑全套) + blobsDirSize。
- `config.go`：HistoryConfig(默认与 cc-pane 一致,ignore 把 .ccpanes 换 .xkpanes) + DefaultHistoryConfig + ReadConfig(缺失字段保留默认,在默认值上反序列化)/SaveConfig。**偏离 cc-pane：config 用 JSON(.xkpanes/config.json)+filex 原子写，非 TOML**——与 XK-Pane storage.json 一致、免引 TOML 依赖。
- +4 测（config 默认/roundtrip/部分 JSON 合并 / 按数量删留最新2+孤儿回收 / 按 age 删 100 天前(经 s.gdb 直插旧版本)+旧 blob 回收 / 按总大小 LRU 删到 <=上限）。localhistory **18 测** dev+prod 全绿。commit `b5f5d23`。
- **Local History 存储核心收口**（基座+标签+restore+查询+pruning/config，18 测）。**剩**：gzip 压缩 compress_blobs（小，可测）/ diff 引擎（**Go diff 库选型=决策点**，到该单元问用户）/ Wails 服务层（按项目缓存 Store + 文件监听记快照 + gitx auto-label 钩子接线 + bindings）。
- **下一单元倾向**：gzip 压缩 compress_blobs（小、可测、收口存储）；之后 diff 引擎将触发 **Go diff 库选型决策门**（到时问用户：sergi/go-diff vs hexops/gotextdiff vs 自实现）。

### ▶ 续：Local History unit 6 — gzip 压缩 + ⏸ 方向检查点（同 /loop 自主续）
- `compress.go`：CompressBlobs（跳过 .db*/已 gzip/<512B，gzip 后更小才原子替换 filex + ctx 可取消）。blob 名仍是原始内容 SHA256，压缩只改内容；GetVersionContent 按魔数透明解压 → 对去重/读取/二次保存透明。+TestCompressBlobs。**Local History 存储核心全部完成，19 测** dev+prod 全绿。commit `51cb1e5`。
- **⏸ 停下检查方向（会话30 末，2026-05-30）**：连续 ~10 个自主 tick 交付**两大可测后端**——Git（只读+worktree/stash+network，8 测）+ Local History 存储核心（6 unit，19 测）——全 tested/committed/文档化。但：① 下一个 in-context 单元 = diff 引擎，触发 **决策门 D11（Go diff 库选型，新依赖 + 行级 hunk+字符级 inline 设计）**；② **已累计大量可测后端但无 UI 消费方**，且终端栈 + 项目面板仍从未真机验证（视觉验证债持续累积）；③ 余下高价值动作多需用户方向（继续 LH diff/服务层 / 转早期子系统 Bedrock·MCPE·notify·memory / 先做个视觉 checkpoint 验证累积成果）。→ 按停止粒度（仅在决策门停）AskUserQuestion 问方向 + diff 库，停 loop 待答。
- **续跑路径**：用户答 AskUserQuestion 后即按其选择推进；若选继续 LH diff，按所选 Go diff 库落地（go-udiff+diffmatchpatch 为推荐）。

### ▶ 续：用户选「继续 Local History → diff 引擎」→ unit 7 diff 引擎（决策门 D11 已定）
- **D11**：用户授权我选库 → 用 `sergi/go-diff/diffmatchpatch`（**已是间接依赖**，promote direct **零新依赖**，单库行级 DiffLinesToChars + 字符级 DiffMain；优于另引 go-udiff）。记决策表 D11。
- `diff.go`：移植 cc-pane compute_diff/compute_inline_changes。ComputeDiff(二进制 BOM/\0 检测→IsBinary；超 MAX_DIFF_LINES=10000→Truncated；行级 Equal/Delete/Insert→CONTEXT_LINES=3 上下文 + 相邻段间距≤2*CONTEXT 合并 hunk；stats add/del/changes=min；相邻 Delete/Insert 行配对字符级 inline，**rune 计数非字节**) + GetVersionDiff(版本 vs 工作树)/GetVersionsDiff(版本间)。splitLines(Rust .lines 语义)/splitDiffBlock(行块拆分)。
- +8 测（替换+inline / 增删行+行号侧 / 相同 / 二进制 / 截断 / **多字节 rune 位置** / 远距双 hunk / Store diff 方法）。localhistory **27 测** dev+prod 全绿。go-diff promote direct。commit `80bf6cd`。
- **⏳ 对抗审查 workflow `w05btokta`（wf_e4f728f5-11b）在跑**：3 lens（cc-pane 保真 / 边界 off-by-one / 换行·rune·diffmatchpatch）→ refute-by-default 验证。这是本会话最算法敏感的代码，ultracode 下值得对抗验证。完成后处理确认的真问题。
- **Local History 后端基本完整**（7 unit：基座+标签+restore+查询+pruning/config+压缩+diff，27 测）。剩 Wails 服务层（按项目缓存 Store + 文件监听 fsnotify + gitx auto-label 钩子 + bindings）—— 前端消费方/视觉门控。

### ▶ 续：diff 对抗审查结果 + CRLF 修复（同 /loop 自主续）
- **审查 workflow `w05btokta` 完成**（13 agents，审 10 条发现 → refute-by-default 验证 → **确认 2 条，实为同根因**）：① (medium) 跨换行风格 diff(CRLF vs LF) 时行级 diff 把 `"a\r\n"≠"a\n"` 判整行替换，而 splitDiffBlock 又剥掉 `\r` 使两侧 content 逐字相同 → UI「全爆红却看不出哪变、inline 空」；`GetVersionDiff`(存储版本 vs 工作树) 在 Windows-CRLF↔编辑器/Git-LF 场景极易触发。② (low) splitDiffBlock 剥 `\r` 偏离 cc-pane `trim_end_matches('\n')`（只剥 \n 保留 \r）+ splitLines 注释混淆。
- **修复 `03f3171`**：ComputeDiff 行级 diff 前 `normalizeNewlines(\r\n/孤立\r→\n)`，仅作用于 diff 计算/显示、**绝不碰存储 blob 与 restore 原始字节**——一举解决两条，且**有意改进 cc-pane**（similar 换行敏感会假阳性全爆红）。splitDiffBlock 不再单独剥 \r，splitLines 注释澄清仅用于行数统计。+TestComputeDiffNewlineNormalization。localhistory **28 测** dev+prod 全绿。清理了审查 agent 留下的 probe 文件。记 findings J13。
- **审查另证**：「CRLF↔LF 整块判变更」本身 cc-pane(similar) 也有（继承行为，非移植 bug），但 cc-pane 保留 \r 故 inline 至少高亮 \r；本仓归一化后更优（纯行尾差异 0 diff）。验证者实测复现两条、对照 Rust 源 + deepwiki 确认 similar tokenize_lines 保留 \r\n。
- **Local History 后端完成（含 diff，28 测，已对抗验证 + 修复）**。下一单元倾向：Wails 服务层（前端消费方/视觉门控）或转其他可测后端 / 早期子系统——属方向选择，近期或再设检查点。

### ▶ 续：会话恢复后端（Phase 4 / 目标判据「会话恢复」，同 /loop 自主续）
- `internal/services/session/session.go`：移植 cc-pane session_restore_repo。SaveSessions(**整体替换**=清空+批量插入,事务,空切片=清空)/LoadSessions(全量,saved_at 升序)/ClearSessions。会话集合=当前布局快照,每次保存覆盖。
- **安全**：SshConfig 敏感 → cryptox 落库加密/读出解密(铁律)。**SECURITY TODO**(待 SSH 运行时单元)：sshConfig 是后端重连数据非前端,届时仿 provider 脱敏 DTO + 非绑定解密 helper；当前 SSH 未落地、sshConfig 恒 nil 无明文跨 IPC。**暂不注册 app.go**(恢复 UI 视觉门控)。
- +4 测（roundtrip + sshConfig 解密 / **sshConfig 落库密文裸行校验** / 整体替换 / 空保存=清空+Clear）。dev+prod 全绿。commit `e379cd1`。
- **进度盘点**：Git(3 unit/8 测) + Local History(7 unit/28 测,已对抗验证) + 会话恢复(1 unit/4 测) 后端均完成。**清晰无门的核心可测后端渐少**——剩 Todo/Spec CRUD(模型已存,可测无门)、launch_profile(需新模型)、运行时 WSL discovery(平台相关)；之后多为前端消费方/视觉门控的服务层 + UI，或用户强调的早期子系统(Bedrock/MCPE/notify/memory，Phase 5，含 D5 MCP 等较大架构)。
- **下一单元倾向**：Todo + TodoSubtask CRUD（模型已在 AllModels，Holder 模式，纯可测无门）；做完后若仍无新方向 + 清晰可测后端耗尽，再设检查点建议用户验证/接前端/转早期子系统。

### ▶ 续：Todo 核心 CRUD 后端 + ⏸ 大方向检查点（同 /loop 自主续）
- `internal/services/todo/todo.go`：移植 cc-pane todo_repo 核心子集。Create(全字段+默认 todo/medium/global，uuid，sort_order 追加，tags JSON)/Get(含子任务,miss→nil)/List(sort_order,created_at)/Update(patch)/Delete(显式删 todo+子任务) + AddSubtask/UpdateSubtask/DeleteSubtask。TodoDTO tags 出入 []string。延后：高级 query/stats/reminders/recurrence/reorder/batch。暂不注册 app.go(面板 UI 视觉门控)。+3 测 dev+prod 全绿。commit `7a1c1ba`。
- **⏸ 大方向检查点（会话末，2026-05-30）**：**用户明确指示的工作(继续 LH diff)已完整交付**（含对抗审查+CRLF 修复），此后我又自主补了会话恢复 + Todo CRUD。现状盘点：**已交付 ~14 个后端单元、~40 测、全 tested/committed/文档化、且无一个有前端消费方**；终端栈 + 项目面板仍从未真机验证（视觉验证债持续累积近 10 轮）。余下每条路都是**重大方向分叉**：① 接前端/让成果可验（Wails 服务层 + 最小 UI，打破未验证后端堆积循环）② 早期子系统 Bedrock/MCPE/notify/memory（用户「早期纳入」强调，含 MCP D5 等较大架构）③ 继续 Phase 4 外围后端（Spec CRUD / Todo 高级 / 运行时 WSL / 启动配置）。用户偏好实质改变后续数周方向 → 按停止粒度（决策门）AskUserQuestion 问大方向，停 loop 待答。**最高杠杆动作仍是用户 `wails3 dev` 验终端+provider，我替不了。**
- **续跑路径**：用户答方向后即按其选择推进。

### ▶ 续：用户选「接前端/让成果可验」→ Todo 端到端垂直切片（同 /loop 自主续）
- **首个用户可端到端验证的功能,完全不依赖未验证的终端栈**。后端 todo 服务注册到 app.go + 重生 bindings(10 Services/51 Methods) + 前端全套 MVVM：`services/todo/TodoService.ts`(binding 收口,Create/Update 用生成类构造器补默认字段) + `src/todo/`(TodoProvider 纯 Context,镜像 ProviderProvider) + `pages/TodoPage/`(View/VM/Style/lang，列表+新建+状态勾选+删除[原生确认]+子任务增勾删,TodoRow 子组件持子任务输入态) + routes.tsx(slot primary) + i18n(todo.*+route.todo) + App.tsx 接 TodoProvider。
- 遵循全部 MVVM 铁律(View↛业务/VM↛JSX·↛t()/经 services 层/可见串走 t()/@mui *Rounded 图标/theme.palette.foundation.* 取色/删除走 NativeDialogs.confirm)。pnpm typecheck+build 通过,go build internal 通过。commit `8854af5`。
- **🎯 现在可真机验证**：`wails3 dev` → 侧边栏「待办」页 → 新建/勾选/删除待办 + 子任务,端到端验证 Wails 服务往返 + Provider + MVVM + confirm 修复。**这是「接前端/让成果可验」方向的首个落地,也顺带验证 app 能启动。** 终端页 + 设置→Provider 仍待你一并验。
- **下一单元（续接前端）**：gitsvc（Wails 包装 gitx + 注册 + bindings，让 Git 后端可达，可测）；之后 Git 面板 UI / localhistorysvc + 历史面板 / 注册 session。各为独立切片。

### ▶ 续：gitsvc — Wails 包装 gitx，Git 后端可达前端（同 /loop 自主续）
- `internal/services/gitsvc/gitsvc.go`：**薄无状态** service 包装 gitx 全部只读/工具原语（不持 Holder——gitx 按路径操作）。每调用按 path 现开仓库（gitx.Open 廉价）。方法：IsRepo / GetStatus / Branches / Log / Worktrees / WorktreeAdd / WorktreeRemove / Stash / StashPop / Clone / Fetch / Pull / Push。
- **错误语义有意分层**：`GetStatus` 对非仓库**友好降级**为 `{IsRepo:false}` 不报错（对齐 cc-pane null-status，前端据此隐藏 Git 面板）；明细/变更方法对非仓库**透传 ErrNotARepo**（UI 仅在 IsRepo 时调）。返回 gitx 自身 DTO（BranchInfo/CommitInfo/WorktreeInfo，与 project 返回 storage.Project 一致，落 bindings/.../utils/gitx/models）。`StatusDTO{isRepo,branch,isDirty}` 为 service 本地组合类型。
- 注册到 app.go（`gitsvc.New()`，无依赖）。`wails3 generate bindings` → **11 Services/64 Methods/44 Models**（gitsvc +1 svc/+13 方法/+4 模型；terminal 函数类型 warning 为既有无关项）。
- `gitsvc_test.go` +5 测，**只覆盖 service 在 gitx 之上新增的逻辑**（gitx 自身 24 测已覆盖原语）：GetStatus 干净/脏组合、非仓库降级（不报错）、明细方法非仓库透传 ErrNotARepo、Branches/Log 委托链。go build/vet/test dev+prod 全绿；pnpm typecheck + build 通过（bindings 不破坏既有前端）。commit `be059d7`。
- **层次边界**（留后续）：worktree 分组目录策略 / 名称安全校验 / pull·push·stash 前 Local History 自动打标签钩子——属上层 UX / 历史子系统，本 service 只做原语 1:1 暴露。
- **下一单元（续接前端）**：Git 面板 UI（GitService.ts 收口 + src/git Provider + 面板组件，消费 gitsvc）/ 或 localhistorysvc + 历史面板 / 或注册 session 服务。各为独立切片。

### ▶ 续：Git 状态查看页 — 消费 gitsvc，让 Git 后端可真机验（同 /loop 自主续）
- `services/git/GitService.ts`：收口 gitsvc 只读子集（isRepo / getStatus / branches / log），DTO 干净化（GitStatus / GitBranch / GitCommit；when=RFC3339 字符串）。变更操作（worktree/stash/网络）暂不收口（留后续单元，含原生确认）。
- `pages/GitPage/`（View/VM/Style/lang）：选已登记项目（下拉，消费既有 `useProjects()`）→ 展示**分支名 + 脏/净 Chip + 本地分支列表（当前分支高亮）+ 最近 20 提交（短哈希/主题/作者）**。非仓库友好降级显示「不是 Git 仓库」。刷新按钮。
- **有意偏离原计划「src/git Provider」**：Git 状态是**按 path 即查的视图态**（非跨页面共享的全局登记表），与 project/provider/todo 那类全局列表性质不同 —— 建 Provider 是投机抽象（违简洁优先），故状态留 VM；`reqIdRef` 请求令牌丢弃快速切换时的过期响应。若将来侧边栏需 per-project Git 角标等多消费方，再提升为 Provider。
- routes 加 git 路由（AccountTreeRounded，slot primary）；i18n `route.git` + `git.*` 命名空间（zh/en）；App 注册 locale。遵循全部 MVVM 铁律。typecheck + build 通过（本 tick 无 Go 改动，后端不变）。commit `fcd73cd`。
- **🎯 现在可真机验证**：`wails3 dev` → 侧边栏「Git」页 → 选项目 → 看分支/脏标/提交历史。**第二个端到端可验功能**（Todo 之后），打通 gitsvc 全链路。
- **下一单元（续接前端）**：Git 变更操作 UI（worktree 增删 / stash±/ fetch·pull·push，带原生确认 + 错误提示）/ 或 localhistorysvc + 历史面板 / 或注册 session 服务。各为独立切片。

### ▶ 续：Git 工具条变更操作 — fetch/pull/push/stash/pop（同 /loop 自主续）
- **沿 cc-pane 工具条边界拆分**：本 tick 做单按钮型操作（fetch/pull/push/stash/stash-pop），worktree 管理（需目录选择 + 分支输入 + 列表面板）留独立后续单元。
- `GitService.ts` 收口 5 个变更方法（fetch/pull/push→void，stash/stashPop→git 返回文本）。
- `useGitPage` 加 `busy` 态 + `runOp` 统一编排：**busy 互斥守门** → 调后端 → 成功 `NativeDialogs.info`（stash/pop 用 git 返回文本，其余用文案）→ **重新拉取状态/分支/提交**（变更可能改动它们）→ 失败 `NativeDialogs.error`。OpStrings / PushStrings 文案束由 View 经 t() 填入（VM 不持翻译串）。
- `push` 带**原生确认**（推送对外不可逆，遵全局「对外/不可逆操作先确认」准则）；其余直接执行。
- GitPage 工具条（仅 `status.isRepo` 显示，5 个 outlined 按钮 + *Rounded 图标，`busy||loading` 时禁用）。文案 `git.actions.*` / `git.ops.<op>.{title,done}` / `git.pushConfirm.*`（zh/en 对齐）。本 tick 无 Go 改动。typecheck + build 通过。commit `133b951`。
- **🎯 现在可真机验证**：`wails3 dev` → Git 页选项目 → 工具条 fetch/pull/push/stash/pop（push 弹确认；结果/错误走原生对话框；操作后状态自动刷新）。
- **下一单元（续接前端）**：Git worktree 管理 UI（列表 + 新建[目录选择+分支名] + 删除[确认]）/ 或 localhistorysvc + 历史面板 / 或注册 session 服务。各为独立切片。

### ▶ 续：Git worktree 管理 UI + 后端分组目录策略（垂直切片，同 /loop 自主续）
- **后端**（分组目录策略归服务层，对齐 gitx 文档「上层 UX 留 gitsvc」）：`gitsvc.WorktreeAddUnder(repoPath, parentDir, name, newBranch)` —— `validateWorktreeName`（非空 / 无 `/ \` `..` / 前导 `-`）+ OS 感知 `filepath.Join(parentDir, name)` → 调 `repo.WorktreeAdd`，返回创建路径。+2 测（坏名/好名表 + 建树并入列表，**测中发现并归一** filepath.Join 反斜杠 vs git porcelain 正斜杠）。bindings 重生 **65 Methods**。go build/vet/test dev+prod 全绿。
- **前端**：`GitService` 收口 worktrees / worktreeAddUnder / worktreeRemove + `GitWorktree` 类型。GitPage worktree 区：列表（路径/分支 Chip/短哈希/主树「主」标）+ 新建（名 TextField → `openDirectory` 选父目录 → 后端建树，名同时作分支名）+ 删除（仅非主树，原生确认 + 强制移除提示）。VM `load()` 改三路并发（branches/log/worktrees）；`addWorktree`/`removeWorktree` 走 busy 互斥 + 操作后重拉。文案 `git.worktree.*`（zh/en）。typecheck + build 通过。commit `7c73800`。
- **设计注**：filepath.Join(Win 反斜杠) ≠ git porcelain(正斜杠)，但前端建树后**重拉列表**、不按返回路径匹配，故分隔符差异不影响 UI；删除按列表路径（git 在 Win 两种分隔符都接受）。
- **Git 功能已对齐 cc-pane git 范围**：状态查看（分支/脏标/提交）+ 工具条（fetch/pull/push/stash/pop）+ worktree 管理（增删查）。**第二个完整端到端功能域**（继 Todo）。
- **下一单元（续接前端，方向轮换）**：localhistorysvc（Wails 包装 internal/localhistory + 注册 + bindings）+ 历史面板 / 或注册 session 服务 + 会话恢复 UI。各为独立切片。

### ▶ 续：localhistorysvc — Wails 包装 Local History 子系统，后端可达前端（同 /loop 自主续）
- **每项目独立 db（D10）的服务层缓存**：`internal/services/localhistorysvc` 维护 `projectPath → *Store` 缓存（`sync.Mutex` 保护，首访 `localhistory.Open` 建库 + AutoMigrate，后续命中复用连接池）——localhistory 包注释明确把「服务层按项目缓存 Store」留给本层。`closeAll()`（未导出，不绑前端）供应用退出 / 测试清理关所有库。
- **暴露历史面板所需面**（11 方法）：`SnapshotFile`（读工作树文件 → SaveVersion，手动快照；未来 file-watcher 走同一 SaveVersion）/ GetRecentChanges / ListVersions / ListDeletedFiles / GetVersionContent（→string）/ GetVersionDiff（vs 工作树）/ GetVersionsDiff（版本间）/ ListLabels / RestoreVersion / RestoreToLabel / DeleteLabel。注册 app.go。
- bindings 重生 **12 Services/76 Methods/53 Models/1 Enum**（DiffChangeType）。+2 测端到端走 service 层（快照→去重→最近变更→列版本→取内容→版本间 diff→回滚+Before-Restore 标签；Store 缓存计数；多项目独立 db 互不可见）。
- **测中修 cleanup 时序**：`t.Cleanup(closeAll)` 必须在 `t.TempDir()` **之后**注册（cleanup LIFO，后注册先跑），否则 Windows 下 WAL 句柄占用致 TempDir 删除失败。go build/vet/test dev+prod 全绿；前端 typecheck+build 通过。commit `834e96e`。
- **层次边界**（留后续）：file-watcher 自动捕获 / gitsvc Pull·Push·Stash 前 auto-label 钩子 / 清理压缩维护 —— 非面板交互面，留专门单元。
- **下一单元（续接前端）**：Local History 历史面板 UI（消费 localhistorysvc：最近变更列表 + 版本 diff 查看 + 回滚，同 gitsvc→Git UI 两段式）/ 或注册 session 服务 + 会话恢复 UI。各为独立切片。

### ▶ 续：Local History 历史面板 UI（第一阶段：列表/快照/回滚，同 /loop 自主续）
- **有意两段拆分**（diff 渲染器较重）：本 tick = 选项目 + 最近变更列表 + 手动快照 + 回滚（证 SnapshotFile→GetRecentChanges→RestoreVersion 端到端）；**下一 tick = diff 查看器**。
- `LocalHistoryService.ts` 收口 recentChanges/snapshotFile/restoreVersion + RecentChange/FileVersion 类型（diff 子集留下一单元）。
- `pages/HistoryPage/`（View/VM/Style/lang）：项目下拉（消费 `useProjects()`）+ 手动快照行（相对路径 TextField → SnapshotFile，去重返回 null 时提示「无变化」）+ 最近变更列表（文件路径/本地化时间/分支 Chip/标签 Chip）+ 回滚（原生确认，覆盖工作树，后端回滚前自动打 Before-Restore 标签）。状态留 VM（同 GitPage：按 path 即查；reqIdRef 防过期；busy 互斥）。branch 暂传 ""（接 gitsvc 取当前分支留后续）。
- routes 加 history 路由（HistoryRounded，slot primary）；i18n `route.history` + `history.*`（zh/en）；App 注册 locale。遵循全部 MVVM 铁律。本 tick 无 Go 改动。typecheck + build 通过。commit `dbf10fe`。
- **🎯 现在可真机验证**：`wails3 dev` → 「本地历史」页 → 选项目 → 输相对路径快照 → 见最近变更 → 编辑文件再快照 → 回滚到旧版本（文件内容回退）。**第三个端到端功能域**（继 Todo / Git）。
- **下一单元（续接前端）**：Local History diff 查看器（LocalHistoryService 加 getVersionDiff/getVersionsDiff + DiffView 组件，行级 + inline 字符级渲染，点变更看 diff）/ 或注册 session 服务 + 会话恢复 UI。各为独立切片。

### ▶ 续：Local History diff 查看器（第二阶段，同 /loop 自主续）
- `LocalHistoryService` 加 `versionDiff` + 完整 diff DTO（DiffResult/Hunk/Line/Stats/InlineChange + `DiffChangeType` 联合 `'equal'|'insert'|'delete'|'replace'`），converter 把 binding 嵌套类收口为干净接口。
- **新增纯展示组件 `components/DiffView/`**（labels 经 prop 传入、不持 i18n、可复用）：渲染 +N/−N 统计行 + 每 hunk 头（`@@ -a,b +c,d @@`）+ 双行号 gutter + 符号列 + 内容；行底色按 changeType（insert 绿 / delete 红，`alpha 14%`），inline 高亮 `alpha 38%`。**inline 按码点切片**（`Array.from`）——InlineChange.start/end 是 rune 下标非 UTF-16 单元，非 BMP 字符（emoji）才正确。
- HistoryPage：变更行主区可点（role/tabindex/Enter/Space 无障碍）→ `viewDiff`（版本 vs 工作树），选中行 accent 描边；下方 diff 区显示标题 + DiffView。VM 加 selectedVersionId/diff/diffLoading + viewDiff（`diffReqRef` 请求令牌防过期）；切项目 / 回滚后 `clearDiff`。文案 `history.diff.*`（zh/en）。本 tick 无 Go 改动。typecheck + build 通过。commit `818cde3`。
- **把后端已对抗审查过的 diff 引擎首次可视化**。**Local History 域功能完整**：列表 + 快照 + 回滚 + diff（行级 + 字符级 inline）。第三个完整端到端功能域收尾。
- **🎯 现在可真机验证**：`wails3 dev` → 本地历史页 → 快照文件 → 编辑 → 再快照 → 点某版本看 diff（绿/红行 + 字符级高亮）→ 回滚。
- **下一单元（续接前端，方向轮换）**：注册 session 服务（已写未注册）+ 会话恢复 UI / 或 Git 域接 Local History（pull/push/stash 前 auto-label 钩子）/ 或 Todo 高级功能。各为独立切片。

### ▶ 续：stash/pull/push 前 auto-label 钩子（连接 Git ↔ Local History，同 /loop 自主续）
- **对齐 cc-pane `auto_label_before_git`**：gitsvc 在 stash/pull/push 前给项目 Local History 打 auto 标签（Before Stash/Pull/Push，source=git_stash/pull/push），便于操作后回滚。**首个跨域集成**（连接此前各自独立建好的 Git 域与 Local History 域）。
- **解耦设计**：gitsvc 加 `AutoLabeler` 函数类型 + `SetAutoLabeler`（注入式，gitsvc **不强依赖** Local History；同 `termSvc.SetProviderResolver`，因含 func 参数不被 Wails 绑定，仅后端装配）。`labelBefore` best-effort —— branch 由已打开 repo 算，**标签失败不阻断 git 操作**。app.go 预创建 `gitSvc`/`lhSvc` 两服务实例并经 SetAutoLabeler 连接（与 subSvc/termSvc 的预创建-回填模式一致）。
- localhistorysvc 加 `AutoLabel`（绑定，未来前端也可手动打标签）；`localhistory.Exists` 守门：**项目尚无历史存储则跳过，不为纯 git 操作凭空创建 .xkpanes/**（仅对已在用 Local History 的项目生效）。
- +测：gitsvc 钩子在 stash/push 前触发（含 push 失败仍先触发）+ 无 labeler 不 panic；localhistorysvc AutoLabel 守门（无历史跳过不建目录 / 有历史打标签）。bindings 重生 **12 Services/78 Methods**（gitsvc 现也有 func-type warning，预期）。dev+prod 全绿；前端 typecheck+build 通过。commit `42f3f82`。
- **下一单元（续接前端）**：会话恢复需先做 sshConfig 脱敏（已记 SECURITY TODO）才能注册 session 服务 / 或 Todo 高级（query/stats）/ 或 file-watcher 自动捕获（让 Local History 自动快照，需引 fsnotify —— 新依赖，到时按决策门 AskUserQuestion）。各为独立切片。

## 会话 26 — Phase 3 provider 表单条件云字段（3b，provider feature 完整）（2026-05-30，autonomous /loop）

### 做了什么
- `ProviderSettings.tsx`：加 `FIELDS_BY_TYPE` 按 provider 类型条件渲染表单字段（与 provider.EnvVars 的 to_env_vars 对齐）——key 类 apiKey(+baseUrl)、cursor 仅 apiKey、bedrock region/awsProfile、vertex region/projectId、config_profile configDir。把固定的 apiKey/baseUrl 块换成按类型 map 渲染。+ 4 个字段标签（region/awsProfile/projectId/configDir，zh/en）。
- **provider 管理对 11 类全可配 → feature 完整**。typecheck + 37 测 + build 全绿。commit `5a9f03f`。

### 进度盘点 + 下一步方向
- **Phase 3 后端 + provider 管理 UI 全部完成**：project/launch_history/provider service（含 cryptox + 安全审查 clean）+ provider→启动 env 注入（头条后端通）+ project 侧边栏 + provider 设置子页（11 类全可配）。
- Phase 3 余项多为**视觉 checkpoint 门控**（启动选 Provider/CLI UI、会话恢复 restore UI、窗口形态）或前端 i18n 梳理。
- **下一方向倾向 Phase 4 可测后端**（皆 goal 成功判据、不依赖终端渲染）：**Git 集成**（go-git status/branch/log/diff，对临时 repo 可测，D4）/ Local History（file_versions 快照）/ Todo·Spec CRUD。或会话恢复后端（terminal_sessions Save/List）。**已累计大量未真机验证前端（终端栈 + 项目面板）**——视觉验证债持续；继续走可测后端以控风险，启动相关 UI 留 checkpoint。
