package bedrockdocsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// build_api_index.go：从 mcdocs markdown 树生成 api-index.json（对齐 build_api_index.rs）。
// 纯 std 风格 markdown → JSON 构建器：无 regexp、无 DOM、无 pulldown-cmark，只有按行/按字节状态机。
// 输出形状与 bedrock 包 loader 回读的 ApiEntry 一致（snake_case tag：type/name/method/side/...）。
// 注：输出 JSON tag 必须 snake_case（被 bedrock loader 回读 + 对参考 JSON 比对），非 manifest 的 camelCase。

// OutputAPIEntry 是输出条目形状（对齐 OutputApiEntry，序列化专用，字段顺序严格对齐 serde）。
type OutputAPIEntry struct {
	Kind        string         `json:"type"`
	Name        string         `json:"name"`
	Method      string         `json:"method"`
	Description string         `json:"description"`
	Remark      string         `json:"remark"`
	Side        string         `json:"side"`
	Link        string         `json:"link"`
	Params      []OutputParam  `json:"params"` // 必须非 nil（空时 []，不能 null）
	Returns     []OutputReturn `json:"return"` // 必须非 nil
	Example     string         `json:"example"`
}

// OutputParam 对齐 OutputParam（tag name/type/description）。
type OutputParam struct {
	Name        string `json:"name"`
	Ty          string `json:"type"`
	Description string `json:"description"`
}

// OutputReturn 对齐 OutputReturn（tag type/description）。
type OutputReturn struct {
	Ty          string `json:"type"`
	Description string `json:"description"`
}

// BuildAPIIndexReport 一次构建报告（非序列化）。
type BuildAPIIndexReport struct {
	APICount         uint32
	EventCount       uint32
	HeaderCount      uint32
	Total            uint32
	AgentNotesLoaded uint32
	FilesSkipped     uint32
}

// skipFilesAPI 是要跳过的文件名：索引页，非真正 API 定义（注意 Api 大小写）。
var skipFilesAPI = []string{"索引.md", "Api索引表.md", "事件索引表.md"}

// buildAPIIndex 从 mcdocsDir 的 mcdocs 树在 outputPath 生成 api-index.json（对齐 build）。
// mcdocsDir 应是 data/docs/mcdocs/ 目录（即 1-ModAPI 的父）。仅在致命失败（写不出输出等）时返 error；
// 单个 markdown 解析失败记进 FilesSkipped 并继续。
func buildAPIIndex(mcdocsDir, outputPath string) (BuildAPIIndexReport, error) {
	apiDir := filepath.Join(mcdocsDir, "1-ModAPI", "接口")
	eventDir := filepath.Join(mcdocsDir, "1-ModAPI", "事件")

	entries := buildHeaderEntries()
	headerCount := uint32(len(entries))

	var filesSkipped, apiCount, eventCount uint32

	if pathExists(apiDir) {
		for _, mdPath := range walkMdFiles(apiDir, skipFilesAPI) {
			parsed, err := parseFile(mdPath, "api", mcdocsDir)
			if err != nil {
				filesSkipped++
				continue
			}
			apiCount += uint32(len(parsed))
			entries = append(entries, parsed...)
		}
	}
	if pathExists(eventDir) {
		for _, mdPath := range walkMdFiles(eventDir, skipFilesAPI) {
			parsed, err := parseFile(mdPath, "event", mcdocsDir)
			if err != nil {
				filesSkipped++
				continue
			}
			eventCount += uint32(len(parsed))
			entries = append(entries, parsed...)
		}
	}

	// 合并输出旁的可选 agent notes 文件。
	agentNotesPath := "api-agent-notes.json"
	if parent := filepath.Dir(outputPath); parent != "" {
		agentNotesPath = filepath.Join(parent, "api-agent-notes.json")
	}
	agentNotesLoaded := mergeAgentNotes(entries, agentNotesPath)

	// 排序：(type, name.lower(), side) 字典序升序，稳定排序。
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		na, nb := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if na != nb {
			return na < nb
		}
		return a.Side < b.Side
	})

	if parent := filepath.Dir(outputPath); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return BuildAPIIndexReport{}, fmt.Errorf("mkdir output parent: %w", err)
		}
	}
	data, err := marshalIndentNoEscape(entries)
	if err != nil {
		return BuildAPIIndexReport{}, fmt.Errorf("serialize api entries: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return BuildAPIIndexReport{}, fmt.Errorf("write api-index.json: %w", err)
	}

	return BuildAPIIndexReport{
		APICount:         apiCount,
		EventCount:       eventCount,
		HeaderCount:      headerCount,
		Total:            uint32(len(entries)),
		AgentNotesLoaded: agentNotesLoaded,
		FilesSkipped:     filesSkipped,
	}, nil
}

// buildHeaderEntries 上游 Python 固定前置的 3 条 header 条目（对齐 build_header_entries）。
func buildHeaderEntries() []OutputAPIEntry {
	names := []string{"ModSdk技术手册", "Apollo技术手册", "预设架构技术手册"}
	out := make([]OutputAPIEntry, 0, len(names))
	for _, h := range names {
		out = append(out, OutputAPIEntry{
			Kind:    "header",
			Name:    h,
			Side:    "server",
			Link:    "/docs/mcdocs/0-概述/0-概述.md",
			Params:  []OutputParam{},
			Returns: []OutputReturn{},
		})
	}
	return out
}

// walkMdFiles 递归遍历 dir，收集文件名不在 skipNames 内的 *.md。最终按路径排序保产物可复现
// （对齐 walk_md_files：栈式 DFS + 末尾 out.sort()）。
func walkMdFiles(dir string, skipNames []string) []string {
	var out []string
	stack := []string{dir}
	for len(stack) > 0 {
		d := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		read, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, entry := range read {
			p := filepath.Join(d, entry.Name())
			if entry.IsDir() {
				stack = append(stack, p)
				continue
			}
			ext := filepath.Ext(entry.Name()) // 带点，如 ".md"
			if !strings.EqualFold(ext, ".md") {
				continue
			}
			if containsStr(skipNames, entry.Name()) {
				continue
			}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// parseFile 解析单个 markdown 文件，返回 0 个或多个条目（对齐 parse_file）。
func parseFile(path, kind, mcdocsRoot string) ([]OutputAPIEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// 归一 setext 标题为 atx，让下游 split_h2/h3（只认 ## / ###）统一处理旧 curated 与新 html2md 两种形状。
	normalized := normalizeSetextHeadings(string(content))
	link := makeLink(path, mcdocsRoot)
	return parseSections(normalized, kind, link), nil
}

// normalizeSetextHeadings 把 setext 标题就地转 atx（对齐 normalize_setext_headings）：
//   - Title\n========（≥3 个 =）→ # Title
//   - Title\n--------（≥3 个 -）→ ## Title
//
// 管道表分隔行 ---|--- 与缩进 bullet 不受影响：下划线行须 trim 后整行全 = 或全 -（含 | 即不命中）。
func normalizeSetextHeadings(content string) string {
	lines := splitLines(content)
	var out strings.Builder
	out.Grow(len(content))
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			isUnderline := len(next) >= 3 && (allByteEq(next, '=') || allByteEq(next, '-'))
			if isUnderline {
				prefix := "## "
				if strings.HasPrefix(next, "=") {
					prefix = "# "
				}
				out.WriteString(prefix)
				out.WriteString(trimmed)
				out.WriteByte('\n')
				i += 2
				continue
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
		i++
	}
	return out.String()
}

// allByteEq 判 s 是否每个字节都等于 b（空串返 true，对齐 Rust bytes().all()）。
func allByteEq(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != b {
			return false
		}
	}
	return true
}

// makeLink 生成 /docs/mcdocs/... 链接（对齐 make_link）。mcdocsRoot 是 data/docs/mcdocs/ 目录，
// 剥到 data/docs/ 前缀使链接读作 /docs/mcdocs/...。手写前缀剥离（不用 filepath.Rel，它会产生 ..）。
func makeLink(filePath, mcdocsRoot string) string {
	docsDir := filepath.Dir(mcdocsRoot) // mcdocs 的父 = docs
	absSlash := filepath.ToSlash(filePath)
	prefix := filepath.ToSlash(docsDir) + "/"
	if strings.HasPrefix(absSlash, prefix) {
		return "/docs/" + absSlash[len(prefix):]
	}
	return absSlash
}

// parseSide 对 body 片段做启发式 side 判定（对齐 parse_side）。
func parseSide(text string) string {
	hasClient := strings.Contains(text, "客户端")
	hasServer := strings.Contains(text, "服务端")
	switch {
	case hasClient && hasServer:
		return "both"
	case hasClient:
		return "client"
	case hasServer:
		return "server"
	default:
		return ""
	}
}

// splitH2Sections 按顶层 ## 标题切 body，返回文档序 (标题, 去标题 body) 对（对齐 split_h2_sections）。
func splitH2Sections(content string) [][2]string {
	var out [][2]string
	started := false
	var currentTitle string
	var currentBody strings.Builder
	for _, line := range splitLines(content) {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			if started {
				out = append(out, [2]string{currentTitle, currentBody.String()})
				currentBody.Reset()
			}
			currentTitle = strings.TrimSpace(rest)
			started = true
		} else if started {
			currentBody.WriteString(line)
			currentBody.WriteByte('\n')
		}
	}
	if started {
		out = append(out, [2]string{currentTitle, currentBody.String()})
	}
	return out
}

// splitH3Sections 按 ### 切 body（对齐 split_h3_sections）。容忍尾随 #（html2md 偶发 ### 标题 ###）。
func splitH3Sections(body string) [][2]string {
	var out [][2]string
	started := false
	var currentTitle string
	var currentBody strings.Builder
	for _, line := range splitLines(body) {
		if rest, ok := strings.CutPrefix(line, "### "); ok {
			if started {
				out = append(out, [2]string{currentTitle, currentBody.String()})
				currentBody.Reset()
			}
			currentTitle = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rest), "#"))
			started = true
		} else if started {
			currentBody.WriteString(line)
			currentBody.WriteByte('\n')
		}
	}
	if started {
		out = append(out, [2]string{currentTitle, currentBody.String()})
	}
	return out
}

// subPartTitles 是上游用的 5 个 sub-part 标题（对齐 SUB_PART_TITLES）。
var subPartTitles = []string{"描述", "参数", "返回值", "备注", "示例"}

// splitSubParts 按顶层 "* <已知标题>" bullet 切 body（对齐 split_sub_parts）。旧式两空格缩进 bullet
// 与新 html2md 无缩进 bullet 都接受；bullet 文字不在白名单的（如 备注 下嵌套项）折进当前 sub-part body。
func splitSubParts(body string) []string {
	var out []string
	var current strings.Builder
	started := false
	for _, line := range splitLines(body) {
		trimmedStart := strings.TrimLeftFunc(line, unicode.IsSpace)
		if rest, ok := strings.CutPrefix(trimmedStart, "* "); ok {
			title := strings.TrimSpace(rest)
			isBoundary := false
			for _, t := range subPartTitles {
				if strings.HasPrefix(title, t) {
					isBoundary = true
					break
				}
			}
			if isBoundary {
				if started {
					out = append(out, current.String())
					current.Reset()
				}
				current.WriteString(rest)
				current.WriteByte('\n')
				started = true
				continue
			}
		}
		if started {
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	if started {
		out = append(out, current.String())
	}
	return out
}

// parseSubParts 解析 sub-part 段，返回 (描述, 备注, 参数, 返回值, 示例)（对齐 parse_sub_parts）。
func parseSubParts(subParts []string) (description, remark string, params []OutputParam, returns []OutputReturn, example string) {
	params = []OutputParam{}
	returns = []OutputReturn{}
	for _, sp := range subParts {
		lines := splitLines(sp)
		if len(lines) == 0 {
			continue
		}
		first := strings.TrimSpace(lines[0])
		bodyLines := lines[1:]
		bodyJoined := strings.Join(bodyLines, "\n")

		switch {
		case strings.HasPrefix(first, "描述"):
			var desc []string
			for _, s := range bodyLines {
				ts := strings.TrimSpace(s)
				if ts != "" {
					desc = append(desc, ts)
				}
			}
			description = strings.Join(desc, "\n")
		case strings.HasPrefix(first, "参数"):
			trimmed := strings.TrimSpace(bodyJoined)
			if trimmed != "" && trimmed != "无" {
				params = parseParamsTable(bodyLines)
			}
		case strings.HasPrefix(first, "返回值"):
			trimmed := strings.TrimSpace(bodyJoined)
			if trimmed != "" && trimmed != "无" {
				returns = parseReturnTable(bodyLines)
			}
		case strings.HasPrefix(first, "备注"):
			var acc []string
			for _, rl := range bodyLines {
				s := strings.TrimSpace(rl)
				if s == "" {
					continue
				}
				if rest, ok := strings.CutPrefix(s, "* "); ok {
					acc = append(acc, "- "+rest)
				} else {
					acc = append(acc, s)
				}
			}
			remark = strings.Join(acc, "\n")
		case strings.HasPrefix(first, "示例"):
			example = extractCodeBlock(bodyJoined)
		}
	}
	return
}

// parseParamsTable 解析管道表 body 为 [{name,type,description}]（对齐 parse_params_table）。
func parseParamsTable(lines []string) []OutputParam {
	params := []OutputParam{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "参数名") {
			continue
		}
		parts := splitTrimNonEmpty(line, "|")
		switch {
		case len(parts) >= 3:
			params = append(params, OutputParam{Name: parts[0], Ty: parts[1], Description: parts[2]})
		case len(parts) == 2:
			params = append(params, OutputParam{Name: parts[0], Ty: parts[1], Description: ""})
		default:
			continue
		}
	}
	return params
}

// parseReturnTable 解析管道表 body 为 [{type,description}]（对齐 parse_return_table）。
func parseReturnTable(lines []string) []OutputReturn {
	returns := []OutputReturn{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "数据类型") {
			continue
		}
		parts := splitTrimNonEmpty(line, "|")
		switch {
		case len(parts) >= 2:
			returns = append(returns, OutputReturn{Ty: parts[0], Description: parts[1]})
		case len(parts) == 1:
			returns = append(returns, OutputReturn{Ty: parts[0], Description: ""})
		default:
			continue
		}
	}
	return returns
}

// splitTrimNonEmpty 按 sep 切、逐段 TrimSpace、剔除空串（对齐 split('|').map(trim).filter(!empty)）。
func splitTrimNonEmpty(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		tp := strings.TrimSpace(p)
		if tp != "" {
			out = append(out, tp)
		}
	}
	return out
}

// extractCodeBlock 提取首个字面 [code]...[/code] 块（对齐 extract_code_block）。大小写敏感，无配对返 ""。
func extractCodeBlock(text string) string {
	const open = "[code]"
	const closeTag = "[/code]"
	if start := strings.Index(text, open); start >= 0 {
		after := start + len(open)
		if endRel := strings.Index(text[after:], closeTag); endRel >= 0 {
			return strings.TrimSpace(text[after : after+endRel])
		}
	}
	return ""
}

// parseSections 解析一个 markdown 文件里所有 ## name 段（对齐 parse_sections）。
func parseSections(content, entryType, link string) []OutputAPIEntry {
	var entries []OutputAPIEntry
	for _, section := range splitH2Sections(content) {
		name := section[0]
		body := section[1]
		if name == "" {
			continue
		}

		// 从前导非 h3 / 非 bullet 行推 side。
		side := ""
		for _, bl := range splitLines(body) {
			s := strings.TrimSpace(bl)
			if s == "" {
				continue
			}
			if strings.HasPrefix(s, "###") || strings.HasPrefix(s, "* ") {
				break
			}
			if strings.HasPrefix(s, "method ") {
				continue
			}
			if strings.Contains(s, "服务端") || strings.Contains(s, "客户端") {
				side = parseSide(s)
				break
			}
		}

		if side == "both" {
			h3 := splitH3Sections(body)
			hasSubsections := false
			for _, sec := range h3 {
				if strings.Contains(sec[0], "服务端") || strings.Contains(sec[0], "客户端") {
					hasSubsections = true
					break
				}
			}
			if hasSubsections {
				// Format A：独立 ### 服务端 / ### 客户端 块。
				for _, sec := range h3 {
					var s string
					if strings.Contains(sec[0], "服务端") {
						s = "server"
					} else if strings.Contains(sec[0], "客户端") {
						s = "client"
					} else {
						continue
					}
					description, remark, params, returns, example := parseSubParts(splitSubParts(sec[1]))
					entries = append(entries, OutputAPIEntry{
						Kind: entryType, Name: name, Method: "",
						Description: description, Remark: remark, Side: s, Link: link,
						Params: params, Returns: returns, Example: example,
					})
				}
			} else {
				// Format B：共享实现，每 side 各发一条。
				description, remark, params, returns, example := parseSubParts(splitSubParts(body))
				for _, s := range []string{"server", "client"} {
					entries = append(entries, OutputAPIEntry{
						Kind: entryType, Name: name, Method: "",
						Description: description, Remark: remark, Side: s, Link: link,
						Params: params, Returns: returns, Example: example,
					})
				}
			}
		} else {
			// 单 side 条目。空 side 默认 server。
			description, remark, params, returns, example := parseSubParts(splitSubParts(body))
			effectiveSide := side
			if effectiveSide == "" {
				effectiveSide = "server"
			}
			entries = append(entries, OutputAPIEntry{
				Kind: entryType, Name: name, Method: "",
				Description: description, Remark: remark, Side: effectiveSide, Link: link,
				Params: params, Returns: returns, Example: example,
			})
		}
	}
	return entries
}

// mergeAgentNotes 读 api-agent-notes.json（数组），按 (type, name_lower, side) 命中计数（对齐
// merge_agent_notes）。只计数不改写（输出 schema 无 agent_note 字段）。文件缺失/非数组 → 0。
func mergeAgentNotes(entries []OutputAPIEntry, notesPath string) uint32 {
	raw, err := os.ReadFile(notesPath)
	if err != nil {
		return 0
	}
	var notes []map[string]any
	if err := json.Unmarshal(raw, &notes); err != nil {
		return 0
	}
	byKey := make(map[[3]string]bool)
	for _, note := range notes {
		kind := strOf(note["type"])
		name := strOf(note["name"])
		side := strOf(note["side"])
		agentNote := strOf(note["agent_note"])
		if name == "" || agentNote == "" {
			continue
		}
		byKey[[3]string{kind, strings.ToLower(name), side}] = true
	}
	var hits uint32
	for _, e := range entries {
		if byKey[[3]string{e.Kind, strings.ToLower(e.Name), e.Side}] {
			hits++
		}
	}
	return hits
}

func strOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// pathExists 判路径是否存在（对齐 Rust Path::exists()，不要求是目录）。
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// marshalIndentNoEscape 复刻 serde_json::to_string_pretty：2 空格缩进 + 不转义 HTML（<>&）+ 无尾换行。
func marshalIndentNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
