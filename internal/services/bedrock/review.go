package bedrock

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// review.go：pe_review_code 引擎框架。对齐 review/{mod,line_index,violation}.rs。
// 全程零正则、零外部依赖：纯行扫描 + 缩进状态机 + 引号态剥注释，无需 Python AST。
// column 是 1-based 字节偏移。12 条规则在 review_rules.go。

// ── Severity / violation（对齐 violation.rs）─────────────────────────────

type severity int

const (
	sevSuggestion severity = 0
	sevWarning    severity = 1
	sevCritical   severity = 2
)

// violation 是单条命中（对齐 violation.rs）。fixHint 为 "" 表示 Rust 的 None
// （所有规则都给非空 fix_hint，故 "" 唯一对应无）。
type violation struct {
	rule     string
	severity severity
	line     int // 1-based，0=整文件
	column   int // 1-based 字节偏移，0=未知
	message  string
	fixHint  string
}

// ── LineIndex（对齐 line_index.rs）──────────────────────────────────────

type lineIndex struct {
	raw   string
	lines []string
}

func newLineIndex(code string) *lineIndex {
	// splitLines 复刻 Rust str::lines()：保中间空行、结尾换行不产尾随空行、剥 \r\n 的 \r。
	return &lineIndex{raw: code, lines: splitLines(code)}
}

// indentOf 数前导缩进列数（空格 1、tab 4，遇其它停）。对齐 line_index.rs:20-30。
func indentOf(line string) int {
	n := 0
	for _, ch := range line {
		switch ch {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// stripComment 剥行尾 `#` 注释（感知单/双引号态 + 反斜杠转义）。对齐 line_index.rs:36-61。
func stripComment(line string) string {
	b := []byte(line)
	i := 0
	inSingle, inDouble := false, false
	for i < len(b) {
		c := b[i]
		if !inSingle && !inDouble && c == '#' {
			return line[:i]
		}
		switch {
		case c == '\\' && (inSingle || inDouble):
			i += 2
			continue
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		}
		i++
	}
	return line
}

var tripleDQ = []byte(`"""`)
var tripleSQ = []byte(`'''`)

// docstringMask 标记每行是否在三引号串内（开闭行及单行 """...""" 也标记）。粗启发式：按源序
// 在每个 """/”' 处 toggle。对齐 line_index.rs:71-116。
func (li *lineIndex) docstringMask() []bool {
	mask := make([]bool, len(li.lines))
	inside := false
	delim := "" // "" = None；否则 `"""` 或 `'''`
	for idx, line := range li.lines {
		startedInside := inside
		sawDelim := false
		localInside := inside
		searchFrom := 0
		b := []byte(line)
		for {
			target := delim
			if target == "" {
				dq := findFrom(b, tripleDQ, searchFrom)
				sq := findFrom(b, tripleSQ, searchFrom)
				switch {
				case dq >= 0 && (sq < 0 || dq <= sq):
					target = `"""`
				case sq >= 0:
					target = `'''`
				}
				if target == "" {
					break // 本行无更多分隔符
				}
			}
			needle := []byte(target)
			pos := findFrom(b, needle, searchFrom)
			if pos < 0 {
				break
			}
			localInside = !localInside
			if localInside {
				delim = target
			} else {
				delim = ""
			}
			searchFrom = pos + len(needle)
			sawDelim = true
		}
		mask[idx] = startedInside || localInside || sawDelim
		inside = localInside
	}
	return mask
}

// findFrom 从 start 起在 haystack 找 needle 的字节偏移，未找到 -1。对齐 line_index.rs:119-127。
func findFrom(haystack, needle []byte, start int) int {
	if len(needle) == 0 || start > len(haystack) {
		return -1
	}
	idx := bytes.Index(haystack[start:], needle)
	if idx < 0 {
		return -1
	}
	return idx + start
}

// ── 流水线（对齐 mod.rs）────────────────────────────────────────────────

type reviewRule func(idx *lineIndex, filename string) []violation

func allRules() []reviewRule {
	return []reviewRule{
		// Critical
		ruleFstring, rulePrintCall, ruleTypeHints, ruleAsyncAwait, ruleWalrus, ruleCrossSide,
		// Warning
		ruleEngineFactory, ruleFnImport, ruleTickThrottle, ruleBlockEntitySalt,
		// Suggestion
		ruleMagicNumber, ruleDictAccess,
	}
}

// reviewCode 对 Python 2.7 源跑 12 条静态规则，返回 Markdown 报告。对齐 review::run（不返错）。
func reviewCode(code, filename string) (string, error) {
	idx := newLineIndex(code)
	var violations []violation
	for _, rule := range allRules() {
		violations = append(violations, rule(idx, filename)...)
	}
	// severity 降、line 升、rule 升 → 确定性输出。
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].severity != violations[j].severity {
			return violations[i].severity > violations[j].severity
		}
		if violations[i].line != violations[j].line {
			return violations[i].line < violations[j].line
		}
		return violations[i].rule < violations[j].rule
	})
	return renderReport(violations, filename, len(idx.lines)), nil
}

func renderReport(violations []violation, filename string, totalLines int) string {
	var critical, warning, suggestion int
	for _, v := range violations {
		switch v.severity {
		case sevCritical:
			critical++
		case sevWarning:
			warning++
		case sevSuggestion:
			suggestion++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Review Report: %s\n", filename)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Lines reviewed: %d\n", totalLines)
	fmt.Fprintf(&b, "Violations: %d (Critical: %d, Warning: %d, Suggestion: %d)\n",
		len(violations), critical, warning, suggestion)
	b.WriteByte('\n')

	if len(violations) == 0 {
		b.WriteString("## No issues found\n")
		return b.String()
	}
	renderReviewSection(&b, "Critical", sevCritical, violations)
	renderReviewSection(&b, "Warning", sevWarning, violations)
	renderReviewSection(&b, "Suggestion", sevSuggestion, violations)
	return b.String()
}

func renderReviewSection(b *strings.Builder, title string, sev severity, all []violation) {
	var bucket []violation
	for _, v := range all {
		if v.severity == sev {
			bucket = append(bucket, v)
		}
	}
	if len(bucket) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n", title)
	b.WriteByte('\n')
	for _, v := range bucket {
		fmt.Fprintf(b, "- line %d col %d: `%s` — %s\n", v.line, v.column, v.rule, v.message)
		if v.fixHint != "" {
			fmt.Fprintf(b, "  Fix: %s\n", v.fixHint)
		}
	}
	b.WriteByte('\n')
}
