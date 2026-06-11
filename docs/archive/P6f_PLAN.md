# P6f 计划 — Bedrock 知识库 MCP（cc-pane → XK 忠实重建，XL）

> 来源：7-agent 并行精读（`wf_562c8b5e-291`，~580k token）通读 cc-pane `bedrock_query/` 全栈（mod/loader/index/models/fuzzy + search/×8 + validate/×4 + review/×3+13 + standards/ops）+ `orchestrator_bedrock_tools.rs`（26 `#[tool]` 权威契约）+ `bedrock_doc_sync/`（OTA 定界）+ XK P6e 接线范本，全部带 `file:line`。本文件 = 动手前事实基线 + 拍板记录 + 单元切分。**已拍板（§2），下一步进 P6f-1。**
> 相关：交接见 [P6F_HANDOFF.md](./P6F_HANDOFF.md)；总计划见 [P6_PLAN.md](./P6_PLAN.md)（§1.5 / §85 映射 / §115-116 / §119）；P6e 范本见 [P6e_PLAN.md](./P6e_PLAN.md)；逐会话进度见 [progress.md](./progress.md)；铁律见 [CLAUDE.md](./CLAUDE.md)。
> 分支：**`master`**（P6 全部工作已提交于此；交接文档里 `rebuild/cc-pane` 仅为概念名，以实际仓库为准）。

---

## 0. 一句话 + 关键依赖发现

cc-pane 的 **Bedrock 知识库**让 pane 里的 Agent 经 MCP **只读查询网易版 Minecraft ModSDK 文档/API/组件/模板/规范**：编译期内嵌 ~26MB 基线数据（1956 条 API + 76 组件 + 42 模板 + 1982 个 docs + 17 标准），首次查询时惰性建索引，暴露 **26 个 `bedrock_*` 只读工具**（纯算法直译，无外网）。**OTA `bedrock_doc_sync`** 是独立大件：从 Gitea/VuePress 爬最新文档、原子换目录、通知查询层清缓存重建。XK 侧**零 Bedrock 代码**。

**关键精读发现（决定单元切分 + 纠偏交接假设）**：

1. **工具数 = 26**（router 26 个 `#[tool]`；文件头注释写"29"过时，以 grep 实际计数 26 为准）。返回统一单段 Markdown 文本；service 报错走 `isError=true` 的**工具结果**（`<tool> failed: {e}`），**不是** JSON-RPC protocol error → 对齐 XK 的 `textResult("错误: …"), nil, nil` 约定（绝不返 Go error）。
2. **⚠️ 没有莱文斯坦距离、没有真正的 BM25。** 全子系统（grep 全目录确认）纯子串匹配 + 字段加权打分。交接手册里的「substring + 莱文斯坦」「guide BM25」假设**不成立**。Go 侧**不引任何模糊/BM25 库**，纯 `strings.Contains/HasPrefix/==` + `sort.SliceStable(score 降序, name 升序)`。
3. **⚠️ CJK 分词真相 = `split_whitespace()`。** guide tokenizer 唯一就是 `keyword.to_lowercase().split_whitespace()`；连续中文串当**单 token** 走子串匹配，**绝无按字切/n-gram/jieba**。Go = `strings.Fields(strings.ToLower(keyword))`。改了分词召回就与 cc-pane 不一致。
4. **⚠️ review_code = 12 条规则，不是 13**（mod.rs 注释写"13 static rules"，`all_rules()` 实际只注册 12 条；交接说的"13 个文件"把 `rules/mod.rs` 注册中心算进去了）。**零正则、零外部依赖、纯行扫描 + 缩进状态机**，无需真 Python AST。column 是字节偏移。
5. **set_data_root / 双源覆盖当前 dormant**（doc-sync 链路移除后无调用者）；**但本轮做 OTA → 必须实现**（OTA 产出的数据正是经 `set_data_root` 被查询层消费）。⇒ 缓存**不能用纯 `sync.Once`**（Once 不可重置，与 invalidate 冲突），改 `RWMutex` 保护的 `*T` 槽位（nil 判定重建）或 `atomic.Pointer[T]`。
6. **OTA `bedrock_doc_sync` ≈ 6200 行 Rust**（去测试 ~4000 行业务），无核心反向依赖、可干净拆成末批单元；唯一硬耦合 `set_data_root` 已 dormant。**前置依赖**：`httpx` 现为单例 client，OTA 需 per-call 代理/socks5 → 须先给 httpx 加代理入口（否则违反「禁裸 net/http」铁律）。`html2md` 需找 Go 等价库（输出未必字节一致，影响下游 build_api_index 解析）。
7. **XK 接线 7 处镜像 P6e 即可**：新 `orchestrator_bedrock_tools.go`（`registerBedrockTools(srv)` + 在 `registerTools` 末尾追一行）+ `orchestrator.Deps.Bedrock` 窄接口 + `internal/services/bedrock/`（service）+ embed + `.gitattributes(* -text)` + `internal/services/bedrockdocsync/`（OTA）+ 前端 KB 面板 + `internal/storage/models_bedrock.go`（OTA manifest/状态，纯只读查询本身无需表）。

---

## 1. 双侧架构映射（cc-pane `file:line` → XK 目标）

cc-pane 源根：`J:/Program/Agent/cc-pane/cc-panes-core/src/services/bedrock_query/`（除工具 router 在 `src-tauri/src/services/orchestrator_bedrock_tools.rs`；OTA 在 `cc-panes-core/.../bedrock_doc_sync/` + `src-tauri/.../commands/bedrock_doc_sync_commands.rs`）。
XK 目标：`internal/services/bedrock/`（查询引擎 + 26 工具的下游）+ `internal/services/orchestrator/orchestrator_bedrock_tools.go`（MCP 工具注册）+ `internal/services/bedrockdocsync/`（OTA）+ `frontend/src/pages/`（KB 面板）+ storage/events/app 接线。

### 1.1 资源内嵌 + 双源加载（跨平台）

| cc-pane | XK 目标 | 要点 |
|---|---|---|
| `mod.rs:22-23` `include_dir!` | `bedrock/resources.go` `//go:embed` | 整 `resources/` 子树编译期内嵌。Go 用 `//go:embed all:resources` + `var bedrockFS embed.FS`，**`fs.Sub(bedrockFS,"resources")`** 得去前缀 sub-FS（让相对路径与 Rust 完全一致 `data/api-index.json`）。`all:` 前缀防 `_`/`.` 开头文件被忽略。 |
| `mod.rs:66-79` `read_resource(rel)` | `bedrock/source.go` `readResource(rel)` | 双源单文件：① `USER_DATA_ROOT` 已设且 `<root>/<rel>` 存在 → `os.ReadFile`（读失败**报错不回退**——让用户看到"同步数据坏了"而非掩盖）；② 否则 `subFS.ReadFile(rel)`，两源都无 → err。 |
| `mod.rs:90-167` `walk_resource_dir(rel,ext)` | `walkResourceDir(rel,ext)` | 双源目录遍历，**目录级整体替换**（用户子目录存在则整棵替换内嵌子树，**非文件级 merge**）。扩展名比较去点、大小写不敏感（`EqualFold`）。用户侧单文件读失败 `continue` 容错；内嵌侧 strip `{base}/` 前缀。路径恒正斜杠（os 侧 `filepath.ToSlash`）。 |
| `mod.rs:39-54` `set_data_root`/`invalidate` | `SetDataRoot(root)` + 缓存失效 | 进程级 `userDataRoot`（RWMutex 或 `atomic.Pointer[string]`）。`SetDataRoot` 写入后**立即** `loader.Invalidate()+index.Invalidate()`。**本轮做 OTA → 必须实现**（OTA `sync()` 末尾调它）。 |

**命门**：embed key 含目录名前缀（`resources/data/...`）→ 用 `fs.Sub` 抹平。两源覆盖是**目录级替换不是合并**（误写成"逐文件覆盖"会把内嵌 599 个 mcguide 全隐藏）。read_resource 严格（用户文件读失败即报错），walk_resource_dir 容错（单文件 continue）——**两者语义不同，别统一**。

### 1.2 索引构建 + 数据模型（跨平台）

`loader.rs`（四槽惰性加载）+ `index.rs`（Indices 构建）+ `models.rs`（serde 模型）→ XK `bedrock/loader.go` + `bedrock/index.go` + `bedrock/models.go`。

| cc-pane | XK 目标 | 要点 |
|---|---|---|
| `loader.rs:33-94` 四槽 lazy + `Box::leak` | `loader.go` 可失效缓存 | apis/components/templates/standards 四槽。**Go 用普通指针 + GC，丢弃 Box::leak**（Rust 生命周期妥协，明确简化点）。因本轮要 invalidate，**每槽用 `RWMutex`+`*T` nil 判定重建**，不用 `sync.Once`。 |
| `loader.rs:56-94` 三索引 JSON 解析 | `Apis()/Components()/Templates()` | `readResource("data/api-index.json")` → `json.Unmarshal` 成 `[]ApiEntry`（1956 条，顶层数组）；同构 components(76)/templates(42)。错误带 context（`parse api-index.json: %w`）。 |
| `loader.rs:99-123` standards 加载 | `Standards()` `map[stem]string` | `walkResourceDir("standards","md")`，key=`file_stem`（去 .md，含 CJK 名如「内存优化」），非 UTF-8 跳过。17 个 .md。 |
| `index.rs:102-131` api-index 按 `type` 拆分 | `index.go` build | `type` 字段 → `api`(1655) / `event`(298) / `header`(3)。api+event 都进 `api_by_name`/`event_by_name`（**lower** key）**且都进** `apis_by_category`（名为 apis 实混 event）；header 只进 `all_headers`。未知 type 静默忽略（刻意容错）。name 空不进 by_name。 |
| `index.rs:67-87` `derive_category` | `deriveCategory(e)` | category 字段实测全空(0/1956) → 真逻辑：`link` 把 `\`→`/` split，找锚点 `["接口","事件"]`（按序，先命中先返回），取下一段去 `.md`。中文锚点 UTF-8 字面量。 |
| `index.rs:171-217` `scan_guides`+`parse_headings` | `scanGuides()` | `walkResourceDir("data/docs","md")` → `GuideEntry{title(首个`# `行,缺则文件 stem), path(相对 data/docs 正斜杠), source(首段 mcguide/mconline/wiki/ui_templates/mcdocs), breadcrumb(父段` / `join), headings(`## `/`### `行)}`。**前缀判定带尾空格**（`# `/`## `/`### `），else-if 链同序。 |
| `models.rs:10-59` ApiEntry/Param/ReturnSpec | `models.go` | 全字段缺失=零值（Go json 天然等价 serde default）。**rename**：`type`→`Kind`/`Ty`（json:"type"）、`return`→`Returns`（json:"return"，return 是 Go 关键字）。`agent_note` snake_case tag。 |
| `models.rs:66-78` ComponentEntry（`serde(flatten) extra`） | 自定义 `UnmarshalJSON` | 显式字段 id/type/name/description + **flatten 兜住 data_type/properties/values/example**。Go 无 flatten → 自定义 `UnmarshalJSON`（先解 `map[string]json.RawMessage`，抽已知字段，余下塞 `Extra`）。**get_component 渲染卡片用到 extra → 不能丢弃**（index 构建只用 id，但工具输出要全字段）。 |
| `models.rs:87-122` TemplateEntry（混 typed+untyped） | `models.go` | 多数强类型；`apis`/`systems`/`file_count` 用 `json.RawMessage`（serde_json::Value，systems 历史混字符串+对象，别强类型）。snake_case tag 逐个对（mod_name/behavior_pack/has_custom_entity…）。 |
| `models.rs:128-137` GuideEntry | `models.go` | 纯内存（不反序列化，无 json tag），由 scanGuides 构造。 |

**Indices 结构**（`index.rs:20-41`）：`ApiByName`/`EventByName`(map[lower]→[]*ApiEntry)、`ApisByCategory`(map[raw category]→[]*ApiEntry，api+event 混)、`AllApis`/`AllEvents`/`AllHeaders`、`Components`(map[id]，空 id 跳过)、`Templates`(map[id])、`Guides`([]GuideEntry)、`Standards`(map[stem]string)。单例可失效。

### 1.3 搜索 / 匹配算法（跨平台，**纯子串，无 BM25/Levenshtein**）

`fuzzy.rs` + `search/{api,event,component,category,guide,template,samples_dump,mod}.rs` → XK `bedrock/fuzzy.go` + `bedrock/search/*.go`（或单包 `bedrock/search.go`）。

| 算法 | cc-pane | 规则（逐字照搬） |
|---|---|---|
| `nameScore(q,name,blob?)` | `fuzzy.rs:18-44` | q 空→0（早退，否则空串 Contains 恒真污染全表）。lower 后：`==`→20 / `HasPrefix`→15 / `Contains`→10 / 0；blob lower 含 q：基础 0→3，否则 **+3 加性**。 |
| `scoreAndRank`（API/event） | `api.rs:90-110`,`16-41` | blob = `name+' '+description+' '+remark+' '+agent_note+' '+cat`（cat 空则 deriveCategory）。`nameScore>0` 保留，排序 (score 降, name 升)，`truncate(limit)`。limit：search=**20**、validate 建议=**3**、not-found suggestions=**5**。 |
| `guideScore`（"BM25-lite" 实为字段加权子串） | `fuzzy.rs:57-97` | title 含 token +5 / breadcrumb +3 / 任一 heading +2 / path +1（每字段每 token 最多一次，hit 布尔）。matched=命中≥1 字段的 token 数；**matched>1 → score += matched*3**。无 k1/b/IDF/TF。 |
| guide tokenizer | `guide.rs:24-29` | **`strings.Fields(ToLower(keyword))`**（= split_whitespace；空→err "keyword must not be empty"）。**连续 CJK = 单 token**。 |
| guide search | `guide.rs:24-90` | source 可选 ∈ `{mcguide,mconline,wiki,ui_templates}`（否则 "invalid source"）。过滤+打分+排序 (score 降, title 升)，`MAX_RESULTS=20`。输出格式是契约：`Found N guide(s)...` + `- [source] title  (score=X)` / `  path:` / `  breadcrumb:`。 |
| component search | `component.rs:29-90` | id `Contains`+3 / name +2 / description +1（**可叠加**，最高 6）。componentType ∈ `{all,item,block,entity,netease_item,netease_block}`。排序 (score 降, id 升) truncate 20。`get` = 精确 map。 |
| template `match_` | `template.rs:21-85` | tags ⊕ keyword 至少给一。每查询 tag：模板某 tag `==`+2 否则 `Contains`+1（**else-if 互斥，每查询 tag 一次**）。keyword：id Contains +3，模板**每个** tag Contains +1（**可累加**）。排序 (score 降, id 升) truncate 20。 |
| category | `category.rs:18-82` | `list()`：枚举 key 升序 `cat (N APIs)`。`by(cat)`：**先原始大小写 key 精确 map 命中**，miss 则 lower 全表子串过滤、key 升序逐个 render。（与 by_name 全 lower 不同，**别统一**。）render 内 APIs 按 name 升序。 |
| get/validate（api/event 共用） | `api.rs:241-419`,`event.rs:8-32` | key=lower(name)。`get`：side 给则须 server/client（否则**返 error**），命中后 retain side，空→suggestions(5)。`validate`：side 非法→**返 "INVALID:" 字符串**（不是 error），合并查 api+event 两表。render 排序 side_rank(server0/client1/其他2)→name。 |
| `truncateChars` | `api.rs:43-57` | description 预览 **80 字符**（rune 非 byte，`[]rune(s)[:80]+"..."`，防切断多字节 UTF-8）。 |

**命门**：①不引 Levenshtein/BM25 库（纯子串）；②CJK 只 split whitespace 不分词；③排序统一 `sort.SliceStable` 二级键（score 降, 名字升）保证 Go map 无序下输出可复现；④category 精确命中用原始大小写 key、by_name 用 lower key——不一致是刻意的。

### 1.4 26 个 `bedrock_*` MCP 工具契约（接进 XK orchestrator）

`orchestrator_bedrock_tools.rs:168-602`（`#[tool_router]` impl，26 `#[tool]`）→ XK 新建 `internal/services/orchestrator/orchestrator_bedrock_tools.go`。所有入参 struct `#[serde(rename_all="camelCase")]` → **wire 名 camelCase**（Go json tag 必须对齐）。`Option<T>`/`#[serde(default)]` = 可选。

**Phase 2 — 高频查询（10）→ P6f-3**

| 工具 | 入参（camelCase） | 委托 service.rs | 输出 / ccPaneRef |
|---|---|---|---|
| `bedrock_search_api` | `name`(必) | `search::api::search` | Markdown ≤20 排名；router:170-182 |
| `bedrock_get_api` | `name`(必), `side?` | `search::api::get` | 完整 API 卡片；router:184-203 |
| `bedrock_search_event` | `name`(必) | `search::event::search` | ≤20 事件；router:205-217 |
| `bedrock_get_event` | `name`(必), `side?` | `search::event::get` | 事件卡片；router:219-238 |
| `bedrock_validate_call` | `name`(必), `side`(**必**,String) | `search::api::validate` | VALID/INVALID；router:240-259 |
| `bedrock_api_stats` | — | `search::api::stats` | KB 总量+top 分类；router:261-268 |
| `bedrock_list_categories` | — | `search::category::list` | 全分类字母序；router:270-277 |
| `bedrock_list_apis_by_category` | `category`(必) | `search::category::by` | 该分类全 API；router:279-298 |
| `bedrock_search_component` | `query`(必), `componentType?`(默认 all) | `search::component::search` | 组件子串匹配；router:300-320（默认 all 在 router:307 解析） |
| `bedrock_get_component` | `componentId`(必) | `search::component::get` | 完整组件卡（用 extra）；router:322-336 |

**Phase 3 — 指南/模板/标准（11）+ list_mod_packs（template 支撑）→ P6f-4**

| 工具 | 入参 | 委托 | ccPaneRef |
|---|---|---|---|
| `bedrock_search_guide` | `keyword`(必), `source?` | `search::guide::search` | router:338-357 |
| `bedrock_read_guide` | `path`(必), `maxLines?` | `search::guide::read` | router:359-378 |
| `bedrock_match_template` | `tags?`, `keyword?`（默认在 router:387-388 `unwrap_or_default`） | `search::template::match_` | router:380-401 |
| `bedrock_get_template_info` | `templateId`(必) | `search::template::info` | router:403-417 |
| `bedrock_get_template_file` | `templateId`(必), `filePath`(必) | `search::template::file` | router:419-438 |
| `bedrock_get_standard` | `name`(必) | `standards::ops::get` | router:440-452 |
| `bedrock_get_expert_guide` | — | `standards::ops::expert_guide` | router:454-461 |
| `bedrock_get_review_rules` | — | `standards::ops::review_rules` | router:463-470 |
| `bedrock_get_optimization_rules` | — | `standards::ops::optimization_rules` | router:472-481 |
| `bedrock_get_json_spec` | `contentType?`(默认 both) | `standards::ops::json_spec` | router:483-502 |
| `bedrock_help` | — | `standards::ops::help` | router:504-511 |
| `bedrock_list_mod_packs` | — | `search::template::list_mod_packs` | router:592-601（cc-pane 归 Phase5 注释段，但 template 支撑，实现归 P6f-4） |

**Phase 4 — 静态审查（1）→ P6f-5**

| 工具 | 入参 | 委托 | ccPaneRef |
|---|---|---|---|
| `bedrock_review_code` | `code`(必), `filename?`(默认 "unknown.py" 在 service.rs:151) | `review::run` | router:513-533 |

**Phase 5 — 文件系统/JSON 校验（3）→ P6f-6**

| 工具 | 入参 | 委托 | ccPaneRef |
|---|---|---|---|
| `bedrock_validate_json_definitions` | `addonDir`(必,绝对路径) | `validate::json_defs::run` | router:535-554 |
| `bedrock_validate_addon_assets` | `addonDir`(必,绝对路径) | `validate::assets::run` | router:556-572 |
| `bedrock_validate_test` | `testJson`(必,JSON 字符串) | `validate::test_schema::run` | router:574-590 |

**默认值落点差异**（Go 移植对齐）：`search_component` 的 `all`、`match_template` 的 tags/keyword 默认在 **router 层** 解析；`review_code` 的 filename 默认在 **service 层**；`get_json_spec` 的 both 在更下游。**参数 struct 复用**：`name` 单字段被 search_api/search_event 共用；`name+side?` 被 get_api/get_event 共用；`addonDir` 被两个 validate 共用。
**接线**：`orchestrator.Deps` 加窄接口 `Bedrock`（声明 26 工具实际要调的方法子集，如 `SearchAPI(name) (string,error)` …）；handler 签名 `func (s *Service) toolBedrockXxx(ctx, *mcp.CallToolRequest, in XxxParams) (*mcp.CallToolResult, any, error)`；失败 `textResult("错误: …"), nil, nil`；nil 兜底返「错误: Bedrock 知识库不可用」。工具 description 中文（参数语义/默认/上限讲清）。**jsonschema 坑（P6e-6 已踩）**：tag 描述不可以 `词=` 开头，否则 go-sdk 当 key 指令解析致 AddTool panic。

### 1.5 review_code — 12 条规则（跨平台纯行扫描）

`review/mod.rs`（流水线）+ `review/line_index.rs` + `review/violation.rs` + `review/rules/*.rs`（12 规则）→ XK `bedrock/review/`。

**流水线**（`mod.rs run`）：`LineIndex::new(code)` → 遍历 `all_rules()` 12 条逐条 `check(&idx,filename)` → 排序 (severity 降, line 升, rule 升) → `render_report`（`# Review Report: {filename}` / `Lines reviewed: N` / `Violations: N (Critical: c, Warning: w, Suggestion: s)` / 空则 `## No issues found` / 否则三段 `- line {L} col {C}: \`{rule}\` — {message}` + 可选 `  Fix: {hint}`）。
**LineIndex**（`line_index.rs`）：`indent_of`(空格 1/tab 4)、`strip_comment`(引号态感知剥行尾 `#`)、`docstring_mask`(triple-quote toggle 粗启发式)。**⚠️ Go 坑**：Rust `str::lines()` 对 `"abc\n"` 不产尾随空行、保中间空行、吃 `\r\n` 的 `\r`；Go `strings.Split(code,"\n")` 会多尾随空串且不 trim `\r` → 必须封装 `splitLines` 复刻，否则行号/总行数偏 1。
**Violation**（`violation.rs`）：`rule`(id) / `severity`(Critical=2/Warning=1/Suggestion=0) / `line`(1-based,0=整文件) / `column`(1-based **字节偏移**,0=未知) / `message` / `fix_hint?`。

| 规则 | 文件 | 检测 / 手法 | severity |
|---|---|---|---|
| fstring | fstring.rs | `f"`/`f'` 字面量（is_token_start 边界，不报 rf/fr）；token-scan | Critical |
| print_call | print_call.rs | 无 `from __future__ import print_function` 时的 `print(...)`；先全文件预扫 future，再逐行（排除 `.print`/`pprint`/Py2 语句） | Critical |
| type_hints | type_hints.rs | def 形参注解 `x: int` 或返回 `-> bool`；括号深度状态机（仅含冒号行命中，不跨行） | Critical |
| async_await | async_await.rs | `async`/`await` 整词（边界非字母数字_）；每行一次，column=1 | Critical |
| walrus | walrus.rs | `:=`（排除 `==`/`!=`/`>=`/`<=`）；字节对扫描 | Critical |
| cross_side | cross_side.rs | ServerSystem 文件 import mod.client（反之亦然）；infer_side(类基类→filename)+path_starts_with 边界判定。**唯一用 filename 参数** | Critical |
| engine_factory | engine_factory.rs | `GetEngineCompFactory()` 缩进>0（非模块级）；精确子串（带空括号） | Warning |
| fn_import | fn_import.rs | 函数体内 import/from-import；缩进栈 def_stack（class 体内不报） | Warning |
| tick_throttle | tick_throttle.rs | Tick/Update 类函数体≥3 行未用质数取模(7/11/19/41/79)节流；缩进体收集（**strip_comment 体**） | Warning |
| block_entity_salt | block_entity_salt.rs | 函数体提 ServerBlockEntityTickEvent 但缺 salt(`hash(`/`salt`/`% 质数`)；缩进体收集（**保留 raw 含注释**，与 tick 不同） | Warning |
| magic_number | magic_number.rs | ≥100 整数字面量且不在白名单(100/128/200/255/256/360/500/512/1000/1024/2048/4096)；字节边界判定排浮点/标识符。**每文件≤3** | Suggestion |
| dict_access | dict_access.rs | `obj['key']`/`obj["key"]` 下标（建议 .get()）；手写下标扫描，键 `[A-Za-z0-9_-]`，SKIP_PREFIXES 跳整行。**每文件≤5** | Suggestion |

**命门**：全部纯字节/字符串扫描 + 缩进状态机，**无需 AST**。column=字节偏移（Go `[]byte` 天然一致）。tick/salt 的体收集差异（strip_comment vs raw）不能合并。docstring_mask 是"够用就好"启发式，**别顺手改进成真解析器**（否则 violation 数量分叉）。

### 1.6 validate_* 校验器 + standards ops（跨平台 + 文件系统）

`validate/{mod,json_defs,assets,test_schema}.rs` + `standards/ops.rs` → XK `bedrock/validate/` + `bedrock/standards.go`。**所有校验器返回 `Ok(报告字符串)` 不返 Err**（路径错误也是报告字符串）。

| 工具/能力 | cc-pane | 要点 |
|---|---|---|
| 共享报告骨架 | `validate/mod.rs:22-112` | Severity(Error/Warning/Info) + Issue{severity,location,message} + render_report（`# {title}` / `Files scanned: N` / `Issues: N (Errors/Warnings/Infos)` / 空→`## All checks passed` / 否则 Errors→Warnings→Infos 三段 `- \`{location}\`: {message}`）。逐字节格式是契约。 |
| validate_json_definitions | `json_defs.rs:23-231` | addonDir 须**绝对/存在/目录**（否则报告字符串）。扫 4 SCAN_DIRS（netease_items_beh 期望 fmt "1.10" / netease_blocks "1.10.0" / netease_recipes / entities）。json 扩展名**大小写敏感**。每文件 C1-C5：BOM 剥离/UTF-8/JSON parse(line:col)/format_version/命名空间(identifier 无 `:`→警告,minecraft:→info,空→警告)。 |
| validate_addon_assets | `assets.rs:46-230` | 扫 `textures/`。json ext **大小写不敏感**（与上不同）。每图 A1-A4：格式(非 png/tga→警告)/文件名(空格/大写/非 `[a-z0-9_.-]`)/>1MB/PNG IHDR(手解析 BigEndian b[16:20],b[20:24]→2 的幂/宽高比)。 |
| validate_test | `test_schema.rs:23-134` | **纯 JSON 字符串，零文件系统**。校验 name(非空字符串) + steps(数组)。每 step type ∈ `[wait,assert,action,screenshot]`，各自字段。输出 VALID/INVALID。**坑**：未知 type 文案用 Rust `{:?}` Debug 格式（`["wait", "assert", ...]`），Go 手拼复刻。 |
| get_standard | `standards/ops.rs:11-65` | name 空→**唯一返 Err**。剥 `.md`/`.MD`（大小写敏感各试）。精确 stem map（原大小写）→ `# {key}\n\n{content}`；miss 则 lower 子串过滤，1 命中→输出，多→"Did you mean"，0→提示 bedrock_help。 |
| get_expert_guide / review_rules / optimization_rules | `ops.rs:67-89` | `fetch_required(stem)` 固定 stem：`modsdk-expert`/`netease-code-review`/`netease-optimization-expert`，缺→Err。**返原文不加 `# ` 头**（与 get_standard 不同）。 |
| get_json_spec | `ops.rs:96-108` | `netease`→`netease-bedrock-json`；`modsdk`→`modsdk-json-generator`；**其它/None/空→both** 拼 `{netease}\n\n---\n\n{modsdk}`（分隔符是测试契约）。 |

**命门**：跨平台 addonDir 用 `filepath.IsAbs`+`os.Stat`(IsDir)；json_defs 扩展名大小写敏感、assets 不敏感（**别统一**）；serde 的 `line:column` 报错文案 Go `encoding/json` 只给 Offset → 需自算行列或显式标注该文案不保证字节一致（待澄清）。

### 1.7 OTA `bedrock_doc_sync`（独立大件，末批单元 P6f-7..10）

`cc-panes-core/.../bedrock_doc_sync/{mod,manifest,progress,proxy,gitea_sync,http_sync,service,build_api_index,build_templates_index}.rs`（~6200 行）+ `src-tauri/.../commands/bedrock_doc_sync_commands.rs`（7 命令）→ XK `internal/services/bedrockdocsync/` + 前端 KB 面板。**无核心反向依赖，可干净拆**；唯一硬耦合 `set_data_root`。

| 模块 | cc-pane | XK 目标 / 要点 |
|---|---|---|
| 数据层 | `manifest.rs:15-238` `progress.rs:16-92` `proxy.rs:13-110` | `Manifest{schema_version:1, synced_at, sources:map[string]SourceInfo(tagged kind=gitea/http/embedded), stats}`。默认源 `DEFAULT_WIKI_REMOTE=https://git.mcplugin.top/xiaokun/netease-bedrock-wiki`(branch main) / `DEFAULT_MCDOCS_BASE_URL=https://mc.163.com/dev/mcmanual/mc-dev/mcdocs/`。ProgressEvent tagged(Started/GiteaSync/HttpFetch/BuildIndex/Validating/Completed/Cancelled/Failed/Log)。ProxyConfig(https_proxy+auth, from_env 优先级)。**snake_case tag 须与磁盘格式字节一致**。 |
| httpx 代理扩展（前置） | `proxy.rs` build_reqwest | **OTA 唯一工具层新增需求**：现 `httpx` 单例 client 不支持 per-call 代理/socks5 → 给 httpx 加「带 proxy/超时/UA 定制」构造入口（socks5 引 `golang.org/x/net/proxy`）。`probe()`=HEAD github.com 5s 金丝雀。 |
| Gitea 爬取 | `gitea_sync.rs` | GET `/api/v1/repos/{o}/{r}` → trees `?recursive=true&per_page=1000` 分页 → 客户端按 subdir+扩展(md/json)过滤 → `/raw/branch/{b}/{path}` 并发拉 blob（绕开 6.7GB 全量 mirror，只要 7MB md）。**无 git2 依赖**（对 Go 利好，不引 go-git）。Go：httpx GetJSON + errgroup+semaphore。 |
| VuePress 爬取 | `http_sync.rs`（~1450 行，最脆） | 命中 mc.163.com VuePress 1.x：Plain（索引页抓 a href）/ Vuepress（SPA 壳→preload 恢复 app.<hash>.js→bundle harvest 路由表→逐叶子页→切正文 DOM→HTML→MD）。**HTML→MD 需 Go 等价库**（`JohannesKaufmann/html-to-markdown`？输出未必字节一致）。强依赖站点结构，失败回退 Embedded。 |
| 编排 | `service.rs:77-447` | `sync()`：解析源/并发(默认4,1-16)→建 `bedrock-data.tmp` staging→proxy/source 解析(explicit>persisted>默认)→Gitea 分桶同步(失败回退 Embedded)→mcdocs VuePress→条件构建 api-index/templates-index→**原子换目录**(bedrock-data→.bak, staging→bedrock-data, 删 .bak, 失败回滚)→`set_data_root(新目录)` 清缓存重建→写 manifest。`reset`(归档保留5份)/`set|load_proxy`/`set|load_sources`/`list|prune_archives`。cancel 用 `context.Context`(替 AtomicBool)。 |
| 索引构建器 | `build_api_index.rs`(881) `build_templates_index.rs`(1116) | 把同步下来的 md/demo 树转 `*-index.json`（纯文件解析无网络，可独立移植）。 |
| 前端/命令 | `bedrock_doc_sync_commands.rs`(7 命令) | status/start/cancel/reset/set_proxy/load_proxy/list_archives + set/load_sources → Wails service 方法。进度 `mpsc→window event "bedrock-doc-sync:progress"` → XK `internal/events` RegisterEvent + `app.Event.Emit`。 |
| 前端面板 | `web/components/mc_pe/knowledge/KnowledgeBasePanel.tsx` | manifest 状态 + 更新/重置/代理对话框 + 进度对话框 + 归档列表。XK：MVVM + MUI + t() + NativeDialogs(reset 确认走原生)。 |

**⚠️ P6f-8/9 实现前须精读**（本计划只做 OTA 定界，未逐字读 4 大文件）：`gitea_sync.rs` 全文 / `http_sync.rs` 1450 行 / `build_api_index.rs` 881 / `build_templates_index.rs` 1116（镜像 P6e 把 Win32 在计划里只到常量级、P6e-7 实现时精读的做法）。

### 1.8 XK 接线镜像（照抄 P6e 7 处）

| 接线点 | P6e 范本（XK file:line） | P6f 对应 |
|---|---|---|
| MCP 工具注册 | `orchestrator_mcpe_tools.go:25-39` `registerMCPETools` + `orchestrator_tools.go` `registerTools` 末尾调用 | 新 `orchestrator_bedrock_tools.go` `registerBedrockTools(srv)` + `registerTools` 末尾追一行（**漏调不报错、工具静默不暴露**） |
| handler/param | `orchestrator_mcpe_tools.go:43-119` | camelCase json + jsonschema tag（不以 `词=` 开头）；`textResult/jsonResult` 复用；nil 兜底文本 |
| Deps 窄接口 | `orchestrator.go:66-87` `MCPE`/`Bridge` interface + Deps 字段 | `type Bedrock interface{...}` + `Deps.Bedrock`（nil 注释）；`*bedrock.Service` 结构性满足 |
| service 形状 | `mcpe/service.go:35-62` `New(holder)`+SetEmitter | `bedrock/service.go`：查询引擎本身**无状态、无需 holder/storage**（BedrockQueryService 是 Default）。**仅 OTA 需 holder**（写 OTA 状态表/manifest 记录）；纯查询服务可不持 holder。`logx.For("bedrock")` |
| app 装配 | `app.go:119-120,144,162-163,201-208,240` | 预创建 `bedrockSvc := bedrock.New(...)` → 注入 `orchestrator.Deps{Bedrock: bedrockSvc}`。**bedrock 查询不进 Wails Services 列表**（无前端绑定）；**OTA service 进 Services 列表**（前端 KB 面板要调）+ SetEmitter(mirrorEmit) 推进度事件 |
| storage 表 | `models_mcpe.go:18-23` init() AllModels | 纯查询**无需表**；**OTA** 若要记同步状态/历史 → `models_bedrock.go` init() append（LabelKey + Clearable 视语义）；否则 manifest 走文件（OTA 目录内） |
| embed + .gitattributes | `mcpe/bridgemod.go:25-32` + `resources/.gitattributes(* -text)` | `bedrock/resources/`（复制 cc-pane 整 `resources/`，~26MB）+ `bedrock/resources/.gitattributes` `* -text`（防 CRLF 损坏 JSON/UI JSON/md 字节，保与 cc-pane 一致） |

**平台分支**：Bedrock 全跨平台（纯算法）；validate_* 扫 addonDir 注意 `filepath` 分隔符，**无需 build tag**。

---

## 2. 拍板记录（2026-06-02）

- **OTA 同步**：✅ **本轮一并做**（用户选）。⇒ `set_data_root`/双源覆盖**必须实现**，缓存用可失效槽（RWMutex+*T，**非** sync.Once）；httpx 须先加 per-call 代理/socks5 入口（前置子任务）；HTML→MD 选 Go 库（动手前小验证）。
- **前端面板**：✅ **全量面板（含 OTA 管理）**——KnowledgeBasePanel：manifest 状态 + 更新/重置/代理对话框 + 进度对话框 + 归档列表。MUI + MVVM + t() + NativeDialogs。
- **工具范围**：✅ **全部 26 个**，按 P6f-3..6 分单元落地，每单元独立可编译 + 独立中文 commit。
- **数据体量**：✅ **全量内嵌**，直接复用 cc-pane `bedrock_query/resources/`（~26MB：api-index 2.28MB + components 19KB + templates 51KB + docs 1982 文件〔mcguide 599/mconline 702/wiki 216/mcdocs 271 .md + ui_templates 191 UI JSON〕+ standards 17）。embed.FS 零拷贝，首次查询才建索引，启动不受影响（task_plan 已记用户「直接拷贝复用」）。
- **默认（已采纳，忠实复刻 cc-pane 优先）**：
  - 搜索 = **纯子串**（无 Levenshtein/BM25），CJK = split whitespace 不分词（§1.3，显式记录交接假设不成立）。
  - review_code = **12 规则**（非 13），纯行扫描无 AST，column 字节偏移（§1.5）。
  - 缓存 = 可失效 RWMutex 槽（因 OTA），丢弃 Rust `Box::leak`（GC 替代，明确简化）。
  - ComponentEntry 用自定义 `UnmarshalJSON` 保 flatten extra（get_component 渲染需要）。
  - validate_* addonDir = **绝对路径、无工作空间限制**（忠实 cc-pane；安全考量见 §4）。
  - bedrock 查询服务**无 storage 表/holder**、**不进 Wails Services**；OTA service 进 Services + 持状态。
  - `cc-memory-mcp` **不做**（P6_PLAN §119 推迟，P6f 仅 `bedrock_query` + `bedrock_doc_sync`）。
  - serde `line:column` 报错文案：Go 自算行列**或**改 Offset 并标注不保证字节一致（实现时定，§4）。

---

## 3. 单元切分（10 单元，每单元自洽可编译 + 独立中文 conventional commit，不 push）

> ⚠️ 改 Go 后端必**完整重启 `wails3 dev`** 才生效；改 embed 资源/前端要重 build。验收：每单元 `go build ./... && go vet && go test`(+前端 `pnpm typecheck`)；查询正确性优先单测（拿真实 API 名/事件名断言），手测由用户在 pane 内让 CLI 调 `bedrock_*` 工具核对（同 P6d/P6e）。

### P6f-1 — embed 资源 + 双源加载 + 数据模型 + loader（只读侧地基）
- `internal/services/bedrock/resources/`（复制 cc-pane 整 `resources/`，~26MB）+ `resources/.gitattributes` `* -text`。
- `bedrock/resources.go`：`//go:embed all:resources` + `fs.Sub` 子 FS。
- `bedrock/source.go`：`userDataRoot`(RWMutex/atomic) + `SetDataRoot`(触发 invalidate) + `readResource`(双源,用户读失败报错) + `walkResourceDir`(目录级替换,容错)。
- `bedrock/models.go`：ApiEntry/Param/ReturnSpec/ComponentEntry(自定义 UnmarshalJSON 保 extra)/TemplateEntry(RawMessage)/GuideEntry。
- `bedrock/loader.go`：四槽可失效缓存(RWMutex+*T) + Apis/Components/Templates/Standards + Invalidate。
- 单测：embed 文件齐全(api≥1000/标准含 `modsdk-expert`)、三索引解析条数、standards stem(含 CJK key)、双源覆盖目录级替换、ComponentEntry extra 保真。
- 验证：`go build/vet/test ./internal/services/bedrock/` + 交叉编译 + gofmt。

### P6f-2 — 索引构建 + 搜索/匹配算法（纯子串）
- `bedrock/index.go`：Indices + build(api/event/header 按 type 拆, by_name lower, by_category 混) + deriveCategory(接口/事件锚点) + scanGuides/parseHeadings(`# `/`## `/`### ` 带空格) + Invalidate + 可失效单例。
- `bedrock/fuzzy.go`：nameScore + guideScore(字段加权,matched*3) + truncateChars(80 rune)。
- `bedrock/search/*.go`：api/event(get/validate/lookup/suggestions, side 语义差异)、component(id/name/desc 叠加)、category(精确原 case→lower 子串)、guide(Fields tokenizer, source 白名单, 输出格式契约)、template(match_ tag/keyword 打分, list_mod_packs)。
- `splitLines` 复刻 `str::lines()` 语义（供 review 复用，**先落地避免重复**）。
- 单测：nameScore 各档+空 query、guideScore 字段加权、排序二级键稳定、CJK 单 token 召回（`自定义`≥5）、deriveCategory 路径解析、category 精确 vs 模糊、side 语义差异、truncate rune。
- 验证：`go build/vet/test` + gofmt。

### P6f-3 — 查询工具 10 + orchestrator 接线 + app 装配（**首个 MCP 手测点**）
- `bedrock/service.go`：BedrockQueryService（无状态委托层）+ search_api/get_api/search_event/get_event/validate_call/api_stats/list_categories/list_apis_by_category/search_component/get_component（输出 Markdown 卡片，逐字对齐 cc-pane 渲染）。
- `orchestrator/orchestrator.go`：Deps 加 `Bedrock` 窄接口（10 方法子集）。
- `orchestrator/orchestrator_bedrock_tools.go`：`registerBedrockTools(srv)`（注册 10 工具，camelCase 参数 + 中文 description + textResult + nil 兜底）+ 在 `registerTools` 末尾追调用。
- `app.go`：`bedrockSvc := bedrock.New(...)` + `orchestrator.Deps{Bedrock: bedrockSvc}`（**不进 Services 列表**）。
- 单测：工具 handler（参数透传/默认值/nil 兜底/错误返文本）、渲染卡片关键串断言。
- 验证：`go build/vet/test`（含原 orchestrator 工具）+ gofmt。orchestrator MCP-only 无 bindings 变更。
- **手测**：pane 内 Claude 调 `bedrock_search_api(name="AddActorAnimation")` / `get_api` / `validate_call` / `search_component` 返回正确卡片。

### P6f-4 — 指南/模板/标准工具 12（含 list_mod_packs）
- `bedrock/search/guide.go`(search/read)、`template.go`(match_/info/file/list_mod_packs)、`bedrock/standards.go`(get_standard/expert_guide/review_rules/optimization_rules/json_spec/help)。
- `orchestrator_bedrock_tools.go`：补 12 工具注册；Deps.Bedrock 接口扩这 12 方法。
- 单测：guide BM25-lite 召回 + 输出格式契约、read maxLines 截断、template match_ tag/keyword 打分、get_standard 精确/模糊/Err 空名、固定 stem 取数(无 `# ` 头)、json_spec netease/modsdk/both 分隔符、list_mod_packs 分组。
- 验证：`go build/vet/test` + gofmt。
- **手测**：`bedrock_search_guide(keyword="自定义 实体")` / `get_standard` / `get_json_spec` / `match_template`。

### P6f-5 — review_code（12 规则）
- `bedrock/review/`：line_index.go(indent_of/strip_comment/docstring_mask/splitLines)、violation.go(Severity/Violation/render_report)、rules/×12（fstring/print_call/type_hints/async_await/walrus/cross_side/engine_factory/fn_import/tick_throttle/block_entity_salt/magic_number/dict_access）、run.go(流水线+排序)。
- `orchestrator_bedrock_tools.go`：补 review_code（code+filename?）；Deps 扩 ReviewCode 方法。
- 单测：每规则触发/不触发用例（拿 cc-pane 测试样本断言）、排序三级键、render_report 计数/格式、column 字节偏移、splitLines 尾随行语义。
- 验证：`go build/vet/test` + gofmt。
- **手测**：`bedrock_review_code(code="msg = f'{x}'")` 报 fstring Critical。

### P6f-6 — validate_*（3 文件系统/JSON 校验器）
- `bedrock/validate/`：report.go(Severity/Issue/render_report)、json_defs.go(4 SCAN_DIRS + C1-C5)、assets.go(textures + A1-A4 + PNG IHDR)、test_schema.go(纯 JSON + steps)。
- `orchestrator_bedrock_tools.go`：补 3 工具；Deps 扩方法。
- 单测：路径校验(绝对/存在/目录)、报告骨架格式、json_defs 5 检查、assets PNG 解析/power-of-two、test_schema VALID/INVALID + `{:?}` 文案、扩展名大小写敏感差异。
- 验证：`go build/vet/test`(临时目录 fixtures)+ gofmt。
- **手测**：`bedrock_validate_test(testJson=...)` / `bedrock_validate_json_definitions(addonDir=<真实包>)`。
- **⇒ 至此 26 工具全部完成（baseline 只读 KB 收口）**。

### P6f-7 — httpx 代理扩展 + OTA 数据层（OTA 前置）
- `internal/utils/httpx`：加「带 proxy/socks5/超时/UA 定制」构造入口（socks5 引 `golang.org/x/net/proxy`），保留单例不破坏现有调用方。
- `internal/services/bedrockdocsync/`：manifest.go(Manifest/SourceInfo tagged + 默认源常量 + effective_*)、progress.go(ProgressEvent tagged)、proxy.go(ProxyConfig from_env + build client + probe HEAD github.com)。
- `internal/events/events.go`：注册 `bedrock-doc-sync:progress`。
- 单测：manifest 序列化字节兼容(snake_case)、proxy 优先级、probe。
- 验证：`go build/vet/test` + gofmt。

### P6f-8 — OTA 爬取引擎（gitea_sync + http_sync + HTML→MD）
- **先精读** cc-pane `gitea_sync.rs` 全文 + `http_sync.rs` 1450 行。
- `bedrockdocsync/gitea.go`：Gitea repo→trees(分页 per_page 1000)→过滤→并发 raw blob（httpx + errgroup+semaphore，部分失败不致命）。
- `bedrockdocsync/httpsync.go`：VuePress Plain/Vuepress 两模式 + HTML→MD（选 Go 库，**动手前小验证输出**）+ 正文 DOM 切片。
- 单测：分页循环/扩展过滤/subdir 前缀边界、HTML→MD 小样本。
- 验证：`go build/vet/test` + gofmt。（live 网络在 P6f-10 手测。）

### P6f-9 — OTA 索引构建器 + service 编排
- **先精读** `build_api_index.rs`(881) + `build_templates_index.rs`(1116) + `service.rs:77-447`。
- `bedrockdocsync/buildapi.go` + `buildtemplates.go`：md/demo 树→`*-index.json`（纯解析无网络）。
- `bedrockdocsync/service.go`：sync(staging→Gitea/VuePress→build→原子换目录 .bak 回滚→`bedrock.SetDataRoot` 清缓存→写 manifest, ctx cancel) + status/reset(归档5份)/set|load_proxy/set|load_sources/list|prune_archives。
- `app.go`：OTA service 进 Services 列表 + SetEmitter(mirrorEmit)（依赖注入 `bedrock.SetDataRoot` 回调解耦）。
- 单测：build 解析器(fixtures)、原子换目录/回滚(临时目录)、reset 归档保留数。
- 验证：`go build/vet/test` + `wails3 generate bindings`(含 OTA service)+ gofmt。

### P6f-10 — OTA 前端知识库面板（全量）→ **OTA live 手测点**
- `frontend/src/.../`（MVVM）：KnowledgeBasePanel（manifest 状态 + 更新按钮 + 进度对话框 + 重置〔NativeDialogs 确认〕 + 代理配置〔测试按钮〕 + 归档列表）+ service(7 方法) + 事件订阅(`bedrock-doc-sync:progress` 合批) + types。
- 铁律：MUI / `theme.palette.foundation.*` / 圆角 12·8 / `*Rounded` / t() + lang 包 / NativeDialogs / 无 localStorage / 路由 keepAlive。
- 验证：`pnpm typecheck`(0) + vitest + `pnpm build` + 后端全绿。
- **手测**：KB 面板 → 配代理(可选)+测试 → 更新 → 进度对话框实时 → 原子换目录 → `bedrock_search_guide` 返回**新同步**内容（验证 set_data_root 生效）→ 重置回基线 → 归档列表。前置：能访问 git.mcplugin.top / mc.163.com（或经代理）。

---

## 4. 风险 / 待澄清（实现中遇到再停下问）

- **serde `line:column` 报错文案**：Go `encoding/json` 只给 `Offset`，json_defs 的 `JSON parse error at line L column C` 无法直接复刻。默认：自算行列复刻文案；若成本高则改 Offset 并在文档标注该报告文本不保证字节级一致（实现时定）。
- **ComponentEntry serde(flatten)**：Go 无内建 → 自定义 `UnmarshalJSON`。需在 P6f-1 确认 get_component 渲染卡片确实读 extra(data_type/properties/values/example)，否则可简化丢弃。
- **find_description 遍历顺序**：Rust serde_json::Map 保插入序，Go map 无序；多 description 子对象时非确定性。实际 addon JSON 通常单 `minecraft:xxx` 子对象，影响小；必要时按 key 排序。
- **validate_* addonDir 安全**：忠实 cc-pane = 绝对路径、无工作空间限制（Agent 可让其 walk 任意目录）。本轮**照搬忠实**（Agent 代用户在其项目内操作）；若后续要收紧到工作空间内再议。
- **OTA httpx 代理前置**：必须先给 httpx 加 per-call 代理/socks5（P6f-7 第一子任务），否则违反「禁裸 net/http」铁律或 OTA 无法走代理。socks5 引 `golang.org/x/net/proxy`。
- **OTA HTML→MD 库差异**：Rust `html2md` → Go `JohannesKaufmann/html-to-markdown`(或同类)，输出未必字节一致，可能影响 build_api_index 解析。P6f-8 动手前小验证。
- **OTA 上游可用性**：git.mcplugin.top(第三方自建 Gitea) + mc.163.com VuePress 站结构是外部不可控依赖；代码已把失败设计成回退 Embedded，Go 须复刻回退语义。P6f-10 live 手测确认可达。
- **OTA Windows 原子换目录**：rename 到已存在目录会失败 → 先 rename 旧目录到 .bak（与 Rust 一致）；跨卷/占用需测。
- **set_data_root 全局状态测试隔离**：Rust 进程级 USER_DATA_ROOT 测试要串行；Go 沿用全局单例有同样坑，建议显式注入 + 测试串行。
- **OTA 工作量**：整体 L 偏 XL（~6200 行）。拆解：数据层 S / gitea M / http_sync(VuePress+HTML→MD) L 高风险 / 编排 M / 两索引构建器各 M / 前端面板 M。P6f-7..10 四单元承载。
- **review_code splitLines**：Rust `str::lines()` vs Go `strings.Split` 尾随空行/`\r` 差异 → P6f-2 先落 `splitLines` 公用函数，review 复用。

---

## 5. Go 移植技术映射补充

- Rust `include_dir!`(~26MB) → Go `//go:embed all:resources` + `fs.Sub(fs,"resources")`（zero-copy 进二进制，首次查询才建索引）。
- Rust `OnceLock<Indices>` + `Box::leak` &'static → **因本轮做 OTA（需 invalidate）**：`RWMutex` 保护的 `*T` 槽位（nil 判定重建），**不用 `sync.Once`**（不可重置）；丢弃 leak（GC 替代）。
- Rust serde `rename`/`default`/`flatten` → Go json tag(`type`→Kind/Ty、`return`→Returns)；缺字段=零值（等价 default）；flatten 需自定义 `UnmarshalJSON`。
- 模糊搜索 → **纯 `strings.Contains/HasPrefix/==` + `sort.SliceStable`**（无 Levenshtein/BM25 库）；CJK = `strings.Fields(ToLower(s))`（无分词器）；预览截断 `[]rune`（非 byte）。
- review_code 12 规则 → 纯字节/字符串扫描 + 缩进状态机（**无 AST**）；column=字节偏移；`splitLines` 复刻 `str::lines()`。
- validate_* → `filepath.IsAbs`+`os.Stat`；PNG IHDR `encoding/binary.BigEndian`；report `strings.Builder` 逐字节对齐。
- 工具注册 → go-sdk `mcp.AddTool(srv,&mcp.Tool{Name,Description},handler)`，camelCase json+jsonschema tag（不以 `词=` 起头），`textResult/jsonResult` 复用，错误返文本（非 Go error）。
- OTA：reqwest → `httpx`（须加代理/socks5 入口）；tokio JoinSet+Semaphore → `errgroup`+`semaphore`；AtomicBool cancel → `context.Context`；mpsc+window event → `events.RegisterEvent`+`app.Event.Emit`；html2md → Go html-to-markdown 库；git2-free（不引 go-git）。

---

## 变更记录
- 2026-06-02：7-agent 精读（`wf_562c8b5e-291`，~580k token）产出本计划；纠偏交接假设（无 BM25/Levenshtein、CJK 不分词、review 12 规则非 13、工具 26 非 29）；拍板 §2（**全做 OTA + 全量前端面板 + 26 工具全做 + 全量内嵌**）；待最终过目后进 P6f-1。
