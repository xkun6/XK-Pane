package bedrock

import (
	"fmt"
	"strconv"
	"strings"
)

// review_rules.go：12 条静态审查规则。逐字对齐 review/rules/*.rs（含 message/fix_hint 字面量、
// 字节扫描边界、缩进状态机、per-file 上限）。全部纯字节/字符串扫描，无 AST。

// ── ASCII 字节判定（对齐 Rust is_ascii_* 系列）────────────────────────────

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
func isASCIIAlpha(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isASCIIAlnum(b byte) bool { return isASCIIDigit(b) || isASCIIAlpha(b) }

// ── Critical 1：fstring（fstring.rs）─────────────────────────────────────

func ruleFstring(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if col := findFstring(line); col >= 0 {
			out = append(out, violation{
				rule: "fstring", severity: sevCritical, line: i + 1, column: col + 1,
				message: "f-string is not supported in Python 2.7",
				fixHint: `Use "{} {}".format(...) or "%s" formatting instead`,
			})
		}
	}
	return out
}

func findFstring(line string) int {
	b := []byte(line)
	i := 0
	for i+1 < len(b) {
		c := b[i]
		if (c == 'f' || c == 'F') && isTokenStart(b, i) {
			next := b[i+1]
			if next == '"' || next == '\'' {
				return i
			}
		}
		i++
	}
	return -1
}

func isTokenStart(b []byte, pos int) bool {
	if pos == 0 {
		return true
	}
	prev := b[pos-1]
	return !(isASCIIAlnum(prev) || prev == '_')
}

// ── Critical 2：print_call（print_call.rs）──────────────────────────────

func rulePrintCall(idx *lineIndex, _ string) []violation {
	if hasPrintFuture(idx.raw) {
		return nil
	}
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if col := findPrintCall(line); col >= 0 {
			out = append(out, violation{
				rule: "print_call", severity: sevCritical, line: i + 1, column: col + 1,
				message: "print() function call requires 'from __future__ import print_function' in Python 2.7",
				fixHint: "Add `from __future__ import print_function` at the top of the file",
			})
		}
	}
	return out
}

func hasPrintFuture(raw string) bool {
	for _, line := range splitLines(raw) {
		trimmed := strings.TrimLeft(stripComment(line), " \t")
		if strings.HasPrefix(trimmed, "from __future__") && strings.Contains(trimmed, "print_function") {
			return true
		}
	}
	return false
}

func findPrintCall(line string) int {
	b := []byte(line)
	needle := []byte("print")
	if len(b) < len(needle)+1 {
		return -1
	}
	i := 0
	for i+len(needle) < len(b) {
		if string(b[i:i+len(needle)]) == "print" {
			leftOK := i == 0 || !(isASCIIAlnum(b[i-1]) || b[i-1] == '_' || b[i-1] == '.')
			if leftOK {
				j := i + len(needle)
				for j < len(b) && (b[j] == ' ' || b[j] == '\t') {
					j++
				}
				if j < len(b) && b[j] == '(' {
					return i
				}
			}
		}
		i++
	}
	return -1
}

// ── Critical 3：type_hints（type_hints.rs）──────────────────────────────

func ruleTypeHints(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if hasDef(line) && hasParamAnnotation(line) {
			out = append(out, violation{
				rule: "type_hints", severity: sevCritical, line: i + 1, column: 1,
				message: "Type hints are not supported in Python 2.7",
				fixHint: "Remove parameter annotations or move them to a `# type:` comment",
			})
			continue
		}
		if hasReturnAnnotation(line) {
			out = append(out, violation{
				rule: "type_hints", severity: sevCritical, line: i + 1, column: 1,
				message: "Type hints are not supported in Python 2.7",
				fixHint: "Remove the `-> Type` return annotation or use a `# type:` comment",
			})
		}
	}
	return out
}

func hasDef(line string) bool {
	t := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "async def ")
}

func hasParamAnnotation(line string) bool {
	open := strings.IndexByte(line, '(')
	if open < 0 {
		return false
	}
	b := []byte(line)
	depth := 0
	i := open
	lastIdentEnd := -1
	inDefault := false
	for i < len(b) {
		c := b[i]
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				return false
			}
		case c == ',' && depth == 1:
			inDefault = false
			lastIdentEnd = -1
		case c == '=' && depth == 1:
			inDefault = true
			lastIdentEnd = -1
		case c == ':' && depth == 1 && !inDefault:
			if lastIdentEnd >= 0 {
				k := lastIdentEnd
				for k < i && (b[k] == ' ' || b[k] == '\t') {
					k++
				}
				if k == i {
					return true
				}
			}
		case isASCIIAlnum(c) || c == '_':
			lastIdentEnd = i + 1
		}
		i++
	}
	return false
}

func hasReturnAnnotation(line string) bool {
	stripped := strings.TrimRight(line, " \t")
	arrow := strings.Index(stripped, "->")
	if arrow < 0 {
		return false
	}
	prefix := strings.TrimRight(stripped[:arrow], " \t")
	if !strings.HasSuffix(prefix, ")") {
		return false
	}
	after := strings.TrimLeft(stripped[arrow+2:], " \t")
	if after == "" {
		return false
	}
	c := after[0]
	return isASCIIAlpha(c) || c == '_' || c == '\'' || c == '"' || c == '[' || c == '('
}

// ── Critical 4：async_await（async_await.rs）────────────────────────────

func ruleAsyncAwait(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if hasWordKeyword(line, "async") || hasWordKeyword(line, "await") {
			out = append(out, violation{
				rule: "async_await", severity: sevCritical, line: i + 1, column: 1,
				message: "async/await not supported in Python 2.7",
				fixHint: "Refactor to callback-based code; ModSDK uses event handlers, not coroutines",
			})
		}
	}
	return out
}

func hasWordKeyword(line, kw string) bool {
	b := []byte(line)
	kb := []byte(kw)
	if len(b) < len(kb) {
		return false
	}
	i := 0
	for i+len(kb) <= len(b) {
		if string(b[i:i+len(kb)]) == kw {
			leftOK := i == 0 || !(isASCIIAlnum(b[i-1]) || b[i-1] == '_')
			right := i + len(kb)
			rightOK := right >= len(b) || !(isASCIIAlnum(b[right]) || b[right] == '_')
			if leftOK && rightOK {
				return true
			}
		}
		i++
	}
	return false
}

// ── Critical 5：walrus（walrus.rs）──────────────────────────────────────

func ruleWalrus(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if col := findWalrus(line); col >= 0 {
			out = append(out, violation{
				rule: "walrus", severity: sevCritical, line: i + 1, column: col + 1,
				message: "walrus operator ':=' requires Python 3.8+",
				fixHint: "Split into a regular assignment before the conditional",
			})
		}
	}
	return out
}

func findWalrus(line string) int {
	b := []byte(line)
	i := 0
	for i+1 < len(b) {
		if b[i] == ':' && b[i+1] == '=' {
			leftBad := i > 0 && (b[i-1] == ':' || b[i-1] == '!' || b[i-1] == '<' || b[i-1] == '>' || b[i-1] == '=')
			rightBad := i+2 < len(b) && b[i+2] == '='
			if !leftBad && !rightBad {
				return i
			}
		}
		i++
	}
	return -1
}

// ── Critical 6：cross_side（cross_side.rs）──────────────────────────────

func ruleCrossSide(idx *lineIndex, filename string) []violation {
	side, ok := inferSide(idx, filename)
	if !ok {
		return nil
	}
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := strings.TrimLeft(stripComment(raw), " \t")
		other, hasImport := detectImportSide(line)
		if hasImport && other != side {
			kind := "serverApi"
			if other == sideClient {
				kind = "clientApi"
			}
			selfLabel := "ServerSystem"
			if side == sideClient {
				selfLabel = "ClientSystem"
			}
			out = append(out, violation{
				rule: "cross_side", severity: sevCritical, line: i + 1, column: 1,
				message: fmt.Sprintf("cross-side import: %s file must not import %s", selfLabel, kind),
				fixHint: "Communicate between sides via events (NotifyToServer / NotifyToClient)",
			})
		}
	}
	return out
}

type crossSide int

const (
	sideServer crossSide = iota
	sideClient
)

func inferSide(idx *lineIndex, filename string) (crossSide, bool) {
	for _, raw := range idx.lines {
		line := stripComment(raw)
		if containsClassInherit(line, "ServerSystem") {
			return sideServer, true
		}
		if containsClassInherit(line, "ClientSystem") {
			return sideClient, true
		}
	}
	lc := strings.ToLower(filename)
	if strings.Contains(lc, "server") {
		return sideServer, true
	}
	if strings.Contains(lc, "client") {
		return sideClient, true
	}
	return 0, false
}

func containsClassInherit(line, base string) bool {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, "class ") {
		return false
	}
	open := strings.IndexByte(t, '(')
	if open < 0 {
		return false
	}
	closeRel := strings.IndexByte(t[open:], ')')
	if closeRel < 0 {
		return false
	}
	bases := t[open+1 : open+closeRel]
	for _, bb := range strings.Split(bases, ",") {
		bt := strings.TrimSpace(bb)
		if bt == base || strings.HasSuffix(bt, base) {
			return true
		}
	}
	return false
}

func detectImportSide(line string) (crossSide, bool) {
	if rest, ok := strings.CutPrefix(line, "import "); ok {
		rest = strings.TrimLeft(rest, " \t")
		if pathStartsWith(rest, "mod.client") {
			return sideClient, true
		}
		if pathStartsWith(rest, "mod.server") {
			return sideServer, true
		}
	}
	if rest, ok := strings.CutPrefix(line, "from "); ok {
		rest = strings.TrimLeft(rest, " \t")
		if pathStartsWith(rest, "mod.client") {
			return sideClient, true
		}
		if pathStartsWith(rest, "mod.server") {
			return sideServer, true
		}
	}
	return 0, false
}

func pathStartsWith(rest, prefix string) bool {
	if !strings.HasPrefix(rest, prefix) {
		return false
	}
	if len(rest) == len(prefix) {
		return true // None
	}
	switch rest[len(prefix)] {
	case '.', ' ', '\t', ',':
		return true
	case 'A':
		return strings.HasPrefix(rest[len(prefix):], "Api")
	default:
		return false
	}
}

// ── Warning 1：engine_factory（engine_factory.rs）──────────────────────

func ruleEngineFactory(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if !strings.Contains(line, "GetEngineCompFactory()") {
			continue
		}
		if indentOf(line) == 0 {
			continue
		}
		col := strings.Index(line, "GetEngineCompFactory()")
		if col < 0 {
			col = 0
		}
		out = append(out, violation{
			rule: "engine_factory", severity: sevWarning, line: i + 1, column: col + 1,
			message: "GetEngineCompFactory() must be called at module level and cached",
			fixHint: "Move to top-of-file: `CF = serverApi.GetEngineCompFactory()`",
		})
	}
	return out
}

// ── Warning 2：fn_import（fn_import.rs）─────────────────────────────────

type defKind int

const (
	defFunction defKind = iota
	defClass
)

func ruleFnImport(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	type frame struct {
		indent int
		kind   defKind
	}
	var defStack []frame

	for i, raw := range idx.lines {
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := indentOf(line)

		for len(defStack) > 0 && indent <= defStack[len(defStack)-1].indent {
			defStack = defStack[:len(defStack)-1]
		}

		t := strings.TrimLeft(line, " \t")
		var kind defKind
		hasKind := false
		switch {
		case strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "async def "):
			kind, hasKind = defFunction, true
		case strings.HasPrefix(t, "class "):
			kind, hasKind = defClass, true
		}

		insideFunction := false
		for _, f := range defStack {
			if f.kind == defFunction {
				insideFunction = true
				break
			}
		}
		if insideFunction && isImportStmt(t) {
			out = append(out, violation{
				rule: "fn_import", severity: sevWarning, line: i + 1, column: 1,
				message: "import statement inside function/method — move it to module level",
				fixHint: "Move the import to the top of the file",
			})
		}

		if hasKind {
			defStack = append(defStack, frame{indent, kind})
		}
	}
	return out
}

func isImportStmt(t string) bool {
	return strings.HasPrefix(t, "import ") || strings.HasPrefix(t, "from ")
}

// ── Warning 3：tick_throttle（tick_throttle.rs）─────────────────────────

func ruleTickThrottle(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	i := 0
	for i < len(idx.lines) {
		if mask[i] {
			i++
			continue
		}
		line := stripComment(idx.lines[i])
		t := strings.TrimLeft(line, " \t")
		if name, ok := extractDefName(t); ok && isTickLike(name) {
			defIndent := indentOf(line)
			body, end := collectBody(idx, mask, i+1, defIndent, true)
			if len(body) >= 3 && !bodyHasThrottle(body) {
				out = append(out, violation{
					rule: "tick_throttle", severity: sevWarning, line: i + 1, column: 1,
					message: fmt.Sprintf("%s should throttle using a prime modulus (7/11/19/41/79)", name),
					fixHint: "Add a counter: `self._tick = (self._tick+1) % 11; if self._tick: return`",
				})
			}
			i = end
			continue
		}
		i++
	}
	return out
}

func extractDefName(t string) (string, bool) {
	var after string
	if rest, ok := strings.CutPrefix(t, "async def "); ok {
		after = rest
	} else if rest, ok := strings.CutPrefix(t, "def "); ok {
		after = rest
	} else {
		return "", false
	}
	paren := strings.IndexByte(after, '(')
	if paren < 0 {
		return "", false
	}
	return strings.TrimSpace(after[:paren]), true
}

func isTickLike(name string) bool {
	return name == "Tick" || name == "Update" || name == "OnTick" || name == "OnUpdate" ||
		strings.HasSuffix(name, "Tick") || strings.HasSuffix(name, "Update")
}

var throttlePrimes = []int{7, 11, 19, 41, 79}

// collectBody 收集 def 体（indent > defIndent 的非空行）。stripComments=true 用 stripComment 后的
// 行（tick），false 用 raw 行（salt，保留注释里的事件名）。返回 (体, 体后索引)。
func collectBody(idx *lineIndex, mask []bool, start, defIndent int, stripComments bool) ([]string, int) {
	var body []string
	i := start
	for i < len(idx.lines) {
		if mask[i] {
			i++
			continue
		}
		raw := idx.lines[i]
		if strings.TrimSpace(raw) == "" {
			i++
			continue
		}
		if indentOf(raw) <= defIndent {
			break
		}
		if stripComments {
			body = append(body, stripComment(raw))
		} else {
			body = append(body, raw)
		}
		i++
	}
	return body, i
}

func bodyHasThrottle(body []string) bool {
	for _, line := range body {
		for _, p := range throttlePrimes {
			if strings.Contains(line, fmt.Sprintf("%% %d", p)) || strings.Contains(line, fmt.Sprintf("%%%d", p)) {
				return true
			}
		}
		if strings.Contains(line, "if ") && strings.Contains(line, "%") {
			return true
		}
	}
	return false
}

// ── Warning 4：block_entity_salt（block_entity_salt.rs）─────────────────

func ruleBlockEntitySalt(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	i := 0
	for i < len(idx.lines) {
		if mask[i] {
			i++
			continue
		}
		line := stripComment(idx.lines[i])
		t := strings.TrimLeft(line, " \t")
		if !(strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "async def ")) {
			i++
			continue
		}
		defIndent := indentOf(line)
		body, end := collectBody(idx, mask, i+1, defIndent, false) // raw 体（保留注释里的事件名）
		if bodyMentionsEvent(body) && !bodyHasSalt(body) {
			out = append(out, violation{
				rule: "block_entity_salt", severity: sevWarning, line: i + 1, column: 1,
				message: "ServerBlockEntityTickEvent handler needs load-balancing salt (use hash of pos % prime)",
				fixHint: "Use `if hash((x,y,z)) % 11: return` to spread cost across ticks",
			})
		}
		i = end
	}
	return out
}

func bodyMentionsEvent(body []string) bool {
	for _, l := range body {
		if strings.Contains(l, "ServerBlockEntityTickEvent") {
			return true
		}
	}
	return false
}

func bodyHasSalt(body []string) bool {
	for _, line := range body {
		if strings.Contains(line, "hash(") || strings.Contains(line, "salt") {
			return true
		}
		for _, p := range throttlePrimes {
			if strings.Contains(line, fmt.Sprintf("%% %d", p)) || strings.Contains(line, fmt.Sprintf("%%%d", p)) {
				return true
			}
		}
	}
	return false
}

// ── Suggestion 1：magic_number（magic_number.rs，每文件≤3）──────────────

const magicMaxReports = 3

var magicAllowed = map[uint64]bool{
	100: true, 128: true, 200: true, 255: true, 256: true, 360: true,
	500: true, 512: true, 1000: true, 1024: true, 2048: true, 4096: true,
}

type numLit struct {
	col int
	val uint64
}

func ruleMagicNumber(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if len(out) >= magicMaxReports {
			break
		}
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "__version__") || strings.HasPrefix(trimmed, "VERSION") {
			continue
		}
		for _, n := range scanNumbers(line) {
			if n.val >= 100 && !magicAllowed[n.val] {
				out = append(out, violation{
					rule: "magic_number", severity: sevSuggestion, line: i + 1, column: n.col + 1,
					message: fmt.Sprintf("Magic number %d — consider extracting to a named constant", n.val),
					fixHint: "Define `MY_CONSTANT = ...` at the top of the module",
				})
				if len(out) >= magicMaxReports {
					break
				}
			}
		}
	}
	return out
}

func scanNumbers(line string) []numLit {
	b := []byte(line)
	var out []numLit
	i := 0
	for i < len(b) {
		if isASCIIDigit(b[i]) {
			prevBad := i > 0 && (isASCIIAlnum(b[i-1]) || b[i-1] == '_' || b[i-1] == '.')
			start := i
			for i < len(b) && isASCIIDigit(b[i]) {
				i++
			}
			nextBad := i < len(b) && (b[i] == '.' || b[i] == 'e' || b[i] == 'E' || isASCIIAlpha(b[i]) || b[i] == '_')
			if !prevBad && !nextBad {
				if n, err := strconv.ParseUint(line[start:i], 10, 64); err == nil {
					out = append(out, numLit{start, n})
				}
			}
			continue
		}
		i++
	}
	return out
}

// ── Suggestion 2：dict_access（dict_access.rs，每文件≤5）────────────────

const dictMaxReports = 5

var dictSkipPrefixes = []string{"json.loads(", "re.match(", "re.search(", "os.environ["}

func ruleDictAccess(idx *lineIndex, _ string) []violation {
	mask := idx.docstringMask()
	var out []violation
	for i, raw := range idx.lines {
		if len(out) >= dictMaxReports {
			break
		}
		if mask[i] {
			continue
		}
		line := stripComment(raw)
		skip := false
		for _, p := range dictSkipPrefixes {
			if strings.Contains(line, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, col := range scanSubscript(line) {
			out = append(out, violation{
				rule: "dict_access", severity: sevSuggestion, line: i + 1, column: col + 1,
				message: "Direct dict subscript may raise KeyError — consider .get() with default",
				fixHint: "Replace `d['k']` with `d.get('k', default)`",
			})
			if len(out) >= dictMaxReports {
				break
			}
		}
	}
	return out
}

func scanSubscript(line string) []int {
	b := []byte(line)
	var out []int
	i := 0
	for i < len(b) {
		if isIdentStart(b[i]) {
			identStart := i
			for i < len(b) && isIdentContinue(b[i]) {
				i++
			}
			for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
				i++
			}
			if i < len(b) && b[i] == '[' {
				j := i + 1
				for j < len(b) && (b[j] == ' ' || b[j] == '\t') {
					j++
				}
				if j < len(b) && (b[j] == '"' || b[j] == '\'') {
					quote := b[j]
					keyStart := j + 1
					j = keyStart
					for j < len(b) && b[j] != quote {
						if b[j] == '\\' && j+1 < len(b) {
							j += 2
						} else {
							j++
						}
					}
					if j < len(b) && b[j] == quote {
						k := j + 1
						for k < len(b) && (b[k] == ' ' || b[k] == '\t') {
							k++
						}
						if k < len(b) && b[k] == ']' {
							key := line[keyStart:j]
							if key != "" && keyIsIdentLike(key) {
								out = append(out, identStart)
							}
						}
					}
				}
			}
			continue
		}
		i++
	}
	return out
}

func keyIsIdentLike(key string) bool {
	for i := 0; i < len(key); i++ {
		c := key[i]
		if !(isASCIIAlnum(c) || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func isIdentStart(b byte) bool    { return isASCIIAlpha(b) || b == '_' }
func isIdentContinue(b byte) bool { return isASCIIAlnum(b) || b == '_' }
