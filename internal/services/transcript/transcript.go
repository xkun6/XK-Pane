// Package transcript 把 Claude Code 的 JSONL 会话转录解析为「结构化时间线事件」，供 Web/手机
// 的结构化重排视图（不镜像 PTY 网格 —— 内容是 user/assistant/thinking/tool 事件，自由重排适配窄屏）。
//
// 为什么从转录切入（而非解析 PTY ANSI）：raw PTY 字节是 Claude 已按某宽度排好版的「绘制指令」
// （绝对光标定位 + alt-screen），与宽度焊死、无法重排；而 ~/.claude/projects/<编码cwd>/<id>.jsonl
// 是排版之前的结构化真相（每个 content block 一行）。详见记忆 xkpane-mobile-strategy。
//
// 纯文件系统只读、无状态：给定转录文件路径 + 行游标 after，逐行解析增量事件。append-only 的
// jsonl 保证行索引稳定，增量轮询只需跳过 < after 的行。文件定位（sessionId → cwd → 最新 jsonl）
// 由调用方（webapi）借 SessionSnapshot.Cwd + sessionscan 完成，本包不关心。
package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveClaudePath 返回 cwd 下指定 Claude session id 的转录文件路径（若存在且为文件）。
// 目录名编码同 Claude CLI（`: \ / _` → `-`，对照 sessionscan）。供「精确映射」：已知会话 id 时
// 直接定位转录，避免同一项目目录下多个并发会话「取最新」的歧义。空入参 / 无 home / 文件不存在返回 ok=false。
func ResolveClaudePath(cwd, sessionID string) (string, bool) {
	if strings.TrimSpace(cwd) == "" || strings.TrimSpace(sessionID) == "" {
		return "", false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	enc := strings.NewReplacer(":", "-", "\\", "-", "/", "-", "_", "-").Replace(cwd)
	p := filepath.Join(home, ".claude", "projects", enc, sessionID+".jsonl")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", false
	}
	return p, true
}

// scanLineCap 单行 jsonl 上限（会话行可能极长，超 bufio 默认 64KB；同 sessionscan）。
const scanLineCap = 16 * 1024 * 1024

// 文本上限（rune）：防超大 payload 拖垮手机渲染。工具结果/输入更短。
const (
	textCap       = 16000
	toolResultCap = 4000
	toolInputCap  = 4000
	askFieldCap   = 2000 // AskUserQuestion 单字段（问题 / 选项标签 / 描述）文本上限（rune）
)

// askToolName 是触发「结构化问题」解析的工具名（卡片渲染可点选项 + 作答）。
const askToolName = "AskUserQuestion"

// Kind 是时间线事件的类别。
const (
	KindUser       = "user"
	KindAssistant  = "assistant"
	KindThinking   = "thinking"
	KindToolUse    = "tool_use"
	KindToolResult = "tool_result"
)

// Event 是一条结构化时间线事件（一个 content block）。
type Event struct {
	Seq       int    `json:"seq"` // 所在行索引（0 基），用于增量游标
	UUID      string `json:"uuid,omitempty"`
	Kind      string `json:"kind"`
	Text      string `json:"text,omitempty"`
	ToolName  string `json:"toolName,omitempty"`
	ToolInput string `json:"toolInput,omitempty"` // 紧凑 JSON 字符串（前端按工具名摘要）
	ToolUseID string `json:"toolUseId,omitempty"`
	IsError   bool   `json:"isError,omitempty"` // 仅 tool_result：工具调用失败（content 含 <tool_use_error>）
	Timestamp string `json:"timestamp,omitempty"`
	// Questions 仅 Kind==KindToolUse 且 ToolName=="AskUserQuestion" 时填充：结构化问题，供卡片渲染
	// 可点选项并把点选翻译成 PTY 按键作答。其余工具此字段为空 → 前端回退裸 JSON（ToolInput）渲染。
	Questions []AskQuestion `json:"questions,omitempty"`
}

// AskOption / AskQuestion 是 AskUserQuestion 工具 input 的结构化形态（字段对齐其 input schema：
// questions[].{question, header, multiSelect, options[].{label, description}}）。
type AskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type AskQuestion struct {
	Header      string      `json:"header,omitempty"`
	Question    string      `json:"question"`
	MultiSelect bool        `json:"multiSelect,omitempty"`
	Options     []AskOption `json:"options"`
}

// Result 是一次解析结果。Total = 文件总行数（= 下次轮询的 after 游标）。
type Result struct {
	Events []Event `json:"events"`
	Total  int     `json:"total"`
	Title  string  `json:"title,omitempty"`
}

type rawLine struct {
	Type      string `json:"type"`
	AITitle   string `json:"aiTitle"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentItem struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Parse 读 path（jsonl），把 [after, 末尾) 的行解析成事件。读不到文件返回空结果（非 nil events）。
// Total 始终为文件实际行数（必须扫全才能给出正确游标），但仅 >= after 的行参与解析。
func Parse(path string, after int) Result {
	res := Result{Events: []Event{}}
	f, err := os.Open(path)
	if err != nil {
		return res
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), scanLineCap)
	idx := -1
	for sc.Scan() {
		idx++
		res.Total = idx + 1
		if idx < after {
			continue // 增量：游标之前的行只计数、不解析
		}
		// Bytes() 在下次 Scan 失效；解析当行即用，无需拷贝。
		parseLine(sc.Bytes(), idx, &res)
	}
	return res
}

func parseLine(raw []byte, idx int, res *Result) {
	var ln rawLine
	if json.Unmarshal(raw, &ln) != nil {
		return
	}
	switch ln.Type {
	case "ai-title":
		if ln.AITitle != "" {
			res.Title = ln.AITitle
		}
	case "user":
		if ln.Message != nil {
			parseUser(ln, idx, res)
		}
	case "assistant":
		if ln.Message != nil {
			parseAssistant(ln, idx, res)
		}
	default:
		// 跳过 permission-mode / file-history-snapshot / last-prompt / attachment / system / summary 等元信息行
	}
}

func parseUser(ln rawLine, idx int, res *Result) {
	// 形态1：content 是字符串（用户 prompt）
	var str string
	if json.Unmarshal(ln.Message.Content, &str) == nil {
		if t := cleanUserText(str); t != "" {
			res.Events = append(res.Events, Event{Seq: idx, UUID: ln.UUID, Kind: KindUser, Text: t, Timestamp: ln.Timestamp})
		}
		return
	}
	// 形态2：content 是数组（文本块 + 工具结果块混排）
	var arr []contentItem
	if json.Unmarshal(ln.Message.Content, &arr) != nil {
		return
	}
	for _, it := range arr {
		switch it.Type {
		case "text":
			if t := cleanUserText(it.Text); t != "" {
				res.Events = append(res.Events, Event{Seq: idx, UUID: ln.UUID, Kind: KindUser, Text: t, Timestamp: ln.Timestamp})
			}
		case "tool_result":
			res.Events = append(res.Events, Event{
				Seq: idx, UUID: ln.UUID, Kind: KindToolResult,
				ToolUseID: it.ToolUseID, Text: extractToolResult(it), IsError: it.IsError, Timestamp: ln.Timestamp,
			})
		}
	}
}

func parseAssistant(ln rawLine, idx int, res *Result) {
	var arr []contentItem
	if json.Unmarshal(ln.Message.Content, &arr) != nil {
		return
	}
	for _, it := range arr {
		switch it.Type {
		case "text":
			if t := strings.TrimSpace(it.Text); t != "" {
				res.Events = append(res.Events, Event{Seq: idx, UUID: ln.UUID, Kind: KindAssistant, Text: capRunes(t, textCap), Timestamp: ln.Timestamp})
			}
		case "thinking":
			if t := strings.TrimSpace(it.Thinking); t != "" {
				res.Events = append(res.Events, Event{Seq: idx, UUID: ln.UUID, Kind: KindThinking, Text: capRunes(t, textCap), Timestamp: ln.Timestamp})
			}
		case "tool_use":
			ev := Event{
				Seq: idx, UUID: ln.UUID, Kind: KindToolUse,
				ToolName: it.Name, ToolInput: capRunes(string(it.Input), toolInputCap), ToolUseID: it.ID, Timestamp: ln.Timestamp,
			}
			// AskUserQuestion 额外结构化解析（从未截断的原始 input，避免长描述被 toolInputCap 砍掉）。
			if it.Name == askToolName {
				ev.Questions = parseAskQuestions(it.Input)
			}
			res.Events = append(res.Events, ev)
		}
	}
}

// parseAskQuestions 把 AskUserQuestion 的 input 解析为结构化问题。解析失败 / 无问题返回 nil
// （前端据此回退到裸 JSON 渲染）。各文本字段按 askFieldCap 截断，防超大 payload 拖垮渲染。
func parseAskQuestions(input json.RawMessage) []AskQuestion {
	if len(input) == 0 {
		return nil
	}
	var parsed struct {
		Questions []struct {
			Question    string `json:"question"`
			Header      string `json:"header"`
			MultiSelect bool   `json:"multiSelect"`
			Options     []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(input, &parsed) != nil {
		return nil
	}
	out := make([]AskQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		opts := make([]AskOption, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, AskOption{
				Label:       capRunes(o.Label, askFieldCap),
				Description: capRunes(o.Description, askFieldCap),
			})
		}
		out = append(out, AskQuestion{
			Header:      capRunes(q.Header, askFieldCap),
			Question:    capRunes(q.Question, askFieldCap),
			MultiSelect: q.MultiSelect,
			Options:     opts,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extractToolResult 把工具结果 content（字符串 / [{type:text,text}] 数组）抽成纯文本（去 ANSI、截断）。
func extractToolResult(it contentItem) string {
	if len(it.Content) == 0 {
		return ""
	}
	var str string
	if json.Unmarshal(it.Content, &str) == nil {
		return capRunes(stripANSI(str), toolResultCap)
	}
	var arr []contentItem
	if json.Unmarshal(it.Content, &arr) == nil {
		var b strings.Builder
		for _, c := range arr {
			if c.Type == "text" {
				b.WriteString(c.Text)
			}
		}
		return capRunes(stripANSI(b.String()), toolResultCap)
	}
	return ""
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func stripANSI(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	return ansiRe.ReplaceAllString(s, "")
}

// cleanUserText 过滤明显的系统注入 / 包装文本（对齐 sessionscan 的取首 prompt 思路）。
func cleanUserText(s string) string {
	t := strings.TrimSpace(s)
	if len([]rune(t)) < 1 {
		return ""
	}
	switch {
	case strings.HasPrefix(t, "[Request interrupted"),
		strings.HasPrefix(t, "Caveat:"),
		strings.Contains(t, "<command-name>"),
		strings.Contains(t, "<local-command-stdout>"),
		strings.Contains(t, "<system-reminder>"),
		strings.Contains(t, "tool_use_id"):
		return ""
	}
	return capRunes(t, textCap)
}

func capRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
