package events

import "github.com/wailsapp/wails/v3/pkg/application"

// Names 集中声明前后端共享的事件名，避免硬编码字符串散落各处。
const (
	// 终端会话事件（后端 PTY reader/wait goroutine → 前端 xterm.js）。
	// payload 见下方对应 *Payload 类型；前端按 sessionId 分发到对应面板。
	TerminalOutput = "terminal:output" // PTY 原始字节（批量）
	TerminalStatus = "terminal:status" // 会话状态变更（active/idle/...）
	TerminalExit   = "terminal:exit"   // 子进程退出

	// TerminalAskPending：被启动 CLI 的 PreToolUse(AskUserQuestion) hook 经 REST 上报「待答问题」，
	// orchestrator 转发给前端卡片，使其在该问题落盘转录之前就能渲染可点选项并作答（点选→写 PTY 按键）。
	// payload 见 TerminalAskPendingPayload；前端按 sessionId 分发。仅 XK 启动的 claude/reclaude 会话有此事件。
	TerminalAskPending = "terminal:ask-pending"

	// Local History 自动捕获事件（后端 lhwatcher 保存版本成功 → 前端历史页 live 刷新）。
	// payload 见 LocalHistoryChangedPayload；前端按 projectPath 过滤后重拉当前列表。
	LocalHistoryChanged = "localhistory:changed"

	// Orchestrator 编排事件（后端 MCP launch_task → 前端开新面板附着已建会话）。
	// payload 见 OrchestratorLaunchPayload；前端 listener 见 P6c-3。
	OrchestratorLaunch = "orchestrator:launch"

	// cli-hook 回报事件（被启动 CLI 的 SessionStart hook → POST /api/terminal/session-started
	// → orchestrator 转发）。payload 见 OrchestratorSessionStartedPayload。P6d-3：本轮仅
	// log + emit（XK 暂无 LaunchCwd 消费方，不回填 launch_history；前端订阅留后续）。
	OrchestratorSessionStarted = "orchestrator:session-started"

	// MC PE 运行事件（后端 launcher PTY reader/wait goroutine → 前端 MC PE 面板）。P6e。
	// payload 见 MCOutputPayload / MCLaunchedPayload / MCExitedPayload；前端按 runId 分发。
	MCPeOutput   = "mc_pe:output"   // 一行已过滤/分类的 MC PE 日志
	MCPeLaunched = "mc_pe:launched" // 一次启动已建 run
	MCPeExited   = "mc_pe:exited"   // MC PE 进程退出

	// Bedrock 知识库 OTA 同步进度（后端 bedrockdocsync.sync → 前端知识库面板进度对话框）。P6f。
	// payload 见 BedrockDocSyncProgressPayload，按 stage 分发；对齐 cc-pane ProgressEvent(tag=stage)。
	BedrockDocSyncProgress = "bedrock-doc-sync:progress"

	// NavigateSettings 托盘「设置」菜单 → 主窗口前端导航到设置页（桌面托盘专属，无 payload，
	// 不进 Web 镜像白名单）。前端 AppLayout 经 useTrayNavigation 监听后 navigate('settings')。
	NavigateSettings = "app:navigate-settings"
)

// TerminalOutputPayload 是 TerminalOutput 事件载荷。
// Data 是 PTY 原始输出（含 ANSI / 控制序列），前端直接喂 xterm.write()。
// Seq 是该会话内单调递增的 chunk 序号（P7 M4）：attach 重放快照带 lastSeq，客户端据 seq
// 丢弃 live 中已被重放覆盖的 chunk（seq ≤ lastSeq）、写入新 chunk（seq > lastSeq），不漏不重。
type TerminalOutputPayload struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
	Seq       uint64 `json:"seq"`
}

// TerminalStatusPayload 是 TerminalStatus 事件载荷。
type TerminalStatusPayload struct {
	SessionID    string `json:"sessionId"`
	Status       string `json:"status"`       // "active" | "idle" | "waiting_input" | "exited"
	LastOutputAt int64  `json:"lastOutputAt"` // unix milli
	PID          int    `json:"pid"`
}

// TerminalExitPayload 是 TerminalExit 事件载荷。
type TerminalExitPayload struct {
	SessionID string `json:"sessionId"`
	ExitCode  int    `json:"exitCode"`
}

// AskOption / AskQuestion 是 AskUserQuestion 工具 input 的结构化形态（字段对齐其 input schema；
// 与 transcript.AskOption/AskQuestion 同形，但作为前后端事件契约的独立 wire 类型，不跨层耦合 transcript 包）。
type AskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type AskQuestion struct {
	Header      string      `json:"header,omitempty"`
	Question    string      `json:"question"`
	MultiSelect bool        `json:"multiSelect,omitempty"`
	Options     []AskOption `json:"options"`
}

// TerminalAskPendingPayload 是 TerminalAskPending 事件载荷。SessionID = PTY 会话 id（前端据此分发）；
// Questions = 待答问题（PreToolUse hook 从 tool_input 透传、orchestrator 反序列化）。
type TerminalAskPendingPayload struct {
	SessionID string        `json:"sessionId"`
	Questions []AskQuestion `json:"questions"`
}

// LocalHistoryChangedPayload 是 LocalHistoryChanged 事件载荷。
// 由 lhwatcher 在 debounce 窗口内成功保存若干版本后发出。
type LocalHistoryChangedPayload struct {
	ProjectPath  string   `json:"projectPath"`  // 发生变更的项目根（前端按此过滤）
	FilePaths    []string `json:"filePaths"`    // 本批保存了新版本的相对路径
	VersionCount int      `json:"versionCount"` // 本批保存的版本数（= len(FilePaths)）
	Timestamp    int64    `json:"timestamp"`    // unix milli
}

// OrchestratorLaunchPayload 是 OrchestratorLaunch 事件载荷。
// 后端 launch_task 已创建 PTY 会话后发出，前端据此开新面板并附着 SessionID。
type OrchestratorLaunchPayload struct {
	TaskID        string `json:"taskId"`
	SessionID     string `json:"sessionId"`
	ProjectPath   string `json:"projectPath"`
	WorkspaceName string `json:"workspaceName,omitempty"`
	ProviderID    string `json:"providerId,omitempty"`
	Title         string `json:"title,omitempty"`
	ResumeID      string `json:"resumeId,omitempty"`
	CliTool       string `json:"cliTool,omitempty"`
}

// OrchestratorSessionStartedPayload 是 OrchestratorSessionStarted 事件载荷。
// 由被启动 CLI 的 session-start hook 经 REST 上报，orchestrator 据此转发给前端。
type OrchestratorSessionStartedPayload struct {
	PtySessionID    string `json:"ptySessionId"`
	ResumeSessionID string `json:"resumeSessionId,omitempty"`
	CliTool         string `json:"cliTool,omitempty"`
	RuntimeKind     string `json:"runtimeKind,omitempty"`
	Cwd             string `json:"cwd,omitempty"`
}

// MCOutputPayload 是 MCPeOutput 事件载荷。Timestamp/Category/Level 为 nil 时序列化成
// JSON null（对齐 cc-pane emit 行为，前端按可选字段处理）。
type MCOutputPayload struct {
	RunID     string  `json:"runId"`
	PackageID string  `json:"packageId"`
	Line      string  `json:"line"`
	Severity  string  `json:"severity"` // error|warn|success|info|verbose|developer|plain
	Timestamp *string `json:"timestamp"`
	Category  *string `json:"category"`
	Level     *string `json:"level"`
}

// MCLaunchedPayload 是 MCPeLaunched 事件载荷。
type MCLaunchedPayload struct {
	RunID     string `json:"runId"`
	PackageID string `json:"packageId"`
}

// MCExitedPayload 是 MCPeExited 事件载荷。
type MCExitedPayload struct {
	RunID     string `json:"runId"`
	PackageID string `json:"packageId"`
	Code      int    `json:"code"`
}

// BedrockDocSyncProgressPayload 是 BedrockDocSyncProgress 事件载荷（扁平化 cc-pane 的
// tagged ProgressEvent：Stage 是判别字段，其余按 stage 取用）。前端按 Stage 分发：
// started/gitea_sync/http_fetch/build_index/validating/completed/cancelled/failed/log。
type BedrockDocSyncProgressPayload struct {
	Stage        string   `json:"stage"`
	Sources      []string `json:"sources,omitempty"`      // started
	Source       string   `json:"source,omitempty"`       // gitea_sync / http_fetch
	Status       string   `json:"status,omitempty"`       // gitea_sync
	Fetched      uint32   `json:"fetched,omitempty"`      // http_fetch
	Total        uint32   `json:"total,omitempty"`        // http_fetch
	Kind         string   `json:"kind,omitempty"`         // build_index
	Entries      uint32   `json:"entries,omitempty"`      // build_index
	ManifestPath string   `json:"manifestPath,omitempty"` // completed
	DurationMs   uint64   `json:"durationMs,omitempty"`   // completed
	Error        string   `json:"error,omitempty"`        // failed
	Partial      bool     `json:"partial,omitempty"`      // failed
	Level        string   `json:"level,omitempty"`        // log（info/warn/error）
	Msg          string   `json:"msg,omitempty"`          // log
}

// Register 在 application 初始化前注册所有强类型事件，
// Wails 的 binding 生成器会据此为前端生成类型化 API。
func Register() {
	application.RegisterEvent[TerminalOutputPayload](TerminalOutput)
	application.RegisterEvent[TerminalStatusPayload](TerminalStatus)
	application.RegisterEvent[TerminalExitPayload](TerminalExit)
	application.RegisterEvent[TerminalAskPendingPayload](TerminalAskPending)
	application.RegisterEvent[LocalHistoryChangedPayload](LocalHistoryChanged)
	application.RegisterEvent[OrchestratorLaunchPayload](OrchestratorLaunch)
	application.RegisterEvent[OrchestratorSessionStartedPayload](OrchestratorSessionStarted)
	application.RegisterEvent[MCOutputPayload](MCPeOutput)
	application.RegisterEvent[MCLaunchedPayload](MCPeLaunched)
	application.RegisterEvent[MCExitedPayload](MCPeExited)
	application.RegisterEvent[BedrockDocSyncProgressPayload](BedrockDocSyncProgress)
}
