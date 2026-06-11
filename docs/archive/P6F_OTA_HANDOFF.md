# P6f OTA 收尾交接文档（清空上下文后从这里接手 9b/9c/10）

> 本文件 = P6f OTA 尾段（索引构建器 + service 编排 + 前端面板）的「会话状态 + 下一步」快照，专为
> **清空上下文后全力自动推完 P6f 剩余全部内容**。
> P6f 开工入口见 [P6F_HANDOFF.md](./P6F_HANDOFF.md)（历史，P6f 从零启动用）；签字计划见 `P6f_PLAN.md`；
> 阶段计划 [P6_PLAN.md](./P6_PLAN.md)；逐会话进度 [progress.md](./progress.md)；铁律 [CLAUDE.md](./CLAUDE.md)。
> 全程用**简体中文**回复。
>
> 总目标：把 cc-pane（Tauri+Rust）的 `bedrock_doc_sync` OTA 文档同步子系统忠实移植成 Go
> （Wails v3 + Go 1.25 + React 19 + MUI 9）。

## 已完成（本仓库 master 上的提交，勿重复造）

| 单元 | 内容 | commit |
|------|------|--------|
| P6f-1..6 | Bedrock 知识库 baseline：embed 资源 + 惰性索引 + 26 个 `bedrock_*` 只读 MCP 工具 + 前端 | （见 git log） |
| P6f-7 | httpx 代理扩展 + OTA 数据层（manifest / progress / proxy） | `b1b30e6` |
| P6f-8a | OTA Gitea 爬取引擎（Tree API + 子目录/扩展白名单 + 并发 blob） | `12e4725` |
| P6f-8b | OTA http_sync VuePress/Plain 爬取 + HTML→MD（html-to-markdown/v2） | `0237502` |
| P6f-9a | OTA build_api_index（MD→API 索引） | `2defb08` |

已落地文件在 `internal/services/bedrockdocsync/`：`manifest.go` `gitea.go` `proxy.go` `progress.go`
`http_sync.go` `build_api_index.go` `lines.go`（及各自 `_test.go`）。

## 剩余任务（按序；每个 = 自含 + 可独立编译 + 独立中文 conventional 提交 + **绝不 push**）

- **P6f-9b**：移植 cc-pane `build_templates_index.rs`(1116 行) → `bedrockdocsync/build_templates_index.go`
  （Python 项目扫描：`findProjects` / `extractEvents` / `extractAPIs` / `extractRegisterSystems` /
  `extractModName` / `detectHas*` / `countFiles` / `generateTags`）+ `_test.go`。无新外部依赖。
- **P6f-9c**：移植 `service.rs`(1317 行)+`mod.rs` → `bedrockdocsync/service.go`（sync 编排：discover →
  gitea/http → build index → validate → 原子换目录 → `bedrock.SetDataRoot` 热切 → 写 manifest →
  completed；reset / 归档 / 代理 / 源持久化）；+ `internal/app/app.go` 装配；+ `_test.go`。
  **前置（并入 9c 提交）**：`internal/storage/paths.go` 加导出 `UserDataDir()`；`internal/services/bedrock/`
  加导出 `RuntimeStats()`（返回轻量 struct，避免 bedrock ↔ bedrockdocsync 循环 import）。
- **P6f-10**：前端知识库面板（设置 → 知识库子页）MVVM + MUI + `t()` + NativeDialogs + 订阅
  `events.BedrockDocSyncProgress` 进度。实现完编译 + `pnpm typecheck` 通过后 **【停下，交 OTA live 手测】**。

**每单元 DoD**：原生 `go build ./internal/services/bedrockdocsync/` + `go test` + `go vet` + `gofmt -l` 全过 +
独立提交。

## 源 / 目标路径

- **cc-pane 源**：`J:\Program\Agent\cc-pane\cc-panes-core\src\services\bedrock_doc_sync\`
  （`build_templates_index.rs` / `service.rs` / `mod.rs`；`gitea_sync.rs` / `http_sync.rs` / `manifest.rs` /
  `proxy.rs` / `progress.rs` 已移植完，可参照对齐）
- **XK 目标包**：`J:\Program\Agent\XK-Pane\internal\services\bedrockdocsync\`
- **bedrock 读取侧**：`J:\Program\Agent\XK-Pane\internal\services\bedrock\`
  （`source.go` 有 `SetDataRoot`/`CurrentDataRoot`；`models.go` 是 `ApiEntry`/`TemplateEntry` 读取契约；
  `lines.go` 是 `splitLines` 语义参考）
- **app 装配**：`J:\Program\Agent\XK-Pane\internal\app\app.go`（`mcpeSvc` 的 New+SetEmitter 模式可仿；
  `events.BedrockDocSyncProgress` 已在 `internal/events/events.go` 注册）

## 起步方式（推荐：先并行抽蓝图再亲手移植，对 8b/9a 已两次验证有效）

开 ultracode（`/effort ultracode`）后，用 **Workflow** 对 `build_templates_index.rs`(9b) 和
`service.rs`+`mod.rs`(9c) 各起一个「实现级规格抽取」agent（**整文件读，要逐字字面量 / struct /
状态机 / 排序键，不要摘要**）+ 一个汇总 agent，产出蓝图后据此亲手保真移植。读原文是保真的前提，
蓝图只是导航。

## 保真铁律（务必遵守，避免重复踩坑）

1. **纯 std 风格**：无 `regexp`、无 DOM、无第三方解析器；按字节/按行状态机逐字对齐 Rust。Rust 的
   `str.find()`/`&s[a..b]` 是字节偏移，Go 的 `string` + `strings.Index` 同为字节偏移 —— 全程**不转 `[]rune`**。
2. **行切分**一律用包内已有的 `splitLines()`（`lines.go`，复刻 Rust `str::lines()`）。
3. **索引产物 JSON**（`api-index.json` / `templates-index.json`）的 struct tag 必须 **snake_case**，对齐
   `bedrock/models.go` 读取侧（`type`/`name`/`method`/`side`/`mod_name`/`has_client`/`file_count`...）。
   camelCase 只用于 manifest/proxy/sources 元数据 + 返给前端的 read-model。
4. **序列化**复刻 serde `to_string_pretty`：用包内已有的 `marshalIndentNoEscape()`（`SetEscapeHTML(false)`
   + 2 空格缩进 + 去尾换行）。空集合必须用非 nil 切片 / 初始化 map 渲染成 `[]`/`{}`（不能 `null`）。
   `systems` **不排序不去重**（保遍历序）；`tags`/`events_used`/`apis`/`file_count` 字典序去重。
5. **包内已有可复用**：`containsStr()`、`FetchFailure{Path,Error}`、`splitLines`、`marshalIndentNoEscape`、
   `ProgressSink`、`ProxyConfig/HTTPClient`、`GiteaSyncParams/giteaSync`、`HTTPSyncParams/httpSync`。勿重复定义。
6. **htmlToMarkdown 配置**（`http_sync.go`，table 插件 + `BulletListMarker "*"` + `EscapeModeDisabled`）已经过
   `build_api_index` 真实解析器验证 **【完全正确，勿改】**。
7. **验证只跑原生**：`go build ./internal/services/bedrockdocsync/` + `go test` + `go vet` + `gofmt -l`。
   `bedrockdocsync` 经 `internal/events` 间接依赖 `wails/application`，**无法 linux 交叉编译**（既有限制，非 bug）。

## 已拍板的决策（勿再纠结）

- 默认源列表 = `[mcguide, mconline]`（以 `service.rs` 代码为准，非注释里的三源）。
- bedrock guide 计数 = 全树 `walkResourceDir("data/docs","md")`（复用现成，等价）。
- 原子换目录布局：`<dataDir>/bedrock-data`（live）、`bedrock-data.tmp`（staging）、`bedrock-data.bak`
  （换目录临时备份，事后删）、`bedrock-data.archive-<stamp>`（reset 归档，保留最近 5）。用 `os.Rename` 换目录。
- bedrock 注入用窄接口 `BedrockHooks{SetDataRoot, RuntimeStats}` 回调，`service.go` **不 import bedrock**。
- `manifest.json` / `proxy.json` / `sources.json` 写盘用 `filex.WriteAtomic`；9a/9b 索引产物写 staging 用
  `os.WriteFile`（目录原子换提供原子性）。
- `SetProxy` 单测只测失败路径（**不真连 github.com**）。
- 前置依赖（`storage.UserDataDir()` / `bedrock.RuntimeStats()`）并入 9c 提交。

## 项目铁律（CLAUDE.md，9c/10 相关）

- HTTP 走 `httpx`；子进程走 `procx`；敏感串 `cryptox.EncryptString` 后入库；日志 `logx`（禁 `log.Printf`/
  `fmt.Println`）；配置文件 `filex.WriteAtomic`（禁 `os.WriteFile` 写配置）；跨会话状态只走 SQLite
  （禁 `localStorage`/`sessionStorage`）。
- **前端（P6f-10）**：MVVM（View/ViewModel/Style 三件分离，文件夹/组件）；设置页子目录隔离
  （`SettingsPage/<sub>/` 含 View/VM/Style/lang，文案命名空间 `settings.<sub>.*`）；所有人类可见字符串
  走 `t()`（含 `aria-label`/`placeholder`/Tooltip/对话框）；图标只用 `@mui/icons-material` 的 `*Rounded`；
  取色只从 `theme.palette.foundation.*`；原生对话框只走 `NativeDialogs`（禁 `alert`/`confirm`/`prompt`）；
  组件 → ViewModel → `services/<Domain>Service.ts` → `@bindings/...`（View/VM 不直接 import bindings）。
- 9c 装配新 Wails Service 后须跑 `wails3 generate bindings` 重生成绑定，10 的前端才能调；前端改完
  `cd frontend && pnpm typecheck` 须过。

## 工作方式（用户已授权全自动连推）

拍板后多单元自主连推，逐单元独立提交（**中文 conventional commit**，绝不 push），只在「**手测点 /
阶段边界 / 真外部依赖决策**」停。9b/9c 纯移植无新外部依赖、无新契约待验，直接连推。推到 **P6f-10
实现完 + 前端 typecheck 过后【停下】**，交付 OTA live 手测（用户在桌面 app 设置 → 知识库面板触发一次
同步验证）。另有 4 个 baseline 手测 gate（P6f-3/4/5/6 的 `bedrock_*` 查询工具）是用户自行运行的，
你只需提醒。

---

**触发方式**：新会话只需说「读 P6F_OTA_HANDOFF.md，按它从 P6f-9b 开始」。先抽 9b/9c 蓝图（Workflow
并行深读 `build_templates_index.rs` + `service.rs`），再从 P6f-9b 起逐单元连推；遇任何架构歧义随时问。
