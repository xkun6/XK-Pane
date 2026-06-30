package webapi

// rest.go：远程会话编排 REST（P7 §9 fail-closed 白名单）。所有端点已过 requirePaired 闸门。
// 只挂显式白名单：launch（= LaunchTask）、只读列表（sessions/projects/workspaces）。
// 写文件系统/子进程/明文密钥红线一律不在此注册（§7 铁律 3/4）。M3 为只读镜像，不含
// input/resize/kill（那些 = M5/M6）。

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"xkpane/internal/events"
	"xkpane/internal/services/orchestrator"
	"xkpane/internal/services/hudstats"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/transcript"
	"xkpane/internal/storage"

	"xkpane/internal/utils/logx"
)

// mirroredEvents 是允许转发到浏览器 WS 的事件白名单（§6.1）。只镜像会话级事件，其余
// （time / localhistory:changed / orchestrator:session-started 等本地或无关事件）一律不转发。
var mirroredEvents = map[string]bool{
	events.TerminalOutput:     true,
	events.TerminalStatus:     true,
	events.TerminalExit:       true,
	events.TerminalAskPending: true,
	events.OrchestratorLaunch: true,
}

// ── 下游能力窄接口（*orchestrator.Service / *terminal.Service / *project.Service /
//    *workspace.Service 分别满足）。窄接口便于测试 fake，且明确网关只用到这几样能力。──

// Launcher 启动新会话（公共化的 LaunchTask）。
type Launcher interface {
	LaunchTask(ctx context.Context, p orchestrator.LaunchParams) (orchestrator.LaunchResult, error)
}

// SessionLister 列活跃会话快照（喂 Web 会话列表）。
type SessionLister interface {
	SessionSnapshots() []terminal.SessionSnapshot
}

// ProjectLister / WorkspaceLister 列已注册项目 / 工作空间（供 Web 启动器选合法 projectPath）。
type ProjectLister interface {
	List(ctx context.Context) ([]storage.Project, error)
}
type WorkspaceLister interface {
	List(ctx context.Context) ([]storage.Workspace, error)
}

// SessionScanner 扫盘列举某项目可恢复的历史会话（claude/codex），供 Web「按项目恢复会话」。
// 纯文件系统只读（~/.claude/projects + ~/.codex/sessions），无 DB 副作用、不碰明文密钥。
type SessionScanner interface {
	ListProjectSessions(ctx context.Context, projectPath string, limit int) ([]sessionscan.ResumeSession, error)
}

// SessionReplayer 返回活跃会话的原始 ANSI 重放快照（P7 M4），供 Web attach 时重建 xterm 画面。
type SessionReplayer interface {
	SessionReplay(sessionID string) (terminal.RawReplay, bool)
}

// SessionWriter 把输入字节写入活跃会话的 PTY stdin（P7 M6 双向输入）。写已存在、用户可见的
// 会话 stdin（= 远程键入），非任意进程 spawn；经 requirePaired 闸门，未知/已退出会话报错。
type SessionWriter interface {
	WriteInput(ctx context.Context, sessionID string, data string) error
}

// AskPender 返回某会话当前待答的 AskUserQuestion 快照（*orchestrator.Service 满足）。供 timeline 回灌
// 给晚到 / 重连的 web viewer —— 待答的 ask-pending 事件 fire-and-forget 无重放，且其 tool_use 在阻塞
// 期间不落转录，故唯一可靠恢复途径是经轮询的 timeline 把后端记下的待答带回前端。
type AskPender interface {
	PendingAsk(sessionID string) (orchestrator.PendingAsk, bool)
}

// ServiceDeps 是网关远程端点的下游依赖（app 装配处经 AttachServices 注入）。
type ServiceDeps struct {
	Launcher   Launcher
	Sessions   SessionLister
	Projects   ProjectLister
	Workspaces WorkspaceLister
	Scanner    SessionScanner
	Replayer   SessionReplayer
	Writer     SessionWriter
	AskPend    AskPender
}

// launchRequest 是 POST /api/launch 的请求体（对齐 orchestrator.LaunchParams 的 json 形态）。
type launchRequest struct {
	ProjectPath   string `json:"projectPath"`
	Prompt        string `json:"prompt"`
	ResumeID      string `json:"resumeId"`
	CliTool       string `json:"cliTool"`
	WorkspaceName string `json:"workspaceName"`
	ProviderID    string `json:"providerId"`
	Title         string `json:"title"`
}

// handleLaunch：POST /api/launch → LaunchTask → 广播 orchestrator:launch（两端开窗）。
func (g *Gateway) handleLaunch(w http.ResponseWriter, r *http.Request) {
	deps := g.services()
	if deps.Launcher == nil {
		writeErr(w, http.StatusServiceUnavailable, "launch 暂不可用")
		return
	}
	var req launchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	res, err := deps.Launcher.LaunchTask(r.Context(), orchestrator.LaunchParams{
		ProjectPath:   req.ProjectPath,
		Prompt:        req.Prompt,
		ResumeID:      req.ResumeID,
		CliTool:       req.CliTool,
		WorkspaceName: req.WorkspaceName,
		ProviderID:    req.ProviderID,
		Title:         req.Title,
	})
	if err != nil {
		// LaunchTask 的错误以校验类为主（项目未注册 / prompt 与 resume 互斥）→ 400。
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSessions：GET /api/sessions → 活跃会话快照列表。
func (g *Gateway) handleSessions(w http.ResponseWriter, _ *http.Request) {
	deps := g.services()
	if deps.Sessions == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": deps.Sessions.SessionSnapshots()})
}

// handleSessionsScan：GET /api/sessions/scan?projectPath=&limit= → 该项目可恢复的历史会话
// （扫本机 ~/.claude/projects + ~/.codex/sessions，按 mtime 倒序）。只读，无 DB 副作用。
// 与 GET /api/sessions（活跃会话快照）区分：本端点是磁盘上的历史会话，供「按项目恢复」UI。
func (g *Gateway) handleSessionsScan(w http.ResponseWriter, r *http.Request) {
	deps := g.services()
	if deps.Scanner == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}})
		return
	}
	projectPath := strings.TrimSpace(r.URL.Query().Get("projectPath"))
	if projectPath == "" {
		writeErr(w, http.StatusBadRequest, "缺少 projectPath")
		return
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := deps.Scanner.ListProjectSessions(r.Context(), projectPath, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "扫描会话失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": rows})
}

// inputRequest 是 POST /api/sessions/input 的请求体（远程键入字节，含回车 '\r' / 控制序列）。
type inputRequest struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

// handleSessionInput：POST /api/sessions/input → 把字节写入活跃会话 PTY stdin（P7 M6 双向输入）。
// 写已存在、用户可见会话的 stdin（远程键入），非任意进程 spawn。未知/已退出会话 → 400。
func (g *Gateway) handleSessionInput(w http.ResponseWriter, r *http.Request) {
	deps := g.services()
	if deps.Writer == nil {
		writeErr(w, http.StatusServiceUnavailable, "input 暂不可用")
		return
	}
	var req inputRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		writeErr(w, http.StatusBadRequest, "缺少 sessionId")
		return
	}
	if err := deps.Writer.WriteInput(r.Context(), req.SessionID, req.Data); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSessionReplay：GET /api/sessions/replay?sessionId= → 活跃会话原始 ANSI 重放快照
// （P7 M4，供 attach 重建 xterm 画面，再切 live）。未知/已退出会话返回空重放（200，data=""），
// 让客户端能优雅 attach（无历史画面，仅接后续 live），不报错。
func (g *Gateway) handleSessionReplay(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sid == "" {
		writeErr(w, http.StatusBadRequest, "缺少 sessionId")
		return
	}
	deps := g.services()
	if deps.Replayer == nil {
		writeJSON(w, http.StatusOK, terminal.RawReplay{SessionID: sid})
		return
	}
	rep, _ := deps.Replayer.SessionReplay(sid)
	rep.SessionID = sid
	writeJSON(w, http.StatusOK, rep)
}

// handleSessionTimeline：GET /api/sessions/timeline?sessionId=&after= → 活跃会话的「结构化对话
// 时间线」（从 Claude JSONL 转录解析为 user/assistant/thinking/tool 事件）。供 Web/手机的结构化
// 重排视图：不镜像 PTY 网格、内容自由重排适配窄屏（见记忆 xkpane-mobile-strategy）。
// claude/reclaude（~/.claude/projects）与 codex（~/.codex/sessions）支持；其余引擎 / 裸 shell 返回
// supported=false（前端回退 xterm）。after = 已消费行数游标（增量轮询）。只读转录目录，无 DB / 子进程 /
// 明文密钥副作用。复用已注入的 Sessions（sessionId→cwd+cliTool）与 Scanner（cwd→最新 jsonl），无需新依赖。
func (g *Gateway) handleSessionTimeline(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sid == "" {
		writeErr(w, http.StatusBadRequest, "缺少 sessionId")
		return
	}
	after := 0
	if v := r.URL.Query().Get("after"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			after = n
		}
	}
	unsupported := func() {
		writeJSON(w, http.StatusOK, map[string]any{"supported": false, "events": []any{}, "total": 0})
	}

	deps := g.services()
	if deps.Sessions == nil || deps.Scanner == nil {
		unsupported()
		return
	}
	// sessionId → cwd + cliTool + claudeSessionID（活跃会话快照）
	var cwd, cliTool, claudeID string
	found := false
	for _, snap := range deps.Sessions.SessionSnapshots() {
		if snap.ID == sid {
			cwd, cliTool, claudeID, found = snap.Cwd, snap.CliTool, snap.ClaudeSessionID, true
			break
		}
	}
	// claude/reclaude 写 ~/.claude/projects；codex 写 ~/.codex/sessions。其余引擎无结构化转录。
	isClaude := cliTool == "claude" || cliTool == "reclaude"
	isCodex := cliTool == "codex"
	if !found || (!isClaude && !isCodex) {
		unsupported()
		return
	}
	// 待答 AskUserQuestion 回灌（晚到 / 重连的 web viewer 错过 fire-and-forget 事件后的恢复途径）。
	// 后端只「记得」最近一条；是否仍有效由前端据 toolUseId 是否已现身累积转录裁决（作答 / 取消都让其落盘）。
	pendingAsk := func() any {
		if deps.AskPend == nil {
			return nil
		}
		if p, ok := deps.AskPend.PendingAsk(sid); ok {
			return p
		}
		return nil
	}
	emptyTimeline := func() {
		// 支持但暂无转录（会话刚启动、Claude 尚未落首行）→ 空时间线，前端继续轮询。
		writeJSON(w, http.StatusOK, map[string]any{"supported": true, "events": []any{}, "total": 0, "pending": pendingAsk()})
	}

	// 转录文件定位：claude 已知 id 时精确映射，否则（codex 恒、claude 旧会话）回退扫该项目对应引擎最新 jsonl。
	wantSource := "claude"
	if isCodex {
		wantSource = "codex"
	}
	file := ""
	if isClaude && claudeID != "" {
		// 精确映射：已知本会话的 Claude 转录 id（新会话 = --session-id 钉死、恢复 = ResumeID）→ 直接定位。
		// 文件未就绪时返回空并继续轮询，**不回退扫最新**（避免误取同目录里别的并发会话，如开发 XK-Pane 时
		// 本机另有同目录会话在写）。
		if p, ok := transcript.ResolveClaudePath(cwd, claudeID); ok {
			file = p
		} else {
			emptyTimeline()
			return
		}
	} else {
		// 回退扫该项目最新转录（按 mtime 倒序）：codex 无钉死 id 恒走此路；claude 旧会话（未带 --session-id）兼容。
		rows, err := deps.Scanner.ListProjectSessions(r.Context(), cwd, 0)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "扫描转录失败")
			return
		}
		for _, row := range rows {
			if row.Source == wantSource {
				file = row.FilePath
				break
			}
		}
	}
	if file == "" {
		emptyTimeline()
		return
	}

	// codex 与 claude 转录格式不同，分派各自解析器（均产出同一套 transcript.Event）。
	// metrics（claude-hud 落盘产物）仅 claude 有，codex 为 nil。
	var res transcript.Result
	var metrics any
	if isCodex {
		res = transcript.ParseCodex(file, after)
	} else {
		res = transcript.Parse(file, after)
		metrics = hudstats.Collect(file) // 卡片底部状态行指标（与桌面 transcriptsvc 同源，见 hudstats）
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": true,
		"events":    res.Events,
		"total":     res.Total,
		"title":     res.Title,
		"metrics":   metrics,
		"pending":   pendingAsk(), // 当前待答 AskUserQuestion（回灌晚到的 web viewer），无则 null
	})
}

// handleProjects：GET /api/projects → 已注册项目（供 Web 启动器选 projectPath）。
func (g *Gateway) handleProjects(w http.ResponseWriter, r *http.Request) {
	deps := g.services()
	if deps.Projects == nil {
		writeJSON(w, http.StatusOK, map[string]any{"projects": []any{}})
		return
	}
	rows, err := deps.Projects.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取项目失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": rows})
}

// handleWorkspaces：GET /api/workspaces → 工作空间列表（含其项目）。
func (g *Gateway) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	deps := g.services()
	if deps.Workspaces == nil {
		writeJSON(w, http.StatusOK, map[string]any{"workspaces": []any{}})
		return
	}
	rows, err := deps.Workspaces.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取工作空间失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": rows})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logx.For("webapi").Debug("write json failed", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
