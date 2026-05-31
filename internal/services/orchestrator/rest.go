package orchestrator

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"xkpane/internal/events"
	"xkpane/internal/services/notification"
	"xkpane/internal/utils/logx"
)

// REST 限流参数（对齐 cc-pane：10s 窗口、≤20）。
const (
	restRateWindowSecs = 10
	restRateMaxReq     = 20
)

// sessionStartedRequest 是 POST /api/terminal/session-started 的请求体（与 cli-hook
// sessionStartedRequest 序列化形态对齐）。偏离 cc-pane：去掉 launch_id/wsl_distro（D6d-1）。
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

// handleSessionStarted 接收 cli-hook 的 session-start 回报：校验 token + 限流 → log + emit
// 前端事件（P6d-3：本轮不回填 launch_history，XK 暂无 LaunchCwd 消费方）。
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

	s.emitEvent(events.OrchestratorSessionStarted, events.OrchestratorSessionStartedPayload{
		PtySessionID:    req.PtySessionID,
		ResumeSessionID: req.ResumeSessionID,
		CliTool:         req.CliTool,
		RuntimeKind:     req.RuntimeKind,
		Cwd:             req.Cwd,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
