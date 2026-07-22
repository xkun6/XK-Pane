// Package sessionscan 扫描本机 CLI Agent 的历史会话文件，供「会话恢复」UI 列举可 resume 的会话。
//
// 移植 cc-pane 的 claude_session_service / codex_session_service 的 **local** 子集，并补 Grok：
//   - Claude：~/.claude/projects/<编码目录>/*.jsonl —— 目录名 = project_path 把所有非字母数字字符换 `-`
//   - Codex： ~/.codex/sessions/**/*.jsonl（递归）—— id+cwd 从首 20 行 session_meta 取
//   - Grok：  ~/.grok/sessions/<URL 编码 cwd>/<session-id>/ —— summary.json 作索引
//
// Claude/Codex 从文件前若干行解析「首条用户 prompt」作 description（截 80 字 + …）；
// Grok 优先用 summary 的 generated_title / session_summary，再回退 chat_history 首条用户句。
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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResumeSession 是一条可恢复的历史会话（claude / codex / grok 统一形态）。
type ResumeSession struct {
	ID          string `json:"id"`
	Source      string `json:"source"`      // "claude" | "codex" | "grok"
	ProjectPath string `json:"projectPath"` // claude=传入目标；codex/grok=会话记录的 cwd
	ModifiedAt  int64  `json:"modifiedAt"`  // 文件 mtime 或 summary 时间（unix 秒）
	FilePath    string `json:"filePath"`
	Description string `json:"description"` // 首条用户 prompt / 标题，截 80 字
}

type Service struct{}

func New() *Service { return &Service{} }

// ListProjectSessions 返回指定项目路径下 claude + codex + grok 的历史会话，按 mtime 倒序。
// limit<=0 表示不限。空结果返回非 nil 空切片（JSON []）。
func (s *Service) ListProjectSessions(_ context.Context, projectPath string, limit int) ([]ResumeSession, error) {
	out := make([]ResumeSession, 0)
	out = append(out, scanClaudeProject(projectPath)...)
	out = append(out, scanCodexProject(projectPath)...)
	out = append(out, scanGrokProject(projectPath)...)

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
//   - codex / grok：无钉死 id，取 cwd 匹配且 afterUnix 之后最新的 session（对齐 cc-pane detect_resume_session）。
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
	case "grok":
		for _, p := range candidatePaths(projectPath, workspacePath) {
			if id := newestAfter(scanGrokProject(p), afterUnix); id != "" {
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

// ── Grok ────────────────────────────────────────────────────────────────────

// encodeGrokCwd 把工作目录编码为 Grok 会话分组目录名：对非 RFC3986 unreserved 字节做
// 百分号编码（大写 hex）。与 Grok 磁盘布局一致：
//   J:\Program\Agent\XK-Pane → J%3A%5CProgram%5CAgent%5CXK-Pane
// 注意：不能用 url.PathEscape（不编码 ':'）也不能直接用 url.QueryEscape（空格变 '+'）。
// 超长路径 Grok 会改用 slug+hash 并写 .cwd；此时靠 walk 匹配 cwd（见 scanGrokProject）。
func encodeGrokCwd(path string) string {
	var b strings.Builder
	b.Grow(len(path) * 3)
	for i := 0; i < len(path); i++ {
		c := path[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// scanGrokProject 扫 ~/.grok/sessions 下与 projectPath 匹配的会话目录。
// 布局：~/.grok/sessions/<encoded-cwd>/<session-id>/{summary.json,chat_history.jsonl,...}
func scanGrokProject(projectPath string) []ResumeSession {
	home := homeDir()
	if home == "" {
		return nil
	}
	root := filepath.Join(home, ".grok", "sessions")
	target := normalizeComparePath(projectPath)

	// 快路径：直接打开 URL 编码后的分组目录（绝大多数短路径命中）。
	if out := scanGrokGroupDir(filepath.Join(root, encodeGrokCwd(projectPath)), projectPath); len(out) > 0 {
		return out
	}
	// 路径分隔符变体再试一次（/ vs \），避免注册路径与 Grok cwd 写法不一致。
	alt := filepath.FromSlash(filepath.ToSlash(projectPath))
	if alt != projectPath {
		if out := scanGrokGroupDir(filepath.Join(root, encodeGrokCwd(alt)), projectPath); len(out) > 0 {
			return out
		}
	}

	// 慢路径：遍历分组目录，用解码名 / .cwd / summary.info.cwd 匹配（覆盖超长路径 hash 布局）。
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []ResumeSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		groupDir := filepath.Join(root, e.Name())
		if !grokGroupMatches(groupDir, e.Name(), target) {
			continue
		}
		out = append(out, scanGrokGroupDir(groupDir, projectPath)...)
	}
	return out
}

// grokGroupMatches 判断分组目录是否对应 target（normalizeComparePath 后的项目路径）。
func grokGroupMatches(groupDir, name, target string) bool {
	// 1) 目录名 URL 解码
	if dec, err := urlPathUnescape(name); err == nil && normalizeComparePath(dec) == target {
		return true
	}
	// 2) 超长路径布局：组内 .cwd 文件写原始路径
	if data, err := os.ReadFile(filepath.Join(groupDir, ".cwd")); err == nil {
		if normalizeComparePath(strings.TrimSpace(string(data))) == target {
			return true
		}
	}
	// 3) 窥探任一 session 的 summary.json info.cwd
	entries, err := os.ReadDir(groupDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sum := readGrokSummary(filepath.Join(groupDir, e.Name(), "summary.json"))
		if sum != nil && normalizeComparePath(sum.Info.Cwd) == target {
			return true
		}
	}
	return false
}

// scanGrokGroupDir 扫描一个 cwd 分组目录下的全部 session 子目录。
func scanGrokGroupDir(groupDir, projectPath string) []ResumeSession {
	entries, err := os.ReadDir(groupDir)
	if err != nil {
		return nil
	}
	var out []ResumeSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionDir := filepath.Join(groupDir, e.Name())
		summaryPath := filepath.Join(sessionDir, "summary.json")
		info, err := os.Stat(summaryPath)
		if err != nil {
			// 无 summary.json 不算可恢复会话（可能是半截目录）。
			continue
		}
		sum := readGrokSummary(summaryPath)
		id := e.Name()
		desc := ""
		modAt := info.ModTime().Unix()
		cwd := projectPath
		if sum != nil {
			if sum.Info.ID != "" {
				id = sum.Info.ID
			}
			if sum.Info.Cwd != "" {
				cwd = sum.Info.Cwd
			}
			if t := parseGrokTime(sum.LastActiveAt); t > 0 {
				modAt = t
			} else if t := parseGrokTime(sum.UpdatedAt); t > 0 {
				modAt = t
			}
			desc = firstNonEmpty(sum.GeneratedTitle, sum.SessionSummary)
		}
		if desc == "" {
			desc = extractGrokDescription(filepath.Join(sessionDir, "chat_history.jsonl"))
		}
		out = append(out, ResumeSession{
			ID:          id,
			Source:      "grok",
			ProjectPath: cwd,
			ModifiedAt:  modAt,
			FilePath:    summaryPath,
			Description: truncateDesc(desc),
		})
	}
	return out
}

// grokSummary 是 ~/.grok/sessions/.../summary.json 的索引子集。
type grokSummary struct {
	Info struct {
		ID  string `json:"id"`
		Cwd string `json:"cwd"`
	} `json:"info"`
	SessionSummary string `json:"session_summary"`
	GeneratedTitle string `json:"generated_title"`
	UpdatedAt      string `json:"updated_at"`
	LastActiveAt   string `json:"last_active_at"`
}

func readGrokSummary(path string) *grokSummary {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s grokSummary
	if json.Unmarshal(data, &s) != nil {
		return nil
	}
	return &s
}

func parseGrokTime(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Grok 写 RFC3339 / RFC3339Nano（带 Z）。
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Unix()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	return 0
}

type grokChatLine struct {
	Type            string          `json:"type"`
	Content         json.RawMessage `json:"content"`
	SyntheticReason string          `json:"synthetic_reason"`
}

// extractGrokDescription 从 chat_history.jsonl 取首条「真实」用户消息文本。
// 跳过 system / 带 synthetic_reason 的注入行 / user_info·git_status·system-reminder 包装。
func extractGrokDescription(path string) string {
	return scanLines(path, 80, func(raw []byte) (string, bool) {
		var ln grokChatLine
		if json.Unmarshal(raw, &ln) != nil || ln.Type != "user" {
			return "", false
		}
		if ln.SyntheticReason != "" {
			return "", false
		}
		text := grokContentText(ln.Content)
		text = strings.TrimSpace(text)
		if text == "" || shouldSkipGrokDesc(text) {
			return "", false
		}
		// <user_query>...</user_query> 取内文（Grok 常见用户消息包装）。
		if inner, ok := extractTagInner(text, "user_query"); ok {
			text = strings.TrimSpace(inner)
			if text == "" {
				return "", false
			}
		}
		return truncateDesc(text), true
	})
}

func grokContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &arr) == nil {
		var parts []string
		for _, it := range arr {
			if it.Type == "text" && it.Text != "" {
				parts = append(parts, it.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func shouldSkipGrokDesc(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) < 3 {
		return true
	}
	// 注入上下文 / 系统提醒，不是用户真实意图。
	if strings.HasPrefix(t, "<user_info>") ||
		strings.HasPrefix(t, "<git_status>") ||
		strings.HasPrefix(t, "<system-reminder>") ||
		strings.HasPrefix(t, "<agent_skills>") ||
		strings.Contains(t, "<system-reminder>") && !strings.Contains(t, "<user_query>") {
		return true
	}
	return false
}

// extractTagInner 取 <tag>...</tag> 内文；找不到返回 ok=false。
func extractTagInner(text, tag string) (string, bool) {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	i := strings.Index(text, open)
	if i < 0 {
		return "", false
	}
	i += len(open)
	j := strings.Index(text[i:], close)
	if j < 0 {
		return "", false
	}
	return text[i : i+j], true
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// urlPathUnescape 解码 %XX（大小写 hex 均可）；非法序列返回 error。
// 仅用于 Grok 分组目录名（不含 '+'→空格 的 form 语义）。
func urlPathUnescape(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' {
			if i+2 >= len(s) {
				return "", fmt.Errorf("truncated percent escape")
			}
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi < 0 || lo < 0 {
				return "", fmt.Errorf("invalid percent escape")
			}
			b.WriteByte(byte(hi<<4 | lo))
			i += 3
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
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
