package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// preToolUseInput 是 Claude Code PreToolUse hook 经 stdin 传入的 JSON 子集。
// matcher 已限定 AskUserQuestion，故 tool_input 即该工具的 {questions:[...]}。
type preToolUseInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	ToolUseID string          `json:"tool_use_id"`
}

// askPendingRequest 是回连 orchestrator 的上报体：透传 tool_input（含 questions），由后端取 questions 转发。
// ToolUseID 一并上报：供前端据「它是否已现身转录」判定待答收尾（作答 / 取消都会让阻塞轮次落盘）。
type askPendingRequest struct {
	PtySessionID string          `json:"ptySessionId"`
	ToolUseID    string          `json:"toolUseId"`
	ToolInput    json.RawMessage `json:"toolInput"`
}

// runAskPending 是 PreToolUse(AskUserQuestion) hook 入口：读 stdin 的 tool_input → 回连 orchestrator
// 上报「待答问题」，让卡片在该问题落盘转录【之前】就能渲染可点选项并作答。
//
// 为什么需要：待答的 AskUserQuestion 在终端答完前不会落进 JSONL 转录，卡片轮询转录看不到待答态。
// 本 hook 在工具执行前把问题旁路推给 XK-Pane，补上这个缺口。
//
// 任何失败都静默 exit 0（绝不阻塞工具 / CLI；本 hook 注册为 async，stdout/退出码本就被忽略）。
func runAskPending() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return
	}
	var in preToolUseInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return
	}
	// matcher 应已限定 AskUserQuestion，这里双保险：非该工具直接放弃。
	if in.ToolName != "" && in.ToolName != "AskUserQuestion" {
		return
	}
	if len(in.ToolInput) == 0 {
		return
	}

	pty := strings.TrimSpace(os.Getenv("XKPANE_PTY_SESSION_ID"))
	base := resolveBaseURL()
	token := apiToken()
	if pty == "" || base == "" || token == "" {
		// orchestrator 未注入端口/token（非 XK 启动的会话）→ 静默放弃，绝不阻塞。
		fmt.Fprintln(os.Stderr, "[xkpane-hook] ask-pending: orchestrator 未注入，跳过上报")
		return
	}

	body, err := json.Marshal(askPendingRequest{PtySessionID: pty, ToolUseID: in.ToolUseID, ToolInput: in.ToolInput})
	if err != nil {
		return
	}
	if status, respBody, err := postJSON(base+"/api/terminal/ask-pending", token, body); err != nil {
		fmt.Fprintf(os.Stderr, "[xkpane-hook] ask-pending: 上报失败: %v\n", err)
	} else if status < 200 || status >= 300 {
		fmt.Fprintf(os.Stderr, "[xkpane-hook] ask-pending: HTTP %d: %s\n", status, respBody)
	}
}
