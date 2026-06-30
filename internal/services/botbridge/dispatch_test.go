package botbridge

import (
	"context"
	"reflect"
	"testing"

	"xkpane/internal/events"
)

type fakeWriter struct {
	writes []string
}

func (f *fakeWriter) WriteInput(_ context.Context, _ string, data string) error {
	f.writes = append(f.writes, data)
	return nil
}

func TestDispatchAskSelectionAccumulatesAndSubmits(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{
		writer: writer,
		ask: func(string) (PendingAsk, bool) {
			return PendingAsk{
				ToolUseID: "tool-1",
				Questions: []events.AskQuestion{
					{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}}},
					{Question: "Again", Options: []events.AskOption{{Label: "D"}, {Label: "E"}}},
				},
			}, true
		},
	})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}}},
			{Question: "Again", Options: []events.AskOption{{Label: "D"}, {Label: "E"}}},
		},
	})

	if handled, err := d.handleAction(context.Background(), "scope-a", "", "askq:0:2"); err != nil || !handled {
		t.Fatalf("askq handled=%v err=%v", handled, err)
	}
	if handled, err := d.handleAction(context.Background(), "scope-a", "", "askq:1:1"); err != nil || !handled {
		t.Fatalf("askq handled=%v err=%v", handled, err)
	}
	if handled, err := d.handleAction(context.Background(), "scope-a", "", "asksubmit:tool-1"); err != nil || !handled {
		t.Fatalf("asksubmit handled=%v err=%v", handled, err)
	}

	want := []string{keyDown, keyDown, keyEnter, keyDown, keyEnter, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestDispatchMultiSelectToggleWaitsForSubmit(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{
				Question:    "Pick",
				MultiSelect: true,
				Options:     []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}},
			},
		},
	})

	for _, action := range []string{"askq:0:1", "askq:0:1", "askq:0:2"} {
		handled, err := d.handleAction(context.Background(), "scope-a", "", action)
		if err != nil || !handled {
			t.Fatalf("%s handled=%v err=%v", action, handled, err)
		}
	}
	if len(writer.writes) != 0 {
		t.Fatalf("multi select should wait for submit, wrote %#v", writer.writes)
	}

	handled, err := d.handleAction(context.Background(), "scope-a", "", "asksubmit:tool-1")
	if err != nil || !handled {
		t.Fatalf("asksubmit handled=%v err=%v", handled, err)
	}

	want := []string{keyDown, keyDown, keySpace, keyRight, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestDispatchSingleSelectAskQSubmitsImmediately(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{
		writer: writer,
		ask: func(string) (PendingAsk, bool) {
			return PendingAsk{
				ToolUseID: "tool-1",
				Questions: []events.AskQuestion{
					{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
				},
			}, true
		},
	})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
		},
	})

	if handled, err := d.handleAction(context.Background(), "scope-a", "", "askq:0:1"); err != nil || !handled {
		t.Fatalf("askq handled=%v err=%v", handled, err)
	}

	want := []string{keyDown, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestDispatchIgnoresDuplicateMessageID(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")

	for i := 0; i < 2; i++ {
		handled, err := d.handleMessage(context.Background(), "scope-a", "msg-1", "run")
		if err != nil {
			t.Fatalf("handleMessage #%d err=%v", i+1, err)
		}
		if !handled {
			t.Fatalf("handleMessage #%d handled=false", i+1)
		}
	}

	want := []string{"run\r"}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestSubmitAskMarksToolUseAnsweredLocally(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
		},
	})

	for i := 0; i < 2; i++ {
		handled, err := d.handleAction(context.Background(), "scope-a", "", "asksubmit:tool-1")
		if err != nil || !handled {
			t.Fatalf("asksubmit #%d handled=%v err=%v", i+1, handled, err)
		}
	}

	want := []string{keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestDispatchStaleAskSelectionIsDropped(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")

	handled, err := d.handleAction(context.Background(), "scope-a", "", "askq:0:1")
	if err != nil {
		t.Fatalf("handleAction err=%v", err)
	}
	if !handled {
		t.Fatalf("askq should be treated as handled even when stale")
	}
	if len(writer.writes) != 0 {
		t.Fatalf("stale askq wrote %#v", writer.writes)
	}
}

func TestDispatchRejectsEmptyMessage(t *testing.T) {
	d := newDispatcher(dispatchDeps{})
	handled, err := d.handleMessage(context.Background(), "scope-a", "", "  ")
	if err != nil {
		t.Fatalf("handleMessage err=%v", err)
	}
	if handled {
		t.Fatalf("empty message should be ignored")
	}
}

func TestTextAnswerSingleSelectSubmits(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
		},
	})

	handled, err := d.handleMessage(context.Background(), "scope-a", "", "2")
	if err != nil || !handled {
		t.Fatalf("handleMessage handled=%v err=%v", handled, err)
	}
	want := []string{keyDown, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestTextAnswerMultiQuestionSemicolon(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}}},
			{Question: "Again", Options: []events.AskOption{{Label: "D"}, {Label: "E"}}},
		},
	})

	handled, err := d.handleMessage(context.Background(), "scope-a", "", "3;1")
	if err != nil || !handled {
		t.Fatalf("handleMessage handled=%v err=%v", handled, err)
	}
	// Q1 选第 3 项（2 个 keyDown + Enter），Q2 选第 1 项（Enter），末尾提交 Enter。
	want := []string{keyDown, keyDown, keyEnter, keyEnter, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestTextAnswerMultiSelectComma(t *testing.T) {
	writer := &fakeWriter{}
	d := newDispatcher(dispatchDeps{writer: writer})
	d.setBinding("scope-a", "sid-1")
	d.setPending("sid-1", PendingAsk{
		ToolUseID: "tool-1",
		Questions: []events.AskQuestion{
			{Question: "Pick", MultiSelect: true, Options: []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}}},
		},
	})

	handled, err := d.handleMessage(context.Background(), "scope-a", "", "1,3")
	if err != nil || !handled {
		t.Fatalf("handleMessage handled=%v err=%v", handled, err)
	}
	want := []string{keySpace, keyDown, keyDown, keySpace, keyRight, keyEnter}
	if !reflect.DeepEqual(writer.writes, want) {
		t.Fatalf("writes = %#v, want %#v", writer.writes, want)
	}
}

func TestTextNonAnswerDuringPendingFallsThrough(t *testing.T) {
	// 待答期内的非编号回复（普通文本 / 越界编号）不得误判为作答，必须原样透传到 PTY。
	for _, in := range []string{"hello", "9"} {
		writer := &fakeWriter{}
		d := newDispatcher(dispatchDeps{writer: writer})
		d.setBinding("scope-a", "sid-1")
		d.setPending("sid-1", PendingAsk{
			ToolUseID: "tool-1",
			Questions: []events.AskQuestion{
				{Question: "Pick", Options: []events.AskOption{{Label: "A"}, {Label: "B"}}},
			},
		})

		handled, err := d.handleMessage(context.Background(), "scope-a", "", in)
		if err != nil || !handled {
			t.Fatalf("handleMessage(%q) handled=%v err=%v", in, handled, err)
		}
		want := []string{in + "\r"}
		if !reflect.DeepEqual(writer.writes, want) {
			t.Fatalf("in=%q writes = %#v, want %#v", in, writer.writes, want)
		}
	}
}

func TestParseTextSelections(t *testing.T) {
	qs := []events.AskQuestion{
		{Options: []events.AskOption{{Label: "A"}, {Label: "B"}, {Label: "C"}}},
		{MultiSelect: true, Options: []events.AskOption{{Label: "D"}, {Label: "E"}}},
	}
	cases := []struct {
		name string
		in   string
		want map[int][]int
		ok   bool
	}{
		{"single", "1", map[int][]int{0: {0}}, true},
		{"two questions", "2;1", map[int][]int{0: {1}, 1: {0}}, true},
		{"skip q1 multi q2", ";1,2", map[int][]int{1: {0, 1}}, true},
		{"chinese punctuation", "1；2", map[int][]int{0: {0}, 1: {1}}, true},
		{"single select rejects multi", "1,2", nil, false},
		{"too many segments", "1;1;1", nil, false},
		{"out of range", "9", nil, false},
		{"non numeric", "ab", nil, false},
		{"empty", "", nil, false},
	}
	for _, c := range cases {
		got, ok := parseTextSelections(c.in, qs)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v, want %v", c.name, ok, c.ok)
		}
		if c.ok && !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: got=%#v, want %#v", c.name, got, c.want)
		}
	}
}

func TestToolUseIDSeenInvalidatesPending(t *testing.T) {
	d := newDispatcher(dispatchDeps{})
	d.setPending("sid-1", PendingAsk{ToolUseID: "tool-1"})
	d.markToolUseSeen("sid-1", "tool-1")

	if _, ok := d.effectivePending("sid-1"); ok {
		t.Fatalf("pending should be invalid once toolUseId is seen")
	}
}
