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

### 下一步（Phase 1 续）
- events 类型化事件 → GORM 核心模型 + AutoMigrate 测试 → cliadapters(8+27 测试) → subprocess 白名单。

### 遇到的错误
- Bash 工具默认走 /usr/bin/bash 致 `$null` 重定向报错；改用 PowerShell 工具解决。
- Edit 报「File has not been read yet」：首批改名漏读文件；补 Read 后重做成功。
- Write 工具调用偶发 malformed（一次）；重发成功。
