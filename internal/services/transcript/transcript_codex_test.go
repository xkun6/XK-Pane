package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 合成一份覆盖主要分支的 Codex rollout（每行一个 JSON 对象）。
func writeCodexFixture(t *testing.T) string {
	t.Helper()
	lines := []string{
		`{"timestamp":"2026-06-30T00:00:00.000Z","type":"session_meta","payload":{"id":"019d-aaa","cwd":"J:/x","cli_version":"0.115.0"}}`,
		// developer / 注入消息 → 过滤
		`{"timestamp":"2026-06-30T00:00:01.000Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions instructions>\nno"}]}}`,
		`{"timestamp":"2026-06-30T00:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\ncwd=J:/x"}]}}`,
		// 真实用户消息
		`{"timestamp":"2026-06-30T00:00:03.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"帮我看看 a.txt"}]}}`,
		// 可读思考（event_msg.agent_reasoning）
		`{"timestamp":"2026-06-30T00:00:04.000Z","type":"event_msg","payload":{"type":"agent_reasoning","text":"先读文件"}}`,
		// 助手消息
		`{"timestamp":"2026-06-30T00:00:05.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"好的，我来看"}],"phase":"commentary"}}`,
		// shell 调用 + 结果（call_id 配对，结果在另一行）
		`{"timestamp":"2026-06-30T00:00:06.000Z","type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{\"command\":\"cat a.txt\",\"workdir\":\"J:/x\"}","call_id":"c1"}}`,
		`{"timestamp":"2026-06-30T00:00:07.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"Exit code: 0\nWall time: 0.4 seconds\nOutput:\nhello world\n"}}`,
		// apply_patch 调用 + 结果
		`{"timestamp":"2026-06-30T00:00:08.000Z","type":"response_item","payload":{"type":"custom_tool_call","status":"completed","name":"apply_patch","input":"*** Begin Patch\n*** Update File: a.txt\n@@\n ctx\n-old line\n+new line\n*** End Patch\n","call_id":"c2"}}`,
		`{"timestamp":"2026-06-30T00:00:09.000Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c2","output":"{\"output\":\"Success\",\"metadata\":{\"exit_code\":0}}"}}`,
		// update_plan 调用 + 结果
		`{"timestamp":"2026-06-30T00:00:10.000Z","type":"response_item","payload":{"type":"function_call","name":"update_plan","arguments":"{\"plan\":[{\"step\":\"读文件\",\"status\":\"completed\"},{\"step\":\"改文件\",\"status\":\"in_progress\"}]}","call_id":"c3"}}`,
		`{"timestamp":"2026-06-30T00:00:11.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c3","output":"Plan updated"}}`,
		// event_msg.agent_message 与上面助手消息重复 → 去重跳过
		`{"timestamp":"2026-06-30T00:00:12.000Z","type":"event_msg","payload":{"type":"agent_message","message":"好的，我来看"}}`,
		// 标题
		`{"timestamp":"2026-06-30T00:00:13.000Z","type":"event_msg","payload":{"type":"thread_name_updated","thread_id":"019d-aaa","thread_name":"看 a.txt"}}`,
		// token_count / task_complete 等元信息 → 跳过
		`{"timestamp":"2026-06-30T00:00:14.000Z","type":"event_msg","payload":{"type":"token_count","info":{}}}`,
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "rollout.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseCodex_FullMapping(t *testing.T) {
	p := writeCodexFixture(t)
	res := ParseCodex(p, 0)

	if res.Title != "看 a.txt" {
		t.Errorf("Title = %q, 期望 %q", res.Title, "看 a.txt")
	}

	// 收集各类事件
	var users, assistants, thinks []Event
	toolUseByID := map[string]Event{}
	toolResultByID := map[string]Event{}
	for _, ev := range res.Events {
		switch ev.Kind {
		case KindUser:
			users = append(users, ev)
		case KindAssistant:
			assistants = append(assistants, ev)
		case KindThinking:
			thinks = append(thinks, ev)
		case KindToolUse:
			toolUseByID[ev.ToolUseID] = ev
		case KindToolResult:
			toolResultByID[ev.ToolUseID] = ev
		}
	}

	// 用户消息：注入的两条被过滤，只剩真实一条
	if len(users) != 1 || users[0].Text != "帮我看看 a.txt" {
		t.Fatalf("user 事件 = %+v，期望仅 1 条真实用户消息", users)
	}
	if len(assistants) != 1 || !strings.Contains(assistants[0].Text, "好的") {
		t.Fatalf("assistant 事件 = %+v", assistants)
	}
	if len(thinks) != 1 || thinks[0].Text != "先读文件" {
		t.Fatalf("thinking 事件 = %+v", thinks)
	}

	// shell → Bash + bash Detail
	bashUse, ok := toolUseByID["c1"]
	if !ok || bashUse.ToolName != "Bash" {
		t.Fatalf("c1 工具调用 = %+v，期望 Bash", bashUse)
	}
	if !strings.Contains(bashUse.ToolInput, `"command"`) || !strings.Contains(bashUse.ToolInput, "cat a.txt") {
		t.Errorf("Bash toolInput = %q，期望含 command:cat a.txt", bashUse.ToolInput)
	}
	bashRes := toolResultByID["c1"]
	if bashRes.Detail == nil || bashRes.Detail.Kind != "bash" {
		t.Fatalf("c1 结果 Detail = %+v，期望 kind=bash", bashRes.Detail)
	}
	if strings.TrimSpace(bashRes.Detail.Stdout) != "hello world" {
		t.Errorf("bash stdout = %q，期望 'hello world'", bashRes.Detail.Stdout)
	}
	if bashRes.IsError {
		t.Errorf("exit 0 不应标 isError")
	}

	// apply_patch → Edit + edit Detail
	editUse := toolUseByID["c2"]
	if editUse.ToolName != "Edit" {
		t.Fatalf("c2 工具调用 = %+v，期望 Edit", editUse)
	}
	if !strings.Contains(editUse.ToolInput, "a.txt") {
		t.Errorf("Edit toolInput = %q，期望含 file_path a.txt", editUse.ToolInput)
	}
	editRes := toolResultByID["c2"]
	if editRes.Detail == nil || editRes.Detail.Kind != "edit" {
		t.Fatalf("c2 结果 Detail = %+v，期望 kind=edit", editRes.Detail)
	}
	if editRes.Detail.FilePath != "a.txt" {
		t.Errorf("edit FilePath = %q", editRes.Detail.FilePath)
	}
	if editRes.Detail.Added != 1 || editRes.Detail.Removed != 1 {
		t.Errorf("edit 增删 = +%d/-%d，期望 +1/-1", editRes.Detail.Added, editRes.Detail.Removed)
	}
	// diff 行保留 +/-/空格前缀，且不含 @@ / *** 头
	joined := strings.Join(editRes.Detail.Diff, "\n")
	if !strings.Contains(joined, "+new line") || !strings.Contains(joined, "-old line") {
		t.Errorf("edit diff = %q，期望含 +new/-old", joined)
	}
	if strings.Contains(joined, "@@") || strings.Contains(joined, "*** ") {
		t.Errorf("edit diff 不应含 @@ / *** 头：%q", joined)
	}

	// update_plan → TodoWrite + todos 形状
	planUse := toolUseByID["c3"]
	if planUse.ToolName != "TodoWrite" {
		t.Fatalf("c3 工具调用 = %+v，期望 TodoWrite", planUse)
	}
	var pin struct {
		Todos []struct {
			Content    string `json:"content"`
			Status     string `json:"status"`
			ActiveForm string `json:"activeForm"`
		} `json:"todos"`
	}
	if err := json.Unmarshal([]byte(planUse.ToolInput), &pin); err != nil {
		t.Fatalf("TodoWrite toolInput 非法 JSON: %v (%q)", err, planUse.ToolInput)
	}
	if len(pin.Todos) != 2 || pin.Todos[0].Content != "读文件" || pin.Todos[0].Status != "completed" || pin.Todos[1].Status != "in_progress" {
		t.Errorf("todos = %+v，期望 [读文件/completed, 改文件/in_progress]", pin.Todos)
	}

	// 去重：助手文本只出现一次（response_item.message 计入，event_msg.agent_message 跳过）
	if len(assistants) != 1 {
		t.Errorf("助手消息应去重为 1 条，实得 %d", len(assistants))
	}
}

// 增量游标：after>0 时早于游标的行不 emit，但 call_id 索引仍全量构建 → 跨游标的 output 仍能配对出 Detail。
func TestParseCodex_IncrementalPairingAcrossCursor(t *testing.T) {
	p := writeCodexFixture(t)
	full := ParseCodex(p, 0)

	// 找到 shell function_call（c1 调用）所在行号，把游标设在它之后、其 output 之前。
	var callSeq, outSeq int = -1, -1
	for _, ev := range full.Events {
		if ev.ToolUseID == "c1" && ev.Kind == KindToolUse {
			callSeq = ev.Seq
		}
		if ev.ToolUseID == "c1" && ev.Kind == KindToolResult {
			outSeq = ev.Seq
		}
	}
	if callSeq < 0 || outSeq <= callSeq {
		t.Fatalf("未找到 c1 的调用/结果行：call=%d out=%d", callSeq, outSeq)
	}

	// after 设为 callSeq+1：调用行被跳过（不 emit），但其结果行仍应出现且带 bash Detail。
	inc := ParseCodex(p, callSeq+1)
	for _, ev := range inc.Events {
		if ev.Kind == KindToolUse && ev.ToolUseID == "c1" {
			t.Errorf("after=%d 不应再 emit c1 的调用行（seq=%d）", callSeq+1, callSeq)
		}
	}
	var got *Event
	for i := range inc.Events {
		if inc.Events[i].ToolUseID == "c1" && inc.Events[i].Kind == KindToolResult {
			got = &inc.Events[i]
		}
	}
	if got == nil {
		t.Fatal("增量结果里缺 c1 的结果行")
	}
	if got.Detail == nil || got.Detail.Kind != "bash" {
		t.Fatalf("跨游标配对失败：c1 结果 Detail = %+v，期望 kind=bash（调用行虽在游标前仍应被索引）", got.Detail)
	}
	if inc.Total != full.Total {
		t.Errorf("Total = %d，期望与全量一致 %d", inc.Total, full.Total)
	}
}

// shell 非零退出码 → isError 置位。
func TestParseCodex_BashNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	lines := []string{
		`{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\"command\":\"false\"}","call_id":"x"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","call_id":"x","output":"Exit code: 1\nWall time: 0.1 seconds\nOutput:\nboom\n"}}`,
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res := ParseCodex(p, 0)
	var r *Event
	for i := range res.Events {
		if res.Events[i].Kind == KindToolResult {
			r = &res.Events[i]
		}
	}
	if r == nil {
		t.Fatal("缺 tool_result")
	}
	if !r.IsError {
		t.Errorf("exit 1 应标 isError")
	}
	if r.Detail == nil || strings.TrimSpace(r.Detail.Stdout) != "boom" {
		t.Errorf("stdout = %+v，期望 'boom'", r.Detail)
	}
}

func TestParseCodex_ExecCommandCmdAndProcessExitFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	lines := []string{
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}","call_id":"x"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","call_id":"x","output":"Chunk ID: abc123\nWall time: 0.1327 seconds\nProcess exited with code 1\nOriginal token count: 42\nOutput:\ncompile failed\n"}}`,
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := ParseCodex(p, 0)
	var use, result *Event
	for i := range res.Events {
		if res.Events[i].Kind == KindToolUse {
			use = &res.Events[i]
		}
		if res.Events[i].Kind == KindToolResult {
			result = &res.Events[i]
		}
	}
	if use == nil {
		t.Fatal("缺 tool_use")
	}
	if !strings.Contains(use.ToolInput, `"command":"go test ./..."`) {
		t.Fatalf("exec_command toolInput = %q，期望从 cmd 归一出 command", use.ToolInput)
	}
	if result == nil {
		t.Fatal("缺 tool_result")
	}
	if !result.IsError {
		t.Errorf("Process exited with code 1 应标 isError")
	}
	if result.Detail == nil || strings.TrimSpace(result.Detail.Stdout) != "compile failed" {
		t.Fatalf("exec_command stdout = %+v，期望仅保留 Output 后正文", result.Detail)
	}
}

func TestCodexEditDetailPreservesMoveTo(t *testing.T) {
	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: old.txt",
		"*** Move to: new.txt",
		"@@",
		"-old",
		"+new",
		"*** End Patch",
	}, "\n")

	d := codexEditDetail(patch)
	if d == nil {
		t.Fatal("缺 edit detail")
	}
	joined := strings.Join(d.Diff, "\n")
	if !strings.Contains(joined, "old.txt -> new.txt") {
		t.Fatalf("move diff = %q，期望保留 old -> new 边界", joined)
	}
}

func TestCodexOutputTextJoinsContentItemsWithNewlines(t *testing.T) {
	raw := json.RawMessage(`[{"type":"input_text","text":"one"},{"type":"input_text","text":"two"},{"type":"input_image","image_url":"data:image/png;base64,abc"}]`)

	got := codexOutputText(raw)
	if got != "one\ntwo\n[image]" {
		t.Fatalf("codexOutputText = %q，期望文本/图片项用换行分隔", got)
	}
}

func TestCleanCodexUserTextDoesNotDropLiteralTagsInRealPrompt(t *testing.T) {
	in := "请解释字符串 <environment_context> 在文档里的含义"

	got := cleanCodexUserText(in)
	if got == "" {
		t.Fatal("包含 environment_context 字面量的真实用户消息不应被整条过滤")
	}
}

// 读不到文件 → 空结果（非 nil events）。
func TestParseCodex_MissingFile(t *testing.T) {
	res := ParseCodex(filepath.Join(t.TempDir(), "nope.jsonl"), 0)
	if res.Events == nil || len(res.Events) != 0 || res.Total != 0 {
		t.Errorf("缺文件应返回空结果，实得 %+v", res)
	}
}
