package transcript

// 本文件把 Codex CLI 的 rollout JSONL（~/.codex/sessions/**/*.jsonl）解析为与 Claude 同一套
// 结构化时间线事件（Event），使前端卡片零改复用。
//
// 与 Claude 转录的关键差异（见调研记忆）：
//   - 顶层是 {timestamp, type, payload} 双层 tag；payload 内部再按 type 分流。
//   - 工具调用/结果靠 call_id 跨行配对（非 Claude 的 tool_use_id 同行内联）。
//   - 一次工具拆成多条：function_call（请求）+ function_call_output（模型可见结果，字符串）
//     + 旁路 event_msg.exec_command_end / patch_apply_end（结构化，**老版本常缺**）。
//   - shell 走 function_call 名 shell_command/shell/exec_command；apply_patch 走 custom_tool_call
//     （input 是 *** Begin Patch 私有补丁文本）；update_plan 走 function_call（plan[] 步骤表）。
//
// 设计取舍：以「永远自包含的行」为主源（function_call_output 字符串 / custom_tool_call 补丁文本），
// 旁路 *_end 仅当在场时增强。单遍扫全文件建 call_id→工具 索引（调用行恒先于其 output 行），
// emit output 时回查必命中——增量 after 游标 + 流式写入下均稳。
//
// 归一化策略（方案 A）：把 Codex 工具名归一到 Claude 词汇（shell→Bash / apply_patch→Edit /
// update_plan→TodoWrite / read_file→Read），并重塑 toolInput payload 形状，前端 toolMeta /
// 三张结构卡（BashCard / DiffCard / TodoCard）无需改动即命中。

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// codexCall 记录一次工具调用的归一化信息，供其 output 行回查（按 call_id）。
type codexCall struct {
	tool     string // 归一化后的工具名（Bash/Edit/TodoWrite/Read/MCP 原名/…）
	filePath string // apply_patch：从补丁抽出的目标文件（首个）
	patch    string // apply_patch：原始补丁文本（结果卡 diff 的来源）
}

// codexLineEnvelope 是每行 rollout 的统一外壳。
type codexLineEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// ParseCodex 读 path（Codex rollout jsonl），把 [after, 末尾) 的行解析成事件。读不到文件返回空结果。
// Total 始终为文件实际行数（增量游标）；call_id→工具 索引对全部行构建，仅 idx>=after 的行 emit 事件。
func ParseCodex(path string, after int) Result {
	res := Result{Events: []Event{}}
	f, err := os.Open(path)
	if err != nil {
		return res
	}
	defer func() { _ = f.Close() }()

	calls := map[string]*codexCall{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), scanLineCap)
	idx := -1
	for sc.Scan() {
		idx++
		res.Total = idx + 1
		parseCodexLine(sc.Bytes(), idx, after, calls, &res)
	}
	return res
}

func parseCodexLine(raw []byte, idx, after int, calls map[string]*codexCall, res *Result) {
	var env codexLineEnvelope
	if json.Unmarshal(raw, &env) != nil {
		return
	}
	switch env.Type {
	case "response_item":
		parseCodexResponseItem(env, idx, idx >= after, calls, res)
	case "event_msg":
		parseCodexEventMsg(env, idx, idx >= after, res)
	default:
		// session_meta / turn_context / compacted / world_state / … → 元信息，跳过
	}
}

// ── response_item ─────────────────────────────────────────────────────────────

func parseCodexResponseItem(env codexLineEnvelope, idx int, emit bool, calls map[string]*codexCall, res *Result) {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(env.Payload, &head) != nil {
		return
	}
	switch head.Type {
	case "message":
		if emit {
			parseCodexMessage(env, idx, res)
		}
	case "reasoning":
		// response_item.reasoning 多为 encrypted_content（无明文）→ 仅在 summary/content 有明文时出卡；
		// 可读思考的主源是 event_msg.agent_reasoning（见 parseCodexEventMsg）。
		if emit {
			if t := codexReasoningText(env.Payload); t != "" {
				res.Events = append(res.Events, Event{Seq: idx, Kind: KindThinking, Text: capRunes(t, textCap), Timestamp: env.Timestamp})
			}
		}
	case "function_call":
		var fc struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			CallID    string `json:"call_id"`
		}
		if json.Unmarshal(env.Payload, &fc) != nil || fc.CallID == "" {
			return
		}
		tool := normalizeCodexTool(fc.Name)
		calls[fc.CallID] = &codexCall{tool: tool} // 恒记录（含 idx<after），供后续 output 回查
		if emit {
			res.Events = append(res.Events, Event{
				Seq: idx, Kind: KindToolUse, ToolName: tool,
				ToolInput: codexToolInput(tool, fc.Arguments), ToolUseID: fc.CallID, Timestamp: env.Timestamp,
			})
		}
	case "custom_tool_call":
		var ct struct {
			Name   string `json:"name"`
			Input  string `json:"input"`
			CallID string `json:"call_id"`
		}
		if json.Unmarshal(env.Payload, &ct) != nil || ct.CallID == "" {
			return
		}
		tool := normalizeCodexTool(ct.Name)
		rec := &codexCall{tool: tool}
		if ct.Name == "apply_patch" {
			rec.patch = ct.Input
			rec.filePath = codexPatchFile(ct.Input)
		}
		calls[ct.CallID] = rec
		if emit {
			input := ""
			if rec.filePath != "" {
				input = compactJSON(map[string]string{"file_path": rec.filePath})
			}
			res.Events = append(res.Events, Event{
				Seq: idx, Kind: KindToolUse, ToolName: tool,
				ToolInput: input, ToolUseID: ct.CallID, Timestamp: env.Timestamp,
			})
		}
	case "function_call_output":
		if emit {
			parseCodexCallOutput(env, idx, calls, res, false)
		}
	case "custom_tool_call_output":
		if emit {
			parseCodexCallOutput(env, idx, calls, res, true)
		}
	default:
		// web_search_call / tool_search_* / local_shell_call 等 → 暂跳过
	}
}

func parseCodexMessage(env codexLineEnvelope, idx int, res *Result) {
	var m struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(env.Payload, &m) != nil {
		return
	}
	// developer/system/tool = 注入的权限/系统指令，不当对话渲染。
	if m.Role != "user" && m.Role != "assistant" {
		return
	}
	var b strings.Builder
	for _, c := range m.Content {
		if (c.Type == "input_text" || c.Type == "output_text") && c.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return
	}
	if m.Role == "user" {
		if t := cleanCodexUserText(text); t != "" {
			res.Events = append(res.Events, Event{Seq: idx, Kind: KindUser, Text: t, Timestamp: env.Timestamp})
		}
		return
	}
	res.Events = append(res.Events, Event{Seq: idx, Kind: KindAssistant, Text: capRunes(text, textCap), Timestamp: env.Timestamp})
}

// codexReasoningText 抽 response_item.reasoning 的明文（summary[].text + content[].text）；全空返回 ""。
func codexReasoningText(payload json.RawMessage) string {
	var r struct {
		Summary []struct {
			Text string `json:"text"`
		} `json:"summary"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(payload, &r) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Summary {
		if p.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	for _, p := range r.Content {
		if p.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func parseCodexCallOutput(env codexLineEnvelope, idx int, calls map[string]*codexCall, res *Result, custom bool) {
	var out struct {
		CallID string          `json:"call_id"`
		Output json.RawMessage `json:"output"`
	}
	if json.Unmarshal(env.Payload, &out) != nil {
		return
	}
	text := codexOutputText(out.Output)
	ev := Event{Seq: idx, Kind: KindToolResult, ToolUseID: out.CallID, Timestamp: env.Timestamp}
	rec := calls[out.CallID]
	switch {
	case rec != nil && rec.tool == "Bash":
		// shell：从 "Exit code: N\nWall time…\nOutput:\n<内容>" 字符串解析出 stdout + 退出码。
		d, isErr := codexBashDetail(text)
		ev.Detail = d
		ev.IsError = isErr
		ev.Text = capRunes(stripANSI(text), toolResultCap)
	case custom && rec != nil && rec.tool == "Edit":
		// apply_patch：diff 取自调用行的补丁文本（自包含）；退出码取自 output 的 metadata。
		ev.Detail = codexEditDetail(rec.patch)
		if codexCustomExitCode(text) != 0 {
			ev.IsError = true
		}
		ev.Text = capRunes(text, toolResultCap)
	default:
		ev.Text = capRunes(stripANSI(text), toolResultCap)
	}
	res.Events = append(res.Events, ev)
}

// ── event_msg ─────────────────────────────────────────────────────────────────

func parseCodexEventMsg(env codexLineEnvelope, idx int, emit bool, res *Result) {
	var em struct {
		Type       string `json:"type"`
		Text       string `json:"text"`        // agent_reasoning
		ThreadName string `json:"thread_name"` // thread_name_updated
	}
	if json.Unmarshal(env.Payload, &em) != nil {
		return
	}
	switch em.Type {
	case "thread_name_updated":
		if em.ThreadName != "" {
			res.Title = em.ThreadName // 标题是累积态，不受 after 游标限制
		}
	case "agent_reasoning":
		if emit {
			if t := strings.TrimSpace(em.Text); t != "" {
				res.Events = append(res.Events, Event{Seq: idx, Kind: KindThinking, Text: capRunes(t, textCap), Timestamp: env.Timestamp})
			}
		}
	default:
		// agent_message / user_message（与 response_item.message 重复，去重跳过）、token_count、
		// task_started/complete、exec_command_end/patch_apply_end（旁路增强，v1 不用）等 → 跳过。
	}
}

// ── 归一化 / 解析辅助 ─────────────────────────────────────────────────────────

// normalizeCodexTool 把 Codex 工具名归一到 Claude 词汇，命中前端既有图标/摘要/卡片分支。
// 未识别名（mcp__server__tool、web_search、view_image…）原样保留，走前端兜底图标。
func normalizeCodexTool(name string) string {
	switch name {
	case "shell", "shell_command", "exec_command", "local_shell", "container.exec":
		return "Bash"
	case "apply_patch":
		return "Edit"
	case "update_plan":
		return "TodoWrite"
	case "read_file":
		return "Read"
	}
	return name
}

// codexToolInput 把工具 arguments（JSON 字符串）重塑成前端对应卡片期待的 toolInput 形状。
func codexToolInput(tool, arguments string) string {
	switch tool {
	case "Bash":
		var a struct {
			Command json.RawMessage `json:"command"`
			Cmd     json.RawMessage `json:"cmd"`
		}
		_ = json.Unmarshal([]byte(arguments), &a)
		if cmd := codexCommandString(a.Command); cmd != "" {
			return compactJSON(map[string]string{"command": cmd})
		}
		if cmd := codexCommandString(a.Cmd); cmd != "" {
			return compactJSON(map[string]string{"command": cmd})
		}
		return passthroughArgs(arguments)
	case "TodoWrite":
		return codexPlanToTodos(arguments)
	default:
		// Read / MCP / 其它：原样透传 arguments（前端 summarizeToolInput 取 path/file_path/… 兜底）。
		return passthroughArgs(arguments)
	}
}

// passthroughArgs 透传合法 JSON 的 arguments（截断），非法则返回空（前端回退原串渲染）。
func passthroughArgs(arguments string) string {
	if json.Valid([]byte(arguments)) {
		return capRunes(arguments, toolInputCap)
	}
	return ""
}

// codexCommandString 把 shell command 转为字符串：可能是字符串或字符串数组（exec_command）。
func codexCommandString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return strings.Join(arr, " ")
	}
	return ""
}

// codexPlanToTodos 把 update_plan 的 {plan:[{step,status}]} 重塑为 TodoCard 期待的 {todos:[{content,status,activeForm}]}。
// status 取值（pending/in_progress/completed）与 Claude todo 完全一致，直接沿用。
func codexPlanToTodos(arguments string) string {
	var a struct {
		Plan []struct {
			Step   string `json:"step"`
			Status string `json:"status"`
		} `json:"plan"`
	}
	if json.Unmarshal([]byte(arguments), &a) != nil || len(a.Plan) == 0 {
		return passthroughArgs(arguments)
	}
	type todo struct {
		Content    string `json:"content"`
		Status     string `json:"status"`
		ActiveForm string `json:"activeForm"`
	}
	todos := make([]todo, 0, len(a.Plan))
	for _, p := range a.Plan {
		todos = append(todos, todo{Content: p.Step, Status: p.Status, ActiveForm: p.Step})
	}
	return compactJSON(map[string]any{"todos": todos})
}

// codexOutputText 把工具 output 抽成纯文本：可能是字符串，也可能是 content 数组（如 view_image 返回图片）。
func codexOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL string `json:"image_url"`
	}
	if json.Unmarshal(raw, &arr) == nil {
		var b strings.Builder
		appendPart := func(s string) {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(s)
		}
		for _, it := range arr {
			switch {
			case it.Text != "":
				appendPart(it.Text)
			case it.ImageURL != "":
				appendPart("[image]")
			}
		}
		return b.String()
	}
	return ""
}

// codexBashDetail 把 shell/exec_command 的 output 字符串解析成 bash Detail（stdout）+ 是否出错（退出码非 0）。
// shell_command 形如 "Exit code: 0\nWall time: ...\nOutput:\n<内容>"；
// exec_command 形如 "Chunk ID: ...\nWall time: ...\nProcess exited with code 0\nOutput:\n<内容>"。
func codexBashDetail(output string) (*ToolResultDetail, bool) {
	body := output
	isErr := false
	for _, ln := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimRight(ln, "\r"))
		switch {
		case strings.HasPrefix(line, "Exit code:"):
			codeStr := strings.TrimSpace(strings.TrimPrefix(line, "Exit code:"))
			if n, err := strconv.Atoi(codeStr); err == nil && n != 0 {
				isErr = true
			}
		case strings.HasPrefix(line, "Process exited with code "):
			codeStr := strings.TrimSpace(strings.TrimPrefix(line, "Process exited with code "))
			if n, err := strconv.Atoi(codeStr); err == nil && n != 0 {
				isErr = true
			}
		}
	}
	if i := strings.Index(output, "\nOutput:\n"); i >= 0 {
		body = output[i+len("\nOutput:\n"):]
	} else if strings.HasPrefix(output, "Output:\n") {
		body = output[len("Output:\n"):]
	}
	body = strings.Trim(body, "\r\n")
	return &ToolResultDetail{Kind: "bash", Stdout: capRunes(stripANSI(body), toolResultCap)}, isErr
}

// codexCustomExitCode 取 custom_tool_call_output 的 metadata.exit_code（output 是内嵌 JSON 字符串）。
func codexCustomExitCode(output string) int {
	var o struct {
		Metadata struct {
			ExitCode int `json:"exit_code"`
		} `json:"metadata"`
	}
	if json.Unmarshal([]byte(output), &o) != nil {
		return 0
	}
	return o.Metadata.ExitCode
}

// codexPatchFile 取 apply_patch 补丁里首个目标文件路径（*** Update/Add/Delete File: X）。
func codexPatchFile(patch string) string {
	for _, ln := range strings.Split(patch, "\n") {
		for _, pfx := range []string{"*** Update File: ", "*** Add File: ", "*** Delete File: "} {
			if strings.HasPrefix(ln, pfx) {
				return strings.TrimSpace(strings.TrimPrefix(ln, pfx))
			}
		}
	}
	return ""
}

// codexEditDetail 把 apply_patch 的私有补丁文本解析为 edit Detail（diff 行带 +/-/空格前缀，直接喂 DiffCard）。
// 补丁格式：*** Begin Patch / *** Update File: X / @@ / " 上下文" / "-删" / "+增" / *** End Patch。
func codexEditDetail(patch string) *ToolResultDetail {
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	d := &ToolResultDetail{Kind: "edit"}
	total := 0
	push := func(line string) {
		total++
		if len(d.Diff) < diffLineCap {
			d.Diff = append(d.Diff, capRunes(line, diffLineRuneCap))
		}
	}
	for _, ln := range strings.Split(patch, "\n") {
		switch {
		case ln == "*** Begin Patch" || ln == "*** End Patch" || strings.HasPrefix(ln, "@@"):
			continue
		case strings.HasPrefix(ln, "*** Update File: "),
			strings.HasPrefix(ln, "*** Add File: "),
			strings.HasPrefix(ln, "*** Delete File: "):
			p := strings.TrimSpace(ln[strings.IndexByte(ln, ':')+1:])
			if d.FilePath == "" {
				d.FilePath = p // 首文件 → 卡片标题用其 basename
			} else {
				push(ln) // 多文件补丁：非首文件保留文件头作边界
			}
		case strings.HasPrefix(ln, "*** Move to: "):
			p := strings.TrimSpace(strings.TrimPrefix(ln, "*** Move to: "))
			if d.FilePath != "" && p != "" {
				push(d.FilePath + " -> " + p)
			} else if p != "" {
				push("-> " + p)
			}
		case strings.HasPrefix(ln, "*** "):
			continue // 其它 *** 指令
		default:
			if strings.HasPrefix(ln, "+") {
				d.Added++
			} else if strings.HasPrefix(ln, "-") {
				d.Removed++
			}
			push(ln)
		}
	}
	if len(d.Diff) == 0 {
		return nil
	}
	d.Truncated = total > len(d.Diff)
	return d
}

// cleanCodexUserText 过滤 Codex 注入到 user 消息里的系统包装（AGENTS.md / 环境上下文 / 权限指令等）。
func cleanCodexUserText(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(t, "# AGENTS.md"),
		strings.HasPrefix(t, "<user_instructions>"),
		strings.HasPrefix(t, "<environment_context>"),
		strings.HasPrefix(t, "<permissions instructions>"),
		strings.HasPrefix(t, "<system-reminder>"):
		return ""
	}
	return capRunes(t, textCap)
}

// compactJSON 把 v 编码成紧凑 JSON 字符串（编码失败返回空串）。
func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
