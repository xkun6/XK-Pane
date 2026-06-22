package orchestrator

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"xkpane/internal/events"
	"xkpane/internal/services/notification"
	"xkpane/internal/storage"
	"xkpane/internal/utils/logx"
)

// REST 限流参数（对齐 cc-pane：10s 窗口、≤20）。
const (
	restRateWindowSecs = 10
	restRateMaxReq     = 20
)

// sessionStartedRequest 是 POST /api/terminal/session-started 的请求体（与 cli-hook
// sessionStartedRequest 序列化形态对齐）。偏离 cc-pane：去掉 launch_id（D6d-1）。
type sessionStartedRequest struct {
	PtySessionID    string `json:"ptySessionId"`
	ResumeSessionID string `json:"resumeSessionId"`
	CliTool         string `json:"cliTool"`
	RuntimeKind     string `json:"runtimeKind"`
	Cwd             string `json:"cwd"`
}

// verifyToken 比对 Authorization: Bearer（常量时间）。REST handler 各自调用（MCP 走中间件）。
func (s *Service) verifyToken(r *http.Request) bool {
	got := bearer(r.Header.Get("Authorization"))
	want := s.Token()
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// writeJSONError 输出 {error: msg} + 状态码。
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// handleSessionStarted 接收 cli-hook 的 session-start 回报：校验 token + 限流 → log +
// 回包注入同工作空间兄弟项目（不回填 launch_history，XK 暂无 LaunchCwd 消费方）。
func (s *Service) handleSessionStarted(w http.ResponseWriter, r *http.Request) {
	if !s.verifyToken(r) {
		writeJSONError(w, http.StatusUnauthorized, "Invalid or missing Bearer token")
		return
	}
	if !s.rl.allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}

	var req sessionStartedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.PtySessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "ptySessionId required")
		return
	}

	logx.For("orchestrator").Info("session-started 上报",
		"ptySessionId", req.PtySessionID,
		"cliTool", req.CliTool,
		"runtimeKind", req.RuntimeKind,
		"cwd", req.Cwd,
		"hasResumeId", req.ResumeSessionID != "")

	// 据 cwd 反查同工作空间兄弟项目回给 cli-hook，让其注入「说项目名即引用绝对路径」。
	// 仅当前工作空间（同目录隶属多工作空间时合并去重）；查不到 / 不属任何工作空间则省略该字段。
	resp := map[string]any{"ok": true}
	if siblings := s.workspaceSiblings(r.Context(), req.Cwd); len(siblings) > 0 {
		resp["workspaceProjects"] = siblings
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// askPendingRequest 是 POST /api/terminal/ask-pending 的请求体（cli-hook 的 PreToolUse(AskUserQuestion)
// 上报）。ToolInput 透传工具 input（{questions:[...]}），后端只取 questions 转发给前端卡片。
// ToolUseID = 该 AskUserQuestion 的 tool_use id（PreToolUse hook 输入自带）：供前端据「它是否已现身转录」
// 判定待答收尾（作答 / 取消都会让阻塞的轮次落盘 → tool_use 出现 → 该待答失效）。
type askPendingRequest struct {
	PtySessionID string `json:"ptySessionId"`
	ToolUseID    string `json:"toolUseId"`
	ToolInput    struct {
		Questions []events.AskQuestion `json:"questions"`
	} `json:"toolInput"`
}

// PendingAsk 是某会话当前待答的 AskUserQuestion 快照（供 webapi 经 timeline 回灌晚到的 web viewer）。
type PendingAsk struct {
	ToolUseID string               `json:"toolUseId"`
	Questions []events.AskQuestion `json:"questions"`
}

// PendingAsk 返回某会话当前记下的待答 AskUserQuestion（无则 ok=false）。供 webapi timeline 端点回灌。
// 不在此判「是否已答」—— 由前端据 ToolUseID 是否已现身累积转录裁决（后端不重复解析转录）。
func (s *Service) PendingAsk(sessionID string) (PendingAsk, bool) {
	s.pendingAskMu.RLock()
	defer s.pendingAskMu.RUnlock()
	p, ok := s.pendingAsk[sessionID]
	return p, ok
}

// handleAskPending 接收 PreToolUse(AskUserQuestion) 上报：校验 token + 限流 → emit TerminalAskPending。
// 为什么需要旁路：待答的 AskUserQuestion 在终端答完前【不会落进 JSONL 转录】，卡片轮询转录永远看不到
// 待答态、无从渲染/作答。本端点让 hook 在工具执行前把问题推给卡片，卡片即时渲染可点选项、点选写 PTY 按键作答。
func (s *Service) handleAskPending(w http.ResponseWriter, r *http.Request) {
	if !s.verifyToken(r) {
		writeJSONError(w, http.StatusUnauthorized, "Invalid or missing Bearer token")
		return
	}
	if !s.rl.allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}
	var req askPendingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.PtySessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "ptySessionId required")
		return
	}
	// 存一份供 webapi timeline 轮询回灌（晚到的 web viewer 错过下面这条 fire-and-forget 事件后的兜底）。
	s.pendingAskMu.Lock()
	s.pendingAsk[req.PtySessionID] = PendingAsk{ToolUseID: req.ToolUseID, Questions: req.ToolInput.Questions}
	s.pendingAskMu.Unlock()
	s.emitEvent(events.TerminalAskPending, events.TerminalAskPendingPayload{
		SessionID: req.PtySessionID,
		Questions: req.ToolInput.Questions,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleTriggerNotification 接收 cli-hook notify 的触发：校验 token + 限流 → notification.Trigger。
func (s *Service) handleTriggerNotification(w http.ResponseWriter, r *http.Request) {
	if !s.verifyToken(r) {
		writeJSONError(w, http.StatusUnauthorized, "Invalid or missing Bearer token")
		return
	}
	if !s.rl.allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}
	if s.deps.Notifier == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "notification service unavailable")
		return
	}

	var req notification.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := s.deps.Notifier.Trigger(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// siblingProject 是回给 cli-hook 的同工作空间兄弟项目（展示名 + 绝对路径），供其注入上下文，
// 让用户「说项目名」即可被引用到绝对路径。
type siblingProject struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// workspaceSiblings 据 cwd 反查其所属工作空间，返回这些工作空间内【除 cwd 自身外】的项目
// （按归一化路径去重）。同一目录可隶属多个工作空间 → 合并其全部兄弟。cwd 空 / 不属任何
// 工作空间 / 查询失败 → 返回 nil。展示名取 Alias，回退目录名（与 toolListProjects 一致）。
func (s *Service) workspaceSiblings(ctx context.Context, cwd string) []siblingProject {
	if s.deps.Workspaces == nil {
		return nil
	}
	norm := normalizePath(cwd)
	if norm == "" {
		return nil
	}
	wss, err := s.deps.Workspaces.List(ctx)
	if err != nil {
		return nil
	}
	out := []siblingProject{}
	seen := map[string]bool{norm: true} // 排除自身（按归一化路径）
	for _, ws := range wss {
		if !workspaceContains(ws, norm) {
			continue
		}
		for _, p := range ws.Projects {
			n := normalizePath(p.Path)
			if seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, siblingProject{Name: workspaceProjectName(p), Path: p.Path})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// workspaceContains 判定归一化路径 norm 是否为工作空间 ws 的成员项目。
func workspaceContains(ws storage.Workspace, norm string) bool {
	for _, p := range ws.Projects {
		if normalizePath(p.Path) == norm {
			return true
		}
	}
	return false
}

// workspaceProjectName 取工作空间项目展示名：Alias 优先（去空白非空），回退目录名。
func workspaceProjectName(p storage.WorkspaceProject) string {
	if p.Alias != nil {
		if a := strings.TrimSpace(*p.Alias); a != "" {
			return a
		}
	}
	return baseName(p.Path)
}
