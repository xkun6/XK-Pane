package notification

import (
	"path/filepath"
	"testing"
	"time"

	"xkpane/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

type fakeSender struct {
	calls []notifications.NotificationOptions
	err   error
}

func (f *fakeSender) SendNotification(o notifications.NotificationOptions) error {
	f.calls = append(f.calls, o)
	return f.err
}

func newTestService(t *testing.T) (*Service, *fakeSender) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fs := &fakeSender{}
	s := &Service{
		holder:   storage.NewHolder(db),
		notifier: fs,
		recent:   make(map[string]time.Time),
		nowFn:    time.Now,
	}
	return s, fs
}

func setEnabled(t *testing.T, s *Service, enabled, onlyUnfocused bool) {
	t.Helper()
	err := s.holder.Current().GORM.Model(&storage.AppSettings{}).Where("id = ?", 1).
		Updates(map[string]any{
			"notification_enabled":             enabled,
			"notification_only_when_unfocused": onlyUnfocused,
		}).Error
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
}

func TestTriggerSendsWhenEnabled(t *testing.T) {
	s, fs := newTestService(t)
	res, err := s.Trigger(Request{Kind: "custom", Title: "Hello", Body: "World"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sent {
		t.Fatalf("应发送, res=%+v", res)
	}
	if len(fs.calls) != 1 || fs.calls[0].Title != "Hello" || fs.calls[0].ID == "" {
		t.Errorf("SendNotification 调用不符: %+v", fs.calls)
	}
}

func TestTriggerSkipsWhenDisabled(t *testing.T) {
	s, fs := newTestService(t)
	setEnabled(t, s, false, false)
	res, _ := s.Trigger(Request{Kind: "custom", Title: "Hi"})
	if !res.Skipped || res.Reason != "notifications_disabled" {
		t.Errorf("应 skip notifications_disabled, got %+v", res)
	}
	if len(fs.calls) != 0 {
		t.Error("disabled 不该发送")
	}
}

func TestTriggerSkipsWhenFocusedAndOnlyWhenUnfocused(t *testing.T) {
	s, fs := newTestService(t)
	setEnabled(t, s, true, true)
	s.SetFocusCheck(func() bool { return true }) // 聚焦
	res, _ := s.Trigger(Request{Kind: "custom", Title: "Hi"})
	if !res.Skipped || res.Reason != "window_focused" {
		t.Errorf("聚焦 + only_when_unfocused 应 skip, got %+v", res)
	}
	if len(fs.calls) != 0 {
		t.Error("聚焦时不该发送")
	}
}

func TestTriggerDedupe(t *testing.T) {
	s, fs := newTestService(t)
	r := Request{Kind: "custom", Title: "Hi", DedupeKey: "k1"}
	if res, _ := s.Trigger(r); !res.Sent {
		t.Fatal("首次应发送")
	}
	if res, _ := s.Trigger(r); !res.Skipped || res.Reason != "deduped" {
		t.Errorf("10s 内重复应 deduped, got %+v", res)
	}
	if len(fs.calls) != 1 {
		t.Errorf("去重后只应发一次, got %d", len(fs.calls))
	}
}

func TestTriggerRequiresContent(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.Trigger(Request{Kind: "", Title: "x"}); err == nil {
		t.Error("空 kind 应报错")
	}
	if _, err := s.Trigger(Request{Kind: "custom", Title: "", Body: ""}); err == nil {
		t.Error("无 title/body 应报错")
	}
}

func TestTriggerPerCallOnlyWhenUnfocusedOverride(t *testing.T) {
	s, fs := newTestService(t)
	setEnabled(t, s, true, false) // 设置默认不限聚焦
	s.SetFocusCheck(func() bool { return true })
	yes := true
	res, _ := s.Trigger(Request{Kind: "custom", Title: "Hi", OnlyWhenUnfocused: &yes})
	if !res.Skipped || res.Reason != "window_focused" {
		t.Errorf("per-call override only_when_unfocused 应 skip, got %+v", res)
	}
	if len(fs.calls) != 0 {
		t.Error("override 聚焦时不该发送")
	}
}
