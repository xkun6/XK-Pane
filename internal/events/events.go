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

// Register 在 application 初始化前注册所有强类型事件，
// Wails 的 binding 生成器会据此为前端生成类型化 API。
func Register() {
	application.RegisterEvent[string](Time)
	application.RegisterEvent[TerminalOutputPayload](TerminalOutput)
	application.RegisterEvent[TerminalStatusPayload](TerminalStatus)
	application.RegisterEvent[TerminalExitPayload](TerminalExit)
}
