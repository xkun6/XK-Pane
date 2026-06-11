# XK-Pane 减法审计报告

> 审计日期：2026-06-10 · 分支：`feat/paseo-worktree-composer` @ c454543（工作树干净）
> 方法：多代理工作流（run id `wf_79d1b64e-2ca`，77 个代理）——9 个维度并行排查 → 每条发现由独立核查代理对抗性反驳（覆盖 bindings / webapi 路由 / 事件字符串 / 动态字符串引用 / React.lazy / Taskfile 引用面）→ 完整性查漏补扫。
> 结果：**59 条减法成立，7 条被驳回**；另有 1 条（build/darwin 某资产）核实代理网络故障未获裁决，按跳过处理。
> 本文执行摘要为去重后的可执行清单（同一事项常被多个维度独立发现）；附录 A/B 保留全部 66 条原始明细（含 grep 证据与风险全文），编号 `#N` / `R-N` 与附录对应。

## 总收益概览

| 类别 | 规模 |
|------|------|
| 死代码 | 约 8500 行（前端 x-pro 3534 + 子窗口 ~730 + subprocess ~460 + session ~242 + build/ios+android ~3200 + 零散 ~300） |
| 过期一次性文档 | 约 4181 行（根目录 17 个计划/进度文档）+ skill 文档约 700KB 级 |
| 二进制 / 安装包体积 | 约 320KB（embed 的死资产） |
| npm 依赖 | `@mui/x-data-grid`（node_modules 约 8.8MB）；可选 `@mui/x-charts`（12.4MB，低置信度） |
| 运行时开销 | 1 个每秒空转的常驻 goroutine（time 事件演示链） |

---

## 第一档：无争议，可直接删（全链路查无引用）

1. **`frontend/src/components/x-pro/` 整目录** — 48 文件 / 约 3534 行，最大单项。foundation 预置的自实现 MUI Pro/Premium 库（列钉住/行分组/聚合/Heatmap/Sankey/Gantt 等），全应用零引用。连带删 `@mui/x-data-grid` 依赖（x-pro 之外零消费）与 `.claude/skills/foundation-mui-x-data-grid/`、`foundation-mui-x-charts` 中指向 x-pro 的文档。`@mui/x-charts` 本体仍被设置页使用，保留。（附录 #19 #28 #38 #43 #58）
2. **`greet` 演示服务全链路** — 约 35 行：`internal/services/greet/` + `app.go` import/注册两行 + `frontend/src/services/greet/` + barrel 导出一行。删后重跑 `wails3 generate bindings`，并同步根 CLAUDE.md / frontend 文档里以它为例的段落。（附录 #1 #6 #22 #40 #46）
3. **`time` 演示事件链** — 约 45 行 + 消除每秒一次空转：`emitTimeLoop` goroutine、`events.Time` 常量与注册、前端 `useTimeEvent.ts`、`shared/events.ts` 一行。硬耦合两处：`events.test.ts` 事件名镜像表同删；`hub_test.go:85` 先把 `events.Time` 反例换成字面量字符串。（附录 #2 #8 #21 #47）
4. **`frontend/public` 死资产** — 约 320KB：`Inter-Medium.ttf`（无任何 @font-face 加载，从未参与渲染）、`react.svg`（Vite 模板残留）、`Inter Font License.txt`。`wails.png` 仍是 favicon，保留。（附录 #3 #54）
5. **`build/ios` + `build/android` 整目录** — 45 文件 / 3200+ 行（含入库的 gradle-wrapper.jar）。删目录与根 `Taskfile.yml` 两个 include **必须同一次提交**（include 缺失会让所有 task 解析失败）；删后跑一次 `wails3 task common:update:build-assets` 验证回流。（附录 #51）
6. **构建链死任务**：`frontend:vendor:puppertino`（引用文件与锚点全不存在，约 35 行）；MSIX 打包链路（引用不存在的 `wails.json`，本来就是坏的）。（附录 #52 #53）
7. **Go 零散死代码** — 合计约 250 行：`provider.EnvVars`、`storage.MustOpen`、`bedrock.Warm`、`localhistory` 仅测试可达的三方法（`CompressBlobs`/`ListDirectoryChanges`/`ListVersionsByBranch`）、`cliadapters.Registry.IDs`、`httpx.PostJSON`/`httpx.ReadJSONLimit`/`filex.EnsureDir`/`logx` 包级四函数、`orchestrator:session-started` 事件发射（零订阅方）、`mcdkbridge` 协议读半边与 `Disconnect`（仅测试使用，medium）。（附录 #9–#16）
8. **前端零散死文件** — 约 11 个文件：8 个 `pages/*/index.ts` 死 barrel + `childwindows/index.ts`、`HistoryPage.skeleton.tsx`（历史页迁入右侧抽屉后失去挂载点）、`useTerminalPage.ts`（返回空对象的占位 hook）、`isLinux`/`AppEventName`/`UseCliToolsResult` 等零散死导出。（附录 #23–#25 #27）
9. **i18n 死键** — 18 键 × 2 locale ≈ 36 行：`terminal.tab.{new,menu}`、`common.{remove,reset,yes,no,error,followSystem,auto}`、`sessionTimeline.{showTerminal,showChat}`、`history.timeline.{title,current}`、`history.tag.placeholder`、`todo.item.overdue`、`todo.editor.scopeRefPlaceholder`、`ssh.launchMenu.bareTerminal`、`settings.provider.title`。已排除动态拼 key 误报。（附录 #31–#36）
10. **`wails-skill-update` 残迹**：`INDEX.md` 死引用约 3 行 + `.gitignore` 三行死条目。（附录 #59）

## 第二档：技术上成立，但删前需拍板（功能取舍）

11. **子窗口系统整链路** — 约 730 行代码 + 588 行 skill 文档。前后端全就位但无任何打开者。它是 b25bc86 有意从上游 foundation 整合的框架能力，整删会与上游分叉。二选一：整删（`childwindows/`、`services/childwindow/`、`child.tsx`/`child.html`、vite child 入口、`internal/app/childwindow.go` + app.go 三处接线 + `foundation-childwindow` skill）；或保留能力但删无人用的 `sendToWindow`/`broadcast`/`onChildResult`/`onChildClosed`。（附录 #20 #39 #45）
12. **`subprocess` 白名单服务端到端** — 约 460 行（Go 341 + TS 107）。白名单恒空 + 前端零消费者，是 CLAUDE.md 铁律的投机性实现体。删实现、铁律文字改为「需要时按 git 历史恢复」；同步改 `foundation-utils` SKILL 中以它为示例的段落。（附录 #26 #44）
13. **`session` 服务整包** — 约 242 行。已注册但前端无包装层、三个绑定方法全仓零调用。**注意**：这是「跨重启恢复终端布局」的后端先行（progress.md:761 列为待做、b0a3529 做过 sshConfig 脱敏分层）。该功能还要做就别删；放弃了再删，并连带决定 `TerminalSession` 模型去留（保留则数据存储页恒显一张 0 行表）。（附录 #7）
14. **根目录 17 个一次性计划/进度文档** — 4181 行 / 约 600KB（P3~P7 系列 + planning-with-files 三件套）。**建议归档到 `docs/archive/` 而非直接删**：4 处 Go 注释以「见 P6e_PLAN.md §N」引用拍板出处；跨会话记忆有指针指向 `P7_WEB_PLAN.md`（Web 镜像线仍在推进）与 `progress.md`。（附录 #55）
15. **`.claude/skills/xkpane-*` 11 个文件停止入库** — app 注入产物，与 embedded 模板字节级重复，每次改模板仓库自动变脏。`git rm --cached` + `.gitignore` 追加（对齐 `/.codex/` 处理），XK-Pane 启动会话时 `syncClaudeSkills` 自动重建。会推翻 180b072 的决策（纯 clone 不经 XK-Pane 时这批 skill 缺失），但它们全依赖 panes MCP，脱离 app 基本无用。注意 gitignore 通配会静默吞掉将来手写的 `xkpane-*` 前缀新 skill。（附录 #56）

## 第三档：合并 / 简化类（收敛而非删除）

16. **webapi `handleSessionTimeline` 与 `transcriptsvc.GetTimeline` 整段重复** — 约 60 行双维护点，rest.go 218-295 可收缩到 ~20 行。（附录 #37）
17. **小工具函数多份拷贝** — Go `normalizePath` ×3（两份逐字节相同）、前端 `basename` ×6、`isValidResumeId` ×3；不止行数，更是 Windows 路径大小写口径漂移隐患。（附录 #41）
18. **WorkspaceTree / WebWorkspaceTree** — 不淘汰任何一边（双端定位不同），但「会话子列表 + resume 守门」约 80-120 行重复 VM 逻辑可抽公共 hook。（附录 #42）
19. **httpx 收缩** — 单例 Client / JSON helper / 整个 retry.go 零调用，实际只有 proxyclient 在用，可收缩 99~333 行（视保留策略）。（附录 #48）
20. **`golang.org/x/net` 降级** — stdlib 已原生支持 socks5/socks5h 代理，proxyclient 的 socks5 分支可整段删，依赖降为 indirect。（附录 #17）
21. **`transcriptsvc` 两个单实现接口去仪式** — 包内已 import 具体类型、无测试 fake，约 10 行。（附录 #50）
22. **投机选项字段** — `procx.Spec.Stdin` 与 `logx.Config` 多数字段从未被设置非默认值，约 20-25 行。（附录 #49）
23. **`@mui/system` 版本错位修复** — ^7.3.11 vs material 要求 ^9.0.1，pnpm 装了两份副本、bundle 双份运行时。严格说是修复不是减法，顺手对齐。（附录 #29）
24. **可选（低置信度）**：`@mui/x-charts` 整包 12.4MB 只服务设置页一个子页面的两张图，换简单展示可去依赖。（附录 #30）
25. **文档减法** — README 整本仍是 foundation 脚手架说明书（改名引导早已执行完毕）；根 CLAUDE.md 模块图/服务清单/数据目录与现状脱节约 100-120 行；frontend/CLAUDE.md 有指向已不存在文件的小节。删第一、二档代码时必须同步，否则脱节面扩大。（附录 #4 #5 #57）

## 驳回清单（看着像减法，别动）

- **两套项目表（projects vs workspace_projects）合并** — 「写路径断绝」前提在仓库全局视角不成立，写入口仍可达。（附录 R-5）
- **Docker / 服务器模式构建链** — 是活的：`server` build tag 由 wails 框架自身消费（application_server.go 等），grep 不到承接是机制性误判。（附录 R-6）
- **build/darwin + build/linux** — 非目标 OS 上自动切 docker 跨编译，是真实可用的发布通道；删除以「只发 Windows」这个未做出的决策为前提。（附录 R-7）
- **go.mod** — 实测 `go mod tidy -diff` 零输出，17 个 direct 依赖零浪费。`bep/debounce` 换 stdlib 省不了二进制（wails 链入它）；`go-git/v5` 同理减不动，留作 wails 升级后的条件触发项（附录 #18）。（附录 R-2 R-3 R-4）
- **`todo.BatchUpdateStatus`** — 当前无调用方属实，但属规划内能力，核查员驳回删除。（附录 R-1）

## 执行提示

- 按「一档一批一提交」推进；每批后跑 `wails3 generate bindings` + `cd frontend && pnpm typecheck` + `go build ./... && go test ./...`（typecheck 会自动抓出漏删的前端引用）。
- 涉及后端 service 删除的批次，`app.go` 的 import + RegisterService 与包目录必须同批删，bindings 重新生成。
- 文档（README / 两份 CLAUDE.md / 相关 SKILL）与代码同批改，避免再次脱节（仓库已有 foundation-systray 文档脱节先例）。
- 本文件若执行第 14 项归档动作，可一并移入 `docs/archive/`。

---

# 附录 A：59 条确认发现全文

（以下由审计结果 JSON 程序化导出，未经删改；同一事项可能被多个维度重复发现，以执行摘要的去重编号为准。）

## #1 【脚手架残留·high】greet 演示服务全链路死代码：后端注册着、前端导出着，但无任何调用方

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/greet/greet.go、J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/frontend/src/services/greet/GreetService.ts、J:/Program/Agent/XK-Pane/frontend/src/services/index.ts
- **估计规模**：约 35 行代码（后端 17 行包 + app.go 2 行 + 前端 service 约 12 行 + 导出 1 行）+ 1 个 Go 包目录 + 1 个前端 service 目录 + 生成的 bindings 文件

**证据**：已按 bindings 规则反查前端：grep frontend/src 中 `GreetService|services/greet` 仅命中 services/index.ts:1 的 re-export 与 GreetService.ts 自身定义，无任何组件 / ViewModel / 页面 import（含从 '@/services' 桶文件解构的情况）。HomePage 已重写为仪表盘（useHomePage.ts 只用 LaunchHistoryService / useProjects / useCliTools，View 里的 greeting 是时段问候 i18n key，与 GreetService 无关）。internal/webapi 全目录 grep 'greet' 零命中（Web 镜像端也不用）。后端仅 app.go:24 import + app.go:190 `application.NewService(greet.New())` 注册。progress.md:928 也自记：「遗留孤儿（记一笔，未删）：GreetService / useTimeEvent / 后端 greet service + emitTimeLoop 现无消费方（原仅 HomePage demo 用）」。

**建议**：删除 internal/services/greet/ 整包、app.go 的 import 与 NewService 注册两行、frontend/src/services/greet/ 目录、services/index.ts 的导出一行，然后 `wails3 generate bindings` 重新生成（bindings 目录自动少一个 greet 包）。同步把根 CLAUDE.md 通信链路图（29-31 行）和 frontend/CLAUDE.md 交互边界表 / FAQ 里以 GreetService 为例的段落换成现存活服务（如 preferences）。

**对抗核实裁决（成立）**：逐项反查均未找到任何真实调用方，反驳失败：① bindings 调用：对 frontend/src 全目录做大小写不敏感 grep 'greet'，仅命中 frontend/src/services/greet/GreetService.ts 自身（import '@bindings/xkpane/internal/services/greet'）和 frontend/src/services/index.ts:1 的桶 re-export；逐一核对全部 70+ 处 `from '@/services'` 的桶导入，无一解构 GreetService。HomePage 的命中全是 `greeting`（GreetingKey 时段问候 + i18n key `home.greeting.*`），与 GreetService 无关。② webapi 路由：internal/webapi 全目录 grep 'greet' 零命中，Web 镜像端不用。③ 事件字符串：internal/services/greet/greet.go 全文仅 16 行、只有一个纯函数 Greet(name) string，不发任何事件；internal/events 无 greet。app.go:382/433 的 emitTimeLoop 属独立的 time 事件演示，不在 greet 包内，删 greet 不受影响。④ 动态字符串引用：hooks/cliadapters/subprocess 白名单/i18n key/路由 path 在 internal 与 frontend/src 的全仓库 grep 中均无 'greet'（除上述四个文件）。⑤ React.lazy：lazy 仅用于页面 chunk，grep 全覆盖未见 GreetService。⑥ Taskfile/build：零命中。⑦ 非新功能：greet 是 foundation 脚手架时代的原始演示服务，与最近 5 个 worktree/composer commit 无关；progress.md:928 自记「遗留孤儿（记一笔，未删）：GreetService / 后端 greet service 现无消费方」。后端引用面仅 internal/app/app.go:24（import）与 app.go:190（application.NewService(greet.New()) 注册），属全链路死代码，建议的删除范围（后端包 + 注册两行 + 前端 wrapper + index.ts 导出一行 + regen bindings）完整且正确。

**执行风险**：技术风险接近无：删除后必须执行 `wails3 generate bindings` 重新生成（bindings 目录被 .gitignore，会自动少掉 greet 包），若漏删前端 GreetService.ts/index.ts 导出，pnpm typecheck 会因 bindings 路径消失而报错，本身是安全网。两个轻微注意点：① 文档需同步——根 CLAUDE.md 通信链路图（约 29-31 行）、README 改名指南第 3 步、frontend/CLAUDE.md 与 frontend/DEVELOPMENT.md 中以 GreetService 为例的段落要换成现存服务（如 preferences），否则文档与代码脱节（.claude/skills/wails-* 里的 greet 是通用 Wails 教程示例，无需改）；② greet 是脚手架定位的「最小示例服务」，删除后新项目使用者少一个参照模板，但仓库已有大量真实 service 可参照，影响可忽略。

## #2 【脚手架残留·high】time 演示事件链路：后端每秒空跑一个永不停止的 goroutine，前端 useTimeEvent 无人消费

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/internal/events/events.go、J:/Program/Agent/XK-Pane/frontend/src/shared/hooks/useTimeEvent.ts、J:/Program/Agent/XK-Pane/frontend/src/shared/events.ts、J:/Program/Agent/XK-Pane/frontend/src/shared/events.test.ts、J:/Program/Agent/XK-Pane/internal/webapi/hub_test.go
- **估计规模**：约 45 行代码 + 消除每秒 1 次的无效事件 Emit 与白名单过滤开销 + 1 个常驻 goroutine

**证据**：app.go:382 `go emitTimeLoop(app)` 启动、431-440 行定义：ticker 每秒 `app.Event.Emit(events.Time, ...)`，注释自称「演示后端主动推事件给前端的链路」。前端 grep frontend/src 中 `useTimeEvent` 仅命中其自身定义（useTimeEvent.ts:11）和 useTrayNavigation.ts:6 的一条注释，无任何组件 import。Web 端也不消费：webapi 事件白名单不含 time，hub_test.go:85 恰好把 `g.RouteEvent(events.Time, "12:00")` 当作「非白名单事件应被丢弃」的反例 fixture。额外代价：app.go:241-245 的 mirrorEmit 会把每次 Emit 都送进 webGateway.RouteEvent 做一次白名单过滤——即每秒一次纯空转广播。

**建议**：删除 emitTimeLoop 函数及 app.go:382 的启动行、events.go:7 的 Time 常量与 :173 的 RegisterEvent、前端 useTimeEvent.ts 整文件、shared/events.ts 的 `Time: 'time'` 一行、events.test.ts 镜像断言一行；hub_test.go:85 的白名单反例改用任意字面量字符串（如 "demo:unlisted"）即可保留测试语义。frontend/DEVELOPMENT.md 第 78/158/165/191 行以 AppEvents.Time / useTimeEvent 为范例的文档段落换成现存事件（如 localhistory:changed）。

**对抗核实裁决（成立）**：对抗性反查全部失败，减法成立。逐项证据：① bindings 维度——time 事件链路根本不经 bindings（前端用 @wailsio/runtime 的 Events.On 字符串订阅），且 grep frontend/src 确认 useTimeEvent 仅在其自身定义文件 frontend/src/shared/hooks/useTimeEvent.ts:11 出现，另一处 useTrayNavigation.ts:6 只是注释，无任何组件（含原始消费方 HomePage）import 它；② webapi 维度——internal/webapi/rest.go:29-33 的 Web 镜像事件白名单仅含 TerminalOutput/TerminalStatus/TerminalExit/TerminalAskPending/OrchestratorLaunch 五项，不含 events.Time；hub_test.go:85 正是把 `g.RouteEvent(events.Time, "12:00")` 当作「白名单外应被丢弃」的反例 fixture，反向坐实 Web 端不消费；③ 动态字符串维度——全仓 grep `Emit("time")` / `On("time")` / `RouteEvent("time")` 仅命中根 CLAUDE.md:30 的文档示意，hooks/cliadapters/subprocess 白名单/i18n/路由 path 均无 'time' 事件引用；'time' 字面量在 frontend/src 仅存在于 shared/events.ts:5 与 events.test.ts:7 的镜像表；④ Go 侧——events.Time 的全部引用就是 events.go:7/173（定义+注册）、app.go:382/437（emitTimeLoop 自产自销）、hub_test.go:85（反例）；⑤ 非新功能——app.go:431 注释自称「演示后端主动推事件给前端的链路」，是 foundation 脚手架初始 demo，与最近 5 个 worktree/composer commit 无关；更决定性的是仓库自己的 progress.md:928 已明文记录「遗留孤儿（记一笔，未删）：GreetService / useTimeEvent / 后端 emitTimeLoop 现无消费方（原仅 HomePage demo 用），后续可作脚手架清理」。一处证据更正：原发现称 mirrorEmit（app.go:243-246）会把该事件每秒送进 webGateway.RouteEvent 空转过滤——不准确，emitTimeLoop 在 app.go:437 直接调 app.Event.Emit，不走 mirrorEmit（后者只注入给 termSvc/mcpeSvc/bedrockDocSyncSvc），所以实际代价是「每秒一个 ticker + 一次无订阅者的桌面总线 Emit」，没有 Web 网关空转；代价比原述略小，但死代码结论不变。

**执行风险**：功能上无风险（运行时零消费方），但执行时有四处需同步否则构建/测试/文档会出问题：① frontend/src/shared/events.test.ts:7 的 EXPECTED_BACKEND_EVENT_NAMES 镜像表必须与 shared/events.ts:5 同删 Time，否则 vitest 断言红；② internal/webapi/hub_test.go:85 依赖 events.Time 作「非白名单」反例，删常量前须按建议改成任意字面量字符串（如 "demo:unlisted"），否则 go test 编译失败；③ 文档同步范围比建议列的更大——除 frontend/DEVELOPMENT.md 外，根 CLAUDE.md:30-31 的通信链路图、frontend/CLAUDE.md:30（事件订阅表）与 :434（文件清单）也以 time/useTimeEvent 为范例，需一并换成现存事件（如 localhistory:changed）；④ 删 events.go:173 的 RegisterEvent 后需重跑 wails3 generate bindings（生成的事件类型代码会变化，但前端不消费 time 的生成类型，无下游影响）。另一层非技术考量：该仓库 CLAUDE.md 仍自称「脚手架」，time 链路是其「后端推事件」教学样例，删除会让脚手架少一个最小示例——但仓库已有多个真实事件链路可作范例，且 progress.md 自己已把它标记为待清理孤儿，此考量不构成保留理由。

## #3 【脚手架残留·high】frontend/public 两个零引用脚手架资源被 embed 进 Go 二进制：Inter-Medium.ttf（315KB）与 react.svg

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/public/Inter-Medium.ttf、J:/Program/Agent/XK-Pane/frontend/public/react.svg、J:/Program/Agent/XK-Pane/frontend/Inter Font License.txt
- **估计规模**：约 320KB 二进制 / 安装包体积 + 3 个文件

**证据**：全 frontend（排除 node_modules/dist/bindings）grep `@font-face|\.ttf`：唯一的 @font-face 在 index.html:14，加载的是 /fonts/JetBrainsMono-Regular.woff2（终端字体，活引用）；没有任何地方加载 Inter-Medium.ttf。index.html:63 / buildMuiTheme.ts:15 / child.html:12 的 font-family 栈虽写了 "Inter"，但无 @font-face 时该名只会命中用户系统已装字体，ttf 文件从未参与渲染——删除零视觉影响。react.svg 是 Vite React 模板残留，grep 仅 frontend/CLAUDE.md:398 的文件清单提到。public/ 会被 Vite 原样拷进 dist 并经 main.go embed.FS 打进每个平台的二进制。文件实测大小：ttf 315132 字节、svg 4126 字节。

**建议**：删除 frontend/public/Inter-Medium.ttf 与 frontend/public/react.svg；ttf 删除后 frontend/Inter Font License.txt 也失去存在意义可一并删（若担心 font stack 里 "Inter" 字样构成许可证关联可保留）。wails.png 仍被 index.html:5 当 favicon 引用，保留。

**对抗核实裁决（成立）**：反驳失败，减法建议成立。逐项反查均无活引用：(1) 全 frontend 唯一 @font-face 在 index.html:14-20，加载的是 /fonts/JetBrainsMono-Regular.woff2，没有任何 @font-face 或 url() 指向 Inter-Medium.ttf；index.html:63、child.html:12、buildMuiTheme.ts:15 的 font-family 栈虽含 "Inter"，但无 @font-face 时该名只匹配用户系统已装字体，public 里的 ttf 从未参与渲染。(2) child.html 完整核读，无任何资源 href。(3) Go 侧全仓 grep Inter/.ttf/react.svg/public，命中全是 Internal/Interval 等无关子串；internal/webapi 只是把同一份 dist embed 静态提供，Web 镜像端同样不加载这两个文件。(4) build/Taskfile.yml 仅引用 frontend/public/puppertino/，与这两文件无关。(5) frontend/src 全量 grep .svg/.ttf/Inter 零命中，无动态 import、无路由/i18n/hooks/子进程白名单字符串引用。(6) favicon 是 /wails.png（index.html:5），react.svg 非 favicon。(7) main.go:10 的 //go:embed all:frontend/dist 确认 Vite 原样拷贝的 public 内容（实测 ttf 315132 字节 + svg 4126 字节）被打进每个平台二进制。(8) 两文件是 wails 模板初始脚手架残留，与最近 worktree/composer 新功能无关。唯一提及处是 frontend/CLAUDE.md:398-399 的文件清单与 wails-quickstart skill 参考文档，均为文档性描述而非代码引用。

**执行风险**：几乎无运行时风险：删除前后渲染行为完全一致（ttf 本来就从未被加载，font stack 的 "Inter" 继续按系统字体匹配）。两个次要收尾点：① frontend/CLAUDE.md:398-399 的 public/ 文件清单需同步删去两行，否则文档与实际不符；② frontend/Inter Font License.txt 在 ttf 删除后失去对应物，可一并删（font-family 栈仅出现 "Inter" 字样不构成字体分发，无许可证义务），保留亦无害。

## #4 【脚手架残留·high】README.md 整本仍是 foundation 脚手架说明书：「改名引导」早已执行完毕，留着只会误导

- **涉及文件**：J:/Program/Agent/XK-Pane/README.md、J:/Program/Agent/XK-Pane/CLAUDE.md、J:/Program/Agent/XK-Pane/frontend/CLAUDE.md
- **估计规模**：README 约 60-80 行 + 两份 CLAUDE.md 各若干段落（约 20-30 行）

**证据**：go.mod 第 1 行已是 `module xkpane`，前端 GreetService.ts:6 实际 import `@bindings/xkpane/internal/services/greet`——即改名指南的核心步骤早已完成。但 README.md:1 标题仍是 "Foundation"，:124-131 还在教用户 `git clone .../foundation.git my-app` 并改名，:297-325 是整张 11 步改名对照表（其中第 8 步指向的 GreetService.ts 改名点也已过时）。根 CLAUDE.md 仍自称「Foundation - Wails v3 桌面应用脚手架」且通信链路图写 `frontend/bindings/foundation/...`；frontend/CLAUDE.md 交互表、FAQ 多处写 `@bindings/foundation/...`（实际前缀是 xkpane）。这些文档每次会话都会注入 AI 上下文，模块名错误会直接误导改码。

**建议**：删除 README.md 的「克隆 + 改名」「✏️ 把脚手架改名为你的项目」整节及脚手架推销性段落（改名已完成，属一次性引导）；根 CLAUDE.md 与 frontend/CLAUDE.md 中所有 `foundation` 模块名 / `@bindings/foundation` 路径例子，随 greet/time 链路删除一并清掉或改为 xkpane 现实路径。注意 `theme.palette.foundation.*`、`foundation-light` 等前端命名空间是活代码标识符，不在删除范围。

**对抗核实裁决（成立）**：反驳失败，减法建议成立。逐项实证：① 改名已彻底完成——go.mod:1 为 `module xkpane`；frontend/src 共 48 处 bindings import 全部是 `@bindings/xkpane/...`，grep `@bindings/foundation` 零残留（GreetService.ts:6 实测 `@bindings/xkpane/internal/services/greet`）；Taskfile.yml:4 为 `APP_NAME: "xkpane"`。② README 改名对照表（README.md:297-325）连结构都过时了：第 3-5 步指向的三处 AppDirName 已重构集中到 `internal/appinfo/appinfo.go`（`BaseName = "XK-Pane"`，该文件注释自述「改业务名只需改这里的 BaseName」），根 CLAUDE.md:168 的「改名提醒」同样指向已不存在的旧结构。③ 上游同步顾虑不成立：git remote 仅 `xiaochunkun/XK-Pane.git`，与 `FxRayHughes/foundation.git` 无任何关联，不存在从脚手架上游 merge 的冲突风险。④ README.md 全仓库反查只有根 CLAUDE.md:100 一处文档链接引用，Taskfile、wails 构建、hooks、CLI 适配器、子进程白名单、i18n、路由 path、React.lazy、webapi 均无引用——纯文档，删节零运行时影响。⑤ 误导性实锤：根 CLAUDE.md:17/26/93-95、frontend/CLAUDE.md:25/376/379、README.md:127/303/310/399/410/424 仍写 `foundation` 模块路径/`git clone .../foundation.git`，而这些文档每次会话注入 AI 上下文，照抄会生成编译必败的 import。⑥ 不属于最近 worktree/composer 新功能（feat 分支 5 个 commit 均未触碰这些文档），是脚手架历史遗留。

**执行风险**：风险较低但有四个执行注意点：① README 除「克隆+改名」节外仍有大量准确且有用的内容（wails CLI 安装、命令速查、工程规范表、加页面/加表/加 service 路径），只能删改名节与脚手架推销段并修正示例路径，不能整本删；② 删除改名对照表后，根 CLAUDE.md:100「完整对照表见 README.md」的链接会成死链，须同步删改；③ 批量替换 foundation→xkpane 时严禁误伤活代码标识符：`theme.palette.foundation.*`、`foundation-light/dark/obsidian` 主题 id、`.claude/skills/foundation-*` 目录名与 skill 注册名（建议本身已注意到此点）；④ 建议中「随 greet/time 链路删除一并清掉」依赖另一条发现——若 greet 示例链路最终保留，相应文档示例应改写为 `@bindings/xkpane/...` 现实路径而非删除。另外 `.claude/skills/foundation-utils`、`foundation-persistence`、`foundation-childwindow` 等 SKILL.md 内也有 `import "foundation/...` 过时示例（foundation-utils/SKILL.md:48/128/177/228/273 等），属同类问题，可一并修正但注意 skill 目录名本身不改。

## #5 【脚手架残留·medium】frontend/CLAUDE.md 指向已不存在文件的过时小节：useSidebar.ts 占位类型与严重缩水的「相关文件清单」

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/CLAUDE.md、J:/Program/Agent/XK-Pane/frontend/DEVELOPMENT.md
- **估计规模**：文档约 50-60 行（清单树约 40 行 + 数据模型节数行 + DEVELOPMENT.md 示例若干行）

**证据**：frontend/CLAUDE.md「数据模型」节声称 `src/components/Sidebar/useSidebar.ts` 内有占位 SidebarItem 类型「仅作演示」——实测 Sidebar/ 目录只有 Sidebar.tsx / Sidebar.styles.ts / index.ts，该文件已被删除。文末「相关文件清单」树仍只列 services/greet 一个 service（实际 frontend/src/services/ 有 27 个域）、只列 HomePage/SettingsPage 两个页面（实际 pages/ 有 9 个），并把 HomePage 描述为「示例页：Greet 调用 + Time 事件」（实际已是仪表盘）。DEVELOPMENT.md:78/158/165/191 仍以 AppEvents.Time / useTimeEvent 为标准范例。这些是脚手架时代文档的死留存。

**建议**：删掉 frontend/CLAUDE.md 的「数据模型」小节（指向不存在的 useSidebar.ts）；「相关文件清单」要么删除该树（信息已严重失真，负价值），要么压缩为只列顶层目录不列具体文件以降低腐化速度；DEVELOPMENT.md 的事件示例随 time 链路删除换用现存事件。

**对抗核实裁决（成立）**：主体成立。①「数据模型」节（frontend/CLAUDE.md:364-366）指向的 src/components/Sidebar/useSidebar.ts 实测不存在（Sidebar/ 目录仅 Sidebar.tsx / Sidebar.styles.ts / index.ts），且 git grep useSidebar 在 frontend/src 零命中，删该小节无任何代码影响；②「相关文件清单」树（387-459 行）只列 services/greet 一个 service（实测 frontend/src/services/ 有 27 个域）、只列 2 个页面（实测 pages/ 有 9 个），并把 HomePage 描述为「示例页：Greet 调用 + Time 事件」而实测 useHomePage.ts 已是仪表盘 VM（问候/项目数/最近启动/CLI 检测，不调 Greet/Time），信息严重失真属负价值，删除或压缩为只列顶层目录合理；③两文档自 Initial commit（97751b6）后从未修改，与最近 5 个 worktree/composer 提交无关，不属待手测新功能。但 DEVELOPMENT.md 子项被部分反驳：其前提「time 链路已删」不实——internal/events/events.go:7 仍定义 Time="time"、internal/app/app.go:437 后端仍 Emit、useTimeEvent.ts 与 events.test.ts 均在，AppEvents.Time 示例引用的是仍存在的代码而非死指针，该子项须以另行拍板「删 time 链路」为前提才成立。

**执行风险**：风险很低但有两点：①DEVELOPMENT.md 的示例替换不是纯减法——若 time 链路保留，现有 AppEvents.Time / useTimeEvent 示例并不指向不存在的代码，可以不动；只有真的拍板删 time 演示链路时才需同步改写（届时 events.test.ts 的 Time 断言也要一并更新）。②frontend/CLAUDE.md 同文件的「对外接口」表（25-31 行，仍以 GreetService / time 事件为例）有同样的腐化问题，只删「数据模型」与「相关文件清单」两节会留下残余误导；压缩文件树时注意别波及紧邻的「变更记录」节。

## #6 【Go死代码·high】greet 示例服务整链死代码：后端注册 + 前端包装层均无真实调用方（脚手架遗留）

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/greet/greet.go、J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/frontend/src/services/greet/GreetService.ts、J:/Program/Agent/XK-Pane/frontend/src/services/index.ts
- **估计规模**：约 30 行源码 + 1 个 service 注册 + 1 个生成的 bindings 目录

**证据**：grep 'GreetService\.' frontend/src（排除 wrapper 自身与 index.ts 导出）零命中；HomePage 的「问候」是纯前端时段计算（useHomePage.ts 的 greetingForHour，import 列表里没有 GreetService）；@bindings/xkpane/internal/services/greet 只被 GreetService.ts 自己 import。后端 greet.Greet 在 Go 侧也无调用（app.go:190 注册是唯一引用）。webapi 无对应路由。

**建议**：删除 internal/services/greet/ 整包、app.go:190 的 application.NewService(greet.New())、frontend/src/services/greet/ 目录及 index.ts 中的导出，然后重新 wails3 generate bindings。

**对抗核实裁决（成立）**：对抗性反查全部失败，发现成立。证据：① frontend/src 全量 grep（不区分大小写）中 GreetService 仅命中 wrapper 自身（frontend/src/services/greet/GreetService.ts）与桶导出（frontend/src/services/index.ts:1），无任何页面/组件消费；② 实读 useHomePage.ts 确认 HomePage 问候是纯前端 greetingForHour 本地计算（第 17、46 行），import 列表无 GreetService，i18n 的 home.greeting.* 属于这套前端文案，与后端无关；③ internal/webapi 内 grep greet 零命中，Web 镜像端无对应路由；④ 全仓 *.go grep 显示 greet 包仅被 internal/app/app.go:24（import）与 app.go:190（RegisterService）引用，无其他 Go 调用方；⑤ 动态字符串面排除：frontend/src 无 Call.ByName 动态调用，hooks/CLI 适配器/子进程白名单等 Go 侧已被全量 grep 覆盖且无命中，Taskfile 零命中，其余命中全是文档与 skill 模板示例文本；⑥ greet 是 foundation 脚手架初始遗留，与最近 5 个 worktree/composer commit 无关，不属待手测新功能；⑦ 项目自身的 progress.md:928 已记录「GreetService / 后端 greet service 现无消费方（原仅 HomePage demo 用）」为遗留孤儿，仅因『不擅删既有』原则暂留——项目早已自证其死代码身份。删除路径（internal/services/greet/ 整包 + app.go:190 注册行及 import + frontend/src/services/greet/ + index.ts 导出行 + 重新 wails3 generate bindings）闭环可验证。

**执行风险**：运行时无风险（删除后重新生成 bindings 并跑 pnpm typecheck / go build 即可验证）。残余风险为文档失配：根 CLAUDE.md「通信链路」示意图、frontend/CLAUDE.md 对外接口表与 FAQ、frontend/DEVELOPMENT.md 示例、README 改名对照表第 8 条、foundation-i18n 与 foundation-persistence SKILL 均以 GreetService 为标准范例，删码后需同步更新这些文档（或换用其他 service 当范例），否则会误导后续开发者按已不存在的链路写代码；另注意只删 index.ts 第 1 行导出、保留该桶文件本身（LaunchHistoryService 等其他导出仍被 useHomePage 等消费）。

## #7 【Go死代码·high】session 服务整包死代码：已注册但前端无包装层、三个绑定方法与非绑定函数 LoadSessionsDecrypted 全仓零调用

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/session/session.go、J:/Program/Agent/XK-Pane/internal/services/session/session_test.go、J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/internal/storage/models_ccpane.go
- **估计规模**：约 242 行 Go 源码 + 1 个 service 注册（模型另计约 10 行）

**证据**：frontend/src/services/ 目录下没有 session 包装层（仅 sessionscan，是另一个服务）；grep SaveSessions/LoadSessions/ClearSessions 在 frontend/src 与 internal/（排除包自身和测试）均为 0；包级函数 LoadSessionsDecrypted 仅在 sshmachine/decrypt.go:15 与 webapi/gateway.go:13 的注释中被提及，无实际调用。包头注释自述「当前 SSH 运行时尚未落地、sshConfig 恒为 nil，安全边界先就位」——但 SSH 功能落地后会话恢复链路也未接。

**建议**：若近期不做「跨重启恢复终端布局」，删除 internal/services/session/ 整包与 app.go:201 的注册（session.go 103 行 + 测试 139 行）。注意 storage.TerminalSession 模型仍登记在 models_ccpane.go:21 的 AllModels（terminal_sessions 表 + 数据存储页统计 label），是否连模型一起删需单独拍板；若保留模型则表会继续空跑 AutoMigrate。

**对抗核实裁决（成立）**：反驳失败，五个引用面全部查实为零调用：① bindings 面——frontend/src 全目录 grep SaveSessions/LoadSessions/ClearSessions 与 services/session 路径均零命中，frontend/src/services/ 下无 session 包装层（仅 sessionscan，是 app.go:113 注册的另一服务），也无 ByName 等动态调用；生成的 frontend/bindings/xkpane/internal/services/session/ 存在但无人 import。② webapi 面——internal/webapi/gateway.go:13 是唯一提及处且为反向证据：注释明文「session.LoadSessionsDecrypted 永不挂任何路由」（明文密钥红线），P7_WEB_PLAN.md:163/193 同样把它列入永不暴露清单。③ 动态字符串面——表名 terminal_sessions 仅在 models_ccpane.go:21/172 登记与 models_ccpane_test.go:30 断言中出现，i18n labelKey "terminalSessions" 属模型登记（数据存储页统计经 AllModels 消费），不依赖 session 服务存活；hooks/CLI 适配器/子进程白名单/路由 path 与该服务无交集。④ LoadSessionsDecrypted——生产代码零调用，仅自身测试（session_test.go:69）与两处注释（sshmachine/decrypt.go:15、gateway.go:13）当安全先例引用；已落地的 SSH 页走 launchProject 即时连接，未接会话恢复链路。⑤ 时效豁免——session 包全部提交为 e379cd1/b0a3529（2026-05-30）/3f38e8f，不在最近 5 个 worktree/composer 提交（09bcb10..c454543，2026-06-10 收口）范围内，不属「刚落地待手测」新功能。唯一接近反驳的线索是 progress.md:761 把「会话恢复前端 SessionService.ts 接入」列为规划中的下一单元，但该单元从未实施，正落在发现「若近期不做跨重启恢复终端布局则删」的条件之内，不构成有效反驳。

**执行风险**：三点：① 这是「规划内功能的后端先行」——progress.md:761 明确把 SessionService.ts + TerminalProvider 接入列为待做单元，b0a3529 还专门做了 sshConfig 脱敏分层的安全设计；现在删掉，将来若真做「跨重启恢复终端布局」需重写约 240 行（含安全分层与测试），需用户对该功能取舍先拍板。② 删 app.go:201 注册后须重跑 wails3 generate bindings，否则 frontend/bindings 残留 session 生成目录（脏但无害）；storage.TerminalSession 模型若保留，terminal_sessions 空表会继续 AutoMigrate 并出现在数据存储页统计中（用户可见一张恒为 0 行的表）；若连模型一起删，须同步删 models_ccpane_test.go:30 的表名断言。③ sshmachine/decrypt.go:15 与 webapi/gateway.go:13 的注释以 session.LoadSessionsDecrypted 为安全先例，删除后这两处注释引用悬空（仅文档性，不影响编译）。

## #8 【Go死代码·high】emitTimeLoop 每秒空发 time 事件：前端订阅 hook useTimeEvent 无任何组件使用，纯脚手架演示遗留

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/internal/events/events.go、J:/Program/Agent/XK-Pane/frontend/src/shared/hooks/useTimeEvent.ts、J:/Program/Agent/XK-Pane/frontend/src/shared/events.ts
- **估计规模**：约 40 行（Go + TS）+ 1 条常驻 goroutine

**证据**：app.go:382 起常驻 goroutine emitTimeLoop（app.go:431-439，自述「演示后端主动推事件给前端的链路」）每秒 Emit 一次 time 事件；前端 useTimeEvent（useTimeEvent.ts，24 行）是唯一订阅方，但 grep useTimeEvent 在 frontend/src 只命中定义自身与 useTrayNavigation.ts 的一行注释，无组件调用；Web 端 WS 白名单（webapi/rest.go:28-34）显式不转发 time。事件全程发给空气。

**建议**：删除 emitTimeLoop 函数与 app.go:382 的 go 调用、events.go 的 Time 常量与注册、前端 useTimeEvent.ts 与 AppEvents.Time 条目（events.test.ts 同步更新）。省一条常驻 goroutine 和每秒一次的跨进程事件广播。

**对抗核实裁决（成立）**：反驳失败，建议成立。证据链完整：① 前端无人消费——useTimeEvent 全仓 grep 仅命中定义自身（frontend/src/shared/hooks/useTimeEvent.ts:11）与 useTrayNavigation.ts:6 的一行注释，无任何组件调用；原消费方 HomePage 在 c47c0a3「首页重做为仪表盘」时已移除该用法。② 无动态订阅面——前端各 service 均为逐事件显式订阅（grep AppEvents 命中 Terminal/MCPe/Orchestrator/Transcript/BedrockDocSync），无一订阅 AppEvents.Time；hooks/CLI 适配器/子进程白名单/i18n/路由 path 对字符串 "time" 均无引用。③ Web 端明确排除——internal/webapi/rest.go:29-33 WS 转发白名单不含 time，hub_test.go:84-90 甚至把 events.Time 当作「白名单外事件被丢弃」的测试反例（注释原文「白名单外（time）：丢弃，不进队列」）。④ 非新功能——git log -S emitTimeLoop 仅命中 Initial commit（97751b6），是脚手架演示代码，与最近 5 个 worktree/composer commit 无关。⑤ 项目自证——progress.md:928 已明确记录 emitTimeLoop/useTimeEvent「现无消费方（原仅 HomePage demo 用）……后续可作脚手架清理单独处理」，即项目自己早已认定为孤儿。事件确实全程发给空气：桌面端无订阅者、Web 端被白名单丢弃，常驻 goroutine 每秒一次 Emit 纯属空转。

**执行风险**：风险很低，但删除范围比原建议多三处连带：① internal/webapi/hub_test.go:85 用 events.Time 作白名单过滤测试的反例事件，删常量后需改用字面量 "time" 或其它非白名单名，否则后端测试编译失败；② frontend/src/shared/hooks/useTrayNavigation.ts:6 注释引用 useTimeEvent，需顺改一行；③ 文档陈旧——根 CLAUDE.md 架构图、frontend/CLAUDE.md「事件订阅」表格与文件清单、frontend/DEVELOPMENT.md:191 均以 useTimeEvent 为事件订阅范例，删后需同步更新（或接受文档与代码短暂不一致）。另一个轻微取舍：该链路是脚手架「后端主动推事件」的唯一教学示例，删除后新项目套用此脚手架时少一个最简参照——但仓库现有 terminal:output 等大量真实事件链路可替代示例职能，且 progress.md 已预定将其归入脚手架清理，故此风险可接受。

## #9 【Go死代码·high】provider.EnvVars 已被 ResolveLaunchEnv 完全取代，零调用

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/provider/envvars.go
- **估计规模**：约 9 行

**证据**：envvars.go:47-53 的包级函数 EnvVars 全仓（排除注释与测试）零引用；terminal 启动路径只用同文件的 ResolveLaunchEnv（app.go:275-277 注入闭包）。两者都走同一内部实现 loadDecrypted + envVarsFor，EnvVars 只是历史上的窄版本入口。

**建议**：删除 EnvVars 函数（保留 ResolveLaunchEnv 与内部 helper）。

**对抗核实裁决（成立）**：反驳失败，该减法建议成立。逐面反查证据：① bindings 面——EnvVars 是包级函数而非 provider.Service 的导出方法，envvars.go:18 注释明确「全为非绑定包级函数……绝不能进 Wails bindings」，wails3 不会为包级函数生成 bindings，前端不可能调到；frontend/src 中唯一命中是 ProviderSettings.tsx:45 的注释文字，非调用。② webapi 面——internal/webapi/gateway.go:13 明文密钥红线注明此类函数「永不挂任何路由」，webapi 下零引用。③ 动态字符串面——hooks/cliadapters/subprocess 白名单/i18n/路由 path 均为字符串引用域，与 Go 函数名无关；全仓 grep "EnvVars" 仅命中定义（envvars.go:47）、测试（envvars_test.go 8 个测试）、注释与计划文档（progress.md/P7_WEB_PLAN.md）。④ 生产唯一启动路径是 internal/app/app.go:276 的 SetProviderResolver 闭包，调的是 ResolveLaunchEnv；后者与 EnvVars 同走 loadDecrypted+envVarsFor，是严格超集（多返回 toCliProvider），含 p==nil 时返回空 map 的行为也一致。⑤ 非新功能豁免——EnvVars 落地于 2026-05-30 provider 接线单元（commit 9dde34b），ResolveLaunchEnv 是其后 0114099 重构的超集替代，与最近 5 个 worktree/composer commit 无关。结论：EnvVars 确为被超集取代的历史窄入口，生产代码零调用，可删。

**执行风险**：删除时必须同步改造 internal/services/provider/envvars_test.go——其中 8 个测试（TestEnvVarsEmptyId / TestEnvVarsUnknownId / TestEnvVarsAnthropicDecryptsKey / TestEnvVarsBedrockNoKey / TestEnvVarsOpenAIUsesCodexKey / TestEnvVarsOpencodeUsesOpenAIKey / TestEnvVarsGlm / TestEnvVarsOmitsEmptyBaseUrl）直接调用 EnvVars，直接删函数会编译失败。这些测试覆盖了 envVarsFor 的 ProviderType→env 映射与「加密落库→解密注入」往返，是有价值的覆盖，应迁移为调用 ResolveLaunchEnv（取其第一个返回值断言即可），不能连测试一起删。另有文档陈旧点：P7_WEB_PLAN.md:193 的红线清单仍点名 EnvVars，删除后该条指向不存在的函数（仅文档层面，无功能风险）。

## #10 【Go死代码·high】storage.MustOpen 无人使用（启动路径用的是可恢复的 Open）

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/storage/db.go
- **估计规模**：约 9 行

**证据**：db.go:106-112 定义 MustOpen，注释说「仅用于 main / 启动路径」，但 app.go:84 实际用的是 storage.Open（错误时 return fmt.Errorf，不 panic）；grep 'MustOpen' 全仓（排除注释）零调用。

**建议**：删除 MustOpen。

**对抗核实裁决（成立）**：反驳失败，该减法建议成立。全仓 grep「MustOpen」仅命中定义处 internal/storage/db.go:104-112 与文档 .claude/skills/foundation-persistence/SKILL.md:455 的目录注释，Go 代码零调用。逐项排除：①bindings——MustOpen 是包级 Go 函数而非 RegisterService 注册的 service 方法，不会进 wails3 generate bindings，frontend/src grep 零命中；②webapi——internal/webapi/gateway.go:188 注释明确走的是 storage.Open；③动态字符串面（hooks/CLI 适配器/子进程白名单/i18n/路由 path）只能引用事件名、命令名等字符串，Go 无按字符串调用包级函数的机制，不适用；④Taskfile/测试——启动路径 internal/app/app.go:84、路径切换 internal/services/storagesvc/storagesvc.go:187,230 及全部 10 个 *_test.go 统一使用可恢复的 storage.Open；⑤提交归属——git log -S 显示 MustOpen 来自 Initial commit（97751b6），属脚手架遗留的投机性便捷函数，与最近 worktree/composer 新功能无关。其注释宣称的「仅用于 main / 启动路径」场景实际由 app.go 的 storage.Open + return error 实现，需要 panic 语义的调用方用 Open 自行包一行即可。

**执行风险**：几乎无风险。仅需同步：删除后应顺手更新 .claude/skills/foundation-persistence/SKILL.md:455 的目录注释（db.go # Open / MustOpen / Close），否则文档与代码脱节。另注意本仓库是 foundation 脚手架模板，若有派生项目曾照文档调用 MustOpen 则各自独立，不影响本仓删除。

## #11 【Go死代码·high】bedrock.Warm 预热入口从未被调用（查询层首调即惰性建索引，对齐物残留）

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/bedrock/service.go
- **估计规模**：约 8 行

**证据**：service.go:16-23 的 Warm（注释「对齐 BedrockQueryService::warm」）；grep '\.Warm(' 在 internal/ 与 main.go 零命中——app.go:148 构造 bedrockSvc 后直接注入 orchestrator，从不预热；orchestrator 的 bedrock_* 工具也不调它。所有查询经 getIndices() 惰性构建，Warm 不是必要路径。bedrock 服务未注册进 Wails Services（app.go:147 注释明确「不进 Wails Services 列表」），前端不可达。

**建议**：删除 Warm 方法；若希望保留启动预热能力，改为在 app.go 启动 goroutine 里调一次（当前是两头都没有的半成品）。

**对抗核实裁决（成立）**：反驳失败，建议成立。逐面反查证据：① bindings 面——internal/app/app.go:215 的 Wails Services 列表只注册 bedrockDocSyncSvc，bedrockSvc（app.go:148）仅经 app.go:339 注入 orchestrator，注释明示「不进 Wails Services」，故 wails3 不会为其生成 bindings；frontend/src 中 grep "bedrock" 的 13 个命中全部是 bedrockdocsync（OTA 面板）/ provider / i18n 文案 / 事件名，不存在 bedrock 查询服务的前端封装，前端确实不可达。② webapi 面——internal/webapi 对 "bedrock" 零命中，Web 镜像端无此路由。③ orchestrator 面——orchestrator_bedrock_tools.go:24-54 定义的 Bedrock 接口共 26 个方法（SearchAPI…ValidateTest），不包含 Warm；pe_* 工具集也没有任何 warm 类工具，删除 Warm 不破坏 *bedrock.Service 对该接口的结构性满足。④ 动态字符串面——全仓库不区分大小写 grep "warm"，除 service.go:15-19 自身与 standards_test.go:90-91 外仅剩一条无关的 skill 文档命中（slider.warm）；hooks / CLI 适配器 / 子进程白名单 / i18n key / 路由 path / Taskfile / 脚本均无引用。⑤ 新功能豁免——该文件最近提交是 46ce5b7（pe_* 前缀重构）等 P6f 知识库系列，不属于最近 5 个 worktree/composer commit，不享受「待手测」豁免。生产路径上 Warm 确为零调用：所有查询经 getIndices() 惰性建索引，Warm 是对齐 cc-pane BedrockQueryService::warm 的对齐物残留，且 app.go 启动期也未调它，属于「两头都没接的半成品」，可删。

**执行风险**：两点：① 唯一现存调用在 internal/services/bedrock/standards_test.go:90-91（TestService_Delegation 用 svc.Warm() 预热索引），直接删 Warm 会导致 go test 编译失败——删除时须同步把该测试改为调用同包的 getIndices() 或任一查询方法（如 SearchAPI）触发惰性构建；② 删除后将失去现成的启动预热入口，首个 pe_* 查询会承担一次性索引构建延迟（当前本就如此，无行为回退），若未来想做启动预热需按建议在 app.go 启动 goroutine 里重建入口；另与 cc-pane 上游的逐方法对齐关系会少一个对应物，属可接受取舍。

## #12 【Go死代码·high】localhistory.Store 三个方法仅测试可达：CompressBlobs / ListDirectoryChanges / ListVersionsByBranch

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/localhistory/compress.go、J:/Program/Agent/XK-Pane/internal/localhistory/compress_test.go、J:/Program/Agent/XK-Pane/internal/localhistory/query.go、J:/Program/Agent/XK-Pane/internal/localhistory/store.go
- **估计规模**：约 130 行（含测试）

**证据**：三个方法（compress.go:20、query.go:50、store.go:196）在 internal/（排除测试与注释）零调用；上层门面 localhistorysvc 的全部 24 个导出方法里没有包装它们，前端 localhistory 服务层与 webapi 也不可达；唯一引用是各自的单元测试（compress_test.go / query_test.go / store_test.go）。Cleanup 路径走的是 Store 的其他方法。

**建议**：删除这三个方法及对应测试。compress.go 整文件约 70 行可整体移除（blob 压缩功能从未接入清理链路）；若认为压缩是计划内能力，至少应在 localhistorysvc.Cleanup 里接上，否则就是死重。

**对抗核实裁决（成立）**：反驳失败，三个方法确为「实现了但从未接线」的死重。全仓 grep 仅命中定义、各自单测与规划文档；门面层 localhistorysvc 包装了同批 P4b 计划的 PutLabel（localhistorysvc.go:288）/ GetFileBranches（:301），也包装了 query.go 的兄弟方法 GetRecentChanges / ListVersions / ListDeletedFiles，唯独跳过这三个；frontend/（含本地生成 bindings）零命中——它们挂在 localhistory.Store 而非注册 service 上，未经包装根本进不了 bindings；webapi 只有注释提及 localhistory:changed 事件丢弃，无 REST 路由；hooks/CLI/子进程白名单/i18n/路由 path 等动态字符串面不可能引用 Go 内部方法。且 P4_PLAN.md:76-77 自己已拍板「ListVersionsByBranch 可不暴露（前端本地过滤替代）、CompressBlobs 可选低优」。git log 确认三文件来自 Phase 4（ff04e53/51cb1e5），最近触碰是 42f3f82，均早于最近 5 个 worktree/composer 提交，不属「待手测新功能」保护范围。

**执行风险**：三点低风险：① P4_HANDOFF.md 把 ListVersionsByBranch/CompressBlobs 列为「未暴露 API」储备，若未来做 4-tab 完整本地历史 UI（目录视图可能要 ListDirectoryChanges），删后需重写，但每个约 30 行且 git 历史可找回，成本低；② 删 compress.go 须同步删 compress_test.go，并摘除 query_test.go:68-96、store_test.go:125-128 对应测试段，否则编译失败；③ GetVersionContent 的 gzip 魔数兼容读因 CompressBlobs 从未被生产路径调用，现存 blob 必为未压缩，删除不影响存量数据读取（兼容分支留着也无害）。另注意规划文档（P4_HANDOFF.md/P4_PLAN.md）提及这些 API，删后文档与代码脱节，建议顺带标注。

## #13 【Go死代码·medium】mcdkbridge 协议读半边与 Disconnect 仅测试使用（Go 侧桥是 fire-and-forget 单向发帧）

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/mcdkbridge/protocol.go、J:/Program/Agent/XK-Pane/internal/services/mcdkbridge/bridge.go
- **估计规模**：约 10-40 行（视 ReadFrom 是否一并降级）

**证据**：包级函数 ReadFrom（protocol.go:106，读取完整帧）生产代码零调用——bridge.go 的 readDrain 不解析入站帧；引用只在 bridge_test.go:75 与 protocol_test.go:115/129（测试里模拟对端收帧）。Service.Disconnect（bridge.go:123-132）唯一调用方是 bridge_test.go:123。

**建议**：Disconnect 可直接删除（测试改用 Stop 或直接关连接）；ReadFrom 若删需把帧解析逻辑降级为 _test.go 内的测试 helper——也可保留 ReadFrom 作为协议对称性（成本极小），优先删 Disconnect。

**对抗核实裁决（成立）**：反驳失败，该减法建议成立。逐面反查确证：① bindings 面不存在——internal/app/app.go:142 注释明确「mcdkBridge……非 Wails service」，仅 mcdkbridge.New() 后由装配处直管 Start/Stop（app.go:375-378），从未 RegisterService，故前端不可能经 @bindings 调到 Disconnect；frontend/src 中 grep mcdk/Disconnect 仅命中 MCPEService.GetMcdkBridgeStatus（属 mcpe service 自己的绑定，链路是 mcpe→bridge.Status()，与 Disconnect 无关）。② internal/webapi 对 Disconnect/mcdk 零命中。③ 唯二消费方接口面均不含这两个符号：orchestrator.Bridge 接口（orchestrator.go:77-80）只要求 SendPacket+Status；mcpe/service.go 经 SetBridge 只用 Port/Status/SendPacket。④ 全仓库 Disconnect 唯一调用是 bridge_test.go:123，ReadFrom 唯一调用是 protocol_test.go:115/129 与 bridge_test.go:75，生产代码零引用。⑤ 无动态字符串引用可能——二者是普通 Go 标识符，非事件名/路由 path/子进程白名单；且 ReadFrom 是包级函数、签名 (io.Reader)(Packet,error) 不满足 io.ReaderFrom，不存在 io.Copy 隐式接口分发。⑥ 不属新功能豁免——两文件最近提交为 49ea215/926e168（P6e MC PE 桥），早于 feat/paseo-worktree-composer 最近 5 个 worktree/composer 提交。⑦ 设计佐证：bridge.go:173 注释明言「Mod 实际不发任何数据，读到的内容直接丢弃」，桥结构性单向 fire-and-forget，入站帧解析在生产链路永远用不上。建议的优先级也合理：先删 Disconnect（纯死代码），ReadFrom 可降级为 _test.go helper 或以极小成本保留协议对称性。

**执行风险**：删 Disconnect 需同步改写 bridge_test.go 的 TestDisconnectClearsState（改用 Stop 或直接关连接），否则编译失败；若未来 UI 要加「主动断开桥连接」按钮需重写（约 9 行，成本极低）。删/下沉 ReadFrom 需把帧解析逻辑搬进 _test.go 保住「WriteTo 线上格式与游戏内 IPCSystem.py 拆包对称」的往返测试覆盖，并微调 protocol.go「忠实移植 protocol.rs」的对齐注释（cc-pane 原版含完整编解码，删后不再 1:1）。生产运行时无任何风险。

## #14 【Go死代码·medium】cliadapters.Registry.IDs 仅被自家测试调用

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/cliadapters/registry.go、J:/Program/Agent/XK-Pane/internal/services/cliadapters/adapter_test.go
- **估计规模**：约 7 行

**证据**：registry.go:30-34 的 IDs；grep '\.IDs(' 全仓唯一命中 adapter_test.go:43。生产代码（clitools / terminal / provider）用的都是 ListTools / DetectAll / Get。

**建议**：删除 IDs，adapter_test.go 改为从 ListTools() 取 id 断言注册顺序。

**对抗核实裁决（成立）**：逐面反查均未找到生产引用：① bindings 面——cliadapters.Registry 从未被注册为 Wails service（internal/app/app.go:206 注册的是 clitools.New(cliRegistry)，clitools.Service 只暴露 ListTools/DetectAll），故 IDs 不会被生成进 frontend/bindings，且 grep frontend/src 中 IDs 零命中；② webapi 面——internal/webapi 中无 IDs 引用，Web 镜像端不消费；③ 动态字符串面——IDs 是 Go 方法，全仓 .go/.ts/.tsx 中 \bIDs\b（含不带括号的 method value 形式）仅 4 处命中：registry.go:29-30 定义 + adapter_test.go:43/45 测试调用，无反射/字符串调用迹象；④ 新功能豁免不成立——git log 显示 registry.go 仅有 1ee9771（Phase 1 cli adapters）与 12a59a5 两次提交，与最近 5 个 worktree/composer commit 无关。建议的替代方案等价：ListTools()（registry.go:37-43）同样按 r.order 注册顺序返回含 ID 字段的 ToolInfo，测试改从 ListTools() 取 id 断言注册顺序完全可行。

**执行风险**：删除 IDs 时必须同步改写 adapter_test.go:43-45（改为从 ListTools() 取各项 ID 断言顺序），否则包编译失败；该方法虽是导出方法，但位于 internal/ 路径，外部仓库无法 import，无外部消费者破坏风险。除此之外无风险。

## #15 【Go死代码·medium】utils 层多个导出 API 全仓零调用：httpx.PostJSON / httpx.ReadJSONLimit / filex.EnsureDir / logx 包级 Debug·Info·Warn·Error

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/utils/httpx/httpx.go、J:/Program/Agent/XK-Pane/internal/utils/filex/filex.go、J:/Program/Agent/XK-Pane/internal/utils/logx/logx.go
- **估计规模**：约 40 行 + 文档同步

**证据**：词边界 grep（排除定义行、注释、测试）：httpx.PostJSON（httpx.go:138）、httpx.ReadJSONLimit（httpx.go:154）、filex.EnsureDir（filex.go:100）、logx 包级四个一行 wrapper（logx.go:150-153，业务代码全部走 logx.For("component")）均零调用。httpx.DefaultShouldRetry 有包内默认值引用（retry.go:49），不算死代码。

**建议**：这是脚手架刻意预留的通用 API 面（项目 CLAUDE.md 与 foundation-utils SKILL 把 httpx.PostJSON 列为标准入口），属「可删但需同步文档」档：若坚持最小 API 面，删 logx 四个包级 wrapper（强制带 component 名）和 filex.EnsureDir（os.MkdirAll 一行等价物）最无争议；httpx.PostJSON/ReadJSONLimit 删除需同步改 CLAUDE.md 与 SKILL 文档。

**对抗核实裁决（成立）**：对抗性反查失败，四项 API 确实全仓零调用，减法建议成立。证据：① httpx.PostJSON（httpx.go:138）与 httpx.ReadJSONLimit（httpx.go:154）全仓（cmd/、internal/、frontend/、Taskfile、脚本均覆盖）仅命中定义行与文档（CLAUDE.md:156、README.md:285、foundation-utils SKILL.md）；包内 doJSON（httpx.go:188）是独立实现不复用 ReadJSONLimit，业务侧 httpx 实际只用 SetUserAgent（internal/app/app.go:68）和 NewClient/ClientOptions（bedrockdocsync/manifest.go:4、proxy.go:53），cmd/xkpane-cli-hook/main.go:14 注释明确刻意不用 httpx。② filex.EnsureDir（filex.go:100）在 .go 文件中唯一命中是定义行；progress.md:76 声称 cliadapters 配置写入走「WriteAtomic+EnsureDir」，但实际代码 cliadapters/adapter.go:206 用的是 os.MkdirAll——属文档漂移，不构成反驳；全仓业务建目录一律直接 os.MkdirAll（projecthooks/claude.go:123、worktreesvc.go:124 等数十处）。③ logx 包级 Debug/Info/Warn/Error（logx.go:150-153）正则 logx\.(Debug|Info|Warn|Error)\( 全仓零命中，所有业务日志（含 webapi、worktreesvc 40+ 处）全走 logx.For("component")，与「强制带 component 名」的项目铁律方向一致。覆盖面排除：这些是 Go 内部工具函数而非 Wails service 方法，不进 bindings（frontend/src grep 零命中）；webapi 只用 logx.For；Go 函数无法经字符串/hooks/子进程白名单/i18n/路由 path 动态调用；无 dot-import；Taskfile 无引用；最近 worktree/composer 新功能（worktreesvc）依赖的是 filex.WriteAtomic 与 logx.For，与本条四项 API 无关，不属误报新功能。原发现对 httpx.DefaultShouldRetry 有包内引用（retry.go:49）不算死代码的甄别也准确。

**执行风险**：主要风险是文档契约失配而非代码破坏：① CLAUDE.md:156、README.md:285、foundation-utils SKILL.md（:26/:73/:430）把 httpx.PostJSON 列为「HTTP 铁律标准入口」，删除后若不同步改这三处文档，未来 AI 按文档调用会编译失败，或被迫绕回被禁止的 net/http 裸用；ReadJSONLimit 删除还需同步 httpx.go:33 注释（「业务方需要更大值时显式调 ReadJSONLimit」）。② 本仓库定位是 foundation 脚手架，通用 API 面属刻意预留，删 PostJSON 会让下游派生项目首次需要 POST JSON 时重写约 15 行——属设计取舍而非技术风险。③ logx 四个包级 wrapper 与 filex.EnsureDir 删除接近零风险（前者无调用且违背 component 命名铁律，后者是 os.MkdirAll 一行等价物，但 SKILL.md:293-296 也需同步删除对应小节）。

## #16 【Go死代码·medium】orchestrator:session-started 事件发射无任何订阅方（桌面无 Events.On、WS 白名单显式排除、前端常量表无此名）

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/orchestrator/rest.go、J:/Program/Agent/XK-Pane/internal/events/events.go、J:/Program/Agent/XK-Pane/internal/webapi/rest.go
- **估计规模**：约 20 行

**证据**：rest.go:78-84 在 session-started 回报端点里 emitEvent(events.OrchestratorSessionStarted, ...)；前端 grep 'orchestrator:session-started' 零命中且 frontend/src/shared/events.ts 的 AppEvents 不含此名；webapi/rest.go:27 注释点名该事件「一律不转发」（不在 mirroredEvents 白名单）。事件链路完整地发给空气，只有同函数里的 logx 日志真正起作用。注意：REST 端点本身是活的（兄弟项目注入响应、launchhistory 回填都依赖它），死的只是 emit 这一段。

**建议**：删除 rest.go:78-84 的 emitEvent 调用、events.go:115-117 的 OrchestratorSessionStartedPayload 类型、events.go:180 的 RegisterEvent 与 :31 的常量（events.go:28-30 注释自述 P6d-3「本轮仅 log + emit」——若确有 UI 计划再保留，否则属预留未接的死路）。

**对抗核实裁决（成立）**：七路反查全部反驳失败，发现成立。①前端：frontend/src 全文 grep 'session-started' 与 'OrchestratorSessionStarted' 零命中；frontend/src/shared/events.ts 的 AppEvents 常量表（前后端事件名唯一事实源，有 events.test.ts 防漂移）不含此名，且全前端所有订阅（Events.On / eventTransport.on）均经 AppEvents 常量或 subprocess 流式专属名传入，不存在能命中该名的动态订阅。②Web 镜像：internal/webapi/gateway.go:96-101 RouteEvent 对白名单外事件直接 return 丢弃，rest.go:26-34 的 mirroredEvents 不含它且 :27 注释点名「orchestrator:session-started 等本地或无关事件一律不转发」。③后端 Go：全仓无 app.Event.On 订阅此事件，标识符仅存在于 events.go（定义+RegisterEvent）、rest.go:78-84（emit）、rest_test.go（TestHandleSessionStartedEmitsEvent，测的是 emit 本身而非消费方）。④launchhistory 回填走独立链路（前端 StartBackfill → launchhistory.go:133-143 扫盘轮询），不依赖此事件。⑤hooks/CLI/子进程白名单/i18n/路由 path 等动态字符串面无此名（cmd/xkpane-cli-hook 只 POST REST 端点并消费响应里的 workspaceProjects，不订阅事件）。⑥仓库自证：progress.md:1191 明确把「前端订阅 orchestrator:session-started」列入「未做（留后续）」。⑦新功能豁免不适用：emit 落地于 commit 1344a60（2026-05-31，P6d-3），早于最近 5 个 worktree/composer commit。发现的边界划得也准：REST 端点 /api/terminal/session-started 本身是活的（cli-hook 上报 + 兄弟项目注入响应依赖它），死的只有 emit 链（rest.go:78-84 emitEvent、events.go:31 常量、:115-117 payload 类型、:180 RegisterEvent）。

**执行风险**：三点：①internal/services/orchestrator/rest_test.go 的 TestHandleSessionStartedEmitsEvent（:56-94）直接断言这次 emit，删 emit 必须同步删除/改写该测试，否则 go test 红。②这是文档化的预留扩展点（events.go:28-30 注释与 progress.md:1191「留后续」均自述），若日后做 resume-by-cwd 或启动历史 live 刷新等 UI 消费方，需重新加回常量+payload+RegisterEvent+emit——重加成本低（约 15 行），但属于有意预留而非遗忘，删前可向维护者确认一句是否仍有计划。③删 handleSessionStarted 内的 emit 后端点仍须保留（log + workspaceSiblings 响应是活链路），切勿连带删端点或 handler。

## #17 【Go依赖·high】golang.org/x/net 可从 direct 依赖降级：stdlib 已原生支持 socks5/socks5h 代理，proxyclient.go 的 socks5 分支可整段删除

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/utils/httpx/proxyclient.go、J:/Program/Agent/XK-Pane/go.mod
- **估计规模**：代码约 -20 行；go.mod direct 依赖 -1（降为 indirect）；二进制裁掉 x/net/proxy + internal/socks 两个包

**证据**：仓库内 golang.org/x/net 的唯一直接 import 是 proxyclient.go:12 的 x/net/proxy（go mod why golang.org/x/net/proxy 显示链路仅 xkpane/internal/utils/httpx 一条）。其用途是 L59-75 的 socks5/socks5h 分支：proxy.SOCKS5 造 dialer 再适配 ContextDialer。但本机 Go 1.25 stdlib 已原生覆盖：GOROOT/src/net/http/transport.go L119-121 注明 Proxy URL 支持 "socks5" 与 "socks5h"（且二者等同），L1803-1816 实证 socks5 鉴权直接从 proxy URL 的 userinfo 读取用户名密码，底层拨号也复用 Transport.DialContext（连接超时语义保留）。因此 socks5 分支可塌缩成与 L76-80 http 分支完全相同的写法：u.User = url.UserPassword(...) + tr.Proxy = http.ProxyURL(u)。注意 x/net 不会从 go.mod 消失——html-to-markdown 仍需 x/net/html——但会从 direct 区降到 indirect 区，x/net/proxy 与 x/net/internal/socks 两个包退出二进制。

**建议**：把 NewClient 中 case "socks5", "socks5h" 分支与 http/https 分支合并（统一注入 u.User 后走 http.ProxyURL），删掉 golang.org/x/net/proxy import 与 ContextDialer 适配代码，跑 go mod tidy 让 x/net 降为 indirect。行为等价（socks5h 域名解析同样由代理侧完成）。

**对抗核实裁决（成立）**：对抗反查全部未能推翻该建议。证据：① 引用面干净——仓库内 golang.org/x/net 的直接 import 仅 internal/utils/httpx/proxyclient.go:12 一处（go mod why 实证链路唯一）；proxyclient.go 是内部工具层非 Wails service，唯一调用方 internal/services/bedrockdocsync/proxy.go:53 的导出方法签名不变，bindings/webapi/事件/i18n 零影响；frontend/src 无任何 socks5 硬编码，代理 URL 来自用户配置（proxy.json / env），合并后的分支仍接受 socks5/socks5h scheme，用户契约不变。② stdlib 能力实证（本机 go1.25.5 GOROOT/src/net/http/transport.go）：L118-121 注明支持 "socks5" 与 "socks5h" 且二者等同；L1803-1816 实证 socks 鉴权直接读 proxyURL 的 userinfo；L1783 实证到代理的拨号走 Transport.DialContext，现有 ConnectTimeout 语义保留。③ 语义等价：x/net@v0.53.0 internal/socks/client.go L86-103 证实 proxy.SOCKS5 对主机名发 AddrTypeFQDN（代理侧解析），与 stdlib 行为一致，域名解析位置不变；HTTPS 的 TLS/HTTP2 在 socks setup 后照常协商。④ go.mod 声明 go 1.25.5，socks5h（Go 1.24 引入）有下限保证。⑤ 非新功能：git log --follow 显示该文件出自 b1b30e6（P6f-7 bedrock OTA 时期），不属最近 worktree/composer 五连提交。⑥ x/net 经 html-to-markdown 的 x/net/html 仍被需要，tidy 后确会降为 indirect，与建议描述一致。

**执行风险**：两个小风险：① internal/utils/httpx/proxyclient_test.go:46-58 的 TestNewClient_SOCKS5 把旧实现细节当断言（要求 tr.Proxy 为 nil、走 DialContext），合并分支后该测试必挂，需同步反转断言（改为校验 tr.Proxy 返回 socks5 URL 且 userinfo 注入）；② 行为差异极小但存在——旧实现中 proxy.SOCKS5 构造失败的错误（实践中几乎不触发）在 NewClient 时返回，改后此类错误推迟到首次请求时暴露；且现有测试只验证构造不验证真实代理链路，改完建议用设置页「测试代理」按钮对真 socks5 代理手测一次 Probe 通路。

## #18 【Go依赖·high】反直觉核实：go-git/v5 是最大瘦身候选却当下减不动——wails 运行时包直接 import 它；gitx 残余 5 个 go-git 调用建议维持现状，留作 wails 升级后的条件触发项

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/utils/gitx/gitx.go、J:/Program/Agent/XK-Pane/internal/utils/gitx/gitx_ignore.go、J:/Program/Agent/XK-Pane/go.mod
- **估计规模**：当前 0；条件满足（wails 移除 go-git）后约 -15 个间接模块、二进制减数 MB 量级，gitx 代码量持平

**证据**：xkpane 侧 go-git 只剩 gitx.go 一个文件在用，且仅 5 个只读 API：Open(L66)/IsRepo(L78)/CurrentBranch(L85)/Branches(L111-114)/Log(L135-141)；同包其余全部操作（worktree/stash/clone/fetch/pull/push/status/stage/commit/diff 等 20+ 处）早已走 runGit CLI，甚至 Branches 的 CLI 等价物就在同包 gitx_ignore.go:70（for-each-ref refs/heads）。看似删掉这 5 个调用就能让 go-git 及其 crypto 栈退出二进制（circl 在构建图占 34 个包）——但 go list -deps -f 实证 github.com/wailsapp/wails/v3/pkg/application 的 Imports 列表里直接含 github.com/go-git/go-git/v5，即 gitx.go 头注释「v5.19.1 已随 Wails 间接引入」在链接层面也成立：无论本仓库怎么改，go-git + ProtonMail/go-crypto + cloudflare/circl + ssh 栈都会链入二进制。当下替换 = 纯改写成本、零减量。

**建议**：现在不动。把「gitx 残余 5 个只读 API 改 CLI（rev-parse --show-toplevel / branch --show-current / for-each-ref / log --format）」记为 wails 升级检查项：一旦未来 wails 版本移除对 go-git 的 import，做这次小改写即可让 go-git 连同约 15 个专属间接模块（mergo/go-crypto/circl/gods/gcfg/go-billy/groupcache/ssh_config/sha1cd/knownhosts/ssh-agent/winio 等）整体退场。

**对抗核实裁决（成立）**：反驳成功——但反驳掉的是该发现「当下减不动」的结论，而非减法本身；实证表明这刀现在就能砍，且比原发现更有价值。核查链：① 发现的基础事实部分属实：全仓只有 internal/utils/gitx/gitx.go 一个文件 import go-git（Grep 'go-git/go-git' 仅命中该文件 L33-36），调用点恰为所列 5 个只读 API（Open L66 / IsRepo L78 / CurrentBranch L85 / Branches L111-114 / Log L135-141），同包 CLI 等价物（gitx_ignore.go:66 BranchNamesCLI、cli.go runGit）确已就位；wails v3.0.0-alpha.96 的 application 包在默认构建上下文里也确实直接 import go-git。② 但决定性漏洞在构建标签：wails 里 import go-git 的文件是 application_debug.go，头部是 //go:build !production——而本项目所有平台的生产构建全部带 -tags production（build/windows/Taskfile.yml:66、build/linux/Taskfile.yml:62、build/darwin/Taskfile.yml:51、ios/android 同理）。实测 go list -tags production -deps github.com/wailsapp/wails/v3/pkg/application 的完整依赖图中 go-git 为零；再对 xkpane 全量 production 构建图反查 importer，go-git/v5 的唯一 importer 就是 xkpane/internal/utils/gitx。③ 结论：原发现的 go list -deps 是在不带 production 标签的 dev 上下文跑的，由此得出的「无论怎么改 go-git 都链入二进制、当下替换零减量」对实际发布的生产二进制不成立。把 gitx.go 这 5 个只读调用改写为 CLI（rev-parse / branch --show-current / for-each-ref / log --format），生产二进制即可在当下（无需等 wails 升级）甩掉 go-git 及其专属间接栈（ProtonMail/go-crypto、cloudflare/circl、go-billy、gods、gcfg、sha1cd、knownhosts、ssh_config、xanzy/ssh-agent、mergo、groupcache 等）。减法成立，且不属于最近 worktree/composer 待手测功能（gitx 只读门面是 Phase 4 旧设施；worktreesvc 仅是其调用方，改写 gitx 内部实现不动其调用面）。

**执行风险**：三点：① 收益边界——dev 构建（wails3 dev 不带 production 标签）仍会经 wails 的 application_debug.go 链入 go-git，且 go.mod/go.sum 里 go-git 会从 direct 降为 indirect 而非消失（wails 仍 require 它），所以收益仅限生产二进制体积，需 task build 前后实测确认幅度（Go 链接器本有死代码消除，别按 34 包全量折算）；② 能力回退——gitx.IsRepo/Open 现在靠 go-git PlainOpen 判定、不需要系统装 git 二进制（gitx_test.go:177 明示此特性），改 CLI 后整个只读门面（gitsvc 约 20 处 Open 调用点，多为前端 Git 面板路径）都依赖用户机器有 git，且 Windows 上每次判定多一次进程 spawn，高频轮询场景需评估（不过 IsDirty 等已走 CLI，说明成本量级可接受）；③ 时机——worktreesvc 的 3 处 gitx.Open 属 feat/paseo-worktree-composer 待手测功能，建议改写安排在该分支手测合入之后，避免叠加变量。

## #19 【前端死代码·high】components/x-pro 整目录（图表 Pro/Premium 自实现）零引用，是最大块的前端死代码

- **涉及文件**：frontend/src/components/x-pro/index.ts、frontend/src/components/x-pro/charts/、frontend/src/components/x-pro/data-grid/
- **估计规模**：48 个文件 / 约 3534 行

**证据**：grep 'x-pro' 在 frontend/src 下零命中（仅 tsbuildinfo 构建产物与 .claude/skills/foundation-mui-x-charts、foundation-mui-x-data-grid 文档提及）；从 main.tsx/child.tsx 双入口做 import 图 BFS，x-pro 全部 48 个文件不可达。内含 HeatmapChart/FunnelChart/RadarChart/CandlestickChart/SankeyChart/GanttChart/TreemapChart/ZoomableChart 八个图表 + DataGridPro/DataGridPremium + 14 个 feature hook，共 3534 行。设置页数据存储图表实际用的是 @mui/x-charts 官方 PieChart/BarChart，不走 x-pro。

**建议**：整目录删除 frontend/src/components/x-pro/（foundation 脚手架预置的『自实现 Pro 功能库』，本项目从未消费）；同时归档或删除对应的 foundation-mui-x-charts 中 pro-premium-charts 引用文档与 foundation-mui-x-data-grid 的 pro/premium 引用文档，避免误导后续 AI 按文档找不存在的运行时依赖。

**对抗核实裁决（成立）**：对抗性反查全部失败，该减法建议成立。逐项证据：①规模核实：frontend/src/components/x-pro/ 实测 48 个文件、3534 行，与建议完全一致。②字符串引用面：全仓 grep 'x-pro|xpro|x_pro'（排除 x-pro 自身/node_modules/tsbuildinfo/dist）仅命中 .claude/skills/foundation-mui-x-charts 与 foundation-mui-x-data-grid 文档，以及 build/ios/Taskfile.yml 中 'project.pbxproj' 的子串假阳性（pbxproj 含 xpro），无任何源码引用。③别名绕过排除：frontend/vite.config.ts 仅有 '@'→src 与 '@bindings'→bindings 两个别名，任何对该目录的 import（绝对或相对路径）必含 'x-pro' 字面量，无法绕过 grep。④组件名反查：HeatmapChart/FunnelChart/RadarChart/CandlestickChart/SankeyChart/GanttChart/TreemapChart/ZoomableChart/DataGridPro/DataGridPremium 十个名字在 frontend/src 共 24 个命中，全部位于 x-pro 目录内部，外部零消费。⑤动态导入排除：routes.tsx 与 rightpanel/RightPanelDrawer.tsx 的全部 React.lazy 均指向 pages/ 下显式路径，src 内无 import.meta.glob，无任何指向 x-pro 的动态 import。⑥Web 镜像端排除：Web 端复用同一份 frontend/src（isDesktop 分支与 WebWorkspaceTree 等同在其中），上述 grep 已覆盖；internal/webapi 是 Go 代码不可能消费 TS 组件，全仓 grep 亦无命中。⑦bindings/事件/子进程白名单不适用：x-pro 是纯前端组件库而非后端 service，不存在经 '@bindings/foundation/...'、事件字符串或 subprocess 白名单被引用的通路。⑧实际消费方核实：设置页数据存储图表 frontend/src/pages/SettingsPage/database/Database.tsx 只 import '@mui/x-charts/PieChart' 与 '@mui/x-charts/BarChart' 官方组件，确实不走 x-pro。⑨新功能豁免排除：git log 显示 x-pro 由 commit b25bc86「feat(framework): 整合 foundation 62ee479 框架升级（子窗口/系统托盘/MUI X Pro）」引入，是 foundation 脚手架预置物，不属于最近 5 个 worktree/composer 新功能 commit。

**执行风险**：风险很低但有三个连带点：①frontend/package.json 中 '@mui/x-data-grid'（^9.3.0）在 x-pro 之外零消费（@mui/x-charts 仍被设置页使用须保留），删目录后该 npm 依赖成孤儿，应一并从 package.json 移除并跑 pnpm install，否则留下第二层死代码；②必须同步处理 .claude/skills/foundation-mui-x-charts/references/pro-premium-charts.md 与 .claude/skills/foundation-mui-x-data-grid/{SKILL.md,references/pro-features.md,references/premium-features.md} 中指向 '@/components/x-pro' 的内容，否则后续 AI 会按文档 import 不存在的路径导致构建失败；③该目录是 foundation 脚手架的预置能力库，若未来本项目想用 Pro 级图表/表格（如热力图、行分组），需从 foundation 上游找回——属于可接受的取舍，删除本身不影响 typecheck/构建（零 import 即零断链）。

## #20 【前端死代码·high】子窗口体系整链路休眠：前后端都没有任何代码会打开子窗口

- **涉及文件**：frontend/src/services/childwindow/ChildWindowService.ts、frontend/src/services/childwindow/bus.ts、frontend/src/services/childwindow/index.ts、frontend/src/childwindows/、frontend/src/child.tsx、frontend/child.html、internal/app/childwindow.go
- **估计规模**：前端约 17 个文件 / 约 604 行；连同后端约 726 行

**证据**：打开子窗口的唯一入口是后端 ChildWindowService.Open：Go 侧 grep 'childWinSvc' 仅 app.go:129 创建、:212 注册、:251 AttachApp，从未调用 Open；前端包装 services/childwindow/ChildWindowService.ts 的 ChildWindowService 与 ChildWindowOpenOptions 在全 src 零 import（连 index.ts 都没转出它）。bus.ts 导出的 sendToWindow/broadcast/onChildResult/onChildClosed 同样无任何消费者。childwindows/{Confirm,Message,Blank}Window 只能经 child.tsx 入口进入，而 child.html 只会被 childwindow.go:59 的 '/child.html?...' URL 加载——该路径永远不会触发。前端相关代码 535 行 + child.tsx 45 行 + child.html 24 行，后端 childwindow.go 122 行。

**建议**：两个方向二选一：若近期无子窗口需求，整链路删除（childwindows/、services/childwindow/、child.tsx、child.html、vite.config.ts 的 child 入口、internal/app/childwindow.go 及 app.go 三处接线），需要时再从 foundation 上游脚手架取回；若想保留脚手架能力，至少删掉前端从未转出的 ChildWindowService.ts 与 bus 中无人用的 sendToWindow/broadcast/onChildResult/onChildClosed 四个函数。注意 foundation-childwindow skill 文档描述的就是这套，删除时应一并归档。

**对抗核实裁决（成立）**：对抗反查七个方向均未找到任何活引用，反驳失败：① bindings 链路——前端唯一动态 import `@bindings/xkpane/internal/app/childwindowservice` 的是 frontend/src/services/childwindow/ChildWindowService.ts:15，而该文件在全 frontend/src 零 import，index.ts 也确实未转出它；② Go 侧——childWinSvc 仅 internal/app/app.go:129（创建）/212（注册）/251（AttachApp）三处接线，Open/Close/List 无任何 Go 调用方，加载 child.html 的唯一通路 childwindow.go:59 永不触达；③ 事件字符串——child:result:/child:send:/child:broadcast/child:closed: 全仓库只出现在子系统自身（bus.ts:5-8、childwindow.go:15-18,82）与 .claude/skills/foundation-childwindow/SKILL.md，无外部订阅者；④ webapi——internal/ 全量 grep 无引用，P7_WEB_PLAN.md 仅把 childwindow 列入「不暴露给 Web」清单；⑤ 动态字符串面——childwindow.* i18n key 只在 childwindows/*/lang/ 自带且仅经 child.tsx 注册，无路由 path、无 hooks/CLI/子进程白名单引用；⑥ React.lazy/Taskfile——唯一动态 import 即上述 bindings 一处，Taskfile 零匹配（child.html 仅 vite.config.ts:55 作为独立构建入口）；⑦ 新功能豁免——git log 显示这批文件最后改动在 b25bc86「整合 foundation 框架升级」，早于且独立于最近 5 个 worktree/composer 提交，不属待手测新功能。bus 的 sendToWindow/broadcast/onChildResult/onChildClosed 四函数同样无消费者；childwindows 组件虽内部消费 getChildWindowParams/emitResult 等，但只能经 child.tsx→child.html→后端 Open 这条死链进入。整链路确为休眠死代码，「整删」或「至少删孤儿包装层」的建议均站得住脚。

**执行风险**：三点：① 这是 b25bc86 有意从上游 foundation 脚手架整合进来的框架能力（与系统托盘/MUI X Pro 同批），整删会与上游 foundation 产生分叉，未来再同步框架升级时可能引入合并冲突或被重新带回；② 必须同步归档/删除 .claude/skills/foundation-childwindow/SKILL.md，否则 AI 按 skill 文档找已删代码会脱节（本仓库已有先例：foundation-systray skill 的弹出面板删除后文档曾脱节）；③ 删除时需一并处理 vite.config.ts:55 的 child 构建入口与 app.go 三处接线并重新 generate bindings，漏一处会导致构建或启动报错；若未来需要真 OS 级子窗口（MUI Dialog 拖不出主窗口），需从上游重新取回整链路。

## #21 【前端死代码·high】useTimeEvent 死 hook + 后端每秒空推 time 事件（双向脚手架演示残留）

- **涉及文件**：frontend/src/shared/hooks/useTimeEvent.ts、frontend/src/shared/events.ts、internal/app/app.go
- **估计规模**：前端约 26 行 + 后端约 12 行，并消除一个常驻每秒 goroutine 推送

**证据**：useTimeEvent 在全 src 零 import（grep 'useTimeEvent' 仅命中自身与 useTrayNavigation.ts:6 的一行注释）；AppEvents.Time 除 events.ts 定义与 events.test.ts 防漂移断言外无任何订阅者。而后端 app.go:382 `go emitTimeLoop(app)` 启动了一个每秒 ticker，app.go:437 持续 Emit(events.Time)——一个永远没有监听者的每秒事件推送。webapi 镜像端也未路由该事件（仅 hub_test.go 用它当测试样例）。

**建议**：删除 frontend/src/shared/hooks/useTimeEvent.ts、shared/events.ts 的 Time 条目及 events.test.ts 对应断言；后端同步删掉 emitTimeLoop 函数与 app.go:382 的 goroutine 启动行、internal/events/events.go 的 Time 注册（hub_test 改用其他事件名做样例）。这是 foundation 脚手架的『事件链路演示』，业务上线后属纯开销。

**对抗核实裁决（成立）**：反驳失败，建议成立。逐面反查：① 前端零消费——useTimeEvent 全仓仅命中自身定义、useTrayNavigation.ts:6 一行注释与文档；AppEvents.Time / 字符串 'time' 在 frontend/src 只存在于 events.ts:5 定义与 events.test.ts:7 断言；逐一核对全部 Events.On 调用点（terminal/mcpe/orchestrator/bedrockdocsync/transcript/subprocess/transport），无人订阅 Time，也不存在遍历 AppEvents 的批量订阅。② 后端自证——internal/app/app.go:431 注释原文写明「emitTimeLoop 演示后端主动推事件给前端的链路」，382 行起每秒 ticker、437 行持续 Emit，确为无监听者的空推。③ webapi 镜像端明确排除——gateway.go:92-99 RouteEvent 白名单注释点名「其余（time / localhistory:changed 等本地/无关事件）丢弃」，hub_test.go:85 恰把 events.Time 当『白名单外被丢弃』的反例测试，Web 端不依赖反而主动拦截。④ 维护者亲口确认——progress.md:928 记录「遗留孤儿（未删）：GreetService / useTimeEvent / emitTimeLoop 现无消费方（原仅 HomePage demo 用）」。⑤ 排除保护条款——事件订阅不经 bindings；与 hooks 注入/CLI 适配器/子进程白名单/i18n key/路由 path 等动态字符串面无关；该链路是 foundation 脚手架初始演示，早于 feat/paseo-worktree-composer 最近 5 个 commit，不属于待手测新功能。

**执行风险**：功能上无风险（确无任何消费方），但执行时有四个连带点需同步处理，否则留下悬空引用：① internal/webapi/hub_test.go:85 用 events.Time 作白名单反例，删常量前须换用其他非白名单事件名（如 events.LocalHistoryChanged 或字面量字符串），否则后端测试编译失败；② frontend/src/shared/events.test.ts 的 EXPECTED_BACKEND_EVENT_NAMES 须同步删 Time 行，否则防漂移断言变红；③ 文档残留——CLAUDE.md:31、frontend/CLAUDE.md（接口表与文件清单两处）、frontend/DEVELOPMENT.md:78/158/165/191 均把该链路当「后端推事件」教学示例，删码后示例指向不存在的文件，建议改引真实业务事件（terminal:output 等）；④ useTrayNavigation.ts:6 注释「与 useTimeEvent 同款」需微调措辞（不改也不影响编译）。另注意 emitTimeLoop 兼有「真实业务可参考此结构」的脚手架教学意图，但本仓库已业务化且有大量真实事件链路可参照，教学价值已被取代。

## #22 【前端死代码·high】GreetService 前端包装死代码（HomePage 早已改用本地问候逻辑）

- **涉及文件**：frontend/src/services/greet/GreetService.ts、frontend/src/services/index.ts、internal/services/greet/
- **估计规模**：前端 12 行 + barrel 1 行；连同后端 greet 包约 30-50 行

**证据**：GreetService 仅被 services/index.ts:1 barrel 转出，全 src 无任何实际消费者（grep 'GreetService' 只命中定义文件与 barrel）；HomePage 的问候语来自 useHomePage.ts:17 本地纯函数 greetingForHour(hour) + i18n key，不调后端。后端 internal/services/greet 仍在 app.go:190 注册并生成 bindings，但唯一的前端调用入口（这个包装）无人 import。

**建议**：删除 frontend/src/services/greet/GreetService.ts 与 services/index.ts 的对应导出行；后端 internal/services/greet 包与 app.go:190 注册行也可一并删除（脚手架 demo，模块结构图里它只是示例域）。

**对抗核实裁决（成立）**：反驳失败，该减法建议成立。逐项反查证据：① bindings 调用面——frontend/src 中 GreetService 仅命中定义文件 frontend/src/services/greet/GreetService.ts 与 barrel frontend/src/services/index.ts:1，全部 70+ 处 `from '@/services'` 导入无一引入 GreetService，也无任何文件直接 import `@bindings/xkpane/internal/services/greet`；② webapi——internal/webapi 对 greet 零命中，Web 镜像端无此路由；③ 事件——internal/events 无 greet 事件名；④ 动态字符串——前端无 Call.ByName / 'greet' 字符串调用，internal/ 全量 grep 仅命中 app.go:24（import）、app.go:190（注册行）与 greet.go 自身，projecthooks / cliadapters / subprocess 白名单均无；⑤ React.lazy——HomePage 问候来自本地纯函数 greetingForHour（useHomePage.ts:17/46）+ i18n key，确不调后端；⑥ Taskfile / build / Go 测试零命中；⑦ 决定性佐证：progress.md:928 项目自己已记录「遗留孤儿（记一笔，未删）：GreetService / useTimeEvent / 后端 greet service + emitTimeLoop 现无消费方（原仅 HomePage demo 用）……后续可作脚手架清理单独处理」，即维护者已确认其为无消费方孤儿。该代码属脚手架初始 demo，与最近 worktree/composer 新功能（最近 5 个 commit）无关，不在「待手测」豁免范围内。

**执行风险**：主要是文档同步风险：CLAUDE.md 通信链路示意图、frontend/CLAUDE.md:25/379/438/445、frontend/DEVELOPMENT.md（多处以 GreetService 作 MVVM 服务层标准范例）、README.md:310 改名对照表第 8 条均引用该文件，删码后这些文档会指向不存在的路径，需同步更新（或换用 PreferencesService 等真实服务作范例）。另注意三点：删后端包时须同时删 app.go:24 的 import 与 app.go:190 注册行并重跑 wails3 generate bindings；progress.md:928 显示维护者曾按「不擅删既有」原则有意保留，执行删除前宜确认（但该笔记本身也认可后续可单独清理）；若仍想保留脚手架教学价值（项目自述为 foundation 脚手架模板），可只删前端包装而留后端示例——不过 XK-Pane 已业务化（module 已改 xkpane），demo 价值很低。

## #23 【前端死代码·high】8 个 pages/*/index.ts 桶文件 + childwindows/index.ts 是零引用死 barrel

- **涉及文件**：frontend/src/pages/HomePage/index.ts、frontend/src/pages/SettingsPage/index.ts、frontend/src/pages/TerminalPage/index.ts、frontend/src/pages/FilesPage/index.ts、frontend/src/pages/SshPage/index.ts、frontend/src/pages/TodoPage/index.ts、frontend/src/pages/GitPage/index.ts、frontend/src/pages/HistoryPage/index.ts、frontend/src/childwindows/index.ts
- **估计规模**：9 个文件 / 约 30 行

**证据**：逐个 grep "from '@/pages/<Name>'"（不带子路径）全部零命中：routes.tsx 为了 chunk 拆分刻意从具体文件 import（如 '@/pages/SettingsPage/SettingsPage'，注释自述『不要从 index.ts 走』），App.tsx 的语言包也直接走 '@/pages/GitPage/lang' 子路径；rightpanel/RightPanelDrawer.tsx:13-14 的 lazy 同样直 import GitPage.tsx/HistoryPage.tsx。childwindows/index.ts 同理（child.tsx 逐个 import 各窗口目录）。只有 pages/McpePage/index.ts 真正被 routes.tsx 使用。routes.tsx 第 23 行注释『与 App.tsx 静态 import 同 index 合并』描述的旧状已不存在。

**建议**：删除这 9 个零引用 index.ts。它们不仅是死代码，还构成陷阱：谁要是顺手 import 了 '@/pages/SettingsPage'，就会把本应 lazy 拆分的页面拽回主 chunk（routes.tsx 注释警告的正是这个）。删掉即从源头消除误用可能。

**对抗核实裁决（成立）**：反驳失败，该减法建议成立。逐项反查结果：① alias 引用：grep `from '@/pages/<Name>'`（不带子路径）在 frontend/src 全量仅命中 McpePage 两处（routes.tsx:10、App.tsx:8），建议清单已正确排除 McpePage，其余 8 个页面桶零命中；② 相对路径引用：`pages/<Name>'` 与 `(../)+<Name>'` 形式仅命中各 index.ts 自身的 `export ... from './GitPage'` 等自导出行，无任何外部消费者；③ 尾随 `/index` 显式形式（`<Name>/index`、`childwindows/index`）零命中；④ React.lazy / 动态 import：routes.tsx 五个 lazy（24/31/38/45/52 行）全部直 import 具体文件（如 '@/pages/SettingsPage/SettingsPage'），rightpanel/RightPanelDrawer.tsx:13-14 的 GitPanel/HistoryPanel 同样直 import GitPage.tsx/HistoryPage.tsx，全仓无 import.meta.glob；⑤ App.tsx 语言注册（3-10 行）全部走 '@/pages/<Name>/lang' 子路径；⑥ childwindows：child.tsx:6-8 逐个直 import '@/childwindows/ConfirmWindow' 等子目录桶，顶层 frontend/src/childwindows/index.ts（仅 3 行 re-export）零引用，且无 './childwindows' 相对形式；⑦ 不存在 pages/index.ts 聚合桶；⑧ frontend/src 之外（Taskfile、vite.config、根目录）无代码引用，命中的只有文档示例（.claude/skills/foundation-i18n/SKILL.md:46-47、foundation-childwindow/SKILL.md:268、frontend/CLAUDE.md）；⑨ 这 9 个文件均不属于最近 5 个 worktree/composer commit 的新功能（Composer 在 components/Composer，有独立且被 App.tsx:16 使用的桶）。「陷阱」论点也获官方背书：routes.tsx:21-22 注释与 frontend/CLAUDE.md「lazy 必须 import 具体文件…不要走 index.ts，否则分包失效」自己就在警告经桶 import 的危害，而这些零引用桶恰是该误用的唯一入口。

**执行风险**：三点：① 文档约定冲突——frontend/CLAUDE.md 的 MVVM 铁律 #1 规定每个页面文件夹「至少含 index.ts（出口）」，其「添加新页面标准操作」示例及 .claude/skills/foundation-i18n、foundation-childwindow 两个 SKILL 的示例代码都按桶路径（'@/pages/AccountPage'、'@/childwindows'）书写；只删文件不同步改这些文档，后续贡献者或 AI 按文档操作会撞「模块不存在」错误，或顺手把桶重建回来。② 误删边界——pages/McpePage/index.ts 被 routes.tsx:10 与 App.tsx:8 实际使用必须保留；childwindows 各子目录自身的 index.ts（ConfirmWindow/MessageWindow/BlankWindow）被 child.tsx 使用，只能删顶层 childwindows/index.ts 一个文件。③ 验证手段——删除后跑 `cd frontend && pnpm typecheck && pnpm build` 即可确认无残留引用，成本极低。

## #24 【前端死代码·high】HistoryPage.skeleton.tsx 孤儿骨架：历史页迁入右侧抽屉后失去挂载点

- **涉及文件**：frontend/src/pages/HistoryPage/HistoryPage.skeleton.tsx
- **估计规模**：1 个文件 / 27 行

**证据**：HistoryPageSkeleton 唯一导出口是死 barrel pages/HistoryPage/index.ts:2（上一条已证零引用）；HistoryPage 现在只通过 rightpanel/RightPanelDrawer.tsx 的 lazy 挂载，其 Suspense fallback（RightPanelDrawer.tsx:61）用的是 t('common.loading') 文本而非骨架组件。HistoryPage 也不在 routes.tsx 路由表（无 fallback 消费点）。

**建议**：随死 barrel 一并删除 HistoryPage.skeleton.tsx（27 行）。

**对抗核实裁决（成立）**：反驳失败，孤儿确凿。全仓库 grep HistoryPageSkeleton 仅 2 处命中：定义处 frontend/src/pages/HistoryPage/HistoryPage.skeleton.tsx:6 与 barrel 导出 pages/HistoryPage/index.ts:2；barrel 本身（不带子路径的 @/pages/HistoryPage import，含动态 import 字符串形式）在 frontend/src 零引用——App.tsx:7 直接 import '@/pages/HistoryPage/lang'，RightPanelDrawer.tsx:14 直接 lazy import '@/pages/HistoryPage/HistoryPage'，均绕过 barrel。routes.tsx 无 history 路由、无 HistoryPageSkeleton import（fallback 仅 Home/Settings/Terminal/Files/Ssh 五个骨架）；RightPanelDrawer.tsx:61 的 Suspense fallback 实测为 t('common.loading') 文本而非骨架。骨架是纯 React 组件，不经 i18n key / 路由 path / 事件名 / bindings / hooks / 子进程白名单等字符串机制寻址，frontend/src 之外的命中全是 P4_PLAN.md / progress.md 等历史规划文档。git 历史确证孤儿化路径：800b954（P4c-1）引入骨架并在 routes.tsx 注册为 history 路由 fallback，cc600bd（历史页迁右侧抽屉）将其从 routes.tsx 移除后即失去唯一消费点；这两个 commit 均不在最近 5 个 worktree/composer 提交之列，不属待手测新功能豁免范围。

**执行风险**：基本无。两点轻微提示：① REBUILD_BLUEPRINT.md 的 Q-LH入口 拍板写过「独立页与 Dialog 两者并存」，若日后把 HistoryPage 重新提升为 routes.tsx 顶级路由，会想要骨架做 fallback——届时可从 git 历史（800b954）恢复，成本极低；② 项目规约 frontend/CLAUDE.md 要求页面文件夹含 index.ts 出口，若按建议把 barrel 整个删掉（而非只删 skeleton 导出行）会偏离该文件夹规约，但功能上无任何影响（现有引用全部绕过 barrel）。

## #25 【前端死代码·high】useTerminalPage.ts 是返回空对象的占位 hook，无人 import

- **涉及文件**：frontend/src/pages/TerminalPage/useTerminalPage.ts
- **估计规模**：1 个文件 / 8 行

**证据**：文件全文 8 行：导出空接口 TerminalPageModel {} 和返回 {} 的 useTerminalPage()；grep 'useTerminalPage' 在 src 其余文件零命中（TerminalPage.tsx 不用它）。文件头注释自述『保留此文件以符合页面三件齐全约定，便于将来扩展』——属于为约定而存在的空壳。

**建议**：删除该文件。MVVM 约定服务于真实状态，空壳占位反而让『页面三件齐全』失去信号意义；将来真有页面级状态时再建不迟。

**对抗核实裁决（成立）**：反驳失败，确实查无任何代码引用：①全仓 grep `useTerminalPage` / `TerminalPageModel`，frontend/src 内除自身文件零命中，其余命中全是历史文档（progress.md:948 自述「useTerminalPage 退化为空 stub」、REBUILD_BLUEPRINT.md、task_plan.md），非代码引用；②frontend/src/pages/TerminalPage/index.ts 只导出 TerminalPage 与 TerminalPageSkeleton，根本未导出该 hook，不存在经 barrel 的间接引用；③TerminalPage.tsx 只 import ./TerminalPage.styles；④routes.tsx:31 的 React.lazy 直接 import '@/pages/TerminalPage/TerminalPage'，不经 index 也不经 hook；⑤bindings/webapi/事件字符串/hooks 注入/CLI 适配器/子进程白名单/i18n key/路由 path/Taskfile 等动态引用面机制上均不可能指向前端 TS hook 文件，internal/ 全目录 grep 亦零命中；⑥git log 显示该文件最后改动于 4f7761c（WorkspaceTree 重构时退化为空壳），早于最近 5 个 worktree/composer 提交，不属于待手测新功能。该文件从未被打进 bundle，删除在编译、运行、Web 镜像端均无任何影响。

**执行风险**：非功能性风险一条：frontend/CLAUDE.md「路由规范（铁律）」第 3 条明文要求页面「三件齐全——use&lt;Name&gt;.ts（即使空也要有，便于扩展）」且声明违者 PR 被拒，该空壳正是为满足这条文档铁律而存在（文件头注释自述）。直接删除会让 TerminalPage 与项目自身文档约定不一致，后续按文档执行检查的评审或 AI 可能把文件重建回来。执行删除时应同步修订该条文档（改为「有页面级状态才建 hook」）或为 TerminalPage 记豁免，否则建议视为约定变更而非纯死代码清理。

## #26 【前端死代码·medium】SubprocessService 前端包装（107 行）在前端无任何消费者

- **涉及文件**：frontend/src/services/subprocess/SubprocessService.ts、frontend/src/services/index.ts
- **估计规模**：1 个文件 / 107 行 + barrel 约 8 行

**证据**：grep 'SubprocessService' 在 src 中除定义文件与 services/index.ts barrel 外零命中；其全部伴随类型 RunSpec/RunResult/AvailableCommand/SubprocessItem/ExitInfo 也仅 barrel 转出无消费；亦无绕过包装直 import '@bindings/xkpane/internal/services/subprocess' 的代码。后端白名单 service 本身另说（属后端维度），但『前端经白名单调子进程』这条链路目前没有任何 UI 在用。

**建议**：可删除该前端包装及 barrel 中对应导出块（后端 service 与 bindings 仍在，将来要用时按现成 bindings 重新包一层即可，成本极低）。注意 CLAUDE.md 把 subprocess 白名单当基础设施铁律描述，删前端包装不动后端红线；若团队倾向保留作为『铁律的标准接入示例』，也应在文件头标注当前无消费者。

**对抗核实裁决（成立）**：反驳失败，发现成立。逐项反查均无运行时消费者：① frontend/src 全量 grep SubprocessService/RunSpec/AvailableCommand/SubprocessItem，仅命中定义文件 frontend/src/services/subprocess/SubprocessService.ts 与 barrel frontend/src/services/index.ts:97-103，无任何页面/ViewModel 消费；② 无绕过包装直接 import '@bindings/xkpane/internal/services/subprocess' 的代码（唯一 import 在包装文件自身）；③ 事件字符串 subprocess:stdout/stderr/exit 前缀仅在该文件内部使用，无外部订阅；④ internal/webapi grep subprocess 零命中，Web 镜像端无 subprocess 路由，不存在 viewer 经 fetch 的隐藏链路；⑤ 无 React.lazy/路由 path/i18n/Taskfile 等动态字符串引用（全仓 grep 仅命中文档）；⑥ git log 显示该文件只被 Initial commit 和 Phase 0 rebrand 触碰，与最近 5 个 worktree/composer commit 无关，不属「待手测新功能」。删除前端包装与 barrel 对应导出块不影响编译运行，后端白名单 service（internal/app/app.go 仍 RegisterService）与 bindings 不动，符合 CLAUDE.md 后端红线。

**执行风险**：文档失配风险：.claude/skills/foundation-utils/SKILL.md（329-346、443、447 行）以该前端包装为「前端调白名单子进程」铁律的标准示例代码并列入文件清单，README.md:435 附近也有配套叙述，删除后这些文档会指向不存在的文件，须同步修订 SKILL 示例（或改为「按 bindings 现写包装」的说明）；另外 progress.md:152 记录它曾被当作事件驱动 service 的写法模板参考——它带有脚手架「标准接入示例」属性，删除后未来 AI 按 SKILL 接入前端子进程时会找不到现成入口（重建成本低，bindings 现成，但首次会踩文档与代码不一致）。若团队更看重示例完整性，可改为保留并在文件头注明当前无消费者。

## #27 【前端死代码·medium】零散零引用导出：isLinux / AppEventName / UseCliToolsResult 及 services/index.ts 中约 30 个仅 barrel 转出的类型

- **涉及文件**：frontend/src/shared/platform.ts、frontend/src/shared/events.ts、frontend/src/shared/hooks/useCliTools.ts、frontend/src/services/index.ts
- **估计规模**：约 40-50 行（分散在 4 个文件）

**证据**：导出级扫描（排除定义文件与各 index.ts barrel 后全词匹配计数）：platform.ts 的 isLinux 与 Platform 类型 0 引用（isWindows/isMac 有真实消费）；events.ts 的 AppEventName 类型 0 引用；useCliTools.ts 的 UseCliToolsResult 0 引用。另有一批类型只被 services/index.ts 转出、无任何实际 import，如 TodoService 的 TodoStatus/TodoPriority/TodoScope/CreateTodoInput/UpdateTodoInput/TodoQuery/TodoQueryResult、TaskBindingService 的 4 个类型、TranscriptService 的 Timeline/TimelineKind/TimelineOption、NativeDialogs 的 4 个 Options 类型等。

**建议**：低优先级清理：删 isLinux、AppEventName、UseCliToolsResult 三个确定死导出；barrel 中『仅转出无消费』的类型可顺手裁剪 services/index.ts，但这类类型导出常作为 service 公共 API 面保留，建议只在下次触碰对应 service 时顺带收紧，不值得专项动刀。

**对抗核实裁决（成立）**：对抗反查全部失败，发现成立。逐项证据：① isLinux（frontend/src/shared/platform.ts:17）全仓零代码引用，仅 frontend/CLAUDE.md:431 与 frontend/DEVELOPMENT.md:174 两处文档提及；同文件 isWindows/isMac 有真实消费者，对照之下 isLinux 确为死导出。② AppEventName（frontend/src/shared/events.ts:28）全仓零引用，shared/events.test.ts 只 import AppEvents 常量不碰该类型。③ UseCliToolsResult（useCliTools.ts:28）零外部 import，仅作同文件 useCliTools() 的返回类型标注。④ barrel 抽查：TodoStatus/TodoPriority/TodoScope/CreateTodoInput/UpdateTodoInput/TodoQuery/TodoQueryResult、TaskBinding 4 个类型、Timeline/TimelineKind/TimelineOption、NativeDialogs 的 OpenFileOptions/SaveFileOptions/ConfirmOptions/FileFilter 全部只在各自 service 定义文件内作为方法签名使用，src 内无任何外部 import（既不经 @/services barrel 也不经直接路径）；方法论可靠性用 TimelineEvent 反向验证（确能查到 SessionTimeline 两个真实消费者）。⑤ 这些全是 TS 编译期符号，不可能经 bindings、webapi 路由、事件字符串、hooks 注入、子进程白名单、i18n key、路由 path、React.lazy、Taskfile 等任何动态字符串面引用，反查通道天然封死。⑥ 不属于最近 worktree/composer 新功能——新功能相关的 GitWorktree/WorktreeCreateResult 不在删除清单内，未误伤。建议本身的克制定调（barrel 类型只顺带收紧、不专项动刀）也与事实相符。

**执行风险**：三点低风险注意：① UseCliToolsResult 不能整体删除——它是 useCliTools() 的返回类型标注且该 hook 被 HomePage/SshPage/WorkspaceTree 大量消费，正确做法是去掉 export 关键字或内联返回类型，而非删 interface；② 删 isLinux 需同步更新 frontend/CLAUDE.md 与 frontend/DEVELOPMENT.md 中「isWindows / isMac / isLinux」的描述，且 Linux 是构建目标（build/linux 存在），未来若加 Linux 专属 UI 分支需重新加回（一行代码，成本可忽略）；③ barrel 中的类型转出是 service 公共 API 面，裁掉后未来调用方若想显式声明（如 const q: TodoQuery）需改从 service 文件直接 import——这正是原建议「只在下次触碰对应 service 时顺带收紧」的合理性所在，不宜激进批量删。

## #28 【前端依赖·high】src/components/x-pro/ 整棵树（48 文件 / 约 3534 行）是死代码，连带 @mui/x-data-grid 依赖可整体删除

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/components/x-pro/、J:/Program/Agent/XK-Pane/frontend/package.json、J:/Program/Agent/XK-Pane/frontend/vite.config.ts、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-data-grid/、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-charts/
- **估计规模**：48 个文件 / 约 3534 行 TS/TSX；1 个 npm 依赖（node_modules 约 8.8MB）；vite 配置 1 行；外加 2 个 skill 文档目录

**证据**：在 frontend/src 全量 grep 字符串 "x-pro" 的文件内容零命中——即没有任何 import 路径引用该目录（树内部互相 import 用的是相对路径）；DataGridPro / DataGridPremium / useTreeData / useRowGrouping 等导出名，以及 HeatmapChart / FunnelChart / GanttChart / SankeyChart / TreemapChart / CandlestickChart / RadarChart / ZoomableChart 八个自实现图表组件名，只出现在 x-pro 目录内部（18 个文件全在 x-pro 下）；两个 HTML 入口（index.html→src/main.tsx、child.html→src/child.tsx）的依赖图都够不着它；src/components 下不存在 barrel index.ts 再导出。@mui/x-data-grid 的全部 16 处 import 都位于 x-pro/data-grid/ 内（depcheck JSON 输出同样显示其引用文件全在 x-pro），即依赖随树一起失活。该树来自 b25bc86「feat(framework): 整合 foundation 框架升级（子窗口/系统托盘/MUI X Pro）」的脚手架整合提交，不属于最近 5 个 worktree/composer 新功能提交。x-pro/charts 自实现图表只依赖 @mui/material，不依赖 @mui/x-charts，删树不影响设置页在用的图表。

**建议**：删除 frontend/src/components/x-pro/ 整个目录；从 frontend/package.json dependencies 移除 "@mui/x-data-grid": "^9.3.0"；从 frontend/vite.config.ts 的 optimizeDeps.include 删去 "@mui/x-data-grid" 一行；同步删除或改写 .claude/skills/foundation-mui-x-data-grid/ 整个 skill 与 foundation-mui-x-charts skill 里描述「项目自实现高级图表（Heatmap/Funnel/Radar/...）」的部分，避免文档指向已删代码（类似此前 foundation-systray 弹出面板删除后的文档脱节教训）。若想给未来留脚手架能力，也可只在 XK-Pane 业务仓库删、在上游 foundation 仓库保留。

**对抗核实裁决（成立）**：对抗反查全面失败，该减法建议成立。逐项核验：① 引用面：frontend 全树（含 src 外的配置）grep 字符串 "x-pro" 零命中（除 pnpm-lock/package.json/vite.config.ts 的依赖声明），不存在任何 import 路径指向该目录；② 导出名反查：index.ts 导出的全部符号（DataGridPro / DataGridPremium / FunnelChart / RadarChart / CandlestickChart / SankeyChart / GanttChart / TreemapChart / ZoomableChart / HeatmapChart / useRowGrouping / useAggregation / useExcelExport / useCellSelection / useTreeData）在 frontend/src 的 30 个命中文件全部位于 x-pro 树内部，树外零消费；③ 动态引用面：frontend/src 全部 React.lazy / import() / import.meta.glob 已逐条列举（routes.tsx 5 个 lazy 页、rightpanel/RightPanelDrawer.tsx 2 个 panel、ChildWindowService 的 bindings 动态 import），无一指向 x-pro；两个 HTML 入口（main.tsx、child.tsx）的静态 import 链也够不着它，src/components/ 下无 barrel index.ts 再导出；④ webapi 与 Web 镜像端：viewer 走同一份前端 bundle，依赖图不含 x-pro 即不会进 dist；internal/** grep 仅在 internal/services/bedrock/resources/data 的 Nukkit 文档第 692 行命中巧合子串 "x-pro"，属 MC 文档数据非代码引用；⑤ Taskfile / build / 事件字符串 / i18n / 路由 path / hooks / 子进程白名单：全仓库（Taskfile.yml、build/**、internal/**、.claude/**、*.md）反查导出名仅命中 .claude/skills/foundation-mui-x-data-grid 与 foundation-mui-x-charts 两个 skill 文档（建议已涵盖同步处理）；⑥ @mui/x-data-grid 的全部 18 处文件级引用中，src 内 16 处全在 x-pro/data-grid/ 下，其余仅 package.json / vite.config.ts optimizeDeps / pnpm-lock，依赖随树整体失活；而设置页在用的 @mui/x-charts 仅被 pages/SettingsPage/database/Database.tsx 引用，与 x-pro 无关，删树不影响；⑦ 历史归属：git log 确认 x-pro 由 b25bc86「feat(framework): 整合 foundation 框架升级」引入，不属于最近 5 个 worktree/composer 提交（c454543/b43236f/fa1.../e76.../09b...），不是待手测新功能；⑧ 额外佐证：x-pro/data-grid/features/useExcelExport.ts 通过 Function('m','return import(m)') 动态加载 xlsx 模块，但 package.json 根本没有 xlsx 依赖——即该能力即使被调用也会运行时失败，进一步证明这棵树是脚手架陈列品而非在用代码。

**执行风险**：功能性风险：无（零引用、不进 bundle、不涉及 bindings/事件/动态字符串）。三个执行注意点：① package.json 移除 @mui/x-data-grid 时必须同步删 vite.config.ts optimizeDeps.include 里的同名条目，否则 Vite dev 预构建会因找不到模块而启动报错；② 两个 skill 文档（foundation-mui-x-data-grid 整个、foundation-mui-x-charts 的「项目自实现高级图表」部分）需同步删除或改写，避免重蹈 foundation-systray 弹出面板删除后文档脱节的教训；③ 流程性风险：该树来自上游 foundation 脚手架整合（b25bc86 模式是整批合并），若未来再次从上游整合升级，可能把 x-pro 重新带回或产生合并差异——若想保留脚手架能力可只在 XK-Pane 删、上游 foundation 保留（建议原文已提及此选项）。

## #29 【前端依赖·high】@mui/system 直接依赖版本错位（^7.3.11 vs @mui/material 内部要求 ^9.0.1），导致 pnpm 安装两份副本

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/package.json、J:/Program/Agent/XK-Pane/frontend/src/pages/TodoPage/components/TodoListItem.tsx、J:/Program/Agent/XK-Pane/frontend/src/pages/TerminalPage/TerminalPage.styles.ts
- **估计规模**：node_modules 去掉 1 份约 1.4MB 的重复副本；消除生产 bundle 中 system v7/v9 双份运行时代码

**证据**：package.json 第 24 行直接依赖 "@mui/system": "^7.3.11"，而已安装的 @mui/material 9.0.1 的 package.json dependencies 要求 "@mui/system": "^9.0.1"——node_modules/.pnpm 下实测同时存在 @mui+system@7.3.11（1.4MB）与 @mui+system@9.0.1（878KB）两份副本。src 内共 14 个文件 import '@mui/system'，全部解析到顶层 v7 副本：其中 13 处是 `import type { SystemStyleObject }`（type-only，编译期擦除，不进 bundle），唯一的运行时引用是 TodoPage/components/TodoListItem.tsx 第 3 行 `import { keyframes } from '@mui/system'`——这会把 v7 的运行时代码拉进 bundle，与 MUI material 内部用的 v9 形成双份。

**建议**：把 package.json 的 "@mui/system" 对齐为 "^9.0.1"（与 @mui/material 同主版本）后重装，pnpm 即可只保留一份副本；或更彻底：TodoListItem.tsx 的 keyframes 改从 @emotion/react 引入（@mui/system 的 keyframes 本就是 @emotion 的再导出，且 @emotion/react 已是直接依赖），SystemStyleObject 保持 type-only import 即可。两种改法都不影响运行行为。

**对抗核实裁决（成立）**：对抗核查后无法反驳，发现属实。证据链：① frontend/package.json:24 实测为 "@mui/system": "^7.3.11"，而 node_modules/@mui/material（9.0.1）的 dependencies 实测要求 "@mui/system": "^9.0.1"；② node_modules/.pnpm 下实测同时存在 @mui+system@7.3.11 与 @mui+system@9.0.1 两份副本，顶层 node_modules/@mui/system 解析为 7.3.11，即 src 内全部 14 处 import 走 v7；③ 14 处中 13 处是 `import type { SystemStyleObject }`（编译期擦除），唯一运行时引用是 frontend/src/pages/TodoPage/components/TodoListItem.tsx:3 的 `import { keyframes } from '@mui/system'`（第 20 行 spin 动画），确实把 v7 运行时代码拉进 bundle 与 material 内部的 v9 形成双份。反驳尝试全部失败：@mui/x-charts 与 @mui/x-data-grid 的 peerDependencies 均为 "@mui/system": "^7.3.0 || ^9.0.0"，对齐 ^9.0.1 完全兼容，无任何包强制 v7；v9 副本实测仍导出 keyframes（@mui/styled-engine@9 的 index.js 实测就是 `_react.keyframes`，即 @emotion/react 再导出，而 @emotion/react ^11.14.0 已是 package.json:19 的直接依赖）和 SystemStyleObject（styleFunctionSx.d.ts 实测存在 `export type SystemStyleObject<Theme extends object = {}>` 定义行），13 处 type-only import 升级后不会失效；`git log -S '"@mui/system"' -- frontend/package.json` 显示该依赖自 Initial commit (97751b6) 引入，不属于最近 5 个 worktree/composer 新功能 commit；该发现是纯 npm 依赖问题，bindings 调用、webapi 路由、事件字符串、hooks/CLI/子进程白名单/i18n/路由 path 等动态字符串引用面、React.lazy、Taskfile 均不构成保留 v7 的隐藏引用渠道。两种建议改法（版本对齐为 ^9.0.1，或 keyframes 改从 @emotion/react 引入）均经核实可行。

**执行风险**：风险很低但非零：① v7→v9 的 SystemStyleObject 类型签名可能有细微差异，改完需跑 `pnpm typecheck` 与 `pnpm build` 验证 13 处 type-only 用法（实测 v9 签名为 `SystemStyleObject<Theme extends object = {}>`，与现有无泛型/简单用法兼容）；② 若只改 TodoListItem.tsx 的 keyframes 来源而不对齐 package.json 版本，v7 副本仍会被安装（只是不再进 bundle），减法不彻底——建议两者都做；③ pnpm-lock.yaml 会随之变动，属依赖变更，应独立提交，勿与 feat/paseo-worktree-composer 分支上待手测的功能改动混在一起。

## #30 【前端依赖·low】@mui/x-charts（约 12.4MB）整包依赖只服务设置页一个子页面的两张图

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/pages/SettingsPage/database/Database.tsx、J:/Program/Agent/XK-Pane/frontend/package.json、J:/Program/Agent/XK-Pane/frontend/vite.config.ts
- **估计规模**：1 个 npm 依赖（node_modules 约 12.4MB）+ Database 懒加载 chunk 体积；代码侧约持平（图表换简单展示）

**证据**：全 src 范围内 @mui/x-charts 仅有 2 处 import，都在 SettingsPage/database/Database.tsx 第 8-9 行（'@mui/x-charts/PieChart' 与 '@mui/x-charts/BarChart'，用于数据库表占比饼图 + 字节横向柱图）；depcheck 确认引用文件唯一。x-pro 自实现图表不依赖它（自绘 SVG），删 x-pro 不会增加其引用。node_modules 实测占 11MB（@mui+x-charts@9.3.0）+ 1.4MB（@mui+x-charts-vendor）。已用子路径 import，bundle 端有 tree-shaking，主要成本在依赖体积与安装/预打包时间。

**建议**：若「数据存储」子页不打算扩展更多图表，可把两张图降级为更简单的展示（如 MUI LinearProgress 比例条 + 表行字节列表，配色仍取 theme.palette.foundation.*），随后从 dependencies 移除 @mui/x-charts，并删去 vite.config.ts optimizeDeps.include 中的 "@mui/x-charts" 一行。注意这是产品取舍：双图是 changelog 里有意做的「数据可视化」特性，删除前需维护者拍板；故置信度标低。

**对抗核实裁决（成立）**：对抗反查全部未能推翻该发现：(1) grep "x-charts" 覆盖全 frontend/src，仅 SettingsPage/database/Database.tsx 第 8-9 行两处子路径 import（PieChart 用于 255 行、BarChart 用于 292 行），Web 镜像端与桌面端共用同一份 frontend/src，已被同一 grep 覆盖；(2) Go / Taskfile / internal/webapi 全仓 grep 零命中（仅 pnpm-lock.yaml 与 .claude/skills 文档）；(3) 怀疑「自实现高级图表可能依赖 x-charts 原语」——grep PieChart|BarChart|ChartsTooltip|useDrawingArea 等全 src 仅 Database.tsx 命中，skill 所述自实现图表只存在于 .claude/skills/foundation-mui-x-charts/references/ 文档中，非运行时代码；(4) npm 包不可能经事件字符串 / hooks 注入 / 子进程白名单 / i18n key / 路由 path 等动态字符串引用；(5) React.lazy 核实：SettingsPage 走 lazy（routes.tsx:23），x-charts 已在 lazy chunk 且子路径 import 可 tree-shake，印证发现「主要成本在依赖体积与安装/预打包时间」的判断准确；(6) vite.config.ts:37 确有 optimizeDeps.include 条目需同步删除；(7) 体积属实：.pnpm 实测 11MB + 1.4MB；(8) git log --follow 显示 Database.tsx 自 Initial commit 即存在，图表特性属 2026-05-28「数据可视化」changelog，不属于最近 5 个 worktree/composer 待手测 commit。结论：单消费点、无隐藏引用、降级 UI 后移除依赖不会破坏编译或运行时其他部分，作为「可降级为更简单实现」的减法发现成立。

**执行风险**：非死代码而是产品取舍：双图是 changelog（根 CLAUDE.md 与 frontend/CLAUDE.md「数据可视化 + 表清理」条目）有意建设的特性，删除即移除一个正常工作的可视化功能，必须维护者拍板。执行时的连带项：(1) 需同步删 vite.config.ts:37 optimizeDeps.include 的 "@mui/x-charts" 条目并跑 pnpm install 更新 lockfile；(2) .claude/skills/foundation-mui-x-charts/（含约 10 个 references 文档）及 foundation-mui/SKILL.md 索引会与代码失配，需同步处理否则 AI 会按失效文档写码；(3) 后端 storagesvc.GetTableStats 的 dbstat 精确字节统计链路是为喂图表而建，降级为文字列表后仍可用但价值下降（不会断裂）；(4) 替代 UI 须遵守项目铁律（theme.palette.foundation.* 取色、i18n t() 文案、MUI 组件）。

## #31 【i18n死键·high】公用 locale 9 个死键：terminal.tab.{new,menu} 与 common.{remove,reset,yes,no,error,followSystem,auto} 可删

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/i18n/locales/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/i18n/locales/en-US.ts
- **估计规模**：18 行（9 键 × 2 个 locale 文件）

**证据**：对全部 48 份语言包做了扁平化解析（共 24 组 lang 目录），并把 frontend/src 全量 .ts/.tsx（排除 lang 文件本身）作为引用语料逐键比对，同时把 19 个动态拼 key 前缀（如 settings.provider.types.${...}、home.greeting.${...}）和 .label/.description 动态后缀全部排除以防误报。terminal.tab.new、terminal.tab.menu、common.remove、common.reset、common.yes、common.no、common.error、common.followSystem、common.auto 这 9 个键在全仓（含 Go 后端 / webapi / hooks）零引用——唯一命中是 .claude/skills/foundation-i18n/SKILL.md 里的文档示例文字。terminal.tab.new 死于 commit 47be74e（提交信息明确写「TabBar 移除『新建』+ 按钮（新建会话走工作区树右键）」）；common.* 的 7 个是 foundation 脚手架时代预置的「通用术语」，同组的 common.save/cancel/loading 仍在用（如 SshPage.tsx:251 的 t('common.loading')），证明不是整组失效而是这 7 个从未被消费。另：zh-CN 与 en-US 的键集全量比对完全对称，无单边键，不存在不对称问题。

**建议**：在 zh-CN.ts 与 en-US.ts 中同步删除这 9 个键（zh-CN.ts 第 47、49、65-68、70-72 行及 en-US.ts 对应行）。注意 zh-CN.ts 文件头注释把 common.* 描述为「保存/取消/删除/是/否/加载中/错误/跟随系统」，删键后顺手把注释里已删条目去掉即可。

**对抗核实裁决（成立）**：对抗反查全部通道后无法反驳，9 个死键属实。证据：① 全仓字面量 grep（含 Go 后端 internal/webapi、hooks、Taskfile、.claude）对 terminal.tab.{new,menu} 与 common.{remove,reset,yes,no,error,followSystem,auto} 的唯一命中是 .claude/skills/foundation-i18n/SKILL.md 第 155/254 行的文档示例，无任何代码消费；② 动态拼 key 反查零命中（`common.${`、`'common.' +`、`terminal.tab.${`、裸 'common'、join('.') 等模式均无），唯一相近场景——设置页语言「自动」用的是 settings.language.auto.label（frontend/src/pages/SettingsPage/language/useLanguage.ts:28）而非 common.auto；③ 同组对照证明扫描可信：terminal.tab. 前缀在 frontend/src 恰有 6 处消费（全在 components/TabBar/TabBar.tsx，覆盖 close/rename/renameTitle/renamePlaceholder/pin/unpin），唯独不含 new/menu；common.{save,cancel,loading} 在 TabBar/WorkspaceTree/HistoryPage/SshPage 等多处存活，是这 9 键单独死亡而非整组误判；④ Web 镜像端与桌面共用同一份 frontend/src（isDesktop 分支），grep 已覆盖，i18n key 不经 bindings/事件名/子进程白名单等通道；⑤ git log -S "terminal.tab.new" 证实该键消费由 fa312f0 引入、47be74e 移除（commit body 明确写「TabBar 移除『新建』+ 按钮（新建会话走工作区树右键）」），建议方的历史考证属实；⑥ 这 9 键与最近 5 个 worktree/composer commit 无关（terminal.tab.new 死于更早的 47be74e，common.* 7 键是 foundation 脚手架预置且从未被消费）；⑦ zh-CN.ts 第 47、49、65-68、70-72 行行号核实无误，zh-CN/en-US 键集对称。

**执行风险**：风险很低但非零：① .claude/skills/foundation-i18n/SKILL.md 第 155 行把「是/否/跟随系统/自动」列为 common.* 预置术语、第 254 行示例直接写 t('common.error')，删键后文档与语言包失配——未来按 SKILL 文档写 t('common.error') 会缺键回退（显示 key 本身），建议删键时同步更新该 SKILL 与 zh-CN.ts 第 8 行的文件头注释；② 这些是「通用术语储备」性质的键，删除后新页面再需要「是/否/重置」等文案时须重新添加（属正常按需成本，不算破坏）。代码层面无风险：当前零消费，删除不影响任何运行路径。

## #32 【i18n死键·high】SessionTimeline 残留死键：sessionTimeline.{showTerminal,showChat} 在视图切换重构时漏删

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/components/SessionTimeline/lang/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/components/SessionTimeline/lang/en-US.ts
- **估计规模**：4 行（2 键 × 2 个 locale 文件）

**证据**：git show 47be74e 可见 TerminalView.tsx 删掉了这两个键仅有的两处消费（原 viewer 右上角浮动切换按钮：`-  <Tooltip title={showTimeline ? t('sessionTimeline.showTerminal') : t('sessionTimeline.showChat')}>` 及其 aria-label），卡片/终端切换改用新键 terminal.view.{group,card,terminal}（现由 TabBar.tsx:165-190 消费），但 lang 里的旧键没有一并清理。当前全仓 grep sessionTimeline.showTerminal|showChat 零引用。该 commit 早于最近 5 个 worktree/composer 提交，不属于待手测新功能。

**建议**：删除 SessionTimeline/lang/zh-CN.ts 第 12-13 行（showTerminal: '切到终端' / showChat: '切到对话'）及 en-US.ts 对应两行。

**对抗核实裁决（成立）**：对抗性反查全部未能推翻该发现：1) 全仓 grep `showTerminal|showChat` 仅命中 4 行，全是定义本身（frontend/src/components/SessionTimeline/lang/zh-CN.ts:12-13 与 en-US.ts:12-13），无任何消费点；2) 动态字符串拼接反查（`sessionTimeline.show` 前缀、模板字符串 `sessionTimeline.${...}`、字符串相加形态）在 frontend/src 零命中，不存在经动态 key 引用的可能；i18n key 属前端 t() 体系，与 bindings 调用、webapi 路由、事件名、hooks 注入、CLI 适配器、子进程白名单、路由 path、Taskfile 均无交集，且 Web 镜像端复用同一份 frontend/src 代码，上述 grep 已一并覆盖；3) git 历史确证：`git log -S showTerminal` 显示该键由 ae2d4a7（P7 Web 镜像）引入，47be74e（2026-06-02「顶栏 + 换成卡片/终端视图切换」）的 diff 中可见 TerminalView.tsx 删除了唯一两处消费（浮动切换按钮的 `Tooltip title={showTimeline ? t('sessionTimeline.showTerminal') : t('sessionTimeline.showChat')}` 及同形 aria-label），替代键 terminal.view.{group,card,terminal} 现由 TabBar.tsx:165-190 实际消费；4) 47be74e 早于最近 5 个 worktree/composer 提交（09bcb10 起），不属于 feat/paseo-worktree-composer 待手测新功能。结论：这两个键是视图切换重构时漏删的死键，确实可删。

**执行风险**：基本无。唯一注意点：项目 i18n 铁律要求各 locale keys 完全对齐，删除时必须 zh-CN.ts 与 en-US.ts 两侧同步删（建议本身已包含两侧），只删一侧会造成 keys 不对齐。

## #33 【i18n死键·high】HistoryPage 3 个死键：history.timeline.{title,current} 与 history.tag.placeholder

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/pages/HistoryPage/lang/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/pages/HistoryPage/lang/en-US.ts
- **估计规模**：6 行（3 键 × 2 个 locale 文件）

**证据**：同命名空间的兄弟键均在 HistoryPage.tsx 正常消费：history.timeline.{selectVersion,empty,rightClickHint}（295/308/431/490 行）、history.tag.{add,title,label,ok,added.title,added.message,failed}（479/634-646 行），唯独这 3 个键全仓零引用。打标签的输入对话框（HistoryPage.tsx:634-636）只传了 title/label/okLabel 三个文案参数，没有 placeholder 槽位，history.tag.placeholder（zh-CN.ts:103『如：发布前 / 重要里程碑』）无处可挂。动态引用面已排查：history.projectRestore.source.${...} 与 history.diff.${...} 两个动态前缀已识别并排除，这 3 个键不在其覆盖范围内。

**建议**：删除 HistoryPage/lang/zh-CN.ts 第 24 行（timeline.title: '版本'）、第 27 行（timeline.current: '工作树当前'）、第 103 行（tag.placeholder）及 en-US.ts 对应行。

**对抗核实裁决（成立）**：反驳失败，三个键确为死键。证据：(1) 全仓 grep `history.timeline.title|history.timeline.current|history.tag.placeholder` 零匹配，覆盖 frontend/src、internal/webapi 等 Go 侧与 Taskfile；(2) frontend/src 中 history.* 前缀的动态 t() 模板仅两处——HistoryPage.tsx:80 的 `history.projectRestore.source.${source}` 与 HistoryPage.tsx:298 的 `history.diff.${vm.diffCaption}`，均不触及 timeline.*/tag.* 子空间，useHistoryPage.ts 也无任何 'history.' key 字符串（排除 VM 暴露 key 给 View 的路径）；(3) 打标签对话框组件 frontend/src/components/TextInputDialog/ 四个文件 grep placeholder 零匹配——该组件根本没有 placeholder prop，HistoryPage.tsx:632-637 只传 title/label/okLabel/cancelLabel，history.tag.placeholder（zh-CN.ts:103）无处可挂；(4) zh-CN.ts:24（timeline.title）/:27（timeline.current）与 en-US.ts 同行号键完全对应，可同步删除以维持 locale 对齐铁律；(5) zh-CN.ts:3 注释自述该文件「恢复自旧 HistoryPage（d44625c^）」，这 3 个键是旧页面恢复时带回的残留，HistoryPage 属早已落地的本地历史功能，与最近 5 个 worktree/composer 新提交无关。

**执行风险**：基本无风险。仅两点提示：删除时须 zh-CN.ts 与 en-US.ts 同步删（否则违反「已有 locale keys 完全对齐」铁律）；若日后时间线表头或「工作树当前」版本标记、打标签输入提示等 UI 要补回这些文案，需重新添加键——属正常迭代成本，非破坏性风险。

## #34 【i18n死键·high】TodoPage 2 个死键：todo.item.overdue 从未用过、todo.editor.scopeRefPlaceholder 因改下拉而废弃

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/pages/TodoPage/lang/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/pages/TodoPage/lang/en-US.ts
- **估计规模**：4 行（2 键 × 2 个 locale 文件）

**证据**：逾期文案实际走的是 todo.overview.overdue（TodoOverview.tsx:73），todo.item.overdue（zh-CN.ts:47）全仓零引用，git log -S 也找不到任何曾引用它的提交（定义即死键）。scopeRef 字段已改为结构化下拉挑选——TodoEditor.tsx:35 注释明写「scopeRef 下拉数据源（结构化挑选，替代手填）」，编辑器只消费 todo.editor.{scopeRef,scopeRefRequired,scopeRefNone}（224/232/235 行），手填时代的 scopeRefPlaceholder（zh-CN.ts:63『工作空间名 / 项目路径』）已无处可用。动态前缀 todo.type/todo.filter.group/todo.scope/todo.status/todo.priority 均已识别排除，这 2 个键不在动态引用面内。

**建议**：删除 TodoPage/lang/zh-CN.ts 第 47 行（item.overdue）、第 63 行（editor.scopeRefPlaceholder）及 en-US.ts 对应行。

**对抗核实裁决（成立）**：反驳失败，两个键确为死键。①todo.item.overdue：全仓 grep `overdue`（含 Go 后端）显示 i18n 消费只有 TodoOverview.tsx:73 的 t('todo.overview.overdue')（对应 lang 第 89 行的活键）；todo.item.* 命名空间实际只消费 statusTip/myDayTip/subtasks（TodoListItem.tsx:83/91/109/159）；列表项的逾期表达是纯样式染色（isOverdue ? styles.metaOverdue : styles.metaText，给到期日期染红，不输出「逾期」文字）；动态拼接面排查全前端模板字符串 key 仅 todo.scope/status/priority/type/filter.group 五个前缀，无 todo.item.${...}；git log -G 确认该键自 c902c3a 引入后从未有消费提交，定义即死键。②todo.editor.scopeRefPlaceholder：全仓唯二出现处即 zh-CN.ts:63 / en-US.ts:63 本身；git show 0194b0b 给出直接证据——该提交把 scopeRef 从自由文本改成下拉，diff 明确删除了 placeholder={t('todo.editor.scopeRefPlaceholder')} 消费行却遗留 lang 键；当前 TodoEditor.tsx 只消费 scopeRef/scopeRefRequired/scopeRefNone（224/232/235 行）。③其余反查面全部排除：i18n key 属前端体系，webapi（Go）不消费 lang 文件；Web 镜像端与桌面端共用同一份 frontend/src，grep 已全量覆盖；事件字符串/hooks/CLI 适配器/子进程白名单/路由 path/Taskfile 与 i18n key 无交集；React.lazy 不隐藏文本级 key 引用。TodoPage 属早期落地功能（c902c3a），不在最近 5 个 worktree/composer 提交范围，不适用新功能豁免。

**执行风险**：基本无风险。唯二注意点：①必须 zh-CN.ts 与 en-US.ts 同步删除对应键，保持「已有 locale keys 完全对齐」铁律（仓库无 i18n 完整性校验脚本，运行时缺键有回退，不会崩）；②若未来 scopeRef 想恢复手填模式需重写该 placeholder 文案，可从 git 历史（0194b0b 之前）找回。

## #35 【i18n死键·high】SshPage 死键：ssh.launchMenu.bareTerminal 引入当天就没接线，菜单文案实际来自后端 displayName

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/pages/SshPage/lang/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/pages/SshPage/lang/en-US.ts
- **估计规模**：2 行（1 键 × 2 个 locale 文件）

**证据**：git show 7e4a62f（引入「SSH 连接中心页」的提交）中 bareTerminal 只出现在两份 lang 的新增行（+bareTerminal: 'Bare terminal' / +bareTerminal: '裸终端'），同一提交内没有任何消费方，之后也没有提交引用过它。当前启动菜单的菜单项主文案直接用后端数据 tool.displayName（SshPage.tsx:263），「裸终端」入口的文案由后端工具列表提供；该命名空间只有 ssh.launchMenu.notInstalled 被消费（SshPage.tsx:264）。ssh.auth.${...} 动态前缀已识别排除，launchMenu 不在动态引用面内。

**建议**：删除 SshPage/lang/zh-CN.ts 第 32 行（bareTerminal: '裸终端'）及 en-US.ts 对应行。SSH 功能虽在待手测清单上，但此键从引入起就零消费，与待测路径无关。

**对抗核实裁决（成立）**：对抗反查全部失败，发现成立。证据：①全仓 grep `bareTerminal` 仅命中 frontend/src/pages/SshPage/lang/zh-CN.ts:32 与 en-US.ts:32 两处定义，零消费；②动态引用面排除——全仓搜 `launchMenu`，除 lang 定义外只有 SshPage.tsx 里同名 useState 菜单锚点变量（与 i18n 无关）和唯一字面量消费 t('ssh.launchMenu.notInstalled')（SshPage.tsx:264），不存在模板字符串拼 key；③菜单项主文案确为后端数据 tool.displayName（SshPage.tsx:263，来自 useCliTools → CliToolService.detectAll()），「裸终端」入口实际走独立的 ssh.actions.connect 按钮，与 launchMenu 命名空间无关；④Go 后端及 frontend/bindings 生成产物中均无 bareTerminal / 裸终端 / bare terminal 字符串；⑤git log -S bareTerminal 全分支仅命中引入提交 7e4a62f，引入当天即无消费方且之后无人引用；⑥该键属 SSH 页（7e4a62f），不在最近 5 个 worktree/composer 提交范围内，且自引入起零消费、不在任何待手测路径上，不享新功能豁免。

**执行风险**：基本无风险。仅两点注意：删除时必须 zh-CN.ts 与 en-US.ts 两份同步删，保持项目「各 locale keys 完全对齐」铁律；该键可能是「把裸终端并入启动菜单」的设计意图残留，若日后真做该 UI 需重新加键（成本一行）。

## #36 【i18n死键·high】Provider 设置子页死键：settings.provider.title 被框架级 settings.list.provider.label 取代

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/pages/SettingsPage/provider/lang/zh-CN.ts、J:/Program/Agent/XK-Pane/frontend/src/pages/SettingsPage/provider/lang/en-US.ts
- **估计规模**：2 行（1 键 × 2 个 locale 文件）

**证据**：ProviderSettings.tsx 渲染从 settings.provider.hint 开始（87 行），子页标题由 SettingsPage 框架按 settings.list.<id>.label 统一渲染（动态后缀 .label，已在扫描中识别），settings.provider.title（zh-CN.ts:6『接入 Provider』）全仓零引用。同命名空间下其余 40 个键（含 types.${...} 与 form.${field} 两个动态组共 16 键）均确认有消费方，仅 title 一键死亡。

**建议**：删除 provider/lang/zh-CN.ts 第 6 行（title: '接入 Provider'）及 en-US.ts 对应行。

**对抗核实裁决（成立）**：反驳失败，该键确为死键。证据：(1) `settings.provider.title` 字面量在 frontend/src 与 internal/ 全仓零引用；(2) ProviderSettings.tsx 仅有两个动态拼接组——`settings.provider.types.${p.providerType}`（112/195 行，PROVIDER_TYPES 共 11 个枚举值，无 'title'）与 `settings.provider.form.${field}`（219 行，ProviderField 联合类型 6 个字段且嵌在 form. 命名空间下）——均不可能命中顶层 title 键；(3) 子页标题确由框架渲染：SettingsPage.tsx 55-57 行用 t(vm.activeItem.labelKey) 输出眉头与 h1，labelKey='settings.list.provider.label'（useSettingsPage.ts:39），provider 子页 View 自身从 settings.provider.hint（87 行）开始，从不渲染 title——这与 language/database/knowledge 等自渲染 settings.<sub>.title 的子页不同，正是死键成因；(4) useProviderSettings.ts 遵循 MVVM 铁律不持任何 i18n key；(5) internal/ 后端无 settings.provider 字符串下发（labelKey 动态通道不涉及），Web 镜像端复用同一份 frontend/src 代码，grep 已覆盖；(6) 该文件属 ff99674/5a9f03f（provider 管理功能提交），不在最近 5 个 worktree/composer 提交之列，不享「待手测新功能」豁免。i18n 为注册式深合并（localeRegistry.extend），缺键时回退显示 key 本身，删除无人消费的键无任何运行时副作用。

**执行风险**：基本无。仅两点注意：① 必须 zh-CN.ts:6（title: '接入 Provider'）与 en-US.ts:6（title: 'Providers'）同步删除，否则违反项目「已有 locale 必须 keys 完全对齐」铁律；② 若未来想让 provider 子页改为像其他子页一样自渲染 section 标题（settings.<sub>.title 模式），需重新补回该键——但当前框架统一渲染标题的设计下不需要。

## #37 【重复冗余·high】webapi 的 handleSessionTimeline 与 transcriptsvc.GetTimeline 整段逻辑重复，可合并为单一实现

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/webapi/rest.go、J:/Program/Agent/XK-Pane/internal/services/transcriptsvc/transcriptsvc.go
- **估计规模**：约 60 行重复逻辑（rest.go 218-295 收缩到 ~20 行），并消除一处双维护点

**证据**：rest.go:218-295 的 handleSessionTimeline 与 transcriptsvc.go:59-116 的 GetTimeline 是同一条流水线的两份手写拷贝：sessionId→SessionSnapshots 查 cwd/cliTool/claudeID → 仅 claude/reclaude 放行 → claudeID 非空走 transcript.ResolveClaudePath（含同样的「不回退扫最新」注释）/ 否则 scanner.ListProjectSessions 取最新 claude jsonl → transcript.Parse(file, after) → hudstats.Collect(file)。transcriptsvc 包注释自己承认「本服务把 webapi handler 的同一套逻辑包成一个 Wails 方法」。两份代码连空时间线/unsupported 的分支语义都一一对应，今后任何修改（如 c454543 这类时间线竞态修复牵连）都要双写。

**建议**：让 webapi 网关依赖注入 *transcriptsvc.Service（ServiceDeps 加一个 Timeline 窄接口即可，装配点在 internal/app/app.go 的 AttachServices），handleSessionTimeline 退化为「解析 query 参数 → 调 GetTimeline → writeJSON」约 20 行；或反向抽一个共享包级函数供两端调用。删除 rest.go 中重复的定位/解析逻辑。

**对抗核实裁决（成立）**：反驳失败，重复属实且可合并。证据：① 两份代码逐行对应——rest.go:218-295 与 transcriptsvc.go:59-116 的流水线（SessionSnapshots 查 cwd/cliTool/claudeID → 仅 claude/reclaude 放行 → ResolveClaudePath 或 ListProjectSessions 回退 → transcript.Parse → hudstats.Collect）完全同构，连「不回退扫最新」注释都逐字双份（rest.go:261 / transcriptsvc.go:86），且 transcriptsvc 包注释自认「把 webapi handler 的同一套逻辑包成一个 Wails 方法」；② 双写成本已实际发生——commit 49ef78e（卡片状态行指标）被迫在 rest.go:293 与 transcriptsvc.go:114 各加一次 hudstats.Collect；③ 不属于 worktree/composer 新功能线——handler 来自 d01ceb9（P7 Web 网关）、transcriptsvc 来自 048a658（时间线），与最近 worktree/composer 提交（e87efef/09bcb10/e768b30/fa1f/b43236f）无关，且建议是去重重构而非删功能，两端能力均保留；④ 合并无结构障碍——webapi 引 transcriptsvc 无 import 环（后者只依赖 terminal/sessionscan/transcript/hudstats，皆为 webapi 既有依赖），internal/app/app.go:350 的 AttachServices 在 transcriptSvc 构造点（app.go:120）之后，注入窄接口现成；rest_test.go 经 grep 无任何 timeline 测试钉死现行为；⑤ 前端兼容——frontend/src/services/transcript/TranscriptService.ts 对两种响应形状均做了可选处理（res.title 可选、...(res.metrics ? ...) 展开），Web HTTP 端点唯一消费者就是自家前端的 !isDesktop 分支。

**执行风险**：合并时须对齐三处微小语义差异，否则会引入行为漂移：① 依赖缺失分支——webapi 现要求 Sessions 与 Scanner 都非 nil 才放行（rest.go:235），transcriptsvc 容忍 scanner==nil（仅跳过旧会话回退、claudeID 为空时返回 supported=true 空时间线），合并后 Web 端这个角落分支会随 transcriptsvc 语义（生产装配两者恒注入，仅影响测试/残缺装配场景）；② 错误映射——GetTimeline 出错时返回 (emptySupported, err)，薄 handler 必须先判 err 映射回 HTTP 500「扫描转录失败」，不能直接拿返回值写 200；③ JSON 形状微差——unsupported 响应会多出 "title":""、metrics 键由恒出（可为 null）变 omitempty 省略，现行前端兼容，但若有第三方直接消费原始 HTTP API 会看到形状变化。另外时间线功能线本身未 push、状态行（49ef78e）待手测，重构宜在手测收口后做，避免在待验证代码上叠加变更。

## #38 【重复冗余·high】frontend/src/components/x-pro 自实现 Pro 组件库（48 文件 ~3.5k 行）在应用内零引用

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/components/x-pro/
- **估计规模**：48 个文件 / 约 3534 行 / 约 204KB 源码

**证据**：grep 'x-pro' 在 frontend/src 全部 .ts/.tsx 中零命中（目录内部互相用相对路径 import，目录外无人 import）；按组件名反查 DataGridPro/SankeyChart/GanttChart/Heatmap/Funnel/Radar/Treemap/Candlestick，排除 x-pro 自身后全仓零消费方。唯一用图表的页面 SettingsPage/database/Database.tsx 用的是官方 @mui/x-charts 的 PieChart/BarChart，与 x-pro 无关。该目录来自 foundation 框架合并 commit b25bc86，非最近 5 个 worktree/composer commit 的新功能。前端组件不存在动态字符串引用面（路由表 routes.tsx 也未引用）。

**建议**：整目录删除（48 个文件）。它是 foundation 脚手架的「自实现 MUI X Pro 功能」展示库，XK-Pane 业务从未消费；如担心日后要用，留在上游 foundation 仓库即可，本仓不必背着。删除后同步在 frontend/CLAUDE.md 与 foundation-mui-x-charts / foundation-mui-x-data-grid 两个 skill 描述里注明已移除。

**对抗核实裁决（成立）**：对抗性反查全部失败，该减法建议成立。证据：(1) grep 'x-pro' 在 frontend/src 的全部命中均位于 frontend/src/components/x-pro/ 目录内部，目录外零 import，也不存在 components 桶文件转出口；(2) index.ts 导出的全部运行时符号（FunnelChart/RadarChart/CandlestickChart/SankeyChart/GanttChart/TreemapChart/ZoomableChart/HeatmapChart、useChartColors/useChartDimensions/useChartTooltip、DataGridPro/DataGridPremium、useRowGrouping/useAggregation/useExcelExport/useCellSelection）逐一反查，命中文件全在 x-pro 自身；(3) 按小写域名词 sankey/gantt/treemap/candlestick/funnel/heatmap/radar 大小写不敏感全查，目录外零命中，排除字符串/注册表式动态引用；(4) routes.tsx、App.tsx、vite.config.ts、Taskfile.yml 均无引用，无 React.lazy 动态 import 指向该目录，无 import() 模板字符串拼接；(5) Web 镜像端 viewer 组件与桌面端同住 frontend/src，已被同一 grep 覆盖，同样零消费；(6) 唯一用图表的页面 frontend/src/pages/SettingsPage/database/Database.tsx 第 8-9 行 import 自官方 @mui/x-charts/PieChart 与 @mui/x-charts/BarChart，与 x-pro 无关；(7) bindings 调用面不适用——x-pro 是纯前端组件库而非后端 service；(8) git log --follow 确认该目录来自 commit b25bc86「整合 foundation 框架升级」，最近 5 个 worktree/composer commit 均未触及，不属于待手测新功能豁免范围。结论：48 个文件约 3.5k 行确为零消费的脚手架展示库，可整目录删除。

**执行风险**：风险很低但非零：(1) 文档失配——.claude/skills/foundation-mui-x-charts 与 foundation-mui-x-data-grid 两组 SKILL.md 及其 references/ 子文档把这些组件描述为「项目自实现高级图表 / Pro 功能」，删除后若不同步更新，后续 AI 按 skill 指引可能 import 不存在的组件导致编译失败——建议中已包含同步改文档，需确保执行；(2) 上游同步成本——该目录来自 foundation 框架整合 commit（b25bc86），日后再整合上游 foundation 时可能产生目录级冲突或被无意重新引入，整合时需留意；(3) .xkpanes/plans/ 下一份历史 plan 文件提及 x-pro，属归档文档无需处理。删除本身不影响 pnpm typecheck / pnpm build（零消费方，移除未引用代码不会破坏编译）。

## #39 【重复冗余·medium】子窗口系统（childwindow）前后端全链路就位但零业务调用方

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/app/childwindow.go、J:/Program/Agent/XK-Pane/frontend/src/services/childwindow/、J:/Program/Agent/XK-Pane/frontend/src/childwindows/、J:/Program/Agent/XK-Pane/frontend/child.html、J:/Program/Agent/XK-Pane/frontend/src/child.tsx、J:/Program/Agent/XK-Pane/frontend/vite.config.ts
- **估计规模**：约 820 行（前端 ~700 + 后端 ~120）+ 一个 vite 构建入口

**证据**：grep 'ChildWindowService.' 在 frontend/src（排除 services/childwindow 自身）零命中——没有任何页面/组件调用 open/close/list；后端 ChildWindowService 仅在 app.go:129 构造 + app.go:212 注册，Go 侧无其他调用。链路包含 vite 第二入口 child.html（vite.config.ts:55）+ child.tsx + 三个预设窗口（Confirm/Message/Blank 各含 View/VM/Style/lang 四件套）≈700 行前端 + 后端 childwindow.go ~120 行。所有确认/提示场景实际都走 NativeDialogs（Wails 原生对话框铁律），子窗口系统从合入（foundation 框架升级 b25bc86）至今无人消费。

**建议**：若 XK-Pane 近期无多窗口需求，整套删除：childwindow.go + 注册行、frontend/src/services/childwindow、frontend/src/childwindows、child.html/child.tsx、vite.config.ts 的 child 入口，并更新 foundation-childwindow skill 说明。保守做法是仅删三个预设窗口留骨架，但「半留」仍是死代码，建议整删（上游 foundation 仓有完整版可随时取回）。

**对抗核实裁决（成立）**：反驳失败，建议成立。逐项反查证据：① bindings 调用面——全仓 grep "ChildWindow|childwindow|childWindow"，frontend/src 命中文件仅为子窗口系统自身（services/childwindow/{ChildWindowService,bus,index}.ts、childwindows/{Confirm,Message,Blank}Window/*、child.tsx），唯一的 bindings 动态 import 在 ChildWindowService.ts:15（@bindings/xkpane/internal/app/childwindowservice），而 ChildWindowService.open/close/list 没有任何主应用页面/组件调用；services barrel 也未 re-export（services/index.ts 不在命中列表）。② 后端——internal 下 "ChildWindow" 仅命中 internal/app/childwindow.go 定义自身与 app.go 三处装配（129 构造、212 RegisterService、251 AttachApp），无业务调用。③ webapi 路由——internal/webapi 零命中；P7_WEB_PLAN.md:193 反而把 childwindow 明确列入「永不挂远程路由」红线清单，证实 Web 镜像端永远不会消费它。④ 事件字符串——child:result/send/broadcast/closed 四个事件名只存在于 childwindow.go（emit child:closed:+id）与 bus.ts 自身，internal/events/events.go 无任何 child 事件注册，无第三方订阅/发射。⑤ 动态字符串引用面（hooks/CLI 适配器/子进程白名单/i18n/路由 path）——全仓命中的 23 个文件全是系统自身、foundation-childwindow skill 文档与 wails-window 参考文档，无一处业务引用。⑥ React.lazy/动态 import——child.tsx 是 vite 独立入口（vite.config.ts:55），不进主 bundle，主应用无任何 import('@/childwindows/...')。⑦ Taskfile.yml 零命中。⑧ 新功能豁免不适用——git log 显示整套子窗口系统来自 b25bc86「整合 foundation 62ee479 框架升级」，而最近 5 个 worktree/composer commit（HEAD~5..HEAD）未触及任何 childwindow 文件，不属于待手测新功能。所有确认/提示场景实际由 NativeDialogs（Wails 原生对话框铁律）承担，子窗口系统自合入以来确为零消费的前后端全链路死代码。

**执行风险**：三个执行风险点：① P7_WEB_PLAN.md:160 铁律写明「systray/childwindow 一律不动」——语境是 Web 镜像期间桌面零回归，不构成「被使用」的反证，但若 P7 计划仍在推进，删除前应同步更新该文档措辞，避免后续审查按旧铁律误判。② 这是上游 foundation 脚手架的框架级预留能力（配套 foundation-childwindow skill），删除后若未来出现多窗口需求需从上游 foundation 仓取回；且下次像 b25bc86 那样整合 foundation 框架升级时可能被重新带回，造成 diff 噪音。③ 删除需一次清干净：app.go 三处（129/212/251）、childwindow.go、frontend/src/services/childwindow、frontend/src/childwindows、child.html、child.tsx、vite.config.ts:55 的 child 入口，并重跑 wails3 generate bindings，同时删除或标注 .claude/skills/foundation-childwindow/SKILL.md 防止变成幽灵文档误导后续 AI 按它找不存在的代码。

## #40 【重复冗余·high】Greet 脚手架示例服务残留：前后端两层包装均无消费方

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/greet/greet.go、J:/Program/Agent/XK-Pane/frontend/src/services/greet/GreetService.ts、J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/frontend/src/services/index.ts
- **估计规模**：约 30 行代码 + 1 个注册的 Wails service + bindings 一个生成目录

**证据**：已按 bindings 路径反查：前端 grep GreetService 只命中定义（GreetService.ts）与桶导出（services/index.ts:1），无任何页面/VM 调用——HomePage 的「问候语」是本地按小时计算（useHomePage.ts:17 greetingForHour），与 Greet 服务无关。后端 Greet 方法 Go 侧零调用，webapi 白名单（rest.go）也未路由它，仅 app.go:190 注册。顺带回答「前端 services/ 纯透传包装」一问：其余包装（ProjectService/SessionScanService 等）都做了类型规整 + isDesktop 双路分支，属架构要求非冗余；唯一纯透传且无人用的就是 GreetService。

**建议**：删除 internal/services/greet 包、app.go:190 注册行、frontend/src/services/greet 目录与 index.ts 的导出，然后 wails3 generate bindings 重新生成。注意同步删 CLAUDE.md / frontend/CLAUDE.md 里以 greet 作示例的表述（或保留文档但标注已移除）。

**对抗核实裁决（成立）**：对抗性反查全部失败，建议成立。逐面证据：(1) bindings 调用面——frontend/src 大小写不敏感全量 grep，GreetService 仅命中定义文件 frontend/src/services/greet/GreetService.ts（包装 @bindings/xkpane/internal/services/greet）与桶导出 frontend/src/services/index.ts:1，无任何页面/组件/ViewModel 消费；HomePage 的问候语是本地纯函数 greetingForHour（useHomePage.ts:17/46，按小时返回 morning/afternoon/evening）+ i18n key home.greeting.*，与 Greet 服务无关。(2) webapi 路由面——internal/webapi 目录 grep greet 零命中，Web 镜像端也没用它。(3) 事件面——全仓 .go grep greet 仅命中 app.go:24 import、app.go:190 注册行、greet.go 自身，events.go 无相关事件。(4) 动态字符串面——hooks/cliadapters/subprocess 白名单已被 .go 全量 grep 覆盖且零命中，前端无 Call.ByName 类动态调用，路由 path 无 greet。(5) React.lazy 与其余前端引用——frontend 全目录（排 node_modules）grep Greet|services/greet，代码命中仅定义+桶导出，其余命中全是文档示例文字（frontend/CLAUDE.md:25/379、frontend/DEVELOPMENT.md:143-232）。(6) Taskfile/build/配置文件零命中。(7) 新功能豁免检查——git log 显示 greet 相关文件只被 Initial commit（97751b6）和 Phase 0 rebrand（b7281a3）触碰，与最近 5 个 worktree/composer 提交无关，属 foundation 脚手架原始示例残留，不在待手测豁免范围。greet.go 仅 11 行纯函数、无 ServiceStartup 副作用，前后端两层包装均为死代码，可安全删除。

**执行风险**：风险很低，主要是文档与生成物联动：(1) frontend/CLAUDE.md（第 25/379/438/445 行）与 frontend/DEVELOPMENT.md（第 77/143-232 行整段 MVVM 示例代码）以及根 CLAUDE.md 的通信链路图都以 Greet 作架构示例，删代码后这些文档指向不存在的代码，需同步改写或换用其他真实 service 作示例，否则误导后来者；(2) 删除后必须重新跑 wails3 generate bindings 并过一遍 pnpm typecheck（bindings 目录虽被 .gitignore，但本地残留旧 greet 绑定目录无害、删了更干净）；(3) frontend/src/services/index.ts 删第 1 行导出后仍有约 20 组其他导出，文件不会变空，无连锁影响。

## #41 【重复冗余·high】小工具函数多份拷贝：Go 侧 normalizePath 三份（两份逐字节相同）、前端 basename 六份、isValidResumeId 三份

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/workspace/workspace.go、J:/Program/Agent/XK-Pane/internal/services/worktreesvc/worktreesvc.go、J:/Program/Agent/XK-Pane/internal/services/orchestrator/orchestrator_tools.go、J:/Program/Agent/XK-Pane/frontend/src/components/WorkspaceTree/projectDisplay.ts、J:/Program/Agent/XK-Pane/frontend/src/components/WebWorkspaceTree/useWebWorkspaceTree.ts、J:/Program/Agent/XK-Pane/frontend/src/components/WebWorkspaceTree/WebWorkspaceTree.tsx、J:/Program/Agent/XK-Pane/frontend/src/rightpanel/RightPanelDrawer.tsx、J:/Program/Agent/XK-Pane/frontend/src/project/useRegisteredProjects.ts、J:/Program/Agent/XK-Pane/frontend/src/components/WorkspaceTree/useSessionSubList.ts、J:/Program/Agent/XK-Pane/frontend/src/pages/HomePage/useHomePage.ts
- **估计规模**：约 50-70 行 + 消除 Windows 路径大小写口径漂移的隐患

**证据**：Go：workspace.go:430 normalizeProjectPath 与 worktreesvc.go:431 normalizePath 实现逐字节相同（统一斜杠+去尾斜杠+Windows 转小写，后者注释自述「与 workspace service 同口径」）；orchestrator_tools.go:253 是第三份变体（不转小写，注释称复刻 cc-pane）——同仓三份、口径还有微妙分歧，Windows 上大小写敏感性不一致是潜在 bug 面。前端：function basename 出现 6 处（projectDisplay.ts:6、useWebWorkspaceTree.ts:25、WebWorkspaceTree.tsx:40、toolMeta.tsx:112、RightPanelDrawer.tsx:17、useRegisteredProjects.ts:18），同一文件夹的 WebWorkspaceTree 里 VM 和 View 各写了一份；isValidResumeId（resume id 非空且不含 ':'）3 处：useSessionSubList.ts:25、useWebWorkspaceTree.ts:103、useHomePage.ts:70 内联。

**建议**：Go 侧抽 internal/utils/pathx（或并入现有 utils）提供 Normalize；至少把 workspace 与 worktreesvc 两份相同实现合一；orchestrator 若确需 cc-pane 保真的不转小写变体，保留但注释点明与 pathx.Normalize 的差异。前端在 src/shared/ 加 path.ts（basename）与 resume.ts（isValidResumeId），替换 6+3 处拷贝。

**对抗核实裁决（成立）**：反驳失败，重复全部实证成立。Go 侧：workspace.go:430-437 normalizeProjectPath 与 worktreesvc.go:431-438 normalizePath 函数体逐字节相同（统一斜杠+去尾斜杠+Windows 转小写），后者注释自认「与 workspace service 的 normalizeProjectPath 同口径」——即靠人工保持同步的拷贝；orchestrator_tools.go:253 是第三份变体（不转小写，注释「复刻 cc-pane normalize_path」）。前端：basename 6 处全部实测在位，其中三对逐字节相同——RightPanelDrawer.tsx:17 与 projectDisplay.ts:6 完全一致、useWebWorkspaceTree.ts:25 与同目录 WebWorkspaceTree.tsx:40 完全一致（VM 与 View 各抄一份）；isValidResumeId 3 处实测在位（useSessionSubList.ts:25、useWebWorkspaceTree.ts:103 具名同实现，useHomePage.ts:70 内联同逻辑 !r.resumeSessionId.includes(':')）。反查引用面无阻碍：这些全是 Go 包内非导出函数 / 前端模块局部函数（唯一例外 toolMeta.tsx:112 的 export basename 仅被同目录 DiffCard.tsx:12 引用），不是 service 方法、事件名、路由 path、i18n key 或子进程白名单条目，不可能被 bindings / webapi / 动态字符串引用，合并不会破坏任何外部调用。新功能豁免不适用：git log 证实仅 worktreesvc.go 一份属最近 worktree 分支（e87efef），其余全部拷贝来自更早提交（workspace.go=20a9f7a、orchestrator=23d7956、前端各份=97751b6/d3c12b0/cc600bd/5709534/ae2d4a7/e1c6f3a/9a70edb），剔除新功能那一份后发现依然成立——这不是把新功能报成死代码，而是跨新旧代码的真实重复。且 frontend/src/shared/ 已存在（platform.ts/uuid.ts/clipboard.ts 等），加 path.ts/resume.ts 与现有结构一致。

**执行风险**：三点：① worktreesvc.go 那一份属于待手测的 feat/paseo-worktree-composer 分支，建议等手测通过后再合并它，避免给未验证功能添改动噪音（其余拷贝可先合）。② orchestrator_tools.go 的变体是故意不转小写（复刻 cc-pane，用于 MCP 工具白名单/去重比对），合并时绝不能误统一成转小写版，否则改变白名单比对语义——按原建议保留为独立变体并注释差异即可。③ 前端 6 份 basename 有边缘行为差异（toolMeta 版接受 undefined 返回 ''、对尾斜杠路径会整串返回；其余两种风格剔除尾斜杠），统一为 filter(Boolean) 语义时需扫一遍调用点确认（实查均为展示名提取，行为兼容）；toolMeta.basename 是 export 且被 DiffCard.tsx:12 引用，替换时要同步改该 import。

## #42 【重复冗余·medium】WorkspaceTree 与 WebWorkspaceTree：不建议淘汰任何一边，但「会话子列表 + resume 守门」子集重复可抽公共 hook

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/components/WorkspaceTree/、J:/Program/Agent/XK-Pane/frontend/src/components/WebWorkspaceTree/
- **估计规模**：约 80-120 行重复 VM 逻辑

**证据**：两套合计 2622 行（桌面 ~1684 / Web ~938），但逐行对比后真正重复的逻辑有限：交互模型根本不同——桌面是右键 ContextMenu + NativeDialogs + DnD 排序 + worktree 隔离启动 + SSH 项目登记（useWorkspaceTree.ts 暴露 20 个 mutation 方法），Web 是 fail-closed 只读 + ⋮ 锚定菜单 + /api/launch（useWebWorkspaceTree.ts 仅浏览/恢复/新建），数据源也分桌面 bindings 与 HTTP 双路。真正重复的子集：①按需扫盘加载项目历史会话（useSessionSubList.ts:35-51 与 useWebWorkspaceTree.ts:183-206 同语义）；②resume 守门 isValidResumeId（两处+HomePage 内联）；③basename/label 计算与展开互斥 state。合并整组件会把 fail-closed 边界和移动端交互搅进桌面代码，违背既有 viewer 专用组件范式。

**建议**：保留双组件，只做小抽取：新建 shared hook useProjectSessions(projectPath)（封装 SessionScanService 按需加载 + loading/error 态）供 useSessionSubList 与 useWebWorkspaceTree.loadSessions 共用；label/basename 与 isValidResumeId 并入上一条的 shared 工具。不要试图抽「通用树组件」——两边 View 的差异（菜单/对话框/DnD）远大于共性。

**对抗核实裁决（成立）**：建议整体站得住脚，核心事实全部核实：①行数精确吻合（WorkspaceTree 不含 lang 1684 行 / WebWorkspaceTree 938 行，合计 2622）；②「不合并整组件」的判断正确——useWorkspaceTree.ts 确有约 20 个 mutation 方法（submitCreate/deleteWorkspace/launchInWorktree/importProject/addSSHProject/reorderWorkspaces 等，含最近 worktree 分支 09bcb10 新增的 launchInWorktree），Web 侧 fail-closed 只读 + OrchestratorService.launch（HTTP），数据与动作路径根本不同，硬合并会破坏 viewer 专用组件范式；③重复子集确证：isValidResumeId 三处（useSessionSubList.ts:25-27、useWebWorkspaceTree.ts:103-105 一字不差，useHomePage.ts:70 内联等价 !r.resumeSessionId.includes(':')）；扫盘加载两处同语义（useSessionSubList.ts:35-51 与 useWebWorkspaceTree.ts:183-190 都调 SessionScanService.listProjectSessions）；basename 实际比发现声称的更严重——全前端 7 处定义（projectDisplay.ts:6、WebWorkspaceTree.tsx:40 与 useWebWorkspaceTree.ts:25 同一组件 View/VM 各一份、useRegisteredProjects.ts:18、RightPanelDrawer.tsx:17、toolMeta.tsx:112、paneTree.ts:73），抽公共工具的减法价值确凿。该发现不属于「把新功能当死代码删」——它明确保留双组件，仅提议去重小工具。

**执行风险**：两点：①建议中的共享 hook useProjectSessions(projectPath) 对 Web 侧并非即插即用——useWebWorkspaceTree 的会话状态是树级单 hook 内的 Record&lt;projectId, SessionState&gt;（带缓存与 error 标签联合），loadSessions 是 useCallback 而非 hook 调用点，按 React 规则无法在回调里逐项目调 hook；要共用须把 WebWorkspaceTree 重构为逐项目子组件（改动远超「小抽取」），或退化为抽普通函数（服务层本已共享，仅省约 15 行），且两侧语义有意不同（Web 缓存已扫结果不重扫、桌面每次挂载重扫并带 alive 清理），强行统一可能改变 viewer 行为。建议只落地 isValidResumeId 与 basename 的公共工具抽取，hook 抽取慎做或不做。②WorkspaceTree 刚被未 push、待手测的 feat/paseo-worktree-composer 分支改动（09bcb10/b43236f 的 launchInWorktree 链路），现在动这些文件做重构会干扰待手测功能的验证，应等手测通过后再做。

## #43 【过度抽象·high】frontend/src/components/x-pro/ 整目录（自实现 MUI Pro/Premium 功能）零引用，整块可删

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/src/components/x-pro/、J:/Program/Agent/XK-Pane/frontend/package.json、J:/Program/Agent/XK-Pane/frontend/vite.config.ts、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-data-grid/、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-charts/
- **估计规模**：代码 3534 行（50 文件）+ 1 个 npm 依赖 + 约 5000 行 skill 文档

**证据**：x-pro 含 50 个文件共 3534 行：7 个高级图表（Funnel/Radar/Candlestick/Sankey/Gantt/Treemap/Zoomable + 未导出的 Heatmap）、DataGridPro/DataGridPremium 及 14 个 feature hook（列钉住/树形/行分组/聚合/Excel 导出等）。在 frontend/src 全量 grep 'x-pro' 与全部组件名（DataGridPro|FunnelChart|...）排除 x-pro 自身后零匹配；6 个 vitest 测试文件也无引用。依赖 @mui/x-data-grid 仅被 x-pro 使用（排除 x-pro 后 grep '@mui/x-data-grid' 零匹配），@mui/x-charts 另有 SettingsPage/database/Database.tsx 使用须保留。vite.config.ts:38 的 '@mui/x-data-grid' 预构建条目随之可删。配套 skill 文档 foundation-mui-x-data-grid + foundation-mui-x-charts 共 5787 行，其中自实现部分章节随组件一起失效。

**建议**：删除 frontend/src/components/x-pro/ 整目录；从 package.json 移除 @mui/x-data-grid 依赖；删 vite.config.ts 中对应 optimizeDeps 条目；同步删除/缩减 .claude/skills/foundation-mui-x-data-grid（整个）与 foundation-mui-x-charts 中「项目自实现高级图表」章节。将来真要用 Pro 功能时按需从 git 历史找回。

**对抗核实裁决（成立）**：对抗反查全部失败，建议成立。逐面证据：(1) 引用面——grep 'x-pro' 在 frontend/src 文件内容中零匹配（静态 import、相对路径 '../x-pro'、React.lazy 动态 import('…x-pro…') 均必然含该子串，故一并排除）；x-pro/index.ts 导出的全部 17 个标识符（DataGridPro/DataGridPremium/FunnelChart/RadarChart/CandlestickChart/SankeyChart/GanttChart/TreemapChart/ZoomableChart/HeatmapChart 及各 hook）的 grep 匹配全部限于 x-pro 自身 36 个文件，外部零使用。(2) bindings/webapi/事件——x-pro 是纯前端 TSX 组件目录，不涉后端 service/bindings/webapi；repo 全量 grep 'x-pro' 仅命中 .claude/skills 文档、package.json、vite.config.ts、pnpm-lock.yaml。(3) 动态字符串面——非页面组件，routes.tsx 无引用（组件名 grep 零外部匹配），与 i18n key/hooks/CLI/子进程白名单无关。(4) 测试——仓库 6 个 .test.ts（splitGeometry/terminalTheme/events/layoutScheduler/reducer/sessionReconcile）均不引用 x-pro。(5) Taskfile——零命中。(6) 依赖核查——@mui/x-data-grid 的全部 16 处 import 都在 x-pro 内部（frontend/src/components/x-pro/data-grid/ 下），删目录后 package.json:26 依赖与 vite.config.ts:38 optimizeDeps 条目确可一并移除；@mui/x-charts 被 frontend/src/pages/SettingsPage/database/Database.tsx:8-9（PieChart/BarChart）使用须保留，且 x-pro 图表为自绘实现并不 import @mui/x-charts，二者解耦，建议对此的区分正确。(7) 新功能排除——x-pro 由 commit b25bc86（2026-06-01，feat(framework) 整合 foundation 62ee479 框架升级）从 foundation 脚手架合并引入，不属于最近 5 个 worktree/composer 提交，也非「待手测」功能，是合并进来后从未被业务消费的储备组件库。

**执行风险**：主要是「脚手架储备」属性带来的低风险：(1) x-pro 来自 foundation 上游框架整合（b25bc86），删除后若再次从 foundation 同步框架升级，可能产生目录级合并差异，或上游后续引用这些组件的代码同步进来时缺失，需在同步时留意；找回路径是 git 历史（b25bc86）。(2) vite.config.ts 只能删第 38 行 '@mui/x-data-grid'，第 37 行 '@mui/x-charts' 必须保留（Database.tsx 在用），误删会复发冷启动 optimizeDeps 重载竞态。(3) 整删 .claude/skills/foundation-mui-x-data-grid 时需同步修改 .claude/skills/foundation-mui/SKILL.md:18 索引表中 @mui/x-data-grid 行，否则 skill 总索引悬空；foundation-mui-x-charts 只删「自实现高级图表」章节、保留社区版章节。(4) 执行后建议跑 pnpm typecheck + pnpm build 验证（零引用理应直接通过）。

## #44 【过度抽象·high】subprocess 白名单子进程服务端到端投机：白名单恒空 + 前端零消费者

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/subprocess/commands.go、J:/Program/Agent/XK-Pane/internal/services/subprocess/subprocess.go、J:/Program/Agent/XK-Pane/frontend/src/services/subprocess/SubprocessService.ts、J:/Program/Agent/XK-Pane/internal/app/app.go
- **估计规模**：约 460 行（Go 341 + TS 107 + 装配/导出若干行）

**证据**：commands.go:89 `var whitelist = []Command{}` 为空数组，仅含两段注释掉的示例（ping / git-status）——任何调用都会在 lookupCommand 落空报错，服务当前不可能执行任何命令。前端侧已核对 bindings 引用面：SubprocessService.ts（107 行）import 了 '@bindings/xkpane/internal/services/subprocess'，但除 services/index.ts 的 re-export 外，pages/components/ViewModel 中 grep 'SubprocessService' 零消费者。Go 侧 grep 'services/subprocess' 仅 app.go 装配处引用。Command 结构体 7 个字段（ArgPattern/MaxArgs/AllowedCwdRoots/PrependArgs 等）从未被任何真实条目实例化。

**建议**：摘除 app.go 中 subprocess 的构造与 RegisterService，删除 internal/services/subprocess/（341 行 Go）与 frontend/src/services/subprocess/（107 行 TS）及 services/index.ts 的 re-export。CLAUDE.md 的「白名单子进程」铁律作为规范文字保留即可，等第一个真实命令出现时再按 git 历史恢复实现。

**对抗核实裁决（成立）**：对抗性反查全面失败，建议成立。逐面证据：① 白名单恒空——commands.go:89 `var whitelist = []Command{}` 确为空，仅含注释掉的 ping / git-status 示例，lookupCommand 必落空，Run 任何调用都报错，服务在功能上等价于不存在；② bindings 调用面——前端 src 全量 grep 只命中 SubprocessService.ts 自身与 services/index.ts 的 re-export，pages/components/ViewModel 零消费者，即使经 index 桶导入也无人使用；③ webapi——internal/webapi 零命中，Web 镜像端同样没用；④ 事件字符串——事件名 `subprocess:stdout:<id>` 等只在 SubprocessService.ts 内自产自销，internal/events/events.go 无注册，前端无其他 Events.On 订阅；⑤ 动态字符串引用——projecthooks / cliadapters / i18n key / 路由 path / Taskfile.yml / build/ 全部零命中，terminal.go:183 仅注释里参照其「AttachApp 回填」模式，非代码依赖；⑥ React.lazy / 动态 import——前端全量字符串 grep 已覆盖，无；⑦ 新功能豁免——git log --follow 显示 internal/services/subprocess/ 与 frontend/src/services/subprocess/ 均只有 97751b6（Initial commit）与 b7281a3（Phase 0 rebrand）两个 commit，属 foundation 脚手架原始预置，与最近 5 个 worktree/composer commit 无关，不在待手测范围。Go 侧引用仅 app.go 的 import / New / NewService / AttachApp 四处装配代码，按建议一并摘除即可，删除是端到端干净的。

**执行风险**：三个低风险点：① 文档失配——CLAUDE.md「前端调子进程必须经 internal/services/subprocess 白名单」铁律及 .claude/skills/foundation-utils/SKILL.md、.claude/skills/INDEX.md、README.md 中指向该实现的段落会变成悬空引用，删除时需同步把文档措辞改为「需要时按 git 历史（b7281a3 之前）恢复实现」；② 删除后需重跑 wails3 generate bindings 清掉本地残留的 subprocess 绑定目录（gitignore 产物，不重生成也不会编译失败，但会留脏）；③ 脚手架模板属性——该服务是 foundation 脚手架的预置安全样板，若本仓库仍兼任上游模板供新项目 fork，删除会让下游失去现成的白名单子进程范式；纯就 XK-Pane 业务仓库而言无运行时风险。

## #45 【过度抽象·high】子窗口系统（childwindow）整套零使用：无任何打开者

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/app/childwindow.go、J:/Program/Agent/XK-Pane/frontend/src/childwindows/、J:/Program/Agent/XK-Pane/frontend/src/services/childwindow/、J:/Program/Agent/XK-Pane/frontend/src/child.tsx、J:/Program/Agent/XK-Pane/frontend/child.html、J:/Program/Agent/XK-Pane/frontend/vite.config.ts、J:/Program/Agent/XK-Pane/.claude/skills/foundation-childwindow/SKILL.md
- **估计规模**：约 730 行代码（Go 122 + 前端约 605）+ 588 行 skill 文档

**证据**：Go 侧 ChildWindowService.Open（childwindow.go:46）在后端无任何调用方：grep 'childWinSvc' 仅 app.go 三处（构造/注册/AttachApp），grep 'ChildWindowOptions{' 在 internal 下零匹配。前端侧已考虑 bindings 动态引用：唯一 import bindings 的是 services/childwindow/ChildWindowService.ts，但 grep 'ChildWindow' 在 pages/ 与 components/ 下零匹配，services/index.ts 也未 re-export——即主窗口 UI 从未打开过任何子窗口；childwindows/{Blank,Confirm,Message}Window 只在 child.tsx 入口（子窗口自身运行时）被引用，形成自闭环死代码。vite.config.ts:55 的 child 多入口与 child.html 仅为此服务。这是 foundation 脚手架能力原样带入 XK-Pane 的遗留（记忆中 systray 弹出面板已删，本系统是另一套）。

**建议**：删除 internal/app/childwindow.go（122 行）及 app.go 三处装配；删除 frontend/src/childwindows/、frontend/src/services/childwindow/、src/child.tsx、child.html，并移除 vite.config.ts 的 child 入口；.claude/skills/foundation-childwindow（588 行）一并删除。

**对抗核实裁决（成立）**：对抗反查全部失败，建议成立。（1）bindings 面：全前端唯一 import '@bindings/xkpane/internal/app/childwindowservice' 的是 frontend/src/services/childwindow/ChildWindowService.ts:15，但该包装 service 的 open/close/list 在 pages/ 与 components/ 下零调用，services/index.ts 也未 re-export——主窗口 UI 从未打开过任何子窗口；child.tsx 与 childwindows/ 三个窗口组件只消费 bus/params 工具且只能在子窗口运行时执行，而子窗口唯一打开入口 Go 侧 ChildWindowService.Open（internal/app/childwindow.go:46）在后端无任何调用方（grep childWinSvc 仅 app.go:129/212/251 构造、注册、AttachApp 三处），构成确凿的自闭环死代码。（2）webapi 面：internal 下 ChildWindow 仅出现在 internal/app 两文件，webapi 无引用，且 P7_WEB_PLAN.md:193 明确将 childwindow 列入「永不挂远程路由」红线，Web 镜像端永不消费。（3）事件字符串面：child:result/send/broadcast/closed 仅存在于 childwindow.go:15-18,82、services/childwindow/bus.ts:5-8 与 SKILL.md，无任何外部订阅者。（4）动态字符串面：'/child.html' URL 仅 childwindow.go:59 构造，NewWithOptions 在 internal 仅 childwindow.go:61 与 app.go:289（主窗口）两处；hooks 注入、CLI 适配器、subprocess 白名单、路由 path 均无引用；childwindow.* i18n keys 仅在 childwindows/ 自身 lang 文件注册，删除不留悬挂 key。（5）React.lazy/动态 import 面：唯一动态 import 即 ChildWindowService.ts 自身的 bindings import。（6）Taskfile 面：全部 Taskfile 无 child 引用。（7）新功能豁免检查：childwindow 整套来自 commit b25bc86「feat(framework): 整合 foundation 框架升级（子窗口/系统托盘/MUI X Pro）」即 foundation 脚手架原样带入，最近 5 个 worktree/composer 提交（c454543/b43236f/fa1ea1=fa1.../e768b30/09bcb10）均不触及这些文件，不属于待手测新功能，不适用豁免。

**执行风险**：风险较低但有四点注意：（1）P7_WEB_PLAN.md:160 的铁律把 systray/childwindow 列入「桌面零回归一律不动」、:193 列入「永不挂远程路由」清单——该铁律本意是约束 webapi 改造不破坏桌面侧，并非禁止删除，但执行删除后这两处计划文档需同步更新，否则文档与代码失真；（2）删除 app.go 三处装配后必须重新跑 wails3 generate bindings 清掉 childwindowservice 生成物，并跑 go build + pnpm typecheck + pnpm build 验证（vite.config.ts 移除 child 多入口后 rollup input 仅剩 main）；（3）这是 foundation 脚手架的通用能力，若未来 XK-Pane 需要轻量置顶浮窗（confirm/message 类）需重新引入——但现有 NativeDialogs（Wails Dialogs.*）已覆盖确认/提示场景，实际需求缺口很小；（4）.claude/skills/foundation-childwindow 删除后该 skill 从可用列表消失，属预期一致行为（该 skill 只描述这套系统本身）。

## #46 【过度抽象·high】greet 演示服务端到端死链：后端注册 + 前端包装层均无人调用

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/greet/greet.go、J:/Program/Agent/XK-Pane/frontend/src/services/greet/GreetService.ts、J:/Program/Agent/XK-Pane/frontend/src/services/index.ts、J:/Program/Agent/XK-Pane/internal/app/app.go
- **估计规模**：约 35 行（Go 16 + TS 12 + 注册/导出行）

**证据**：已按 bindings 反查规则核对：前端 GreetService.ts import 了 '@bindings/xkpane/internal/services/greet'，但 grep 'GreetService' 全前端仅命中其自身与 services/index.ts:1 的 re-export，无任何组件/ViewModel 消费。HomePage 的问候已改为本地按小时计算（useHomePage.ts:17 greetingForHour），不再调后端。Go 侧 grep 'greet\.' 仅 app.go:190 的 application.NewService(greet.New()) 一处。webapi（Web 镜像端）也无 greet 路由。

**建议**：删除 internal/services/greet/、app.go:190 的注册行、frontend/src/services/greet/ 与 services/index.ts 的导出行，然后重新 wails3 generate bindings。

**对抗核实裁决（成立）**：对抗反查全部覆盖面后无法反驳，greet 确为端到端死链：① bindings 反查——frontend/src 全量 grep 'GreetService' 仅命中自身定义（frontend/src/services/greet/GreetService.ts）与 services/index.ts:1 的 re-export，逐一核对全部 70 余处 `from '@/services'` 的 import，无任何组件/ViewModel 消费；② HomePage 已自给——useHomePage.ts:17 的 greetingForHour 纯前端按小时算问候，import 的是 LaunchHistoryService，与后端无关；③ Go 侧——全仓 .go 文件 grep 仅命中 app.go:24（import）、app.go:190（application.NewService(greet.New()) 注册）与 greet.go 自身，无业务调用；④ webapi——internal/webapi 下 grep 零命中，Web 镜像端无 greet 路由；⑤ 动态字符串面——events.go、projecthooks、cliadapters、subprocess/commands.go、路由表、i18n key、React.lazy、Taskfile/yml/json 均零命中；⑥ 非新功能——greet 是 foundation 脚手架初始演示 service，与最近 worktree/composer 5 个 commit 无关，且 progress.md:928 已自行记录「遗留孤儿：GreetService / 后端 greet service 现无消费方」，项目方早已确认其孤儿身份。

**执行风险**：两点：① 文档同步债——greet 链路被多份文档当作「前后端调用链教学样例」引用（根 CLAUDE.md 通信链路图与模块索引、README.md 改名对照表第 8 条与目录树、frontend/CLAUDE.md 交互边界表与 FAQ、frontend/DEVELOPMENT.md 的 MVVM 示例代码、.claude/skills/foundation-persistence/SKILL.md:159），删代码后必须同步改写这些文档，否则文档与代码脱节，且会削弱脚手架「示例 service」的教学价值；② 建议清单不完整——除 app.go:190 注册行外还需删 app.go:24 的 `"xkpane/internal/services/greet"` import（否则 Go 编译失败），并重新跑 wails3 generate bindings 清掉生成产物。功能层面无风险（无任何运行时消费方）。

## #47 【过度抽象·high】time 事件演示链：每秒空播一次的常驻 goroutine + 零订阅者的 hook

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/app/app.go、J:/Program/Agent/XK-Pane/internal/events/events.go、J:/Program/Agent/XK-Pane/frontend/src/shared/hooks/useTimeEvent.ts、J:/Program/Agent/XK-Pane/frontend/src/shared/events.ts
- **估计规模**：约 45 行 + 消除一个 1Hz 常驻定时器广播

**证据**：app.go:382 `go emitTimeLoop(app)` 启动 1 秒一次的 ticker 永久向前端 Emit events.Time（app.go:437），注释自述「演示后端主动推事件给前端的链路」。前端唯一订阅者 useTimeEvent.ts（24 行）经 grep 确认零消费者（仅 useTrayNavigation.ts 注释里提了一句「同款」）。webapi 的 mirroredEvents 白名单（rest.go:28-34）明确不转发 time——Web 端也用不到。结果是生产应用里一个每秒唤醒一次、序列化 RFC1123 时间字符串广播给零监听者的常驻 goroutine。

**建议**：删除 emitTimeLoop 及 app.go:382 的启动行、events.go 的 Time 常量与 RegisterEvent[string](Time)、前端 useTimeEvent.ts、shared/events.ts 的 Time 条目（events.test.ts 对应断言同步删）。

**对抗核实裁决（成立）**：反驳失败，建议成立。逐项核实：① 后端 internal/app/app.go:382 确有 `go emitTimeLoop(app)`，app.go:431-439 每秒 ticker 向 events.Time 广播 RFC1123 时间串，注释自述为演示代码；② 前端唯一订阅者 useTimeEvent.ts 经 grep 确认零代码消费者（frontend/src/pages 及全部组件无 import，仅 useTrayNavigation.ts:6 注释提及）；③ AppEvents.Time 仅存在于 events.ts 定义、events.test.ts 防漂移断言、useTimeEvent.ts 自身，transport 层不订阅；④ Web 镜像端 internal/webapi/rest.go:26-32 的 mirroredEvents 白名单明确排除 time，gateway.go RouteEvent 对其直接丢弃，注释点名 time 为「本地/无关事件」；⑤ 动态字符串反查：Go 侧裸 "time" 命中全是 bedrock 文档数据（Minecraft JSON），hooks/CLI 适配器/子进程白名单/i18n/路由 path 均无引用；⑥ 非最近 worktree/composer 新功能——是 foundation 脚手架原始 demo，且仓库 progress.md:928 早已自记「遗留孤儿：emitTimeLoop 现无消费方，后续可作脚手架清理」。生产应用里确实存在一条每秒空转广播给零监听者的常驻链路，整链可删。

**执行风险**：三个执行风险（均可控）：① internal/webapi/hub_test.go:85 的 TestRouteEventWhitelist 用 events.Time 常量充当「白名单外事件」样例，删常量会编译失败——需同步改为字面量 "time" 或 events.LocalHistoryChanged（同为白名单外），测试语义不变；② 文档漂移：根 CLAUDE.md 通信链路图、frontend/CLAUDE.md（事件订阅表第 30 行 + 文件清单第 434 行）、frontend/DEVELOPMENT.md:191 均以该链路为示例，删码后需同步更新，否则误导后来者；③ 若仍想保留 foundation 作为可复用脚手架的「后端推事件示范」教学价值，删除后需在文档里另指一条真实事件链（如 terminal:status）作范例。另 useTrayNavigation.ts:6 的「与 useTimeEvent 同款」注释需顺手改一句（仅注释，不会坏）。

## #48 【过度抽象·medium】httpx 核心（单例 Client / JSON helper / 整个 retry.go）零调用方，实际只有 proxyclient 在用

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/utils/httpx/httpx.go、J:/Program/Agent/XK-Pane/internal/utils/httpx/retry.go、J:/Program/Agent/XK-Pane/internal/utils/httpx/proxyclient.go、J:/Program/Agent/XK-Pane/internal/app/app.go
- **估计规模**：99 至 333 行（视保留策略）

**证据**：全仓 grep 'httpx\.' 仅三处真实调用：app.go:68 SetUserAgent、bedrockdocsync/proxy.go:53 NewClient（proxyclient.go，与 httpx.go 核心完全独立）。httpx.go（234 行）的 Client/NewRequest/Do/GetJSON/PostJSON/ReadJSONLimit/HTTPError/IsHTTPError 与 retry.go（99 行）的 Retry/RetryConfig/DefaultShouldRetry 全部零调用方；RetryConfig.ShouldRetry 字段自然也从未被设置。app.go 那行 SetUserAgent 设置的 UA 只作用于无人使用的单例 Client，本身也是空操作（bedrockdocsync 探测用自己的 probeUserAgent，proxy.go:33）。注意：项目 CLAUDE.md 工具层铁律点名了 httpx.GetJSON/PostJSON/Do，属脚手架预置基建，删除需同步改文档。

**建议**：至少删除 retry.go（99 行，纯投机）；进一步可把 httpx.go 精简为仅保留 proxyclient.go 所需部分并删掉 app.go:68 的空操作 SetUserAgent，同时更新 CLAUDE.md / foundation-utils skill 中的 httpx 铁律描述（或保留 httpx.go 作为待用基建，但 retry.go 无保留理由）。

**对抗核实裁决（成立）**：反驳失败，发现属实。穷尽全部反查路径后确认：① 全仓 *.go 中 `httpx\.[A-Z]` 仅两处真实调用——internal/app/app.go:68 的 SetUserAgent 与 internal/services/bedrockdocsync/proxy.go:53 的 NewClient；httpx.go 的 Client/NewRequest/Do/GetJSON/PostJSON/ReadJSONLimit/HTTPError/IsHTTPError 与 retry.go 的 Retry/RetryConfig/DefaultShouldRetry 外部零调用方（IsHTTPError 唯一的包外引用在同样无人调用的 retry.go:90 内部）。② proxyclient.go 的 NewClient 自建 Transport，不引用 httpx.go 的 currentUA/Client()/sharedClient 任何符号，二者确实完全独立；app.go:68 设置的 UA 只被 NewRequest（httpx.go:108-110）消费而 NewRequest 零调用方，bedrockdocsync 用自己的 probeUserAgent（proxy.go:33,73）——SetUserAgent 空操作坐实。③ 更强的实证：唯一真实 HTTP 消费方 bedrockdocsync 需要重试时自己实现了 httpRetryMax/httpRetryBackoff/fetchText（http_sync.go:183-205），需要 JSON GET 时自己写了 giteaGetJSON（gitea.go:205），即真实需求出现时 httpx 核心与 Retry 均未被选用（per-client 代理与单例模型不适配）。④ bindings 反查：httpx 是 internal/utils 包非注册 Wails service，不生成 bindings，frontend/src grep "httpx" 零命中；webapi/事件字符串/hooks/CLI 适配器/子进程白名单/i18n/路由 path/Taskfile 均无引用，cmd/xkpane-cli-hook/main.go:14 注释还明确刻意绕开 httpx。⑤ 测试：包内仅 proxyclient_test.go，不覆盖 retry.go。⑥ 非新功能：git log 显示 retry.go/httpx.go 最后改动停留在 b7281a3（Phase 0 rebrand，脚手架时代遗留），proxyclient.go 为 P6f-7 OTA 所加且在用，均不属于最近 5 个 worktree/composer 提交。建议中「至少删 retry.go（99 行纯投机）」无任何反证；精简 httpx.go 与删 app.go:68 空操作亦站得住脚。

**执行风险**：① 文档契约：CLAUDE.md:156、README.md:285/410-414、.claude/skills/foundation-utils/SKILL.md 的工具层铁律点名 httpx.GetJSON/PostJSON/Do/Retry，删 httpx.go 核心或 retry.go 必须同步改这三处，否则未来 AI 按铁律写新 HTTP 代码会直接编译失败或重新发明轮子。② 依赖顺序：retry.go 的 DefaultShouldRetry 调用 IsHTTPError（httpx.go），若要精简 httpx.go 必须连 retry.go 一起删（先删 retry.go 单独可行，反向不行）；httpx.go 还持有 package 级文档注释，整删需把包注释挪到 proxyclient.go。③ 未来回补成本：项目源自「foundation 脚手架」，httpx 核心是预置基建，P6f_PLAN 曾承诺「保留单例不破坏现有调用方」（实际调用方为零）；若日后出现走默认直连的 HTTP 需求需重新引入 GetJSON/PostJSON——这是保留 httpx.go 的唯一论据，对 retry.go 不适用（真实重试需求已被 http_sync.go 自实现证伪）。

## #49 【过度抽象·medium】零散投机选项字段：procx.Spec.Stdin 与 logx.Config 多数字段从未被设置非默认值

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/utils/procx/procx.go、J:/Program/Agent/XK-Pane/internal/utils/logx/logx.go、J:/Program/Agent/XK-Pane/internal/app/app.go
- **估计规模**：约 20-25 行 + 缩小两处公共 API 面

**证据**：procx.Spec.Stdin（procx.go:40，接线在 106-107 行）：全仓 grep 'Stdin:'（排除 procx 自身）零匹配——25 个 procx.Run/StartCtx 调用点没有一个传过 Stdin。logx.Config 7 个字段中 Dir/Filename/MaxSizeBytes/Backups/AddSource 从未被设置：logx.Init 全仓唯一调用点 app.go:61-63 只设 Level 与 ConsoleEnabled，其余全部走包内默认值。这些是「可配置性」预留，与 CLAUDE.md「No 'flexibility' that wasn't requested」准则相悖。

**建议**：删 procx.Spec 的 Stdin 字段及其接线（需要时再加回）；logx.Config 可收缩为 {Level, ConsoleEnabled} 两字段、其余降级为包内常量。属低优先级清理，可与其它 utils 改动顺手做。

**对抗核实裁决（成立）**：反驳失败，发现属实。① procx.Spec.Stdin：当前分支全仓 .go 文件 grep「Stdin」，除 procx.go 自身（40 行定义、106-107 行接线）外仅剩 cmd/xkpane-cli-hook 与 build/ios 脚本里的 os.Stdin（直接用原生 exec.Cmd，与 procx.Spec 无关）；全部约 20 个 procx.Run/StartCtx 调用点（gitx/cliadapters/file/worktreesvc/mcpe/subprocess/sshmachine 及测试）无一设置 Stdin，也无变量式赋值。再用 git grep 横扫其余四个本地分支（master、feat/ssh-over-mcp、feat/daemon-true-three-ends、wasteful-baboon）同样零使用——连记忆中「ssh 密码自动应答」也明确走 /dev/tty 拦截而非 stdin。动态引用面全部排除：procx 不是注册的 Wails service，前端 bindings 触达不到（frontend/src 中 stdin 仅为 PTY 按键注释）；Spec 无 json tag，不可能经 webapi/hooks/子进程白名单等字符串路径引用。② logx.Config：全仓「logx.Init|logx.Config」仅两处——唯一真实调用点 internal/app/app.go:61-63 只设 Level 与 ConsoleEnabled，另一处是 .claude/skills/foundation-utils/SKILL.md:213 的文档示例且同样只示范这两个字段；Dir/Filename/MaxSizeBytes/Backups/AddSource 确实从未被设置非默认值。③ 最近 worktree/composer 新功能用 procx 但不涉及这些字段，该发现并未把新功能误判为死代码。两项均为仓库内零使用者的投机配置面，删除/收缩不会破坏任何编译与运行路径，与 CLAUDE.md「无未被要求的灵活性」准则一致。

**执行风险**：三个低风险点：① .claude/skills/foundation-utils/SKILL.md:264 把 Config.MaxSizeBytes / Backups 文档化为「要改阈值传 …」的可调项，收缩 logx.Config 字段时必须同步修订该 SKILL 文档，否则文档与代码脱节；② 本项目自我定位为可复用脚手架（foundation），utils 属面向下游的工具层 API 表面，若将来把脚手架抽出给新项目用会少这几个开箱配置项——但加回成本极低（字段纯增量、logx.Init 为单点幂等初始化，无兼容包袱）；③ procx.go 同一文件在 feat/ssh-over-mcp / feat/daemon-true-three-ends 等未合并分支上也存在，删字段后合并这些分支时可能出现轻微文本冲突（均未使用该字段，属易解的常规冲突）。

## #50 【过度抽象·medium】transcriptsvc 的两个单实现接口是纯仪式：包内已 import 具体实现类型且无测试 fake

- **涉及文件**：J:/Program/Agent/XK-Pane/internal/services/transcriptsvc/transcriptsvc.go
- **估计规模**：约 10 行

**证据**：transcriptsvc.go:23/28 定义 SessionLister / SessionScanner，各自仅有一个实现（terminal.Service / sessionscan.Service，注释自述）。该包没有任何 _test.go（目录仅 transcriptsvc.go 一个文件），不存在 fake 注入需求；且包已经直接 import xkpane/internal/services/terminal 与 sessionscan 取 SessionSnapshot / ResumeSession 类型（13-19 行），改持具体指针不会增加任何耦合。对比之下，webapi/rest.go、reminder、lhwatcher、launchhistory、notification、orchestrator 的同类窄接口都有对应测试 fake（rest_test.go、fakeSender 等），均不属此问题——本仓库接口滥用面整体很干净，仅此一处例外。

**建议**：把 Service 字段改为 *terminal.Service 与 *sessionscan.Service，删除两个接口定义；或为其补测试以让抽象自洽。前者更省。

**对抗核实裁决（成立）**：反查全部落空，发现成立。证据：① 全仓 grep 显示 transcriptsvc.SessionLister / SessionScanner 仅在包内自用；internal/webapi/rest.go:44-59 的同名接口是 webapi 自己独立定义（配 rest_test.go fake），与本包互不引用。② 唯一构造点 internal/app/app.go:120 `transcriptsvc.New(termSvc, scanSvc)` 注入的就是具体类型，且 terminal.go:471 SessionSnapshots 与 sessionscan.go:43 ListProjectSessions 签名匹配，改持 *terminal.Service / *sessionscan.Service 可直接编译。③ 该包目录仅 transcriptsvc.go 一个文件、无任何 _test.go，全仓也没有别处构造该 Service，不存在 fake 注入需求。④ 两接口只是 Service 的未导出字段类型，不出现在 GetTimeline 签名中，wails bindings 不为其生成任何东西，前端 TranscriptService.ts 只调 GetTimeline，替换为具体指针对 bindings/前端零影响。⑤ 包在 13-19 行本就直接 import terminal 与 sessionscan，改具体指针不增加耦合、无导入环。⑥ 该文件属 timeline 卡片功能（commit 048a658 / 49ef78e），不在 worktree/composer 最近五连提交之列，且建议只动内部抽象不删功能。

**执行风险**：风险极低：纯包内重构，不改 GetTimeline 签名、不触 bindings、不触 webapi。唯一取舍：仓库测试策略本就建议给 internal/services 补 go test，若日后要为 GetTimeline 写单测，*terminal.Service 这种带 PTY 的重型依赖难以在测试中实例化，到时需把接口加回来——属可逆的小返工，不构成现在保留的理由。

## #51 【构建与资产·high】build/ios 与 build/android 两套移动端原生构建链是纯脚手架拷贝，从未使用，可整目录删除

- **涉及文件**：J:/Program/Agent/XK-Pane/build/ios/、J:/Program/Agent/XK-Pane/build/android/、J:/Program/Agent/XK-Pane/Taskfile.yml、J:/Program/Agent/XK-Pane/build/Taskfile.yml
- **估计规模**：约 45 个文件、3200+ 行（含 1 个提交进仓库的 gradle-wrapper.jar 二进制）

**证据**：git log 显示 build/android 全部 31 个文件仅出现在 Initial commit（97751b6）；build/ios 仅 Initial commit + 32cf794（那是一次 `wails3 update build-assets` 自动 rebrand，非人工开发）。包名仍是模板默认值：build/ios/Taskfile.yml 的 BUNDLE_ID 默认 `com.wails.app`，build/android/app/src/main/java 路径仍是 `com/wails/app/`，从未改成 xkpane。仓库无 .github/CI。产品的移动端策略是手机浏览器访问 internal/webapi Web 镜像（无任何原生移动代码），与这两套 Gradle/Xcode 工程完全无关。android 目录还提交了 gradle-wrapper.jar 二进制。连带：根 Taskfile.yml 第 14-15 行 `ios:`/`android:` 两个 include，以及 build/Taskfile.yml 第 311-369 行 `ios:device:list`/`ios:run:device` 两个 task（约 60 行）均无人调用。

**建议**：删除 build/ios（14 文件）与 build/android（31 文件）整目录；同步删掉根 Taskfile.yml 的 ios/android 两个 include、build/Taskfile.yml 末尾的 ios:device:list 与 ios:run:device 两个 task；build/CLAUDE.md 中对应表格行一并清理。

**对抗核实裁决（成立）**：对抗性反查全部失败，建议成立且证据比原报告更充分。逐项核实：① git 历史属实——build/android 全部 31 个文件仅存在于 Initial commit（97751b6）；build/ios（14 文件）仅多一次 32cf794，该 commit message 自述是「common:update:build-assets 从 config.yml 重生成全平台资源」的自动 rebrand，非人工开发。② 模板默认值属实——build/ios/Taskfile.yml 第 7 行 BUNDLE_ID 默认 "com.wails.app"，android java 包路径仍是 com/wails/app/，从未业务化为 xkpane。③ 引用面反查全空：全仓 grep ios/android（排除 node_modules/dist/bindings）命中的 19 个文件中，internal/（含 webapi）、frontend/src、事件字符串、hooks/cliadapters/subprocess 白名单、i18n、路由 path、React.lazy 全部零引用；唯一代码侧命中 frontend/pnpm-lock.yaml 是 npm 包平台元数据，.claude/skills 命中是通用 wails 教学文档。④ Taskfile 链路确认孤立：根 Taskfile 的 task build/package/run 走 {{OS}}:build，桌面只解析为 windows/darwin/linux；build/windows、darwin、linux、docker 四个子 Taskfile 对 ios/android 零引用；build/Taskfile.yml 311-369 行 ios:device:list/ios:run:device 两个 task 确无调用方。⑤ 无 .github/CI。⑥ 不属于 worktree/composer 新功能（最近 5 个 commit 未触碰这两个目录）。⑦ 反查反而发现加分证据：go list ./... 显示 xkpane/build/ios 等是 Go 模块内的包，progress.md 至少 5 处（371/883/1291/1391/1404 行）记载 Windows 上 go build ./... 因 build/ios stub 报「main is undeclared」、团队长期被迫绕用 ./internal/...——删除这两个目录还能消除长期构建噪声。产品移动端策略已定为手机浏览器访问 internal/webapi Web 镜像，与原生 Xcode/Gradle 工程无关。

**执行风险**：三个低风险点需在执行时注意：① 删目录与删根 Taskfile.yml 第 14-15 行 include 必须同一次提交完成——go-task 的 includes 指向缺失文件会让所有 task 命令（含 task dev）直接解析失败，建议中已包含此步骤但顺序是硬绑定；② 实测（32cf794 的 diff 范围）`wails3 task common:update:build-assets` 会重写 build/ios 下的 plist/pbxproj 等资源，删除后若再跑该命令，wails3 CLI 可能把部分 ios 模板文件再生成回来（回流无害，再删一次或接受即可），建议删后跑一次该命令验证；③ 根 CLAUDE.md 的模块结构图与「build/ 一句话职责」也写有 ios/android，宜与 build/CLAUDE.md 表格行、FAQ、文件清单一并清理（纯文档同步，漏改不影响构建）。未来若真要做原生移动端，可随时从 git 历史或 wails3 官方模板找回，无不可逆损失。

## #52 【构建与资产·high】frontend:vendor:puppertino 是 wails 模板残留死任务，引用的文件与锚点全部不存在

- **涉及文件**：J:/Program/Agent/XK-Pane/build/Taskfile.yml
- **估计规模**：约 35 行

**证据**：build/Taskfile.yml 第 143-176 行定义 frontend:vendor:puppertino，全仓 grep `vendor:puppertino` 只命中定义本身，零调用方。其逻辑三处全部失效：frontend/public/puppertino/ 目录不存在；awk 注入锚点 `href="/style.css"` 在 frontend/index.html 中不存在（grep 零命中）；sed 替换的 `class="btn"` 也是 wails 默认模板的类名，本项目前端是 MUI。这是从 wails3 模板带过来的移动端样式补丁任务，对本项目纯属噪音。

**建议**：整段删除 build/Taskfile.yml 第 143-176 行的 frontend:vendor:puppertino task。

**对抗核实裁决（成立）**：反驳失败，建议成立。证据：① 调用方为零——无视 .gitignore 全仓 grep `puppertino`（含全部平台子 Taskfile、shell 脚本、package.json）仅命中 build/Taskfile.yml 第 143-176 行定义本身；根 Taskfile 虽以 `common:` 命名空间 include 了 build/Taskfile.yml，但全仓无任何 `common:frontend:vendor:puppertino` 的 task 调用（对比同文件其他任务如 common:build:hook / common:build:server 均有明确调用方）。② 逻辑三处全部失效实锤——frontend/public/puppertino/ 目录不存在（public 下仅 fonts / Inter-Medium.ttf / react.svg / wails.png）；frontend/index.html 全文 259 行无 awk 注入锚点 `href="/style.css"`，也无 sed 替换目标 `class="btn"`；frontend/src 零引用 `/puppertino/puppertino.css`，即使手动执行也只是下载一个无人加载的 CSS。③ 与最近 5 个 worktree/composer commit 无关，是 wails3 模板带来的移动端样式补丁残留，本项目前端为 MUI 9 自绘体系，永不走该路径。`vendor` 一词唯一其他命中是 build/linux/nfpm/nfpm.yaml:14 的包元数据字段，与此任务无关。

**执行风险**：无。该任务无任何调用方且执行逻辑全部为 no-op，删除不影响 dev / build / package / 移动端（android/ios 子 Taskfile 均不引用它）。唯一理论场景是未来若想给移动端引入 Puppertino CSS 需重新编写，但本项目 MUI 体系下不存在此需求。

## #53 【构建与资产·high】MSIX 打包链路已损坏（引用不存在的 wails.json）且从未使用，可删

- **涉及文件**：J:/Program/Agent/XK-Pane/build/windows/Taskfile.yml、J:/Program/Agent/XK-Pane/build/windows/msix/app_manifest.xml、J:/Program/Agent/XK-Pane/build/windows/msix/template.xml
- **估计规模**：2 个文件 + 约 35 行 task 定义

**证据**：build/windows/Taskfile.yml 的 create:msix:package 执行 `wails3 tool msix --config "{{.ROOT_DIR}}/wails.json"`，而仓库根本没有 wails.json（ls 确认不存在）——该任务一跑必挂，说明从未被执行过。package task 默认 FORMAT=nsis，实际打包走 NSIS（NSIS 路径有真实开发痕迹：e6cfe4e 把 cli-hook 打进安装目录、32cf794 修 NSIS 中文编码）；build/windows/msix/ 两个文件 git log 仅 Initial commit。

**建议**：删除 build/windows/msix/ 目录与 build/windows/Taskfile.yml 中的 create:msix:package、install:msix:tools 两个 task，package task 收敛为只走 NSIS。

**对抗核实裁决（成立）**：反驳失败，建议成立。实证：① 仓库根确无 wails.json（Glob 确认），而 build/windows/Taskfile.yml:149-150 的 create:msix:package 执行 `wails3 tool msix --config "{{.ROOT_DIR}}/wails.json"`，引用的是不存在的文件（wails v3 实际配置在 build/config.yml），该任务一跑必挂；② package 任务默认 FORMAT=nsis（106/110 行），根 Taskfile 仅做 `package → {{OS}}:package` 转发且全文无 FORMAT/msix 字样，即 MSIX 分支从不会被默认命中；③ `git log --all -S 'msix' -i` 全分支搜索仅命中 Initial commit（97751b6），build/windows/msix/ 两文件与 Taskfile 的 msix 段自脚手架初始化以来零改动，而 NSIS 路径有真实开发痕迹（Taskfile.yml:47 注释 cli-hook 经 NSIS 打进安装目录）；④ 无 .github/workflows，无 CI 引用；⑤ 全仓 grep msix，除 windows 子 Taskfile 自身外仅命中文档（build/CLAUDE.md 文件清单、.claude/skills/wails-build 与 wails-advanced 的 wails 上游文档拷贝），均非运行时引用；bindings/webapi/事件/hooks/子进程白名单/i18n/路由/React.lazy 与打包链路无交集。该链路是 wails3 init 模板自带遗留，本项目（含 worktree/composer 新功能在内）从未定制、从未执行，删除 build/windows/msix/ 目录与 create:msix:package、install:msix:tools 两个 task 不影响任何现用功能。

**执行风险**：三个非阻断风险：① build/Taskfile.yml:236-240 的 update:build-assets 调 `wails3 update build-assets`，按 build/CLAUDE.md FAQ 它会重新生成「plist、manifest、msix 等」平台资源——删除后下次跑 `wails3 task common:update:build-assets` 时 wails3 工具可能把 msix 模板文件再生成回来（无害但删除不持久，可顺手核实该命令行为）；② 需同步更新 build/CLAUDE.md 的文件清单与 FAQ（仍列有 msix/{app_manifest.xml, template.xml}），且 .claude/skills/wails-advanced/references/custom-protocols.md:195 明确引用 build/windows/msix/app_manifest.xml 路径，删除后该文档描述与仓库脱节（skill 是 wails 上游文档拷贝，可不改但需知情）；③ 若将来需要 Microsoft Store 分发或 MSIX 形式的自定义 URL 协议注册，需从 wails3 模板重新引入该链路（修复 --config 指向 build/config.yml 即可恢复）。

## #54 【构建与资产·high】frontend/public 三个未引用静态资产随每次构建打进 exe：Inter-Medium.ttf（315KB）从未被 @font-face 加载、react.svg 零引用

- **涉及文件**：J:/Program/Agent/XK-Pane/frontend/public/Inter-Medium.ttf、J:/Program/Agent/XK-Pane/frontend/Inter Font License.txt、J:/Program/Agent/XK-Pane/frontend/public/react.svg
- **估计规模**：3 个文件、约 324KB 二进制体积（同时减小 embed 后的 exe）

**证据**：全前端 grep `Inter-Medium` 零命中：index.html 与 buildMuiTheme.ts 的 FONT_STACK 虽写了 font-family "Inter"，但全仓没有任何 @font-face 声明加载该 ttf，Windows 上实际一直回落 Segoe UI，这个 315KB 的字体文件纯属占位（对照组：JetBrainsMono-Regular.woff2 在 index.html 第 14-19 行有完整 @font-face + preload，是真在用的）。react.svg 在 *.ts/*.tsx/*.html/*.css 中 grep 零命中（Vite 模板遗留）。public/ 下文件会被 Vite 原样拷入 dist 并经 go:embed 打进二进制。注意 wails.png 被 index.html 第 5 行用作 favicon，不可删。

**建议**：删除 frontend/public/Inter-Medium.ttf、frontend/public/react.svg，以及陪葬的 frontend/Inter Font License.txt；若未来真想用 Inter 再补 @font-face（当前删除无任何视觉影响）。

**对抗核实裁决（成立）**：反驳失败，建议成立。逐项反查均无引用：①全仓 grep `Inter-Medium` 与 `react.svg` 仅命中 frontend/CLAUDE.md:398-399 的目录树文档行，*.ts/*.tsx/*.html/*.css 零命中；②全前端唯一 @font-face 在 frontend/index.html:14-19，加载的是 /fonts/JetBrainsMono-Regular.woff2；useTerminalView.ts:335 的 document.fonts.load 也只拉 "JetBrains Mono"，无 new FontFace、无 CSS url() 指向该 ttf——index.html:63 / child.html:12 / buildMuiTheme.ts 虽把 font-family 写成 "Inter"，但浏览器不会自动从 public/ 找字体文件，Windows 实际一直回落 Segoe UI，该 ttf 从未被加载；③internal/webapi（Web 镜像端）无任何引用；④Taskfile/build 无命中（build/android/WailsPathHandler.java:96 的 .ttf 仅是通用 MIME 映射，不针对具体文件）；⑤事件字符串/i18n/路由/子进程白名单等动态引用面均不适用静态资产，且查无；⑥git log --diff-filter=A 确认两文件均来自 Initial commit（97751b6，Vite/Wails 模板遗留），与最近 worktree/composer 分支（最近 5 commit）无关——public/ 最近一次变更 ecad17b 只新增 fonts/ 子目录（JetBrains Mono，正是建议中真在用的对照组）；⑦wails.png 被 index.html:5 用作 favicon，建议已正确排除，未误伤。实测文件大小：Inter-Medium.ttf 315132 字节、react.svg 4126 字节，确随 Vite 拷入 dist 并经 go:embed 打进二进制。

**执行风险**：基本无风险，两点提示：①小偏差——Inter Font License.txt 位于 frontend/ 根目录而非 public/，它不进 dist 也不打进 exe，删除它属仓库清理而非二进制瘦身（但与字体成对删除是对的，删字体留 License 反成孤儿）；②删除后 frontend/CLAUDE.md:398-399 的目录树会过时，应顺手更新；若未来真要启用 Inter（FONT_STACK 首位仍写着 "Inter"），需重新引入字体文件 + @font-face + License。视觉上零影响（该 ttf 从未被任何渲染路径加载）。

## #55 【构建与资产·high】根目录 17 个一次性计划/交接/进度文档（P3~P7 系列 + planning-with-files 三件套）已全部落地，可删除或归档

- **涉及文件**：J:/Program/Agent/XK-Pane/LOOP.md、J:/Program/Agent/XK-Pane/P3_PLAN.md、J:/Program/Agent/XK-Pane/P3_HANDOFF.md、J:/Program/Agent/XK-Pane/P4_PLAN.md、J:/Program/Agent/XK-Pane/P4_HANDOFF.md、J:/Program/Agent/XK-Pane/P6_PLAN.md、J:/Program/Agent/XK-Pane/P6D_PLAN.md、J:/Program/Agent/XK-Pane/P6E_HANDOFF.md、J:/Program/Agent/XK-Pane/P6F_HANDOFF.md、J:/Program/Agent/XK-Pane/P6F_OTA_HANDOFF.md、J:/Program/Agent/XK-Pane/P6e_PLAN.md、J:/Program/Agent/XK-Pane/P6f_PLAN.md、J:/Program/Agent/XK-Pane/P7_WEB_PLAN.md、J:/Program/Agent/XK-Pane/REBUILD_BLUEPRINT.md、J:/Program/Agent/XK-Pane/findings.md、J:/Program/Agent/XK-Pane/progress.md、J:/Program/Agent/XK-Pane/task_plan.md
- **估计规模**：17 个文件、4181 行、约 600KB

**证据**：17 个文件均被 git 跟踪（git ls-files 确认），合计 4181 行、约 600KB，其中 progress.md 一个文件就 282KB。全仓交叉引用 grep（P3_PLAN/P6F_OTA_HANDOFF/REBUILD_BLUEPRINT/LOOP.md/P7_WEB_PLAN）只命中 LOOP.md/progress.md/task_plan.md 三件套互相引用，无任何代码、skill、CLAUDE.md 引用。这些是 P3~P7 阶段的规划/交接/进度产物，对应功能均已实现并提交（如 P6d 的 cli-hook 见 e6cfe4e，P7 Web 镜像即 internal/webapi）。历史内容 git log 永久可查，留在根目录只会污染仓库首页与全文检索。另：build/bin/mcp-spike.exe 是被 .gitignore `bin` 规则忽略的一次性 spike 调试二进制（未跟踪），属磁盘遗留可顺手清掉。

**建议**：直接 git rm 这 17 个文档（需要回溯时走 git 历史）；若想保留可读入口，最多移入 docs/archive/ 一个子目录。RECLAUDE_COMPAT.md 已被 .gitignore 排除、未跟踪，不在此列。

**对抗核实裁决（成立）**：建议总体成立：17 个文件确均被 git 跟踪（git ls-files 核实），合计 4181 行，progress.md 单文件 282KB。运行时零依赖——逐项反查 Taskfile.yml、build/、main.go（embed 只打 frontend/dist，md 不进二进制）、frontend/src、internal/webapi、.claude/skills、CLAUDE.md、README.md 均无引用；互相链接只在这 17 个文件内部闭环，整组移走不产生断链（.claude/skills/foundation-mui-feedback/SKILL.md 命中的 progress.md 是该 skill 自身的 references/progress.md，与根目录无关）。「对应功能已落地」声称属实：P6e 即 internal/services/mcpe + mcdkbridge；P6f 含 OTA 全部单元 9b/9c/10 均有代码与提交（internal/services/bedrockdocsync/ 全套 + frontend/src/pages/SettingsPage/knowledge/ 面板）；P7 即 internal/webapi。时效上三件套最后提交停在 2026-06-03（27d1795），最近 5 个 worktree/composer commit 完全不更新它们，工作跟踪已迁移至用户 memory 文件；这 17 个文件本身不属于 worktree/composer 新功能。mcp-spike.exe 确认未跟踪、被 .gitignore 第 2 行 bin 规则忽略，清掉无碍。但建议证据有一处不准确：「无任何代码引用」是错的——存在 4 处 Go 注释引用 P6e_PLAN.md 章节（mcpe/launcher.go:112、mcdkbridge/window_windows.go:7、mcdkbridge/capture_windows.go:12、orchestrator/orchestrator_mcpe_tools.go:12），故执行方式应取其备选项「移入 docs/archive/」而非直接 git rm。

**执行风险**：三处实际代价，决定了应「归档」而非「直接删除」：① 4 处 Go 代码注释以「见 P6e_PLAN.md §N」形式引用拍板出处，直接 git rm 后注释悬空（不影响编译，但丢失决策导航）；② 用户跨会话 memory 显式指向两个文件——xkpane-web-mirror-plan.md 标注「完整方案已落盘 P7_WEB_PLAN.md」（Web 镜像线 S3/S4 仍在推进、M4–M7 里程碑未全落地，该计划仍可能被未来会话当方案入口读取），xkpane-todo-feature.md 指向「progress.md 2026-06-02 条」（todo 功能仍待 live 手测）——删除会令 memory 指针失效；③ task_plan.md 残留 24 个未勾选项，planning-with-files skill 的 /clear 会话恢复机制会扫根目录三件套（弱风险）。归档到 docs/archive/ 并保留 P7_WEB_PLAN.md 与 progress.md 的可达路径即可全部化解。

## #56 【查漏补充·medium】.claude/skills/xkpane-* 是 app 注入产物却被提交入库：与 embedded 模板字节级重复，每次改模板需双处提交

- **涉及文件**：J:/Program/Agent/XK-Pane/.claude/skills/.xkpane-skills-version、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-browse-sessions/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-dispatch-todos/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-fork-session/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-launch-task/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-mc-pe-debug/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-parallel-run/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-plantocodex/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-plantocodex-exec/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-plantocodex-spec/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/xkpane-spec-review/SKILL.md、J:/Program/Agent/XK-Pane/internal/services/projecthooks/skills.go、J:/Program/Agent/XK-Pane/.gitignore
- **估计规模**：11 个 tracked 文件 / 约 81KB，并消除每次模板更新的重复提交

**证据**：internal/services/projecthooks/skills.go 的 syncProjectSkills 会在每次会话启动时把 //go:embed 的 templates/skills/<name>/SKILL.md 自动写入被打开项目的 .claude/skills/ 并落 .xkpane-skills-version 版本标记（当前版本 "2"，与已提交的标记文件内容一致）。逐一 diff 证实 10 个已提交的 .claude/skills/xkpane-*/SKILL.md 与 internal/services/projecthooks/templates/skills/ 下同名模板全部 IDENTICAL（字节级相同，共 81485 字节 / 11 个 tracked 文件）。git log 显示双源维护成本已经发生：commit 8d213e9「docs(skills): 内置打包 skill 去除 WSL 相关内容」不得不同时改模板和 .claude/skills 副本两处。对照组：.gitignore L17-18 把 codex 侧同类注入产物 /.codex/ 整目录忽略并注释「全为运行时产物」，Claude 侧却入了库，处理不一致。注：入库是有意为之（commit 180b072「chore(skills): 入库注入产物 xkpane-* 项目级 skill，clone 即用」），但「clone 即用」收益与 app 自动重建机制重叠。

**建议**：git rm --cached 这 11 个文件并在 .gitignore 追加 /.claude/skills/xkpane-*/ 与 /.claude/skills/.xkpane-skills-version（对齐 /.codex/ 的处理）。功能零损失：用 XK-Pane 打开本仓库启动任一会话时 syncClaudeSkills 会自动重建全部文件；纯 clone 场景跑一次 app 即恢复。这样模板更新 / projectSkillsVersion bump 后仓库不会再自动变脏，也不再需要双处提交。

**对抗核实裁决（成立）**：建议的全部事实主张经反查成立：① 字节级重复实证——cmp 逐一比对 internal/services/projecthooks/templates/skills/ 下恰好 10 个模板与 .claude/skills/xkpane-*/SKILL.md，10/10 IDENTICAL；.xkpane-skills-version 内容 "2" 与 skills.go:28 的 projectSkillsVersion="2" 一致，git ls-files 确认 11 个文件全部被跟踪。② 自动重建链路实证——projecthooks.go:61-87 的 Sync 在每次 claude/reclaude 会话启动（触发点 terminal.CreateSession，progress.md:1475）调 syncClaudeSkills，skills.go:48-79 在版本标记缺失/不符时整体重写并先于 CLI 进程拉起，故「纯 clone 跑一次 app 即恢复」成立。③ 双处提交成本已实际发生——git show 8d213e9 确实同时改模板与 .claude 副本共 8 个文件（4 skill × 2 处）。④ 与 .codex/ 处理不一致实证——.gitignore:18 把同一批 skill 的 codex 侧副本整目录忽略并注释「全为运行时产物」。⑤ 反驳各引用面均失败：这 11 个文件是 markdown 文档非 service 代码，bindings/webapi/事件/Taskfile 均无关；全仓唯一引用该路径字符串的代码 worktreesvc.go:165-166/224-228（最近 worktree 新功能）对「被跟踪」与「已 ignore」两种状态都有独立分支（tracked→尊重现状跳过；untrack 后 probe 命中→跳过），删除后行为等价不破坏，其测试用临时仓库自建场景亦不受影响；副本入库于 180b072（6月3日），不属最近 5 个 worktree/composer 新功能 commit。唯一存留的反对点是 180b072 的有意取舍（见风险），不足以否决。

**执行风险**：三点：① 推翻 180b072 的有意决策——纯 clone 后直接用 claude CLI（不经 XK-Pane 启动会话）的场景下这 10 个项目级 skill 缺失，需仓库所有者确认放弃该收益；缓解因素是这批 skill 全依赖仅 XK-Pane 运行时提供的 panes MCP 工具，脱离 app 基本无用武之地。② 新增 gitignore 通配 /.claude/skills/xkpane-*/ 会静默吞掉将来手写且想入库的 xkpane-* 前缀新 skill（git add 不报错地漏收，正是 worktreesvc.go:180-182 注释警示的同类问题），命名新 skill 时需避开该前缀或用 git add -f。③ 瞬态空窗：其他工作副本/worktree pull 到删除提交后这 11 个文件会从工作树消失，直到下次经 XK-Pane 在该项目启动会话才重建。

## #57 【查漏补充·high】根 CLAUDE.md 仍是 foundation 脚手架说明书：模块名、服务清单、改名指南、数据目录全部与现状脱节

- **涉及文件**：J:/Program/Agent/XK-Pane/CLAUDE.md、J:/Program/Agent/XK-Pane/go.mod、J:/Program/Agent/XK-Pane/internal/services
- **估计规模**：约 100~120 行过时文档（含两张过时结构图）

**证据**：CLAUDE.md 含 18 处 foundation 引用且核心事实全部失效：L93 改名指南仍写「改 go.mod 的 module foundation → module <你的名字>」，而 go.mod 第 1 行已是 module xkpane；L26 通信链路示例写 @bindings/foundation/internal/services/greet，而 frontend/src 实际 import 前缀全部是 @bindings/xkpane（grep 确认无任何 @bindings/foundation）；模块结构图 / 模块索引只列 greet / preferences / appsettings / storagesvc / subprocess 共 5 个 service，实际 internal/services/ 下有 33 个域（terminal、orchestrator、bedrock、todo、worktreesvc、mcpe、sshmachine 等核心域全部缺席）；L106 称数据存到 %AppData%\Foundation\foundation.db，实际 internal/appinfo/appinfo.go 的 AppDirName = BaseName + devSuffix（xkpane 品牌，含 dev/release 隔离）；L23-31 通信链路与变更记录大量描述 greet/time 演示链——这两条已被本轮审计确认为全链路死代码待删。前一轮审计只覆盖了 README.md 和 frontend/CLAUDE.md，根 CLAUDE.md 被遗漏，而它是 AI 每次会话注入的最高权重项目文档，过时内容会持续误导。

**建议**：做减法重写：删掉「项目愿景（脚手架定位）」「改名 / 业务化指南」整节（改名早已执行完毕）、greet/time 通信链路示例（随死代码删除）、5 个 service 的过时模块结构图 / 索引、Foundation 数据目录描述、以及 2026-05-28 系列脚手架变更记录；保留并修正仍然成立的铁律部分（持久化 / 工具层 / i18n / Icon 等）。

**对抗核实裁决（成立）**：对抗反查后无法推翻，建议成立。核心证据全部独立验证属实：① go.mod L1 已是 module xkpane，CLAUDE.md L93 改名指南（「改 module foundation → <你的名字>」）描述的是已执行完毕的历史操作；② frontend/src 全量 grep @bindings/foundation 为 0 处，实际 import 前缀全部是 @bindings/xkpane（WorktreeService.ts / TerminalService.ts / TodoService.ts 等），L17/L26 通信链路示例路径已失效；③ internal/services/ 实测 33 个域（terminal、orchestrator、worktreesvc、sshmachine、mcpe、cliadapters 等核心域齐全），CLAUDE.md 模块结构图/索引只列 greet/preferences/appsettings/storagesvc/subprocess 共 5 个；④ internal/appinfo/appinfo.go L16-20 确认 BaseName="XK-Pane"、AppDirName=BaseName+devSuffix（dev 构建为 XK-Pane-Dev），L106 的 %AppData%\Foundation\foundation.db 数据目录描述失效；且 L168 改名提醒所指 internal/storage / cryptox / logx 三处 AppDirName 已全部改为 = appinfo.AppDirName 同源别名（storage/paths.go L20、cryptox.go L43、logx.go L54），「分别改三处」的指引同样过时；⑤ git log -- CLAUDE.md 仅有 97751b6 Initial commit，文档自创建后零更新；⑥ 覆盖全部要求的引用面反查：无任何代码、Taskfile（APP_NAME 已是 xkpane）、webapi、事件名、动态字符串依赖 CLAUDE.md 的文本内容，build/CLAUDE.md 仅有一条「[根目录](../CLAUDE.md)」导航反链，重写零运行时风险。CLAUDE.md 是每次会话注入的最高权重项目文档（本次会话注入的正是这份过时内容），脱节信息会持续误导 AI 按 foundation 前缀写 import、按 5 个 service 理解架构、去 Foundation 目录找数据库，减法价值真实存在。

**执行风险**：三个执行注意点：① 「greet/time 链路示例随死代码删除」有前提依赖——反查发现 greet 当前并非严格死代码：internal/app/app.go L190 仍注册 greet.New()，HomePage 在 routes.tsx L70 活路由上经 frontend/src/services/greet/GreetService.ts 调用它（time 事件确实更接近死：useTimeEvent.ts 无任何消费者，仅后端 app.go L437 空 Emit）；若「删 greet/HomePage 演示链」那条审计最终不执行，CLAUDE.md 的通信链路示例应改写为 xkpane 前缀的真实业务链路而非直接删除。② 铁律段重写时勿过度纠错：theme.palette.foundation.* 是前端仍在真实使用的主题命名空间（6 处引用，未随模块改名），.claude/skills/ 下 foundation-persistence / foundation-utils / foundation-i18n 等 skill 名也仍有效，这些「带 foundation 字样但仍准确」的引用必须原样保留。③ CLAUDE.md L101「完整对照表见 README.md」——README.md 当前也仍是 Foundation 脚手架文档（前一轮审计已单独报过），两份文档的修订需协调，避免重写后互相指向不存在的章节。

## #58 【查漏补充·high】死功能的配套 skill 文档可连带删除：childwindow 与 x-pro/x-data-grid 的 .claude/skills 文档约 110KB

- **涉及文件**：J:/Program/Agent/XK-Pane/.claude/skills/foundation-childwindow/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-data-grid/SKILL.md、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-data-grid/references/pro-features.md、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-data-grid/references/premium-features.md、J:/Program/Agent/XK-Pane/.claude/skills/foundation-mui-x-charts/references/pro-premium-charts.md
- **估计规模**：约 13 个文件 / 约 110KB skill 文档

**证据**：既有审计已确认「子窗口系统（childwindow）前后端全链路零业务调用方」与「components/x-pro 整目录（48 文件 ~3.5k 行）+ @mui/x-data-grid 依赖可整体删除」，但配套 skill 文档未被列入连带清单：foundation-childwindow/SKILL.md 20821 字节专门文档化这个零使用系统；foundation-mui-x-data-grid 整目录 73071 字节 / 11 个文件文档化将被整包移除的 @mui/x-data-grid 及 x-pro 自实现（grep 确认 SKILL.md 3 处、pro-features.md 5 处、premium-features.md 5 处 x-pro 路径引用）；foundation-mui-x-charts/references/pro-premium-charts.md 含 19 处 x-pro 引用，文档化的 Heatmap/Funnel/Radar 等「项目自实现高级图表」正是死掉的 x-pro 图表树。一旦执行既有删除项，这些文档全部指向不存在的代码，留着会引导 AI 重新使用已删 API。

**建议**：随 childwindow 系统删除 foundation-childwindow/ 整目录；随 x-pro + @mui/x-data-grid 删除 foundation-mui-x-data-grid/ 整目录；删 foundation-mui-x-charts/references/pro-premium-charts.md 并从其 SKILL.md 中剔除 x-pro 章节（x-charts 社区版部分仍被数据存储页使用需保留）。同时更新 .claude/skills/INDEX.md 对应表行。

**对抗核实裁决（成立）**：对抗性反查全部失败，建议成立。（1）文件与体积属实：foundation-childwindow/SKILL.md 实测 20821 字节；foundation-mui-x-data-grid/ 实测 73071 字节、11 个文件；foundation-mui-x-charts/references/pro-premium-charts.md 实测 17828 字节，三者合计约 111.7KB，与「约 110KB」一致；文档内 x-pro 路径引用计数与声称完全吻合（x-data-grid SKILL.md 3 处、pro-features.md 5 处、premium-features.md 5 处、pro-premium-charts.md 19 处）。（2）死代码前提独立复核成立：childwindow 后端 internal/app/childwindow.go 虽在 app.go:129 注册为 service，但全仓库（含 internal/webapi、事件名、hooks/cliadapters/subprocess 动态字符串面）引用仅限系统自身闭环——frontend 侧 services/childwindow 包装层只被 child.tsx 与 childwindows/* 三个预设窗口 import，没有任何业务页面/组件调用 ChildWindowService.open*，属典型「自引用基础设施零业务调用方」；x-pro 方面，frontend/src/components/x-pro 目录之外 grep 零命中（含 '@/components/x-pro' 与相对路径），@mui/x-data-grid 仅被 x-pro 内部 16 处 import，package.json 中该依赖随之可删。（3）最近 5 个 worktree/composer 提交（git log --name-only）未触及 childwindow、x-pro 或这些 skill 文档，不属于新功能误报。（4）保留边界正确：SettingsPage/database/Database.tsx 确实使用 @mui/x-charts 社区版 PieChart/BarChart，建议「x-charts SKILL 保留社区版、只剔 x-pro 章节（SKILL.md 第 14、37-54、120 行）+ 删 pro-premium-charts.md」与代码事实吻合。（5）留着这些文档的危害真实存在：foundation-mui-x-charts/SKILL.md 第 54 行示范 import '@/components/x-pro'，foundation-mui SKILL.md 第 170 行导航指向 x-data-grid skill——代码删除后这些文档会引导 AI 重新调用已删 API。

**执行风险**：三点：（1）强前置依赖——本条删除严格以「childwindow 系统 + x-pro 目录 + @mui/x-data-grid 依赖的代码删除真正执行」为前提，若那两项最终保留，skill 文档必须同步保留；（2）建议中「更新 .claude/skills/INDEX.md 对应表行」无的放矢——实测 INDEX.md 只收录 wails-* 与 theme/i18n/persistence/utils 四个 foundation skill，并未收录这三个目标 skill，无行可删；真正需要同步剔除的交叉引用是 .claude/skills/foundation-mui/SKILL.md 第 18 行（依赖表中的 @mui/x-data-grid 行）和第 170 行（指向 foundation-mui-x-data-grid 的导航表行），遗漏会留下死链接；（3）改写 foundation-mui-x-charts/SKILL.md 时务必只删 x-pro 相关段落（第 14、37-54、120 行），社区版图表文档仍被数据存储页（Database.tsx 的 PieChart/BarChart）依赖，误删整个 skill 会丢失在用文档。

## #59 【查漏补充·high】wails-skill-update 维护 skill 已消失但残迹仍在：INDEX.md 死引用 + .gitignore 三行死条目

- **涉及文件**：J:/Program/Agent/XK-Pane/.claude/skills/INDEX.md、J:/Program/Agent/XK-Pane/.gitignore
- **估计规模**：INDEX.md 约 3 行 + .gitignore 3 行；可选项另减约 1.26MB / 约 110 个文档文件

**证据**：INDEX.md 开头写「12 个内容 SKILL + 1 个维护 SKILL」并有引导「同步上游文档 → 用 /wails-skill-update 自动拉取…（脚本在 wails-skill-update/）」及对应表格行，但 ls .claude/skills/ 与 git ls-files 均确认 wails-skill-update/ 目录在工作树和 git 里都不存在（仅 .gitignore L9 残留 /.claude/skills/wails-skill-update/ 忽略规则，说明它曾被刻意排除后又被删除）。另外 .gitignore 还残留 /foundation.exe 与 /foundation 两行——module 已改名 xkpane 且紧随其后的 /*.exe 已覆盖 exe 情形。顺带影响：维护脚本消失意味着 12 个 wails-* skill（合计约 1.26MB，wails-overview 单目录 198KB）失去了与上游 wails3 文档的同步通道，长期会逐渐失真，可一并评估是否继续整包随仓库维护。

**建议**：删除 INDEX.md 中 wails-skill-update 的引导句与表格行；删除 .gitignore 的 /.claude/skills/wails-skill-update/、/foundation.exe、/foundation 三行死条目。若决定不再人工维护 wails 文档镜像，可进一步把 12 个 wails-* skill 目录外置或删除（低优先，属可选项）。

**对抗核实裁决（成立）**：反驳失败，残迹坐实。①目录确不存在：Test-Path .claude/skills/wails-skill-update 为 False，git ls-files/log 均空——该目录因 .gitignore L9 从未入库，仅存在于作者本机且已删除；INDEX.md 的 L7 引导句、L23 表格行、L77-85 整段维护脚本示例（fetch-docs.ps1/diff-docs.ps1/apply-update.ps1）全仓 grep 仅命中 INDEX.md 自身，/wails-skill-update 作为 slash command 也无法解析（当前可用 skill 列表无此项）。②动态引用面反查干净：internal/services/projecthooks/skills.go 的 bundled-skills 同步只管 xkpane-* + .xkpane-skills-version 标记，internal/services/worktreesvc/worktreesvc.go L166/L171 拷贝清单同样不含 wails-skill-update，不会有机制重建它；bindings/webapi/events 与纯文档+gitignore 事项无交集。③两行编译产物条目确死：go.mod L1 已是 module xkpane（go build . 产物为 xkpane.exe/xkpane），Taskfile.yml L4-5 APP_NAME="xkpane"、BIN_DIR="bin"（bin 已被 .gitignore L2 覆盖）；build/windows/msix 模板里的 foundation.exe 只是子目录内打包模板引用，不产出仓库根文件，且根目录 exe 已被 L25 /*.exe 兜底，故 /foundation.exe 冗余、/foundation 无任何生成路径。删除 INDEX.md 三处引用与 .gitignore 三行死条目的核心建议成立。

**执行风险**：核心清理近乎无风险，两点提醒：①若作者本机他处仍留有维护脚本、日后重建 wails-skill-update/ 目录，删掉 .gitignore L9 后该目录（含可能拉取的大体积上游文档缓存）会以未跟踪状态出现并有误提交风险——届时需重新加回忽略行；但 INDEX 引用一并删除后已无任何指引促使重建，风险很低。②建议尾部「外置/删除 12 个 wails-* skill 目录」不属于死代码减法：它们是当前可正常加载的活 skill（本会话 skill 列表在册，开发 wails3 时仍被消费），删除属价值取舍，不应与本次残迹清理捆绑执行（原建议已自标可选，按可选对待即可）。另注：删 /foundation 行不影响现状中「Linux/macOS 根目录 xkpane 裸二进制本就未被忽略」这一既有缺口，该缺口与本减法无关。

---

# 附录 B：7 条驳回发现全文

## R-1 【Go死代码】todo.BatchUpdateStatus 无任何调用方（cc-pane 移植残留）

**驳回理由**：「当前无调用方」这一事实核实为真：frontend/src 零命中（bindings 面无人消费）、internal/webapi 无任何 todo 路由、orchestrator MCP 的 update_todo 走单条 Todos.Update（orchestrator_todo_tools.go:179），hooks/CLI/子进程白名单/i18n/路由 path 等动态字符串面也只命中 todo.go 注释与 progress.md。但把它定性为「cc-pane 移植残留死代码」不成立：(1) progress.md:782 明确记录该方法是刻意的「后端先行」落地——「新方法暂无消费方，与 Query 分层同纪律」，而同纪律的 Todo Query 后端先落地后即被 commit e44d11d 的过滤/搜索 UI 消费，证明这种分层会被后续接上；(2) progress.md:859 已排期「batch-status UI（多选→批量改状态，消费已就位的 BatchUpdateStatus）」并标注为优先候选——存在书面的待消费计划；(3) 用户记忆 xkpane-todo-feature 明令 Todo 功能曾被 dc11388 误删又接回、「别再删」。综上这是在建功能的已就位后端半（待 UI 消费），属于「待落地」而非死代码，删除建议不站得住脚。

## R-2 【Go依赖】github.com/bep/debounce 仅 1 处使用，可用约 15 行 stdlib（time.Timer）替代——但二进制不减，因为 wails 自己也链入它

**驳回理由**：发现的事实层全部核实为真：bep/debounce 在自有代码中确实仅 internal/services/lhwatcher/watcher.go 一处使用（L26 import、L198 debounce.New、L299 每事件调用），且 `go list -deps github.com/wailsapp/wails/v3/pkg/application` 实证输出含 github.com/bep/debounce，即 wails v3 自身链入该库。但作为「减法」它站不住脚，且比发现自述的还要弱：因为 wails 仍 import 该模块，替换后 go mod tidy 也不会删除 go.mod 里这一行，只会把它从 direct 区改标 // indirect——发现声称的唯一收益「direct 区 -1」实际只是注释标注变化，go.mod 行数、go.sum、模块图、二进制全部零减少。而代价是真实的：用约 15 行手写 Timer/Mutex 并发代码替换一个经过实战检验的库，自有代码净增加；且 lhwatcher 是活跃注册服务（internal/app/app.go:122 装配），pipeline.go:48 注明 flush 运行在 debounce 内部 goroutine 上，与 stateMu、分支切换 3s 静默窗口存在并发协作，手写 trailing-edge 语义有引入竞态的回归风险。净效果=代码变多、依赖一个不少、风险非零，违背「最小代码解决问题」原则。发现自己也承认「若以二进制/模块图为准绳，此项收益为零，可不动」——这等于自我否定了减法价值。

## R-3 【Go依赖】golang.org/x/image 仅为 1 行截图缩放引入（可选项，不建议优先做）：替换意味着画质降级或手写缩放器，且该选择有计划文档拍板记录

**驳回理由**：该建议的事实描述准确但不构成真正的减法。反查确认：golang.org/x/image v0.40.0 是 go.mod L18 的直接依赖，全仓唯一代码使用点确为 internal/services/mcdkbridge/capture_windows.go（L21 import xdraw + L133 xdraw.CatmullRom.Scale）。但它不是死代码——CaptureWindowByPID 被 internal/services/orchestrator/orchestrator_mcpe_capture_windows.go:61 调用，注册为 MCP 工具 capture_window（orchestrator_mcpe_tools.go:38），且 projecthooks 模板 skills/xkpane-mc-pe-debug/SKILL.md 把 capture_window 定为 MC PE 调试闭环的核心观察手段（execute_mc_code/reload_mc 均 fire-and-forget，效果全靠截图验证），截图缩到 480p 后供多模态直接读图，画质直接决定可用性。选型有三处拍板记录：capture_windows.go L11-12 注释（Lanczos3→CatmullRom 显式偏离）、P6e_PLAN.md L204、progress.md L1402。stdlib 的 image/draw 只有合成无插值缩放器，删除该依赖只有两条路：手写约 40-60 行双线性缩放（go.mod 减 1 行换 Go 代码净增数十行，违背简化初衷）或退化最近邻（画质再降）。x/image 是官方 golang.org/x 子仓，仓库已直接依赖 x/net / x/sys / x/text，不引入新信任域，维护成本趋近于零。建议本身也结论「维持现状」——一条自己都不建议执行的减法不是可执行的减法项。

## R-4 【Go依赖】核验结论：go.mod 已完全整洁，17 个 direct 依赖零浪费，无任何可直接删除项

**驳回理由**：该条不是一个真正的减法建议，而是「无减法可做」的核验声明，因此不存在可执行的删除/简化标的，按口径判 isReal=false。同时我独立复核确认其声称完全属实：①在 J:/Program/Agent/XK-Pane 实际执行 go mod tidy -diff，退出码 0、零输出，证明 go.mod 的 17 个 direct require 与源码 import 完全一致，无任何可被移除的依赖；②抽查最易漏网的 6 个依赖均有真实 import——html-to-markdown 在 internal/services/bedrockdocsync/http_sync.go（converter + base/commonmark/table 三个 plugin 包）、go-pty 在 internal/utils/ptyx 三个文件、bep/debounce 在 internal/services/lhwatcher/watcher.go、sergi/go-diff 在 internal/localhistory/diff.go、golang.org/x/image 在 internal/services/mcdkbridge/capture_windows.go、pelletier/go-toml 在 internal/services/projecthooks/codex.go；③剩余 11 个 direct 依赖（fsnotify/coder-websocket/glebarez-sqlite/go-git/uuid/mcp-go-sdk/wails/x-net/x-sys/x-text/gorm）批量 grep 共 87 处 import、64 个 Go 文件命中；④u-root/u-root 在 *.go 中零 import，但 go.mod 第 65 行已正确标记 // indirect（go-pty 的模块图依赖带入），不进构建图、go mod tidy 也不会移除，确实无需任何处理。结论：go.mod 干净属实，但「干净、无需动作」本身不构成减法发现。

## R-5 【重复冗余】两套并行项目存储确证脱节：扁平 projects 表已无任何可达写入口，「并集去重」逻辑在前后端重复了 4 份

**驳回理由**：反驳成功，核心前提「写路径断绝、Web 端 fail-closed 不暴露」在仓库全局视角不成立。发现的事实诊断在当前分支（feat/paseo-worktree-composer）逐条核实为真：project.Service.Add/Remove/UpdateName/UpdateAlias 在 Go 侧仅 project_test.go 引用；useProjects() 全仓只有 useHomePage.ts:38 与 useRegisteredProjects.ts:32 两个消费方且都只解构只读 projects；无 project 事件、无直接 GORM 写、orchestrator 工具只读、webapi 当前分支仅 GET /api/projects（ProjectLister.List）；4 处并集去重（useRegisteredProjects.ts:31-59 / app.go:399-429 / orchestrator_tools.go:119-144 / orchestrator_tools.go:276-295）与 projectCount 口径分裂（useHomePage.ts:85 只数扁平表）均读码确证。但决定性反证是：git branch --contains 28f9b76 显示兄弟分支 feat/daemon-true-three-ends 上，commit 28f9b76「P8 S3 数据域端点 — project CRUD 暴露到 LAN 面（首个域）」新增 internal/webapi/data.go，其 ProjectAPI 接口恰好以 Add/Remove/UpdateName/UpdateAlias 四个方法为契约（由 *project.Service 满足并挂 POST /api/projects、/api/projects/remove、/api/projects/update-name、/api/projects/update-alias 四个端点）；配套 commit ed551d6 已给前端 ProjectService.ts 全部写方法接上 !isDesktop Web 分支。即这四个「死写方法」正是进行中的 P8 S3 Web 数据域工作流的承载实现——project 扁平表被刻意选为第一个暴露到 Web 面的数据域，viewer 管理 UI 是该工作流明确的下一步。这属于「刚落地待手测/进行中」的功能（与任务约束第 4 条保护 worktree/composer 新功能同理），不能当死代码删；「以 workspace_projects 为唯一登记表」的合并方向也与 S3 已拍板的范式（后端数据域端点 + 前端 !isDesktop 分支 + viewer 专用组件，首个域即 project）正面冲突。发现中真正站得住的只有「4 处并集逻辑重复」与「projectCount 口径分裂」两个次级观察，但其给出的减法动作（删写方法、废扁平表）会直接拆掉在建工程，故整体裁定不成立。

## R-6 【构建与资产】Docker / 服务器模式整条构建链（Dockerfile×2 + 根 Taskfile 5 个 task + common 5 个 task）从未跑通也无法跑通

**驳回理由**：两条核心证据均被反查推翻。其一，「grep //go:build server 零命中＝无承接」是机制性误判：server tag 由 wails 框架本身消费，已在模块缓存实证 go.mod 锁定版本 wails/v3@v3.0.0-alpha.96 的 pkg/application/application_server.go、events_common_server.go、autostart_server.go、websocket_server.go 均带 server build tag——`go build -tags server` 有完整框架代码承接，这是 wails3 官方模板的实验性 server 模式，「无法跑通」未被证明；且项目并非纯 Windows（internal/services/terminal/shell_unix.go 存在，go.mod 含 aymanbagabas/go-pty 与 creack/pty 的 unix PTY 支持），「distroless Linux 跑不起来」是未经构建验证的断言。其二，「docker 链无任何其他调用方」漏查了 build/darwin/Taskfile.yml:19 与 build/linux/Taskfile.yml:24——在 Windows 主机上 `task darwin:build` 无条件路由到 build:docker，`task linux:build` 同样默认走 build:docker，二者都依赖 CROSS_IMAGE: wails-cross 镜像，该镜像唯一构建来源就是建议要删的 setup:docker + build/docker/Dockerfile.cross（precondition 报错文案明示「Build it first: wails3 task setup:docker」）；README.md 第 266 行还文档化了 `task <os>:build`（windows/darwin/linux）。按建议执行会砍断本机构建 macOS/Linux 二进制的唯一通路，与项目 CLAUDE.md「跨平台桌面应用脚手架」定位直接冲突。可承认的残余价值：server 模式四个 task + Dockerfile.server + README 第 267 行确实查无仓库内调用方（无 CI、git 历史仅 Initial commit），若拍板放弃 server 模式可单独裁掉这半条，但需保留 setup:docker / Dockerfile.cross / 各平台 build:docker 跨编译链。

## R-7 【构建与资产】build/darwin 与 build/linux 桌面打包配置仅被自动 rebrand 触碰过，若明确只发 Windows 可删（低置信度，建议确认产品意图后再动）

**驳回理由**：反驳成立，该减法建议站不住脚。①build/darwin 与 build/linux 不是「Windows 机器够不着的死配置」：build/darwin/Taskfile.yml:19 与 build/linux/Taskfile.yml:24 都写明非目标 OS 主机自动切到 build:docker（wails-cross 镜像跨编译），根 Taskfile.yml:40-43 专门保留 setup:docker 入口、build/docker/Dockerfile.cross 就位——在当前这台 Windows 开发机上 `wails3 task darwin:package` / `task linux:package` 是真实可执行链路。②删除波及面远大于建议声称的「两个 include」：build/Taskfile.yml:197-207 的 common:generate:icons 任务 generates 包含 darwin/icons.icns 且命令行带 -macfilename darwin/icons.icns -macassetdir darwin，而 build/windows/Taskfile.yml:46,80 的 Windows 构建/打包自身依赖 common:generate:icons——删掉 build/darwin 目录后下一次 Windows 构建会把它再生成回来（目录复活），要彻底删还得改 common 任务命令行；且 Task 的 includes 是硬依赖，漏改即令全部 task 命令（含 Windows）解析失败。③与项目定位冲突：根 CLAUDE.md 愿景明确为「跨平台桌面应用脚手架」，internal/app/window_darwin.go / window_linux.go 运行时分支真实存在，build/CLAUDE.md 把两目录列为正式模块。④这些资产由 wails3 update build-assets / generate:icons 自动维护（git log 确证两目录仅被 Initial commit 与 32cf794 自动刷新触碰），留着零负担，删除收益仅约 13 个文件。发现自身也承认这是「产品决策项而非死代码」，且『不发 macOS/Linux』的决策并未做出——一条以未做出的决策为前提、技术上又有耦合误判的建议，不构成可执行的减法。

