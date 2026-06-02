// Package transcriptsvc 把「结构化对话时间线」暴露为 Wails 服务，供桌面（owner）端的卡片视图取数。
//
// 为什么需要：transcript 包是纯文件系统库，webapi 已用它给 Web（viewer）提供 GET /api/sessions/timeline。
// 但桌面前端走 Wails bindings（无 HTTP client），TranscriptService.getTimeline 在桌面原本直接返回
// unsupported。本服务把 webapi handler 的「sessionId→cwd/cliTool/claudeID→定位转录→解析」同一套逻辑
// 包成一个 Wails 方法 GetTimeline，让桌面也能渲染 SessionTimeline 卡片视图（桌面/Web 同源同语义）。
//
// 依赖（与 webapi handler 同源）：SessionLister（活跃会话快照，由 terminal.Service 实现）+
// SessionScanner（按项目扫历史 jsonl 回退，由 sessionscan.Service 实现）。只读 ~/.claude/projects，
// 无 DB / 子进程 / 明文密钥副作用。
package transcriptsvc

import (
	"context"

	"xkpane/internal/services/sessionscan"
	"xkpane/internal/services/terminal"
	"xkpane/internal/services/transcript"
)

// SessionLister 提供活跃会话快照（sessionId → cwd + cliTool + claudeSessionID）。terminal.Service 实现。
type SessionLister interface {
	SessionSnapshots() []terminal.SessionSnapshot
}

// SessionScanner 按项目目录扫可恢复的历史会话（回退定位旧会话转录）。sessionscan.Service 实现。
type SessionScanner interface {
	ListProjectSessions(ctx context.Context, projectPath string, limit int) ([]sessionscan.ResumeSession, error)
}

// Service 是结构化时间线的 Wails 门面。无状态：每次调用即时按 sessionId 解析转录。
type Service struct {
	sessions SessionLister
	scanner  SessionScanner
}

// New 构造 Service。sessions/scanner 由 app 装配处注入（分别是 terminal / sessionscan 服务）。
func New(sessions SessionLister, scanner SessionScanner) *Service {
	return &Service{sessions: sessions, scanner: scanner}
}

// Timeline 是 GetTimeline 的返回：supported=false 表示该会话无结构化转录（非 claude/reclaude），
// 前端回退到 xterm 镜像。events 为 [after, 末尾) 的增量；total 为文件总行数（下次轮询的 after 游标）。
type Timeline struct {
	Supported bool               `json:"supported"`
	Events    []transcript.Event `json:"events"`
	Total     int                `json:"total"`
	Title     string             `json:"title"`
}

// GetTimeline 解析活跃会话的 Claude 转录为结构化时间线（与 webapi handleSessionTimeline 同语义）。
// 仅 cliTool=="claude"/"reclaude" 支持；其余返回 supported=false。会话尚未落转录时返回 supported=true
// 的空时间线（前端继续轮询）。after = 已消费行数游标（增量轮询不漏不重）。
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
	// 仅 claude/reclaude 有结构化转录（reclaude = Claude Code 同包，写同一 ~/.claude/projects）。
	if !found || (cliTool != "claude" && cliTool != "reclaude") {
		return empty, nil
	}
	// 支持但暂无转录（会话刚启动、Claude 尚未落首行）→ 空时间线，前端继续轮询。
	emptySupported := Timeline{Supported: true, Events: []transcript.Event{}}

	file := ""
	if claudeID != "" {
		// 精确映射：已知本会话的 Claude 转录 id（新会话 = --session-id 钉死、恢复 = ResumeID）→ 直接定位。
		// 文件未就绪时返回空并继续轮询，**不回退扫最新**（避免误取同目录里别的并发会话）。
		if p, ok := transcript.ResolveClaudePath(cwd, claudeID); ok {
			file = p
		} else {
			return emptySupported, nil
		}
	} else if s.scanner != nil {
		// 兼容旧会话（未带 --session-id）：回退扫该项目最新 claude jsonl（按 mtime 倒序）。
		rows, err := s.scanner.ListProjectSessions(ctx, cwd, 0)
		if err != nil {
			return emptySupported, err
		}
		for _, row := range rows {
			if row.Source == "claude" {
				file = row.FilePath
				break
			}
		}
	}
	if file == "" {
		return emptySupported, nil
	}
	res := transcript.Parse(file, after)
	return Timeline{Supported: true, Events: res.Events, Total: res.Total, Title: res.Title}, nil
}
