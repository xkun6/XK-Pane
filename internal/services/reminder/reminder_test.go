package reminder

import (
	"context"
	"testing"
	"time"

	"xkpane/internal/services/notification"
	"xkpane/internal/services/todo"
)

// fakeLister 返回固定的到期待办。
type fakeLister struct {
	due []todo.TodoDTO
	err error
}

func (f *fakeLister) CheckReminders(context.Context) ([]todo.TodoDTO, error) {
	return f.due, f.err
}

// fakeNotifier 记录每次 Trigger，并按 sent 决定 Result.Sent。
type fakeNotifier struct {
	calls []notification.Request
	sent  bool
}

func (f *fakeNotifier) Trigger(req notification.Request) (notification.Result, error) {
	f.calls = append(f.calls, req)
	return notification.Result{Sent: f.sent, Skipped: !f.sent}, nil
}

func newDispatcher(l dueLister, n notifier, locale string) (*Dispatcher, *time.Time) {
	d := New(l, n, func() string { return locale })
	now := time.Now()
	d.nowFn = func() time.Time { return now }
	return d, &now
}

func TestDispatcherNotifiesAndDedups(t *testing.T) {
	lister := &fakeLister{due: []todo.TodoDTO{{ID: "a", Title: "买牛奶"}}}
	notif := &fakeNotifier{sent: true}
	d, now := newDispatcher(lister, notif, "zh-CN")

	// 第一轮：发一次，body=标题，title=中文标签，dedupeKey 带 id。
	d.runOnce(context.Background())
	if len(notif.calls) != 1 {
		t.Fatalf("round1 calls = %d, want 1", len(notif.calls))
	}
	c := notif.calls[0]
	if c.Body != "买牛奶" || c.Title != "待办提醒" || c.DedupeKey != "todo-reminder-a" || c.Kind != "todo_reminder" {
		t.Fatalf("notification req wrong: %+v", c)
	}

	// 第二轮（窗口内）：不重复发。
	d.runOnce(context.Background())
	if len(notif.calls) != 1 {
		t.Fatalf("within window calls = %d, want still 1", len(notif.calls))
	}

	// 推进超过 10min：重提醒一次。
	*now = now.Add(renotifyWindow + time.Second)
	d.runOnce(context.Background())
	if len(notif.calls) != 2 {
		t.Fatalf("after window calls = %d, want 2", len(notif.calls))
	}
}

func TestDispatcherSkippedNotRecorded(t *testing.T) {
	// 通知被跳过（禁用/聚焦）→ Result.Sent=false → 不消耗窗口 → 下一轮补发。
	lister := &fakeLister{due: []todo.TodoDTO{{ID: "a", Title: "x"}}}
	notif := &fakeNotifier{sent: false}
	d, _ := newDispatcher(lister, notif, "zh-CN")

	d.runOnce(context.Background())
	d.runOnce(context.Background())
	if len(notif.calls) != 2 {
		t.Fatalf("skipped should retry each round: calls = %d, want 2", len(notif.calls))
	}
}

func TestReminderLabelLocale(t *testing.T) {
	if reminderLabel("en-US") != "Todo Reminder" {
		t.Errorf("en label wrong: %q", reminderLabel("en-US"))
	}
	for _, loc := range []string{"zh-CN", "auto", "", "fr"} {
		if reminderLabel(loc) != "待办提醒" {
			t.Errorf("label(%q) = %q, want 待办提醒", loc, reminderLabel(loc))
		}
	}
}

func TestDispatcherListerErrorNoCalls(t *testing.T) {
	lister := &fakeLister{err: context.DeadlineExceeded}
	notif := &fakeNotifier{sent: true}
	d, _ := newDispatcher(lister, notif, "zh-CN")
	d.runOnce(context.Background())
	if len(notif.calls) != 0 {
		t.Fatalf("lister error should yield no notifications, got %d", len(notif.calls))
	}
}
