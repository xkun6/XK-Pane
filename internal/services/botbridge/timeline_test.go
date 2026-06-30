package botbridge

import (
	"strings"
	"testing"

	"xkpane/internal/services/transcript"
)

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
