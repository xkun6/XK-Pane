# P6e 计划 — MC PE 控制（cc-pane → XK 忠实重建，XL）

> 来源：10-agent 并行精读（`wf_7078158a`，~657k token）通读 cc-pane MC PE 全栈（`mc_pe/` 11 文件 + `mcdk_bridge/` 6 文件 + `orchestrator_service.rs` 5 工具 + 游戏内 Python 桥模组 + 前端 5 组件/store/service/types）+ XK 现有约定（orchestrator / terminal / app / storage / go.mod），全部带 `file:line`。本文件 = 动手前事实基线 + 拍板记录 + 单元切分。**已拍板，下一步进 P6e-1。**
> 相关：交接见 [P6E_HANDOFF.md](./P6E_HANDOFF.md)；总计划见 [P6_PLAN.md](./P6_PLAN.md)（§1.4 / §3 映射表 / §P6e）；逐会话进度见 [progress.md](./progress.md)；铁律见 [CLAUDE.md](./CLAUDE.md)。

---

## 0. 一句话 + 关键依赖发现

cc-pane 的 MC PE 让 **pane 里的 Agent 经 MCP 工具驱动一局正在跑的网易版 Minecraft**：截图（多模态识图）、归一化点击、在游戏进程内 exec Python 2.7、热重载、读游戏日志。XK 侧目前**零 MC PE 代码**。

**核心架构发现（决定单元切分）**：MC PE = 两个耦合子系统 ——

- **(A) 启动子系统** `mc_pe/`：复刻 MCStudio 12 步启动 MC PE + 把游戏内 Python 桥模组解压进 behavior_packs + 日志环形缓冲 + 前端 MC PE 面板 + `mc_pe_*` 表。
- **(B) MCDK 调试桥 + 工具** `mcdk_bridge/`：TCP 桥（等游戏内 Python 模组反向连入）+ Win32 截图/点击 + 5 个 MCP 工具。

**(B) 离不开 (A)**：5 个工具全靠 (A) 提供三样东西 —— ① `active_run_pids()` 给 MC PE 的 PID（capture/click 定位窗口靠它，cc-pane 刻意不用 EnumWindows 枚举进程避免硬编码 exe 名）；② launch 时把桥端口注入 env `MCDEV_DEBUG_IPC_PORT`，游戏内 Python 模组才会**反向连入 XK 的桥**（execute/reload 靠它）；③ `LogCapturingEmitter` 把 `mc_pe:output` 旁路抄进日志缓冲（get_mc_pe_logs 读它）。

> ⇒ **要让 XK 自己驱动 MC，XK 必须自己启动 MC PE**（注入自己的桥端口 + 记录 PID）。"只做 5 工具、不做 launch" 不可行。P6e 最小可用核 = (A) 启动后端 + (B) 桥/工具。

---

## 1. 双侧架构映射（cc-pane `file:line` → XK 目标）

cc-pane 源根：`J:/Program/Agent/cc-pane/cc-panes-core/src/services/`（除 orchestrator_service.rs 在 `src-tauri/src/services/`）。
XK 目标：`internal/services/mcpe/`（启动子系统）+ `internal/services/mcdkbridge/`（桥 + Win32）+ orchestrator/storage/events/app/前端接线。

### 1.1 桥协议 + TCP server（(B)，跨平台）

| cc-pane | XK 目标 | 要点 |
|---|---|---|
| `mcdk_bridge/protocol.rs` (296) | `mcdkbridge/protocol.go` | 6 字节大端帧 `[2B type BE][4B len BE][payload]`，`encoding/binary.BigEndian`。`PacketType` 常量 + `Packet` + `Encode`/`Decode`（HEADER_LEN=6）+ 工厂（ExecuteServer/ClientCode、ReloadGame/AllShaders/SingleShader/AddonAndGame） |
| `mcdk_bridge/bridge.rs` (327) | `mcdkbridge/bridge.go` | `Service`：`net.Listen("tcp","127.0.0.1:0")` → `Addr().Port`；accept goroutine；单 active client（新连顶替旧连，`next_id` 从 1）；`SendPacket`（fire-and-forget，无 client→错误）；`Status()`；read-drain goroutine 仅检 EOF（buf 1024，游戏不回数据）；`Port()` |
| `mcdk_bridge/mod.rs` (14) | 包 doc + build tag 分文件 | protocol/bridge 跨平台；capture/click/window 仅 Windows |

**命门常量**：`HEADER_LEN=6`；type id `ExecuteServerCode=3 / ExecuteClientCode=4 / ReloadGame=5 / ReloadAllShaders=6 / ReloadSingleShader=7 / ReloadAddonAndGame=8`（0/1/2 未用，Go 端永不发 type 1）；payload = **裸 UTF-8**（非 JSON、无长度前缀、无 NUL 尾）；fire-and-forget，**C→game 单向，游戏永不回帧**；字节布局权威样例 `Packet(ExecuteServerCode,"abc")` → `[0x00,0x03, 0x00,0x00,0x00,0x03, 'a','b','c']`；错误码 `MCDK_BRIDGE_BIND_FAILED/NO_CLIENT/WRITE_FAILED`、`MCDK_PROTOCOL_INCOMPLETE_HEADER/PAYLOAD`。
**隐藏坑**：游戏内 Python `connect(("localhost", port))`，而 Rust bind `127.0.0.1:0` → **Go 必须 bind `127.0.0.1`**（不要 `::1`/`0.0.0.0`，否则 IPv6 优先解析连不上）。

### 1.2 Win32 截图 / 点击 / 窗口定位（(B)，Windows-only）

| cc-pane | XK 目标 | 要点 |
|---|---|---|
| `mcdk_bridge/window.rs` (113, `pub(crate)`) | `mcdkbridge/window_windows.go` | `EnumWindows` 回调（`windows.NewCallback`）收候选：`IsWindowVisible` + `GetWindow(GW_OWNER)`==null + 同 pid（`GetWindowThreadProcessId`）+ 客户区面积>0；选择：标题含 `"minecraft"`(小写) 或 `"我的世界"` 优先，否则最大客户区面积 |
| `mcdk_bridge/capture.rs` (387) | `mcdkbridge/capture_windows.go` | `find_mc_window_for_pid` → DPI `PER_MONITOR_AWARE_V2` → `IsIconic` 拒最小化 → **`GetWindowRect`(整窗)** → `capture_raw_bgra` → `encode_to_jpeg` → base64(STANDARD)。`capture_raw_bgra`：**桌面 DC** `GetDC(None)` 建兼容位图（MC PE 走 D3D11/DXGI，窗口 DC 兼容位图会全白）→ `PrintWindow(PW_RENDERFULLCONTENT=0x2)`，FALSE 则降级 **窗口 DC `GetDC(hwnd)` + `BitBlt` 源(0,0) SRCCOPY** → `GetDIBits`（biBitCount=32/biCompression=BI_RGB/biHeight=-h 自顶向下，BGRA）。`encode_to_jpeg`：BGRA→RGBA swap(0,2) → 最长边>480 按比例缩放 → 丢 alpha → JPEG q75 |
| `mcdk_bridge/click.rs` (259) | `mcdkbridge/click_windows.go` | 校验 [0,1] → 找窗口 → `IsIconic`→`ShowWindow(SW_RESTORE)`+sleep 50ms → `bring_to_foreground`（`AttachThreadInput`+`SetForegroundWindow`+`BringWindowToTop`，轮询 25×10ms 等前台）→ DPI `PER_MONITOR_AWARE_V2` → **`GetClientRect`(客户区)** → `(n*dim+0.5)` 四舍五入 → `ClientToScreen` → 虚拟桌面 0~65535 归一化 → `SendInput[MOVE,LEFTDOWN,LEFTUP]`（`ABSOLUTE\|VIRTUALDESK`，三段无延时） |

**命门常量**：`PW_RENDERFULLCONTENT=0x2`、`MAX_DIMENSION=480`（**最长边**，保持比例）、`JPEG_QUALITY=75`、`BI_RGB`、`biBitCount=32`、`biHeight=-height`、BGRA→RGBA swap 索引 (0,2)、base64 STANDARD（**无 `data:` 前缀**）；click 三段顺序 MOVE→DOWN→UP、`MOUSEEVENTF_MOVE=0x0001/LEFTDOWN=0x0002/LEFTUP=0x0004/ABSOLUTE=0x8000/VIRTUALDESK=0x4000`、`SM_XVIRTUALSCREEN=76/Y=77/CX=78/CY=79`、`SW_RESTORE=9`、`GW_OWNER=4`、`DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2`=handle 值 `-4`、归一化 65535、前台轮询 25×10ms、最小化恢复 sleep 50ms；错误码 `MCDK_CAPTURE_NO_WINDOW/FAILED`、`MCDK_CLICK_OUT_OF_BOUNDS/NO_WINDOW/FAILED`。
**已知忠实坑（手测验证）**：截图基准 `GetWindowRect`（整窗含边框/标题栏）、点击基准 `GetClientRect`（客户区）——**不一致**。若 MC PE 窗口有非客户区边框，前端在截图上点的归一化坐标回 click 时会系统性偏移。MC PE dev 窗口通常可视为客户区≈整窗时无碍；忠实照搬两者，手测时验证偏移。

### 1.3 启动子系统 12 步（(A)，多数跨平台 + paths/symlink/launcher Windows 特化）

`mc_pe/service.rs:212` `MCPEService::launch(package_id, mcdk_port: Option<u16>)` 的 **12 步确切顺序**（XK 目标 `mcpe/service.go:Launch`）：

1. **校验路径**（service.rs:222）：`safety::validate_command_line_path(exe / project_path / 每个 included_mod_dir)` + `validate_relative_name(world.folder_name)` + `paths::validate_launcher(exe)`。
2. **清 symlink**（service.rs:236）：`symlink::remove_symlinks_in(behavior_packs_dir)` + `(resource_packs_dir)`。
3. **扫描包**（service.rs:244）：`pack::parse_pack(project_path)` + 每个 `included_mod_dir` → `HashMap<PackType, Vec<PackInfo>>`。
4. **重建 symlink**（service.rs:258）：每包 `create_symlink(pack.path, packs_dir/uuid)`；权限不足（1314）收集后 `sudo_create_symlinks`（一次 UAC）。
5. **桥模组解压**（service.rs:277，仅 `debug_options.include_test_mod==true`）：`bridge_mod::extract_to(behavior_packs_dir/BRIDGE_UUID)` → `parse_pack` 并入。
6. **世界目录**（service.rs:291）：有 `source_dir` → `reset_on_launch && exists` 时 `remove_dir_safely` → 不存在则 `copy_world_essentials`（仅 `db/`+`level.dat`）；无 source_dir 且不存在 → `create_dir_all`。
7. **level.dat**（service.rs:319）：不存在则 `write_default_level_dat`(内嵌) → `read_level_dat` → 写 11 字段 → `write_level_dat`。
8. **world_*_packs.json**（service.rs:345）：按 uuid 去重 → 每包 `{pack_id, version:[int...]}` → 写 `world_behavior_packs.json` + `world_resource_packs.json`。
9. **launch_config.cppconfig**（service.rs:386）：`{world_info.level_id, room_info:{}, player_info:{urs:"",user_id:0,user_name}, skin_info:{slim:false, skin:<exe父>/data/skin_packs/vanilla/steve.png}}`。
10. **组 env**（service.rs:416）：注入 3 个 env（下表）。
11. **启动**（service.rs:441）：`args=["config=<cppconfig>"]` → `run_id=uuid` → `LogBuffer(2000)` + `LogCapturingEmitter` 包装 → `launcher::spawn`（PTY cols=200 rows=50）→ 取 pid → 注册进 `runs` map → emit `mc_pe:launched`。
12. **返回** `MCRunSessionInfo{run_id, package_id, pid}`。

| cc-pane | XK 目标 | 要点 |
|---|---|---|
| `mc_pe/paths.rs` (162) | `mcpe/paths_windows.go` + `_other.go` | `mc_dir`=`%APPDATA%\MinecraftPE_Netease`（fallback home\AppData\Roaming）；`game_dir`=`/games/com.netease`；`behavior_packs_dir`/`resource_packs_dir`/`worlds_dir`=`mc_dir/minecraftWorlds`；`scan_launchers`=`GetLogicalDrives()` 遍历 A-Z 拼 `<盘>:\MCStudioDownload\game\MinecraftPE_Netease\<版本>\Minecraft.Windows.exe`；`validate_launcher`（is_file + `.exe`） |
| `mc_pe/safety.rs` (99) | `mcpe/safety.go` | `validate_relative_name`（拒 空/`/`/`\`/`.`/`..`/NUL/控制字符）；`validate_command_line_path`（拒 `FORBIDDEN` + 控制字符） |
| `mc_pe/pack.rs` (252) | `mcpe/pack.go` | `PackType{Behavior,Resource}`（serde lowercase）+ `PackInfo{type,uuid,version,path}` + `parse_pack`（深度≤2 扫 manifest.json，深度1命中即不下钻）+ `parse_manifest`（header.uuid/version[]、modules[].type `data`→Behavior/`resources`→Resource） |
| `mc_pe/world_copy.rs` (228) | `mcpe/worldcopy.go` | `copy_world_essentials`（白名单仅 `db/`+`level.dat`，绝不带 BP/RP）+ `copy_dir_recursive`（`os.Lstat` 跳 symlink）+ `remove_dir_safely`（reparse point 用 `os.Remove` 不递归，普通目录 `os.RemoveAll`）；`trash` 删除（move_world_to_trash）走 SHFileOperation/IFileOperation |
| `mc_pe/symlink.rs` (240) | `mcpe/symlink_windows.go` + `_other.go` | `remove_symlinks_in`（`os.Lstat` 仅删 symlink，保留实体）+ `create_symlink`（`os.Symlink`，1314→收待提权）+ `sudo_create_symlinks`（生成 `mklink /D` .bat → PowerShell `Start-Process -Verb RunAs -Wait -WindowStyle Hidden` 经 procx，CREATE_NO_WINDOW=0x08000000） |
| `mc_pe/level_data.rs` (373) | `mcpe/leveldata.go` | 自实现**小端 NBT** 读写器（`encoding/binary.LittleEndian`）：8 字节头（`i32 LE version`+`i32 LE size`）+ TAG 0x00-0x0C + u16 LE 字符串长度；`read/write_level_dat`（write 重算 size）；`write_default_level_dat`（embed `resources/level.dat`） |
| `mc_pe/bridge_mod.rs` (98) | `mcpe/bridgemod.go` | `embed.FS` 整目录 `include_test_mod` + `BRIDGE_UUID` + `ExtractTo`（`fs.WalkDir` 递归覆盖写） |
| `mc_pe/log_buffer.rs` (151) | `mcpe/logbuffer.go` | `LogBuffer{deque, cap}`（满则去头）+ `tail(n)` + `format_log_payload`（`HH:MM:SS [Category] [LEVEL] line`）+ `short_ts`；**加 `sync.Mutex`**（Rust 靠外层锁，Go 跨 goroutine 必须自带） |
| `mc_pe/log_filter.rs` (478) | `mcpe/logfilter.go` | `parse_stdout_line` 白名单（5 规则：Python 时间戳头/`LoadWindowsAddonPy:`/`ecs_framework `/行首 `[ModTag]`/`[Python]` 兜底）+ `leading_mod_tag`（黑名单仅 `UniSDK`）+ `detect_severity`（固定优先级）+ `parse_python_header`（时间戳含 `,`、len 19-40）；emit `mc_pe:output` |
| `mc_pe/launcher.rs` (347) | `mcpe/launcher.go` | `RunSession{run_id,package_id,pid,process,log_cancel}` + `spawn`（**ConPTY 经 `ptyx.Spawn`**，cols=200 rows=50，cwd=exe父目录，drop writer）+ reader goroutine（逐行 strip-ansi + GBK/UTF-8 容错 + log_filter）+ wait goroutine（emit `mc_pe:exited` code 默认 -1）+ `kill` |
| `mc_pe/service.rs` (499) | `mcpe/service.go` | `MCPEService{repo(holder), emitter, runs map}` + `Launch`(12 步) + `active_run_pids` + `recent_logs(run_id?, lines)` + `stop` + `cleanup_all` + CRUD（list/create/update/delete_package、scan_launchers、move_world_to_trash、open_world_folder）+ `LogCapturingEmitter`（拦 `mc_pe:output` 同 run_id 抄进缓冲再转发） |
| `mc_pe/mod.rs` (19) | 包 doc | 导出聚合 |

**注入的 env（精确名 + 取值，service.rs:416-439）**：
- `MCS_HELPER_DEBUG_OPTIONS` = JSON `{reload_key(默认82=R)、reload_world_key(默认96)、reload_addon_key、reload_shaders_key、reload_key_global(默认false)}`。
- `MCS_HELPER_TARGET_MOD_DIRS` = JSON 数组 `[project_path.replace('\\','/')]`（**反斜杠转正斜杠**，忠实照搬）。
- `MCDEV_DEBUG_IPC_PORT` = 桥端口字符串（**仅 `include_test_mod==true` 且端口非空**）。

**level.dat 写入 11 字段（service.rs:325-342）**：`LastPlayed`(Long, Unix秒)、`LevelName`(String=包名)、`RandomSeed`(Long)、`GameType`(Int, Survival=0/Creative=1)、`Generator`(Int, Default=1/Flat=2)、`cheatsEnabled`/`keepInventory`/`dodaylightcycle`/`doweathercycle`(bool=Byte 0/1)、**`permissionsLevel`=1**、**`playerPermissionsLevel`=1**（强制 OP；值 2 会被引擎当访客拒绝调试指令）。

**命门常量**：`MC_PE_LOG_BUFFER_CAP=2000`；PTY `cols=200 rows=50`；safety `FORBIDDEN`=`" \r \n < > | & ^ % ` $ !`；`ERROR_PRIVILEGE_NOT_HELD=1314`；symlink 用 **`mklink /D`（软链，非 junction `/J`）**；bat 名 `ccpanes_mcpe_symlink_<uuid>.bat`（CRLF 行尾）；桥模组 behavior pack `BRIDGE_UUID="183b0dc7-ae4a-48fb-800d-9d68f7162e7e"` + module uuid `e468c44c-d970-4598-abd4-ed03a4e34b42` + `min_engine_version [1,18,0]`（manifest.json，**不改**）；world_copy 白名单仅 `db/`+`level.dat`；NBT 全 LittleEndian；`[Python]` 前缀；log_filter 引擎黑名单仅 `UniSDK`。

### 1.4 5 个 MCP 工具（(B)，接进 XK orchestrator）

`orchestrator_service.rs:2664-2810` 5 工具 → XK `internal/services/orchestrator/`（新文件 `orchestrator_mcpe_tools.go`，Windows build tag）。`mcp.AddTool` 注册法 + handler 签名 `(ctx, *mcp.CallToolRequest, in T) (*mcp.CallToolResult, any, error)`（见 orchestrator_tools.go 既有 9 工具样板）。

| 工具 | 入参 | 行为 | 返回 |
|---|---|---|---|
| `capture_window` | — | 取桥+校验 connected → `current_mc_pe_pid` → `capture_window_by_pid` → base64 | **`mcp.ImageContent{Data: jpegBytes, MIMEType:"image/jpeg"}`**（传**原始字节**，go-sdk 自动 base64，**别自己 base64**） |
| `click_window` | `x,y` (f32, **无 camelCase**) | 校验 connected → pid → `click_window_by_pid(pid,x,y)` | 文本 `已点击 (x=…, y=…)` / `错误: …` |
| `execute_mc_code` | `code, isClient?` (camelCase) | `isClient?execute_client_code(type4):execute_server_code(type3)` → `send_packet`（**不查 connected**） | 文本（成功文案统一改 `用 get_mc_pe_logs 看日志`，修 cc-pane 自相矛盾） |
| `reload_mc` | `kind, fileName?` (camelCase) | kind→`game=5/addon_and_game=8/all_shaders=6/single_shader=7`(fileName 必填) → `send_packet` | 文本 |
| `get_mc_pe_logs` | `lines?` (camelCase) | `lines=unwrap_or(200).min(2000)` → `recent_logs(None, lines)`（**不碰桥**） | 文本（多行 join，`HH:MM:SS [Category] [LEVEL] line`） |

工具中文 description 逐字照搬（见 orchestrator_service.rs:2663/2701/2728/2760/2780）。`MCP image content` wire 形 `{"type":"image","data":"<裸base64>","mimeType":"image/jpeg"}` —— go-sdk `ImageContent.MarshalJSON` 产同形，已验证可行。
**接线**：`orchestrator.Deps` 加窄接口 `MCPE`（`ActiveRunPids()/RecentLogs()`）+ `Bridge`（`SendPacket()/Status()`）+ `Capture/Click`（Windows）；nil 兜底返「错误: …」文本（仿 Notifier）。

### 1.5 前端 5 组件 + service/store/types/events（(A) UI）

cc-pane `web/components/mc_pe/` + `web/services/mcPeService.ts` + `web/stores/useMCPEStore.ts` + `web/types/mcPe.ts` → XK `frontend/src/`（MVVM：每组件 View/VM/Style 三件分离 + lang）。

**后端须暴露 10 个 Wails service 方法**（`mcpe.Service`，位置参数）：`ListPackages / CreatePackage(path,name) / UpdatePackage / DeletePackage / ScanLaunchers / Launch(id) / Stop(runId) / MoveWorldToTrash(id) / OpenWorldFolder(id) / GetMcdkBridgeStatus`。
**2 个事件**（`internal/events/events.go`）：`mc_pe:output`（`{runId,packageId,line,severity?,timestamp?,category?,level?}`）、`mc_pe:exited`（`{runId,packageId,code}`）；`mc_pe:launched` 仅后端记账可选。
**数据契约**（Go struct camelCase，落 SQLite）：`MCPackage{id,name,projectPath,gameExecutablePath?,includedModDirs[],world,debugOptions,createdAt,updatedAt}`、`MCWorldConfig`、`MCDebugOptions{...,includeTestMod=桥开关}`、`MCRunSessionInfo{runId,packageId,pid?}`、`MCOutputLine`、`McdkBridgeStatus{port,connected,remoteAddr,connectedAtMs}`。
**前端魔法值**：日志合批 `FLUSH_INTERVAL_MS=50`（20Hz）、`MAX_OUTPUT_LINES=2000`、restart 超时 `5000ms`、桥状态轮询 `2000ms`、severity 缺省 `plain`。
**铁律改造**：`open()`/`window.confirm` → `NativeDialogs`；Tailwind → MUI 9（`theme.palette.foundation.*` + 方圆 12/8 + `*Rounded` 图标）；所有文案 `t()` + `pages`/`components` lang 包；移除 `waitForTauri` gating（Wails bindings 同步可用）；`navigator.userAgent` 判 Windows 可换 `runtime.GOOS` 经一个薄方法（或保留前端判断）。
**Config 面板细节**：`Field` 组件必须模块顶层定义（否则 Input 失焦）；桥状态运行面板 2s 轮询（忠实先保留，后续可改 event 推送）。

---

## 2. 拍板记录（2026-06-01）

- **前端范围**：✅ **全量 5 组件（忠实）** —— MCPEView + 包列表 + 详情双 Tab + 配置面板(487 行) + 运行面板（日志着色 + file:line 跳转 + 桥状态）。MUI + MVVM 重写。
- **Win32 分期**：✅ **拆为独立单元、最后做（去风险）** —— 先桥/启动/execute/reload/日志（可手测 agent 跑 Python/热重载/读日志），capture_window/click_window 隔离成收尾单元单独攻坚（仿 P6a spike 思路）。
- **启动忠实度**：✅ **全忠实照搬** —— 目录软链 `mklink /D` + 无权限时 PowerShell `Start-Process -Verb RunAs` 一次性 UAC 提权；world 删除走回收站（trash）。
- **默认（已采纳，无异议）**：bind `127.0.0.1`；复用桥模组 UUID（替代 cc-pane 而非并存）；缩放 `Lanczos3`→`golang.org/x/image/draw.CatmullRom` 近似（Go 无原生 Lanczos3，**显式偏离**，质量略降）；截图/点击基准差异照搬；launch 走 `ptyx.Spawn`（ConPTY）；`mc_pe_*` 表经 `AllModels` AutoMigrate；MCStudio 路径检测照搬（扫盘 + 手填兜底）；5 工具走 MCP（非 REST，无限流）；桥模组 + level.dat + BSD-3 license 经 `embed.FS` 随附；MCP 工具 + Win32 + symlink/paths 走 `//go:build windows` build tag，非 Windows 留 stub。

---

## 3. 单元切分（7 单元，每单元自洽可编译 + 独立中文 conventional commit，不 push）

> ⚠️ 改 Go 后端必**完整重启 `wails3 dev`** 才生效；改 embed 资源/独立二进制要重 build。MC PE 手测需真机（MCStudio + MC PE 已装、MC PE Panel 勾「启用 MCDK 调试桥」、Bridge 显示「已连入」），用户实跑、agent 盯 `get_mc_pe_logs`/`app.log` 核对。

### P6e-1 — 桥协议 + TCP server（跨平台，无 UI 依赖）
- `internal/services/mcdkbridge/protocol.go`：`PacketType`(3-8) + `Packet` + `Encode/Decode`(BigEndian, HEADER_LEN=6) + 6 工厂；错误码字符串照搬。
- `internal/services/mcdkbridge/bridge.go`：`Service` `net.Listen("127.0.0.1:0")` + accept goroutine + 单 active 顶替 + `SendPacket`/`Status`/`Port`/`Disconnect` + read-drain EOF goroutine。
- 单测：encode 字节布局断言、type id 往返、bind/port 非零、顶替逻辑、无 client 报错。
- 验证：`go build/vet ./internal/services/mcdkbridge/` + `go test` + gofmt。

### P6e-2 — embed 资源 + 路径/安全/包解析（启动只读侧 + 资源）
- `internal/services/mcpe/resources/`（embed）：整目录 `include_test_mod`（含 QuModLibs，一字不改，~57KB）+ `level.dat` + `license/{mcdk,nbt}`。
- `mcpe/bridgemod.go`：`BRIDGE_UUID` + `ExtractTo`（`fs.WalkDir` 覆盖写）。
- `mcpe/paths_windows.go` + `paths_other.go`（stub）：mc_dir/game_dir/behavior_packs_dir/resource_packs_dir/worlds_dir/scan_launchers(`GetLogicalDrives`)/validate_launcher。
- `mcpe/safety.go`：validate_relative_name + validate_command_line_path（FORBIDDEN）。
- `mcpe/pack.go`：PackType/PackInfo/parse_pack(深度≤2)/parse_manifest。
- 单测：safety 拒列、pack 扫描深度、bridgemod UUID 与 manifest.json 一致断言、embed 文件齐全。
- 验证：`go build/vet/test`（含非 Windows 编译 stub）+ gofmt。

### P6e-3 — world 准备 + level.dat 小端 NBT + symlink（含 UAC 提权）
- `mcpe/worldcopy.go`：copy_world_essentials + copy_dir_recursive + remove_dir_safely + trash 删除（`mcpe/trash_windows.go` SHFileOperation/IFileOperation FFI + `trash_other.go` stub）。
- `mcpe/leveldata.go`：小端 NBT 读写器 + read/write_level_dat + write_default_level_dat(embed)。
- `mcpe/symlink_windows.go` + `symlink_other.go`：remove_symlinks_in + create_symlink(1314 判定) + sudo_create_symlinks(mklink /D .bat + PowerShell RunAs via procx)。
- 单测：NBT round-trip 字节布局、level.dat 11 字段写入读回、world_copy 白名单（临时目录）、symlink 1314 路径用 fake（提权 .bat 生成内容断言，不真弹 UAC）。
- 验证：`go build/vet/test` + gofmt。

### P6e-4 — launch 12 步编排 + launcher(ConPTY) + 日志缓冲/过滤 + storage 表 + events + app 装配
- `mcpe/logbuffer.go`（+`sync.Mutex`）、`mcpe/logfilter.go`（5 规则 + detect_severity + parse_python_header）。
- `mcpe/launcher.go`：RunSession + spawn(`ptyx.Spawn` ConPTY cols=200/rows=50) + reader/wait goroutine + kill。
- `mcpe/service.go`：MCPEService + Launch(12 步) + active_run_pids + recent_logs + stop + cleanup_all + CRUD + LogCapturingEmitter。
- `internal/storage/models_mcpe.go`：MCPackage(+嵌入 world/debug) GORM model + `init()` append AllModels + TableName。
- `internal/events/events.go`：注册 `mc_pe:output` / `mc_pe:launched` / `mc_pe:exited`。
- `internal/app/app.go`：构造 `mcpeSvc := mcpe.New(holder)`（注册进 Services 供前端调）+ `mcdkBridge := mcdkbridge.NewService()` 启动（fail-open）+ `SetEmitter(mirrorEmit)` 双路 + `defer cleanup_all/Stop`。
- 单测：launch 各步（pack/symlink/level.dat 临时目录）、log_filter 白名单/丢弃枚举、logbuffer FIFO/tail、CRUD。
- 验证：`go build/vet/test` + `wails3 generate bindings`（含 mcpe service）+ gofmt。**后端可启动但无 UI 触发**（live 手测在 P6e-5）。

### P6e-5 — 前端 MC PE 面板（全量 5 组件，MVVM）→ **首个 live 手测点**
- `frontend/src/.../MCPE*`：MCPEView + MCPackageList + MCPackageDetail + MCPackageConfigPanel + MCPELaunchPanel（View/VM/Style + lang）+ store（事件订阅 + 合批 + 截断 + restart 超时）+ service（10 方法）+ types（camelCase 契约）。
- 铁律改造：NativeDialogs / MUI / t() / *Rounded / 移除 waitForTauri。
- 验证：`pnpm typecheck`(0) + vitest + `pnpm build` + 后端全绿。
- **手测**：MC PE 面板配置包（项目/启动器扫描或手填/世界/热键/勾「启用 MCDK 调试桥」）→ 启动 → 日志实时流（着色 + file:line）→ 桥状态「监听 :port」→ 游戏内模组反向连入显示「已连入」→ 停止/重启。

### P6e-6 — 3 个非 Win32 MCP 工具（execute_mc_code / reload_mc / get_mc_pe_logs）
- `internal/services/orchestrator/`：Deps 加 `MCPE`/`Bridge` 窄接口；新 `orchestrator_mcpe_tools.go`（无需 Win32，跨平台或 windows tag 视实现）注册 3 工具 + 中文 description 照搬；nil 兜底文本。
- `app.go`：orchestrator.Deps 注入 mcpeSvc + mcdkBridge。
- 单测：工具 handler（execute/reload 编帧、get_mc_pe_logs lines clamp、nil 兜底）。
- 验证：`go build/vet/test` + bindings + gofmt。
- **手测**：pane 内 Claude 调 `mcp__panes__execute_mc_code(code="print('hi')")` → `get_mc_pe_logs` 看到输出；`reload_mc(kind="game")` 游戏重启世界。

### P6e-7 — Win32 capture_window + click_window（去风险收尾单元）
- `mcdkbridge/window_windows.go` + `capture_windows.go` + `click_windows.go`（手写 LazyDLL.NewProc + INPUT/MOUSEINPUT/BITMAPINFOHEADER struct）+ `_other.go` stub。
- `orchestrator_mcpe_tools.go`：补 capture_window（ImageContent）+ click_window 两工具。
- 前端运行面板：截图预览/点击交互（如 cc-pane 有则补，否则纯 MCP）。
- 单测：window 选择策略（标题/面积，fake EnumWindows 难，至少结构 + 常量断言）、坐标归一化公式、struct size 断言（INPUT=40、BITMAPINFOHEADER=40）。
- 验证：`go build/vet/test` + bindings + gofmt。
- **手测**：Claude 调 `capture_window` 拿到 480p JPEG 多模态识图 → `click_window(x,y)` 归一化点击命中游戏内 UI。

---

## 4. 风险 / 待澄清（实现中遇到再停下问）

- **GBK/UTF-8 日志解码**：cc-pane `from_utf8_lossy`（丢无效字节）。Go 无等价；**默认先照搬 lossy**（`strings.ToValidUTF8` 替换），若中文日志乱码再上 `golang.org/x/text/encoding/simplifiedchinese` 先 GBK 解码（mindfs 经验倾向后者）。手测时核对中文日志可读性。
- **截图/点击基准不一致**（GetWindowRect vs GetClientRect）：忠实照搬，P6e-7 手测验证点击偏移；若偏移明显再议补偿。
- **Lanczos3→CatmullRom**：显式偏离（Go 无原生 Lanczos3），画质略降，手测确认可接受。
- **ConPTY 复用 ptyx**：`ptyx.Spawn` 给真 console（mod 层 Python stdout 可见），但 ptyx 为交互终端设计（含 xterm 批处理）；MC PE 复用其 PTY/JobObject，但跑自己的 read→filter→buffer→emit 循环（不走 terminal 的批处理）。实现前确认 `ptyx.Spawn` 签名 + 是否暴露 reader/pid/kill。
- **UAC 提权从 Go 实现**：PowerShell `Start-Process -Verb RunAs -Wait` 经 procx，需确认 procx 支持 CREATE_NO_WINDOW（0x08000000）+ 同步等待 UAC（请求 goroutine 内阻塞可接受）。
- **trash（回收站删除）**：SHFileOperation/IFileOperation FFI（FOF_ALLOWUNDO），Go 无内置，手写 FFI；P6e-3 落地。
- **Win32 struct ABI**：INPUT/MOUSEINPUT/BITMAPINFOHEADER 手写 + `unsafe.Sizeof` 断言（SendInput/GetDIBits 对 size 敏感）。
- **DPI API**：`SetThreadDpiAwarenessContext` 需 Win10 1607+（旧系统 no-op）。
- **桥状态推送**：前端 2s 轮询（忠实）；后续可改 event 推送（偏离，省）。
- **typeID 1 AUTO_RELOAD / 2**：Go 端不发（脚本热更靠游戏内按键 `updata_all`）；ExecuteServerCode(type3) 游戏侧是 stub（仅报错让用 isClient=true）——execute_mc_code 默认 isClient=false 走 type3 会被游戏拒，**手测注意默认行为**（cc-pane 原样，agent 通常传 isClient=true）。

---

## 5. Go 移植技术映射补充（Win32 FFI）

- `x/sys/windows` 已在 go.mod（v0.44.0）；**新增 `golang.org/x/image`**（CatmullRom 缩放）→ `go mod tidy`。
- GDI/User32 多数符号无现成 wrapper → `windows.NewLazySystemDLL("gdi32.dll"/"user32.dll").NewProc(...)` + `proc.Call(...)`：`GetDC/CreateCompatibleDC/CreateCompatibleBitmap/SelectObject/BitBlt/GetDIBits/DeleteDC/DeleteObject/ReleaseDC/PrintWindow/SetThreadDpiAwarenessContext/SendInput/AttachThreadInput/GetForegroundWindow/SetForegroundWindow/BringWindowToTop/GetClientRect/GetWindowRect/ClientToScreen/IsIconic/ShowWindow/GetSystemMetrics/EnumWindows/GetWindow/GetWindowText*W/GetWindowThreadProcessId/IsWindowVisible`。
- `GetLogicalDrives` → `windows.GetLogicalDrives()`；`GetLastError` → `windows.GetLastError()`；回调 → `windows.NewCallback`；UTF-16 → `windows.UTF16ToString`；RAII Drop guard → Go `defer`（注意 SelectObject 还原须先于 DeleteObject）。
- `image/jpeg`(q75) + `encoding/base64.StdEncoding`（裸 base64）+ `encoding/binary`（BigEndian 桥帧 / LittleEndian NBT）全标准库。

---

## 变更记录
- 2026-06-01：10-agent 精读（`wf_7078158a`）产出本计划；拍板 §2（前端全量 / Win32 拆末单元 / 启动全忠实）；待最终过目后进 P6e-1。
