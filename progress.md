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
