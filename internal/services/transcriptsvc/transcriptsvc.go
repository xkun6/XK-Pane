// Package transcriptsvc 把「结构化对话时间线」暴露为 Wails 服务，供桌面（owner）端的卡片视图取数。
//
// 为什么需要：transcript 包是纯文件系统库，webapi 已用它给 Web（viewer）提供 GET /api/sessions/timeline。
// 但桌面前端走 Wails bindings（无 HTTP client），TranscriptService.getTimeline 在桌面原本直接返回
// unsupported。本服务把 webapi handler 的「sessionId→cwd/cliTool/claudeID→定位转录→解析」同一套逻辑
// 包成一个 Wails 方法 GetTimeline，让桌面也能渲染 SessionTimeline 卡片视图（桌面/Web 同源同语义）。
//
// 依赖（与 webapi handler 同源）：terminal.Service（活跃会话快照）+ sessionscan.Service
// （按项目扫历史 jsonl 回退）。只读 ~/.claude/projects，无 DB / 子进程 / 明文密钥副作用。
package transcriptsvc

import (
	"context"

	"xkpane/internal/services/hudstats"
	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/transcript"
)

// Service 是结构化时间线的 Wails 门面。无状态：每次调用即时按 sessionId 解析转录。
type Service struct {
	sessions *terminal.Service
	scanner  *sessionscan.Service
}

// New 构造 Service。sessions/scanner 由 app 装配处注入（分别是 terminal / sessionscan 服务）。
func New(sessions *terminal.Service, scanner *sessionscan.Service) *Service {
	return &Service{sessions: sessions, scanner: scanner}
}

// Timeline 是 GetTimeline 的返回：supported=false 表示该会话无结构化转录（非 claude/reclaude），
// 前端回退到 xterm 镜像。events 为 [after, 末尾) 的增量；total 为文件总行数（下次轮询的 after 游标）。
type Timeline struct {
	Supported bool               `json:"supported"`
	Events    []transcript.Event `json:"events"`
	Total     int                `json:"total"`
	Title     string             `json:"title"`
	// Metrics 是卡片底部状态行的「上下文/5h/周限」指标（复用 claude-hud 落盘产物，见 hudstats）。
	// nil = 暂不可用（claude-hud 未渲染过本会话 / 未开启 external-usage 快照）。前端经 bindings
	// 的 Object.assign 透传，无需 regen（见 TranscriptService 的 metrics 手写接口）。
	Metrics *hudstats.Metrics `json:"metrics,omitempty"`
}

// GetTimeline 解析活跃会话的转录为结构化时间线（与 webapi handleSessionTimeline 同语义）。
// 支持 cliTool=="claude"/"reclaude"（~/.claude/projects）与 "codex"（~/.codex/sessions）；其余返回
// supported=false。会话尚未落转录时返回 supported=true 的空时间线（前端继续轮询）。
// after = 已消费行数游标（增量轮询不漏不重）。codex 无钉死 id，回退扫该项目最新 rollout（按 mtime 倒序）。
func (s *Service) GetTimeline(ctx context.Context, sessionID string, after int) (Timeline, error) {
	empty := Timeline{Events: []transcript.Event{}}
	if s.sessions == nil {
		return empty, nil
	}
	if after < 0 {
		after = 0
	}
	// sessionId → cwd + cliTool + claudeSessionID（活跃会话快照）
	var cwd, cliTool, claudeID string
	found := false
	for _, snap := range s.sessions.SessionSnapshots() {
		if snap.ID == sessionID {
			cwd, cliTool, claudeID, found = snap.Cwd, snap.CliTool, snap.ClaudeSessionID, true
			break
		}
	}
	// claude/reclaude 写 ~/.claude/projects；codex 写 ~/.codex/sessions。其余引擎无结构化转录。
	isClaude := cliTool == "claude" || cliTool == "reclaude"
	isCodex := cliTool == "codex"
	if !found || (!isClaude && !isCodex) {
		return empty, nil
	}
	// 支持但暂无转录（会话刚启动、Agent 尚未落首行）→ 空时间线，前端继续轮询。
	emptySupported := Timeline{Supported: true, Events: []transcript.Event{}}

	// 转录文件定位：claude 已知 id 时精确映射，否则（codex 恒、claude 旧会话）回退扫该项目对应引擎最新 jsonl。
	wantSource := "claude"
	if isCodex {
		wantSource = "codex"
	}
	file := ""
	if isClaude && claudeID != "" {
		// 精确映射：已知本会话的 Claude 转录 id（新会话 = --session-id 钉死、恢复 = ResumeID）→ 直接定位。
		// 文件未就绪时返回空并继续轮询，**不回退扫最新**（避免误取同目录里别的并发会话）。
		if p, ok := transcript.ResolveClaudePath(cwd, claudeID); ok {
			file = p
		} else {
			return emptySupported, nil
		}
	} else if s.scanner != nil {
		// 回退扫该项目最新转录（按 mtime 倒序）：codex 无钉死 id 恒走此路；claude 旧会话（未带 --session-id）兼容。
		rows, err := s.scanner.ListProjectSessions(ctx, cwd, 0)
		if err != nil {
			return emptySupported, err
		}
		for _, row := range rows {
			if row.Source == wantSource {
				file = row.FilePath
				break
			}
		}
	}
	if file == "" {
		return emptySupported, nil
	}

	// codex 与 claude 转录格式不同，分派各自解析器（均产出同一套 transcript.Event）。
	// metrics（上下文/5h/周限）复用 claude-hud 落盘产物，仅 claude 有；codex 传 nil。
	var res transcript.Result
	var metrics *hudstats.Metrics
	if isCodex {
		res = transcript.ParseCodex(file, after)
	} else {
		res = transcript.Parse(file, after)
		metrics = hudstats.Collect(file) // file = 本会话转录原生路径，正是 claude-hud context 缓存的 sha256 入参
	}
	return Timeline{
		Supported: true,
		Events:    res.Events,
		Total:     res.Total,
		Title:     res.Title,
		Metrics:   metrics,
	}, nil
}
