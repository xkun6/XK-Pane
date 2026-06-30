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

func TestToolUseIDSeenInvalidatesPending(t *testing.T) {
	d := newDispatcher(dispatchDeps{})
	d.setPending("sid-1", PendingAsk{ToolUseID: "tool-1"})
	d.markToolUseSeen("sid-1", "tool-1")

	if _, ok := d.effectivePending("sid-1"); ok {
		t.Fatalf("pending should be invalid once toolUseId is seen")
	}
}
