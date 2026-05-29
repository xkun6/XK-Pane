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
