package events

import "github.com/wailsapp/wails/v3/pkg/application"

// Names 集中声明前后端共享的事件名，避免硬编码字符串散落各处。
const (
	Time = "time"

	// 终端会话事件（后端 PTY reader/wait goroutine → 前端 xterm.js）。
	// payload 见下方对应 *Payload 类型；前端按 sessionId 分发到对应面板。
	TerminalOutput = "terminal:output" // PTY 原始字节（批量）
	TerminalStatus = "terminal:status" // 会话状态变更（active/idle/...）
	TerminalExit   = "terminal:exit"   // 子进程退出

	// Local History 自动捕获事件（后端 lhwatcher 保存版本成功 → 前端历史页 live 刷新）。
	// payload 见 LocalHistoryChangedPayload；前端按 projectPath 过滤后重拉当前列表。
	LocalHistoryChanged = "localhistory:changed"
)

// TerminalOutputPayload 是 TerminalOutput 事件载荷。
// Data 是 PTY 原始输出（含 ANSI / 控制序列），前端直接喂 xterm.write()。
type TerminalOutputPayload struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
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

// LocalHistoryChangedPayload 是 LocalHistoryChanged 事件载荷。
// 由 lhwatcher 在 debounce 窗口内成功保存若干版本后发出。
type LocalHistoryChangedPayload struct {
	ProjectPath  string   `json:"projectPath"`  // 发生变更的项目根（前端按此过滤）
	FilePaths    []string `json:"filePaths"`    // 本批保存了新版本的相对路径
	VersionCount int      `json:"versionCount"` // 本批保存的版本数（= len(FilePaths)）
	Timestamp    int64    `json:"timestamp"`    // unix milli
}

// Register 在 application 初始化前注册所有强类型事件，
// Wails 的 binding 生成器会据此为前端生成类型化 API。
func Register() {
	application.RegisterEvent[string](Time)
	application.RegisterEvent[TerminalOutputPayload](TerminalOutput)
	application.RegisterEvent[TerminalStatusPayload](TerminalStatus)
	application.RegisterEvent[TerminalExitPayload](TerminalExit)
	application.RegisterEvent[LocalHistoryChangedPayload](LocalHistoryChanged)
}
