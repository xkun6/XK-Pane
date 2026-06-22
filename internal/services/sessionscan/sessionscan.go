// Package sessionscan 扫描本机 CLI Agent 的历史会话文件，供「会话恢复」UI 列举可 resume 的会话。
//
// 移植 cc-pane 的 claude_session_service / codex_session_service 的 **local** 子集：
//   - Claude：~/.claude/projects/<编码目录>/*.jsonl —— 目录名 = project_path 把所有非字母数字字符换 `-`
//   - Codex： ~/.codex/sessions/**/*.jsonl（递归）—— id+cwd 从首 20 行 session_meta 取
//
// 两者都从文件前若干行解析「首条用户 prompt」作 description（截 80 字 + …）。
//
// **刻意裁剪**（Windows 桌面优先，SSH 子系统未落地）：cc-pane 的
// 远端会话扫描子集不移植，本期只扫本机 local 会话。
//
// **无 DB 依赖**：本 service 纯文件系统扫描、只读，不持 *storage.Holder。
// cc-pane 的「扫盘 backfill 到 launch_history」属轮询增强（P3b 拍板点 9 留后续）；
// 本期「按需扫盘」只把磁盘真相直接返回给前端 SessionSubList。
package sessionscan

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResumeSession 是一条可恢复的历史会话（claude / codex 统一形态）。
type ResumeSession struct {
	ID          string `json:"id"`
	Source      string `json:"source"`      // "claude" | "codex"
	ProjectPath string `json:"projectPath"` // claude=传入目标；codex=session_meta.cwd
	ModifiedAt  int64  `json:"modifiedAt"`  // 文件 mtime（unix 秒）
	FilePath    string `json:"filePath"`
	Description string `json:"description"` // 首条用户 prompt，截 80 字
}

type Service struct{}

func New() *Service { return &Service{} }

// ListProjectSessions 返回指定项目路径下 claude + codex 的历史会话，按 mtime 倒序。
// limit<=0 表示不限。空结果返回非 nil 空切片（JSON []）。
func (s *Service) ListProjectSessions(_ context.Context, projectPath string, limit int) ([]ResumeSession, error) {
	out := make([]ResumeSession, 0)
	out = append(out, scanClaudeProject(projectPath)...)
	out = append(out, scanCodexProject(projectPath)...)

	sort.SliceStable(out, func(i, j int) bool { return out[i].ModifiedAt > out[j].ModifiedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// DetectResumeSession 发现「一次启动」对应的可恢复会话 id（launch_history 回填用）。
//   - claude / reclaude：XK 启动时用 `--session-id <ptyId>` 钉死转录 id，故 pinnedSessionID
//     对应的 <id>.jsonl 一旦落盘即本次启动的确切转录 → 直接返回 pinnedSessionID；尚未命中
//     时回退为 afterUnix 之后最新的转录（兜底极少数 pin 未生效场景）。
//   - codex：无钉死 id，取 cwd 匹配且 afterUnix 之后最新的 session（对齐 cc-pane detect_resume_session）。
//   - 其余引擎 / 裸 shell 返回 ""。
// projectPath 优先、workspacePath 兜底（多目录 CLI 的 --add-dir 场景，对齐 cc-pane paths_to_try）。
func (s *Service) DetectResumeSession(projectPath, workspacePath, cliTool, pinnedSessionID string, afterUnix int64) string {
	switch cliTool {
	case "claude", "reclaude":
		for _, p := range candidatePaths(projectPath, workspacePath) {
			if id := detectClaudeAfter(p, pinnedSessionID, afterUnix); id != "" {
				return id
			}
		}
	case "codex":
		for _, p := range candidatePaths(projectPath, workspacePath) {
			if id := newestAfter(scanCodexProject(p), afterUnix); id != "" {
				return id
			}
		}
	}
	return ""
}

// candidatePaths 返回去重的 [projectPath, workspacePath]（空串与重复剔除）。
func candidatePaths(projectPath, workspacePath string) []string {
	out := make([]string, 0, 2)
	for _, p := range []string{projectPath, workspacePath} {
		if p == "" {
			continue
		}
		dup := false
		for _, e := range out {
			if e == p {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return out
}

// detectClaudeAfter：pinnedSessionID 对应转录已落盘则优先返回它；否则取 afterUnix 后最新转录。
func detectClaudeAfter(projectPath, pinnedSessionID string, afterUnix int64) string {
	sessions := scanClaudeProject(projectPath)
	if pinnedSessionID != "" {
		for _, s := range sessions {
			if s.ID == pinnedSessionID {
				return pinnedSessionID
			}
		}
	}
	return newestAfter(sessions, afterUnix)
}

// newestAfter 取 ModifiedAt >= afterUnix 中最新一条的 id；无则 ""。
func newestAfter(sessions []ResumeSession, afterUnix int64) string {
	id := ""
	var best int64 = -1
	for _, s := range sessions {
		if s.ModifiedAt < afterUnix {
			continue
		}
		if s.ModifiedAt > best {
			best = s.ModifiedAt
			id = s.ID
		}
	}
	return id
}

// scanLineCap 单行 jsonl 上限（会话行可能很长，超 bufio 默认 64KB）。
const scanLineCap = 16 * 1024 * 1024

// homeDir 失败时返回 ""（调用方据空目录早返回空结果）。
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// ── Claude ──────────────────────────────────────────────────────────────────

// encodeClaudeProjectPath 把项目路径编码为 Claude CLI 的目录名：所有**非字母数字**字符
// （含中文 / 空格 / 点 / 盘符冒号 / 路径分隔符 / 下划线等）逐字符替换为 `-`，与 Claude CLI 的
// path.replace(/[^a-zA-Z0-9]/g, '-') 一致。旧实现只替换 `: \ / _`，会漏掉中文等非 ASCII 字符
// → 中文项目目录名（如 红石洲行动 → `------`）对不上、读不到会话历史（对照 cc-pane claude_path.rs:5）。
func encodeClaudeProjectPath(path string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, path)
}

func scanClaudeProject(projectPath string) []ResumeSession {
	home := homeDir()
	if home == "" {
		return nil
	}
	dir := filepath.Join(home, ".claude", "projects", encodeClaudeProjectPath(projectPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // 目录不存在 = 该项目无 claude 会话
	}
	var out []ResumeSession
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".jsonl") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, ResumeSession{
			ID:          strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Source:      "claude",
			ProjectPath: projectPath,
			ModifiedAt:  info.ModTime().Unix(),
			FilePath:    full,
			Description: extractClaudeDescription(full),
		})
	}
	return out
}

type claudeLine struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// extractClaudeDescription 取首条用户消息文本（对照 claude_session_service.rs:23）。
func extractClaudeDescription(path string) string {
	return scanLines(path, 100, func(raw []byte) (string, bool) {
		var ln claudeLine
		if json.Unmarshal(raw, &ln) != nil {
			return "", false
		}
		if ln.Type != "user" || len(ln.Data) > 0 || ln.Message == nil {
			return "", false
		}
		// 情况1：content 是字符串
		var str string
		if json.Unmarshal(ln.Message.Content, &str) == nil {
			if strings.HasPrefix(str, "[Request interrupted") ||
				strings.HasPrefix(str, "Implement the following plan") ||
				len([]rune(str)) < 5 {
				return "", false
			}
			return truncateDesc(str), true
		}
		// 情况2：content 是数组
		var arr []claudeContentItem
		if json.Unmarshal(ln.Message.Content, &arr) == nil {
			for _, item := range arr {
				if item.Type != "text" {
					continue
				}
				if strings.HasPrefix(item.Text, "[Request interrupted") ||
					strings.Contains(item.Text, "tool_use_id") ||
					len([]rune(item.Text)) < 5 {
					continue
				}
				return truncateDesc(item.Text), true
			}
		}
		return "", false
	})
}

// ── Codex ───────────────────────────────────────────────────────────────────

// normalizeComparePath 路径归一比较（对照 codex_session_service.rs:24）：`\`→`/`、去尾`/`、小写。
func normalizeComparePath(path string) string {
	return strings.ToLower(strings.TrimRight(strings.ReplaceAll(path, "\\", "/"), "/"))
}

func scanCodexProject(projectPath string) []ResumeSession {
	home := homeDir()
	if home == "" {
		return nil
	}
	root := filepath.Join(home, ".codex", "sessions")
	target := normalizeComparePath(projectPath)

	var out []ResumeSession
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".jsonl") {
			return nil
		}
		id, cwd, ok := parseCodexMeta(p)
		if !ok || normalizeComparePath(cwd) != target {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, ResumeSession{
			ID:          id,
			Source:      "codex",
			ProjectPath: cwd,
			ModifiedAt:  info.ModTime().Unix(),
			FilePath:    p,
			Description: extractCodexDescription(p),
		})
		return nil
	})
	return out
}

type codexLine struct {
	Type    string `json:"type"`
	Payload struct {
		ID      string              `json:"id"`
		Cwd     string              `json:"cwd"`
		Type    string              `json:"type"`
		Role    string              `json:"role"`
		Content []claudeContentItem `json:"content"`
	} `json:"payload"`
}

// parseCodexMeta 从首 20 行取 session_meta 的 id+cwd（对照 codex_session_service.rs:109）。
func parseCodexMeta(path string) (id, cwd string, ok bool) {
	res := scanLines(path, 20, func(raw []byte) (string, bool) {
		var ln codexLine
		if json.Unmarshal(raw, &ln) != nil || ln.Type != "session_meta" {
			return "", false
		}
		if ln.Payload.ID == "" || ln.Payload.Cwd == "" {
			return "", false
		}
		// 用 \x00 拼回 id+cwd（scanLines 只回单串）。
		return ln.Payload.ID + "\x00" + ln.Payload.Cwd, true
	})
	if res == "" {
		return "", "", false
	}
	parts := strings.SplitN(res, "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// extractCodexDescription 取首条用户 input_text（对照 codex_session_service.rs:67）。
func extractCodexDescription(path string) string {
	return scanLines(path, 200, func(raw []byte) (string, bool) {
		var ln codexLine
		if json.Unmarshal(raw, &ln) != nil {
			return "", false
		}
		if ln.Type != "response_item" || ln.Payload.Type != "message" || ln.Payload.Role != "user" {
			return "", false
		}
		var merged []string
		for _, item := range ln.Payload.Content {
			if item.Type != "input_text" {
				continue
			}
			t := strings.TrimSpace(item.Text)
			if t != "" {
				merged = append(merged, t)
			}
		}
		if len(merged) == 0 {
			return "", false
		}
		text := strings.Join(merged, "\n")
		if shouldSkipCodexDesc(text) {
			return "", false
		}
		return truncateDesc(text), true
	})
}

func shouldSkipCodexDesc(text string) bool {
	t := strings.TrimSpace(text)
	return len(t) < 3 ||
		t == "继续" ||
		strings.EqualFold(t, "continue") ||
		strings.HasPrefix(t, "# AGENTS.md instructions") ||
		strings.Contains(t, "<environment_context>")
}

// ── 共用 ────────────────────────────────────────────────────────────────────

// scanLines 逐行读 path 前 maxLines 行，对每行调 fn；fn 返回 (结果, true) 时立即返回该结果。
// 读不到 / 全不命中返回 ""。
func scanLines(path string, maxLines int, fn func(raw []byte) (string, bool)) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), scanLineCap)
	for i := 0; i < maxLines && sc.Scan(); i++ {
		if res, ok := fn(sc.Bytes()); ok {
			return res
		}
	}
	return ""
}

// truncateDesc 取前 80 个 rune，超出补 …（对照 cc-pane truncate_description）。
func truncateDesc(text string) string {
	r := []rune(text)
	if len(r) <= 80 {
		return string(r)
	}
	return string(r[:80]) + "..."
}
