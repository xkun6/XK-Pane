# P6f 交接文档（清空上下文后从这里接手）

> 本文件 = 「会话状态 + 下一步」快照，专为开启 **P6f — Bedrock 知识库 MCP（ModSDK 文档/API 查询）**。
> 完整阶段计划见 [P6_PLAN.md](./P6_PLAN.md)（§1.5 Bedrock 真身约 line 46-47 / §P6f line 115-116 / §Go 移植映射表 line 85 / §推迟清单 line 119），P6e 实现范本见 [P6e_PLAN.md](./P6e_PLAN.md)，逐会话进度见 [progress.md](./progress.md)（末节），铁律见 [CLAUDE.md](./CLAUDE.md)。
> 总目标：把 `J:\Program\Agent\cc-pane`（Tauri+Rust+React）忠实重建到本仓库（Wails v3 + Go 1.25 + React 19 + MUI 9 + foundation MVVM）。分支 `rebuild/cc-pane`。

## 当前进度

- **P1~P5 ✅**：终端真跑 CLI / 删 Todo·Spec·Memory + Git SCM + Home / Workspace 树 + 会话恢复 / Local History / 终端进阶。
- **P6a~P6e ✅（全部 live 验过）**：orchestrator 地基（P6a）+ MCP 注入被启动 CLI（P6b）+ 33 编排工具（P6c）+ cli-hook 全栈（P6d，三引擎注入/上报/plan-archive/notify）+ **MC PE 控制全栈（P6e，7 单元：MCDK 桥协议 / embed 桥模组 / world+level.dat NBT / launch 12 步 / 日志缓冲 / 5 个 MCP 工具 execute_mc_code·reload_mc·get_mc_pe_logs·capture_window·click_window / 前端面板）**。
- **P7 ✅**：Web 镜像（后端 Web 网关 + 会话镜像/转录 + 前端结构化卡片视图 + 移动端适配）。
- **P6f（本次）= Bedrock 知识库 MCP，未开始。XK 侧目前零 Bedrock 代码。** 这是 P6 的最后一块；P6_PLAN 标其为「独立、L–XL」，不阻塞已完成的自我编排 headline。

## 本次目标

完整做 **P6f — Bedrock 知识库 MCP**，忠实移植 cc-pane `bedrock_query` 子系统：**embed.FS 内嵌 ~26MB ModSDK 基线数据 + sync.Once 惰性索引 + 26 个 `bedrock_*` 只读查询工具（纯算法直译）**。OTA 同步默认后置（见决策点）。

**执行顺序铁律**：先精读（下方清单）→ 产出 `P6f_PLAN.md`（cc-pane/XK 双侧 file:line 映射 + 单元切分 + 决策点 + 风险，对齐 P6e_PLAN 结构）→ 用户拍板 → 再逐单元实现。

## cc-pane Bedrock 子系统全貌（移植对象）

**恰好 26 个 `bedrock_*` 工具**（已 grep `orchestrator_bedrock_tools.rs` 核实 = 26，与 P6_PLAN §1.5/§23 一致；剔除非工具标识 `bedrock_query_service`/`bedrock_tool_router`/`bedrock_help_contains_all_phase_tool_names`）：
- **API/Event/分类（10）**：search_api / get_api / search_event / get_event / validate_call(name,side) / api_stats / list_categories / list_apis_by_category / search_component(item|block|entity) / get_component
- **文档/指南/模板/标准（12）**：search_guide(BM25) / read_guide / match_template / get_template_info / get_template_file / get_standard / get_expert_guide / get_review_rules / get_optimization_rules / get_json_spec(netease|modsdk|both) / help / list_mod_packs
- **审查/校验（4）**：review_code（Python2.7 静态 lint，13 条 ModSDK 规则）/ validate_json_definitions(addon_dir) / validate_addon_assets(addon_dir) / validate_test(test_json)

**内嵌资源**（`J:/Program/Agent/cc-pane/cc-panes-core/src/services/bedrock_query/resources/`，~26MB）：
- `data/api-index.json`（2.2MB，1936+ 条 `{type,name,method,side,description,params[],returns[],link,category}`）
- `data/components-index.json`（19KB）、`data/templates-index.json`（51KB，~40 模板）
- `data/docs/`（24MB，1788 个 .md，5 源：mcguide / mconline / mcdocs / ui_templates / wiki）
- `standards/`（176KB，17 个 .md：modsdk-expert / netease-code-review / netease-optimization-expert / netease-bedrock-json …）

**索引架构**（`loader.rs` + `index.rs`）：lazy load JSON/markdown → 一次性 build `Indices`（`api_by_name` / `event_by_name` / `apis_by_category` / `components` / `templates` / `guides`）→ O(1) 查询。`fuzzy.rs` = substring + 莱文斯坦距离排序；`search/mod.rs` = guide 用 BM25、component 用 substring。**两源混合**：`set_data_root(path)` 注入 OTA 目录覆盖嵌入资源（`read_resource` 优先 user-data 降级 embedded）。

**OTA `bedrock_doc_sync`**（**非 MCP，UI 命令，P6_PLAN 标后置**）：Gitea API 拉 `git.mcplugin.top/xiaokun/netease-bedrock-wiki`（mcguide+mconline）+ VuePress 爬 `mc.163.com/.../mcdocs/` → staging → 原子换目录 → `set_data_root` 清缓存重建。前端 `KnowledgeBasePanel`（manifest 状态 + 更新/重置/代理）。

**不在范围**：`cc-memory-mcp`（独立 stdio JSON-RPC 二进制、Memory DB、不挂 orchestrator，P6_PLAN §119 本轮推迟）——P6f 只重建 `bedrock_query`。

## 动手前必读

**XK 侧（建立现状事实，勿凭记忆）**
- `CLAUDE.md` —— 铁律：SQLite-only、工具层（httpx/procx/cryptox/logx/filex）、NativeDialogs、`@mui/icons-material` *Rounded、`t()` i18n、`theme.palette.foundation.*`、MVVM 调用边界、分层、平台 build tag。**用户可能调整了 wails3 工具链，以 `go.mod`/`Taskfile.yml` 现状为准。**
- `P6_PLAN.md` §1.5 / §85 / §115-116 / §119；`P6e_PLAN.md`（**文档结构范本**：双侧 file:line 映射 + 单元切分 + 拍板记录 + 风险待澄清 + 变更记录）。
- `progress.md` 末尾（P6a~P6e + P7 落地结论）；`findings.md`（line 16/69/118 Bedrock 22MB embed 风险评估）；`task_plan.md`（line 28/113-114 Bedrock 实现清单）。

**cc-pane 源（逐文件精读）**
- `…/cc-panes-core/src/services/bedrock_query/`：`service.rs`（26 工具 method 签名）、`mod.rs`（include_dir! 内嵌点 + 两源混合 read_resource/walk_resource_dir）、`loader.rs`（四槽 lazy + Box::leak）、`index.rs`（Indices 构建 + derive_category）、`fuzzy.rs`、`search/mod.rs`、`validate/*`、`review`。
- `…/src-tauri/src/services/orchestrator_bedrock_tools.rs`（**MCP 工具 router，26 个 `#[tool]` + 入参 struct，权威工具清单/签名**）。
- `…/cc-panes-core/src/services/bedrock_doc_sync/`（OTA：`service.rs` 主编排、`gitea_sync.rs`、`http_sync.rs`、`manifest.rs` 默认源 URL）——仅当本轮做 OTA 才精读。
- `…/web/components/mc_pe/knowledge/KnowledgeBasePanel.tsx`（前端面板）。
- `…/cc-panes-core/src/services/bedrock_query/resources/`（**直接复用的数据包**）。

## P6e 接线模式（P6f 镜像范本，照抄结构）

> P6f 是「又一组 MCP 工具 + 又一份 embed 资源 + 又几张表」，**接线方式与 P6e 完全同构**，照下面 file:line 镜像即可，不要另起炉灶。

- **MCP 工具注册**：`internal/services/orchestrator/orchestrator_mcpe_tools.go` —— `registerMCPETools(srv)` 被 `registerTools()` 调用，逐工具 `mcp.AddTool(srv,&mcp.Tool{...},handler)`；handler 签名 `func(s *Service)(ctx,*mcp.CallToolRequest,ParamStruct)(*mcp.CallToolResult,any,error)`；参数 struct camelCase；返回 `textResult(string)`（Bedrock 全是文本卡片）；失败返「错误: …」文本不返 Go error。**P6f 新建 `orchestrator_bedrock_tools.go` 同构注册 26 工具。**
- **embed.FS 资源**：`internal/services/mcpe/bridgemod.go:31` `//go:embed all:resources/...` + `var bridgeFS embed.FS`；`internal/services/mcpe/resources/.gitattributes` `* -text` 锁字节保真。**P6f 把 cc-pane `bedrock_query/resources/` 整目录复制进 `internal/services/bedrock/resources/` 再 `//go:embed`（注意 .gitattributes 防 CRLF 损坏 JSON/二进制）。**
- **storage 表**：`internal/storage/models_mcpe.go` —— 定义 GORM model + `TableName()` + `init()` 里 `AllModels = append(AllModels, ModelDescriptor{...})` → 重启 AutoMigrate。**P6f 若需表（如 OTA manifest / 同步状态）同样走 init()+AllModels；纯只读查询本身无需建表。**
- **app 装配**：`internal/app/app.go` —— service 预创建（`xxxSvc := pkg.New(holder)`）→ `application.NewService(xxxSvc)` 注册给前端 → `SetEmitter(mirrorEmit)` → `orchestrator.Deps{… Bedrock: bedrockSvc}` 注入。**P6f bedrock service 同样预创建 + 注入 Deps（给 orchestrator_bedrock_tools 消费）。**
- **平台分支**：`//go:build windows` / `!windows` 拆文件（如 `xxx_windows.go`/`xxx_other.go`），不在主逻辑写 `runtime.GOOS`。Bedrock 大概率**跨平台无需分支**（纯算法），validate_* 扫 addon_dir 路径注意分隔符。
- **现有依赖**（`go.mod`，go 1.25.5）：`modelcontextprotocol/go-sdk v1.6.1`（MCP）、`x/image`、`x/sys`、`go-pty`。**P6f 可能新增**：BM25/中文分词（自实现或轻量库）、Levenshtein（自实现即可）；尽量零新依赖，纯算法直译。

## Go 移植技术映射（P6_PLAN §85 已定）

- Rust `include_dir!`(~26MB) → Go `//go:embed`（zero-copy 进二进制）。
- Rust `OnceLock<Indices>` 惰性索引 → Go `sync.Once` 首次查询时 build maps（数据不进 `init()`，兼顾启动速度）；Rust `Box::leak` &'static → Go 直接包级变量持有（进程生命周期）。
- 模糊搜索（substring + 莱文斯坦）→ 自实现；guide 的 BM25 → 自实现 tokenizer（**注意 ModSDK 文档含中文，需考虑 CJK 分词/按字切分**）。
- 两源混合（`set_data_root` 覆盖 embedded）→ Go `SetDataRoot(path)` + 读取优先级函数。
- review_code 的 13 条 Python2.7 规则 → 直译为 Go 静态检查（正则/AST 视 cc-pane 实现而定）。

## P6f_PLAN 须先回答的决策点（精读后用 AskUserQuestion 收口）

1. **数据体量**：全量内嵌 ~26MB（二进制膨胀同等量级）vs 精简（去 ui_templates/wiki 等低频源）？默认建议全量直接复用 cc-pane 资源包。
2. **OTA 同步本轮做不做**：P6_PLAN 标「后置」。建议**本轮只 embed baseline、不做 OTA**（OTA 牵涉 Gitea API + VuePress 爬虫 + httpx + 前端面板，是独立大件）；若要做则单列 P6f 末单元。
3. **前端知识库面板**：全做（KnowledgeBasePanel + 状态/更新/代理对话框）/ 只做只读浏览 / **暂不做 UI、仅把 26 工具暴露给 Agent**（推荐起步，工具本就是给 pane 内 CLI 用的）。
4. **26 工具全做 vs 分批**：高频（search_api/get_api/validate_call/search_guide/get_standard）必做；review_code（13 规则）与 validate_*（依赖 addon_dir 真实路径）是否同批？
5. **validate_* 文件系统工具**：依赖被检查 addon 目录路径，需确认调用语义（相对/绝对、是否限制在工作空间内）。
6. **确认 cc-memory-mcp 不做**（P6f 仅 `bedrock_query`）。

## 建议单元起点骨架（以精读后判断为准）

- **P6f-1** 资源内嵌 + loader：复制 cc-pane `resources/` 进 XK + `//go:embed` + `.gitattributes` 字节保真 + 解析 api/components/templates 索引 + walk standards；容量自测（API≥1000 等）。
- **P6f-2** 索引构建 + 搜索算法：`Indices`（maps + derive_category）sync.Once 构建 + fuzzy（substring+Levenshtein）+ guide BM25（含中文分词取舍）。
- **P6f-3** API/Event/组件查询工具（~10）接进 orchestrator（新建 `orchestrator_bedrock_tools.go`）。
- **P6f-4** 文档/指南/模板/标准工具（~11）+ BM25 检索接线。
- **P6f-5** 审查/校验工具（review_code 13 规则 + validate_*，按拍板范围）。
- **P6f-6**（按拍板）前端知识库面板。
- **P6f-7**（后置/可选）OTA `bedrock_doc_sync`（Gitea + VuePress + 原子换目录 + 前端）。

## 工作流铁律（务必遵守）

1. 先出 `P6f_PLAN.md` 并让用户拍板，再分单元实现。每单元独立可编译 + 独立提交（**中文 conventional commit**，含单元号），**不要 push**。全程**中文回复**。
2. 改 Go 后端必须**完整重启 wails3 dev** 才生效（提醒用户）；改 embed 资源要重 build。
3. 架构有歧义/多解先停下问用户（CLAUDE.md「Think Before Coding」），不要静默选路。
4. **忠实复刻 cc-pane 优先**，偏离必须显式标注理由（如中文分词差异、无现成 BM25 库等）。
5. 验收：每单元 `go build ./... && go vet && go test`（+ 前端 `pnpm typecheck`）；查询正确性优先用单测（拿真实 API 名/事件名断言），手测由用户在 pane 内让 CLI 调 `bedrock_*` 工具核对（同 P6d/P6e 验法）。

---

**触发方式**：新会话只需说「读 P6F_HANDOFF.md，按它开始 P6f」。先精读上述 XK + cc-pane 源，再产出 `P6f_PLAN.md` 给用户拍板；读的过程中遇任何架构歧义随时问。
