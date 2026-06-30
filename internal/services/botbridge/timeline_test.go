package botbridge

import (
	"fmt"
	"strings"
	"testing"

	"xkpane/internal/events"
	"xkpane/internal/services/transcript"
)

func countActions(c *Card) int {
	n := 0
	for _, e := range c.Elements {
		if _, ok := e.(CardActions); ok {
			n++
		}
	}
	return n
}

func hasButtonValue(c *Card, value string) bool {
	for _, e := range c.Elements {
		if a, ok := e.(CardActions); ok {
			for _, b := range a.Buttons {
				if b.Value == value {
					return true
				}
			}
		}
	}
	return false
}

func TestAskCardsOneCardPerQuestion(t *testing.T) {
	p := PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Header: "Q1", Question: "选一个", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
			{Header: "Q2", Question: "再选", MultiSelect: true, Options: []events.AskOption{{Label: "C"}, {Label: "D"}}},
			{Header: "Q3", Question: "最后一个", Options: []events.AskOption{{Label: "E"}}},
		},
	}
	cards := askCards(p)
	if len(cards) != 3 {
		t.Fatalf("askCards 多问题 = %d 张卡, want 3（一题一卡）", len(cards))
	}
	for i, c := range cards {
		// 飞书对单卡 3+ 个 action 模块会整卡降级成纯文字、丢掉所有按钮，故每张卡的 action 模块必须 ≤2。
		if got := countActions(c); got > 2 {
			t.Fatalf("第 %d 张卡有 %d 个 action 模块, want ≤2（飞书 3+ 会降级丢按钮）", i, got)
		}
		if c.Header == nil || !strings.Contains(c.Header.Title, fmt.Sprintf("%d/3", i+1)) {
			t.Fatalf("第 %d 张卡标题 = %#v, want 含 %d/3", i, c.Header, i+1)
		}
	}
	// 前两张只有选项行（1 个 action 模块）、不带「提交全部」。
	for i := 0; i < 2; i++ {
		if got := countActions(cards[i]); got != 1 {
			t.Fatalf("第 %d 张卡 action 模块 = %d, want 1（仅选项行）", i, got)
		}
		if hasButtonValue(cards[i], "asksubmit:tool-1") {
			t.Fatalf("第 %d 张卡不该带「提交全部」按钮", i)
		}
	}
	// 末张 = 选项行 + 「提交全部」，共 2 个 action 模块。
	if got := countActions(cards[2]); got != 2 {
		t.Fatalf("末张 action 模块 = %d, want 2（选项+提交全部）", got)
	}
	if !hasButtonValue(cards[2], "asksubmit:tool-1") {
		t.Fatalf("末张缺「提交全部」(asksubmit:tool-1) 按钮")
	}
}

func TestAskCardsSingleQuestionUsesOneCard(t *testing.T) {
	p := PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "只有一个", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
		},
	}
	cards := askCards(p)
	if len(cards) != 1 {
		t.Fatalf("单问题 = %d 张卡, want 1", len(cards))
	}
	// 单问题：选项行 + 提交 = 2 个 action 模块，飞书可正常渲染。
	if got := countActions(cards[0]); got != 2 {
		t.Fatalf("单问题卡 action 模块 = %d, want 2（选项+提交）", got)
	}
	if !hasButtonValue(cards[0], "asksubmit:tool-1") {
		t.Fatalf("单问题卡缺「提交」(asksubmit:tool-1) 按钮")
	}
}

func TestAskTextNumbersOptionsAndInstructs(t *testing.T) {
	p := PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "选一个", Options: []events.AskOption{{Label: "蓝绿", Description: "零停机"}, {Label: "滚动"}}},
			{Question: "再选", MultiSelect: true, Options: []events.AskOption{{Label: "监控"}, {Label: "告警"}}},
		},
	}
	text := askText(p)
	// 每题选项必须 1-based 编号，描述随选项附上。
	for _, want := range []string{"1) 蓝绿", "2) 滚动", "零停机", "1) 监控", "2) 告警", "问题 1", "问题 2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("askText 缺片段 %q：\n%s", want, text)
		}
	}
	// 多题 + 含多选 → 提示分号分隔每题、逗号分隔多选。
	if !strings.Contains(text, "分号") || !strings.Contains(text, "逗号") {
		t.Fatalf("askText 多题多选未给出分隔说明：\n%s", text)
	}
}

func TestAskTextSingleQuestionSimpleHint(t *testing.T) {
	p := PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "只有一个", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
		},
	}
	text := askText(p)
	if strings.Contains(text, "分号") {
		t.Fatalf("单题单选不应提示分号：\n%s", text)
	}
	if !strings.Contains(text, "1") || !strings.Contains(text, "回复编号") {
		t.Fatalf("单题缺编号作答提示：\n%s", text)
	}
}

func TestEventToCardSkipsAskUserQuestionToolUse(t *testing.T) {
	card, ok := eventToCard(transcript.Event{Kind: transcript.KindToolUse, ToolName: "AskUserQuestion"})
	if ok {
		t.Fatalf("AskUserQuestion card ok=true, card=%#v", card)
	}
}

func TestEventToCardMapsBashDetail(t *testing.T) {
	card, ok := eventToCard(transcript.Event{
		Kind:      transcript.KindToolResult,
		ToolUseID: "tool-1",
		Detail: &transcript.ToolResultDetail{
			Kind:   "bash",
			Stdout: "hello",
			Stderr: "warn",
		},
	})
	if !ok {
		t.Fatalf("eventToCard ok=false")
	}
	text := card.RenderText()
	if !strings.Contains(text, "hello") || !strings.Contains(text, "warn") {
		t.Fatalf("card text %q does not include stdout/stderr", text)
	}
}

func TestEventToCardMapsEditDetail(t *testing.T) {
	card, ok := eventToCard(transcript.Event{
		Kind: transcript.KindToolResult,
		Detail: &transcript.ToolResultDetail{
			Kind:     "edit",
			FilePath: "a.go",
			Diff:     []string{"-old", "+new"},
			Added:    1,
			Removed:  1,
		},
	})
	if !ok {
		t.Fatalf("eventToCard ok=false")
	}
	text := card.RenderText()
	if !strings.Contains(text, "a.go") || !strings.Contains(text, "-old") || !strings.Contains(text, "+new") {
		t.Fatalf("card text %q does not include edit detail", text)
	}
}
