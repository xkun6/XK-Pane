package mcpe

// logfilter.go — MC PE 日志解析与严重度分类。忠实移植 log_filter.rs（emit 部分在 launcher.go）。
//
// 目标：与官方 MC Studio 输出严格对齐 —— PTY 模式下 stdout/stderr 是唯一通道，
// parse 做白名单过滤后才保留。只留 Python/Mod 层日志，拒绝所有 C++ 引擎噪音。

import (
	"strings"
	"unicode"
)

// LogSeverity 序列化值小写（对齐 serde rename_all="lowercase"，前端按此着色）。
type LogSeverity string

const (
	sevError     LogSeverity = "error"
	sevWarn      LogSeverity = "warn"
	sevSuccess   LogSeverity = "success"
	sevInfo      LogSeverity = "info"
	sevVerbose   LogSeverity = "verbose"
	sevDeveloper LogSeverity = "developer" // [Developer] INFO 单独打灰
	sevPlain     LogSeverity = "plain"
)

// ParsedLog 已解析的一条结构化日志。Timestamp/Level/Category 为 "" 表示缺省（对齐 Rust Option::None）。
type ParsedLog struct {
	Display   string // 给前端展示的文本（剥掉 [Python] 前缀后的明文）
	Severity  LogSeverity
	Timestamp string // 如 2026-05-14 13:08:11,772
	Level     string // 如 INFO / ERROR / WARN
	Category  string // 如 Developer / Engine / Scripting
}

const pythonHeader = "[Python]"

func trimStart(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

// parseStdoutLine 解析进程 stdout/stderr 一行 → (ParsedLog, true)，false 表示丢弃。
// 命中以下任一规则才保留：
//  1. Python 时间戳头 [YYYY-MM-DD HH:MM:SS,mmm] [LEVEL][Category?] msg（毫秒前是 ,）
//  2. LoadWindowsAddonPy: 开头（mod 主入口加载）
//  3. ecs_framework  开头（mod ecs 框架 print）
//  4. [<ModName>] 开头（首字母大写、ASCII 字母数字下划线、非引擎黑名单）
//  5. 行有 [Python] 前缀（兜底）
func parseStdoutLine(raw string) (ParsedLog, bool) {
	line := strings.TrimRight(raw, "\r\n")
	if line == "" {
		return ParsedLog{}, false
	}

	// 剥 [Python] 前缀（client 进程 stdout 会被引擎加这个前缀）
	hadPython := false
	body := line
	if rest, ok := strings.CutPrefix(line, pythonHeader); ok {
		hadPython = true
		body = trimStart(rest)
	}

	// 规则 1：Python 时间戳头
	if ts, level, category, message, ok := parsePythonHeader(body); ok {
		// 与 MC Studio 一致：丢弃 [INFO][Engine]（ModSDK 内部 trace），保留 ERROR/WARN/其他类别
		if level == "INFO" && category == "Engine" {
			return ParsedLog{}, false
		}
		// 屏蔽引擎"空错误转储"噪音：ModSDK 的 post_error_dump 每帧轮询，无真实错误时
		// 恒定刷 "Starting error dump" + "Error dump: None" 一对（实测每秒数十次），几秒就
		// 刷爆 cap=2000 环形缓冲，把真实 mod 日志全顶掉。这两行属噪音；真有错误时
		// "Error dump: <堆栈>" 不等于 None，仍照常保留，不影响真实错误暴露。
		if isEngineEmptyDump(level, category, message) {
			return ParsedLog{}, false
		}
		return ParsedLog{
			Display:   body,
			Severity:  detectSeverity(line),
			Timestamp: ts,
			Level:     level,
			Category:  category,
		}, true
	}

	// 规则 2：LoadWindowsAddonPy: 开头
	if strings.HasPrefix(body, "LoadWindowsAddonPy:") {
		return ParsedLog{Display: body, Severity: sevPlain}, true
	}

	// 规则 3：ecs_framework 开头（注意尾部空格）
	if strings.HasPrefix(body, "ecs_framework ") {
		return ParsedLog{Display: body, Severity: sevPlain}, true
	}

	// 规则 4：[<ModName>] 开头
	if tag, ok := leadingModTag(body); ok {
		return ParsedLog{Display: body, Severity: detectSeverity(line), Category: tag}, true
	}

	// 规则 5：兜底 — [Python] 前缀的任何内容
	if hadPython {
		return ParsedLog{Display: body, Severity: detectSeverity(line)}, true
	}

	return ParsedLog{}, false
}

// leadingModTag 提取行首 [<Tag>] 中的 <Tag>，要求首字符 ASCII 大写 + 其余 ASCII 字母数字下划线 + 不在引擎黑名单。
func leadingModTag(body string) (string, bool) {
	rest, ok := strings.CutPrefix(body, "[")
	if !ok {
		return "", false
	}
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return "", false
	}
	tag := rest[:end]
	if tag == "" {
		return "", false
	}
	if tag[0] < 'A' || tag[0] > 'Z' { // 首字符必须 ASCII 大写
		return "", false
	}
	for i := 1; i < len(tag); i++ {
		c := tag[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return "", false
		}
	}
	if isEngineBracketTag(tag) {
		return "", false
	}
	return tag, true
}

// isEngineBracketTag 已知 C++ 引擎 / SDK 方括号前缀黑名单（当前仅 UniSDK）。
func isEngineBracketTag(tag string) bool { return tag == "UniSDK" }

// isEngineEmptyDump 判定引擎"空错误转储"噪音 —— ModSDK 的 post_error_dump 每帧轮询，
// 没有真实错误时恒定刷出 "Starting error dump" + "Error dump: None" 一对（实测每秒数十次，
// 几秒就刷爆 cap=2000 环形缓冲，把真实 mod 日志全顶掉）。仅作用于 [ERROR][Engine]：
// 起始标记行恒定丢弃，"Error dump:" 只在等于 None 时丢弃 —— 真有内容（如堆栈）时仍保留。
func isEngineEmptyDump(level, category, message string) bool {
	if level != "ERROR" || category != "Engine" {
		return false
	}
	msg := strings.TrimSpace(message)
	return msg == "post_error_dump: Starting error dump" || msg == "Error dump: None"
}

// parsePythonHeader 解析 [YYYY-MM-DD HH:MM:SS,mmm] [LEVEL][Category?] message。
// 返回 (timestamp, level, category, message, ok)；category 为 "" 表示无。关键鉴别点：时间戳含 , 分隔毫秒。
func parsePythonHeader(body string) (ts, level, category, message string, ok bool) {
	rest, found := strings.CutPrefix(body, "[")
	if !found {
		return "", "", "", "", false
	}
	tsEnd := strings.IndexByte(rest, ']')
	if tsEnd < 0 {
		return "", "", "", "", false
	}
	timestamp := rest[:tsEnd]
	// 毫秒必须用 , 分隔（区分 C++ 头用 :）
	if !strings.Contains(timestamp, ",") || len(timestamp) < 19 || len(timestamp) > 40 {
		return "", "", "", "", false
	}
	afterTS := trimStart(rest[tsEnd+1:])

	// [LEVEL]
	afterLbr, found := strings.CutPrefix(afterTS, "[")
	if !found {
		return "", "", "", "", false
	}
	levelEnd := strings.IndexByte(afterLbr, ']')
	if levelEnd < 0 {
		return "", "", "", "", false
	}
	lvl := afterLbr[:levelEnd]
	if lvl == "" || strings.Contains(lvl, " ") || len(lvl) > 16 {
		return "", "", "", "", false
	}
	afterLevel := afterLbr[levelEnd+1:]

	// 可选 [Category]（无空格、不含 [）
	cat := ""
	msg := trimStart(afterLevel)
	if afterCbr, hasCat := strings.CutPrefix(afterLevel, "["); hasCat {
		if catEnd := strings.IndexByte(afterCbr, ']'); catEnd >= 0 {
			c := afterCbr[:catEnd]
			if c != "" && !strings.Contains(c, " ") && !strings.Contains(c, "[") {
				cat = c
				msg = trimStart(afterCbr[catEnd+1:])
			}
		}
	}
	return timestamp, lvl, cat, msg, true
}

// detectSeverity 行内严重度推断 — 与 MCStudioHelper getColoredLog 等价（优先级顺序固定）。
func detectSeverity(line string) LogSeverity {
	if strings.Contains(line, "[Developer]") && strings.Contains(line, "INFO") {
		return sevDeveloper
	}
	switch {
	case strings.Contains(line, "ERROR") || strings.Contains(line, "[error]"):
		return sevError
	case strings.Contains(line, "WARN") || strings.Contains(line, "[warning]"):
		return sevWarn
	case strings.Contains(line, "SUC"):
		return sevSuccess
	case strings.Contains(line, "INFO") || strings.Contains(line, "[info]"):
		return sevInfo
	case strings.Contains(line, "VERBOSE"):
		return sevVerbose
	default:
		return sevPlain
	}
}
